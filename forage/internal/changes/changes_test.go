package changes

import (
	"encoding/json"
	"testing"

	"forage/internal/model"
	"forage/internal/storage"
)

func testStore(t *testing.T) *storage.Store {
	t.Helper()
	s, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("testStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestApply_Create(t *testing.T) {
	s := testStore(t)

	cl := Changelog{
		Version:  1,
		Exported: "2026-03-01T12:00:00Z",
		Changes: []Entry{
			{
				Op: "create",
				Book: &model.Book{
					ID:        "f10d",
					Title:     "Test Book",
					Author:    "Test Author",
					Status:    "wishlist",
					DateAdded: "2026-03-01",
				},
				Ts: "2026-03-01T12:00:00Z",
			},
		},
	}

	summary := Apply(s, cl)

	if summary.Applied != 1 {
		t.Errorf("Applied = %d, want 1", summary.Applied)
	}
	if summary.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", summary.Skipped)
	}
	if summary.Errors != 0 {
		t.Errorf("Errors = %d, want 0", summary.Errors)
	}

	b, err := s.GetBook("f10d")
	if err != nil {
		t.Fatalf("GetBook() error: %v", err)
	}
	if b.Title != "Test Book" {
		t.Errorf("Title = %q, want %q", b.Title, "Test Book")
	}
	if b.Author != "Test Author" {
		t.Errorf("Author = %q, want %q", b.Author, "Test Author")
	}
	if b.Status != "wishlist" {
		t.Errorf("Status = %q, want wishlist", b.Status)
	}
}

func TestApply_Update(t *testing.T) {
	s := testStore(t)

	_, err := s.CreateBookWithID("a1b2", "Original Title", "Some Author", map[string]string{
		"status": "wishlist",
	})
	if err != nil {
		t.Fatalf("CreateBookWithID() error: %v", err)
	}

	cl := Changelog{
		Version:  1,
		Exported: "2026-03-01T12:00:00Z",
		Changes: []Entry{
			{
				Op: "update",
				ID: "a1b2",
				Fields: map[string]interface{}{
					"status": "read",
					"rating": float64(4),
				},
				Ts: "2026-03-01T12:01:00Z",
			},
		},
	}

	summary := Apply(s, cl)

	if summary.Applied != 1 {
		t.Errorf("Applied = %d, want 1", summary.Applied)
	}

	b, err := s.GetBook("a1b2")
	if err != nil {
		t.Fatalf("GetBook() error: %v", err)
	}
	if b.Status != "read" {
		t.Errorf("Status = %q, want read", b.Status)
	}
	if b.Rating != 4 {
		t.Errorf("Rating = %d, want 4", b.Rating)
	}
}

func TestApply_Delete(t *testing.T) {
	s := testStore(t)

	_, err := s.CreateBookWithID("d3e4", "To Delete", "Author", nil)
	if err != nil {
		t.Fatalf("CreateBookWithID() error: %v", err)
	}

	cl := Changelog{
		Version:  1,
		Exported: "2026-03-01T12:00:00Z",
		Changes: []Entry{
			{
				Op: "delete",
				ID: "d3e4",
				Ts: "2026-03-01T12:02:00Z",
			},
		},
	}

	summary := Apply(s, cl)

	if summary.Applied != 1 {
		t.Errorf("Applied = %d, want 1", summary.Applied)
	}

	_, err = s.GetBook("d3e4")
	if err == nil {
		t.Fatal("expected error getting deleted book")
	}
}

func TestApply_SkipMissing(t *testing.T) {
	s := testStore(t)

	cl := Changelog{
		Version:  1,
		Exported: "2026-03-01T12:00:00Z",
		Changes: []Entry{
			{
				Op: "update",
				ID: "ffff",
				Fields: map[string]interface{}{
					"status": "read",
				},
				Ts: "2026-03-01T12:01:00Z",
			},
		},
	}

	summary := Apply(s, cl)

	if summary.Applied != 0 {
		t.Errorf("Applied = %d, want 0", summary.Applied)
	}
	if summary.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", summary.Skipped)
	}
	if summary.Errors != 0 {
		t.Errorf("Errors = %d, want 0", summary.Errors)
	}
}

func TestApply_MixedOps(t *testing.T) {
	s := testStore(t)

	_, err := s.CreateBookWithID("bb01", "Update Me", "Author A", map[string]string{"status": "wishlist"})
	if err != nil {
		t.Fatalf("CreateBookWithID() error: %v", err)
	}
	_, err = s.CreateBookWithID("cc02", "Delete Me", "Author B", nil)
	if err != nil {
		t.Fatalf("CreateBookWithID() error: %v", err)
	}

	cl := Changelog{
		Version:  1,
		Exported: "2026-03-01T12:00:00Z",
		Changes: []Entry{
			{
				Op: "create",
				Book: &model.Book{
					ID:        "aa00",
					Title:     "New Book",
					Author:    "New Author",
					Status:    "reading",
					DateAdded: "2026-03-01",
				},
				Ts: "2026-03-01T12:00:00Z",
			},
			{
				Op: "update",
				ID: "bb01",
				Fields: map[string]interface{}{
					"status": "read",
					"rating": float64(5),
				},
				Ts: "2026-03-01T12:01:00Z",
			},
			{
				Op: "delete",
				ID: "cc02",
				Ts: "2026-03-01T12:02:00Z",
			},
			{
				Op: "update",
				ID: "zzzz",
				Fields: map[string]interface{}{
					"status": "read",
				},
				Ts: "2026-03-01T12:03:00Z",
			},
		},
	}

	summary := Apply(s, cl)

	if summary.Applied != 3 {
		t.Errorf("Applied = %d, want 3", summary.Applied)
	}
	if summary.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", summary.Skipped)
	}
	if summary.Errors != 0 {
		t.Errorf("Errors = %d, want 0", summary.Errors)
	}

	b, _ := s.GetBook("aa00")
	if b.Title != "New Book" {
		t.Errorf("created book Title = %q", b.Title)
	}

	b, _ = s.GetBook("bb01")
	if b.Status != "read" {
		t.Errorf("updated book Status = %q, want read", b.Status)
	}
	if b.Rating != 5 {
		t.Errorf("updated book Rating = %d, want 5", b.Rating)
	}

	_, err = s.GetBook("cc02")
	if err == nil {
		t.Fatal("expected error getting deleted book cc02")
	}
}

func TestApply_JSONRoundtrip(t *testing.T) {
	cl := Changelog{
		Version:  1,
		Exported: "2026-03-01T12:00:00Z",
		Changes: []Entry{
			{
				Op: "create",
				Book: &model.Book{
					ID:        "f10d",
					Title:     "Test",
					Author:    "Auth",
					Status:    "wishlist",
					DateAdded: "2026-03-01",
				},
				Ts: "2026-03-01T12:00:00Z",
			},
			{
				Op:     "update",
				ID:     "f10d",
				Fields: map[string]interface{}{"status": "read", "rating": float64(4)},
				Ts:     "2026-03-01T12:01:00Z",
			},
			{
				Op: "delete",
				ID: "f10d",
				Ts: "2026-03-01T12:02:00Z",
			},
		},
	}

	data, err := json.Marshal(cl)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Changelog
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(parsed.Changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(parsed.Changes))
	}
	if parsed.Changes[0].Op != "create" {
		t.Errorf("change[0].Op = %q", parsed.Changes[0].Op)
	}
	if parsed.Changes[1].Op != "update" {
		t.Errorf("change[1].Op = %q", parsed.Changes[1].Op)
	}
	if parsed.Changes[2].Op != "delete" {
		t.Errorf("change[2].Op = %q", parsed.Changes[2].Op)
	}
}

func dates(t *testing.T, s *storage.Store, id string) []string {
	t.Helper()
	reads, err := s.ReadsForBook(id)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, r := range reads {
		out = append(out, r.DateFinished)
	}
	return out
}

func TestApply_ReadHistory(t *testing.T) {
	s := testStore(t)
	update := func(id string, fields map[string]any) Summary {
		return Apply(s, Changelog{Changes: []Entry{{Op: "update", ID: id, Fields: fields}}})
	}

	// Created as read: one undated read.
	Apply(s, Changelog{Changes: []Entry{{Op: "create", Book: &model.Book{ID: "a1a1a1a1a1a1", Title: "Dune", Author: "Frank Herbert", Status: "read"}}}})
	if got := dates(t, s, "a1a1a1a1a1a1"); len(got) != 1 || got[0] != "" {
		t.Errorf("created as read: reads %v, want one undated", got)
	}

	// Wishlist -> read records a read finished today.
	Apply(s, Changelog{Changes: []Entry{{Op: "create", Book: &model.Book{ID: "b2b2b2b2b2b2", Title: "Emma", Author: "Jane Austen"}}}})
	if got := dates(t, s, "b2b2b2b2b2b2"); len(got) != 0 {
		t.Fatalf("wishlist book has reads %v", got)
	}
	update("b2b2b2b2b2b2", map[string]any{"title": "Emma", "status": "read", "rating": 4.0})
	if got := dates(t, s, "b2b2b2b2b2b2"); len(got) != 1 || got[0] != storage.Today() {
		t.Errorf("after marking read: %v, want [today]", got)
	}

	// Saving again with status still read adds nothing (the PWA resends all fields).
	update("b2b2b2b2b2b2", map[string]any{"title": "Emma", "status": "read", "rating": 5.0})
	if got := dates(t, s, "b2b2b2b2b2b2"); len(got) != 1 {
		t.Errorf("resave added a read: %v", got)
	}

	// A re-read: reading again, then read again, records a second read.
	update("b2b2b2b2b2b2", map[string]any{"status": "reading"})
	update("b2b2b2b2b2b2", map[string]any{"status": "read"})
	if got := dates(t, s, "b2b2b2b2b2b2"); len(got) != 2 {
		t.Errorf("re-read: %v, want 2 reads", got)
	}

	// An explicit date_read sets the latest read's date.
	update("a1a1a1a1a1a1", map[string]any{"date_read": "2019-05-01"})
	if got := dates(t, s, "a1a1a1a1a1a1"); len(got) != 1 || got[0] != "2019-05-01" {
		t.Errorf("date_read: %v", got)
	}
	b, _ := s.GetBook("a1a1a1a1a1a1")
	if b.DateRead != "2019-05-01" {
		t.Errorf("derived DateRead = %q", b.DateRead)
	}
}

func TestApply_UpdateIsAllOrNothing(t *testing.T) {
	s := testStore(t)
	s.CreateBookWithID("c3c3c3c3c3c3", "Dune", "Frank Herbert", nil)
	sum := Apply(s, Changelog{Changes: []Entry{{Op: "update", ID: "c3c3c3c3c3c3", Fields: map[string]any{"title": "Changed", "status": "bogus"}}}})
	if sum.Skipped != 1 {
		t.Errorf("summary = %+v, want 1 skipped", sum)
	}
	if b, _ := s.GetBook("c3c3c3c3c3c3"); b.Title != "Dune" {
		t.Errorf("partial update applied: title %q", b.Title)
	}
}
