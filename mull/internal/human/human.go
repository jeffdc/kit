// Package human keeps, per machine, the name the person using mull here goes
// by, in ~/.config/mull/ beside the registry. It's never on the mull branch:
// a repository is shared, and everyone who works on it has their own name.
package human

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"mull/internal/confdir"
)

// File is where the name is kept.
func File() (string, error) {
	dir, err := confdir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "name"), nil
}

// Name returns the name set on this machine, or "" when none is.
func Name() (string, error) {
	f, err := File()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(f)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// SetName sets the name on this machine; an empty name clears it.
func SetName(name string) error {
	f, err := File()
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if strings.ContainsAny(name, "\n\r") {
		return errors.New("a name is one line")
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return err
	}
	return os.WriteFile(f, []byte(name+"\n"), 0o644)
}
