package storage

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"forage/internal/model"

	_ "modernc.org/sqlite"
)

type Store struct {
	root string
	db   *sql.DB
}

func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(root, "forage.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS books (
			id         TEXT PRIMARY KEY,
			title      TEXT NOT NULL,
			author     TEXT NOT NULL,
			status     TEXT NOT NULL DEFAULT 'wishlist',
			tags       TEXT DEFAULT '',
			rating     INTEGER DEFAULT 0,
			date_added TEXT NOT NULL,
			date_read  TEXT DEFAULT '',
			body       TEXT DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS booksellers (
			id   INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url  TEXT NOT NULL
		);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating tables: %w", err)
	}

	return &Store{root: root, db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func DefaultRoot() string {
	if dir := os.Getenv("FORAGE_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forage")
}

func (s *Store) Root() string { return s.root }

func (s *Store) generateID(title, author string) (string, error) {
	t := time.Now()
	for i := range 100 {
		ts := t.Add(time.Duration(i) * time.Nanosecond).Format(time.RFC3339Nano)
		h := sha256.Sum256([]byte(title + author + ts))
		id := fmt.Sprintf("%x", h[:2])

		var exists int
		err := s.db.QueryRow("SELECT COUNT(*) FROM books WHERE id = ?", id).Scan(&exists)
		if err != nil {
			return "", err
		}
		if exists == 0 {
			return id, nil
		}
	}
	return "", fmt.Errorf("could not generate unique ID after 100 attempts")
}

func (s *Store) CreateBook(title, author string, meta map[string]string) (*model.Book, error) {
	id, err := s.generateID(title, author)
	if err != nil {
		return nil, err
	}

	today := time.Now().Format("2006-01-02")
	b := &model.Book{
		ID:        id,
		Title:     title,
		Author:    author,
		Status:    "wishlist",
		DateAdded: today,
	}

	if meta != nil {
		if err := applyMeta(b, meta); err != nil {
			return nil, err
		}
	}

	tags := strings.Join(b.Tags, ",")
	_, err = s.db.Exec(
		"INSERT INTO books (id, title, author, status, tags, rating, date_added, date_read, body) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		b.ID, b.Title, b.Author, b.Status, tags, b.Rating, b.DateAdded, b.DateRead, b.Body,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting book: %w", err)
	}
	return b, nil
}

func (s *Store) GetBook(id string) (*model.Book, error) {
	b := &model.Book{}
	var tags string
	err := s.db.QueryRow(
		"SELECT id, title, author, status, tags, rating, date_added, date_read, body FROM books WHERE id = ?", id,
	).Scan(&b.ID, &b.Title, &b.Author, &b.Status, &tags, &b.Rating, &b.DateAdded, &b.DateRead, &b.Body)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("book not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	b.Tags = splitTags(tags)
	return b, nil
}

func (s *Store) ListBooks(filters map[string]string) ([]model.Book, error) {
	query := "SELECT id, title, author, status, tags, rating, date_added, date_read, body FROM books"
	var conditions []string
	var args []any

	for k, v := range filters {
		switch k {
		case "status":
			conditions = append(conditions, "status = ?")
			args = append(args, v)
		case "tag":
			conditions = append(conditions, "(tags = ? OR tags LIKE ? OR tags LIKE ? OR tags LIKE ?)")
			args = append(args, v, v+",%", "%,"+v, "%,"+v+",%")
		case "author":
			conditions = append(conditions, "author = ? COLLATE NOCASE")
			args = append(args, v)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBooks(rows)
}

func (s *Store) SearchBooks(query string) ([]model.Book, error) {
	q := "%" + query + "%"
	rows, err := s.db.Query(
		"SELECT id, title, author, status, tags, rating, date_added, date_read, body FROM books WHERE title LIKE ? COLLATE NOCASE OR author LIKE ? COLLATE NOCASE OR body LIKE ? COLLATE NOCASE OR tags LIKE ? COLLATE NOCASE",
		q, q, q, q,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBooks(rows)
}

var validKeys = map[string]bool{
	"title": true, "author": true, "status": true,
	"rating": true, "tags": true, "date_read": true,
}

func (s *Store) UpdateBook(id, key, value string) (*model.Book, error) {
	if !validKeys[key] {
		return nil, fmt.Errorf("invalid key: %s (valid: title, author, status, rating, tags, date_read)", key)
	}

	b, err := s.GetBook(id)
	if err != nil {
		return nil, err
	}

	if err := applyMeta(b, map[string]string{key: value}); err != nil {
		return nil, err
	}

	var column string
	var dbValue any
	switch key {
	case "title":
		column, dbValue = "title", b.Title
	case "author":
		column, dbValue = "author", b.Author
	case "status":
		column, dbValue = "status", b.Status
	case "rating":
		column, dbValue = "rating", b.Rating
	case "tags":
		column, dbValue = "tags", strings.Join(b.Tags, ",")
	case "date_read":
		column, dbValue = "date_read", b.DateRead
	}

	_, err = s.db.Exec("UPDATE books SET "+column+" = ? WHERE id = ?", dbValue, id)
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
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("book not found: %s", id)
	}
	return nil
}

func (s *Store) LoadBooksellers() ([]model.Bookseller, error) {
	rows, err := s.db.Query("SELECT id, name, url FROM booksellers")
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

func (s *Store) AddBookseller(name, url string) (*model.Bookseller, error) {
	result, err := s.db.Exec("INSERT INTO booksellers (name, url) VALUES (?, ?)", name, url)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &model.Bookseller{ID: int(id), Name: name, URL: url}, nil
}

func (s *Store) DeleteBookseller(id int) error {
	result, err := s.db.Exec("DELETE FROM booksellers WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("bookseller not found: %d", id)
	}
	return nil
}

func scanBooks(rows *sql.Rows) ([]model.Book, error) {
	var books []model.Book
	for rows.Next() {
		var b model.Book
		var tags string
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.Status, &tags, &b.Rating, &b.DateAdded, &b.DateRead, &b.Body); err != nil {
			return nil, err
		}
		b.Tags = splitTags(tags)
		books = append(books, b)
	}
	return books, rows.Err()
}

func splitTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func applyMeta(b *model.Book, meta map[string]string) error {
	for k, v := range meta {
		switch k {
		case "title":
			b.Title = v
		case "author":
			b.Author = v
		case "status":
			if !model.ValidStatus(v) {
				return fmt.Errorf("invalid status: %s (valid: wishlist, reading, paused, read, dropped)", v)
			}
			b.Status = v
		case "rating":
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid rating: %s", v)
			}
			b.Rating = n
		case "tags":
			tags := strings.Split(v, ",")
			for i := range tags {
				tags[i] = strings.TrimSpace(tags[i])
			}
			b.Tags = tags
		case "date_added":
			b.DateAdded = v
		case "date_read":
			b.DateRead = v
		case "body":
			b.Body = v
		}
	}
	return nil
}
