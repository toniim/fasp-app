package clipboard

// Service defines the interface for clipboard operations
type Service interface {
	// CopyImage copies image data to clipboard
	CopyImage(data []byte) error

	// CopyFiles puts files on the clipboard the way a file manager does, so
	// they can be pasted into chat apps, mail or a folder.
	CopyFiles(paths []string) error
}
