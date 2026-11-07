//go:build darwin

package capture

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"os"
	"testing"
	"time"
)

// Captures the real screen, so it needs Screen Recording permission for the
// process running the test. Opt in with FASP_CAPTURE_TEST=1.
func TestCaptureActiveDisplayReturnsPNG(t *testing.T) {
	if os.Getenv("FASP_CAPTURE_TEST") != "1" {
		t.Skip("set FASP_CAPTURE_TEST=1 to capture the real screen")
	}

	start := time.Now()
	res, err := New().CaptureActiveDisplay()
	if err != nil {
		t.Fatalf("CaptureActiveDisplay: %v", err)
	}
	elapsed := time.Since(start)

	data, err := base64.StdEncoding.DecodeString(res.Data)
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("result is not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != res.Width || b.Dy() != res.Height {
		t.Fatalf("reported %dx%d, PNG is %dx%d", res.Width, res.Height, b.Dx(), b.Dy())
	}
	t.Logf("captured %dx%d in %v", res.Width, res.Height, elapsed)
}
