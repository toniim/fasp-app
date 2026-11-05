//go:build windows

package shell

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Open reveals a file or folder with the user's default handler (Explorer for
// directories, the associated app for images).
func Open(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	// rundll32 handles both files and directories without spawning a console.
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open %s: %w", path, err)
	}

	return nil
}
