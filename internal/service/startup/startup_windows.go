//go:build windows

package startup

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const registryKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
const appName = "Fasp"

// serviceImpl implements the Service interface for Windows
type serviceImpl struct {
	appPath string
}

// New creates a new startup service instance
func New() Service {
	execPath, err := os.Executable()
	if err != nil {
		execPath = ""
	}

	return &serviceImpl{
		appPath: execPath,
	}
}

// Enable enables the app to run at startup using Windows registry
func (s *serviceImpl) Enable() error {
	if s.appPath == "" {
		return fmt.Errorf("app path not found")
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	// Use quoted path in case of spaces
	value := fmt.Sprintf(`"%s"`, filepath.Clean(s.appPath))
	if err := key.SetStringValue(appName, value); err != nil {
		return fmt.Errorf("failed to set registry value: %w", err)
	}

	return nil
}

// Disable disables the app from running at startup
func (s *serviceImpl) Disable() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.SET_VALUE)
	if err != nil {
		// Key doesn't exist, nothing to disable
		return nil
	}
	defer key.Close()

	if err := key.DeleteValue(appName); err != nil {
		// Value doesn't exist, that's fine
		if err == registry.ErrNotExist {
			return nil
		}
		return fmt.Errorf("failed to delete registry value: %w", err)
	}

	return nil
}

// IsEnabled checks if the app is set to run at startup
func (s *serviceImpl) IsEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.QUERY_VALUE)
	if err != nil {
		// Key doesn't exist
		return false, nil
	}
	defer key.Close()

	_, _, err = key.GetStringValue(appName)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("failed to read registry value: %w", err)
	}

	return true, nil
}
