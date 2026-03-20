package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"forage/internal/model"
	"forage/internal/storage"
)

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := storage.New(dir)
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestGetBooks(t *testing.T) {
	s := newTestStore(t)

	// Create two books: one active, one dropped
	_, err := s.CreateBook("The Go Programming Language", "Alan Donovan", nil)
	if err != nil {
		t.Fatalf("creating book: %v", err)
	}
	b2, err := s.CreateBook("Bad Book", "Nobody", nil)
	if err != nil {
		t.Fatalf("creating book: %v", err)
	}
	_, err = s.UpdateBook(b2.ID, "status", "dropped")
	if err != nil {
		t.Fatalf("updating book status: %v", err)
	}

	handler := NewHandler(s, "test-key", "")
	req := httptest.NewRequest("GET", "/api/books", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var books []model.Book
	if err := json.Unmarshal(w.Body.Bytes(), &books); err != nil {
		t.Fatalf("unmarshaling response: %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("expected 1 book, got %d", len(books))
	}
	if books[0].Title != "The Go Programming Language" {
		t.Fatalf("expected 'The Go Programming Language', got %q", books[0].Title)
	}
}

func TestGetVersion(t *testing.T) {
	s := newTestStore(t)
	handler := NewHandler(s, "test-key", "")

	req := httptest.NewRequest("GET", "/api/version", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshaling response: %v", err)
	}
	v, ok := resp["version"]
	if !ok {
		t.Fatal("response missing 'version' key")
	}
	if _, err := time.Parse(time.RFC3339Nano, v); err != nil {
		t.Fatalf("version is not valid RFC3339 timestamp: %q", v)
	}
}

func TestPostChanges_Unauthorized(t *testing.T) {
	s := newTestStore(t)
	handler := NewHandler(s, "correct-key", "")

	body := `{"version":1,"exported":"2025-01-01T00:00:00Z","changes":[]}`

	// No auth header
	req := httptest.NewRequest("POST", "/api/changes", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", w.Code)
	}
	var errResp map[string]string
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp["error"] != "unauthorized" {
		t.Fatalf("expected error 'unauthorized', got %q", errResp["error"])
	}

	// Wrong key
	req = httptest.NewRequest("POST", "/api/changes", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer wrong-key")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong key, got %d", w.Code)
	}
}

func TestPostChanges_Create(t *testing.T) {
	s := newTestStore(t)
	handler := NewHandler(s, "test-key", "")

	body := `{
		"version": 1,
		"exported": "2025-01-01T00:00:00Z",
		"changes": [{
			"op": "create",
			"book": {
				"id": "ab12",
				"title": "Dune",
				"author": "Frank Herbert",
				"status": "wishlist",
				"date_added": "2025-01-01"
			},
			"ts": "2025-01-01T00:00:00Z"
		}]
	}`

	req := httptest.NewRequest("POST", "/api/changes", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var summary map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("unmarshaling summary: %v", err)
	}
	if summary["applied"] != 1 {
		t.Fatalf("expected 1 applied, got %d", summary["applied"])
	}

	// Verify the book now appears in GET /api/books
	req = httptest.NewRequest("GET", "/api/books", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var books []model.Book
	json.Unmarshal(w.Body.Bytes(), &books)
	if len(books) != 1 {
		t.Fatalf("expected 1 book after create, got %d", len(books))
	}
	if books[0].Title != "Dune" {
		t.Fatalf("expected 'Dune', got %q", books[0].Title)
	}
}

func TestPostChanges_BumpsVersion(t *testing.T) {
	s := newTestStore(t)
	handler := NewHandler(s, "test-key", "")

	// Get initial version
	req := httptest.NewRequest("GET", "/api/version", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var v1 map[string]string
	json.Unmarshal(w.Body.Bytes(), &v1)
	t1, _ := time.Parse(time.RFC3339Nano, v1["version"])

	// POST a change
	body := `{
		"version": 1,
		"exported": "2025-01-01T00:00:00Z",
		"changes": [{
			"op": "create",
			"book": {
				"id": "cd34",
				"title": "Test Book",
				"author": "Test Author",
				"status": "wishlist",
				"date_added": "2025-01-01"
			},
			"ts": "2025-01-01T00:00:00Z"
		}]
	}`
	req = httptest.NewRequest("POST", "/api/changes", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Get version again
	req = httptest.NewRequest("GET", "/api/version", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var v2 map[string]string
	json.Unmarshal(w.Body.Bytes(), &v2)
	t2, _ := time.Parse(time.RFC3339Nano, v2["version"])

	if !t2.After(t1) {
		t.Fatalf("expected version to increase: %v -> %v", t1, t2)
	}
}

func TestGetBooksellers(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddBookseller("Powell's Books", "https://powells.com")
	if err != nil {
		t.Fatalf("adding bookseller: %v", err)
	}

	handler := NewHandler(s, "test-key", "")
	req := httptest.NewRequest("GET", "/api/booksellers", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var sellers []model.Bookseller
	if err := json.Unmarshal(w.Body.Bytes(), &sellers); err != nil {
		t.Fatalf("unmarshaling response: %v", err)
	}
	if len(sellers) != 1 {
		t.Fatalf("expected 1 bookseller, got %d", len(sellers))
	}
	if sellers[0].Name != "Powell's Books" {
		t.Fatalf("expected 'Powell's Books', got %q", sellers[0].Name)
	}
}

func TestStaticFiles(t *testing.T) {
	s := newTestStore(t)

	wwwDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(wwwDir, "index.html"), []byte("<html>hello</html>"), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	handler := NewHandler(s, "test-key", wwwDir)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "<html>hello</html>") {
		t.Fatalf("expected html content, got %q", w.Body.String())
	}
}

func TestCORS(t *testing.T) {
	s := newTestStore(t)
	handler := NewHandler(s, "test-key", "")

	req := httptest.NewRequest("OPTIONS", "/api/books", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", w.Code)
	}
	if v := w.Header().Get("Access-Control-Allow-Origin"); v != "*" {
		t.Fatalf("expected CORS origin *, got %q", v)
	}
	if v := w.Header().Get("Access-Control-Allow-Headers"); v != "Authorization, Content-Type" {
		t.Fatalf("expected CORS headers 'Authorization, Content-Type', got %q", v)
	}
	if v := w.Header().Get("Access-Control-Allow-Methods"); v != "GET, POST, OPTIONS" {
		t.Fatalf("expected CORS methods 'GET, POST, OPTIONS', got %q", v)
	}

	// Also verify CORS headers on a regular GET request
	req = httptest.NewRequest("GET", "/api/version", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if v := w.Header().Get("Access-Control-Allow-Origin"); v != "*" {
		t.Fatalf("expected CORS origin on GET, got %q", v)
	}
}
