package clipboard

import "errors"

// ErrNoImage is returned by ReadImage when the clipboard holds no image.
var ErrNoImage = errors.New("no image on the clipboard")

// Service defines the interface for clipboard operations
type Service interface {
	// CopyImage copies image data to clipboard
	CopyImage(data []byte) error
	// ReadImage returns the image on the clipboard as PNG bytes, or
	// ErrNoImage when the clipboard holds none.
	ReadImage() ([]byte, error)
}
