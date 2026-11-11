package model

import "strings"

// StatusList is the canonical ordered list of valid statuses. A status is
// where the book stands as reading; whether it's owned is Book.Owned.
var StatusList = []string{"wishlist", "to_read", "reading", "paused", "read", "dropped"}

// LegacyOwned is the status "to_read" had before ownership became its own
// field. Writes that still send it get "to_read" with Owned set.
const LegacyOwned = "owned"

var validStatuses = func() map[string]bool {
	m := make(map[string]bool, len(StatusList))
	for _, s := range StatusList {
		m[s] = true
	}
	return m
}()

func ValidStatus(s string) bool {
	return validStatuses[s]
}

func IsTerminal(s string) bool {
	return s == "dropped"
}

// AuthorSortKey derives a "Last, First" sort key from a full author name.
func AuthorSortKey(author string) string {
	if author == "" {
		return ""
	}
	parts := strings.Fields(author)
	if len(parts) == 1 {
		return author
	}
	last := parts[len(parts)-1]
	rest := strings.Join(parts[:len(parts)-1], " ")
	return last + ", " + rest
}

type Bookseller struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Book struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Author     string   `json:"author"`
	Status     string   `json:"status"`
	Tags       []string `json:"tags,omitempty"`
	Rating     int      `json:"rating,omitempty"`
	DateAdded  string   `json:"date_added"`
	Body       string   `json:"body,omitempty"`
	SortAuthor string   `json:"sort_author,omitempty"`
	PageCount  int      `json:"page_count,omitempty"`
	// FirstPublished is a year.
	FirstPublished int    `json:"first_published,omitempty"`
	ISBN           string `json:"isbn,omitempty"`
	// Owned is whether there's a copy on the shelf, whatever the status.
	Owned bool `json:"owned,omitempty"`

	// DateRead is the most recent date_finished among the book's reads.
	// It is derived, read-only, and kept for the legacy /api/books response.
	DateRead string `json:"date_read,omitempty"`

	// Sync bookkeeping. Rev increases on every change; deleted books are
	// kept as tombstones so clients learn about the deletion.
	Rev       int64  `json:"rev,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Deleted   bool   `json:"deleted,omitempty"`
}

// Read is one reading of a book. A book can be read many times.
// Dates are YYYY-MM-DD; an empty DateFinished means the date is unknown.
type Read struct {
	ID           string `json:"id"`
	BookID       string `json:"book_id"`
	DateFinished string `json:"date_finished"`
	DateStarted  string `json:"date_started,omitempty"`
	Rev          int64  `json:"rev,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
	Deleted      bool   `json:"deleted,omitempty"`
}

// Op is one change sent by a client. OpID is chosen by the client and makes
// the op idempotent: replaying an op returns its original result.
//
// Types:
//
//	book.create  Book (with client-chosen ID)
//	book.update  ID + Fields
//	book.delete  ID
//	read.create  Read (with client-chosen ID)
//	read.update  ID + Fields (date_finished, date_started)
//	read.delete  ID
type Op struct {
	OpID   string         `json:"op_id"`
	Type   string         `json:"type"`
	ID     string         `json:"id,omitempty"`
	Book   *Book          `json:"book,omitempty"`
	Read   *Read          `json:"read,omitempty"`
	Fields map[string]any `json:"fields,omitempty"`
}

// OpResult reports what happened to one Op. Status is "ok" or "rejected".
// A rejected op will never succeed (bad input, unknown ID, ID taken), so the
// client should drop it rather than retry.
type OpResult struct {
	OpID   string `json:"op_id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Rev    int64  `json:"rev,omitempty"`
}
