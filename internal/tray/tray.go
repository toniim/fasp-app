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
	// ActionOpenRecent opens a recently saved screenshot (arg = file path)
	ActionOpenRecent
	// ActionOpenRecentsFolder opens the folder holding recent saves (arg = folder)
	ActionOpenRecentsFolder
	// ActionCaptureZone freezes the screen to capture a dragged zone
	ActionCaptureZone
	// ActionRecord opens the record dialog, or stops the running recording
	ActionRecord
	// ActionCaptureDelayed captures the screen after a countdown (arg = seconds)
	ActionCaptureDelayed
	// ActionCaptureZoneDelayed freezes the screen for a zone after a countdown
	// (arg = seconds)
	ActionCaptureZoneDelayed
	// ActionRepeatZone captures the last zone again without the overlay
	ActionRepeatZone
	// ActionCancelCountdown cancels a running capture countdown
	ActionCancelCountdown
	// ActionCaptureText freezes the screen, reads the text in a zone and copies it
	ActionCaptureText
)

// DelayChoices are the delays, in seconds, the tray offers for delayed capture.
var DelayChoices = []int{3, 5, 10}

// Callback is the function type for tray action callbacks. arg carries the
// payload for actions that need one (recent file path, folder) and is empty
// for the rest.
type Callback func(action Action, arg string)

// Manager defines the interface for system tray management
type Manager interface {
	// Setup initializes the system tray with a callback
	Setup(callback Callback) error
	// UpdateIcon updates the tray icon (optional, platform-specific)
	UpdateIcon(iconData []byte) error
	// SetIconFromFile sets the tray icon from a file path (platform-specific)
	SetIconFromFile(iconPath string) error
	// SetRecents refreshes the "Recents" submenu, most recent path first
	SetRecents(paths []string) error
	// SetCountdown shows the seconds left of a capture countdown on the tray
	// (menu bar title on macOS, tooltip on Windows) and offers "Cancel
	// Countdown" in the menu. 0 clears both. It is not part of any picture the
	// capture takes: the caller clears it before capturing.
	SetCountdown(remaining int)
	// SetRecording names the record item after what a click does now:
	// "Start Recording…" when idle, "Stop Recording" while recording.
	SetRecording(recording bool)
	// Quit cleans up tray resources
	Quit()
}
