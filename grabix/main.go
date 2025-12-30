package main

import (
	"context"
	"embed"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/heytonyne/grabix/internal/model"
	"github.com/heytonyne/grabix/internal/service/capture"
	"github.com/heytonyne/grabix/internal/service/clipboard"
	"github.com/heytonyne/grabix/internal/service/file"
	"github.com/heytonyne/grabix/internal/service/hotkey"
	"github.com/heytonyne/grabix/internal/service/permission"
	"github.com/heytonyne/grabix/internal/service/settings"
	"github.com/heytonyne/grabix/internal/tray"
	"github.com/heytonyne/grabix/internal/version"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/trayicon.png
var trayIconData []byte

// App struct
type App struct {
	ctx               context.Context
	captureService    capture.Service
	fileService       file.Service
	settingsService   settings.Service
	hotkeyService     hotkey.Service
	permissionService permission.Service
	clipboardService  clipboard.Service
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

	err := a.trayManager.Setup(func(action tray.Action) {
		println("[DEBUG] Tray action received:", action)
		switch action {
		case tray.ActionCapture:
			// Trigger screenshot capture
			runtime.EventsEmit(a.ctx, "hotkey:capture")

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
		println("[ERROR] Failed to setup tray:", err.Error())
	} else {
		println("[DEBUG] Tray setup completed successfully")

		// Set tray icon - delay to ensure status item is created first
		// Status item is created with 300ms delay, so we need to wait
		go func() {
			// Wait for status item to be created (300ms + buffer)
			time.Sleep(500 * time.Millisecond)

			println("[DEBUG] Embedded icon data size:", len(trayIconData), "bytes")

			// 1. Try embedded icon first
			tmpDir := os.TempDir()
			iconPath := filepath.Join(tmpDir, "grabix-trayicon.png")

			// Write embedded icon data to temp file
			writeErr := os.WriteFile(iconPath, trayIconData, 0644)
			if writeErr != nil {
				println("[ERROR] Failed to write embedded tray icon:", writeErr.Error())

				// 2. Fallback to build directory
				iconPath = "build/trayicon.png"
				println("[DEBUG] Trying fallback path:", iconPath)
			} else {
				println("[DEBUG] Tray icon written to:", iconPath)
			}

			// Try to set the icon
			if err := a.trayManager.SetIconFromFile(iconPath); err != nil {
				println("[ERROR] Failed to set tray icon:", err.Error())
			} else {
				println("[DEBUG] Tray icon set successfully from:", iconPath)
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
	settings, err := a.settingsService.GetAll()
	if err != nil {
		println("Failed to get settings:", err.Error())
		return
	}

	// Stop existing hotkey service if running
	a.hotkeyService.Stop()

	// Register capture hotkey BEFORE starting
	if captureKey, ok := settings.Hotkeys["capture_fullscreen"]; ok && captureKey != "" {
		println("Setting up hotkey:", captureKey)
		err := a.hotkeyService.Register(captureKey, func() {
			println("🔥 Hotkey pressed! Emitting event...")
			// Trigger capture when hotkey is pressed
			runtime.EventsEmit(a.ctx, "hotkey:capture")
		})
		if err != nil {
			println("Failed to register hotkey:", err.Error())
			return
		}
	} else {
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
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
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
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
