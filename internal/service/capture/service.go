package capture

import "github.com/heytonyne/fasp/internal/model"

// Service defines the interface for screenshot capture operations
type Service interface {
	// CaptureFullscreen captures the entire screen of the active display
	CaptureFullscreen() (*model.CaptureResult, error)

	// CaptureActiveDisplay captures the active display
	CaptureActiveDisplay() (*model.CaptureResult, error)

	// CaptureAllDisplays captures all connected displays
	CaptureAllDisplays() ([]*model.DisplayCapture, error)

	// GetDisplayInfo returns information about all connected displays
	GetDisplayInfo() ([]*model.DisplayInfo, error)
}
