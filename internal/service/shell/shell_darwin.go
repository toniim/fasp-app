//go:build darwin

package shell

import (
	"fmt"
	"os"
	"os/exec"
)

// Open reveals a file or folder with the user's default handler (Finder for
// directories, the associated app for images).
func Open(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	if err := exec.Command("open", path).Start(); err != nil {
		return fmt.Errorf("failed to open %s: %w", path, err)
	}

	return nil
}
