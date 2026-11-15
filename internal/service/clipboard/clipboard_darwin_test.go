//go:build darwin

package clipboard

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Writes to the real pasteboard, so it is opt-in with FASP_CLIPBOARD_TEST=1.
func TestReadImageReturnsWhatCopyImageWrote(t *testing.T) {
	if os.Getenv("FASP_CLIPBOARD_TEST") != "1" {
		t.Skip("set FASP_CLIPBOARD_TEST=1 to use the real pasteboard")
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 7, 5))); err != nil {
		t.Fatal(err)
	}
	svc := New()
	if err := svc.CopyImage(buf.Bytes()); err != nil {
		t.Fatalf("CopyImage: %v", err)
	}

	data, err := svc.ReadImage()
	if err != nil {
		t.Fatalf("ReadImage: %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("result is not a PNG: %v", err)
	}
	if cfg.Width != 7 || cfg.Height != 5 {
		t.Fatalf("size = %dx%d, want 7x5", cfg.Width, cfg.Height)
	}
}

// Writes to the real pasteboard (overwriting what the user copied), so it is
// opt-in with FASP_CLIPBOARD_TEST=1.
func TestCopyImagePutsPNGOnPasteboard(t *testing.T) {
	if os.Getenv("FASP_CLIPBOARD_TEST") != "1" {
		t.Skip("set FASP_CLIPBOARD_TEST=1 to write to the real pasteboard")
	}

	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(1, 1, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	if err := New().CopyImage(buf.Bytes()); err != nil {
		t.Fatalf("CopyImage: %v", err)
	}

	out, err := exec.Command("osascript", "-e", "clipboard info").Output()
	if err != nil {
		t.Fatalf("clipboard info: %v", err)
	}
	if !strings.Contains(string(out), "PNGf") {
		t.Fatalf("pasteboard has no PNG: %s", out)
	}
}
