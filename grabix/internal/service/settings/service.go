package settings

import "github.com/heytonyne/grabix/internal/model"

// Service defines the interface for settings management
type Service interface {
	// Get retrieves a setting value by key
	Get(key string) (interface{}, error)

	// Set updates a setting value
	Set(key string, value interface{}) error

	// Load loads settings from disk
	Load() error

	// Save persists settings to disk
	Save() error

	// GetAll returns all settings
	GetAll() (*model.Settings, error)

	// Reset resets settings to defaults
	Reset() error
}
