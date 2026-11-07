//go:build windows

package clipboard

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/png" // decode PNG input
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procSetClipboardData = user32.NewProc("SetClipboardData")

	procGlobalAlloc   = kernel32.NewProc("GlobalAlloc")
	procGlobalFree    = kernel32.NewProc("GlobalFree")
	procGlobalLock    = kernel32.NewProc("GlobalLock")
	procGlobalUnlock  = kernel32.NewProc("GlobalUnlock")
	procRtlMoveMemory = kernel32.NewProc("RtlMoveMemory")
)

const (
	cfDIB        = 8
	gmemMoveable = 0x0002
)

// serviceImpl implements the Service interface for Windows
type serviceImpl struct{}

// New creates a new clipboard service instance
func New() Service {
	return &serviceImpl{}
}

// CopyImage puts the image on the clipboard as CF_DIB, the format
// System.Windows.Forms.Clipboard.SetImage produces.
func (s *serviceImpl) CopyImage(data []byte) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to copy to clipboard: decode image: %w", err)
	}
	if img.Bounds().Empty() {
		return errors.New("failed to copy to clipboard: image is empty")
	}
	if err := setClipboardDIB(imageToDIB(img)); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}
	return nil
}

func setClipboardDIB(dib []byte) error {
	// The clipboard is opened per thread: Open, Set and Close must run on the
	// same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := openClipboard(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()

	if r, _, e := procEmptyClipboard.Call(); r == 0 {
		return fmt.Errorf("EmptyClipboard: %w", e)
	}

	h, _, e := procGlobalAlloc.Call(gmemMoveable, uintptr(len(dib)))
	if h == 0 {
		return fmt.Errorf("GlobalAlloc: %w", e)
	}
	ptr, _, e := procGlobalLock.Call(h)
	if ptr == 0 {
		procGlobalFree.Call(h)
		return fmt.Errorf("GlobalLock: %w", e)
	}
	procRtlMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&dib[0])), uintptr(len(dib)))
	procGlobalUnlock.Call(h)

	if r, _, e := procSetClipboardData.Call(cfDIB, h); r == 0 {
		procGlobalFree.Call(h) // still ours when SetClipboardData fails
		return fmt.Errorf("SetClipboardData: %w", e)
	}
	return nil // the system owns h now
}

// openClipboard retries briefly: another app (clipboard history, a password
// manager) may hold the clipboard open for a moment.
func openClipboard() error {
	var lastErr error
	for i := 0; i < 5; i++ {
		r, _, e := procOpenClipboard.Call(0)
		if r != 0 {
			return nil
		}
		lastErr = e
		time.Sleep(20 * time.Millisecond)
	}
	return errors.Join(errors.New("OpenClipboard: clipboard is busy"), lastErr)
}
