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
		go globalCallback(ActionCapture)
	}
}

//export tray_onOpenImage
func tray_onOpenImage() {
	if globalCallback != nil {
		go globalCallback(ActionOpenImage)
	}
}

//export tray_onShow
func tray_onShow() {
	if globalCallback != nil {
		go globalCallback(ActionShowWindow)
	}
}

//export tray_onHide
func tray_onHide() {
	if globalCallback != nil {
		go globalCallback(ActionHideWindow)
	}
}

//export tray_onSettings
func tray_onSettings() {
	if globalCallback != nil {
		go globalCallback(ActionSettings)
	}
}

//export tray_onQuit
func tray_onQuit() {
	if globalCallback != nil {
		go globalCallback(ActionQuit)
	}
}
