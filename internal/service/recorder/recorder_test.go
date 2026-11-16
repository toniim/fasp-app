package recorder

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		o    Options
		ok   bool
	}{
		{"display defaults", Options{Mode: ModeDisplay, Output: "a.mp4"}, true},
		{"upper-case ext", Options{Mode: ModeDisplay, Output: `C:\x\A.MP4`}, true},
		{"no output", Options{Mode: ModeDisplay}, false},
		{"wrong ext", Options{Mode: ModeDisplay, Output: "a.mov"}, false},
		{"window without handle", Options{Mode: ModeWindow, Output: "a.mp4"}, false},
		{"tiny region", Options{Mode: ModeRegion, Output: "a.mp4", Region: Rect{Width: 1, Height: 10}}, false},
		{"region", Options{Mode: ModeRegion, Output: "a.mp4", Region: Rect{Width: 100, Height: 10}}, true},
		{"fps too high", Options{Mode: ModeDisplay, Output: "a.mp4", FPS: 240}, false},
		{"unknown mode", Options{Mode: "tab", Output: "a.mp4"}, false},
	}
	for _, c := range cases {
		err := c.o.normalize()
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}
		if err == nil && c.o.FPS != 30 && c.name == "display defaults" {
			t.Errorf("default fps = %d, want 30", c.o.FPS)
		}
	}
}

func TestBitrateFor(t *testing.T) {
	if got := bitrateFor(1920, 1080, 30, 0); got != 6_220_800 {
		t.Errorf("1080p30 = %d", got)
	}
	if got := bitrateFor(160, 120, 30, 0); got != 500_000 {
		t.Errorf("small frames should use the floor, got %d", got)
	}
	if got := bitrateFor(7680, 4320, 60, 0); got != 40_000_000 {
		t.Errorf("8K60 should be capped, got %d", got)
	}
	if got := bitrateFor(1920, 1080, 30, 1234); got != 1234 {
		t.Errorf("explicit bitrate ignored, got %d", got)
	}
}

func TestEven(t *testing.T) {
	for in, want := range map[int]int{0: 0, 1: 0, 2: 2, 1125: 1124} {
		if got := even(in); got != want {
			t.Errorf("even(%d) = %d, want %d", in, got, want)
		}
	}
}
