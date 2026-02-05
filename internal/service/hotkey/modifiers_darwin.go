//go:build darwin

package hotkey

import "golang.design/x/hotkey"

// Platform-specific modifier mappings for macOS
var (
	modAlt  = hotkey.ModOption // Option key on macOS
	modMeta = hotkey.ModCmd    // Command key on macOS
)
