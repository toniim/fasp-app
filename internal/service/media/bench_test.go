package media

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestClipTimings times a GIF export and a trim of a real clip. It only runs
// when FASP_MEDIA_CLIP names a recording, so CI is unaffected. FASP_MEDIA_OUT
// names a directory that keeps the outputs:
//
//	FASP_MEDIA_CLIP=/path/clip.mp4 go test ./internal/service/media -run ClipTimings -v
func TestClipTimings(t *testing.T) {
	clip := os.Getenv("FASP_MEDIA_CLIP")
	if clip == "" {
		t.Skip("FASP_MEDIA_CLIP not set")
	}
	dir := os.Getenv("FASP_MEDIA_OUT") // keep the outputs here to look at them
	if dir == "" {
		dir = t.TempDir()
	}
	dur, w, h, err := Probe(clip)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("clip: %v %dx%d", dur, w, h)

	opt := GIFOptions{FPS: 12, Width: 960}
	t0 := time.Now()
	est, err := EstimateGIFSize(clip, opt)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("estimate: %d bytes in %v", est, time.Since(t0))

	out := filepath.Join(dir, "clip.gif")
	t0 = time.Now()
	if err := ExportGIF(clip, out, opt, nil); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("gif export: %v, %d bytes", time.Since(t0), st.Size())

	t0 = time.Now()
	if err := Trim(clip, filepath.Join(dir, "trim.mp4"), 2*time.Second, 7*time.Second); err != nil {
		t.Fatal(err)
	}
	t.Logf("trim 2s-7s: %v", time.Since(t0))
}
