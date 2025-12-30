package file

import "github.com/heytonyne/grabix/internal/model"

// Service defines the interface for file operations
type Service interface {
	// OpenSaveDialog opens a native save file dialog
	OpenSaveDialog(defaultName string) (string, error)

	// SaveImage saves image data to the specified path
	SaveImage(options *model.SaveOptions, data []byte) error

	// GetDefaultSavePath returns the default save path from settings
	GetDefaultSavePath() (string, error)

	// GenerateFilename generates a filename with timestamp
	GenerateFilename(format string) string
}
