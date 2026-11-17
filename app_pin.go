package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"math"
	"sync"

	"github.com/heytonyne/fasp/internal/model"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	// pinScreenFit is the share of the screen a pin may fill when it opens.
	pinScreenFit = 0.8
	// pinMinSide and pinMaxZoom keep a pin from shrinking to nothing or growing
	// past anything useful; zoom is relative to the size the pin opened at.
	pinMinSide = 80
	pinMinZoom = 0.1
	pinMaxZoom = 8.0
)

// pin is one image floating on the screen in its own window.
type pin struct {
	id     string
	png    []byte
	window *application.WebviewWindow

	// baseW/baseH is the 100% size in window units; curW/curH the current one.
	// Zoom is computed from this state, not read back from the window, so quick
	// wheel events do not depend on how fast the window reports its new size.
	mu         sync.Mutex
	baseW      float64
	baseH      float64
	curW, curH float64
}

// pinStore holds the image of every open pin by id. The pin window fetches its
// image from here, so the image never travels in a URL.
type pinStore struct {
	mu   sync.Mutex
	pins map[string]*pin
}

func newPinStore() *pinStore {
	return &pinStore{pins: map[string]*pin{}}
}

func (s *pinStore) add(p *pin) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pins[p.id] = p
}

func (s *pinStore) get(id string) (*pin, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pins[id]
	return p, ok
}

func (s *pinStore) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pins, id)
}

// newRandomID returns an unguessable id for a pin, a job or a notification.
func newRandomID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// pinSize returns the window size, in window units, a pin of imgW x imgH
// pixels opens at: the natural size on a screen with the given scale factor,
// shrunk (never enlarged) to fit within pinScreenFit of the work area.
func pinSize(imgW, imgH int, scale float64, area application.Rect) (w, h float64) {
	if scale <= 0 {
		scale = 1
	}
	w, h = float64(imgW)/scale, float64(imgH)/scale
	fit := math.Min(float64(area.Width)*pinScreenFit/w, float64(area.Height)*pinScreenFit/h)
	if fit < 1 {
		w, h = w*fit, h*fit
	}
	return math.Max(w, 1), math.Max(h, 1)
}

// centreIn places a w x h window so its centre sits on (cx, cy), then pulls it
// back inside the work area.
func centreIn(w, h, cx, cy float64, area application.Rect) (x, y int) {
	fx := math.Min(math.Max(cx-w/2, float64(area.X)), float64(area.X+area.Width)-w)
	fy := math.Min(math.Max(cy-h/2, float64(area.Y)), float64(area.Y+area.Height)-h)
	return int(math.Round(fx)), int(math.Round(fy))
}

// zoomPin returns the window bounds after scaling a pin from its current
// bounds cur by factor, keeping the point (ax, ay), given relative to the
// window's top-left corner, fixed on screen. The new size is clamped to the
// zoom range around the 100% size base and the new current size is returned.
func zoomPin(cur application.Rect, curW, curH, baseW, baseH, factor, ax, ay float64) (r application.Rect, newW, newH float64) {
	minRatio := math.Max(pinMinZoom, math.Max(pinMinSide/baseW, pinMinSide/baseH))
	ratio := math.Min(math.Max(curW*factor/baseW, minRatio), pinMaxZoom)
	newW, newH = baseW*ratio, baseH*ratio
	k := newW / curW
	r = application.Rect{
		X:      int(math.Round(float64(cur.X) + ax*(1-k))),
		Y:      int(math.Round(float64(cur.Y) + ay*(1-k))),
		Width:  int(math.Round(newW)),
		Height: int(math.Round(newH)),
	}
	return r, newW, newH
}

// PinImage opens the image (PNG bytes) in a borderless, always-on-top window and
// returns the pin id. Each call opens a new pin; pins live on their own, so
// closing or hiding the editor does not close them.
func (a *App) PinImage(data []byte) (string, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("the image cannot be pinned: %w", err)
	}
	id, err := newRandomID()
	if err != nil {
		return "", err
	}

	area, scale, cx, cy := a.pinPlacement()
	w, h := pinSize(cfg.Width, cfg.Height, scale, area)
	x, y := centreIn(w, h, cx, cy, area)

	p := &pin{id: id, png: data, baseW: w, baseH: h, curW: w, curH: h}
	a.pins.add(p)

	win := a.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "pin-" + id,
		Title:            "Pinned image",
		Width:            int(math.Round(w)),
		Height:           int(math.Round(h)),
		X:                x,
		Y:                y,
		InitialPosition:  application.WindowXY,
		URL:              "/?pin=" + id,
		Frameless:        true,
		AlwaysOnTop:      true,
		DisableResize:    true,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		// A pin is a floating note: no maximise/minimise/fullscreen buttons.
		MinimiseButtonState:   application.ButtonHidden,
		MaximiseButtonState:   application.ButtonHidden,
		FullscreenButtonState: application.ButtonHidden,
		Mac: application.MacWindow{
			WindowLevel: application.MacWindowLevelFloating,
			// On every Space, and over full-screen apps.
			CollectionBehavior: application.MacWindowCollectionBehaviorCanJoinAllSpaces |
				application.MacWindowCollectionBehaviorFullScreenAuxiliary |
				application.MacWindowCollectionBehaviorStationary,
			CornerType: application.MacWindowCornerTypeSquare,
		},
	})
	p.window = win
	// The window can also close from outside ClosePin (Cmd+W, quitting).
	win.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) { a.pins.remove(id) })
	return id, nil
}

// pinPlacement returns the work area of the screen the editor is on (the
// primary screen when the editor is hidden), its scale factor and the point a
// new pin is centred on: the centre of the editor window when it is showing,
// else the centre of the screen.
func (a *App) pinPlacement() (area application.Rect, scale, cx, cy float64) {
	screen := a.app.Screen.GetPrimary()
	if a.window != nil && a.window.IsVisible() {
		if s, err := a.window.GetScreen(); err == nil && s != nil {
			screen = s
		}
	}
	area = application.Rect{X: 0, Y: 0, Width: 1280, Height: 800}
	scale = 1
	if screen != nil {
		area, scale = screen.WorkArea, float64(screen.ScaleFactor)
	}
	cx, cy = float64(area.X)+float64(area.Width)/2, float64(area.Y)+float64(area.Height)/2
	if a.window != nil && a.window.IsVisible() && !a.window.IsMinimised() {
		b := a.window.Bounds()
		if b.Width > 0 && b.Height > 0 {
			cx, cy = float64(b.X)+float64(b.Width)/2, float64(b.Y)+float64(b.Height)/2
		}
	}
	return area, scale, cx, cy
}

func (a *App) pinByID(id string) (*pin, error) {
	p, ok := a.pins.get(id)
	if !ok {
		return nil, errors.New("this pin is no longer open")
	}
	return p, nil
}

// GetPinImage returns the PNG of a pin, for the pin window to draw.
func (a *App) GetPinImage(id string) ([]byte, error) {
	p, err := a.pinByID(id)
	if err != nil {
		return nil, err
	}
	return p.png, nil
}

// ZoomPin scales a pin by factor around the point (x, y), in CSS pixels from
// the pin window's top-left corner, so the part of the image under the cursor
// stays under the cursor. The pin window is resized to match.
func (a *App) ZoomPin(id string, factor, x, y float64) error {
	p, err := a.pinByID(id)
	if err != nil {
		return err
	}
	if factor <= 0 || math.IsNaN(factor) || math.IsInf(factor, 0) {
		return fmt.Errorf("invalid zoom factor %v", factor)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	r, w, h := zoomPin(p.window.Bounds(), p.curW, p.curH, p.baseW, p.baseH, factor, x, y)
	p.curW, p.curH = w, h
	p.window.SetBounds(r)
	return nil
}

// ResetPinZoom puts a pin back to 100% (the size it opened at) around (x, y).
func (a *App) ResetPinZoom(id string, x, y float64) error {
	p, err := a.pinByID(id)
	if err != nil {
		return err
	}
	p.mu.Lock()
	factor := p.baseW / p.curW
	p.mu.Unlock()
	return a.ZoomPin(id, factor, x, y)
}

// CopyPin copies the pinned image to the clipboard.
func (a *App) CopyPin(id string) error {
	p, err := a.pinByID(id)
	if err != nil {
		return err
	}
	return a.clipboardService.CopyImage(p.png)
}

// SavePinAs asks where to save the pinned image and writes it there. It returns
// the saved path, or "" when the dialog was cancelled.
func (a *App) SavePinAs(id string) (string, error) {
	p, err := a.pinByID(id)
	if err != nil {
		return "", err
	}
	path, err := a.fileService.OpenSaveDialog(a.fileService.GenerateFilename("png"))
	if err != nil || path == "" {
		return "", err
	}
	if err := a.fileService.SaveImage(&model.SaveOptions{Path: path, Format: "png", Quality: 90}, p.png); err != nil {
		return "", fmt.Errorf("failed to save %s: %w", path, err)
	}
	a.recordRecent(path)
	return path, nil
}

// ClosePin closes a pin window and forgets its image.
func (a *App) ClosePin(id string) error {
	p, err := a.pinByID(id)
	if err != nil {
		return err
	}
	a.pins.remove(id)
	if p.window != nil {
		p.window.Close()
	}
	return nil
}
