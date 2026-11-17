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

// windowAt returns the window a click at (x, y) takes: the front-most window
// in wins (Z order, front-most first) that contains the point, clipped to
// bounds. A window behind it is never chosen, since it is not what the user
// sees there; ok is false when there is none or the clipped window is too
// small.
func windowAt(wins []image.Rectangle, x, y int, bounds image.Rectangle) (r image.Rectangle, ok bool) {
	p := image.Pt(x, y)
	for _, w := range wins {
		if p.In(w) {
			r = w.Intersect(bounds)
			return r, zoneLargeEnough(r)
		}
	}
	return image.Rectangle{}, false
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
