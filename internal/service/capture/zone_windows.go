//go:build windows

package capture

import (
	"errors"
	"fmt"
	"image"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/heytonyne/fasp/internal/model"
)

// Freeze-zone capture on Windows: the monitor under the cursor is captured
// first, then shown frozen in a native top-most popup window. The user drags
// a rectangle on it and only that part is returned. The overlay is plain
// Win32 + GDI on its own thread, independent of the WebView.

var (
	procRegisterClassExW         = user32.NewProc("RegisterClassExW")
	procCreateWindowExW          = user32.NewProc("CreateWindowExW")
	procDestroyWindow            = user32.NewProc("DestroyWindow")
	procDefWindowProcW           = user32.NewProc("DefWindowProcW")
	procGetMessageW              = user32.NewProc("GetMessageW")
	procTranslateMessage         = user32.NewProc("TranslateMessage")
	procDispatchMessageW         = user32.NewProc("DispatchMessageW")
	procPostQuitMessage          = user32.NewProc("PostQuitMessage")
	procLoadCursorW              = user32.NewProc("LoadCursorW")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
	procSetFocus                 = user32.NewProc("SetFocus")
	procSetCapture               = user32.NewProc("SetCapture")
	procReleaseCapture           = user32.NewProc("ReleaseCapture")
	procInvalidateRect           = user32.NewProc("InvalidateRect")
	procBeginPaint               = user32.NewProc("BeginPaint")
	procEndPaint                 = user32.NewProc("EndPaint")
	procFillRect                 = user32.NewProc("FillRect")
	procDrawTextW                = user32.NewProc("DrawTextW")
	procGetDpiForWindow          = user32.NewProc("GetDpiForWindow")

	procCreateDIBSection = gdi32.NewProc("CreateDIBSection")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procSetTextColor     = gdi32.NewProc("SetTextColor")

	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procGetCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
)

const (
	wsPopup        = 0x80000000
	wsVisible      = 0x10000000
	wsExTopmost    = 0x00000008
	wsExToolWindow = 0x00000080
	idcCross       = 32515

	wmDestroy     = 0x0002
	wmPaint       = 0x000F
	wmEraseBkgnd  = 0x0014
	wmKeyDown     = 0x0100
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmRButtonUp   = 0x0205
	vkReturn      = 0x0D
	vkEscape      = 0x1B

	dtCenter     = 0x01
	dtVCenter    = 0x04
	dtSingleLine = 0x20
	dtCalcRect   = 0x400
	transparent  = 1

	colorAccent = 0x00D47800 // COLORREF is 0x00BBGGRR: #0078D4
	colorLabel  = 0x00202020
	colorText   = 0x00FFFFFF
	dimFactor   = 140 // of 256: the frozen screen is shown at ~55% brightness
)

type wndClassEx struct {
	CbSize     uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type winMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	Private uint32
}

type paintStruct struct {
	Hdc         uintptr
	Erase       int32
	RcPaint     rect
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

// gdiSurface is a top-down 32 bpp DIB section selected into its own DC.
type gdiSurface struct {
	dc, bmp, old uintptr
	pix          []byte
}

func newSurface(w, h int) (*gdiSurface, error) {
	bi := bitmapInfo{Header: bitmapInfoHeader{
		BiSize: uint32(unsafe.Sizeof(bitmapInfoHeader{})), BiWidth: int32(w), BiHeight: -int32(h),
		BiPlanes: 1, BiBitCount: 32, BiCompression: biRGB,
	}}
	var bits unsafe.Pointer
	bmp, _, _ := procCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&bi)), dibRGBColors, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == nil {
		return nil, errors.New("CreateDIBSection failed")
	}
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		procDeleteObject.Call(bmp)
		return nil, errors.New("CreateCompatibleDC failed")
	}
	old, _, _ := procSelectObject.Call(dc, bmp)
	return &gdiSurface{dc: dc, bmp: bmp, old: old, pix: unsafe.Slice((*byte)(bits), w*h*4)}, nil
}

func (s *gdiSurface) free() {
	if s == nil {
		return
	}
	procSelectObject.Call(s.dc, s.old)
	procDeleteDC.Call(s.dc)
	procDeleteObject.Call(s.bmp)
}

// zoneOverlay is the state of the one overlay that can be open at a time. It
// is only touched on the overlay's own thread (inside the window procedure).
type zoneOverlay struct {
	hwnd             uintptr
	bounds           image.Rectangle
	clean, dim, back *gdiSurface
	font             uintptr
	accent, label    uintptr // brushes
	wins             []image.Rectangle // see windowTargets; a click takes one
	dragging         bool
	x0, y0, mx, my   int
	result           image.Rectangle
	selected         bool
}

var (
	zoneMu       sync.Mutex // one overlay at a time
	activeZone   *zoneOverlay
	zoneClass    *uint16
	zoneClassErr error
	zoneClassReg sync.Once
)

func registerZoneClass() error {
	zoneClassReg.Do(func() {
		name, _ := windows.UTF16PtrFromString("FaspZoneOverlay")
		cursor, _, _ := procLoadCursorW.Call(0, idcCross)
		wc := wndClassEx{
			CbSize:    uint32(unsafe.Sizeof(wndClassEx{})),
			WndProc:   syscall.NewCallback(zoneWndProc),
			Cursor:    cursor,
			ClassName: name,
		}
		if r, _, e := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			zoneClassErr = fmt.Errorf("RegisterClassExW: %w", e)
			return
		}
		zoneClass = name
	})
	return zoneClassErr
}

// selectZone shows the frozen pixels over the monitor at mr and blocks until
// the user selects a zone (returned in image coordinates) or cancels. wins are
// the windows a click can take, in image coordinates.
func selectZone(pix []byte, w, h int, mr rect, wins []image.Rectangle) (image.Rectangle, error) {
	if !zoneMu.TryLock() {
		return image.Rectangle{}, errors.New("a zone selection is already open")
	}
	defer zoneMu.Unlock()

	type res struct {
		r   image.Rectangle
		err error
	}
	done := make(chan res, 1)
	go func() {
		// The window and its message loop must stay on one OS thread.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		r, err := runZoneOverlay(pix, w, h, mr, wins)
		done <- res{r, err}
	}()
	out := <-done
	return out.r, out.err
}

func runZoneOverlay(pix []byte, w, h int, mr rect, wins []image.Rectangle) (image.Rectangle, error) {
	if err := registerZoneClass(); err != nil {
		return image.Rectangle{}, err
	}
	o := &zoneOverlay{bounds: image.Rect(0, 0, w, h), wins: wins, mx: -1, my: -1}
	defer o.free()
	var err error
	if o.clean, err = newSurface(w, h); err != nil {
		return image.Rectangle{}, err
	}
	if o.dim, err = newSurface(w, h); err != nil {
		return image.Rectangle{}, err
	}
	if o.back, err = newSurface(w, h); err != nil {
		return image.Rectangle{}, err
	}
	copy(o.clean.pix, pix)
	for i, v := range pix {
		o.dim.pix[i] = byte(int(v) * dimFactor >> 8)
	}
	o.accent, _, _ = procCreateSolidBrush.Call(colorAccent)
	o.label, _, _ = procCreateSolidBrush.Call(colorLabel)

	activeZone = o
	defer func() { activeZone = nil }()
	hwnd, _, e := procCreateWindowExW.Call(wsExTopmost|wsExToolWindow, uintptr(unsafe.Pointer(zoneClass)), 0,
		wsPopup|wsVisible, uintptr(mr.Left), uintptr(mr.Top), uintptr(w), uintptr(h), 0, 0, 0, 0)
	if hwnd == 0 {
		return image.Rectangle{}, fmt.Errorf("CreateWindowExW: %w", e)
	}
	o.hwnd = hwnd
	o.makeFont()
	// The overlay takes the foreground for Esc and Enter. On cancel the window
	// the user was in gets it back, so the screen stays as it was.
	previous, _, _ := procGetForegroundWindow.Call()
	forceForeground(hwnd)
	defer func() {
		if !o.selected && previous != 0 && previous != hwnd {
			procSetForegroundWindow.Call(previous)
		}
	}()

	var msg winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	if !o.selected {
		return image.Rectangle{}, ErrCancelled
	}
	return o.result, nil
}

// forceForeground gives the overlay keyboard focus so Esc and Enter work.
// A hotkey press normally lets us take the foreground; if Windows refuses,
// borrow the foreground thread's input queue for a moment.
func forceForeground(hwnd uintptr) {
	procSetForegroundWindow.Call(hwnd)
	if fg, _, _ := procGetForegroundWindow.Call(); fg != hwnd && fg != 0 {
		fgThread, _, _ := procGetWindowThreadProcessId.Call(fg, 0)
		self, _, _ := procGetCurrentThreadId.Call()
		procAttachThreadInput.Call(self, fgThread, 1)
		procSetForegroundWindow.Call(hwnd)
		procAttachThreadInput.Call(self, fgThread, 0)
	}
	procSetFocus.Call(hwnd)
}

func (o *zoneOverlay) makeFont() {
	dpi, _, _ := procGetDpiForWindow.Call(o.hwnd)
	if dpi == 0 {
		dpi = 96
	}
	face, _ := windows.UTF16PtrFromString("Segoe UI")
	height := -int32(14 * dpi / 96)
	o.font, _, _ = procCreateFontW.Call(uintptr(height), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
}

func (o *zoneOverlay) free() {
	o.clean.free()
	o.dim.free()
	o.back.free()
	for _, h := range []uintptr{o.font, o.accent, o.label} {
		if h != 0 {
			procDeleteObject.Call(h)
		}
	}
}

func lparamXY(l uintptr) (int, int) {
	return int(int16(l & 0xFFFF)), int(int16((l >> 16) & 0xFFFF))
}

func zoneWndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	o := activeZone
	if o == nil || (o.hwnd != 0 && hwnd != o.hwnd) {
		r, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
		return r
	}
	switch msg {
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		o.paint(hwnd)
		return 0
	case wmMouseMove:
		o.mx, o.my = lparamXY(lparam)
		procInvalidateRect.Call(hwnd, 0, 0)
		return 0
	case wmLButtonDown:
		o.x0, o.y0 = lparamXY(lparam)
		o.mx, o.my = o.x0, o.y0
		o.dragging = true
		procSetCapture.Call(hwnd)
		return 0
	case wmLButtonUp:
		if !o.dragging {
			return 0
		}
		o.dragging = false
		procReleaseCapture.Call()
		x, y := lparamXY(lparam)
		if r := normalizeZone(o.x0, o.y0, x, y, o.bounds); zoneLargeEnough(r) {
			o.finish(r)
		} else if win, ok := windowAt(o.wins, o.x0, o.y0, o.bounds); ok {
			o.finish(win) // a click takes the window under it
		} else {
			procInvalidateRect.Call(hwnd, 0, 0) // a click on the desktop: keep waiting
		}
		return 0
	case wmRButtonUp:
		if o.dragging { // right-click while dragging drops the selection
			o.dragging = false
			procReleaseCapture.Call()
			procInvalidateRect.Call(hwnd, 0, 0)
		} else {
			procDestroyWindow.Call(hwnd)
		}
		return 0
	case wmKeyDown:
		switch wparam {
		case vkEscape:
			procDestroyWindow.Call(hwnd)
		case vkReturn: // the whole monitor
			o.finish(o.bounds)
		}
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}

func (o *zoneOverlay) finish(r image.Rectangle) {
	o.result, o.selected = r, true
	procDestroyWindow.Call(o.hwnd)
}

func (o *zoneOverlay) paint(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	defer procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	w, h := o.bounds.Dx(), o.bounds.Dy()
	back := o.back.dc
	procBitBlt.Call(back, 0, 0, uintptr(w), uintptr(h), o.dim.dc, 0, 0, srcCopy)
	procSelectObject.Call(back, o.font)
	procSetBkMode.Call(back, transparent)
	procSetTextColor.Call(back, colorText)

	if o.dragging {
		if r := normalizeZone(o.x0, o.y0, o.mx, o.my, o.bounds); !r.Empty() {
			o.showZone(back, r)
		}
	} else {
		if o.mx >= 0 {
			// The window a click would take, then crosshair guides for
			// precise aiming of a drag.
			if win, ok := windowAt(o.wins, o.mx, o.my, o.bounds); ok {
				o.showZone(back, win)
			}
			o.fill(back, image.Rect(o.mx, 0, o.mx+1, h))
			o.fill(back, image.Rect(0, o.my, w, o.my+1))
		}
		o.badge(back, "Drag to select  ·  Click: window  ·  Enter: whole screen  ·  Esc: cancel", w/2, 24, false)
	}
	procBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), back, 0, 0, srcCopy)
}

// showZone draws r undimmed with a border and its size in pixels.
func (o *zoneOverlay) showZone(dc uintptr, r image.Rectangle) {
	procBitBlt.Call(dc, uintptr(r.Min.X), uintptr(r.Min.Y), uintptr(r.Dx()), uintptr(r.Dy()),
		o.clean.dc, uintptr(r.Min.X), uintptr(r.Min.Y), srcCopy)
	o.frame(dc, r)
	o.badge(dc, fmt.Sprintf("%d × %d", r.Dx(), r.Dy()), r.Min.X, r.Min.Y, true)
}

func (o *zoneOverlay) fill(dc uintptr, r image.Rectangle) {
	rc := rect{int32(r.Min.X), int32(r.Min.Y), int32(r.Max.X), int32(r.Max.Y)}
	procFillRect.Call(dc, uintptr(unsafe.Pointer(&rc)), o.accent)
}

// frame draws a 2 px border just outside r so it never covers selected pixels.
func (o *zoneOverlay) frame(dc uintptr, r image.Rectangle) {
	const t = 2
	o.fill(dc, image.Rect(r.Min.X-t, r.Min.Y-t, r.Max.X+t, r.Min.Y))
	o.fill(dc, image.Rect(r.Min.X-t, r.Max.Y, r.Max.X+t, r.Max.Y+t))
	o.fill(dc, image.Rect(r.Min.X-t, r.Min.Y, r.Min.X, r.Max.Y))
	o.fill(dc, image.Rect(r.Max.X, r.Min.Y, r.Max.X+t, r.Max.Y))
}

// badge draws text on a dark pill. With above it sits on top of (x, y), else
// it is centred horizontally on x with its top at y.
func (o *zoneOverlay) badge(dc uintptr, text string, x, y int, above bool) {
	s, _ := windows.UTF16FromString(text)
	var m rect
	procDrawTextW.Call(dc, uintptr(unsafe.Pointer(&s[0])), uintptr(len(s)-1), uintptr(unsafe.Pointer(&m)), dtCalcRect|dtSingleLine)
	pad := int32(8)
	bw, bh := m.Right-m.Left+2*pad, m.Bottom-m.Top+pad
	var r rect
	if above {
		r.Left, r.Top = int32(x), int32(y)-bh-6
		if r.Top < 0 { // no room above the selection: put it inside
			r.Top = int32(y) + 6
		}
	} else {
		r.Left, r.Top = int32(x)-bw/2, int32(y)
	}
	r.Right, r.Bottom = r.Left+bw, r.Top+bh
	procFillRect.Call(dc, uintptr(unsafe.Pointer(&r)), o.label)
	procDrawTextW.Call(dc, uintptr(unsafe.Pointer(&s[0])), uintptr(len(s)-1), uintptr(unsafe.Pointer(&r)), dtCenter|dtVCenter|dtSingleLine)
}

// CaptureZone freezes the monitor under the cursor and returns the zone the
// user selects on it.
func (s *serviceImpl) CaptureZone() (*model.CaptureResult, *model.ZoneMemory, error) {
	pix, w, _, r, mr, err := freezeAndSelect()
	if err != nil {
		return nil, nil, err
	}
	res, err := encodeResult(cropBGRA(pix, w, r), r.Dx(), r.Dy())
	if err != nil {
		return nil, nil, err
	}
	return res, remember(monitorBounds(mr), rectToPoints(r)), nil
}

// monitorBounds is a monitor rectangle in virtual-screen pixels as a pointRect.
func monitorBounds(mr rect) pointRect {
	return pointRect{float64(mr.Left), float64(mr.Top), float64(mr.Right - mr.Left), float64(mr.Bottom - mr.Top)}
}

func rectToPoints(r image.Rectangle) pointRect {
	return pointRect{float64(r.Min.X), float64(r.Min.Y), float64(r.Dx()), float64(r.Dy())}
}

// CaptureZoneRect captures a remembered zone without the overlay. The monitor
// is found by its rectangle among the live monitors, captured, and the zone is
// cropped from it, as the overlay path does.
func (s *serviceImpl) CaptureZoneRect(zone model.ZoneMemory) (*model.CaptureResult, error) {
	mons, err := monitorRects()
	if err != nil {
		return nil, err
	}
	bounds := make([]pointRect, len(mons))
	for i, mr := range mons {
		bounds[i] = monitorBounds(mr)
	}
	idx, err := resolveZone(zone, bounds)
	if err != nil {
		return nil, err
	}
	pix, w, h, err := captureRect(mons[idx])
	if err != nil {
		return nil, fmt.Errorf("failed to capture screenshot: %w", err)
	}
	r := image.Rect(int(zone.X), int(zone.Y), int(zone.X+zone.Width), int(zone.Y+zone.Height)).
		Intersect(image.Rect(0, 0, w, h))
	if !zoneLargeEnough(r) {
		return nil, ErrZoneUnavailable
	}
	return encodeResult(cropBGRA(pix, w, r), r.Dx(), r.Dy())
}

// SelectRegion returns a zone selected on the frozen monitor, in physical
// virtual-screen pixels.
func (s *serviceImpl) SelectRegion() (*model.CropRegion, error) {
	_, _, _, r, mr, err := freezeAndSelect()
	if err != nil {
		return nil, err
	}
	return &model.CropRegion{X: int(mr.Left) + r.Min.X, Y: int(mr.Top) + r.Min.Y, Width: r.Dx(), Height: r.Dy()}, nil
}

func freezeAndSelect() (pix []byte, w, h int, zone image.Rectangle, mr rect, err error) {
	if mr, err = cursorMonitorRect(); err != nil {
		return
	}
	if pix, w, h, err = captureRect(mr); err != nil {
		err = fmt.Errorf("failed to capture screenshot: %w", err)
		return
	}
	zone, err = selectZone(pix, w, h, mr, windowTargets(mr))
	return
}
