package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/heytonyne/fasp/internal/model"
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

	s.filePath = filepath.Join(configDir, "fasp", "settings.json")

	// Try to load existing settings
	_ = s.Load()

	// Backfill / migrate the server URL. "https://fasp.me" is the static SPA
	// host (serves index.html for /api/*), never the API — rewrite it to the
	// real API host. Also covers settings files written before this field.
	if s.settings.ServerURL == "" || s.settings.ServerURL == "https://fasp.me" {
		s.settings.ServerURL = model.DefaultSettings().ServerURL
	}

	return s
}

// toInt coerces a setting value (int from Go, float64 from JSON) to int.
func toInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
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
	case "after_upload_action":
		return s.settings.AfterUploadAction, nil
	case "server_url":
		return s.settings.ServerURL, nil
	case "api_key":
		return s.settings.APIKey, nil
	case "window_maximized":
		return s.settings.WindowMaximized, nil
	case "window_width":
		return s.settings.WindowWidth, nil
	case "window_height":
		return s.settings.WindowHeight, nil
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
	case "server_url":
		if v, ok := value.(string); ok {
			s.settings.ServerURL = v
		} else {
			return fmt.Errorf("invalid type for server_url")
		}
	case "api_key":
		if v, ok := value.(string); ok {
			s.settings.APIKey = v
		} else {
			return fmt.Errorf("invalid type for api_key")
		}
	case "window_maximized":
		if v, ok := value.(bool); ok {
			s.settings.WindowMaximized = v
		} else {
			return fmt.Errorf("invalid type for window_maximized")
		}
	case "window_width":
		if v, ok := toInt(value); ok {
			s.settings.WindowWidth = v
		} else {
			return fmt.Errorf("invalid type for window_width")
		}
	case "window_height":
		if v, ok := toInt(value); ok {
			s.settings.WindowHeight = v
		} else {
			return fmt.Errorf("invalid type for window_height")
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

	// 0600: settings.json now holds the fasp API key (a secret).
	if err := os.WriteFile(s.filePath, data, 0600); err != nil {
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
