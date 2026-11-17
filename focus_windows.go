//go:build windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// focusWebView gives the editor's WebView2 keyboard focus.
//
// When the window is shown again from the tray or a hotkey, Windows activates
// it without moving focus into the web view, so keyboard shortcuts were dead
// until the user clicked inside the editor. Window.Focus raises the window and
// moves focus into the web view.
func focusWebView(w *application.WebviewWindow) {
	w.Focus()
}
