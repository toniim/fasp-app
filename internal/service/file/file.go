package file

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/heytonyne/fasp/internal/model"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(data)
	return encoded, nil
}

// SaveImage saves image data to the specified path
func (s *serviceImpl) SaveImage(options *model.SaveOptions, data []byte) error {
	if options.Path == "" {
		return fmt.Errorf("save path is required")
	}

	// Decode base64 if needed
	var imgData []byte
	if isBase64(data) {
		decoded, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			return fmt.Errorf("failed to decode base64 data: %w", err)
		}
		imgData = decoded
	} else {
		imgData = data
	}

	// Decode image
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
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

	// Encode based on format
	switch options.Format {
	case "png", "":
		err = png.Encode(file, img)
	case "jpeg", "jpg":
		quality := options.Quality
		if quality == 0 {
			quality = 90
		}
		err = jpeg.Encode(file, img, &jpeg.Options{Quality: quality})
	default:
		return fmt.Errorf("unsupported format: %s", options.Format)
	}

	if err != nil {
		return fmt.Errorf("failed to encode image: %w", err)
	}

	return nil
}

// GetDefaultSavePath returns the default save path from settings
func (s *serviceImpl) GetDefaultSavePath() (string, error) {
	// Get user's home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Default to ~/Pictures/Fasp
	defaultPath := filepath.Join(home, "Pictures", "Fasp")
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

// isBase64 checks if data is base64 encoded
func isBase64(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	// Simple heuristic: base64 data is typically longer and contains only valid base64 chars
	if len(data) > 100 && bytes.Contains(data, []byte("data:image")) {
		return true
	}
	// Try to decode a small portion
	_, err := base64.StdEncoding.DecodeString(string(data[:min(100, len(data))]))
	return err == nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
