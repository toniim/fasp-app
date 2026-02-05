//go:build windows

package hotkey

import "golang.design/x/hotkey"

// Platform-specific modifier mappings for Windows
var (
	modAlt = hotkey.ModAlt  // Alt key on Windows
	modMeta = hotkey.ModWin // Windows key (closest to Cmd on macOS)
)
