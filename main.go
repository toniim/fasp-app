package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/heytonyne/grabix/internal/model"
	"github.com/heytonyne/grabix/internal/service/auth"
	"github.com/heytonyne/grabix/internal/service/capture"
	"github.com/heytonyne/grabix/internal/service/clipboard"
	"github.com/heytonyne/grabix/internal/service/file"
	"github.com/heytonyne/grabix/internal/service/hotkey"
	"github.com/heytonyne/grabix/internal/service/permission"
	"github.com/heytonyne/grabix/internal/service/settings"
	"github.com/heytonyne/grabix/internal/service/startup"
	"github.com/heytonyne/grabix/internal/service/upload"
	"github.com/heytonyne/grabix/internal/tray"
	"github.com/heytonyne/grabix/internal/version"
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
	authService       auth.Service
	uploadService     upload.Service
	trayManager       tray.Manager
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
	a.authService = auth.New()
	a.uploadService = upload.New()

	// Setup hotkeys from settings
	a.setupHotkeys()

	// Setup system tray
	a.setupTray()
}

// setupTray initializes the system tray icon
func (a *App) setupTray() {
	slog.Debug("setupTray called")
	a.trayManager = tray.NewManager()

	err := a.trayManager.Setup(func(action tray.Action) {
		slog.Debug("tray action received", "action", action)
		switch action {
		case tray.ActionCapture:
			// Trigger screenshot capture
			runtime.EventsEmit(a.ctx, "hotkey:capture")

		case tray.ActionOpenImage:
			// Trigger open image dialog
			runtime.EventsEmit(a.ctx, "open:image")

		case tray.ActionShowWindow:
			// Show main window
			runtime.WindowShow(a.ctx)
			runtime.WindowUnminimise(a.ctx)

		case tray.ActionHideWindow:
			// Hide main window
			runtime.WindowHide(a.ctx)

		case tray.ActionSettings:
			// Open settings window
			runtime.EventsEmit(a.ctx, "open:settings")

		case tray.ActionQuit:
			// Quit application
			runtime.Quit(a.ctx)
		}
	})

	if err != nil {
		slog.Error("failed to setup tray", "err", err)
		return
	}
	slog.Debug("tray setup completed")

	// Set tray icon — pass embedded bytes directly to avoid writing to /tmp on every launch
	go func() {
		// Small delay so the status item is fully ready before icon update
		time.Sleep(500 * time.Millisecond)

		var iconData []byte
		if goruntime.GOOS == "windows" {
			iconData = trayIconDataWindows
		} else {
			iconData = trayIconData
		}
		slog.Debug("updating tray icon", "os", goruntime.GOOS, "size", len(iconData))

		if err := a.trayManager.UpdateIcon(iconData); err != nil {
			slog.Error("failed to set tray icon", "err", err)
		}
	}()
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
		runtime.Quit(a.ctx)
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
	current, err := a.settingsService.GetAll()
	if err != nil {
		slog.Error("failed to get settings", "err", err)
		return
	}

	// Stop existing hotkey service if running
	a.hotkeyService.Stop()

	hasAny := false
	register := func(name, eventName string) {
		combo, ok := current.Hotkeys[name]
		if !ok || combo == "" {
			return
		}
		slog.Debug("registering hotkey", "name", name, "combo", combo)
		if err := a.hotkeyService.Register(combo, func() {
			slog.Debug("hotkey pressed", "name", name)
			runtime.EventsEmit(a.ctx, eventName)
		}); err != nil {
			slog.Error("failed to register hotkey", "name", name, "combo", combo, "err", err)
			return
		}
		hasAny = true
	}

	register("capture_fullscreen", "hotkey:capture")
	register("capture_region", "hotkey:capture_region")

	if !hasAny {
		slog.Warn("no capture hotkey configured")
		return
	}

	// Start listening AFTER registering all hotkeys
	if err := a.hotkeyService.Start(); err != nil {
		slog.Error("failed to start hotkey service", "err", err)
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
	return a.fileService.SaveImage(options, data)
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
	slog.Debug("pausing hotkeys")
	return a.hotkeyService.Stop()
}

// ResumeHotkeys resumes hotkey listening after recording
func (a *App) ResumeHotkeys() error {
	slog.Debug("resuming hotkeys")
	return a.hotkeyService.Start()
}

// GetVersion returns application version information
func (a *App) GetVersion() version.Info {
	return version.Get()
}

// Auth methods

// AuthStartLogin initiates the OAuth login flow
func (a *App) AuthStartLogin() (string, error) {
	return a.authService.StartLogin()
}

// AuthHandleCallback processes the OAuth callback
func (a *App) AuthHandleCallback(code string) (*model.User, error) {
	return a.authService.HandleCallback(code)
}

// AuthGetCurrentUser returns the currently authenticated user
func (a *App) AuthGetCurrentUser() (*model.User, error) {
	return a.authService.GetCurrentUser()
}

// AuthLogout logs out the current user
func (a *App) AuthLogout() error {
	return a.authService.Logout()
}

// AuthIsLoggedIn checks if user is logged in
func (a *App) AuthIsLoggedIn() bool {
	return a.authService.IsLoggedIn()
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

// IsUploadConfigured checks if upload is configured
func (a *App) IsUploadConfigured() bool {
	return a.uploadService.IsConfigured()
}

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:             "Grabix - Screenshot & Annotation",
		Width:             1024,
		Height:            768,
		StartHidden:       false, // TEMP: Show window for testing tray
		HideWindowOnClose: true,  // Hide instead of quit when closing
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Menu:             app.buildMenu(),
		Mac: &mac.Options{
			TitleBar: mac.TitleBarDefault(),
			About: &mac.AboutInfo{
				Title:   "Grabix",
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
		slog.Error("wails run failed", "err", err)
	}
}

func init() {
	level := slog.LevelInfo
	if strings.EqualFold(os.Getenv("GRABIX_LOG_LEVEL"), "debug") {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}
