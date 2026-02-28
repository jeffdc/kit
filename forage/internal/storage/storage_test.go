package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Should create books directory
	info, err := os.Stat(filepath.Join(dir, "books"))
	if err != nil {
		t.Fatalf("books dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("books is not a directory")
	}
	_ = s
}

func TestDefaultRoot(t *testing.T) {
	// With env var set
	t.Setenv("FORAGE_DIR", "/tmp/test-forage")
	if got := DefaultRoot(); got != "/tmp/test-forage" {
		t.Errorf("DefaultRoot() with FORAGE_DIR = %q, want /tmp/test-forage", got)
	}

	// Without env var, falls back to ~/.forage
	t.Setenv("FORAGE_DIR", "")
	got := DefaultRoot()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".forage")
	if got != want {
		t.Errorf("DefaultRoot() = %q, want %q", got, want)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"The Left Hand of Darkness", "the-left-hand-of-darkness"},
		{"Blood Meridian", "blood-meridian"},
		{"1984", "1984"},
		{"  Spaces  Everywhere  ", "spaces-everywhere"},
		{"Colon: A Title", "colon-a-title"},
	}
	for _, tt := range tests {
		if got := Slugify(tt.input); got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCreateAndGetBook(t *testing.T) {
	s := testStore(t)

	b, err := s.CreateBook("The Left Hand of Darkness", "Ursula K. Le Guin", nil)
	if err != nil {
		t.Fatalf("CreateBook() error: %v", err)
	}

	if len(b.ID) != 4 {
		t.Errorf("ID length = %d, want 4", len(b.ID))
	}
	if b.Title != "The Left Hand of Darkness" {
		t.Errorf("Title = %q", b.Title)
	}
	if b.Author != "Ursula K. Le Guin" {
		t.Errorf("Author = %q", b.Author)
	}
	if b.Status != "wishlist" {
		t.Errorf("Status = %q, want wishlist", b.Status)
	}
	if b.DateAdded == "" {
		t.Error("DateAdded is empty")
	}

	// Round-trip through GetBook
	got, err := s.GetBook(b.ID)
	if err != nil {
		t.Fatalf("GetBook() error: %v", err)
	}
	if got.Title != b.Title {
		t.Errorf("GetBook Title = %q, want %q", got.Title, b.Title)
	}
	if got.Author != b.Author {
		t.Errorf("GetBook Author = %q, want %q", got.Author, b.Author)
	}
	if got.Status != b.Status {
		t.Errorf("GetBook Status = %q, want %q", got.Status, b.Status)
	}
}

func TestCreateBookWithMeta(t *testing.T) {
	s := testStore(t)

	meta := map[string]string{
		"status": "read",
		"rating": "5",
		"tags":   "sci-fi,classic",
	}
	b, err := s.CreateBook("Dune", "Frank Herbert", meta)
	if err != nil {
		t.Fatalf("CreateBook() error: %v", err)
	}
	if b.Status != "read" {
		t.Errorf("Status = %q, want read", b.Status)
	}
	if b.Rating != 5 {
		t.Errorf("Rating = %d, want 5", b.Rating)
	}
	if len(b.Tags) != 2 || b.Tags[0] != "sci-fi" || b.Tags[1] != "classic" {
		t.Errorf("Tags = %v, want [sci-fi classic]", b.Tags)
	}
}

func TestCreateBookWithBody(t *testing.T) {
	s := testStore(t)

	meta := map[string]string{"body": "Great book about sand."}
	b, err := s.CreateBook("Dune", "Frank Herbert", meta)
	if err != nil {
		t.Fatalf("CreateBook() error: %v", err)
	}

	got, err := s.GetBook(b.ID)
	if err != nil {
		t.Fatalf("GetBook() error: %v", err)
	}
	if got.Body != "Great book about sand." {
		t.Errorf("Body = %q, want %q", got.Body, "Great book about sand.")
	}
}

func TestCreateBookInvalidStatus(t *testing.T) {
	s := testStore(t)
	_, err := s.CreateBook("Bad", "Author", map[string]string{"status": "bogus"})
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestListBooks(t *testing.T) {
	s := testStore(t)

	s.CreateBook("Book A", "Author A", map[string]string{"status": "wishlist", "tags": "sci-fi"})
	s.CreateBook("Book B", "Author B", map[string]string{"status": "read", "tags": "fantasy"})
	s.CreateBook("Book C", "Author C", map[string]string{"status": "dropped"})

	// No filter — returns all
	all, err := s.ListBooks(nil)
	if err != nil {
		t.Fatalf("ListBooks() error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("len = %d, want 3", len(all))
	}

	// Filter by status
	wishlist, _ := s.ListBooks(map[string]string{"status": "wishlist"})
	if len(wishlist) != 1 || wishlist[0].Title != "Book A" {
		t.Errorf("status filter: got %d books", len(wishlist))
	}

	// Filter by tag
	fantasy, _ := s.ListBooks(map[string]string{"tag": "fantasy"})
	if len(fantasy) != 1 || fantasy[0].Title != "Book B" {
		t.Errorf("tag filter: got %d books", len(fantasy))
	}

	// Filter by author
	byAuthor, _ := s.ListBooks(map[string]string{"author": "Author A"})
	if len(byAuthor) != 1 || byAuthor[0].Title != "Book A" {
		t.Errorf("author filter: got %d books", len(byAuthor))
	}
}

func TestUpdateBook(t *testing.T) {
	s := testStore(t)
	b, _ := s.CreateBook("Old Title", "Author", nil)

	// Update status
	updated, err := s.UpdateBook(b.ID, "status", "reading")
	if err != nil {
		t.Fatalf("UpdateBook() error: %v", err)
	}
	if updated.Status != "reading" {
		t.Errorf("Status = %q, want reading", updated.Status)
	}

	// Update title — should rename file
	updated, err = s.UpdateBook(b.ID, "title", "New Title")
	if err != nil {
		t.Fatalf("UpdateBook() error: %v", err)
	}
	if updated.Title != "New Title" {
		t.Errorf("Title = %q, want New Title", updated.Title)
	}
	if !strings.Contains(updated.Filename, "new-title") {
		t.Errorf("Filename = %q, should contain new-title", updated.Filename)
	}

	// Verify old file is gone
	entries, _ := os.ReadDir(filepath.Join(s.root, "books"))
	for _, e := range entries {
		if strings.Contains(e.Name(), "old-title") {
			t.Errorf("old file still exists: %s", e.Name())
		}
	}
}

func TestUpdateBookInvalidKey(t *testing.T) {
	s := testStore(t)
	b, _ := s.CreateBook("Book", "Author", nil)

	_, err := s.UpdateBook(b.ID, "nonexistent", "value")
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestDeleteBook(t *testing.T) {
	s := testStore(t)
	b, _ := s.CreateBook("To Delete", "Author", nil)

	err := s.DeleteBook(b.ID)
	if err != nil {
		t.Fatalf("DeleteBook() error: %v", err)
	}

	_, err = s.GetBook(b.ID)
	if err == nil {
		t.Fatal("expected error getting deleted book")
	}
}

func TestSearchBooks(t *testing.T) {
	s := testStore(t)
	s.CreateBook("The Left Hand of Darkness", "Ursula K. Le Guin", map[string]string{"tags": "sci-fi"})
	s.CreateBook("Blood Meridian", "Cormac McCarthy", map[string]string{"body": "A dark western novel."})
	s.CreateBook("Dune", "Frank Herbert", nil)

	// Search by title
	results, _ := s.SearchBooks("darkness")
	if len(results) != 1 {
		t.Errorf("title search: got %d, want 1", len(results))
	}

	// Search by author
	results, _ = s.SearchBooks("mccarthy")
	if len(results) != 1 {
		t.Errorf("author search: got %d, want 1", len(results))
	}

	// Search by body
	results, _ = s.SearchBooks("western")
	if len(results) != 1 {
		t.Errorf("body search: got %d, want 1", len(results))
	}

	// Search by tag
	results, _ = s.SearchBooks("sci-fi")
	if len(results) != 1 {
		t.Errorf("tag search: got %d, want 1", len(results))
	}

	// No match
	results, _ = s.SearchBooks("zzzzz")
	if len(results) != 0 {
		t.Errorf("no-match search: got %d, want 0", len(results))
	}
}

func TestGetBookNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.GetBook("zzzz")
	if err == nil {
		t.Fatal("expected error for nonexistent book")
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("testStore: %v", err)
	}
	return s
}
