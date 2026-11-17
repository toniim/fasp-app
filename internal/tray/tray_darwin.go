//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "tray_darwin.h"
*/
import "C"
import (
	"strconv"
	"unsafe"
)

var globalCallback Callback

type darwinManager struct {
	initialized bool
}

func NewManager() Manager {
	return &darwinManager{}
}

func (m *darwinManager) Setup(callback Callback) error {
	if m.initialized {
		return nil
	}

	globalCallback = callback
	C.tray_init()
	m.initialized = true

	return nil
}

func (m *darwinManager) UpdateIcon(iconData []byte) error {
	// Not implemented - use SetIconFromFile instead
	return nil
}

func (m *darwinManager) SetIconFromFile(iconPath string) error {
	if !m.initialized {
		return nil
	}

	if iconPath == "" {
		println("[ERROR] SetIconFromFile: iconPath is empty")
		return nil
	}

	println("[DEBUG] SetIconFromFile called with path:", iconPath)

	cPath := C.CString(iconPath)
	defer C.free(unsafe.Pointer(cPath))

	C.tray_set_icon(cPath)
	return nil
}

// SetRecents replaces the contents of the "Recents" submenu.
func (m *darwinManager) SetRecents(paths []string) error {
	if !m.initialized {
		return nil
	}

	if len(paths) == 0 {
		C.tray_set_recents(nil, 0)
		return nil
	}

	cPaths := make([]*C.char, len(paths))
	for i, path := range paths {
		cPaths[i] = C.CString(path)
	}
	defer func() {
		for _, cPath := range cPaths {
			C.free(unsafe.Pointer(cPath))
		}
	}()

	// tray_set_recents copies the strings before returning.
	C.tray_set_recents(&cPaths[0], C.int(len(cPaths)))

	return nil
}

// SetCountdown shows the seconds left in the menu bar and the "Cancel
// Countdown" item; 0 clears them.
func (m *darwinManager) SetCountdown(remaining int) {
	if !m.initialized {
		return
	}
	C.tray_set_countdown(C.int(remaining))
}

func (m *darwinManager) Quit() {
	if !m.initialized {
		return
	}

	C.tray_cleanup()
	m.initialized = false
}

//export tray_onCapture
func tray_onCapture() {
	if globalCallback != nil {
		go globalCallback(ActionCapture, "")
	}
}

//export tray_onCaptureZone
func tray_onCaptureZone() {
	if globalCallback != nil {
		go globalCallback(ActionCaptureZone, "")
	}
}

//export tray_onCaptureDelayed
func tray_onCaptureDelayed(seconds C.int) {
	if globalCallback != nil {
		go globalCallback(ActionCaptureDelayed, strconv.Itoa(int(seconds)))
	}
}

//export tray_onCaptureZoneDelayed
func tray_onCaptureZoneDelayed(seconds C.int) {
	if globalCallback != nil {
		go globalCallback(ActionCaptureZoneDelayed, strconv.Itoa(int(seconds)))
	}
}

//export tray_onCaptureText
func tray_onCaptureText() {
	if globalCallback != nil {
		go globalCallback(ActionCaptureText, "")
	}
}

//export tray_onRepeatZone
func tray_onRepeatZone() {
	if globalCallback != nil {
		go globalCallback(ActionRepeatZone, "")
	}
}

//export tray_onCancelCountdown
func tray_onCancelCountdown() {
	if globalCallback != nil {
		go globalCallback(ActionCancelCountdown, "")
	}
}

//export tray_onRecord
func tray_onRecord() {
	if globalCallback != nil {
		go globalCallback(ActionRecord, "")
	}
}

//export tray_onOpenImage
func tray_onOpenImage() {
	if globalCallback != nil {
		go globalCallback(ActionOpenImage, "")
	}
}

//export tray_onShow
func tray_onShow() {
	if globalCallback != nil {
		go globalCallback(ActionShowWindow, "")
	}
}

//export tray_onHide
func tray_onHide() {
	if globalCallback != nil {
		go globalCallback(ActionHideWindow, "")
	}
}

//export tray_onSettings
func tray_onSettings() {
	if globalCallback != nil {
		go globalCallback(ActionSettings, "")
	}
}

//export tray_onOpenRecent
func tray_onOpenRecent(path *C.char) {
	if globalCallback != nil && path != nil {
		go globalCallback(ActionOpenRecent, C.GoString(path))
	}
}

//export tray_onOpenRecentsFolder
func tray_onOpenRecentsFolder() {
	if globalCallback != nil {
		go globalCallback(ActionOpenRecentsFolder, "")
	}
}

//export tray_onQuit
func tray_onQuit() {
	if globalCallback != nil {
		go globalCallback(ActionQuit, "")
	}
}
