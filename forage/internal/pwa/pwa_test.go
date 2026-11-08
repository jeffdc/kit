package pwa

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
)

func TestAssetsContainAppShell(t *testing.T) {
	for _, name := range []string{"index.html", "js/main.js", "style.css", "sw.js", "manifest.json", "icons/icon-180.png"} {
		if _, err := fs.Stat(Assets(), name); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestIndexReferencesTheApp(t *testing.T) {
	data, err := fs.ReadFile(Assets(), "index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	for _, want := range []string{`src="js/main.js"`, "style.css", "manifest.json", "icons/icon-180.png"} {
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

func TestServiceWorkerPrecachesTheApp(t *testing.T) {
	data, err := fs.ReadFile(Assets(), "sw.js")
	if err != nil {
		t.Fatal(err)
	}
	sw := string(data)
	if !strings.Contains(sw, "__FORAGE_BUILD__") {
		t.Error("sw.js must carry the __FORAGE_BUILD__ placeholder the server stamps")
	}
	start := strings.Index(sw, "const APP = [")
	if start < 0 {
		t.Fatal("sw.js: const APP list not found")
	}
	end := strings.Index(sw[start:], "];")
	listed := map[string]bool{}
	for _, item := range strings.Split(sw[start+len("const APP = ["):start+end], ",") {
		name := strings.TrimPrefix(strings.Trim(strings.TrimSpace(item), `"'`), "./")
		if name == "" {
			continue // "./" is the app root
		}
		listed[name] = true
		if _, err := fs.Stat(Assets(), name); err != nil {
			t.Errorf("sw.js precaches %q, which is not in the assets", name)
		}
	}
	// Every module must be precached, or the app can't start offline.
	mods, _ := fs.Glob(Assets(), "js/*.js")
	for _, m := range append(mods, "index.html", "style.css") {
		if !listed[m] {
			t.Errorf("%s is not precached by sw.js", m)
		}
	}
}
