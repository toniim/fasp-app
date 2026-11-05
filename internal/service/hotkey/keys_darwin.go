//go:build darwin

package hotkey

import "golang.design/x/hotkey"

// platformKeys maps key names to macOS virtual key codes (Carbon Events.h).
// Mac keyboards have no Print Screen, Pause or Scroll Lock key: a PC keyboard
// plugged into a Mac reports them as F13, F15 and F14, and Insert as Help.
var platformKeys = map[string]hotkey.Key{
	"printscreen":   hotkey.KeyF13,
	"prtsc":         hotkey.KeyF13,
	"prtscn":        hotkey.KeyF13,
	"print":         hotkey.KeyF13,
	"scrolllock":    hotkey.KeyF14,
	"pause":         hotkey.KeyF15,
	"help":          0x72, // kVK_Help
	"insert":        0x72, // kVK_Help
	"home":          0x73, // kVK_Home
	"pageup":        0x74, // kVK_PageUp
	"forwarddelete": 0x75, // kVK_ForwardDelete
	"del":           0x75, // kVK_ForwardDelete
	"end":           0x77, // kVK_End
	"pagedown":      0x79, // kVK_PageDown
	"numlock":       0x47, // kVK_ANSI_KeypadClear
}
