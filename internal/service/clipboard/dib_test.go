package clipboard

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

// A CF_DIB block is a 40-byte header plus bottom-up BGRA rows; getting the
// row order or channel order wrong pastes the image flipped or with red and
// blue swapped.
func TestImageToDIBLayout(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff}) // top-left red
	img.Set(1, 1, color.RGBA{B: 0xff, A: 0xff}) // bottom-right blue

	dib := imageToDIB(img)

	if len(dib) != 40+2*2*4 {
		t.Fatalf("len = %d, want %d", len(dib), 40+16)
	}
	le := binary.LittleEndian
	if w, h, bpp := le.Uint32(dib[4:]), int32(le.Uint32(dib[8:])), le.Uint16(dib[14:]); w != 2 || h != 2 || bpp != 32 {
		t.Fatalf("header width=%d height=%d bpp=%d, want 2, 2, 32", w, h, bpp)
	}

	pix := dib[40:]
	// Bottom-up: the first stored row is the image's last row.
	if got := pix[4:8]; got[0] != 0xff || got[2] != 0 {
		t.Errorf("first stored row, x=1 = % x, want blue (ff 00 00 ff)", got)
	}
	if got := pix[8:12]; got[2] != 0xff || got[0] != 0 {
		t.Errorf("second stored row, x=0 = % x, want red (00 00 ff ff)", got)
	}
}
