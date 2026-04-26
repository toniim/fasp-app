//go:build windows

package capture

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/heytonyne/grabix/internal/model"
)

// serviceImpl implements the Service interface for Windows
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
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("grabix_capture_%d.png", time.Now().UnixNano()))
	defer os.Remove(tmpFile)

	// Use PowerShell to capture screenshot. Pass the destination path via
	// environment variable so it is never interpolated as PowerShell source —
	// this avoids script-injection if the temp directory contains quote chars.
	psScript := `
		Add-Type -AssemblyName System.Windows.Forms
		Add-Type -AssemblyName System.Drawing
		$screen = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
		$bitmap = New-Object System.Drawing.Bitmap $screen.Width, $screen.Height
		$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
		$graphics.CopyFromScreen($screen.Location, [System.Drawing.Point]::Empty, $screen.Size)
		$bitmap.Save($env:GRABIX_OUT, [System.Drawing.Imaging.ImageFormat]::Png)
		$graphics.Dispose()
		$bitmap.Dispose()
	`

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	cmd.Env = append(os.Environ(), "GRABIX_OUT="+tmpFile)
	// Hide the PowerShell window
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
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
	return []*model.DisplayInfo{
		{
			ID:     0,
			Name:   "Primary Display",
			Width:  0, // Will be determined on capture
			Height: 0,
			X:      0,
			Y:      0,
		},
	}, nil
}
