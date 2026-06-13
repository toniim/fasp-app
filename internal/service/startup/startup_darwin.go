//go:build darwin

package startup

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// serviceImpl implements the Service interface for macOS
type serviceImpl struct {
	appName string
	appPath string
}

// New creates a new startup service instance
func New() Service {
	// Get the executable path
	execPath, err := os.Executable()
	if err != nil {
		execPath = ""
	}

	// If running from .app bundle, get the bundle path
	appPath := execPath
	if strings.Contains(execPath, ".app/Contents/MacOS/") {
		// Extract .app path
		parts := strings.Split(execPath, ".app/Contents/MacOS/")
		if len(parts) > 0 {
			appPath = parts[0] + ".app"
		}
	}

	return &serviceImpl{
		appName: "Fasp",
		appPath: appPath,
	}
}

// Enable enables the app to run at startup using osascript
func (s *serviceImpl) Enable() error {
	if s.appPath == "" {
		return fmt.Errorf("app path not found")
	}

	// Use osascript to add login item
	script := fmt.Sprintf(`
		tell application "System Events"
			make login item at end with properties {path:"%s", hidden:false}
		end tell
	`, s.appPath)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to enable startup: %w (output: %s)", err, string(output))
	}

	return nil
}

// Disable disables the app from running at startup
func (s *serviceImpl) Disable() error {
	// Use osascript to remove login item
	script := fmt.Sprintf(`
		tell application "System Events"
			delete login item "%s"
		end tell
	`, s.appName)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore error if item doesn't exist
		if !strings.Contains(string(output), "Can't get login item") {
			return fmt.Errorf("failed to disable startup: %w (output: %s)", err, string(output))
		}
	}

	return nil
}

// IsEnabled checks if the app is set to run at startup
func (s *serviceImpl) IsEnabled() (bool, error) {
	// Use osascript to check if login item exists
	script := `
		tell application "System Events"
			get the name of every login item
		end tell
	`

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to check startup status: %w", err)
	}

	// Check if app name is in the list
	items := strings.Split(string(output), ", ")
	for _, item := range items {
		if strings.TrimSpace(item) == s.appName {
			return true, nil
		}
	}

	return false, nil
}
