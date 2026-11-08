package pwa

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
)

func TestAssetsContainAppShell(t *testing.T) {
	for _, name := range []string{"index.html", "app.js", "style.css", "sw.js", "manifest.json"} {
		if _, err := fs.Stat(Assets(), name); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestIndexReferencesScriptsAndServiceWorker(t *testing.T) {
	data, err := fs.ReadFile(Assets(), "index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	for _, want := range []string{"app.js", "style.css", "manifest.json", "sw.js"} {
		if !strings.Contains(html, want) {
			t.Errorf("index.html does not reference %s", want)
		}
	}
}

func TestManifestIsValidJSON(t *testing.T) {
	data, err := fs.ReadFile(Assets(), "manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("manifest.json: %v", err)
	}
	if m["name"] == nil || m["start_url"] == nil {
		t.Errorf("manifest missing name or start_url: %v", m)
	}
}

func TestServiceWorkerPrecachesExistingFiles(t *testing.T) {
	data, err := fs.ReadFile(Assets(), "sw.js")
	if err != nil {
		t.Fatal(err)
	}
	sw := string(data)
	start := strings.Index(sw, "APP_ASSETS = [")
	if start < 0 {
		t.Fatal("sw.js: APP_ASSETS not found")
	}
	list := sw[start+len("APP_ASSETS = [") : start+strings.Index(sw[start:], "]")]
	for _, item := range strings.Split(list, ",") {
		name := strings.Trim(strings.TrimSpace(item), `'"`)
		name = strings.TrimPrefix(name, "./")
		if name == "" {
			continue // "./" is the root
		}
		if _, err := fs.Stat(Assets(), name); err != nil {
			t.Errorf("sw.js precaches %q, which is not in the assets", name)
		}
	}
}
