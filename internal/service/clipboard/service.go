package clipboard

// Service defines the interface for clipboard operations
type Service interface {
	// CopyImage copies image data to clipboard
	CopyImage(data []byte) error
}

