//go:build windows

package recorder

import (
	"errors"
	"unsafe"
)

// Windows.Graphics.Capture: a GraphicsCaptureItem (monitor or window) feeds
// a free-threaded Direct3D11CaptureFramePool through a GraphicsCaptureSession.
// Frames are pulled with TryGetNextFrame, which avoids implementing a WinRT
// FrameArrived delegate in Go.

const (
	// IGraphicsCaptureItemInterop (IUnknown-based)
	interopCreateForWindow  = 3
	interopCreateForMonitor = 4
	// IGraphicsCaptureItem
	itemGetSize = 7
	// IDirect3D11CaptureFramePoolStatics2
	poolStaticsCreateFreeThreaded = 6
	// IDirect3D11CaptureFramePool
	poolRecreate             = 6
	poolTryGetNextFrame      = 7
	poolCreateCaptureSession = 10
	// IGraphicsCaptureSession / 2 / 3
	sessionStartCapture      = 6
	sessionPutCursorEnabled  = 7
	sessionPutBorderRequired = 7
	// IDirect3D11CaptureFrame
	frameGetSurface     = 6
	frameGetContentSize = 8
	// IDirect3DDxgiInterfaceAccess
	dxgiAccessGetInterface = 3

	poolBuffers = 2
)

type sizeInt32 struct{ Width, Height int32 }

// packed passes a SizeInt32 by value: an 8-byte struct travels in one
// register on amd64 and arm64.
func (s sizeInt32) packed() uintptr {
	return uintptr(uint64(uint32(s.Width)) | uint64(uint32(s.Height))<<32)
}

func newCaptureItem(handle uintptr, window bool) (unsafe.Pointer, error) {
	interop, err := activationFactory("Windows.Graphics.Capture.GraphicsCaptureItem", &iidIGraphicsCaptureItemInterop)
	if err != nil {
		return nil, err
	}
	defer release(interop)
	method, what := interopCreateForMonitor, "CreateForMonitor"
	if window {
		method, what = interopCreateForWindow, "CreateForWindow"
	}
	var item unsafe.Pointer
	if r := comCall(interop, method, handle, uintptr(unsafe.Pointer(&iidIGraphicsCaptureItem)), uintptr(unsafe.Pointer(&item))); failed(r) {
		return nil, hresultErr(what, r)
	}
	return item, nil
}

func itemSize(item unsafe.Pointer) (sizeInt32, error) {
	var s sizeInt32
	if r := comCall(item, itemGetSize, uintptr(unsafe.Pointer(&s))); failed(r) {
		return s, hresultErr("GraphicsCaptureItem.Size", r)
	}
	return s, nil
}

type wgcCapture struct {
	dev     *d3dDevice
	pool    unsafe.Pointer
	session unsafe.Pointer
	size    sizeInt32 // current frame pool size
}

func startWGC(dev *d3dDevice, item unsafe.Pointer, cursor bool) (*wgcCapture, error) {
	size, err := itemSize(item)
	if err != nil {
		return nil, err
	}
	statics, err := activationFactory("Windows.Graphics.Capture.Direct3D11CaptureFramePool", &iidIDirect3D11CaptureFramePoolStatic2)
	if err != nil {
		return nil, err
	}
	defer release(statics)

	c := &wgcCapture{dev: dev, size: size}
	if r := comCall(statics, poolStaticsCreateFreeThreaded, uintptr(dev.winrt), dxgiFormatB8G8R8A8, poolBuffers,
		size.packed(), uintptr(unsafe.Pointer(&c.pool))); failed(r) {
		return nil, hresultErr("CreateFreeThreaded", r)
	}
	if r := comCall(c.pool, poolCreateCaptureSession, uintptr(item), uintptr(unsafe.Pointer(&c.session))); failed(r) {
		c.close()
		return nil, hresultErr("CreateCaptureSession", r)
	}

	// Optional knobs; older Windows builds lack these interfaces.
	if s2, err := queryInterface(c.session, &iidIGraphicsCaptureSession2); err == nil { // Win10 2004+
		comCall(s2, sessionPutCursorEnabled, boolArg(cursor))
		release(s2)
	}
	if s3, err := queryInterface(c.session, &iidIGraphicsCaptureSession3); err == nil { // Win11 / Win10 21H1+
		// Unpackaged desktop apps may drop the yellow capture border.
		comCall(s3, sessionPutBorderRequired, 0)
		release(s3)
	}

	if r := comCall(c.session, sessionStartCapture); failed(r) {
		c.close()
		return nil, hresultErr("StartCapture", r)
	}
	return c, nil
}

// latestFrame drains the pool and returns only the newest frame (nil if no
// new frame arrived) plus the number of frames it consumed. Older frames are
// released at once so the small pool never stalls.
func (c *wgcCapture) latestFrame() (unsafe.Pointer, int) {
	var latest unsafe.Pointer
	n := 0
	for {
		var f unsafe.Pointer
		if r := comCall(c.pool, poolTryGetNextFrame, uintptr(unsafe.Pointer(&f))); failed(r) || f == nil {
			return latest, n
		}
		release(latest)
		latest = f
		n++
	}
}

// frameTexture returns the frame's ID3D11Texture2D (caller releases) and the
// size of the content inside it, which differs from the texture size while a
// window is being resized.
func frameTexture(frame unsafe.Pointer) (unsafe.Pointer, sizeInt32, error) {
	var content sizeInt32
	comCall(frame, frameGetContentSize, uintptr(unsafe.Pointer(&content)))
	var surface unsafe.Pointer
	if r := comCall(frame, frameGetSurface, uintptr(unsafe.Pointer(&surface))); failed(r) {
		return nil, content, hresultErr("Frame.Surface", r)
	}
	defer release(surface)
	access, err := queryInterface(surface, &iidIDirect3DDxgiInterfaceAccess)
	if err != nil {
		return nil, content, err
	}
	defer release(access)
	var tex unsafe.Pointer
	if r := comCall(access, dxgiAccessGetInterface, uintptr(unsafe.Pointer(&iidID3D11Texture2D)), uintptr(unsafe.Pointer(&tex))); failed(r) {
		return nil, content, hresultErr("GetInterface(ID3D11Texture2D)", r)
	}
	if tex == nil {
		return nil, content, errors.New("frame has no texture")
	}
	return tex, content, nil
}

// resize recreates the pool at a new size after the captured window resized.
func (c *wgcCapture) resize(size sizeInt32) error {
	if r := comCall(c.pool, poolRecreate, uintptr(c.dev.winrt), dxgiFormatB8G8R8A8, poolBuffers, size.packed()); failed(r) {
		return hresultErr("FramePool.Recreate", r)
	}
	c.size = size
	return nil
}

func (c *wgcCapture) close() {
	closeWinRT(c.session)
	release(c.session)
	closeWinRT(c.pool)
	release(c.pool)
	c.session, c.pool = nil, nil
}

func boolArg(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}
