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
	r, err := a.captureService.CaptureZone()
	if errors.Is(err, capture.ErrCancelled) {
		return nil, nil
	}
	return r, err
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
