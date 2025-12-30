package hotkey

// Service defines the interface for hotkey management
type Service interface {
	// Register registers a hotkey with a callback
	Register(key string, callback func()) error

	// Unregister removes a hotkey registration
	Unregister(key string) error

	// Start starts listening for hotkeys
	Start() error

	// Stop stops listening for hotkeys
	Stop() error

	// UpdateHotkey updates a hotkey binding
	UpdateHotkey(oldKey, newKey string, callback func()) error
}

