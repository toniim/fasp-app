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
	println("[DEBUG] Creating new settings service...")
	s := &serviceImpl{
		settings: model.DefaultSettings(),
	}

	// Get config file path
	configDir, err := os.UserConfigDir()
	if err != nil {
		println("[ERROR] Failed to get user config dir:", err.Error())
		configDir = os.TempDir()
		println("[WARN] Using temp dir:", configDir)
	} else {
		println("[INFO] User config dir:", configDir)
	}

	s.filePath = filepath.Join(configDir, "grabix", "settings.json")
	println("[INFO] Settings file path:", s.filePath)

	// Try to load existing settings
	if err := s.Load(); err != nil {
		println("[WARN] Failed to load settings, using defaults:", err.Error())
	} else {
		println("[INFO] Settings loaded successfully")
		println("[DEBUG] DefaultSavePath after load:", s.settings.DefaultSavePath)
	}

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
			println("[INFO] Settings file does not exist, using defaults")
			return nil
		}
		return fmt.Errorf("failed to read settings file: %w", err)
	}

	println("[DEBUG] Settings file content:", string(data))

	if err := json.Unmarshal(data, s.settings); err != nil {
		return fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	// Validate and fix empty default_save_path from old settings files
	if s.settings.DefaultSavePath == "" {
		println("[WARN] Loaded settings has empty default_save_path")
		println("[REASON] Old settings file with empty path detected")

		// Get user's home directory for default save path
		home, err := os.UserHomeDir()
		if err != nil {
			s.settings.DefaultSavePath = "/Users/Shared/Grabix"
			println("[ERROR] Failed to get home dir:", err.Error())
			println("[FIX] Setting default_save_path to fallback:", s.settings.DefaultSavePath)
		} else {
			s.settings.DefaultSavePath = filepath.Join(home, "Pictures", "Grabix")
			println("[FIX] Setting default_save_path to:", s.settings.DefaultSavePath)
		}

		// Save updated settings
		if err := s.Save(); err != nil {
			println("[ERROR] Failed to save updated settings:", err.Error())
		} else {
			println("[INFO] Settings file updated with new default_save_path")
		}
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
