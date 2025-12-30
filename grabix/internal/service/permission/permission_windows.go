//go:build windows

package permission

type serviceImpl struct{}

// New creates a new permission service instance
func New() Service {
	return &serviceImpl{}
}

// CheckAccessibility checks if the app has Accessibility permissions
// On Windows, always return true as there's no equivalent permission
func (s *serviceImpl) CheckAccessibility() (bool, error) {
	return true, nil
}

// RequestAccessibility requests Accessibility permissions
// On Windows, this is a no-op
func (s *serviceImpl) RequestAccessibility() error {
	return nil
}

