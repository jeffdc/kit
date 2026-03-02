package storage

import (
	"testing"
)

func TestNew(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer s.Close()

	// Verify DB is usable by inserting and querying
	_, err = s.CreateBook("Test", "Author", nil)
	if err != nil {
		t.Fatalf("CreateBook after New: %v", err)
	}
}

func TestDefaultRoot(t *testing.T) {
	t.Setenv("FORAGE_DIR", "/tmp/test-forage")
	if got := DefaultRoot(); got != "/tmp/test-forage" {
		t.Errorf("DefaultRoot() with FORAGE_DIR = %q, want /tmp/test-forage", got)
	}

	t.Setenv("FORAGE_DIR", "")
	got := DefaultRoot()
	if got == "" {
		t.Error("DefaultRoot() returned empty string")
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

	all, err := s.ListBooks(nil)
	if err != nil {
		t.Fatalf("ListBooks() error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("len = %d, want 3", len(all))
	}

	wishlist, _ := s.ListBooks(map[string]string{"status": "wishlist"})
	if len(wishlist) != 1 || wishlist[0].Title != "Book A" {
		t.Errorf("status filter: got %d books", len(wishlist))
	}

	fantasy, _ := s.ListBooks(map[string]string{"tag": "fantasy"})
	if len(fantasy) != 1 || fantasy[0].Title != "Book B" {
		t.Errorf("tag filter: got %d books", len(fantasy))
	}

	byAuthor, _ := s.ListBooks(map[string]string{"author": "Author A"})
	if len(byAuthor) != 1 || byAuthor[0].Title != "Book A" {
		t.Errorf("author filter: got %d books", len(byAuthor))
	}
}

func TestUpdateBook(t *testing.T) {
	s := testStore(t)
	b, _ := s.CreateBook("Old Title", "Author", nil)

	updated, err := s.UpdateBook(b.ID, "status", "reading")
	if err != nil {
		t.Fatalf("UpdateBook() error: %v", err)
	}
	if updated.Status != "reading" {
		t.Errorf("Status = %q, want reading", updated.Status)
	}

	updated, err = s.UpdateBook(b.ID, "title", "New Title")
	if err != nil {
		t.Fatalf("UpdateBook() error: %v", err)
	}
	if updated.Title != "New Title" {
		t.Errorf("Title = %q, want New Title", updated.Title)
	}

	// Verify persisted
	got, _ := s.GetBook(b.ID)
	if got.Title != "New Title" {
		t.Errorf("persisted Title = %q, want New Title", got.Title)
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

	results, _ := s.SearchBooks("darkness")
	if len(results) != 1 {
		t.Errorf("title search: got %d, want 1", len(results))
	}

	results, _ = s.SearchBooks("mccarthy")
	if len(results) != 1 {
		t.Errorf("author search: got %d, want 1", len(results))
	}

	results, _ = s.SearchBooks("western")
	if len(results) != 1 {
		t.Errorf("body search: got %d, want 1", len(results))
	}

	results, _ = s.SearchBooks("sci-fi")
	if len(results) != 1 {
		t.Errorf("tag search: got %d, want 1", len(results))
	}

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

func TestLoadBooksellers(t *testing.T) {
	s := testStore(t)

	sellers, err := s.LoadBooksellers()
	if err != nil {
		t.Fatalf("LoadBooksellers() error: %v", err)
	}
	if sellers != nil {
		t.Errorf("expected nil, got %v", sellers)
	}

	s.AddBookseller("TestShop", "https://example.com/search?q={query}")
	s.AddBookseller("AnotherShop", "https://another.com/?s={query}")

	sellers, err = s.LoadBooksellers()
	if err != nil {
		t.Fatalf("LoadBooksellers() error: %v", err)
	}
	if len(sellers) != 2 {
		t.Fatalf("got %d sellers, want 2", len(sellers))
	}
	if sellers[0].Name != "TestShop" {
		t.Errorf("sellers[0].Name = %q, want TestShop", sellers[0].Name)
	}
	if sellers[1].URL != "https://another.com/?s={query}" {
		t.Errorf("sellers[1].URL = %q", sellers[1].URL)
	}
}

func TestAddAndDeleteBookseller(t *testing.T) {
	s := testStore(t)

	bs, err := s.AddBookseller("Shop", "https://shop.com/{query}")
	if err != nil {
		t.Fatalf("AddBookseller() error: %v", err)
	}
	if bs.ID == 0 {
		t.Error("expected non-zero ID")
	}

	err = s.DeleteBookseller(bs.ID)
	if err != nil {
		t.Fatalf("DeleteBookseller() error: %v", err)
	}

	sellers, _ := s.LoadBooksellers()
	if len(sellers) != 0 {
		t.Errorf("expected 0 sellers after delete, got %d", len(sellers))
	}
}

func TestCreateBookWithID(t *testing.T) {
	s := testStore(t)

	meta := map[string]string{
		"status":     "read",
		"rating":     "5",
		"tags":       "sci-fi,classic",
		"date_added": "2026-03-01",
	}
	b, err := s.CreateBookWithID("f10d", "Dune", "Frank Herbert", meta)
	if err != nil {
		t.Fatalf("CreateBookWithID() error: %v", err)
	}
	if b.ID != "f10d" {
		t.Errorf("ID = %q, want f10d", b.ID)
	}
	if b.Title != "Dune" {
		t.Errorf("Title = %q", b.Title)
	}
	if b.Status != "read" {
		t.Errorf("Status = %q, want read", b.Status)
	}
	if b.Rating != 5 {
		t.Errorf("Rating = %d, want 5", b.Rating)
	}

	// Verify persisted
	got, err := s.GetBook("f10d")
	if err != nil {
		t.Fatalf("GetBook() error: %v", err)
	}
	if got.Title != "Dune" {
		t.Errorf("persisted Title = %q", got.Title)
	}

	// Duplicate ID should fail
	_, err = s.CreateBookWithID("f10d", "Another Book", "Another Author", nil)
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("testStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
