//go:build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procFindWindowExW            = user32.NewProc("FindWindowExW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procPostMessageW             = user32.NewProc("PostMessageW")
)

const wmSetFocus = 0x0007

// focusWebView gives the editor's WebView2 keyboard focus.
//
// Wails focuses the web view only when its main window receives WM_SETFOCUS.
// When the window is shown again from the tray or a hotkey, Windows activates
// it without sending one, so keyboard shortcuts were dead until the user
// clicked inside the editor. Posting WM_SETFOCUS runs Wails' own handler,
// which moves focus into the web view.
func focusWebView() {
	class, err := windows.UTF16PtrFromString("wailsWindow")
	if err != nil {
		return
	}
	self := uint32(os.Getpid())
	var hwnd uintptr
	for {
		hwnd, _, _ = procFindWindowExW.Call(0, hwnd, uintptr(unsafe.Pointer(class)), 0)
		if hwnd == 0 {
			return
		}
		var pid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == self {
			procPostMessageW.Call(hwnd, wmSetFocus, 0, 0)
			return
		}
	}
}
