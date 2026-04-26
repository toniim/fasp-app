package file

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/heytonyne/grabix/internal/model"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/image/webp"
)

// allowedImageExtensions limits SaveImage output to known formats
var allowedImageExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
}

// imageMagic is the set of magic byte prefixes we accept as image data
var imageMagic = [][]byte{
	{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, // PNG
	{0xFF, 0xD8, 0xFF},                            // JPEG
	{'G', 'I', 'F', '8'},                          // GIF
	{'B', 'M'},                                    // BMP
}

// serviceImpl implements the Service interface
type serviceImpl struct {
	ctx context.Context
}

// New creates a new file service instance
func New(ctx context.Context) Service {
	return &serviceImpl{
		ctx: ctx,
	}
}

// OpenSaveDialog opens a native save file dialog
func (s *serviceImpl) OpenSaveDialog(defaultName string) (string, error) {
	if defaultName == "" {
		defaultName = s.GenerateFilename("png")
	}

	path, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultName,
		Title:           "Save Screenshot",
		Filters: []runtime.FileFilter{
			{DisplayName: "PNG Images (*.png)", Pattern: "*.png"},
			{DisplayName: "JPEG Images (*.jpg)", Pattern: "*.jpg;*.jpeg"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})

	if err != nil {
		return "", fmt.Errorf("failed to open save dialog: %w", err)
	}

	return path, nil
}

// OpenImageDialog opens a native open file dialog for selecting images
func (s *serviceImpl) OpenImageDialog() (string, error) {
	path, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "Open Image",
		Filters: []runtime.FileFilter{
			{DisplayName: "Image Files", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.bmp;*.webp"},
			{DisplayName: "PNG Images (*.png)", Pattern: "*.png"},
			{DisplayName: "JPEG Images (*.jpg)", Pattern: "*.jpg;*.jpeg"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})

	if err != nil {
		return "", fmt.Errorf("failed to open image dialog: %w", err)
	}

	return path, nil
}

// ReadImageFile reads an image file and returns base64 encoded data
func (s *serviceImpl) ReadImageFile(path string) (string, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %w", err)
	}

	if !isImageBytes(data) {
		return "", errors.New("file is not a recognized image format")
	}

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(data)
	return encoded, nil
}

// SaveImage saves image data to the specified path
func (s *serviceImpl) SaveImage(options *model.SaveOptions, data []byte) error {
	if options.Path == "" {
		return fmt.Errorf("save path is required")
	}

	ext := strings.ToLower(filepath.Ext(options.Path))
	if !allowedImageExtensions[ext] {
		return fmt.Errorf("unsupported file extension: %s", ext)
	}

	// Decode base64 if needed
	imgData, err := decodeImagePayload(data)
	if err != nil {
		return err
	}

	// Decode image (PNG/JPEG via stdlib, WebP via x/image)
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		// Fall back to webp decoder if stdlib could not detect the format
		if w, werr := webp.Decode(bytes.NewReader(imgData)); werr == nil {
			img = w
		} else {
			return fmt.Errorf("failed to decode image: %w", err)
		}
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(options.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(options.Path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	format := options.Format
	if format == "" {
		format = strings.TrimPrefix(ext, ".")
	}

	// Encode based on format
	switch format {
	case "png":
		err = png.Encode(file, img)
	case "jpeg", "jpg":
		quality := options.Quality
		if quality == 0 {
			quality = 90
		}
		err = jpeg.Encode(file, img, &jpeg.Options{Quality: quality})
	case "webp":
		// Go's standard ecosystem only ships a webp decoder. Re-encode as PNG
		// for now and surface a clear message instead of writing a bad file.
		return fmt.Errorf("webp encoding is not supported; choose png or jpeg")
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to encode image: %w", err)
	}

	return nil
}

// decodeImagePayload accepts either raw image bytes, a "data:image/...;base64," URI,
// or plain base64 text and returns raw bytes ready for decoding.
func decodeImagePayload(data []byte) ([]byte, error) {
	if isImageBytes(data) {
		return data, nil
	}

	s := string(data)
	if strings.HasPrefix(s, "data:") {
		if idx := strings.Index(s, ","); idx != -1 {
			s = s[idx+1:]
		}
	}
	s = strings.TrimSpace(s)

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 data: %w", err)
	}
	if !isImageBytes(decoded) {
		return nil, errors.New("decoded payload is not a recognized image format")
	}
	return decoded, nil
}

// GetDefaultSavePath returns the default save path from settings
func (s *serviceImpl) GetDefaultSavePath() (string, error) {
	// Get user's home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Default to ~/Pictures/Grabix
	defaultPath := filepath.Join(home, "Pictures", "Grabix")
	return defaultPath, nil
}

// GenerateFilename generates a filename with timestamp
func (s *serviceImpl) GenerateFilename(format string) string {
	if format == "" {
		format = "png"
	}
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	return fmt.Sprintf("screenshot_%s.%s", timestamp, format)
}

// isImageBytes returns true if data starts with a known image magic header.
func isImageBytes(data []byte) bool {
	for _, magic := range imageMagic {
		if bytes.HasPrefix(data, magic) {
			return true
		}
	}
	// WebP: "RIFF????WEBP"
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return true
	}
	return false
}
