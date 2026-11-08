package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"forage/internal/model"
	"forage/internal/resolve"
	"forage/internal/storage"
)

// maxOpsPerRequest bounds one POST /api/ops batch; clients send more in
// several requests.
const maxOpsPerRequest = 500

// resolveTimeout bounds one /api/resolve call (several outbound requests).
const resolveTimeout = 20 * time.Second

// handleSync serves GET /api/sync?since=REV: everything changed after REV,
// or a full snapshot when since is 0, missing, or ahead of the server.
// Clients should also start over (since=0) when the epoch changes.
func (h *handler) handleSync(w http.ResponseWriter, r *http.Request) {
	var since int64
	if v := r.URL.Query().Get("since"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "since must be a non-negative integer")
			return
		}
		since = n
	}
	c, err := h.store.ChangesSince(since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, c)
}

type opsRequest struct {
	Ops []model.Op `json:"ops"`
}

type opsResponse struct {
	Results []model.OpResult `json:"results"`
}

// handleOps serves POST /api/ops: apply a batch of ops in order and report
// a result for each. Ops are idempotent by op_id. On a server error the
// response is a 500 and the client should resend the whole batch; ops that
// already went through are recognized and not applied twice.
func (h *handler) handleOps(w http.ResponseWriter, r *http.Request) {
	var req opsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.Ops) > maxOpsPerRequest {
		writeError(w, http.StatusBadRequest, "too many ops in one request (max "+strconv.Itoa(maxOpsPerRequest)+")")
		return
	}

	resp := opsResponse{Results: make([]model.OpResult, 0, len(req.Ops))}
	changed := false
	for _, op := range req.Ops {
		res, err := h.store.ApplyOp(op)
		if err != nil {
			log.Printf("ops: %s (%s): %v", op.OpID, op.Type, err)
			if changed {
				h.bumpVersion()
			}
			writeError(w, http.StatusInternalServerError, "server error applying op "+op.OpID)
			return
		}
		if res.Status == "ok" {
			changed = true
		} else {
			log.Printf("ops: rejected %s (%s): %s", op.OpID, op.Type, res.Error)
		}
		resp.Results = append(resp.Results, res)
	}
	if changed {
		h.bumpVersion()
	}
	writeJSON(w, resp)
}

// createBookRequest is the simple create used by the iOS Shortcut.
type createBookRequest struct {
	Title          string   `json:"title"`
	Author         string   `json:"author"`
	Status         string   `json:"status"`
	ISBN           string   `json:"isbn"`
	PageCount      int      `json:"page_count"`
	FirstPublished int      `json:"first_published"`
	Tags           []string `json:"tags"`
	DateAdded      string   `json:"date_added"`
}

// handleCreateBook serves POST /api/books: create one book with a
// server-chosen ID. Status defaults to wishlist and date_added to today.
func (h *handler) handleCreateBook(w http.ResponseWriter, r *http.Request) {
	var req createBookRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	meta := map[string]string{}
	set := func(k, v string) {
		if v != "" {
			meta[k] = v
		}
	}
	set("status", req.Status)
	set("isbn", req.ISBN)
	set("date_added", req.DateAdded)
	set("tags", strings.Join(req.Tags, ","))
	if req.PageCount > 0 {
		meta["page_count"] = strconv.Itoa(req.PageCount)
	}
	if req.FirstPublished > 0 {
		meta["first_published"] = strconv.Itoa(req.FirstPublished)
	}

	b, err := h.store.CreateBook(req.Title, req.Author, meta)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.bumpVersion()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(b)
}

// writeStoreError maps storage errors to HTTP statuses.
func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, storage.ErrExists):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, storage.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

type resolveRequest struct {
	Input string `json:"input"`
}

// handleResolve serves POST /api/resolve: turn a shared URL, ISBN or text
// into candidate books, each marked if it's already in the library.
func (h *handler) handleResolve(w http.ResponseWriter, r *http.Request) {
	var req resolveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), resolveTimeout)
	defer cancel()

	res, err := h.resolver.Resolve(ctx, req.Input)
	if errors.Is(err, resolve.ErrEmptyInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		log.Printf("resolve %q: %v", req.Input, err)
		writeError(w, http.StatusBadGateway, "lookup failed: "+err.Error())
		return
	}
	books, err := h.store.ListBooks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resolve.MarkLibraryMatches(res.Candidates, books)
	writeJSON(w, res)
}
