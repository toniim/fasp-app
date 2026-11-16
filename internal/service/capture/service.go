package capture

import "github.com/heytonyne/fasp/internal/model"

// Service defines the interface for screenshot capture operations
type Service interface {
	// CaptureFullscreen captures the entire screen of the active display
	CaptureFullscreen() (*model.CaptureResult, error)

	// CaptureActiveDisplay captures the display the mouse cursor is on
	CaptureActiveDisplay() (*model.CaptureResult, error)

	// CaptureAllDisplays captures all connected displays
	CaptureAllDisplays() ([]*model.DisplayCapture, error)

	// GetDisplayInfo returns information about all connected displays
	GetDisplayInfo() ([]*model.DisplayInfo, error)

	// CaptureZone freezes the display under the cursor, lets the user drag a
	// zone on it and returns only that zone. It returns ErrCancelled when the
	// user presses Esc.
	CaptureZone() (*model.CaptureResult, error)

	// SelectRegion shows the same frozen overlay and returns the selected
	// rectangle in desktop coordinates (physical pixels on Windows, points on
	// macOS), e.g. for recording a region. It returns ErrCancelled on Esc.
	SelectRegion() (*model.CropRegion, error)
}
