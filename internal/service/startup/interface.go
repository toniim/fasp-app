package startup

// Service defines the interface for managing app startup behavior
type Service interface {
	// Enable enables the app to run at startup
	Enable() error

	// Disable disables the app from running at startup
	Disable() error

	// IsEnabled checks if the app is set to run at startup
	IsEnabled() (bool, error)
}

