package cmd

import (
	"encoding/json"
	"testing"

	"forage/internal/model"
	"forage/internal/storage"
)

func testStoreCmd(t *testing.T) *storage.Store {
	t.Helper()
	s, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("testStoreCmd: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestImportChanges_Create(t *testing.T) {
	s := testStoreCmd(t)

	cl := changelog{
		Version:  1,
		Exported: "2026-03-01T12:00:00Z",
		Changes: []changeEntry{
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

	summary := applyChanges(s, cl)

	if summary.Applied != 1 {
		t.Errorf("Applied = %d, want 1", summary.Applied)
	}
	if summary.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", summary.Skipped)
	}
	if summary.Errors != 0 {
		t.Errorf("Errors = %d, want 0", summary.Errors)
	}

	// Verify the book exists with correct fields
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

func TestImportChanges_Update(t *testing.T) {
	s := testStoreCmd(t)

	// Create a book first
	_, err := s.CreateBookWithID("a1b2", "Original Title", "Some Author", map[string]string{
		"status": "wishlist",
	})
	if err != nil {
		t.Fatalf("CreateBookWithID() error: %v", err)
	}

	cl := changelog{
		Version:  1,
		Exported: "2026-03-01T12:00:00Z",
		Changes: []changeEntry{
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

	summary := applyChanges(s, cl)

	if summary.Applied != 1 {
		t.Errorf("Applied = %d, want 1", summary.Applied)
	}
	if summary.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", summary.Skipped)
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

func TestImportChanges_Delete(t *testing.T) {
	s := testStoreCmd(t)

	// Create a book first
	_, err := s.CreateBookWithID("d3e4", "To Delete", "Author", nil)
	if err != nil {
		t.Fatalf("CreateBookWithID() error: %v", err)
	}

	cl := changelog{
		Version:  1,
		Exported: "2026-03-01T12:00:00Z",
		Changes: []changeEntry{
			{
				Op: "delete",
				ID: "d3e4",
				Ts: "2026-03-01T12:02:00Z",
			},
		},
	}

	summary := applyChanges(s, cl)

	if summary.Applied != 1 {
		t.Errorf("Applied = %d, want 1", summary.Applied)
	}

	_, err = s.GetBook("d3e4")
	if err == nil {
		t.Fatal("expected error getting deleted book")
	}
}

func TestImportChanges_SkipMissing(t *testing.T) {
	s := testStoreCmd(t)

	cl := changelog{
		Version:  1,
		Exported: "2026-03-01T12:00:00Z",
		Changes: []changeEntry{
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

	summary := applyChanges(s, cl)

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

func TestImportChanges_MixedOps(t *testing.T) {
	s := testStoreCmd(t)

	// Pre-create a book to update and one to delete
	_, err := s.CreateBookWithID("bb01", "Update Me", "Author A", map[string]string{"status": "wishlist"})
	if err != nil {
		t.Fatalf("CreateBookWithID() error: %v", err)
	}
	_, err = s.CreateBookWithID("cc02", "Delete Me", "Author B", nil)
	if err != nil {
		t.Fatalf("CreateBookWithID() error: %v", err)
	}

	cl := changelog{
		Version:  1,
		Exported: "2026-03-01T12:00:00Z",
		Changes: []changeEntry{
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

	summary := applyChanges(s, cl)

	if summary.Applied != 3 {
		t.Errorf("Applied = %d, want 3", summary.Applied)
	}
	if summary.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", summary.Skipped)
	}
	if summary.Errors != 0 {
		t.Errorf("Errors = %d, want 0", summary.Errors)
	}

	// Verify create worked
	b, err := s.GetBook("aa00")
	if err != nil {
		t.Fatalf("GetBook(aa00) error: %v", err)
	}
	if b.Title != "New Book" {
		t.Errorf("created book Title = %q", b.Title)
	}
	if b.Status != "reading" {
		t.Errorf("created book Status = %q, want reading", b.Status)
	}

	// Verify update worked
	b, err = s.GetBook("bb01")
	if err != nil {
		t.Fatalf("GetBook(bb01) error: %v", err)
	}
	if b.Status != "read" {
		t.Errorf("updated book Status = %q, want read", b.Status)
	}
	if b.Rating != 5 {
		t.Errorf("updated book Rating = %d, want 5", b.Rating)
	}

	// Verify delete worked
	_, err = s.GetBook("cc02")
	if err == nil {
		t.Fatal("expected error getting deleted book cc02")
	}
}

func TestImportChanges_JSONRoundtrip(t *testing.T) {
	// Verify our changelog struct marshals/unmarshals correctly
	cl := changelog{
		Version:  1,
		Exported: "2026-03-01T12:00:00Z",
		Changes: []changeEntry{
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

	var parsed changelog
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
