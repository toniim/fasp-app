package clipboard

import (
	"encoding/binary"
	"image"
)

const bitmapInfoHeaderSize = 40

// imageToDIB encodes an image as a packed CF_DIB block: a BITMAPINFOHEADER
// followed by bottom-up 32 bpp BGRA rows, the layout Windows apps read from
// the clipboard.
func imageToDIB(img image.Image) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]byte, bitmapInfoHeaderSize+w*h*4)

	le := binary.LittleEndian
	le.PutUint32(out[0:], bitmapInfoHeaderSize) // biSize
	le.PutUint32(out[4:], uint32(w))            // biWidth
	le.PutUint32(out[8:], uint32(h))            // biHeight > 0: bottom-up
	le.PutUint16(out[12:], 1)                   // biPlanes
	le.PutUint16(out[14:], 32)                  // biBitCount
	le.PutUint32(out[16:], 0)                   // biCompression = BI_RGB
	le.PutUint32(out[20:], uint32(w*h*4))       // biSizeImage

	pix := out[bitmapInfoHeaderSize:]
	for y := 0; y < h; y++ {
		row := pix[(h-1-y)*w*4:]
		for x := 0; x < w; x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			row[x*4+0] = byte(bl >> 8)
			row[x*4+1] = byte(g >> 8)
			row[x*4+2] = byte(r >> 8)
			row[x*4+3] = byte(a >> 8)
		}
	}
	return out
}
