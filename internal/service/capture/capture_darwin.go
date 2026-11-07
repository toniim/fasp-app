//go:build darwin

package capture

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework ScreenCaptureKit -framework CoreGraphics -framework ImageIO -framework Foundation -framework UniformTypeIdentifiers
#include <stdlib.h>
#include "capture_darwin.h"
*/
import "C"

import (
	"encoding/base64"
	"errors"
	"fmt"
	"time"
	"unsafe"

	"github.com/heytonyne/fasp/internal/model"
)

// serviceImpl implements the Service interface for macOS
type serviceImpl struct{}

// New creates a new capture service instance
func New() Service {
	return &serviceImpl{}
}

// CaptureFullscreen captures the entire screen of the active display
func (s *serviceImpl) CaptureFullscreen() (*model.CaptureResult, error) {
	return s.CaptureActiveDisplay()
}

// CaptureActiveDisplay captures the display the mouse cursor is on, falling
// back to the main display when it cannot be determined. It uses
// ScreenCaptureKit in-process (see capture_darwin.m): physical pixel size, no
// cursor, no temp file.
func (s *serviceImpl) CaptureActiveDisplay() (*model.CaptureResult, error) {
	var (
		buf           unsafe.Pointer
		size          C.int
		width, height C.int
		cerr          *C.char
	)
	if C.fasp_capture_cursor_display(&buf, &size, &width, &height, &cerr) != 0 {
		msg := "unknown error"
		if cerr != nil {
			msg = C.GoString(cerr)
			C.free(unsafe.Pointer(cerr))
		}
		return nil, fmt.Errorf("failed to capture screenshot: %w", errors.New(msg))
	}
	data := C.GoBytes(buf, size)
	C.free(buf)

	return &model.CaptureResult{
		Data:      base64.StdEncoding.EncodeToString(data),
		Width:     int(width),
		Height:    int(height),
		Timestamp: time.Now(),
	}, nil
}

// CaptureAllDisplays captures all connected displays
func (s *serviceImpl) CaptureAllDisplays() ([]*model.DisplayCapture, error) {
	// For MVP, just capture the main display
	result, err := s.CaptureActiveDisplay()
	if err != nil {
		return nil, err
	}

	// Decode base64 to bytes
	data, err := base64.StdEncoding.DecodeString(result.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	return []*model.DisplayCapture{
		{
			DisplayID: 0,
			Data:      data,
			Width:     result.Width,
			Height:    result.Height,
		},
	}, nil
}

// GetDisplayInfo returns information about all connected displays
func (s *serviceImpl) GetDisplayInfo() ([]*model.DisplayInfo, error) {
	// For MVP, return single display info
	// In future, can parse system_profiler SPDisplaysDataType
	return []*model.DisplayInfo{
		{
			ID:     0,
			Name:   "Main Display",
			Width:  0, // Will be determined on capture
			Height: 0,
			X:      0,
			Y:      0,
		},
	}, nil
}
