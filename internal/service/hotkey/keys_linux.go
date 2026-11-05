//go:build linux && cgo

package hotkey

import "golang.design/x/hotkey"

// platformKeys maps key names to X11 keysyms (X11/keysymdef.h).
var platformKeys = map[string]hotkey.Key{
	"printscreen":   0xff61, // XK_Print
	"prtsc":         0xff61,
	"prtscn":        0xff61,
	"print":         0xff61,
	"scrolllock":    0xff14, // XK_Scroll_Lock
	"pause":         0xff13, // XK_Pause
	"help":          0xff6a, // XK_Help
	"insert":        0xff63, // XK_Insert
	"home":          0xff50, // XK_Home
	"pageup":        0xff55, // XK_Prior
	"forwarddelete": 0xffff, // XK_Delete
	"del":           0xffff, // XK_Delete
	"end":           0xff57, // XK_End
	"pagedown":      0xff56, // XK_Next
	"numlock":       0xff7f, // XK_Num_Lock
}
