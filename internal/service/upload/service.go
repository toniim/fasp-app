package upload

import "github.com/heytonyne/fasp/internal/model"

// Service defines the interface for upload operations
type Service interface {
	// Upload uploads image data using the full 3-step flow (init -> PUT -> complete)
	Upload(data []byte, filename string) (*model.UploadResult, error)

	// Init initiates a file upload and returns a presigned upload URL.
	// visibility is VisibilityPublic, VisibilityPrivate, or empty to use the
	// default from the settings.
	Init(filename string, size int64, contentType, visibility string) (*InitResponse, error)

	// Complete completes a file upload and returns public URLs
	Complete(fileID string) (*CompleteResponse, error)

	// IsConfigured reports whether a server URL and API key are set
	IsConfigured() bool

	// TestConnection verifies the configured server URL + API key are valid
	TestConnection() error
}
