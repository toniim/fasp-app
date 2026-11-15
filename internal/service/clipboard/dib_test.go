package clipboard

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

func testDIBSource() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 0xff, A: 0xff})
	img.SetNRGBA(1, 0, color.NRGBA{G: 0x80, A: 0xff})
	img.SetNRGBA(2, 0, color.NRGBA{B: 0xff, A: 0xff})
	img.SetNRGBA(0, 1, color.NRGBA{R: 1, G: 2, B: 3, A: 0xff})
	img.SetNRGBA(1, 1, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	img.SetNRGBA(2, 1, color.NRGBA{R: 9, G: 8, B: 7, A: 0xff})
	return img
}

func samePixels(t *testing.T, got, want *image.NRGBA) {
	t.Helper()
	if got.Bounds() != want.Bounds() {
		t.Fatalf("bounds = %v, want %v", got.Bounds(), want.Bounds())
	}
	for y := 0; y < want.Bounds().Dy(); y++ {
		for x := 0; x < want.Bounds().Dx(); x++ {
			if g, w := got.NRGBAAt(x, y), want.NRGBAAt(x, y); g != w {
				t.Errorf("pixel (%d,%d) = %v, want %v", x, y, g, w)
			}
		}
	}
}

// The decoder must read back what the encoder wrote, with rows and channels
// in the right order.
func TestDIBToImageRoundTrip(t *testing.T) {
	want := testDIBSource()
	got, err := dibToImage(imageToDIB(want))
	if err != nil {
		t.Fatal(err)
	}
	samePixels(t, got, want)
}

// Top-down DIBs (negative height) store the first row first.
func TestDIBToImageTopDown(t *testing.T) {
	want := testDIBSource()
	dib := imageToDIB(want)
	w, h := 3, 2
	binary.LittleEndian.PutUint32(dib[8:], uint32(int32(-h)))
	flipped := make([]byte, len(dib))
	copy(flipped, dib[:bitmapInfoHeaderSize])
	for y := 0; y < h; y++ {
		copy(flipped[bitmapInfoHeaderSize+y*w*4:], dib[bitmapInfoHeaderSize+(h-1-y)*w*4:bitmapInfoHeaderSize+(h-y)*w*4])
	}
	got, err := dibToImage(flipped)
	if err != nil {
		t.Fatal(err)
	}
	samePixels(t, got, want)
}

// BI_BITFIELDS 32 bpp with the masks after a 40-byte header, the layout apps
// such as browsers produce (here RGB byte order instead of BGR).
func TestDIBToImageBitfields(t *testing.T) {
	le := binary.LittleEndian
	dib := make([]byte, bitmapInfoHeaderSize+12+4)
	le.PutUint32(dib[0:], bitmapInfoHeaderSize)
	le.PutUint32(dib[4:], 1)
	le.PutUint32(dib[8:], 1)
	le.PutUint16(dib[12:], 1)
	le.PutUint16(dib[14:], 32)
	le.PutUint32(dib[16:], biBitfields)
	le.PutUint32(dib[40:], 0x000000ff) // red in the low byte
	le.PutUint32(dib[44:], 0x0000ff00)
	le.PutUint32(dib[48:], 0x00ff0000)
	le.PutUint32(dib[52:], 0x000011ff) // pixel: r=0xff g=0x11 b=0
	got, err := dibToImage(dib)
	if err != nil {
		t.Fatal(err)
	}
	if px := got.NRGBAAt(0, 0); px != (color.NRGBA{R: 0xff, G: 0x11, B: 0, A: 0xff}) {
		t.Errorf("pixel = %v, want r=ff g=11 b=0 a=ff", px)
	}
}

// 24 bpp rows are padded to 4 bytes.
func TestDIBToImage24bpp(t *testing.T) {
	le := binary.LittleEndian
	// 1x2, bottom-up: stride = 4. Row 0 stored first = bottom row.
	dib := make([]byte, bitmapInfoHeaderSize+8)
	le.PutUint32(dib[0:], bitmapInfoHeaderSize)
	le.PutUint32(dib[4:], 1)
	le.PutUint32(dib[8:], 2)
	le.PutUint16(dib[12:], 1)
	le.PutUint16(dib[14:], 24)
	copy(dib[40:], []byte{3, 2, 1, 0}) // bottom: r=1 g=2 b=3
	copy(dib[44:], []byte{9, 8, 7, 0}) // top: r=7 g=8 b=9
	got, err := dibToImage(dib)
	if err != nil {
		t.Fatal(err)
	}
	if px := got.NRGBAAt(0, 0); px != (color.NRGBA{R: 7, G: 8, B: 9, A: 0xff}) {
		t.Errorf("top = %v", px)
	}
	if px := got.NRGBAAt(0, 1); px != (color.NRGBA{R: 1, G: 2, B: 3, A: 0xff}) {
		t.Errorf("bottom = %v", px)
	}
}

func TestDIBToImageRejectsBadInput(t *testing.T) {
	good := imageToDIB(testDIBSource())
	cases := map[string][]byte{
		"empty":     nil,
		"truncated": good[:len(good)-1],
		"short":     good[:20],
	}
	for name, in := range cases {
		if _, err := dibToImage(in); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

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
