package capture

import (
	"errors"
	"image"
	"math"

	"github.com/heytonyne/fasp/internal/model"
)

// ErrZoneUnavailable is returned when a remembered zone cannot be captured
// again: its display is gone or changed size, or the zone is not on it.
var ErrZoneUnavailable = errors.New("the last zone is no longer on screen")

// zoneEpsilon absorbs float noise when comparing remembered geometry with the
// live one (a Retina selection can sit on half points).
const zoneEpsilon = 0.5

// remember describes a selection on a display as the zone to store.
func remember(display, sel pointRect) *model.ZoneMemory {
	return &model.ZoneMemory{
		DisplayX: display.X, DisplayY: display.Y, DisplayWidth: display.W, DisplayHeight: display.H,
		X: sel.X, Y: sel.Y, Width: sel.W, Height: sel.H,
	}
}

// resolveZone finds the display a remembered zone belongs to among the live
// displays (their global bounds) and returns its index. A display matches only
// when its position and size are the same as when the zone was picked, so a
// changed resolution or a moved display never captures a wrong area. The zone
// must also lie inside the display and be large enough.
func resolveZone(z model.ZoneMemory, displays []pointRect) (int, error) {
	// The size limit is checked in pixels once the zone is mapped to the image.
	if z.Width <= 0 || z.Height <= 0 {
		return 0, ErrZoneUnavailable
	}
	for i, d := range displays {
		if math.Abs(d.X-z.DisplayX) > zoneEpsilon || math.Abs(d.Y-z.DisplayY) > zoneEpsilon ||
			math.Abs(d.W-z.DisplayWidth) > zoneEpsilon || math.Abs(d.H-z.DisplayHeight) > zoneEpsilon {
			continue
		}
		if z.X < -zoneEpsilon || z.Y < -zoneEpsilon ||
			z.X+z.Width > d.W+zoneEpsilon || z.Y+z.Height > d.H+zoneEpsilon {
			return 0, ErrZoneUnavailable
		}
		return i, nil
	}
	return 0, ErrZoneUnavailable
}

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
