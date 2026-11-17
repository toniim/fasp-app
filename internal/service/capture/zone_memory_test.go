package capture

import (
	"errors"
	"testing"
)

// The display the zone was picked on is found by its bounds; a zone that no
// longer fits, or a display that is gone or has another size, is refused.
func TestResolveZone(t *testing.T) {
	main := pointRect{0, 0, 1440, 900}
	left := pointRect{-1920, 0, 1920, 1080}
	displays := []pointRect{main, left}

	// A zone picked on the left display, with half-point edges from a Retina drag.
	zone := remember(left, pointRect{10.5, 20, 300.5, 200})

	cases := []struct {
		name      string
		displays  []pointRect
		wantIndex int
		wantErr   bool
	}{
		{name: "same display", displays: displays, wantIndex: 1},
		{name: "display order does not matter", displays: []pointRect{left, main}, wantIndex: 0},
		{name: "display unplugged", displays: []pointRect{main}, wantErr: true},
		{name: "display has another size", displays: []pointRect{main, {-1920, 0, 1680, 1050}}, wantErr: true},
		{name: "display moved", displays: []pointRect{main, {-1920, 100, 1920, 1080}}, wantErr: true},
	}
	for _, c := range cases {
		got, err := resolveZone(*zone, c.displays)
		if c.wantErr {
			if !errors.Is(err, ErrZoneUnavailable) {
				t.Errorf("%s: err = %v, want ErrZoneUnavailable", c.name, err)
			}
			continue
		}
		if err != nil || got != c.wantIndex {
			t.Errorf("%s: index %d, err %v; want index %d", c.name, got, err, c.wantIndex)
		}
	}

	// Zones that are off the display are refused even when the display matches.
	for name, sel := range map[string]pointRect{
		"past the right edge":  {1700, 0, 400, 100},
		"past the bottom edge": {0, 1000, 100, 200},
		"negative origin":      {-50, 0, 100, 100},
		"empty":                {10, 10, 0, 50},
	} {
		z := remember(left, sel)
		if _, err := resolveZone(*z, displays); !errors.Is(err, ErrZoneUnavailable) {
			t.Errorf("%s: err = %v, want ErrZoneUnavailable", name, err)
		}
	}

	// The whole display is a valid zone (Enter in the overlay).
	if _, err := resolveZone(*remember(main, pointRect{0, 0, 1440, 900}), displays); err != nil {
		t.Errorf("whole display refused: %v", err)
	}
}
