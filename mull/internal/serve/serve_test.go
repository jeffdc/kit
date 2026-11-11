package serve

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/mull"
)

// get requests path from h as the browser would, on localhost.
func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.Host = "localhost:6855"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func onMull(t *testing.T) string {
	t.Helper()
	dir := gittest.NewRepo(t)
	m, err := mull.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Init("", ""); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestProjects(t *testing.T) {
	good := onMull(t)
	bare := gittest.NewRepo(t) // a repository with no mull branch
	gone := filepath.Join(t.TempDir(), "gone")
	repos := []string{good, bare, gone}
	h := Handler(func() ([]string, error) { return repos, nil })

	rec := get(t, h, "/api/projects")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("GET /api/projects: %d %s\n%s", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
	var list ProjectList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	byPath := map[string]Project{}
	for _, p := range list.Projects {
		byPath[p.Path] = p
	}
	if len(byPath) != 3 {
		t.Fatalf("projects = %+v", list.Projects)
	}
	if p := byPath[good]; p.Error != nil || p.Summary == nil || p.Summary.Integration != "main" || p.Name != filepath.Base(good) {
		t.Errorf("good project = %+v", p)
	}
	if p := byPath[bare]; p.Summary != nil || p.Error == nil || p.Error.Code != mull.CodeNotInitialized {
		t.Errorf("repo without mull = %+v", p)
	}
	if p := byPath[gone]; p.Summary != nil || p.Error == nil || p.Error.Code != mull.CodeNotRepo {
		t.Errorf("missing repo = %+v", p)
	}
	for i := 1; i < len(list.Projects); i++ {
		if list.Projects[i-1].ID > list.Projects[i].ID {
			t.Errorf("projects not sorted by ID: %+v", list.Projects)
		}
	}

	// The registry is read on every request.
	repos = []string{}
	rec = get(t, h, "/api/projects")
	if body := strings.TrimSpace(rec.Body.String()); body != "{\n  \"projects\": []\n}" {
		t.Errorf("empty registry: %s", body)
	}
}

func TestProjectsRegistryError(t *testing.T) {
	h := Handler(func() ([]string, error) { return nil, errors.New("unreadable") })
	rec := get(t, h, "/api/projects")
	var e mull.Error
	if rec.Code != 500 || json.Unmarshal(rec.Body.Bytes(), &e) != nil || e.Code != mull.CodeInternal {
		t.Errorf("registry error: %d %s", rec.Code, rec.Body)
	}
}

func TestProjectIDs(t *testing.T) {
	got := projectIDs([]string{"/a/kit", "/b/forage", "/c/kit", "/d/kit-2", "/e/kit"})
	if want := []string{"kit", "forage", "kit-2", "kit-2-2", "kit-3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("projectIDs = %v, want %v", got, want)
	}
}

func TestStaticAndUnknown(t *testing.T) {
	h := Handler(func() ([]string, error) { return []string{}, nil })
	for path, want := range map[string]string{"/": "<title>mull</title>", "/app.js": "/api/events", "/style.css": "--ground"} {
		rec := get(t, h, path)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("GET %s: %d, want a body with %q", path, rec.Code, want)
		}
	}
	// Module scripts load only with a JavaScript content type.
	if ct := get(t, h, "/app.js").Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("GET /app.js: Content-Type %q, want text/javascript", ct)
	}
	rec := get(t, h, "/api/nope")
	var e mull.Error
	if rec.Code != 404 || json.Unmarshal(rec.Body.Bytes(), &e) != nil || e.Code != mull.CodeNotFound {
		t.Errorf("unknown endpoint: %d %s", rec.Code, rec.Body)
	}
	req := httptest.NewRequest("POST", "/api/projects", nil)
	req.Host = "127.0.0.1:6855"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/projects: %d, want 405 (the API is read-only)", rec.Code)
	}
}

// TestModuleImports checks that every relative import in the embedded UI
// names an embedded file, since there's no build step to catch a typo.
func TestModuleImports(t *testing.T) {
	imp := regexp.MustCompile(`(?m)^import [^"]*"\./([^"]+)";$`)
	files, err := fs.Glob(web, "web/*.js")
	if err != nil || len(files) == 0 {
		t.Fatalf("no embedded modules: %v", err)
	}
	for _, f := range files {
		src, err := fs.ReadFile(web, f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range imp.FindAllStringSubmatch(string(src), -1) {
			if _, err := fs.Stat(web, "web/"+m[1]); err != nil {
				t.Errorf("%s imports ./%s, which isn't embedded", f, m[1])
			}
		}
	}
}

func TestLocalOnly(t *testing.T) {
	h := Handler(func() ([]string, error) { return []string{}, nil })
	for host, want := range map[string]int{
		"localhost:6855": 200, "127.0.0.1:6855": 200, "localhost": 200,
		"evil.example:6855": 403, "evil.example": 403, "localhost.evil.example:6855": 403,
	} {
		req := httptest.NewRequest("GET", "/api/projects", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Host %q: %d, want %d", host, rec.Code, want)
		}
	}
}
