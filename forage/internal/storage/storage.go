package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"forage/internal/model"

	_ "modernc.org/sqlite"
)

// ErrExists is returned when creating a book whose ID is already taken.
var ErrExists = errors.New("book already exists")

type Store struct {
	db *sql.DB
}

var defaultBooksellers = []model.Bookseller{
	{Name: "Better World Books", URL: "https://www.betterworldbooks.com/search/results?q={query}"},
	{Name: "Biblio", URL: "https://www.biblio.com/search.php?keyisbn={query}"},
	{Name: "Bookshop.org", URL: "https://bookshop.org/search?keywords={query}"},
	{Name: "ThriftBooks", URL: "https://www.thriftbooks.com/browse/?b.search={query}"},
	{Name: "AbeBooks", URL: "https://www.abebooks.com/servlet/SearchResults?kn={query}"},
	{Name: "WorldCat", URL: "https://search.worldcat.org/search?q={query}"},
	{Name: "Goodreads", URL: "https://www.goodreads.com/search?q={query}"},
	{Name: "Amazon", URL: "https://www.amazon.com/s?k={query}&i=stripbooks"},
}

const bookColumns = "id, title, author, status, tags, rating, date_added, date_read, body, sort_author, page_count, first_published, isbn"

// New opens (creating if needed) the database in dir and brings its schema up to date.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	dsn := "file:" + filepath.Join(dir, "forage.db") +
		"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	// One process, one writer: a single connection serializes access and
	// avoids SQLITE_BUSY without needing WAL (whose shared-memory file may
	// not behave on the Sprite's filesystem).
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	if err := ensureDefaultBooksellers(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("seeding booksellers: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func ensureDefaultBooksellers(db *sql.DB) error {
	for _, seller := range defaultBooksellers {
		var exists int
		if err := db.QueryRow(
			"SELECT COUNT(*) FROM booksellers WHERE name = ? OR url = ?",
			seller.Name, seller.URL,
		).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		if _, err := db.Exec("INSERT INTO booksellers (name, url) VALUES (?, ?)", seller.Name, seller.URL); err != nil {
			return err
		}
	}
	return nil
}

// NewID returns a random 12-hex-character book ID.
func NewID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ValidID reports whether id is an acceptable book ID: 4–32 lowercase hex characters.
func ValidID(id string) bool {
	if len(id) < 4 || len(id) > 32 {
		return false
	}
	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// CreateBook creates a book with a freshly generated ID.
func (s *Store) CreateBook(title, author string, meta map[string]string) (*model.Book, error) {
	for range 5 {
		id, err := NewID()
		if err != nil {
			return nil, err
		}
		b, err := s.CreateBookWithID(id, title, author, meta)
		if errors.Is(err, ErrExists) {
			continue
		}
		return b, err
	}
	return nil, fmt.Errorf("could not generate a unique ID")
}

// CreateBookWithID creates a book with a caller-chosen ID (used for PWA-created books).
func (s *Store) CreateBookWithID(id, title, author string, meta map[string]string) (*model.Book, error) {
	if !ValidID(id) {
		return nil, fmt.Errorf("invalid book id: %q", id)
	}
	if strings.TrimSpace(title) == "" || strings.TrimSpace(author) == "" {
		return nil, fmt.Errorf("title and author are required")
	}

	b := &model.Book{
		ID:         id,
		Title:      title,
		Author:     author,
		Status:     "wishlist",
		SortAuthor: model.AuthorSortKey(author),
		DateAdded:  time.Now().Format("2006-01-02"),
	}
	if err := applyMeta(b, meta); err != nil {
		return nil, err
	}

	res, err := s.db.Exec(
		"INSERT INTO books ("+bookColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING",
		b.ID, b.Title, b.Author, b.Status, strings.Join(b.Tags, ","), b.Rating, b.DateAdded, b.DateRead,
		b.Body, b.SortAuthor, b.PageCount, b.FirstPublished, b.ISBN,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting book: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("%w: %s", ErrExists, id)
	}
	return b, nil
}

func (s *Store) GetBook(id string) (*model.Book, error) {
	row := s.db.QueryRow("SELECT "+bookColumns+" FROM books WHERE id = ?", id)
	b, err := scanBook(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("book not found: %s", id)
	}
	return b, err
}

// ListBooks returns all books.
func (s *Store) ListBooks() ([]model.Book, error) {
	rows, err := s.db.Query("SELECT " + bookColumns + " FROM books")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []model.Book
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			return nil, err
		}
		books = append(books, *b)
	}
	return books, rows.Err()
}

var validKeys = map[string]bool{
	"title": true, "author": true, "status": true,
	"rating": true, "tags": true, "date_read": true, "body": true,
	"sort_author": true, "page_count": true,
	"first_published": true, "isbn": true,
}

// UpdateBook sets one field on a book. Changing the author also refreshes
// the author sort key.
func (s *Store) UpdateBook(id, key, value string) (*model.Book, error) {
	if !validKeys[key] {
		return nil, fmt.Errorf("invalid key: %s", key)
	}

	b, err := s.GetBook(id)
	if err != nil {
		return nil, err
	}
	if err := applyMeta(b, map[string]string{key: value}); err != nil {
		return nil, err
	}

	_, err = s.db.Exec(
		`UPDATE books SET title = ?, author = ?, status = ?, tags = ?, rating = ?, date_read = ?, body = ?,
		 sort_author = ?, page_count = ?, first_published = ?, isbn = ? WHERE id = ?`,
		b.Title, b.Author, b.Status, strings.Join(b.Tags, ","), b.Rating, b.DateRead, b.Body,
		b.SortAuthor, b.PageCount, b.FirstPublished, b.ISBN, id,
	)
	if err != nil {
		return nil, fmt.Errorf("updating book: %w", err)
	}
	return b, nil
}

func (s *Store) DeleteBook(id string) error {
	result, err := s.db.Exec("DELETE FROM books WHERE id = ?", id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("book not found: %s", id)
	}
	return nil
}

func (s *Store) LoadBooksellers() ([]model.Bookseller, error) {
	rows, err := s.db.Query("SELECT id, name, url FROM booksellers ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sellers []model.Bookseller
	for rows.Next() {
		var bs model.Bookseller
		if err := rows.Scan(&bs.ID, &bs.Name, &bs.URL); err != nil {
			return nil, err
		}
		sellers = append(sellers, bs)
	}
	return sellers, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanBook(sc scanner) (*model.Book, error) {
	var b model.Book
	var tags string
	if err := sc.Scan(&b.ID, &b.Title, &b.Author, &b.Status, &tags, &b.Rating, &b.DateAdded, &b.DateRead,
		&b.Body, &b.SortAuthor, &b.PageCount, &b.FirstPublished, &b.ISBN); err != nil {
		return nil, err
	}
	b.Tags = splitTags(tags)
	return &b, nil
}

func splitTags(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func applyMeta(b *model.Book, meta map[string]string) error {
	_, sortGiven := meta["sort_author"]
	for k, v := range meta {
		switch k {
		case "title":
			b.Title = v
		case "author":
			// The PWA sends the author on every save, so only a real change
			// may reset the sort key (which can be a manual override).
			if v != b.Author && !sortGiven {
				b.SortAuthor = model.AuthorSortKey(v)
			}
			b.Author = v
		case "status":
			if !model.ValidStatus(v) {
				return fmt.Errorf("invalid status: %s (valid: %s)", v, strings.Join(model.StatusList, ", "))
			}
			b.Status = v
		case "rating":
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || n > 5 {
				return fmt.Errorf("invalid rating: %s", v)
			}
			b.Rating = n
		case "tags":
			b.Tags = splitTags(v)
		case "date_added":
			b.DateAdded = v
		case "date_read":
			b.DateRead = v
		case "body":
			b.Body = v
		case "sort_author":
			b.SortAuthor = v
		case "page_count":
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid page_count: %s", v)
			}
			b.PageCount = n
		case "first_published":
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid first_published: %s", v)
			}
			b.FirstPublished = n
		case "isbn":
			b.ISBN = v
		}
	}
	return nil
}
