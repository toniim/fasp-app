//go:build darwin

package capture

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreGraphics/CoreGraphics.h>

// cursorDisplayIndex returns the 1-based position of the display under the
// mouse cursor in the active display list, the same numbering `screencapture -D`
// uses (1 is the main display). It returns 0 when the display cannot be found.
static int cursorDisplayIndex(void) {
    CGEventRef ev = CGEventCreate(NULL);
    if (ev == NULL) return 0;
    CGPoint p = CGEventGetLocation(ev);
    CFRelease(ev);

    CGDirectDisplayID under;
    uint32_t found = 0;
    if (CGGetDisplaysWithPoint(p, 1, &under, &found) != kCGErrorSuccess || found == 0) return 0;

    CGDirectDisplayID list[32];
    uint32_t count = 0;
    if (CGGetActiveDisplayList(32, list, &count) != kCGErrorSuccess) return 0;
    for (uint32_t i = 0; i < count; i++) {
        if (list[i] == under) return (int)i + 1;
    }
    return 0;
}
*/
import "C"

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

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
// back to the main display when it cannot be determined.
func (s *serviceImpl) CaptureActiveDisplay() (*model.CaptureResult, error) {
	// Create temp file
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("fasp_capture_%d.png", time.Now().UnixNano()))
	defer os.Remove(tmpFile)

	// Use macOS screencapture command. -x = silent, no shutter sound. The
	// cursor is intentionally not captured (-C omitted) so it is never baked
	// into the screenshot. Without -D, screencapture writes only the main
	// display when given a single output file.
	args := []string{"-x", "-t", "png"}
	if idx := int(C.cursorDisplayIndex()); idx > 0 {
		args = append(args, "-D", strconv.Itoa(idx))
	}
	cmd := exec.Command("screencapture", append(args, tmpFile)...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to capture screenshot: %w", err)
	}

	// Read the captured image
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read screenshot: %w", err)
	}

	// Decode to get dimensions
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()

	// Convert to base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	return &model.CaptureResult{
		Data:      base64Data,
		Width:     bounds.Dx(),
		Height:    bounds.Dy(),
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
