//go:build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procSetWindowDisplayAffinity = user32.NewProc("SetWindowDisplayAffinity")

const (
	wdaNone               = 0x00
	wdaExcludeFromCapture = 0x11 // Windows 10 2004+
)

// setExcludedFromCapture hides the Fasp window from screen captures and
// recordings (it stays visible on screen), so the recording control bar never
// shows up in the video.
func setExcludedFromCapture(excluded bool) {
	hwnd := ownWindow()
	if hwnd == 0 {
		return
	}
	affinity := uintptr(wdaNone)
	if excluded {
		affinity = wdaExcludeFromCapture
	}
	procSetWindowDisplayAffinity.Call(hwnd, affinity)
}

// ownWindow finds this process's Wails main window.
func ownWindow() uintptr {
	class, err := windows.UTF16PtrFromString("wailsWindow")
	if err != nil {
		return 0
	}
	self := uint32(os.Getpid())
	var hwnd uintptr
	for {
		hwnd, _, _ = procFindWindowExW.Call(0, hwnd, uintptr(unsafe.Pointer(class)), 0)
		if hwnd == 0 {
			return 0
		}
		var pid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == self {
			return hwnd
		}
	}
}
