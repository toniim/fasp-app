//go:build windows

package capture

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"time"
	"unsafe"

	"github.com/heytonyne/fasp/internal/model"
)

// serviceImpl implements the Service interface for Windows
type serviceImpl struct{}

// New creates a new capture service instance
func New() Service {
	return &serviceImpl{}
}

// CaptureFullscreen captures the entire screen of the active display
func (s *serviceImpl) CaptureFullscreen() (*model.CaptureResult, error) {
	return s.CaptureActiveDisplay()
}

// CaptureActiveDisplay captures the monitor the mouse cursor is on with GDI,
// in-process. The app manifest declares per-monitor v2 DPI awareness, so the
// monitor rectangle is in physical pixels.
func (s *serviceImpl) CaptureActiveDisplay() (*model.CaptureResult, error) {
	pix, width, height, err := captureCursorMonitor()
	if err != nil {
		return nil, fmt.Errorf("failed to capture screenshot: %w", err)
	}

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, bgraToRGBA(pix, width, height)); err != nil {
		return nil, fmt.Errorf("failed to encode screenshot: %w", err)
	}

	return &model.CaptureResult{
		Data:      base64.StdEncoding.EncodeToString(buf.Bytes()),
		Width:     width,
		Height:    height,
		Timestamp: time.Now(),
	}, nil
}

// captureCursorMonitor copies the monitor under the cursor into a top-down
// 32 bpp BGRA buffer.
func captureCursorMonitor() ([]byte, int, int, error) {
	var pt point
	if r, _, e := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); r == 0 {
		return nil, 0, 0, fmt.Errorf("GetCursorPos: %w", e)
	}
	monitor := monitorFromPoint(pt)
	mi := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	if r, _, e := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&mi))); r == 0 {
		return nil, 0, 0, fmt.Errorf("GetMonitorInfoW: %w", e)
	}
	left, top := mi.RcMonitor.Left, mi.RcMonitor.Top
	width := int(mi.RcMonitor.Right - left)
	height := int(mi.RcMonitor.Bottom - top)
	if width <= 0 || height <= 0 {
		return nil, 0, 0, errors.New("monitor has an empty area")
	}

	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return nil, 0, 0, errors.New("GetDC failed")
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, 0, 0, errors.New("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(memDC)

	bitmap, _, _ := procCreateCompatibleBitmap.Call(screenDC, uintptr(width), uintptr(height))
	if bitmap == 0 {
		return nil, 0, 0, errors.New("CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(bitmap)

	old, _, _ := procSelectObject.Call(memDC, bitmap)
	if r, _, e := procBitBlt.Call(memDC, 0, 0, uintptr(width), uintptr(height),
		screenDC, uintptr(left), uintptr(top), srcCopy|captureBlt); r == 0 {
		procSelectObject.Call(memDC, old)
		return nil, 0, 0, fmt.Errorf("BitBlt: %w", e)
	}
	// GetDIBits needs the bitmap deselected from the DC.
	procSelectObject.Call(memDC, old)

	bi := bitmapInfo{Header: bitmapInfoHeader{
		BiSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		BiWidth:       int32(width),
		BiHeight:      -int32(height), // negative: top-down rows
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: biRGB,
	}}
	pix := make([]byte, width*height*4)
	if r, _, _ := procGetDIBits.Call(memDC, bitmap, 0, uintptr(height),
		uintptr(unsafe.Pointer(&pix[0])), uintptr(unsafe.Pointer(&bi)), dibRGBColors); r == 0 {
		return nil, 0, 0, errors.New("GetDIBits failed")
	}
	return pix, width, height, nil
}

// monitorFromPoint calls MonitorFromPoint, which takes the POINT by value: on
// 64-bit Windows the 8-byte struct travels packed in one register, on 32-bit
// it is pushed as two stack slots.
func monitorFromPoint(pt point) uintptr {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		packed := uint64(uint32(pt.X)) | uint64(uint32(pt.Y))<<32
		m, _, _ := procMonitorFromPoint.Call(uintptr(packed), monitorDefaultToPrimary)
		return m
	}
	m, _, _ := procMonitorFromPoint.Call(uintptr(uint32(pt.X)), uintptr(uint32(pt.Y)), monitorDefaultToPrimary)
	return m
}

// CaptureAllDisplays captures all connected displays
func (s *serviceImpl) CaptureAllDisplays() ([]*model.DisplayCapture, error) {
	// For MVP, just capture the main display
	result, err := s.CaptureActiveDisplay()
	if err != nil {
		return nil, err
	}

	// Decode base64 to bytes
	data, err := base64.StdEncoding.DecodeString(result.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	return []*model.DisplayCapture{
		{
			DisplayID: 0,
			Data:      data,
			Width:     result.Width,
			Height:    result.Height,
		},
	}, nil
}

// GetDisplayInfo returns information about all connected displays
func (s *serviceImpl) GetDisplayInfo() ([]*model.DisplayInfo, error) {
	// For MVP, return single display info
	return []*model.DisplayInfo{
		{
			ID:     0,
			Name:   "Primary Display",
			Width:  0, // Will be determined on capture
			Height: 0,
			X:      0,
			Y:      0,
		},
	}, nil
}
