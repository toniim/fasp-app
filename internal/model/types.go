package model

import (
	"runtime"
	"time"
)

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

// ZoneMemory is a zone picked in Capture Zone, kept so "Repeat Last Zone" can
// capture the same rectangle again without the overlay. Every value is in the
// units the capture service works in: points on macOS, physical pixels on
// Windows. The display is stored by its bounds, not by an OS id, because ids
// are not stable across restarts or replugging. The zone can be fractional on
// a Retina display, so these are floats.
type ZoneMemory struct {
	// Display bounds in global desktop coordinates (top-left origin).
	DisplayX      float64 `json:"display_x"`
	DisplayY      float64 `json:"display_y"`
	DisplayWidth  float64 `json:"display_width"`
	DisplayHeight float64 `json:"display_height"`
	// The zone, relative to the top-left corner of that display.
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
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
	DefaultSavePath   string                    `json:"default_save_path"`
	DefaultFormat     string                    `json:"default_format"`
	DefaultQuality    int                       `json:"default_quality"`
	Hotkeys           map[string]string         `json:"hotkeys"`
	UploadProviders   map[string]UploadProvider `json:"upload_providers"`
	ActiveProvider    string                    `json:"active_provider"`
	RunAtStartup      bool                      `json:"run_at_startup"`
	AfterUploadAction string                    `json:"after_upload_action"` // none, direct, site, image
	// ServerURL is the base URL of the fasp server (e.g. https://fasp.me).
	ServerURL string `json:"server_url"`
	// APIKey is the fasp API key (fsk_live_*) used to authorize uploads.
	APIKey string `json:"api_key"`
	// WindowMaximized remembers whether the editor window was maximized so the
	// next time it opens it restores the same state.
	WindowMaximized bool `json:"window_maximized"`
	// WindowWidth/WindowHeight remember the editor window size when it is not
	// maximized (e.g. manually resized), restored on next open.
	WindowWidth  int `json:"window_width"`
	WindowHeight int `json:"window_height"`

	// Recording defaults, pre-selected when a recording starts.
	RecordSystemAudio bool   `json:"record_system_audio"`
	RecordMicrophone  bool   `json:"record_microphone"`
	RecordMicDevice   string `json:"record_mic_device"` // "" = system default
	RecordFPS         int    `json:"record_fps"`

	// OCRLanguage is the language hint for text recognition, a tag from the
	// system OCR engine (for example "en-US"). Empty means automatic.
	OCRLanguage string `json:"ocr_language"`

	// EditorPrefs holds the editor's tool settings (backdrop, highlight, blur
	// mode, ...). The frontend owns its shape: it is kept as written, so a new
	// editor option needs no change here. Nil when never saved.
	EditorPrefs map[string]interface{} `json:"editor_prefs,omitempty"`

	// LastZone is the last zone picked in Capture Zone; nil until one was.
	LastZone *ZoneMemory `json:"last_zone,omitempty"`

	// UploadPublic is the default visibility of an upload. The server only has
	// public or private files; there is no password or expiry. New and old
	// settings files default to true, which is what the app always sent.
	UploadPublic bool `json:"upload_public"`
}

// DefaultSettings returns default application settings
func DefaultSettings() *Settings {
	// Platform-specific default hotkeys.
	// cmd+shift+3/4 are macOS native screenshot shortcuts, so avoid them.
	hotkeys := map[string]string{
		"capture_fullscreen": "cmd+shift+5",
		"capture_region":     "cmd+shift+6",
		"capture_zone":       "cmd+shift+7",
		"record":             "cmd+shift+8",
		"capture_text":       "cmd+shift+9",
	}

	// Use Windows-friendly defaults on Windows
	if runtime.GOOS == "windows" {
		hotkeys = map[string]string{
			"capture_fullscreen": "PrintScreen",
			"capture_region":     "Ctrl+Shift+S",
			"capture_zone":       "Shift+PrintScreen",
			"record":             "Ctrl+PrintScreen",
			"capture_text":       "Ctrl+Shift+9",
		}
	}

	return &Settings{
		DefaultSavePath: "",
		DefaultFormat:   "png",
		DefaultQuality:  90,
		Hotkeys:         hotkeys,
		UploadProviders: map[string]UploadProvider{
			"clipboard": {
				Name:    "Clipboard",
				Enabled: true,
			},
		},
		ActiveProvider:    "clipboard",
		RunAtStartup:      false,
		AfterUploadAction: "direct",
		ServerURL:         "https://api.fasp.me",
		APIKey:            "",
		RecordSystemAudio: true,
		RecordFPS:         30,
		UploadPublic:      true,
	}
}
