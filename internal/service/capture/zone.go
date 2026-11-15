package capture

import (
	"errors"
	"image"
)

// ErrCancelled is returned by CaptureZone when the user pressed Esc or
// right-clicked instead of selecting a zone.
var ErrCancelled = errors.New("capture cancelled")

// minZone is the smallest selection accepted, in pixels; a plain click (or a
// tiny drag) keeps the overlay open instead of producing a 1×1 image.
const minZone = 4

// normalizeZone turns two drag corners into a rectangle clamped to bounds.
func normalizeZone(x0, y0, x1, y1 int, bounds image.Rectangle) image.Rectangle {
	return image.Rect(x0, y0, x1, y1).Canon().Intersect(bounds)
}

func zoneLargeEnough(r image.Rectangle) bool {
	return r.Dx() >= minZone && r.Dy() >= minZone
}

// cropBGRA copies r out of a top-down BGRA buffer of the given width.
func cropBGRA(pix []byte, width int, r image.Rectangle) []byte {
	out := make([]byte, r.Dx()*r.Dy()*4)
	row := r.Dx() * 4
	for y := 0; y < r.Dy(); y++ {
		src := ((r.Min.Y+y)*width + r.Min.X) * 4
		copy(out[y*row:(y+1)*row], pix[src:src+row])
	}
	return out
}
