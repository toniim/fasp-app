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
