// Package pwa embeds the PWA's static files, which the server serves directly.
package pwa

import (
	"embed"
	"io/fs"
)

//go:embed assets/*
var assets embed.FS

// Assets returns the PWA files, rooted so that "index.html" is at the top level.
func Assets() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err) // the embed path is fixed at compile time
	}
	return sub
}
