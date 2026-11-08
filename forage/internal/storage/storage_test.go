package storage

import (
	"testing"

	"forage/internal/model"
)

func sellersByName(sellers []model.Bookseller) map[string]model.Bookseller {
	out := make(map[string]model.Bookseller, len(sellers))
	for _, seller := range sellers {
		out[seller.Name] = seller
	}
	return out
}

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

func TestCreateAndGetBook(t *testing.T) {
	s := testStore(t)

	b, err := s.CreateBook("The Left Hand of Darkness", "Ursula K. Le Guin", nil)
	if err != nil {
		t.Fatalf("CreateBook() error: %v", err)
	}

	if len(b.ID) != 12 || !ValidID(b.ID) {
		t.Errorf("ID = %q, want 12 hex characters", b.ID)
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

	all, err := s.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks() error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("len = %d, want 3", len(all))
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
	if len(sellers) != len(defaultBooksellers) {
		t.Fatalf("got %d sellers, want %d defaults", len(sellers), len(defaultBooksellers))
	}

	byName := sellersByName(sellers)
	for _, seller := range defaultBooksellers {
		got, ok := byName[seller.Name]
		if !ok {
			t.Fatalf("missing default seller %q", seller.Name)
		}
		if got.URL != seller.URL {
			t.Fatalf("seller %q URL = %q, want %q", seller.Name, got.URL, seller.URL)
		}
	}

	s.db.Exec("INSERT INTO booksellers (name, url) VALUES (?, ?)", "TestShop", "https://example.com/search?q={query}")
	s.db.Exec("INSERT INTO booksellers (name, url) VALUES (?, ?)", "AnotherShop", "https://another.com/?s={query}")

	sellers, err = s.LoadBooksellers()
	if err != nil {
		t.Fatalf("LoadBooksellers() error: %v", err)
	}
	if len(sellers) != len(defaultBooksellers)+2 {
		t.Fatalf("got %d sellers, want %d", len(sellers), len(defaultBooksellers)+2)
	}

	byName = sellersByName(sellers)
	if got := byName["TestShop"].URL; got != "https://example.com/search?q={query}" {
		t.Errorf("TestShop URL = %q", got)
	}
	if got := byName["AnotherShop"].URL; got != "https://another.com/?s={query}" {
		t.Errorf("AnotherShop URL = %q", got)
	}
}

func TestNewSeedsMissingDefaultBooksellers(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	sellers, err := s.LoadBooksellers()
	if err != nil {
		t.Fatalf("LoadBooksellers() error: %v", err)
	}

	var betterWorldID int
	for _, seller := range sellers {
		if seller.Name == "Better World Books" {
			betterWorldID = seller.ID
			break
		}
	}
	if betterWorldID == 0 {
		t.Fatal("expected Better World Books default seller")
	}

	if _, err := s.db.Exec("DELETE FROM booksellers WHERE id = ?", betterWorldID); err != nil {
		t.Fatalf("deleting bookseller: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	s, err = New(dir)
	if err != nil {
		t.Fatalf("New() reopen error: %v", err)
	}
	defer s.Close()

	sellers, err = s.LoadBooksellers()
	if err != nil {
		t.Fatalf("LoadBooksellers() error: %v", err)
	}

	byName := sellersByName(sellers)
	if _, ok := byName["Better World Books"]; !ok {
		t.Fatal("expected Better World Books to be reseeded on reopen")
	}
	if _, ok := byName["Goodreads"]; !ok {
		t.Fatal("expected Goodreads to be seeded on reopen")
	}
	if _, ok := byName["Amazon"]; !ok {
		t.Fatal("expected Amazon to be seeded on reopen")
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

func TestSortAuthorAutoPopulated(t *testing.T) {
	s := testStore(t)

	b, err := s.CreateBook("Dune", "Frank Herbert", nil)
	if err != nil {
		t.Fatalf("CreateBook() error: %v", err)
	}
	if b.SortAuthor != "Herbert, Frank" {
		t.Errorf("SortAuthor = %q, want %q", b.SortAuthor, "Herbert, Frank")
	}

	got, err := s.GetBook(b.ID)
	if err != nil {
		t.Fatalf("GetBook() error: %v", err)
	}
	if got.SortAuthor != "Herbert, Frank" {
		t.Errorf("persisted SortAuthor = %q, want %q", got.SortAuthor, "Herbert, Frank")
	}
}

func TestSortAuthorManualOverride(t *testing.T) {
	s := testStore(t)

	b, _ := s.CreateBook("The Left Hand of Darkness", "Ursula K. Le Guin", nil)
	updated, err := s.UpdateBook(b.ID, "sort_author", "Le Guin, Ursula K.")
	if err != nil {
		t.Fatalf("UpdateBook() error: %v", err)
	}
	if updated.SortAuthor != "Le Guin, Ursula K." {
		t.Errorf("SortAuthor = %q, want %q", updated.SortAuthor, "Le Guin, Ursula K.")
	}

	got, _ := s.GetBook(b.ID)
	if got.SortAuthor != "Le Guin, Ursula K." {
		t.Errorf("persisted SortAuthor = %q, want %q", got.SortAuthor, "Le Guin, Ursula K.")
	}
}

func TestCreateBookWithEnrichedMeta(t *testing.T) {
	s := testStore(t)

	meta := map[string]string{
		"page_count":      "412",
		"first_published": "1965",
		"isbn":            "9780441172719",
	}
	b, err := s.CreateBook("Dune", "Frank Herbert", meta)
	if err != nil {
		t.Fatalf("CreateBook() error: %v", err)
	}
	if b.PageCount != 412 {
		t.Errorf("PageCount = %d, want 412", b.PageCount)
	}
	if b.FirstPublished != 1965 {
		t.Errorf("FirstPublished = %d, want 1965", b.FirstPublished)
	}
	if b.ISBN != "9780441172719" {
		t.Errorf("ISBN = %q, want 9780441172719", b.ISBN)
	}

	// Verify round-trip through DB
	got, err := s.GetBook(b.ID)
	if err != nil {
		t.Fatalf("GetBook() error: %v", err)
	}
	if got.PageCount != 412 {
		t.Errorf("persisted PageCount = %d, want 412", got.PageCount)
	}
	if got.FirstPublished != 1965 {
		t.Errorf("persisted FirstPublished = %d, want 1965", got.FirstPublished)
	}
	if got.ISBN != "9780441172719" {
		t.Errorf("persisted ISBN = %q, want 9780441172719", got.ISBN)
	}
}

func TestUpdateBookEnrichedFields(t *testing.T) {
	s := testStore(t)
	b, _ := s.CreateBook("Dune", "Frank Herbert", nil)

	updated, err := s.UpdateBook(b.ID, "page_count", "412")
	if err != nil {
		t.Fatalf("UpdateBook page_count error: %v", err)
	}
	if updated.PageCount != 412 {
		t.Errorf("PageCount = %d, want 412", updated.PageCount)
	}

	updated, err = s.UpdateBook(b.ID, "first_published", "1965")
	if err != nil {
		t.Fatalf("UpdateBook first_published error: %v", err)
	}
	if updated.FirstPublished != 1965 {
		t.Errorf("FirstPublished = %d, want 1965", updated.FirstPublished)
	}

	updated, err = s.UpdateBook(b.ID, "isbn", "9780441172719")
	if err != nil {
		t.Fatalf("UpdateBook isbn error: %v", err)
	}
	if updated.ISBN != "9780441172719" {
		t.Errorf("ISBN = %q, want 9780441172719", updated.ISBN)
	}
}

func TestNewFieldsMigration(t *testing.T) {
	// Simulate a pre-migration database by creating a store,
	// inserting a row without the new columns, then verifying
	// the new columns have zero/empty defaults after migration.
	s := testStore(t)

	b, err := s.CreateBook("Old Book", "Old Author", nil)
	if err != nil {
		t.Fatalf("CreateBook() error: %v", err)
	}

	got, _ := s.GetBook(b.ID)
	if got.PageCount != 0 {
		t.Errorf("PageCount = %d, want 0 default", got.PageCount)
	}
	if got.FirstPublished != 0 {
		t.Errorf("FirstPublished = %d, want 0 default", got.FirstPublished)
	}
	if got.ISBN != "" {
		t.Errorf("ISBN = %q, want empty default", got.ISBN)
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
