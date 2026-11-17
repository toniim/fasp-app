//go:build !darwin && !windows

package ocr

// Recognize is not available on this platform.
func Recognize(png []byte, languages []string) (*Result, error) {
	return nil, ErrUnsupported
}

// SupportedLanguages is not available on this platform.
func SupportedLanguages() ([]string, error) {
	return nil, ErrUnsupported
}
