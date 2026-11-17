//go:build darwin

package capture

/*
#include <stdlib.h>
#include "capture_darwin.h"
*/
import "C"

import (
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"sync"
	"time"
	"unsafe"

	"github.com/heytonyne/fasp/internal/model"
)

// zoneBusy allows one overlay at a time.
var zoneBusy sync.Mutex

// frozenZone is what the overlay returns: the selection and the geometry
// needed to map it to pixels and to global points.
type frozenZone struct {
	sel              pointRect
	boundsX, boundsY float64
	boundsW, boundsH float64
	img              image.Rectangle
	image            C.CGImageRef // the whole screenshot, when requested
}

func (z *frozenZone) scale() (sx, sy float64) {
	return float64(z.img.Dx()) / z.boundsW, float64(z.img.Dy()) / z.boundsH
}

func (z *frozenZone) release() {
	C.fasp_release_image(z.image)
	z.image = 0
}

// selectFrozenZone captures the display under the cursor, shows it in the
// overlay (AppKit runs on the main thread, see zone_darwin.m) and blocks until
// the user selects a zone or cancels.
func selectFrozenZone(wantImage bool) (*frozenZone, error) {
	if !zoneBusy.TryLock() {
		return nil, errors.New("a zone selection is already open")
	}
	defer zoneBusy.Unlock()

	var res C.fasp_zone_result
	want := C.int(0)
	if wantImage {
		want = 1
	}
	status := C.fasp_zone_select(want, C.int(minZone), &res)
	switch status {
	case 1:
		return nil, ErrCancelled
	case 0:
	default:
		msg := "unknown error"
		if res.err != nil {
			msg = C.GoString(res.err)
			C.free(unsafe.Pointer(res.err))
		}
		C.fasp_release_image(res.image)
		return nil, fmt.Errorf("failed to capture screenshot: %w", errors.New(msg))
	}

	z := &frozenZone{
		sel:     pointRect{float64(res.selX), float64(res.selY), float64(res.selW), float64(res.selH)},
		boundsX: float64(res.boundsX), boundsY: float64(res.boundsY),
		boundsW: float64(res.boundsW), boundsH: float64(res.boundsH),
		img:   image.Rect(0, 0, int(res.imgW), int(res.imgH)),
		image: res.image,
	}
	if z.boundsW <= 0 || z.boundsH <= 0 || z.img.Empty() {
		z.release()
		return nil, errors.New("failed to capture screenshot: invalid display geometry")
	}
	return z, nil
}

// CaptureZone freezes the display under the cursor and returns the zone the
// user selects on it, cropped at pixel resolution with the display's colour
// profile, like a full screenshot.
func (s *serviceImpl) CaptureZone() (*model.CaptureResult, *model.ZoneMemory, error) {
	z, err := selectFrozenZone(true)
	if err != nil {
		return nil, nil, err
	}
	defer z.release()

	sx, sy := z.scale()
	res, err := cropZone(z.image, zonePixelRect(z.sel, sx, sy, z.img))
	if err != nil {
		return nil, nil, err
	}
	display := pointRect{z.boundsX, z.boundsY, z.boundsW, z.boundsH}
	return res, remember(display, z.sel), nil
}

// cropZone crops the pixel rectangle r out of a display screenshot and encodes
// it as PNG with the display's colour profile.
func cropZone(screenshot C.CGImageRef, r image.Rectangle) (*model.CaptureResult, error) {
	if !zoneLargeEnough(r) {
		return nil, errors.New("selected zone is too small")
	}
	var out unsafe.Pointer
	var n C.int
	var cerr *C.char
	if C.fasp_crop_png(screenshot, C.int(r.Min.X), C.int(r.Min.Y), C.int(r.Dx()), C.int(r.Dy()), &out, &n, &cerr) != 0 {
		msg := "unknown error"
		if cerr != nil {
			msg = C.GoString(cerr)
			C.free(unsafe.Pointer(cerr))
		}
		return nil, fmt.Errorf("failed to capture zone: %s", msg)
	}
	defer C.free(out)
	return &model.CaptureResult{
		Data:      base64.StdEncoding.EncodeToString(C.GoBytes(out, n)),
		Width:     r.Dx(),
		Height:    r.Dy(),
		Timestamp: time.Now(),
	}, nil
}

// CaptureZoneRect captures a remembered zone without the overlay: the display
// is found by its bounds, captured, and the zone is cropped from it exactly as
// the overlay path does, so the result has the same pixel size.
func (s *serviceImpl) CaptureZoneRect(zone model.ZoneMemory) (*model.CaptureResult, error) {
	var list [16]C.fasp_display
	n := int(C.fasp_list_displays(&list[0], C.int(len(list))))
	bounds := make([]pointRect, n)
	for i := 0; i < n; i++ {
		bounds[i] = pointRect{float64(list[i].x), float64(list[i].y), float64(list[i].w), float64(list[i].h)}
	}
	idx, err := resolveZone(zone, bounds)
	if err != nil {
		return nil, err
	}

	var shot C.CGImageRef
	var cerr *C.char
	if C.fasp_capture_display_image(C.CGDirectDisplayID(list[idx].id), &shot, &cerr) != 0 {
		msg := "unknown error"
		if cerr != nil {
			msg = C.GoString(cerr)
			C.free(unsafe.Pointer(cerr))
		}
		return nil, fmt.Errorf("failed to capture screenshot: %s", msg)
	}
	defer C.fasp_release_image(shot)

	img := image.Rect(0, 0, int(C.CGImageGetWidth(shot)), int(C.CGImageGetHeight(shot)))
	d := bounds[idx]
	if d.W <= 0 || d.H <= 0 || img.Empty() {
		return nil, errors.New("failed to capture screenshot: invalid display geometry")
	}
	sx, sy := float64(img.Dx())/d.W, float64(img.Dy())/d.H
	sel := pointRect{zone.X, zone.Y, zone.Width, zone.Height}
	r := zonePixelRect(sel, sx, sy, img)
	if !zoneLargeEnough(r) {
		return nil, ErrZoneUnavailable
	}
	return cropZone(shot, r)
}

// SelectRegion shows the same overlay and returns the zone in global Quartz
// points (origin at the top-left of the main display, y down), which is what
// the recorder takes for a region.
func (s *serviceImpl) SelectRegion() (*model.CropRegion, error) {
	z, err := selectFrozenZone(false)
	if err != nil {
		return nil, err
	}
	x, y, w, h := zoneGlobalPoints(z.sel, z.boundsX, z.boundsY)
	return &model.CropRegion{X: x, Y: y, Width: w, Height: h}, nil
}
