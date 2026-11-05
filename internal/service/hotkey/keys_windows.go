//go:build windows

package hotkey

import "golang.design/x/hotkey"

// platformKeys maps key names to Windows virtual-key codes (VK_*).
var platformKeys = map[string]hotkey.Key{
	"printscreen":   0x2C, // VK_SNAPSHOT
	"prtsc":         0x2C,
	"prtscn":        0x2C,
	"print":         0x2C,
	"scrolllock":    0x91, // VK_SCROLL
	"pause":         0x13, // VK_PAUSE
	"help":          0x2F, // VK_HELP
	"insert":        0x2D, // VK_INSERT
	"home":          0x24, // VK_HOME
	"pageup":        0x21, // VK_PRIOR
	"forwarddelete": 0x2E, // VK_DELETE
	"del":           0x2E, // VK_DELETE
	"end":           0x23, // VK_END
	"pagedown":      0x22, // VK_NEXT
	"numlock":       0x90, // VK_NUMLOCK
}
