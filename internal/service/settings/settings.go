package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/heytonyne/grabix/internal/model"
)

// serviceImpl implements the Service interface
type serviceImpl struct {
	mu       sync.RWMutex
	settings *model.Settings
	filePath string
}

// New creates a new settings service instance
func New() Service {
	s := &serviceImpl{
		settings: model.DefaultSettings(),
	}

	// Get config file path
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}

	s.filePath = filepath.Join(configDir, "grabix", "settings.json")

	// Try to load existing settings
	_ = s.Load()

	return s
}

// Get retrieves a setting value by key
func (s *serviceImpl) Get(key string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch key {
	case "default_save_path":
		return s.settings.DefaultSavePath, nil
	case "default_format":
		return s.settings.DefaultFormat, nil
	case "default_quality":
		return s.settings.DefaultQuality, nil
	case "hotkeys":
		return s.settings.Hotkeys, nil
	case "upload_providers":
		return s.settings.UploadProviders, nil
	case "active_provider":
		return s.settings.ActiveProvider, nil
	case "run_at_startup":
		return s.settings.RunAtStartup, nil
	default:
		return nil, fmt.Errorf("unknown setting key: %s", key)
	}
}

// Set updates a setting value
func (s *serviceImpl) Set(key string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Handle nested keys like "hotkeys.capture_fullscreen"
	if len(key) > 8 && key[:8] == "hotkeys." {
		hotkeyName := key[8:]
		if v, ok := value.(string); ok {
			if s.settings.Hotkeys == nil {
				s.settings.Hotkeys = make(map[string]string)
			}
			s.settings.Hotkeys[hotkeyName] = v
			return s.Save()
		}
		return fmt.Errorf("invalid type for hotkey value")
	}

	switch key {
	case "default_save_path":
		if v, ok := value.(string); ok {
			s.settings.DefaultSavePath = v
		} else {
			return fmt.Errorf("invalid type for default_save_path")
		}
	case "default_format":
		if v, ok := value.(string); ok {
			s.settings.DefaultFormat = v
		} else {
			return fmt.Errorf("invalid type for default_format")
		}
	case "default_quality":
		if v, ok := value.(int); ok {
			s.settings.DefaultQuality = v
		} else if v, ok := value.(float64); ok {
			s.settings.DefaultQuality = int(v)
		} else {
			return fmt.Errorf("invalid type for default_quality")
		}
	case "active_provider":
		if v, ok := value.(string); ok {
			s.settings.ActiveProvider = v
		} else {
			return fmt.Errorf("invalid type for active_provider")
		}
	case "run_at_startup":
		if v, ok := value.(bool); ok {
			s.settings.RunAtStartup = v
		} else {
			return fmt.Errorf("invalid type for run_at_startup")
		}
	case "after_upload_action":
		if v, ok := value.(string); ok {
			s.settings.AfterUploadAction = v
		} else {
			return fmt.Errorf("invalid type for after_upload_action")
		}
	default:
		return fmt.Errorf("unknown setting key: %s", key)
	}

	return s.Save()
}

// Load loads settings from disk
func (s *serviceImpl) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, use defaults
			return nil
		}
		return fmt.Errorf("failed to read settings file: %w", err)
	}

	if err := json.Unmarshal(data, s.settings); err != nil {
		return fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	return nil
}

// Save persists settings to disk
func (s *serviceImpl) Save() error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

// GetAll returns all settings
func (s *serviceImpl) GetAll() (*model.Settings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modifications
	settingsCopy := *s.settings
	return &settingsCopy, nil
}

// Reset resets settings to defaults
func (s *serviceImpl) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.settings = model.DefaultSettings()
	return s.Save()
}
