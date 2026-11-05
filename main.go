package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
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
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/trayicon.png
var trayIconData []byte

//go:embed build/windows/trayicon.ico
var trayIconDataWindows []byte

// App struct
type App struct {
	ctx               context.Context
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
	quitting          bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize services
	a.captureService = capture.New()
	a.fileService = file.New(ctx)
	a.settingsService = settings.New()
	a.hotkeyService = hotkey.New()
	a.permissionService = permission.New()
	a.clipboardService = clipboard.New()
	a.startupService = startup.New()
	a.uploadService = upload.New(a.settingsService)
	a.recentsService = recents.New()

	// Setup hotkeys from settings
	a.setupHotkeys()

	// Setup system tray
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
			runtime.EventsEmit(a.ctx, "hotkey:capture")

		case tray.ActionOpenImage:
			// Trigger open image dialog
			runtime.EventsEmit(a.ctx, "open:image")

		case tray.ActionShowWindow:
			// Show main window (restore last maximized/normal state)
			a.ShowWindow()

		case tray.ActionHideWindow:
			// Hide main window (save state first)
			a.HideWindow()

		case tray.ActionSettings:
			// Open settings window
			runtime.EventsEmit(a.ctx, "open:settings")

		case tray.ActionOpenRecent:
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

// shutdown is called when the app is shutting down
func (a *App) shutdown(ctx context.Context) {
	// Cleanup tray
	if a.trayManager != nil {
		a.trayManager.Quit()
	}

	// Stop hotkey service
	a.hotkeyService.Stop()
}

// buildMenu creates the application menu
func (a *App) buildMenu() *menu.Menu {
	appMenu := menu.NewMenu()

	// File menu
	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Capture Screenshot", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "hotkey:capture")
	})
	fileMenu.AddText("Open Image...", keys.CmdOrCtrl("o"), func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "open:image")
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Settings...", keys.CmdOrCtrl(","), func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "open:settings")
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		a.quitApp()
	})

	// Window menu
	windowMenu := appMenu.AddSubmenu("Window")
	windowMenu.AddText("Show Window", nil, func(_ *menu.CallbackData) {
		runtime.WindowShow(a.ctx)
		runtime.WindowUnminimise(a.ctx)
	})
	windowMenu.AddText("Hide Window", nil, func(_ *menu.CallbackData) {
		runtime.WindowHide(a.ctx)
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
	registerHotkey := func(settingKey, eventName string) {
		combo, ok := settings.Hotkeys[settingKey]
		if !ok || combo == "" {
			return
		}

		println("Setting up hotkey:", settingKey, "=", combo)
		if err := a.hotkeyService.Register(combo, func() {
			println("🔥 Hotkey pressed:", settingKey)
			runtime.EventsEmit(a.ctx, eventName)
		}); err != nil {
			println("Failed to register hotkey", settingKey, ":", err.Error())
			return
		}
		registered++
	}

	registerHotkey("capture_fullscreen", "hotkey:capture")
	registerHotkey("capture_region", "hotkey:capture_region")

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

	return a.captureService.CaptureActiveDisplay()
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

// UploadInit initiates a file upload
func (a *App) UploadInit(filename string, size int64, contentType string) (*upload.InitResponse, error) {
	return a.uploadService.Init(filename, size, contentType)
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
	s, err := a.settingsService.GetAll()
	if err != nil {
		return
	}
	println("[window] applyWindowState maximized=", s.WindowMaximized, "size=", s.WindowWidth, s.WindowHeight)
	if s.WindowMaximized {
		runtime.WindowMaximise(a.ctx)
		return
	}
	runtime.WindowUnmaximise(a.ctx)
	if s.WindowWidth > 0 && s.WindowHeight > 0 {
		runtime.WindowSetSize(a.ctx, s.WindowWidth, s.WindowHeight)
	}
}

// saveWindowState persists the current editor window size and maximized state.
func (a *App) saveWindowState() {
	if a.ctx == nil {
		return
	}
	maxed := runtime.WindowIsMaximised(a.ctx)
	w, h := runtime.WindowGetSize(a.ctx)
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
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
	a.applyWindowState()
	// Re-apply shortly after: on Windows, maximising right after Show can be
	// ignored because the window hasn't finished becoming visible yet.
	go func() {
		time.Sleep(120 * time.Millisecond)
		a.applyWindowState()
	}()
}

// HideWindow saves the current window state and hides the editor window.
func (a *App) HideWindow() {
	a.saveWindowState()
	runtime.WindowHide(a.ctx)
}

// onBeforeClose saves the window state, then decides whether the app may quit.
//
// macOS: Wails routes the close button AND every app termination (Cmd+Q, Dock
// "Quit", "Quit & Reopen" after a permission grant, logout, SIGTERM) through
// this hook, with no way to tell them apart. HideWindowOnClose is therefore on
// for macOS, so the close button hides the app without reaching this hook, and
// every call that does reach it is a real quit.
//
// Windows: the close button reaches this hook, so it hides the window and keeps
// running unless the quit came from the tray/menu.
func (a *App) onBeforeClose(_ context.Context) bool {
	a.saveWindowState()
	if a.quitting || goruntime.GOOS == "darwin" {
		return false // allow the app to quit
	}
	runtime.WindowHide(a.ctx)
	return true // prevent close; just hide the window
}

// quitApp marks a real quit and exits (used by tray/menu Quit).
func (a *App) quitApp() {
	a.quitting = true
	runtime.Quit(a.ctx)
}

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:       "Fasp - Screenshot & Annotation",
		Width:       1024,
		Height:      768,
		StartHidden: false, // TEMP: Show window for testing tray
		// See onBeforeClose: on macOS this keeps the close button from being
		// treated like a quit; on Windows onBeforeClose hides the window itself.
		HideWindowOnClose: goruntime.GOOS == "darwin",
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.onBeforeClose,
		Menu:             app.buildMenu(),
		Mac: &mac.Options{
			TitleBar: mac.TitleBarDefault(),
			About: &mac.AboutInfo{
				Title:   "Fasp",
				Message: "Screenshot & Annotation Tool\n\n" + version.Get().String(),
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
