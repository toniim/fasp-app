//go:build darwin

package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// serviceImpl implements the Service interface for macOS
type serviceImpl struct{}

// New creates a new clipboard service instance
func New() Service {
	return &serviceImpl{}
}

// CopyImage copies image data to clipboard using osascript
func (s *serviceImpl) CopyImage(data []byte) error {
	// Create temp file
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("fasp_clipboard_%d.png", time.Now().UnixNano()))
	defer os.Remove(tmpFile)

	// Write image data to temp file
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Use osascript to copy image to clipboard
	script := fmt.Sprintf(`set the clipboard to (read (POSIX file "%s") as «class PNGf»)`, tmpFile)
	cmd := exec.Command("osascript", "-e", script)
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}

	return nil
}

