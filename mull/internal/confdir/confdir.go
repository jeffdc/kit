// Package confdir is where mull keeps per-machine state: the registry, the
// last looks and the human's name. It imports nothing of mull's, so any
// package can use it.
package confdir

import (
	"os"
	"path/filepath"
)

// Dir is mull's config directory: $XDG_CONFIG_HOME/mull, or ~/.config/mull.
func Dir() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(x) {
		return filepath.Join(x, "mull"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "mull"), nil
}
