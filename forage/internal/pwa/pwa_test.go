package pwa

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forage/internal/model"
)

func TestGenerate_CreatesAllFiles(t *testing.T) {
	dir := t.TempDir()

	books := []model.Book{
		{ID: "a1b2", Title: "The Go Programming Language", Author: "Donovan & Kernighan", Status: "read"},
		{ID: "c3d4", Title: "Designing Data-Intensive Applications", Author: "Martin Kleppmann", Status: "reading"},
	}
	booksellers := []model.Bookseller{
		{ID: 1, Name: "Bookshop", URL: "https://bookshop.org/search?keywords={query}"},
	}

	if err := Generate(books, booksellers, dir); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	expected := []string{"index.html", "app.js", "style.css", "sw.js", "manifest.json"}
	for _, name := range expected {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist, but it does not", name)
		}
	}
}

func TestGenerate_InjectsBookData(t *testing.T) {
	dir := t.TempDir()

	books := []model.Book{
		{ID: "a1b2", Title: "The Go Programming Language", Author: "Donovan & Kernighan", Status: "read"},
		{ID: "c3d4", Title: "Designing Data-Intensive Applications", Author: "Martin Kleppmann", Status: "reading"},
	}
	booksellers := []model.Bookseller{
		{ID: 1, Name: "Bookshop", URL: "https://bookshop.org/search?keywords={query}"},
	}

	if err := Generate(books, booksellers, dir); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}

	content := string(data)

	if !strings.Contains(content, "The Go Programming Language") {
		t.Error("index.html should contain book title 'The Go Programming Language'")
	}
	if !strings.Contains(content, "Designing Data-Intensive Applications") {
		t.Error("index.html should contain book title 'Designing Data-Intensive Applications'")
	}
	if !strings.Contains(content, "Bookshop") {
		t.Error("index.html should contain bookseller name 'Bookshop'")
	}
	if !strings.Contains(content, "__FORAGE_DATA__") {
		t.Error("index.html should contain __FORAGE_DATA__")
	}
	if !strings.Contains(content, "__FORAGE_DATA_VERSION__") {
		t.Error("index.html should contain __FORAGE_DATA_VERSION__")
	}
}

func TestGenerate_EmptyLibrary(t *testing.T) {
	dir := t.TempDir()

	if err := Generate([]model.Book{}, []model.Bookseller{}, dir); err != nil {
		t.Fatalf("Generate with empty slices returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "__FORAGE_DATA__") {
		t.Error("index.html should contain __FORAGE_DATA__ even with empty library")
	}
}

func TestGenerate_ManifestValid(t *testing.T) {
	dir := t.TempDir()

	books := []model.Book{
		{ID: "a1b2", Title: "Test Book", Author: "Test Author", Status: "read"},
	}

	if err := Generate(books, []model.Bookseller{}, dir); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to read manifest.json: %v", err)
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("manifest.json is not valid JSON: %v", err)
	}

	required := []string{"name", "short_name", "start_url", "display", "theme_color"}
	for _, field := range required {
		if _, ok := manifest[field]; !ok {
			t.Errorf("manifest.json missing required field %q", field)
		}
	}
}

func TestGenerate_ServiceWorkerExists(t *testing.T) {
	dir := t.TempDir()

	books := []model.Book{
		{ID: "a1b2", Title: "Test Book", Author: "Test Author", Status: "read"},
	}

	if err := Generate(books, []model.Bookseller{}, dir); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "sw.js"))
	if err != nil {
		t.Fatalf("failed to read sw.js: %v", err)
	}

	content := string(data)
	if len(strings.TrimSpace(content)) < 50 {
		t.Error("sw.js appears to be a placeholder; expected a real service worker")
	}
}
