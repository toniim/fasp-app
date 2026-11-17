//go:build windows

package capture

import (
	"image"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows a click on the freeze overlay can take.

var (
	procEnumWindows          = user32.NewProc("EnumWindows")
	procIsWindowVisible      = user32.NewProc("IsWindowVisible")
	procIsIconic             = user32.NewProc("IsIconic")
	procGetWindowLongPtrW    = user32.NewProc("GetWindowLongPtrW")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procGetClassNameW        = user32.NewProc("GetClassNameW")
	procGetWindowRect        = user32.NewProc("GetWindowRect")

	dwmapi                    = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmGetWindowAttribute = dwmapi.NewProc("DwmGetWindowAttribute")
)

const (
	gwlExStyle               = ^uintptr(19) // -20
	wsExTransparent          = 0x00000020
	dwmwaExtendedFrameBounds = 9
	dwmwaCloaked             = 14
)

var (
	// One callback for the life of the process: Go limits how many
	// syscall.NewCallback calls a process may make.
	enumTargetsOnce sync.Once
	enumTargetsCB   uintptr
	enumTargetsMu   sync.Mutex
	enumTargetsOut  []image.Rectangle
)

// windowTargets lists the visible top-level windows in Z order (front-most
// first), as rectangles in physical pixels relative to the top-left of mr.
// Called right after the freeze, before the overlay exists, so it matches the
// frozen picture.
func windowTargets(mr rect) []image.Rectangle {
	enumTargetsOnce.Do(func() {
		enumTargetsCB = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
			if r, ok := targetRect(hwnd); ok {
				enumTargetsOut = append(enumTargetsOut, r)
			}
			return 1
		})
	})
	enumTargetsMu.Lock()
	defer enumTargetsMu.Unlock()
	enumTargetsOut = nil
	procEnumWindows.Call(enumTargetsCB, 0)

	off := image.Pt(int(mr.Left), int(mr.Top))
	out := make([]image.Rectangle, 0, len(enumTargetsOut))
	for _, r := range enumTargetsOut {
		out = append(out, r.Sub(off))
	}
	return out
}

// targetRect returns the on-screen frame of a window a user would call "a
// window": visible, not minimized, not on another virtual desktop, with a
// title, and not click-through. The desktop itself is left out so a click on
// it keeps the overlay open.
func targetRect(hwnd uintptr) (image.Rectangle, bool) {
	if r, _, _ := procIsWindowVisible.Call(hwnd); r == 0 {
		return image.Rectangle{}, false
	}
	if r, _, _ := procIsIconic.Call(hwnd); r != 0 {
		return image.Rectangle{}, false
	}
	if ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle); ex&wsExTransparent != 0 {
		return image.Rectangle{}, false
	}
	var cloaked uint32
	procDwmGetWindowAttribute.Call(hwnd, dwmwaCloaked, uintptr(unsafe.Pointer(&cloaked)), 4)
	if cloaked != 0 {
		return image.Rectangle{}, false
	}
	if n, _, _ := procGetWindowTextLengthW.Call(hwnd); n == 0 {
		return image.Rectangle{}, false
	}
	var cls [32]uint16
	procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&cls[0])), uintptr(len(cls)))
	if name := windows.UTF16ToString(cls[:]); name == "Progman" || name == "WorkerW" {
		return image.Rectangle{}, false
	}
	// The extended frame leaves out the invisible resize borders that
	// GetWindowRect includes on Windows 10 and later.
	var r rect
	if hr, _, _ := procDwmGetWindowAttribute.Call(hwnd, dwmwaExtendedFrameBounds, uintptr(unsafe.Pointer(&r)), unsafe.Sizeof(r)); hr != 0 {
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	}
	out := image.Rect(int(r.Left), int(r.Top), int(r.Right), int(r.Bottom))
	return out, !out.Empty()
}
