//go:build linux

package hotkey

import "golang.design/x/hotkey"

// Platform-specific modifier mappings for Linux
// On X11: Mod1 is typically Alt, Mod4 is typically Super/Meta
var (
	modAlt  = hotkey.Mod1 // Alt key on Linux (X11)
	modMeta = hotkey.Mod4 // Super/Meta key on Linux (X11)
)
