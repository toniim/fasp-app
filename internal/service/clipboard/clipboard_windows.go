//go:build windows

package clipboard

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png" // decode PNG input, encode PNG output
	"os"
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
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procIsFormatAvail    = user32.NewProc("IsClipboardFormatAvailable")
	procRegisterFormat   = user32.NewProc("RegisterClipboardFormatW")

	procGlobalAlloc   = kernel32.NewProc("GlobalAlloc")
	procGlobalFree    = kernel32.NewProc("GlobalFree")
	procGlobalLock    = kernel32.NewProc("GlobalLock")
	procGlobalUnlock  = kernel32.NewProc("GlobalUnlock")
	procGlobalSize    = kernel32.NewProc("GlobalSize")
	procRtlMoveMemory = kernel32.NewProc("RtlMoveMemory")
)

const (
	cfDIB        = 8
	cfHDrop      = 15
	cfDIBV5      = 17
	gmemMoveable = 0x0002

	// Upper bound on a clipboard block we copy out, so a bogus size cannot
	// exhaust memory.
	maxClipboardBytes = 1 << 30
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
	if err := setClipboardData(cfDIB, imageToDIB(img)); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}
	return nil
}

// CopyFiles puts the files on the clipboard as CF_HDROP, like Explorer's Copy.
func (s *serviceImpl) CopyFiles(paths []string) error {
	if len(paths) == 0 {
		return errors.New("failed to copy files: no files")
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("failed to copy files: %w", err)
		}
	}
	if err := setClipboardData(cfHDrop, dropFiles(paths)); err != nil {
		return fmt.Errorf("failed to copy files: %w", err)
	}
	return nil
}

// ReadImage returns the clipboard image as PNG bytes. It prefers the registered
// "PNG" format (browsers, Snipping Tool, most editors keep alpha there) and
// falls back to CF_DIBV5, then CF_DIB.
func (s *serviceImpl) ReadImage() ([]byte, error) {
	// Open, read and Close must run on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := openClipboard(); err != nil {
		return nil, err
	}
	defer procCloseClipboard.Call()

	if pngFmt := registerPNGFormat(); pngFmt != 0 {
		if r, _, _ := procIsFormatAvail.Call(pngFmt); r != 0 {
			if data, err := readClipboardBlock(pngFmt); err == nil {
				// Only trust the block if it really decodes as an image.
				if _, _, derr := image.DecodeConfig(bytes.NewReader(data)); derr == nil {
					return data, nil
				}
			}
		}
	}

	for _, format := range []uintptr{cfDIBV5, cfDIB} {
		if r, _, _ := procIsFormatAvail.Call(format); r == 0 {
			continue
		}
		block, err := readClipboardBlock(format)
		if err != nil {
			return nil, fmt.Errorf("failed to read clipboard: %w", err)
		}
		img, err := dibToImage(block)
		if err != nil {
			return nil, fmt.Errorf("failed to read clipboard: %w", err)
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("failed to read clipboard: encode png: %w", err)
		}
		return buf.Bytes(), nil
	}
	return nil, ErrNoImage
}

func registerPNGFormat() uintptr {
	name, err := windows.UTF16PtrFromString("PNG")
	if err != nil {
		return 0
	}
	r, _, _ := procRegisterFormat.Call(uintptr(unsafe.Pointer(name)))
	return r
}

// readClipboardBlock copies the global memory block for a format out of the
// already-open clipboard.
func readClipboardBlock(format uintptr) ([]byte, error) {
	h, _, e := procGetClipboardData.Call(format)
	if h == 0 {
		return nil, fmt.Errorf("GetClipboardData: %w", e)
	}
	size, _, _ := procGlobalSize.Call(h)
	if size == 0 || size > maxClipboardBytes {
		return nil, fmt.Errorf("clipboard block has unusable size %d", size)
	}
	ptr, _, e := procGlobalLock.Call(h)
	if ptr == 0 {
		return nil, fmt.Errorf("GlobalLock: %w", e)
	}
	defer procGlobalUnlock.Call(h)

	out := make([]byte, size)
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&out[0])), ptr, size)
	return out, nil
}

// setClipboardData replaces the clipboard with one format.
func setClipboardData(format uintptr, dib []byte) error {
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

	if r, _, e := procSetClipboardData.Call(format, h); r == 0 {
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
