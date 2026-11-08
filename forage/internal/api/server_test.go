package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// authed builds a request carrying the test API key.
func authed(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Authorization", "Bearer test-key")
	return req
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestReadsRequireAuth(t *testing.T) {
	h := NewHandler(newTestStore(t), "test-key", "test", nil)
	for _, path := range []string{"/api/books", "/api/version", "/api/booksellers"} {
		if w := serve(h, httptest.NewRequest("GET", path, nil)); w.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without key: %d, want 401", path, w.Code)
		}
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer wrong")
		if w := serve(h, req); w.Code != http.StatusUnauthorized {
			t.Errorf("GET %s with wrong key: %d, want 401", path, w.Code)
		}
		if w := serve(h, authed("GET", path, nil)); w.Code != http.StatusOK {
			t.Errorf("GET %s with key: %d, want 200", path, w.Code)
		}
	}
}

func TestHealthIsPublicAndReportsBuild(t *testing.T) {
	h := NewHandler(newTestStore(t), "test-key", "abc123", nil)
	w := serve(h, httptest.NewRequest("GET", "/api/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("health: %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["build"] != "abc123" || resp["status"] != "ok" {
		t.Errorf("health = %v", resp)
	}
}

func TestUnknownAPIRouteIsJSON404(t *testing.T) {
	h := NewHandler(newTestStore(t), "test-key", "test", nil)
	for _, req := range []*http.Request{
		authed("GET", "/api/prices", nil),
		authed("POST", "/api/prices", strings.NewReader(`{"id":"a1b2"}`)),
	} {
		w := serve(h, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("%s %s: %d, want 404", req.Method, req.URL.Path, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("%s %s: content type %q", req.Method, req.URL.Path, ct)
		}
	}
}

func TestChangesRejectsOversizedBody(t *testing.T) {
	h := NewHandler(newTestStore(t), "test-key", "test", nil)
	big := `{"changes":[],"pad":"` + strings.Repeat("x", maxBodyBytes) + `"}`
	if w := serve(h, authed("POST", "/api/changes", strings.NewReader(big))); w.Code != http.StatusBadRequest {
		t.Errorf("oversized body: %d, want 400", w.Code)
	}
}

func TestStaticFilesServedWithNoCache(t *testing.T) {
	static := fstest.MapFS{"index.html": {Data: []byte("<html>hello</html>")}}
	h := NewHandler(newTestStore(t), "test-key", "test", static)

	w := serve(h, httptest.NewRequest("GET", "/", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("GET /: %d %q", w.Code, w.Body.String())
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
	if acao := w.Header().Get("Access-Control-Allow-Origin"); acao != "" {
		t.Errorf("unexpected CORS header %q", acao)
	}
}

func TestServiceWorkerGetsBuildStamp(t *testing.T) {
	static := fstest.MapFS{"sw.js": {Data: []byte(`const BUILD = "__FORAGE_BUILD__";`)}}
	h := NewHandler(newTestStore(t), "test-key", "abc123", static)
	w := serve(h, httptest.NewRequest("GET", "/sw.js", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"abc123"`) || strings.Contains(w.Body.String(), "__FORAGE_BUILD__") {
		t.Fatalf("sw.js: %d %q", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q", cc)
	}
}
