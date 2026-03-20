package pwa

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate_CreatesAllFiles(t *testing.T) {
	dir := t.TempDir()

	if err := Generate(dir); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	expected := []string{"index.html", "app.js", "style.css", "sw.js", "manifest.json", "add.html"}
	for _, name := range expected {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist, but it does not", name)
		}
	}
}

func TestGenerate_IndexHtmlValid(t *testing.T) {
	dir := t.TempDir()

	if err := Generate(dir); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "__FORAGE_DATA__") {
		t.Error("index.html should contain __FORAGE_DATA__")
	}
	if !strings.Contains(content, "app.js") {
		t.Error("index.html should reference app.js")
	}
}

func TestGenerate_ManifestValid(t *testing.T) {
	dir := t.TempDir()

	if err := Generate(dir); err != nil {
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

	if err := Generate(dir); err != nil {
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
