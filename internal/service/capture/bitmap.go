package capture

import "image"

// bgraToRGBA converts a top-down 32 bpp BGRA pixel buffer (what GDI GetDIBits
// returns) to an opaque RGBA image. GDI leaves the fourth byte undefined, so
// alpha is forced to 255.
func bgraToRGBA(pix []byte, width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i+3 < len(pix) && i+3 < len(img.Pix); i += 4 {
		img.Pix[i+0] = pix[i+2]
		img.Pix[i+1] = pix[i+1]
		img.Pix[i+2] = pix[i+0]
		img.Pix[i+3] = 0xff
	}
	return img
}
