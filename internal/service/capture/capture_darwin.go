//go:build darwin

package capture

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
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

// CaptureActiveDisplay captures the active display (primary display)
func (s *serviceImpl) CaptureActiveDisplay() (*model.CaptureResult, error) {
	// Create temp file
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("fasp_capture_%d.png", time.Now().UnixNano()))
	defer os.Remove(tmpFile)

	// Use macOS screencapture command
	cmd := exec.Command("screencapture", "-x", "-C", tmpFile)
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
