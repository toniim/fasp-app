package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// The recording options open in their own small window instead of over the
// editor: the editor keeps its state (a hidden editor stays hidden), and the
// options come to the front with the keyboard focus wherever they are opened
// from (tray, File menu, hotkey).

const (
	recordWindowName   = "record"
	recordWindowWidth  = 480
	recordWindowHeight = 560 // first size; the page fits the window to its content
)

// recordMenuLabel names the record item after what a click does now.
func recordMenuLabel(recording bool) string {
	if recording {
		return "Stop Recording"
	}
	return "Start Recording…"
}

// setRecordingUI renames the record items in the tray and the File menu.
func (a *App) setRecordingUI(recording bool) {
	if a.trayManager != nil {
		a.trayManager.SetRecording(recording)
	}
	if a.recordMenuItem != nil {
		a.recordMenuItem.SetLabel(recordMenuLabel(recording))
	}
}

// OpenRecordWindow shows the recording options window, creating it when
// needed, in front and focused.
func (a *App) OpenRecordWindow() {
	a.recordWinMu.Lock()
	defer a.recordWinMu.Unlock()
	if w := a.recordWin; w != nil {
		w.Show()
		w.Focus()
		return
	}
	// Centre it on the editor's screen when the editor shows, else the
	// primary screen.
	screen := a.app.Screen.GetPrimary()
	if a.window != nil && a.window.IsVisible() {
		if s, err := a.window.GetScreen(); err == nil && s != nil {
			screen = s
		}
	}
	w := a.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:                  recordWindowName,
		Title:                 "Record Screen",
		Width:                 recordWindowWidth,
		Height:                recordWindowHeight,
		InitialPosition:       application.WindowCentered,
		Screen:                screen,
		URL:                   "/?view=record",
		DisableResize:         true,
		MinimiseButtonState:   application.ButtonHidden,
		MaximiseButtonState:   application.ButtonHidden,
		FullscreenButtonState: application.ButtonHidden,
	})
	w.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		a.recordWinMu.Lock()
		if a.recordWin == w {
			a.recordWin = nil
		}
		a.recordWinMu.Unlock()
	})
	a.recordWin = w
	// The page also asks for the focus once it has loaded: a window that is
	// still being created cannot take it yet.
	w.Show()
	w.Focus()
}

// CloseRecordWindow closes the options window: Cancel, Esc, or the recording
// has started.
func (a *App) CloseRecordWindow() {
	a.recordWinMu.Lock()
	w := a.recordWin
	a.recordWin = nil
	a.recordWinMu.Unlock()
	if w != nil {
		w.Close()
	}
}
