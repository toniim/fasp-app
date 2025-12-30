//go:build windows

package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// serviceImpl implements the Service interface for Windows
type serviceImpl struct{}

// New creates a new clipboard service instance
func New() Service {
	return &serviceImpl{}
}

// CopyImage copies image data to clipboard using PowerShell
func (s *serviceImpl) CopyImage(data []byte) error {
	// Create temp file
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("grabix_clipboard_%d.png", time.Now().UnixNano()))
	defer os.Remove(tmpFile)

	// Write image data to temp file
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Use PowerShell to copy image to clipboard
	psScript := fmt.Sprintf(`
		Add-Type -AssemblyName System.Windows.Forms
		Add-Type -AssemblyName System.Drawing
		$img = [System.Drawing.Image]::FromFile('%s')
		[System.Windows.Forms.Clipboard]::SetImage($img)
		$img.Dispose()
	`, tmpFile)

	cmd := exec.Command("powershell", "-Command", psScript)
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}

	return nil
}

