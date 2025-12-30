package permission

// Service defines the interface for permission checking
type Service interface {
	// CheckAccessibility checks if the app has Accessibility permissions
	CheckAccessibility() (bool, error)
	
	// RequestAccessibility requests Accessibility permissions (opens System Settings)
	RequestAccessibility() error
}

