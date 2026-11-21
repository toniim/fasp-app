//go:build windows

package main

import (
	"golang.org/x/sys/windows"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
)

// focusWebView brings the editor to the front and gives its WebView2 keyboard
// focus.
//
// Window.Focus only calls SetForegroundWindow, which Windows refuses once
// another process holds the foreground. After a hotkey capture that is often
// the case: the zone overlay has closed, or the user moved the mouse over
// another app while the capture ran, so the editor came up behind the active
// app and only Alt+Tab or the taskbar brought it forward. Borrowing the
// foreground thread's input queue for a moment lets the call through.
//
// Window.Focus also moves focus into the web view: shown again from the tray
// or a hotkey, Windows activates the window without it, and keyboard shortcuts
// were dead until the user clicked inside the editor.
func focusWebView(w *application.WebviewWindow) {
	application.InvokeSync(func() {
		hwnd := uintptr(w.NativeWindow())
		if hwnd != 0 {
			forceForeground(hwnd)
		}
	})
	w.Focus()
}

// forceForeground must run on the thread that owns hwnd (the main thread), so
// attaching that thread's input to the foreground thread's applies to it.
func forceForeground(hwnd uintptr) {
	fg, _, _ := procGetForegroundWindow.Call()
	if fg == hwnd {
		return
	}
	self := uintptr(windows.GetCurrentThreadId())
	var fgThread uintptr
	if fg != 0 {
		fgThread, _, _ = procGetWindowThreadProcessId.Call(fg, 0)
	}
	if fgThread != 0 && fgThread != self {
		if r, _, _ := procAttachThreadInput.Call(self, fgThread, 1); r != 0 {
			defer procAttachThreadInput.Call(self, fgThread, 0)
		}
	}
	procBringWindowToTop.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
}
