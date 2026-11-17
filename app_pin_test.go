package main

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestPinSizeFitsScreenWithoutEnlarging(t *testing.T) {
	area := application.Rect{Width: 1000, Height: 800}

	// Small image on a Retina screen: natural size in points, not enlarged.
	w, h := pinSize(400, 200, 2, area)
	if w != 200 || h != 100 {
		t.Errorf("natural size = %vx%v, want 200x100", w, h)
	}

	// Large image: shrunk to 80% of the work area, aspect ratio kept.
	w, h = pinSize(4000, 1000, 1, area)
	if w != 800 || h != 200 {
		t.Errorf("fitted size = %vx%v, want 800x200", w, h)
	}
	w, h = pinSize(1000, 4000, 1, area)
	if w != 160 || h != 640 {
		t.Errorf("fitted tall size = %vx%v, want 160x640", w, h)
	}
}

func TestCentreInStaysInsideWorkArea(t *testing.T) {
	area := application.Rect{X: 0, Y: 25, Width: 1000, Height: 775}
	if x, y := centreIn(200, 100, 500, 400, area); x != 400 || y != 350 {
		t.Errorf("centred = %d,%d, want 400,350", x, y)
	}
	// A centre near the corner is pulled back so the window is fully visible.
	if x, y := centreIn(200, 100, 990, 790, area); x != 800 || y != 700 {
		t.Errorf("clamped = %d,%d, want 800,700", x, y)
	}
	if x, y := centreIn(200, 100, 0, 0, area); x != 0 || y != 25 {
		t.Errorf("clamped = %d,%d, want 0,25", x, y)
	}
}

func TestZoomPinKeepsCursorPointFixed(t *testing.T) {
	cur := application.Rect{X: 100, Y: 200, Width: 400, Height: 200}
	// Zoom in 2x with the cursor at (100, 50) inside the window.
	r, w, h := zoomPin(cur, 400, 200, 400, 200, 2, 100, 50)
	if w != 800 || h != 400 || r.Width != 800 || r.Height != 400 {
		t.Fatalf("size = %vx%v (%dx%d), want 800x400", w, h, r.Width, r.Height)
	}
	// The window point under the cursor: old screen pos = 100+100, 200+50.
	// New pos must put the doubled offset (200, 100) at the same screen point.
	if r.X+200 != 200 || r.Y+100 != 250 {
		t.Errorf("cursor drifted: window at %d,%d", r.X, r.Y)
	}
}

func TestZoomPinClampsRange(t *testing.T) {
	cur := application.Rect{Width: 400, Height: 200}
	_, w, _ := zoomPin(cur, 400, 200, 400, 200, 1000, 0, 0)
	if w != 400*pinMaxZoom {
		t.Errorf("max zoom width = %v, want %v", w, 400*pinMaxZoom)
	}
	// The 80 px floor applies to the shorter side: 200 tall -> ratio 0.4.
	_, w, h := zoomPin(cur, 400, 200, 400, 200, 0.0001, 0, 0)
	if w != 160 || h != pinMinSide {
		t.Errorf("min zoom = %vx%v, want 160x%d", w, h, pinMinSide)
	}
}

func TestPinImageStoreById(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	a.pins.add(&pin{id: "abc", png: buf.Bytes()})

	got, err := a.GetPinImage("abc")
	if err != nil || !bytes.Equal(got, buf.Bytes()) {
		t.Fatalf("GetPinImage = %d bytes, %v", len(got), err)
	}
	if _, err := a.GetPinImage("other"); err == nil {
		t.Fatal("an unknown pin id must fail")
	}

	// Closing forgets the image; a pin without a window (never opened) is fine.
	if err := a.ClosePin("abc"); err != nil {
		t.Fatalf("ClosePin: %v", err)
	}
	if _, err := a.GetPinImage("abc"); err == nil {
		t.Fatal("a closed pin must not serve its image")
	}
	if err := a.ZoomPin("abc", 2, 0, 0); err == nil {
		t.Fatal("zooming a closed pin must fail")
	}
}

func TestPinImageRejectsNonPNG(t *testing.T) {
	a := NewApp()
	if _, err := a.PinImage([]byte("not a png")); err == nil {
		t.Fatal("garbage must not open a pin")
	}
}
