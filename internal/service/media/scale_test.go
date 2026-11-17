package media

import "testing"

func TestScalerKeepsColoursWithUnusedAlpha(t *testing.T) {
	s := newScaler(64, 36, 16, 9)
	for y := 0; y < 36; y++ {
		row := s.row(y)
		for x := 0; x < 64; x++ {
			row[x*4], row[x*4+1], row[x*4+2], row[x*4+3] = 10, 120, 240, 0 // BGRA with alpha 0
		}
	}
	out := s.scale()
	if len(out) != 16*9*4 {
		t.Fatalf("%d bytes, want %d", len(out), 16*9*4)
	}
	for i := 0; i < len(out); i += 4 {
		if out[i] != 10 || out[i+1] != 120 || out[i+2] != 240 {
			t.Fatalf("pixel %d = %v, want 10,120,240", i/4, out[i:i+3])
		}
	}
}

func TestScalerSameSizePassesThrough(t *testing.T) {
	s := newScaler(8, 8, 8, 8)
	s.row(3)[5] = 77
	if out := s.scale(); len(out) != 8*8*4 || out[3*32+5] != 77 {
		t.Error("same-size frame was changed")
	}
}
