package main

import (
	"context"
	"errors"
	"image/gif"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/heytonyne/fasp/internal/service/media"
)

const sampleVideo = "internal/service/media/testdata/sample.mp4"

// collect gathers the events of one job until it reaches a final state.
func collect(t *testing.T, events <-chan MediaJobEvent) []MediaJobEvent {
	t.Helper()
	var got []MediaJobEvent
	timeout := time.After(60 * time.Second)
	for {
		select {
		case e := <-events:
			got = append(got, e)
			if e.State != jobRunning {
				return got
			}
		case <-timeout:
			t.Fatalf("job did not finish; events so far: %+v", got)
		}
	}
}

func newTestJobs() (*mediaJobs, chan MediaJobEvent) {
	events := make(chan MediaJobEvent, 1024)
	return newMediaJobs(func(e MediaJobEvent) { events <- e }), events
}

func skipIfNoMedia(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, media.ErrUnsupported) {
		t.Skip("no media backend on this platform")
	}
}

func TestGIFExportJobReportsProgressAndWritesGIF(t *testing.T) {
	a := NewApp()
	var events chan MediaJobEvent
	a.jobs, events = newTestJobs()

	id, err := a.StartGIFExport(sampleVideo, 0, 0, 10, 160)
	if err != nil {
		t.Fatalf("StartGIFExport: %v", err)
	}
	got := collect(t, events)
	last := got[len(got)-1]
	if last.State == jobError {
		skipIfNoMedia(t, errors.New(last.Error))
		t.Fatalf("job failed: %s", last.Error)
	}
	if last.State != jobDone || last.ID != id || last.Kind != "gif" {
		t.Fatalf("last event = %+v", last)
	}
	var sawProgress bool
	for _, e := range got {
		if e.Total > 0 && e.Done > 0 && e.Done <= e.Total {
			sawProgress = true
		}
	}
	if !sawProgress {
		t.Errorf("no progress events: %+v", got)
	}
	if filepath.Ext(last.Path) != ".gif" || last.SizeBytes <= 0 {
		t.Fatalf("result = %q (%d bytes)", last.Path, last.SizeBytes)
	}
	f, err := os.Open(last.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatalf("the result is not a valid GIF: %v", err)
	}
	if len(g.Image) == 0 || g.Config.Width != 160 {
		t.Errorf("gif has %d frames, width %d", len(g.Image), g.Config.Width)
	}

	// Releasing deletes the temporary output.
	a.ReleaseMediaJob(id)
	if _, err := os.Stat(last.Path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("released job left %s behind (%v)", last.Path, err)
	}
}

func TestGIFExportJobCanBeCancelled(t *testing.T) {
	a := NewApp()
	var events chan MediaJobEvent
	a.jobs, events = newTestJobs()

	// Cancel from the first progress event, while frames are still left.
	id, err := a.StartGIFExport("internal/service/media/testdata/big.mp4", 0, 0, 15, 480)
	if err != nil {
		t.Fatalf("StartGIFExport: %v", err)
	}
	var got []MediaJobEvent
	timeout := time.After(60 * time.Second)
loop:
	for {
		select {
		case e := <-events:
			got = append(got, e)
			if e.State == jobRunning && e.Done > 0 {
				a.CancelMediaJob(id)
			}
			if e.State != jobRunning {
				break loop
			}
		case <-timeout:
			t.Fatalf("job did not stop; events: %+v", got)
		}
	}
	last := got[len(got)-1]
	if last.State == jobError {
		skipIfNoMedia(t, errors.New(last.Error))
	}
	if last.State != jobCancelled {
		t.Fatalf("last event = %+v, want cancelled", last)
	}
	if _, ok := a.jobs.dir(id); ok {
		t.Error("a cancelled job must be released")
	}
}

func TestTrimJobWritesTheRequestedRange(t *testing.T) {
	a := NewApp()
	var events chan MediaJobEvent
	a.jobs, events = newTestJobs()

	total, _, _, err := media.Probe(sampleVideo)
	if err != nil {
		skipIfNoMedia(t, err)
		t.Fatal(err)
	}
	start, end := total/4, total*3/4
	dst := filepath.Join(t.TempDir(), "out.mp4")
	if _, err := a.jobs.start("trim", a.trimJob(sampleVideo, dst, start, end)); err != nil {
		t.Fatal(err)
	}
	last := func() MediaJobEvent { g := collect(t, events); return g[len(g)-1] }()
	if last.State != jobDone || last.Path != dst {
		t.Fatalf("last event = %+v", last)
	}
	got, _, _, err := media.Probe(dst)
	if err != nil {
		t.Fatal(err)
	}
	want := end - start
	if diff := got - want; diff < -150*time.Millisecond || diff > 150*time.Millisecond {
		t.Errorf("trimmed length = %v, want about %v", got, want)
	}
}

func TestCancelledTrimLeavesNothingAtTheDestination(t *testing.T) {
	a := NewApp()
	var events chan MediaJobEvent
	a.jobs, events = newTestJobs()
	dst := filepath.Join(t.TempDir(), "out.mp4")

	ctxStarted := make(chan struct{})
	release := make(chan struct{})
	inner := a.trimJob(sampleVideo, dst, 0, time.Second)
	id, err := a.jobs.start("trim", func(ctx context.Context, dir string, progress func(done, total int)) (string, error) {
		close(ctxStarted)
		<-release // the job is cancelled while the trim is still running
		return inner(ctx, dir, progress)
	})
	if err != nil {
		t.Fatal(err)
	}
	<-ctxStarted
	a.CancelMediaJob(id)
	close(release)

	last := func() MediaJobEvent { g := collect(t, events); return g[len(g)-1] }()
	if last.State != jobCancelled {
		t.Fatalf("last event = %+v, want cancelled", last)
	}
	if _, err := os.Stat(dst); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a cancelled trim wrote %s (%v)", dst, err)
	}
}

func TestMediaJobOptionsAreValidated(t *testing.T) {
	cases := []struct {
		name              string
		start, end        int64
		fps, width        int
		wantErrSubstrings bool
	}{
		{"ok", 0, 0, 10, 480, false},
		{"ok range", 1000, 5000, 15, 0, false},
		{"negative start", -1, 0, 10, 480, true},
		{"end before start", 5000, 1000, 10, 480, true},
		{"fps too high", 0, 0, 60, 480, true},
		{"fps zero", 0, 0, 0, 480, true},
		{"width absurd", 0, 0, 10, 100000, true},
	}
	for _, c := range cases {
		_, err := gifOptions(c.start, c.end, c.fps, c.width)
		if (err != nil) != c.wantErrSubstrings {
			t.Errorf("%s: err = %v", c.name, err)
		}
	}
}

func TestSavedGIFMustBelongToTheJob(t *testing.T) {
	a := NewApp()
	a.jobs, _ = newTestJobs()
	id, err := a.jobs.start("gif", func(ctx context.Context, dir string, _ func(int, int)) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.ReleaseMediaJob(id)
	if _, err := a.QuickSaveGIF(id, "/etc/hosts"); err == nil {
		t.Fatal("a file outside the job folder must be refused")
	}
	if _, err := a.SaveGIFAs("unknown", "/tmp/x.gif"); err == nil {
		t.Fatal("an unknown job must be refused")
	}
}

func TestUniquePathNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	if got := uniquePath(dir, "a.gif"); got != filepath.Join(dir, "a.gif") {
		t.Fatalf("free name = %s", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.gif"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := uniquePath(dir, "a.gif"); got != filepath.Join(dir, "a_1.gif") {
		t.Fatalf("taken name = %s", got)
	}
}
