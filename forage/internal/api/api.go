package api

import (
	"crypto/subtle"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"forage/internal/changes"
	"forage/internal/model"
	"forage/internal/storage"
)

// maxBodyBytes caps request bodies. A changelog of a few hundred edits is well under this.
const maxBodyBytes = 1 << 20

type handler struct {
	store   *storage.Store
	apiKey  string
	build   string
	mu      sync.Mutex
	version time.Time
}

// NewHandler returns an http.Handler serving the API and, if static is non-nil,
// the PWA files. Every /api route except /api/health requires the API key.
func NewHandler(store *storage.Store, apiKey, build string, static fs.FS) http.Handler {
	h := &handler{
		store:   store,
		apiKey:  apiKey,
		build:   build,
		version: time.Now(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.handleHealth)
	mux.Handle("GET /api/books", h.auth(h.handleBooks))
	mux.Handle("GET /api/version", h.auth(h.handleVersion))
	mux.Handle("POST /api/changes", h.auth(h.handleChanges))
	mux.Handle("GET /api/booksellers", h.auth(h.handleBooksellers))
	mux.Handle("GET /api/sync", h.auth(h.handleSync))
	mux.Handle("POST /api/ops", h.auth(h.handleOps))
	mux.Handle("POST /api/books", h.auth(h.handleCreateBook))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})

	if static != nil {
		files := http.FileServerFS(static)
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Always revalidate: the service worker is network-first and
			// relies on getting fresh files whenever the network is up.
			w.Header().Set("Cache-Control", "no-cache")
			files.ServeHTTP(w, r)
		}))
	}

	return logRequests(mux)
}

func (h *handler) auth(next http.HandlerFunc) http.Handler {
	want := []byte("Bearer " + h.apiKey)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	})
}

func (h *handler) bumpVersion() {
	h.mu.Lock()
	h.version = time.Now()
	h.mu.Unlock()
}

func (h *handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok", "build": h.build})
}

func (h *handler) handleBooks(w http.ResponseWriter, r *http.Request) {
	books, err := h.store.ListBooks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// The current PWA can't show dropped books; this goes away with the new sync API.
	active := []model.Book{}
	for _, b := range books {
		if b.Status != "dropped" {
			active = append(active, b)
		}
	}
	writeJSON(w, active)
}

func (h *handler) handleVersion(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	v := h.version.UTC().Format(time.RFC3339Nano)
	h.mu.Unlock()
	writeJSON(w, map[string]string{"version": v})
}

func (h *handler) handleChanges(w http.ResponseWriter, r *http.Request) {
	var cl changes.Changelog
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&cl); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	summary := changes.Apply(h.store, cl)
	if summary.Skipped > 0 || summary.Errors > 0 {
		log.Printf("changes: applied=%d skipped=%d errors=%d", summary.Applied, summary.Skipped, summary.Errors)
	}
	h.bumpVersion()
	writeJSON(w, summary)
}

func (h *handler) handleBooksellers(w http.ResponseWriter, r *http.Request) {
	sellers, err := h.store.LoadBooksellers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sellers == nil {
		sellers = []model.Bookseller{}
	}
	writeJSON(w, sellers)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// logRequests logs API calls and any failed request. Successful static file
// requests are skipped to keep the log readable.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/api/") || rec.status >= 400 {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
		}
	})
}
