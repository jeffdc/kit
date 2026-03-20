package model

import "strings"

// StatusList is the canonical ordered list of valid statuses.
var StatusList = []string{"wishlist", "owned", "reading", "paused", "read", "dropped"}

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
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Author    string   `json:"author"`
	Status    string   `json:"status"`
	Tags      []string `json:"tags,omitempty"`
	Rating    int      `json:"rating,omitempty"`
	DateAdded      string   `json:"date_added"`
	DateRead       string   `json:"date_read,omitempty"`
	Body           string   `json:"body,omitempty"`
	SortAuthor     string   `json:"sort_author,omitempty"`
	PageCount      int      `json:"page_count,omitempty"`
	FirstPublished int      `json:"first_published,omitempty"`
	ISBN           string   `json:"isbn,omitempty"`
}
