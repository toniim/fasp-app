package media

import (
	"image"

	"golang.org/x/image/draw"
)

// scaler resizes BGRA frames with a Catmull-Rom filter. macOS scales inside
// vImage; the Media Foundation reader only gives full-size frames, so Windows
// scales here. The channels are filtered independently, so the byte order of
// the frame does not matter: it is treated as RGBA and comes back as it went in.
type scaler struct {
	src, dst *image.RGBA
}

func newScaler(sw, sh, dw, dh int) *scaler {
	s := &scaler{src: image.NewRGBA(image.Rect(0, 0, sw, sh))}
	if sw != dw || sh != dh {
		s.dst = image.NewRGBA(image.Rect(0, 0, dw, dh))
	}
	return s
}

// row returns the bytes of row y of the source frame, to be filled by the caller.
func (s *scaler) row(y int) []byte {
	return s.src.Pix[y*s.src.Stride : y*s.src.Stride+s.src.Rect.Dx()*4]
}

// scale returns the filled source frame at the destination size. The result
// is valid until the next call.
func (s *scaler) scale() []byte {
	// The fourth byte of a Media Foundation RGB32 pixel is unused and often 0.
	// The filter treats it as alpha, and a zero alpha would blank the colours.
	for i := 3; i < len(s.src.Pix); i += 4 {
		s.src.Pix[i] = 0xFF
	}
	if s.dst == nil {
		return s.src.Pix
	}
	draw.CatmullRom.Scale(s.dst, s.dst.Bounds(), s.src, s.src.Bounds(), draw.Src, nil)
	return s.dst.Pix
}
