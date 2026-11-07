package capture

import (
	"image/color"
	"testing"
)

// GDI hands back BGRA with an undefined alpha byte; the PNG must come out as
// opaque RGBA in the same pixel order.
func TestBGRAToRGBASwapsChannelsAndForcesAlpha(t *testing.T) {
	pix := []byte{
		0x10, 0x20, 0x30, 0x00, // pixel (0,0): B G R X
		0xff, 0x00, 0x00, 0x7f, // pixel (1,0): pure blue
	}
	img := bgraToRGBA(pix, 2, 1)

	if got, want := img.RGBAAt(0, 0), (color.RGBA{R: 0x30, G: 0x20, B: 0x10, A: 0xff}); got != want {
		t.Errorf("pixel (0,0) = %v, want %v", got, want)
	}
	if got, want := img.RGBAAt(1, 0), (color.RGBA{B: 0xff, A: 0xff}); got != want {
		t.Errorf("pixel (1,0) = %v, want %v", got, want)
	}
}
