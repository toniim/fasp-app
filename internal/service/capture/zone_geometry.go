package capture

import (
	"image"
	"math"
)

// pointRect is a rectangle in points (fractional), as reported by a
// point-based windowing system such as AppKit.
type pointRect struct{ X, Y, W, H float64 }

// zonePixelRect maps a selection in points, relative to the top-left of a
// display, to the pixel rectangle of that display's screenshot. The scale is
// image pixels per point on each axis (2 on Retina). Edges are rounded
// outward so no selected pixel is lost, then clamped to the image.
func zonePixelRect(sel pointRect, scaleX, scaleY float64, img image.Rectangle) image.Rectangle {
	r := image.Rect(
		int(math.Floor(sel.X*scaleX)),
		int(math.Floor(sel.Y*scaleY)),
		int(math.Ceil((sel.X+sel.W)*scaleX)),
		int(math.Ceil((sel.Y+sel.H)*scaleY)),
	)
	return r.Canon().Intersect(img)
}

// zoneGlobalPoints turns a selection in points, relative to the top-left of a
// display whose top-left corner sits at (originX, originY) in global Quartz
// space (origin at the top-left of the main display, y down), into the global
// integer rectangle the recorder takes. Each edge is rounded on its own so
// the width never drifts by a point. The origin can be negative for a display
// left of or above the main one.
func zoneGlobalPoints(sel pointRect, originX, originY float64) (x, y, w, h int) {
	x0 := int(math.Round(originX + sel.X))
	y0 := int(math.Round(originY + sel.Y))
	x1 := int(math.Round(originX + sel.X + sel.W))
	y1 := int(math.Round(originY + sel.Y + sel.H))
	return x0, y0, x1 - x0, y1 - y0
}
