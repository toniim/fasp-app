package tray

// Action represents a tray menu action
type Action int

const (
	// ActionCapture triggers screenshot capture
	ActionCapture Action = iota + 1
	// ActionOpenImage opens an image file for annotation
	ActionOpenImage
	// ActionShowWindow shows the main window
	ActionShowWindow
	// ActionHideWindow hides the main window
	ActionHideWindow
	// ActionSettings opens settings window
	ActionSettings
	// ActionQuit quits the application
	ActionQuit
)

// Callback is the function type for tray action callbacks
type Callback func(action Action)

// Manager defines the interface for system tray management
type Manager interface {
	// Setup initializes the system tray with a callback
	Setup(callback Callback) error
	// UpdateIcon updates the tray icon (optional, platform-specific)
	UpdateIcon(iconData []byte) error
	// SetIconFromFile sets the tray icon from a file path (platform-specific)
	SetIconFromFile(iconPath string) error
	// Quit cleans up tray resources
	Quit()
}
