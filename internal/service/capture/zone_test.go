package capture

import (
	"image"
	"testing"
)

func TestNormalizeZone(t *testing.T) {
	b := image.Rect(0, 0, 100, 50)
	cases := []struct {
		x0, y0, x1, y1 int
		want           image.Rectangle
	}{
		{10, 10, 30, 20, image.Rect(10, 10, 30, 20)},
		{30, 20, 10, 10, image.Rect(10, 10, 30, 20)},   // dragged up-left
		{90, 40, 200, 90, image.Rect(90, 40, 100, 50)}, // dragged past the edge
		{-5, -5, 5, 5, image.Rect(0, 0, 5, 5)},
	}
	for _, c := range cases {
		if got := normalizeZone(c.x0, c.y0, c.x1, c.y1, b); got != c.want {
			t.Errorf("normalizeZone(%d,%d,%d,%d) = %v, want %v", c.x0, c.y0, c.x1, c.y1, got, c.want)
		}
	}
	if zoneLargeEnough(image.Rect(0, 0, 3, 100)) {
		t.Error("a 3 px wide zone should be rejected")
	}
	if !zoneLargeEnough(image.Rect(0, 0, 4, 4)) {
		t.Error("a 4×4 zone should be accepted")
	}
}

// A Retina display (scale 2) sitting left of and above the main display has a
// negative Quartz origin; the selection is local to that display's top-left.
func TestZoneGeometry(t *testing.T) {
	img := image.Rect(0, 0, 2880, 1800) // 1440x900 points at 2x
	sel := pointRect{X: 10.25, Y: 20, W: 100.5, H: 50}

	if got, want := zonePixelRect(sel, 2, 2, img), image.Rect(20, 40, 222, 140); got != want {
		t.Errorf("zonePixelRect = %v, want %v", got, want)
	}
	// Whole screen maps to the whole image; an overshoot is clamped.
	if got := zonePixelRect(pointRect{0, 0, 1440, 900}, 2, 2, img); got != img {
		t.Errorf("whole screen = %v, want %v", got, img)
	}
	if got, want := zonePixelRect(pointRect{1400, 880, 100, 100}, 2, 2, img), image.Rect(2800, 1760, 2880, 1800); got != want {
		t.Errorf("clamped = %v, want %v", got, want)
	}

	x, y, w, h := zoneGlobalPoints(sel, -1440, -900)
	if x != -1430 || y != -880 || w != 101 || h != 50 {
		t.Errorf("zoneGlobalPoints = %d,%d %dx%d, want -1430,-880 101x50", x, y, w, h)
	}
	x, y, w, h = zoneGlobalPoints(pointRect{0, 0, 1440, 900}, 0, 0)
	if x != 0 || y != 0 || w != 1440 || h != 900 {
		t.Errorf("main display whole screen = %d,%d %dx%d", x, y, w, h)
	}
}

func TestCropBGRA(t *testing.T) {
	const w, h = 4, 3
	pix := make([]byte, w*h*4)
	for i := range pix {
		pix[i] = byte(i / 4) // pixel index in every channel
	}
	got := cropBGRA(pix, w, image.Rect(1, 1, 3, 3))
	want := []byte{5, 6, 9, 10} // pixels (1,1) (2,1) (1,2) (2,2)
	if len(got) != len(want)*4 {
		t.Fatalf("len = %d", len(got))
	}
	for i, p := range want {
		if got[i*4] != p {
			t.Errorf("pixel %d = %d, want %d", i, got[i*4], p)
		}
	}
}
