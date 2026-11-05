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
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("fasp_clipboard_%d.png", time.Now().UnixNano()))
	defer os.Remove(tmpFile)

	// Write image data to temp file
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Use PowerShell to copy the image to the clipboard. The file path goes
	// through the environment rather than the script text, so a temp path with
	// quotes can't break the command or inject script code.
	psScript := `
		Add-Type -AssemblyName System.Windows.Forms
		Add-Type -AssemblyName System.Drawing
		$img = [System.Drawing.Image]::FromFile($env:FASP_CLIPBOARD_IN)
		[System.Windows.Forms.Clipboard]::SetImage($img)
		$img.Dispose()
	`

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	cmd.Env = append(os.Environ(), "FASP_CLIPBOARD_IN="+tmpFile)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}

	return nil
}

