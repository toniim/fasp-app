package upload

import "github.com/heytonyne/grabix/internal/model"

// Service defines the interface for upload operations
type Service interface {
	// Upload uploads image data to the configured provider (legacy)
	Upload(data []byte, filename string) (*model.UploadResult, error)

	// Init initiates a file upload and returns upload URL
	Init(filename string, size int64, contentType string) (*InitResponse, error)

	// Complete completes a file upload and returns public URLs
	Complete(fileID string) (*CompleteResponse, error)

	// IsConfigured checks if upload service is properly configured
	IsConfigured() bool

	// GetProviders returns a list of available upload providers
	GetProviders() []string

	// SetProvider sets the active upload provider
	SetProvider(name string) error

	// GetActiveProvider returns the currently active provider
	GetActiveProvider() string
}
