package main

import (
	"context"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/heytonyne/fasp/internal/model"
	"github.com/heytonyne/fasp/internal/service/capture"
	"github.com/heytonyne/fasp/internal/service/clipboard"
	"github.com/heytonyne/fasp/internal/service/file"
	"github.com/heytonyne/fasp/internal/service/hotkey"
	"github.com/heytonyne/fasp/internal/service/permission"
	"github.com/heytonyne/fasp/internal/service/recents"
	"github.com/heytonyne/fasp/internal/service/settings"
	"github.com/heytonyne/fasp/internal/service/shell"
	"github.com/heytonyne/fasp/internal/service/startup"
	"github.com/heytonyne/fasp/internal/service/upload"
	"github.com/heytonyne/fasp/internal/tray"
	"github.com/heytonyne/fasp/internal/version"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/trayicon.png
var trayIconData []byte

//go:embed build/windows/trayicon.ico
var trayIconDataWindows []byte

// App is the Wails service whose exported methods the frontend calls. Its
// bindings are generated into frontend/bindings.
type App struct {
	app *application.App
	// window is the one window the app owns. It is created before the app runs,
	// so every method here can use it.
	window            *application.WebviewWindow
	captureService    capture.Service
	fileService       file.Service
	settingsService   settings.Service
	hotkeyService     hotkey.Service
	permissionService permission.Service
	clipboardService  clipboard.Service
	startupService    startup.Service
	uploadService     upload.Service
	recentsService    recents.Service
	trayManager       tray.Manager
	quitting          atomic.Bool

	recording recordingState
	delayed   delayedCapture
	media     *mediaServer
	// recordingBar is set while the window is the small recording control
	// bar, so its size is not saved as the editor size.
	recordingBar atomic.Bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{media: newMediaServer()}
}

// emit sends an event to the frontend.
func (a *App) emit(name string, data ...any) {
	a.app.Event.Emit(name, data...)
}

// ServiceStartup runs when the application starts (Wails calls it; the
// generator does not bind it).
func (a *App) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	// Initialize services
	a.captureService = capture.New()
	a.fileService = file.New()
	a.settingsService = settings.New()
	a.hotkeyService = hotkey.New()
	a.permissionService = permission.New()
	a.clipboardService = clipboard.New()
	a.startupService = startup.New()
	a.uploadService = upload.New(a.settingsService)
	a.recentsService = recents.New()
	return nil
}

// onApplicationStarted sets up what needs the native run loop to be running:
// the global hotkeys and the tray icon.
func (a *App) onApplicationStarted(_ *application.ApplicationEvent) {
	a.setupHotkeys()
	a.setupTray()
}

// setupTray initializes the system tray icon
func (a *App) setupTray() {
	println("[DEBUG] setupTray called")
	a.trayManager = tray.NewManager()
	println("[DEBUG] tray.NewManager() returned:", a.trayManager)

	err := a.trayManager.Setup(func(action tray.Action, arg string) {
		println("[DEBUG] Tray action received:", action, arg)
		switch action {
		case tray.ActionCapture:
			// Trigger screenshot capture
			a.emit("hotkey:capture")

		case tray.ActionCaptureZone:
			a.emit("hotkey:capture_zone")

		case tray.ActionRecord:
			a.toggleRecording()

		case tray.ActionCaptureDelayed:
			if n, ok := delayFromArg(arg); ok {
				a.startDelayed("hotkey:capture", n)
			}

		case tray.ActionCaptureZoneDelayed:
			if n, ok := delayFromArg(arg); ok {
				a.startDelayed("hotkey:capture_zone", n)
			}

		case tray.ActionRepeatZone:
			a.emit("hotkey:repeat_zone")

		case tray.ActionCancelCountdown:
			a.cancelCountdown()

		case tray.ActionOpenImage:
			// Trigger open image dialog
			a.emit("open:image")

		case tray.ActionShowWindow:
			// Show main window (restore last maximized/normal state)
			a.ShowWindow()

		case tray.ActionHideWindow:
			// Hide main window (save state first)
			a.HideWindow()

		case tray.ActionSettings:
			// Open settings window
			a.emit("open:settings")

		case tray.ActionOpenRecent:
			if isVideo(arg) {
				a.openVideo(arg) // play it in the editor, like screenshots
				break
			}
			if err := shell.Open(arg); err != nil {
				println("[error] open recent:", err.Error())
			}

		case tray.ActionOpenRecentsFolder:
			folder := arg
			if folder == "" && a.recentsService != nil {
				// macOS sends no argument: fall back to the newest saved file.
				folder, _ = a.recentsService.Folder()
			}
			if folder == "" {
				folder, _ = a.fileService.GetDefaultSavePath()
			}
			if folder == "" {
				println("[error] open recents folder: no folder to open")
				break
			}
			if err := shell.Open(folder); err != nil {
				println("[error] open recents folder:", err.Error())
			}

		case tray.ActionQuit:
			// Quit application
			a.quitApp()
		}
	})

	if err != nil {
		println("[ERROR] Failed to setup tray:", err.Error())
	} else {
		println("[DEBUG] Tray setup completed successfully")

		// Push the stored recents as soon as the menu exists.
		a.refreshTrayRecents()

		// Set tray icon - delay to ensure status item is created first
		go func() {
			// Wait for tray to be initialized
			time.Sleep(500 * time.Millisecond)

			// Use platform-specific icon data and extension
			var iconData []byte
			var iconExt string

			if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") || filepath.Separator == '\\' {
				// Windows: use ICO format
				iconData = trayIconDataWindows
				iconExt = ".ico"
				println("[DEBUG] Using Windows ICO icon, size:", len(iconData), "bytes")
			} else {
				// macOS/Linux: use PNG format
				iconData = trayIconData
				iconExt = ".png"
				println("[DEBUG] Using PNG icon, size:", len(iconData), "bytes")
			}

			// Write icon to temp file
			tmpDir := os.TempDir()
			iconPath := filepath.Join(tmpDir, "fasp-trayicon"+iconExt)

			writeErr := os.WriteFile(iconPath, iconData, 0644)
			if writeErr != nil {
				println("[ERROR] Failed to write tray icon:", writeErr.Error())
				return
			}
			println("[DEBUG] Tray icon written to:", iconPath)

			// Set the icon
			if err := a.trayManager.SetIconFromFile(iconPath); err != nil {
				println("[ERROR] Failed to set tray icon:", err.Error())
			} else {
				println("[DEBUG] Tray icon set successfully")
			}
		}()
	}
}

// shutdown runs when the app is shutting down.
func (a *App) shutdown() {
	// A quit with the window open (Cmd+Q, logout) must remember its size too.
	a.saveWindowState()

	// Cleanup tray
	if a.trayManager != nil {
		a.trayManager.Quit()
	}

	// Stop hotkey service
	if a.hotkeyService != nil {
		a.hotkeyService.Stop()
	}
}

// buildMenu creates the application menu
func (a *App) buildMenu() *application.Menu {
	appMenu := application.NewMenu()

	// File menu
	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.Add("Capture Screenshot").OnClick(func(_ *application.Context) {
		a.emit("hotkey:capture")
	})
	fileMenu.Add("Capture Zone").OnClick(func(_ *application.Context) {
		a.emit("hotkey:capture_zone")
	})
	fileMenu.Add("Record Screen...").OnClick(func(_ *application.Context) {
		a.toggleRecording()
	})
	fileMenu.Add("Open Image...").SetAccelerator("CmdOrCtrl+O").OnClick(func(_ *application.Context) {
		a.emit("open:image")
	})
	fileMenu.AddSeparator()
	fileMenu.Add("Settings...").SetAccelerator("CmdOrCtrl+,").OnClick(func(_ *application.Context) {
		a.emit("open:settings")
	})
	fileMenu.AddSeparator()
	fileMenu.Add("Quit").SetAccelerator("CmdOrCtrl+Q").OnClick(func(_ *application.Context) {
		a.quitApp()
	})

	// On macOS, Cmd+C/V/X/A/Z reach a text field only through the Edit menu's
	// actions; without it, fields such as the Settings URL and token cannot be
	// pasted into.
	if goruntime.GOOS == "darwin" {
		appMenu.AddRole(application.EditMenu)
	}

	// Window menu
	windowMenu := appMenu.AddSubmenu("Window")
	windowMenu.Add("Show Window").OnClick(func(_ *application.Context) {
		a.window.Show()
		a.window.UnMinimise()
	})
	windowMenu.Add("Hide Window").OnClick(func(_ *application.Context) {
		a.window.Hide()
	})

	return appMenu
}

// setupHotkeys registers hotkeys from settings
func (a *App) setupHotkeys() {
	settings, err := a.settingsService.GetAll()
	if err != nil {
		println("Failed to get settings:", err.Error())
		return
	}

	// Stop existing hotkey service if running
	a.hotkeyService.Stop()

	// Register every configured capture hotkey BEFORE starting the listener.
	// capture_region used to be ignored here, so the setting did nothing.
	registered := 0
	registerAction := func(settingKey string, action func()) {
		combo, ok := settings.Hotkeys[settingKey]
		if !ok || combo == "" {
			return
		}

		println("Setting up hotkey:", settingKey, "=", combo)
		if err := a.hotkeyService.Register(combo, func() {
			println("🔥 Hotkey pressed:", settingKey)
			action()
		}); err != nil {
			println("Failed to register hotkey", settingKey, ":", err.Error())
			return
		}
		registered++
	}

	registerHotkey := func(settingKey, eventName string) {
		registerAction(settingKey, func() { a.emit(eventName) })
	}
	registerHotkey("capture_fullscreen", "hotkey:capture")
	registerHotkey("capture_region", "hotkey:capture_region")
	registerHotkey("capture_zone", "hotkey:capture_zone")
	registerHotkey("repeat_zone", "hotkey:repeat_zone")
	registerAction("record", a.toggleRecording)

	if registered == 0 {
		println("No capture hotkey configured")
		return
	}

	// Start listening AFTER registering all hotkeys
	err = a.hotkeyService.Start()
	if err != nil {
		println("Failed to start hotkey service:", err.Error())
	}
}

// CaptureFullscreen captures the entire screen
func (a *App) CaptureFullscreen() (*model.CaptureResult, error) {
	return a.captureService.CaptureFullscreen()
}

// CaptureActiveDisplay captures the active display
func (a *App) CaptureActiveDisplay() (*model.CaptureResult, error) {
	// Check Screen Recording permission first
	hasPermission, err := a.permissionService.CheckScreenRecording()
	if err != nil {
		return nil, fmt.Errorf("failed to check screen recording permission: %w", err)
	}

	if !hasPermission {
		return nil, fmt.Errorf("screen recording permission is required. Please grant permission in System Settings > Privacy & Security > Screen Recording")
	}

	r, err := a.captureService.CaptureActiveDisplay()
	if err == nil {
		println("[capture] active display captured:", r.Width, "x", r.Height)
	}
	return r, err
}

// GetDisplayInfo returns information about all displays
func (a *App) GetDisplayInfo() ([]*model.DisplayInfo, error) {
	return a.captureService.GetDisplayInfo()
}

// OpenSaveDialog opens a save file dialog
func (a *App) OpenSaveDialog(defaultName string) (string, error) {
	return a.fileService.OpenSaveDialog(defaultName)
}

// OpenImageDialog opens an open file dialog for selecting images
func (a *App) OpenImageDialog() (string, error) {
	return a.fileService.OpenImageDialog()
}

// ReadImageFile reads an image file and returns base64 encoded data
func (a *App) ReadImageFile(path string) (string, error) {
	return a.fileService.ReadImageFile(path)
}

// SaveImage saves an image to disk
func (a *App) SaveImage(options *model.SaveOptions, data []byte) error {
	if err := a.fileService.SaveImage(options, data); err != nil {
		println("[error] SaveImage:", err.Error())
		return err
	}
	if options != nil {
		a.recordRecent(options.Path)
	}
	return nil
}

// recordRecent remembers a saved file and refreshes the tray "Recents" submenu.
func (a *App) recordRecent(path string) {
	if a.recentsService == nil || path == "" {
		return
	}
	if err := a.recentsService.Add(path); err != nil {
		println("[error] record recent:", err.Error())
		return
	}
	a.refreshTrayRecents()
}

// refreshTrayRecents pushes the current recent list to the tray menu.
func (a *App) refreshTrayRecents() {
	if a.recentsService == nil || a.trayManager == nil {
		return
	}

	paths, err := a.recentsService.List()
	if err != nil {
		println("[error] list recents:", err.Error())
	}
	if err := a.trayManager.SetRecents(paths); err != nil {
		println("[error] set tray recents:", err.Error())
	}
}

// QuickSaveImage saves image data to the configured default folder (falling
// back to the platform default) using a timestamped filename, and returns the
// full path that was written. The path is resolved in Go so the frontend never
// has to guess a platform-specific location.
func (a *App) QuickSaveImage(data []byte) (string, error) {
	settings, err := a.settingsService.GetAll()
	if err != nil {
		return "", fmt.Errorf("failed to read settings: %w", err)
	}

	dir := settings.DefaultSavePath
	if dir == "" {
		dir, err = a.fileService.GetDefaultSavePath()
		if err != nil {
			return "", fmt.Errorf("failed to resolve default save folder: %w", err)
		}
	}

	base := strings.TrimSuffix(filepath.Join(dir, a.fileService.GenerateFilename("png")), ".png")
	path := base + ".png"
	// Two quick saves can land in the same second (the filename has 1s
	// resolution), so never overwrite an existing file.
	for i := 1; ; i++ {
		if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
			break
		}
		path = fmt.Sprintf("%s_%d.png", base, i)
	}

	options := &model.SaveOptions{
		Path:    path,
		Format:  "png",
		Quality: 90,
	}
	if err := a.fileService.SaveImage(options, data); err != nil {
		println("[error] QuickSaveImage:", err.Error())
		return "", fmt.Errorf("failed to save %s: %w", path, err)
	}

	a.recordRecent(path)

	return path, nil
}

// ReadClipboardImage returns the image on the system clipboard as base64 PNG.
func (a *App) ReadClipboardImage() (string, error) {
	data, err := a.clipboardService.ReadImage()
	if err != nil {
		// clipboard.ErrNoImage reads "no image on the clipboard"; the frontend
		// shows the message as a toast.
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// CopyImageToClipboard copies image data to clipboard
func (a *App) CopyImageToClipboard(data []byte) error {
	return a.clipboardService.CopyImage(data)
}

// GenerateFilename generates a filename with timestamp
func (a *App) GenerateFilename(format string) string {
	return a.fileService.GenerateFilename(format)
}

// GetSettings returns all settings
func (a *App) GetSettings() (*model.Settings, error) {
	return a.settingsService.GetAll()
}

// UpdateSetting updates a single setting
func (a *App) UpdateSetting(key string, value interface{}) error {
	err := a.settingsService.Set(key, value)
	if err != nil {
		return err
	}

	// If hotkey setting changed, re-register hotkeys
	if strings.HasPrefix(key, "hotkeys.") {
		a.setupHotkeys()
	}

	// If run_at_startup setting changed, update login item
	if key == "run_at_startup" {
		if enabled, ok := value.(bool); ok {
			if enabled {
				_ = a.startupService.Enable()
			} else {
				_ = a.startupService.Disable()
			}
		}
	}

	return nil
}

// CheckAccessibilityPermission checks if the app has Accessibility permissions
func (a *App) CheckAccessibilityPermission() (bool, error) {
	return a.permissionService.CheckAccessibility()
}

// RequestAccessibilityPermission requests Accessibility permissions
func (a *App) RequestAccessibilityPermission() error {
	return a.permissionService.RequestAccessibility()
}

// CheckScreenRecordingPermission checks if the app has Screen Recording permissions
func (a *App) CheckScreenRecordingPermission() (bool, error) {
	return a.permissionService.CheckScreenRecording()
}

// RequestScreenRecordingPermission requests Screen Recording permissions
func (a *App) RequestScreenRecordingPermission() error {
	return a.permissionService.RequestScreenRecording()
}

// PauseHotkeys temporarily stops hotkey listening (for recording new hotkeys)
func (a *App) PauseHotkeys() error {
	println("[DEBUG] Pausing hotkeys...")
	return a.hotkeyService.Stop()
}

// ResumeHotkeys resumes hotkey listening after recording
func (a *App) ResumeHotkeys() error {
	println("[DEBUG] Resuming hotkeys...")
	return a.hotkeyService.Start()
}

// GetVersion returns application version information
func (a *App) GetVersion() version.Info {
	return version.Get()
}

// Upload methods

// UploadInit initiates a file upload. visibility is "public", "private", or
// "" to use the "Upload as public" setting.
func (a *App) UploadInit(filename string, size int64, contentType, visibility string) (*upload.InitResponse, error) {
	return a.uploadService.Init(filename, size, contentType, visibility)
}

// UploadComplete completes a file upload
func (a *App) UploadComplete(fileID string) (*upload.CompleteResponse, error) {
	return a.uploadService.Complete(fileID)
}

// IsUploadConfigured checks if upload is configured (server URL + API key set)
func (a *App) IsUploadConfigured() bool {
	return a.uploadService.IsConfigured()
}

// TestUploadConnection verifies the configured server URL + API key are valid
func (a *App) TestUploadConnection() error {
	return a.uploadService.TestConnection()
}

// Window state methods

// applyWindowState restores the saved maximized/normal editor window state.
func (a *App) applyWindowState() {
	if a.recordingBar.Load() {
		return // the window is the recording bar; keep its size
	}
	s, err := a.settingsService.GetAll()
	if err != nil {
		return
	}
	println("[window] applyWindowState maximized=", s.WindowMaximized, "size=", s.WindowWidth, s.WindowHeight)
	if s.WindowMaximized {
		a.window.Maximise()
		return
	}
	a.window.UnMaximise()
	if s.WindowWidth > 0 && s.WindowHeight > 0 {
		a.window.SetSize(s.WindowWidth, s.WindowHeight)
	}
}

// saveWindowState persists the current editor window size and maximized state.
func (a *App) saveWindowState() {
	// A hidden or already destroyed window reports no size; its state was saved
	// when it was hidden.
	if a.window == nil || a.recordingBar.Load() || !a.window.IsVisible() {
		return
	}
	maxed := a.window.IsMaximised()
	w, h := a.window.Size()
	println("[window] saveWindowState maximized=", maxed, "size=", w, h)
	_ = a.settingsService.Set("window_maximized", maxed)
	// Only remember the size when not maximized (maximized size is the screen).
	if !maxed && w > 0 && h > 0 {
		_ = a.settingsService.Set("window_width", w)
		_ = a.settingsService.Set("window_height", h)
	}
}

// ShowWindow shows the editor window, restoring its last maximized/normal state.
func (a *App) ShowWindow() {
	if a.recordingBar.Load() {
		// Keep the recording bar's size and position.
		a.window.Show()
		return
	}
	a.window.Show()
	a.window.UnMinimise()
	a.applyWindowState()
	focusWebView(a.window)
	// Re-apply shortly after: on Windows, maximising right after Show can be
	// ignored because the window hasn't finished becoming visible yet.
	go func() {
		time.Sleep(120 * time.Millisecond)
		a.applyWindowState()
		focusWebView(a.window)
	}()
}

// HideWindow saves the current window state and hides the editor window.
func (a *App) HideWindow() {
	a.saveWindowState()
	a.window.Hide()
}

// onWindowClosing is the close button. The app lives in the tray, so closing
// the window only hides it, after saving its state. Only a real quit (tray or
// menu Quit) lets the window close. Cmd+Q and logout on macOS do not come
// through here: they end the app directly and run shutdown.
func (a *App) onWindowClosing(e *application.WindowEvent) {
	if a.quitting.Load() {
		return
	}
	e.Cancel()
	a.HideWindow()
}

// onFilesDropped passes files dropped on the editor to the frontend, with the
// drop point in CSS pixels inside the web view.
func (a *App) onFilesDropped(e *application.WindowEvent) {
	paths := e.Context().DroppedFiles()
	if len(paths) == 0 {
		return
	}
	var x, y int
	if d := e.Context().DropTargetDetails(); d != nil {
		x, y = d.X, d.Y
	}
	a.emit("files:dropped", FilesDropped{X: x, Y: y, Paths: paths})
}

// FilesDropped is the payload of the "files:dropped" event.
type FilesDropped struct {
	X     int      `json:"x"`
	Y     int      `json:"y"`
	Paths []string `json:"paths"`
}

// quitApp marks a real quit and exits (used by tray/menu Quit).
func (a *App) quitApp() {
	a.quitting.Store(true)
	a.app.Quit()
}

func main() {
	// Create an instance of the app structure
	fasp := NewApp()

	app := application.New(application.Options{
		Name:        "fasp",
		Description: "Screenshot & Annotation Tool\n\n" + version.Get().String(),
		Services: []application.Service{
			application.NewService(fasp),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			// Requests the embedded assets cannot answer (/media/...) play
			// local videos in the editor.
			Middleware: fasp.media.middleware,
		},
		// The window is hidden, not closed, so the last window closing must
		// not end the app.
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		// A second launch brings the running instance to the front instead of
		// starting a second tray icon and a second set of global hotkeys.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.wails.fasp",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				fasp.ShowWindow()
				fasp.window.Focus()
			},
		},
		OnShutdown: fasp.shutdown,
	})
	fasp.app = app

	app.Menu.Set(fasp.buildMenu())

	fasp.window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Fasp - Screenshot & Annotation",
		Width:            1024,
		Height:           768,
		URL:              "/",
		BackgroundColour: application.NewRGB(27, 38, 54),
		// Dropped files reach the frontend through the "files:dropped" event.
		EnableFileDrop: true,
	})
	fasp.window.RegisterHook(events.Common.WindowClosing, fasp.onWindowClosing)
	fasp.window.OnWindowEvent(events.Common.WindowFilesDropped, fasp.onFilesDropped)
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, fasp.onApplicationStarted)

	if err := app.Run(); err != nil {
		println("Error:", err.Error())
	}
}
