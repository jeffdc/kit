package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"forage/internal/changes"
	"forage/internal/model"
	"forage/internal/storage"
)

type handler struct {
	store   *storage.Store
	apiKey  string
	version time.Time
	mu      sync.Mutex
}

// NewHandler returns an http.Handler with all API routes and static file serving.
func NewHandler(store *storage.Store, apiKey string, wwwDir string) http.Handler {
	h := &handler{
		store:   store,
		apiKey:  apiKey,
		version: time.Now(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/books", h.handleBooks)
	mux.HandleFunc("/api/version", h.handleVersion)
	mux.HandleFunc("/api/changes", h.handleChanges)
	mux.HandleFunc("/api/booksellers", h.handleBooksellers)

	if wwwDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(wwwDir)))
	}

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *handler) handleBooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	books, err := h.store.ListBooks(nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Filter out dropped books
	var active []model.Book
	for _, b := range books {
		if b.Status != "dropped" {
			active = append(active, b)
		}
	}

	if active == nil {
		active = []model.Book{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(active)
}

func (h *handler) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	h.mu.Lock()
	v := h.version.UTC().Format(time.RFC3339Nano)
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"version": v})
}

func (h *handler) handleChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check auth
	auth := r.Header.Get("Authorization")
	if auth != "Bearer "+h.apiKey {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	var cl changes.Changelog
	if err := json.NewDecoder(r.Body).Decode(&cl); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	summary := changes.Apply(h.store, cl)

	h.mu.Lock()
	h.version = time.Now()
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func (h *handler) handleBooksellers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	sellers, err := h.store.LoadBooksellers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if sellers == nil {
		sellers = []model.Bookseller{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sellers)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
