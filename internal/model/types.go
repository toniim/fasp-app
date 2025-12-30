package model

import "time"

// DisplayInfo contains information about a display/monitor
type DisplayInfo struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
}

// DisplayCapture represents a captured screenshot from a display
type DisplayCapture struct {
	DisplayID int    `json:"display_id"`
	Data      []byte `json:"data"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// CaptureResult represents the result of a screenshot capture
type CaptureResult struct {
	Data      string    `json:"data"` // Base64 encoded image
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Timestamp time.Time `json:"timestamp"`
}

// CropRegion represents a rectangular region to crop
type CropRegion struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// SaveOptions contains options for saving an image
type SaveOptions struct {
	Path    string `json:"path"`
	Format  string `json:"format"`  // png, jpeg, webp
	Quality int    `json:"quality"` // 1-100 for jpeg
}

// UploadResult represents the result of an upload
type UploadResult struct {
	URL       string    `json:"url"`
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
}

// UploadProvider represents an upload provider configuration
type UploadProvider struct {
	Name     string            `json:"name"`
	Enabled  bool              `json:"enabled"`
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers"`
}

// Settings represents application settings
type Settings struct {
	DefaultSavePath string                    `json:"default_save_path"`
	DefaultFormat   string                    `json:"default_format"`
	DefaultQuality  int                       `json:"default_quality"`
	Hotkeys         map[string]string         `json:"hotkeys"`
	UploadProviders map[string]UploadProvider `json:"upload_providers"`
	ActiveProvider  string                    `json:"active_provider"`
	RunAtStartup    bool                      `json:"run_at_startup"`
}

// DefaultSettings returns default application settings
func DefaultSettings() *Settings {
	return &Settings{
		DefaultSavePath: "",
		DefaultFormat:   "png",
		DefaultQuality:  90,
		Hotkeys: map[string]string{
			"capture_fullscreen": "cmd+shift+3",
			"capture_region":     "cmd+shift+4",
		},
		UploadProviders: map[string]UploadProvider{
			"clipboard": {
				Name:    "Clipboard",
				Enabled: true,
			},
		},
		ActiveProvider: "clipboard",
		RunAtStartup:   false,
	}
}
