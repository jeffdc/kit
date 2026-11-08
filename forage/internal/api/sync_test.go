package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"forage/internal/model"
	"forage/internal/storage"
)

func decode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
	return v
}

func TestNewEndpointsRequireAuth(t *testing.T) {
	h := NewHandler(newTestStore(t), "test-key", "test", nil)
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/sync"}, {"POST", "/api/ops"}, {"POST", "/api/books"},
	} {
		req, _ := http.NewRequest(c.method, c.path, strings.NewReader("{}"))
		if w := serve(h, req); w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without key: %d", c.method, c.path, w.Code)
		}
	}
}

func TestOpsThenSync(t *testing.T) {
	h := NewHandler(newTestStore(t), "test-key", "test", nil)

	full := decode[storage.Changes](t, serve(h, authed("GET", "/api/sync", nil)).Body.Bytes())
	if !full.Full || full.Epoch == 0 {
		t.Fatalf("initial sync: %+v", full)
	}

	body := `{"ops":[
		{"op_id":"o1","type":"book.create","book":{"id":"a1b2c3d4e5f6","title":"Dune","author":"Frank Herbert"}},
		{"op_id":"o2","type":"read.create","read":{"id":"bbbbbbbbbbbb","book_id":"a1b2c3d4e5f6","date_finished":"2026-11-08"}},
		{"op_id":"o3","type":"book.update","id":"a1b2c3d4e5f6","fields":{"status":"read","rating":5}},
		{"op_id":"o4","type":"book.update","id":"ffffffffffff","fields":{"rating":1}}
	]}`
	w := serve(h, authed("POST", "/api/ops", strings.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("ops: %d %s", w.Code, w.Body)
	}
	resp := decode[opsResponse](t, w.Body.Bytes())
	want := []string{"ok", "ok", "ok", "rejected"}
	if len(resp.Results) != len(want) {
		t.Fatalf("results: %+v", resp.Results)
	}
	for i, r := range resp.Results {
		if r.Status != want[i] {
			t.Errorf("op %d: %+v, want %s", i, r, want[i])
		}
	}

	// Resending the same batch changes nothing and gives the same answers.
	again := decode[opsResponse](t, serve(h, authed("POST", "/api/ops", strings.NewReader(body))).Body.Bytes())
	for i := range 3 {
		if again.Results[i] != resp.Results[i] {
			t.Errorf("replayed op %d: %+v, want %+v", i, again.Results[i], resp.Results[i])
		}
	}

	delta := decode[storage.Changes](t, serve(h, authed("GET", "/api/sync?since="+itoa(full.Rev), nil)).Body.Bytes())
	if delta.Full || len(delta.Books) != 1 || len(delta.Reads) != 1 {
		t.Fatalf("delta: %+v", delta)
	}
	b := delta.Books[0]
	if b.Status != "read" || b.Rating != 5 || b.DateRead != "2026-11-08" {
		t.Errorf("synced book: %+v", b)
	}

	if w := serve(h, authed("GET", "/api/sync?since=-1", nil)); w.Code != http.StatusBadRequest {
		t.Errorf("negative since: %d", w.Code)
	}
}

func TestOpsRejectsHugeBatch(t *testing.T) {
	h := NewHandler(newTestStore(t), "test-key", "test", nil)
	ops := make([]model.Op, maxOpsPerRequest+1)
	data, _ := json.Marshal(opsRequest{Ops: ops})
	if w := serve(h, authed("POST", "/api/ops", strings.NewReader(string(data)))); w.Code != http.StatusBadRequest {
		t.Errorf("oversized batch: %d", w.Code)
	}
}

func TestCreateBookEndpoint(t *testing.T) {
	s := newTestStore(t)
	h := NewHandler(s, "test-key", "test", nil)

	w := serve(h, authed("POST", "/api/books", strings.NewReader(
		`{"title":"Piranesi","author":"Susanna Clarke","first_published":2020,"date_added":"2026-11-08"}`)))
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	b := decode[model.Book](t, w.Body.Bytes())
	if len(b.ID) != 12 || b.Status != "wishlist" || b.SortAuthor != "Clarke, Susanna" || b.FirstPublished != 2020 || b.DateAdded != "2026-11-08" {
		t.Errorf("created: %+v", b)
	}
	if _, err := s.GetBook(b.ID); err != nil {
		t.Errorf("not stored: %v", err)
	}

	for _, bad := range []string{`{"title":"","author":"X"}`, `{"title":"T","author":"A","status":"gone"}`, `not json`} {
		if w := serve(h, authed("POST", "/api/books", strings.NewReader(bad))); w.Code != http.StatusBadRequest {
			t.Errorf("POST %s: %d, want 400", bad, w.Code)
		}
	}
}

func TestLegacyVersionMovesOnNewWrites(t *testing.T) {
	h := NewHandler(newTestStore(t), "test-key", "test", nil)
	v := func() string {
		return decode[map[string]string](t, serve(h, authed("GET", "/api/version", nil)).Body.Bytes())["version"]
	}
	v1 := v()
	serve(h, authed("POST", "/api/books", strings.NewReader(`{"title":"T","author":"A"}`)))
	v2 := v()
	serve(h, authed("POST", "/api/ops", strings.NewReader(`{"ops":[{"op_id":"x","type":"book.create","book":{"id":"a1b2c3d4e5f6","title":"T","author":"A"}}]}`)))
	v3 := v()
	if !(v2 > v1 && v3 > v2) {
		t.Errorf("legacy version did not advance: %s %s %s", v1, v2, v3)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
