package upload

import "github.com/heytonyne/grabix/internal/model"

// Service defines the interface for upload operations
type Service interface {
	// Upload uploads image data to the configured provider
	Upload(data []byte, filename string) (*model.UploadResult, error)

	// GetProviders returns a list of available upload providers
	GetProviders() []string

	// SetProvider sets the active upload provider
	SetProvider(name string) error

	// GetActiveProvider returns the currently active provider
	GetActiveProvider() string
}
