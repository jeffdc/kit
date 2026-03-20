package pwa

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed assets/*
var assets embed.FS

// Generate copies all PWA assets to the output directory.
func Generate(outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	entries, err := fs.ReadDir(assets, "assets")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		content, err := assets.ReadFile("assets/" + entry.Name())
		if err != nil {
			return err
		}

		if err := os.WriteFile(filepath.Join(outDir, entry.Name()), content, 0o644); err != nil {
			return err
		}
	}

	return nil
}
