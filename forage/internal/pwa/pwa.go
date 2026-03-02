package pwa

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"forage/internal/model"
)

//go:embed assets/*
var assets embed.FS

type templateData struct {
	Books       template.JS
	Booksellers template.JS
	DataVersion string
}

func Generate(books []model.Book, booksellers []model.Bookseller, outDir string) error {
	if books == nil {
		books = []model.Book{}
	}
	if booksellers == nil {
		booksellers = []model.Bookseller{}
	}

	booksJSON, err := json.Marshal(books)
	if err != nil {
		return err
	}
	sellersJSON, err := json.Marshal(booksellers)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	// Parse and execute index.html template
	tmplBytes, err := assets.ReadFile("assets/index.html")
	if err != nil {
		return err
	}

	tmpl, err := template.New("index.html").Parse(string(tmplBytes))
	if err != nil {
		return err
	}

	indexFile, err := os.Create(filepath.Join(outDir, "index.html"))
	if err != nil {
		return err
	}
	defer indexFile.Close()

	data := templateData{
		Books:       template.JS(booksJSON),
		Booksellers: template.JS(sellersJSON),
		DataVersion: time.Now().UTC().Format(time.RFC3339),
	}

	if err := tmpl.Execute(indexFile, data); err != nil {
		return err
	}

	// Copy all other embedded files as-is
	entries, err := fs.ReadDir(assets, "assets")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Name() == "index.html" {
			continue
		}

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
