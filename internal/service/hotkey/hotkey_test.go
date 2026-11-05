package hotkey

import (
	"maps"
	"testing"

	"golang.design/x/hotkey"
)

// parseKey must return the library's code for the running platform; macOS
// codes are not sequential, so offsets from KeyA/Key0 register the wrong key.
func TestParseKeyUsesPlatformCodes(t *testing.T) {
	cases := map[string]hotkey.Key{
		"a": hotkey.KeyA,
		"s": hotkey.KeyS,
		"z": hotkey.KeyZ,
		"0": hotkey.Key0,
		"5": hotkey.Key5,
		"6": hotkey.Key6,
	}
	maps.Copy(cases, platformKeys)
	for name, want := range cases {
		got, err := parseKey(name)
		if err != nil {
			t.Fatalf("parseKey(%q): %v", name, err)
		}
		if got != want {
			t.Errorf("parseKey(%q) = %v, want %v", name, got, want)
		}
	}
}
