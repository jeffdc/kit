package storage

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// legacyDB creates a database shaped like production before versioned
// migrations: no user_version, no enrichment columns, an unused price_quotes
// column, and a row with an empty sort key.
func legacyDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "forage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
		CREATE TABLE books (
			id TEXT PRIMARY KEY, title TEXT NOT NULL, author TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'wishlist', tags TEXT DEFAULT '',
			rating INTEGER DEFAULT 0, date_added TEXT NOT NULL, date_read TEXT DEFAULT '',
			body TEXT DEFAULT '', price_quotes TEXT DEFAULT ''
		);
		CREATE TABLE booksellers (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, url TEXT NOT NULL);
		INSERT INTO books (id, title, author, status, date_added, price_quotes)
			VALUES ('a1b2', 'Dune', 'Frank Herbert', 'read', '2020-01-01', '[{"seller_name":"X"}]');
	`)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestMigrateLegacyDatabase(t *testing.T) {
	dir := legacyDB(t)
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() on legacy DB: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	var version int
	s.db.QueryRow("PRAGMA user_version").Scan(&version)
	if version != len(migrations) {
		t.Errorf("user_version = %d, want %d", version, len(migrations))
	}

	b, err := s.GetBook("a1b2")
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	if b.SortAuthor != "Herbert, Frank" {
		t.Errorf("SortAuthor = %q, want backfilled", b.SortAuthor)
	}
	if b.PageCount != 0 || b.ISBN != "" {
		t.Errorf("new columns should default empty, got %+v", b)
	}

	// The legacy row and the new columns are all writable.
	if _, err := s.UpdateBook("a1b2", map[string]string{"isbn": "9780441172719"}); err != nil {
		t.Fatalf("UpdateBook after migration: %v", err)
	}
}

func TestMigrateIsIdempotentAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.CreateBook("Dune", "Frank Herbert", nil)
	s.Close()

	s, err = New(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if _, err := s.GetBook(b.ID); err != nil {
		t.Fatalf("book lost across reopen: %v", err)
	}
}

func TestMigrateRefusesNewerSchema(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.db.Exec("PRAGMA user_version = 999")
	s.Close()

	if _, err := New(dir); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("New() with future schema: err = %v, want refusal", err)
	}
}

func TestUpdateAuthorRefreshesSortKey(t *testing.T) {
	s := testStore(t)
	b, _ := s.CreateBook("Babel", "R.F. Kuang", nil)

	got, err := s.UpdateBook(b.ID, map[string]string{"author": "Rebecca F. Kuang"})
	if err != nil {
		t.Fatal(err)
	}
	if got.SortAuthor != "Kuang, Rebecca F." {
		t.Errorf("SortAuthor = %q, want refreshed", got.SortAuthor)
	}
	persisted, _ := s.GetBook(b.ID)
	if persisted.SortAuthor != "Kuang, Rebecca F." {
		t.Errorf("persisted SortAuthor = %q", persisted.SortAuthor)
	}
}

func TestUnchangedAuthorKeepsManualSortKey(t *testing.T) {
	s := testStore(t)
	b, _ := s.CreateBook("Babel", "R. F. Kuang", nil)
	s.UpdateBook(b.ID, map[string]string{"sort_author": "Kuang, R.F."})

	// The PWA resends the author on every save.
	got, err := s.UpdateBook(b.ID, map[string]string{"author": "R. F. Kuang"})
	if err != nil {
		t.Fatal(err)
	}
	if got.SortAuthor != "Kuang, R.F." {
		t.Errorf("SortAuthor = %q, manual override lost", got.SortAuthor)
	}
}

func TestCreateBookWithIDRejectsBadInput(t *testing.T) {
	s := testStore(t)
	for _, id := range []string{"", "abc", "ZZZZ", "a1b2/../x", strings.Repeat("a", 33)} {
		if _, err := s.CreateBookWithID(id, "T", "A", nil); err == nil {
			t.Errorf("CreateBookWithID(%q) accepted invalid id", id)
		}
	}
	if _, err := s.CreateBookWithID("a1b2c3d4e5f6", "", "A", nil); err == nil {
		t.Error("accepted empty title")
	}
	if _, err := s.CreateBookWithID("a1b2c3d4e5f6", "T", "A", map[string]string{"rating": "9"}); err == nil {
		t.Error("accepted rating 9")
	}

	if _, err := s.CreateBookWithID("a1b2c3d4e5f6", "T", "A", nil); err != nil {
		t.Fatal(err)
	}
	_, err := s.CreateBookWithID("a1b2c3d4e5f6", "T2", "A2", nil)
	if !errors.Is(err, ErrExists) {
		t.Errorf("duplicate id: err = %v, want ErrExists", err)
	}
}

func TestMigrateOwnedStatusToFlag(t *testing.T) {
	dir := legacyDB(t) // a1b2: status read
	db, err := sql.Open("sqlite", filepath.Join(dir, "forage.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO books (id, title, author, status, date_added) VALUES
		('c3d4', 'Babel', 'R. F. Kuang', 'owned', '2024-01-01')`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	b, _ := s.GetBook("c3d4")
	if b.Status != "to_read" || !b.Owned {
		t.Errorf("owned book after migration: status %q, owned %v", b.Status, b.Owned)
	}
	if read, _ := s.GetBook("a1b2"); read.Status != "read" || read.Owned {
		t.Errorf("read book after migration: status %q, owned %v", read.Status, read.Owned)
	}
	// Clients synced before the migration (at rev 1) learn about the change.
	c, err := s.ChangesSince(1)
	if err != nil {
		t.Fatal(err)
	}
	if c.Full || len(c.Books) != 1 || c.Books[0].ID != "c3d4" {
		t.Errorf("feed since rev 1: %+v", c)
	}
}
