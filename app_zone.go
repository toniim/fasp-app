package main

import (
	"errors"
	"fmt"

	"github.com/heytonyne/fasp/internal/model"
	"github.com/heytonyne/fasp/internal/service/capture"
)

// CaptureZone freezes the display under the cursor, lets the user drag a zone
// and returns only that zone. It returns nil (not an error) when the user
// cancels with Esc, so the frontend can simply ignore it.
func (a *App) CaptureZone() (*model.CaptureResult, error) {
	if err := a.requireScreenRecording(); err != nil {
		return nil, err
	}
	r, zone, err := a.captureService.CaptureZone()
	if errors.Is(err, capture.ErrCancelled) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// A failed save must not lose the capture, so it is only logged.
	if err := a.settingsService.Set("last_zone", zone); err != nil {
		println("[error] remember last zone:", err.Error())
	}
	return r, nil
}

// CaptureLastZone captures the zone picked last in Capture Zone again, without
// the overlay. It returns an error (shown as a toast) when no zone was picked
// yet, or when its display is gone or no longer matches.
func (a *App) CaptureLastZone() (*model.CaptureResult, error) {
	if err := a.requireScreenRecording(); err != nil {
		return nil, err
	}
	s, err := a.settingsService.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read settings: %w", err)
	}
	if s.LastZone == nil {
		println("[zone] repeat last zone failed: none remembered")
		return nil, errors.New("no zone to repeat yet: capture a zone first")
	}
	r, err := a.captureService.CaptureZoneRect(*s.LastZone)
	if errors.Is(err, capture.ErrZoneUnavailable) {
		err = errors.New("the last zone is no longer on screen (the display changed or was disconnected)")
	}
	if err != nil {
		println("[zone] repeat last zone failed:", err.Error())
		return nil, err
	}
	println("[zone] repeated last zone:", r.Width, "x", r.Height)
	return r, nil
}

// SelectRecordingRegion shows the frozen-screen overlay and returns the region
// the user drags, for recording it. nil means cancelled.
func (a *App) SelectRecordingRegion() (*model.CropRegion, error) {
	if err := a.requireScreenRecording(); err != nil {
		return nil, err
	}
	r, err := a.captureService.SelectRegion()
	if errors.Is(err, capture.ErrCancelled) {
		return nil, nil
	}
	return r, err
}

func (a *App) requireScreenRecording() error {
	ok, err := a.permissionService.CheckScreenRecording()
	if err != nil {
		return fmt.Errorf("failed to check screen recording permission: %w", err)
	}
	if !ok {
		return errors.New("screen recording permission is required. Please grant permission in System Settings > Privacy & Security > Screen Recording")
	}
	return nil
}
