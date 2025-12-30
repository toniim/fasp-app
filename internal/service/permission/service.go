package permission

// Service defines the interface for permission checking
type Service interface {
	// CheckAccessibility checks if the app has Accessibility permissions
	CheckAccessibility() (bool, error)

	// RequestAccessibility requests Accessibility permissions (opens System Settings)
	RequestAccessibility() error

	// CheckScreenRecording checks if the app has Screen Recording permissions
	CheckScreenRecording() (bool, error)

	// RequestScreenRecording requests Screen Recording permissions (opens System Settings)
	RequestScreenRecording() error
}
