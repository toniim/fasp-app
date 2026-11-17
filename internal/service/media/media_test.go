package media

import (
	"bytes"
	"image/gif"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

const sample = "testdata/sample.mp4" // 3 s, 640x360, 30 fps, H.264 + AAC

func near(t *testing.T, what string, got, want, tol time.Duration) {
	t.Helper()
	if d := got - want; d < -tol || d > tol {
		t.Errorf("%s = %v, want %v +-%v", what, got, want, tol)
	}
}

func TestProbe(t *testing.T) {
	d, w, h, err := Probe(sample)
	if err != nil {
		t.Fatal(err)
	}
	near(t, "duration", d, 3*time.Second, 100*time.Millisecond)
	if w != 640 || h != 360 {
		t.Errorf("size = %dx%d, want 640x360", w, h)
	}
	if _, _, _, err := Probe(filepath.Join(t.TempDir(), "missing.mp4")); err == nil {
		t.Error("Probe of a missing file succeeded")
	}
}

func TestTrim(t *testing.T) {
	cases := []struct {
		name       string
		start, end time.Duration
	}{
		{"mid-gop start is re-encoded", 500 * time.Millisecond, 2 * time.Second},
		{"keyframe start", 0, time.Second},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "trim.mp4")
			if err := Trim(sample, out, c.start, c.end); err != nil {
				t.Fatal(err)
			}
			d, w, h, err := Probe(out)
			if err != nil {
				t.Fatal(err)
			}
			near(t, "trimmed duration", d, c.end-c.start, 100*time.Millisecond)
			if w != 640 || h != 360 {
				t.Errorf("size = %dx%d, want 640x360", w, h)
			}
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			// The handler type of an audio track is written as "soun".
			if !bytes.Contains(data, []byte("soun")) {
				t.Error("trimmed file has no audio track")
			}
		})
	}
}

// thumb decodes the frame at t, scaled to 96x54 BGRA.
func thumb(t *testing.T, path string, at time.Duration) []byte {
	t.Helper()
	var px []byte
	err := decodeFrames(path, at, 1, 30, 96, 54, func(_ int, bgra []byte) error {
		px = append([]byte(nil), bgra...)
		return nil
	})
	if err != nil {
		t.Fatalf("decode %s at %v: %v", path, at, err)
	}
	return px
}

func meanDiff(a, b []byte) float64 {
	var sum float64
	for i := range a {
		sum += math.Abs(float64(a[i]) - float64(b[i]))
	}
	return sum / float64(len(a))
}

// A recording larger than 4K must keep its size when it is cut off a keyframe,
// and the cut must start at the requested time, not at the keyframe before it.
func TestTrimKeepsSizeAboveUHD(t *testing.T) {
	const big = "testdata/big.mp4" // 2 s, 4800x2700, 30 fps, H.264 + AAC
	out := filepath.Join(t.TempDir(), "trim.mp4")
	start, end := 400*time.Millisecond, 1400*time.Millisecond
	if err := Trim(big, out, start, end); err != nil {
		t.Fatal(err)
	}
	d, w, h, err := Probe(out)
	if err != nil {
		t.Fatal(err)
	}
	near(t, "trimmed duration", d, end-start, 100*time.Millisecond)
	if runtime.GOOS == "windows" && (w != 4800 || h != 2700) {
		t.Skipf("Media Foundation re-encoded to %dx%d; the above-4K size is only asserted on macOS", w, h)
	}
	if w != 4800 || h != 2700 {
		t.Errorf("size = %dx%d, want 4800x2700", w, h)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("soun")) {
		t.Error("trimmed file has no audio track")
	}
	// testsrc draws a running counter, so a frame differs from every other
	// time: the output frame at 0.5 s must be the source frame at 0.9 s.
	got := thumb(t, out, 500*time.Millisecond)
	if d := meanDiff(got, thumb(t, big, 900*time.Millisecond)); d > 8 {
		t.Errorf("frame at 0.5 s differs from the source at 0.9 s (mean diff %.1f)", d)
	}
	if d := meanDiff(got, thumb(t, big, 500*time.Millisecond)); d < 2 {
		t.Errorf("frame at 0.5 s matches the source at 0.5 s (mean diff %.1f): the cut did not shift", d)
	}

	// The keyframe start takes the copy path: it must end where asked too.
	kf := filepath.Join(t.TempDir(), "kf.mp4")
	if err := Trim(big, kf, 0, time.Second); err != nil {
		t.Fatal(err)
	}
	d, w, h, err = Probe(kf)
	if err != nil {
		t.Fatal(err)
	}
	near(t, "keyframe-start duration", d, time.Second, 100*time.Millisecond)
	if w != 4800 || h != 2700 {
		t.Errorf("keyframe-start size = %dx%d, want 4800x2700", w, h)
	}
}

func TestTrimWithoutAudio(t *testing.T) {
	const silent = "testdata/silent.mp4" // 2 s, 640x360, 30 fps, H.264, no audio
	for _, start := range []time.Duration{0, 400 * time.Millisecond} { // keyframe, then mid-GOP
		out := filepath.Join(t.TempDir(), "trim.mp4")
		if err := Trim(silent, out, start, start+time.Second); err != nil {
			t.Fatalf("start %v: %v", start, err)
		}
		d, w, h, err := Probe(out)
		if err != nil {
			t.Fatal(err)
		}
		near(t, "trimmed duration", d, time.Second, 100*time.Millisecond)
		if w != 640 || h != 360 {
			t.Errorf("size = %dx%d, want 640x360", w, h)
		}
	}
}

func TestTrimRefusesBadRange(t *testing.T) {
	dir := t.TempDir()
	if err := Trim(sample, filepath.Join(dir, "a.mp4"), 2*time.Second, time.Second); err == nil {
		t.Error("end before start succeeded")
	}
	if err := Trim(sample, sample, 0, time.Second); err == nil {
		t.Error("trimming onto the input succeeded")
	}
}

func TestExportGIF(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.gif")
	var last, total int
	err := ExportGIF(sample, out, GIFOptions{End: 2 * time.Second, FPS: 10, Width: 320}, func(done, n int) {
		last, total = done, n
	})
	if err != nil {
		t.Fatal(err)
	}
	if last != total || total != 20 {
		t.Errorf("progress ended at %d/%d, want 20/20", last, total)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatalf("not a valid GIF: %v", err)
	}
	if n := len(g.Image); n < 19 || n > 21 {
		t.Errorf("%d frames, want 20 +-1", n)
	}
	if g.Config.Width != 320 || g.Config.Height != 180 {
		t.Errorf("size = %dx%d, want 320x180", g.Config.Width, g.Config.Height)
	}
	for i, d := range g.Delay {
		if d != 10 {
			t.Errorf("frame %d delay = %d, want 10", i, d)
		}
	}
	if g.LoopCount != 0 {
		t.Errorf("loop count = %d, want 0 (forever)", g.LoopCount)
	}
	if _, err := os.Stat(out + ".part.gif"); err == nil {
		t.Error("temporary file left behind")
	}
}

func TestEstimateGIFSize(t *testing.T) {
	opt := GIFOptions{End: 2 * time.Second, FPS: 10, Width: 320}
	out := filepath.Join(t.TempDir(), "out.gif")
	if err := ExportGIF(sample, out, opt, nil); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	est, err := EstimateGIFSize(sample, opt)
	if err != nil {
		t.Fatal(err)
	}
	if ratio := float64(est) / float64(st.Size()); ratio < 0.5 || ratio > 2 {
		t.Errorf("estimate %d vs real %d (ratio %.2f), want within a factor of 2", est, st.Size(), ratio)
	}
}

func TestPlanGIFDefaults(t *testing.T) {
	p, err := planGIF(sample, GIFOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if p.fps != 12 || p.w != 640 || p.h != 360 {
		t.Errorf("defaults = %d fps %dx%d, want 12 fps 640x360", p.fps, p.w, p.h)
	}
	if want := int(math.Round(p.duration.Seconds() * 12)); p.count != want {
		t.Errorf("count = %d, want %d", p.count, want)
	}
	p, err = planGIF(sample, GIFOptions{FPS: 99, Width: 100, Start: time.Second, End: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if p.fps != 30 || p.w != 100 || p.h != 56 {
		t.Errorf("clamped = %d fps %dx%d, want 30 fps 100x56", p.fps, p.w, p.h)
	}
	near(t, "range", p.duration, 2*time.Second, 100*time.Millisecond)
}
