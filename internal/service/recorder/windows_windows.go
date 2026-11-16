//go:build windows

package recorder

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	gwlExStyle       = ^uintptr(19) // -20
	wsExToolWindow   = 0x00000080
	dwmwaCloaked     = 14
	processQueryInfo = windows.PROCESS_QUERY_LIMITED_INFORMATION
)

// ListWindows returns the visible top-level windows a user would expect in a
// "record window" picker, in Z order (front-most first).
func ListWindows() ([]Window, error) {
	var out []Window
	cb := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		if w, ok := describeWindow(hwnd); ok {
			out = append(out, w)
		}
		return 1
	})
	if r, _, e := procEnumWindows.Call(cb, 0); r == 0 {
		return nil, e
	}
	return out, nil
}

func describeWindow(hwnd uintptr) (Window, bool) {
	if r, _, _ := procIsWindowVisible.Call(hwnd); r == 0 {
		return Window{}, false
	}
	if r, _, _ := procIsIconic.Call(hwnd); r != 0 {
		return Window{}, false // minimized windows deliver no frames
	}
	if ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle); ex&wsExToolWindow != 0 {
		return Window{}, false
	}
	var cloaked uint32
	procDwmGetWindowAttribute.Call(hwnd, dwmwaCloaked, uintptr(unsafe.Pointer(&cloaked)), 4)
	if cloaked != 0 { // e.g. windows on another virtual desktop, suspended UWP apps
		return Window{}, false
	}
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return Window{}, false
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)

	var r rect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	w := Window{
		Handle: uint64(hwnd),
		Title:  windows.UTF16ToString(buf),
		Width:  int(r.Right - r.Left),
		Height: int(r.Bottom - r.Top),
	}
	if w.Width < 2 || w.Height < 2 {
		return Window{}, false
	}
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == uint32(os.Getpid()) { // never offer Fasp's own windows
		return Window{}, false
	}
	w.App = processName(pid)
	return w, true
}

func processName(pid uint32) string {
	h, err := windows.OpenProcess(processQueryInfo, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return ""
	}
	return filepath.Base(windows.UTF16ToString(buf[:n]))
}
