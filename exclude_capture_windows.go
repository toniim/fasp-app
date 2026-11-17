//go:build windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// setExcludedFromCapture hides the Fasp window from screen captures and
// recordings (it stays visible on screen), so the recording control bar never
// shows up in the video. Wails applies WDA_EXCLUDEFROMCAPTURE for it.
func setExcludedFromCapture(w *application.WebviewWindow, excluded bool) {
	w.SetContentProtection(excluded)
}
