package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/binary"
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

// Errors that mean the request can never succeed as sent. API handlers turn
// them into 4xx responses or "rejected" op results; anything else is a 5xx.
var (
	ErrNotFound = errors.New("not found")
	ErrExists   = errors.New("already exists")
	ErrInvalid  = errors.New("invalid")
)

// IsRejection reports whether err is a permanent, client-caused failure.
func IsRejection(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, ErrExists) || errors.Is(err, ErrInvalid)
}

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

// appliedOpsRetention bounds how long op IDs are remembered for replay.
const appliedOpsRetention = 90 * 24 * time.Hour

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
	cutoff := time.Now().UTC().Add(-appliedOpsRetention).Format(time.RFC3339)
	if _, err := db.Exec("DELETE FROM applied_ops WHERE applied_at < ?", cutoff); err != nil {
		db.Close()
		return nil, fmt.Errorf("pruning applied ops: %w", err)
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

// --- IDs, dates, helpers ---

// NewID returns a random 12-hex-character ID for a book or read.
func NewID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ValidID reports whether id is an acceptable book or read ID: 4–32 lowercase hex characters.
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

func randomEpoch() (int64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(b[:]) >> 12), nil // keep it well inside JS's safe-integer range
}

func nowStamp() string { return time.Now().UTC().Format(time.RFC3339) }

// Today is the current date in the server's local time zone (FORAGE_TZ).
func Today() string { return time.Now().Format("2006-01-02") }

// validDate accepts "" (unknown) or a real YYYY-MM-DD date.
func validDate(d string) bool {
	if d == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", d)
	return err == nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalid}, args...)...)
}

// write runs fn in a transaction with a freshly bumped revision number.
// Every change to books or reads goes through here, so revisions are
// strictly increasing and the sync feed never misses a change.
func (s *Store) write(fn func(tx *sql.Tx, rev int64, now string) error) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	var rev int64
	if err := tx.QueryRow("UPDATE meta SET value = value + 1 WHERE key = 'rev' RETURNING value").Scan(&rev); err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("bumping rev: %w", err)
	}
	if err := fn(tx, rev, nowStamp()); err != nil {
		tx.Rollback()
		return 0, err
	}
	return rev, tx.Commit()
}

type queryer interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
}

// --- Books ---

const bookSelect = `SELECT b.id, b.title, b.author, b.status, b.tags, b.rating, b.date_added, b.body,
	b.sort_author, b.page_count, b.first_published, b.isbn, b.rev, b.updated_at, b.deleted,
	COALESCE((SELECT MAX(r.date_finished) FROM reads r WHERE r.book_id = b.id AND r.deleted = 0), '')
	FROM books b`

type scanner interface {
	Scan(dest ...any) error
}

func scanBook(sc scanner) (*model.Book, error) {
	var b model.Book
	var tags string
	if err := sc.Scan(&b.ID, &b.Title, &b.Author, &b.Status, &tags, &b.Rating, &b.DateAdded, &b.Body,
		&b.SortAuthor, &b.PageCount, &b.FirstPublished, &b.ISBN, &b.Rev, &b.UpdatedAt, &b.Deleted,
		&b.DateRead); err != nil {
		return nil, err
	}
	b.Tags = splitTags(tags)
	return &b, nil
}

func getBook(q queryer, id string) (*model.Book, error) {
	b, err := scanBook(q.QueryRow(bookSelect+" WHERE b.id = ? AND b.deleted = 0", id))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: book %s", ErrNotFound, id)
	}
	return b, err
}

func queryBooks(q queryer, where string, args ...any) ([]model.Book, error) {
	rows, err := q.Query(bookSelect+" "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	books := []model.Book{}
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			return nil, err
		}
		books = append(books, *b)
	}
	return books, rows.Err()
}

// GetBook returns a book that has not been deleted.
func (s *Store) GetBook(id string) (*model.Book, error) {
	return getBook(s.db, id)
}

// ListBooks returns all books that have not been deleted.
func (s *Store) ListBooks() ([]model.Book, error) {
	return queryBooks(s.db, "WHERE b.deleted = 0")
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

// CreateBookWithID creates a book with a caller-chosen ID. meta holds
// optional fields as strings (see applyMeta).
func (s *Store) CreateBookWithID(id, title, author string, meta map[string]string) (*model.Book, error) {
	b := &model.Book{ID: id, Title: title, Author: author}
	if err := applyMeta(b, meta); err != nil {
		return nil, err
	}
	var out *model.Book
	_, err := s.write(func(tx *sql.Tx, rev int64, now string) error {
		var err error
		out, err = createBookTx(tx, rev, now, b)
		return err
	})
	return out, err
}

// createBookTx fills in defaults, validates, and inserts b.
func createBookTx(tx *sql.Tx, rev int64, now string, b *model.Book) (*model.Book, error) {
	nb := *b
	nb.Title = strings.TrimSpace(nb.Title)
	nb.Author = strings.TrimSpace(nb.Author)
	if nb.Status == "" {
		nb.Status = "wishlist"
	}
	if nb.SortAuthor == "" {
		nb.SortAuthor = model.AuthorSortKey(nb.Author)
	}
	if nb.DateAdded == "" {
		nb.DateAdded = Today()
	}
	nb.Tags = splitTags(strings.Join(nb.Tags, ","))
	nb.DateRead = ""
	nb.Rev, nb.UpdatedAt, nb.Deleted = rev, now, false
	if err := validateBook(&nb); err != nil {
		return nil, err
	}

	res, err := tx.Exec(
		`INSERT INTO books (id, title, author, status, tags, rating, date_added, body, sort_author,
		 page_count, first_published, isbn, rev, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`,
		nb.ID, nb.Title, nb.Author, nb.Status, strings.Join(nb.Tags, ","), nb.Rating, nb.DateAdded, nb.Body,
		nb.SortAuthor, nb.PageCount, nb.FirstPublished, nb.ISBN, rev, now,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting book: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("%w: book %s", ErrExists, nb.ID)
	}
	return &nb, nil
}

func validateBook(b *model.Book) error {
	switch {
	case !ValidID(b.ID):
		return invalid("book id %q", b.ID)
	case b.Title == "" || b.Author == "":
		return invalid("title and author are required")
	case !model.ValidStatus(b.Status):
		return invalid("status %q (valid: %s)", b.Status, strings.Join(model.StatusList, ", "))
	case b.Rating < 0 || b.Rating > 5:
		return invalid("rating %d", b.Rating)
	case !validDate(b.DateAdded) || b.DateAdded == "":
		return invalid("date_added %q", b.DateAdded)
	}
	return nil
}

// UpdateBook changes several fields of a book at once, atomically.
// Field values are strings (see applyMeta).
func (s *Store) UpdateBook(id string, fields map[string]string) (*model.Book, error) {
	var out *model.Book
	_, err := s.write(func(tx *sql.Tx, rev int64, now string) error {
		var err error
		out, err = updateBookTx(tx, rev, now, id, fields)
		return err
	})
	return out, err
}

var updatableBookFields = map[string]bool{
	"title": true, "author": true, "status": true, "rating": true, "tags": true, "body": true,
	"sort_author": true, "page_count": true, "first_published": true, "isbn": true, "date_added": true,
}

func updateBookTx(tx *sql.Tx, rev int64, now, id string, fields map[string]string) (*model.Book, error) {
	if len(fields) == 0 {
		return nil, invalid("no fields to update")
	}
	for k := range fields {
		if !updatableBookFields[k] {
			return nil, invalid("field %q cannot be updated", k)
		}
	}
	b, err := getBook(tx, id)
	if err != nil {
		return nil, err
	}
	if err := applyMeta(b, fields); err != nil {
		return nil, err
	}
	b.Title, b.Author = strings.TrimSpace(b.Title), strings.TrimSpace(b.Author)
	if err := validateBook(b); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(
		`UPDATE books SET title = ?, author = ?, status = ?, tags = ?, rating = ?, body = ?, sort_author = ?,
		 page_count = ?, first_published = ?, isbn = ?, date_added = ?, rev = ?, updated_at = ? WHERE id = ?`,
		b.Title, b.Author, b.Status, strings.Join(b.Tags, ","), b.Rating, b.Body, b.SortAuthor,
		b.PageCount, b.FirstPublished, b.ISBN, b.DateAdded, rev, now, id,
	); err != nil {
		return nil, fmt.Errorf("updating book: %w", err)
	}
	b.Rev, b.UpdatedAt = rev, now
	return b, nil
}

// DeleteBook marks a book and its reads deleted. The rows stay as
// tombstones so syncing clients learn about the deletion.
func (s *Store) DeleteBook(id string) error {
	_, err := s.write(func(tx *sql.Tx, rev int64, now string) error {
		return deleteBookTx(tx, rev, now, id)
	})
	return err
}

func deleteBookTx(tx *sql.Tx, rev int64, now, id string) error {
	res, err := tx.Exec("UPDATE books SET deleted = 1, rev = ?, updated_at = ? WHERE id = ? AND deleted = 0", rev, now, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: book %s", ErrNotFound, id)
	}
	_, err = tx.Exec("UPDATE reads SET deleted = 1, rev = ?, updated_at = ? WHERE book_id = ? AND deleted = 0", rev, now, id)
	return err
}

// --- Reads ---

const readSelect = "SELECT id, book_id, date_finished, date_started, rev, updated_at, deleted FROM reads"

func scanRead(sc scanner) (*model.Read, error) {
	var r model.Read
	err := sc.Scan(&r.ID, &r.BookID, &r.DateFinished, &r.DateStarted, &r.Rev, &r.UpdatedAt, &r.Deleted)
	return &r, err
}

func queryReads(q queryer, where string, args ...any) ([]model.Read, error) {
	rows, err := q.Query(readSelect+" "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reads := []model.Read{}
	for rows.Next() {
		r, err := scanRead(rows)
		if err != nil {
			return nil, err
		}
		reads = append(reads, *r)
	}
	return reads, rows.Err()
}

// ReadsForBook returns a book's reads, oldest first (undated reads first).
func (s *Store) ReadsForBook(bookID string) ([]model.Read, error) {
	return queryReads(s.db, "WHERE book_id = ? AND deleted = 0 ORDER BY date_finished, rowid", bookID)
}

// CreateRead records a read. r.ID is chosen by the caller (use NewID).
func (s *Store) CreateRead(r model.Read) (*model.Read, error) {
	var out *model.Read
	_, err := s.write(func(tx *sql.Tx, rev int64, now string) error {
		var err error
		out, err = createReadTx(tx, rev, now, r)
		return err
	})
	return out, err
}

func createReadTx(tx *sql.Tx, rev int64, now string, r model.Read) (*model.Read, error) {
	switch {
	case !ValidID(r.ID):
		return nil, invalid("read id %q", r.ID)
	case !validDate(r.DateFinished):
		return nil, invalid("date_finished %q", r.DateFinished)
	case !validDate(r.DateStarted):
		return nil, invalid("date_started %q", r.DateStarted)
	}
	if _, err := getBook(tx, r.BookID); err != nil {
		return nil, err
	}
	res, err := tx.Exec(
		`INSERT INTO reads (id, book_id, date_finished, date_started, rev, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`,
		r.ID, r.BookID, r.DateFinished, r.DateStarted, rev, now,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting read: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("%w: read %s", ErrExists, r.ID)
	}
	r.Rev, r.UpdatedAt, r.Deleted = rev, now, false
	return &r, nil
}

// UpdateRead changes a read's date_finished and/or date_started.
func (s *Store) UpdateRead(id string, fields map[string]string) (*model.Read, error) {
	var out *model.Read
	_, err := s.write(func(tx *sql.Tx, rev int64, now string) error {
		var err error
		out, err = updateReadTx(tx, rev, now, id, fields)
		return err
	})
	return out, err
}

func updateReadTx(tx *sql.Tx, rev int64, now, id string, fields map[string]string) (*model.Read, error) {
	if len(fields) == 0 {
		return nil, invalid("no fields to update")
	}
	r, err := scanRead(tx.QueryRow(readSelect+" WHERE id = ? AND deleted = 0", id))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: read %s", ErrNotFound, id)
	}
	if err != nil {
		return nil, err
	}
	for k, v := range fields {
		if !validDate(v) {
			return nil, invalid("%s %q", k, v)
		}
		switch k {
		case "date_finished":
			r.DateFinished = v
		case "date_started":
			r.DateStarted = v
		default:
			return nil, invalid("field %q cannot be updated", k)
		}
	}
	if _, err := tx.Exec("UPDATE reads SET date_finished = ?, date_started = ?, rev = ?, updated_at = ? WHERE id = ?",
		r.DateFinished, r.DateStarted, rev, now, id); err != nil {
		return nil, err
	}
	r.Rev, r.UpdatedAt = rev, now
	return r, nil
}

// DeleteRead marks a read deleted.
func (s *Store) DeleteRead(id string) error {
	_, err := s.write(func(tx *sql.Tx, rev int64, now string) error {
		return deleteReadTx(tx, rev, now, id)
	})
	return err
}

func deleteReadTx(tx *sql.Tx, rev int64, now, id string) error {
	res, err := tx.Exec("UPDATE reads SET deleted = 1, rev = ?, updated_at = ? WHERE id = ? AND deleted = 0", rev, now, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: read %s", ErrNotFound, id)
	}
	return nil
}

// SetLatestReadDate sets the finish date of the book's most recent read,
// creating a read if the book has none. It supports the legacy date_read field.
func (s *Store) SetLatestReadDate(bookID, date string) error {
	_, err := s.write(func(tx *sql.Tx, rev int64, now string) error {
		var readID string
		err := tx.QueryRow(`SELECT id FROM reads WHERE book_id = ? AND deleted = 0
			ORDER BY date_finished DESC, rowid DESC LIMIT 1`, bookID).Scan(&readID)
		if err == sql.ErrNoRows {
			id, err := NewID()
			if err != nil {
				return err
			}
			_, err = createReadTx(tx, rev, now, model.Read{ID: id, BookID: bookID, DateFinished: date})
			return err
		}
		if err != nil {
			return err
		}
		_, err = updateReadTx(tx, rev, now, readID, map[string]string{"date_finished": date})
		return err
	})
	return err
}

// --- Sync ---

// Changes is the payload of the sync feed.
type Changes struct {
	Epoch int64        `json:"epoch"`
	Rev   int64        `json:"rev"`
	Full  bool         `json:"full"`
	Books []model.Book `json:"books"`
	Reads []model.Read `json:"reads"`
}

// ChangesSince returns everything that changed after revision since,
// including deletions. since <= 0, or a since ahead of the database (for
// example after restoring a backup), returns a full snapshot without
// tombstones, marked Full.
func (s *Store) ChangesSince(since int64) (*Changes, error) {
	tx, err := s.db.Begin() // one consistent snapshot
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	c := &Changes{}
	if err := tx.QueryRow("SELECT value FROM meta WHERE key = 'rev'").Scan(&c.Rev); err != nil {
		return nil, err
	}
	if err := tx.QueryRow("SELECT value FROM meta WHERE key = 'epoch'").Scan(&c.Epoch); err != nil {
		return nil, err
	}

	if since <= 0 || since > c.Rev {
		c.Full = true
		if c.Books, err = queryBooks(tx, "WHERE b.deleted = 0"); err != nil {
			return nil, err
		}
		c.Reads, err = queryReads(tx, "WHERE deleted = 0")
		return c, err
	}
	if c.Books, err = queryBooks(tx, "WHERE b.rev > ?", since); err != nil {
		return nil, err
	}
	c.Reads, err = queryReads(tx, "WHERE rev > ?", since)
	return c, err
}

// --- Booksellers ---

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

// --- Field parsing ---

func splitTags(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// applyMeta sets book fields from string values.
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
				b.SortAuthor = model.AuthorSortKey(strings.TrimSpace(v))
			}
			b.Author = v
		case "status":
			b.Status = v
		case "rating", "page_count", "first_published":
			n := 0
			if v != "" {
				var err error
				if n, err = strconv.Atoi(v); err != nil {
					return invalid("%s %q", k, v)
				}
			}
			switch k {
			case "rating":
				b.Rating = n
			case "page_count":
				b.PageCount = n
			default:
				b.FirstPublished = n
			}
		case "tags":
			b.Tags = splitTags(v)
		case "date_added":
			b.DateAdded = v
		case "body":
			b.Body = v
		case "sort_author":
			b.SortAuthor = v
		case "isbn":
			b.ISBN = strings.TrimSpace(v)
		}
	}
	return nil
}

// FieldString converts a JSON-decoded field value to the string form
// UpdateBook and UpdateRead expect: whole numbers without decimals, arrays
// joined with commas, null as "".
func FieldString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = FieldString(e)
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(x)
	}
}
