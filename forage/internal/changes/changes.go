package changes

import (
	"fmt"
	"strconv"
	"strings"

	"forage/internal/model"
)

// Changelog represents a JSON changelog exported from the PWA.
type Changelog struct {
	Version  int     `json:"version"`
	Exported string  `json:"exported"`
	Changes  []Entry `json:"changes"`
}

// Entry represents a single operation in a changelog.
type Entry struct {
	Op     string                 `json:"op"`
	Book   *model.Book            `json:"book,omitempty"`   // for create
	ID     string                 `json:"id,omitempty"`     // for update/delete
	Fields map[string]interface{} `json:"fields,omitempty"` // for update
	Ts     string                 `json:"ts"`
}

// Summary is the JSON output for a changes operation.
type Summary struct {
	Applied int `json:"applied"`
	Skipped int `json:"skipped"`
	Errors  int `json:"errors"`
}

// Store is the interface required to apply changes.
type Store interface {
	CreateBookWithID(id, title, author string, meta map[string]string) (*model.Book, error)
	UpdateBook(id, key, value string) (*model.Book, error)
	DeleteBook(id string) error
}

// Apply reads a changelog and applies operations to the store.
func Apply(s Store, cl Changelog) Summary {
	var summary Summary

	for _, c := range cl.Changes {
		switch c.Op {
		case "create":
			if c.Book == nil {
				summary.Errors++
				continue
			}
			meta := make(map[string]string)
			if c.Book.Status != "" {
				meta["status"] = c.Book.Status
			}
			if c.Book.Rating > 0 {
				meta["rating"] = strconv.Itoa(c.Book.Rating)
			}
			if len(c.Book.Tags) > 0 {
				meta["tags"] = strings.Join(c.Book.Tags, ",")
			}
			if c.Book.DateAdded != "" {
				meta["date_added"] = c.Book.DateAdded
			}
			if c.Book.DateRead != "" {
				meta["date_read"] = c.Book.DateRead
			}
			if c.Book.Body != "" {
				meta["body"] = c.Book.Body
			}
			if c.Book.PageCount > 0 {
				meta["page_count"] = strconv.Itoa(c.Book.PageCount)
			}
			if c.Book.FirstPublished > 0 {
				meta["first_published"] = strconv.Itoa(c.Book.FirstPublished)
			}
			if c.Book.ISBN != "" {
				meta["isbn"] = c.Book.ISBN
			}
			_, err := s.CreateBookWithID(c.Book.ID, c.Book.Title, c.Book.Author, meta)
			if err != nil {
				summary.Skipped++
				continue
			}
			summary.Applied++

		case "update":
			if c.ID == "" || len(c.Fields) == 0 {
				summary.Errors++
				continue
			}
			errored := false
			for key, val := range c.Fields {
				strVal := FieldToString(key, val)
				_, err := s.UpdateBook(c.ID, key, strVal)
				if err != nil {
					summary.Skipped++
					errored = true
					break
				}
			}
			if !errored {
				summary.Applied++
			}

		case "delete":
			if c.ID == "" {
				summary.Errors++
				continue
			}
			err := s.DeleteBook(c.ID)
			if err != nil {
				summary.Skipped++
				continue
			}
			summary.Applied++

		default:
			summary.Errors++
		}
	}

	return summary
}

// FieldToString converts a changelog field value to a string suitable for Store.UpdateBook.
func FieldToString(key string, val interface{}) string {
	switch key {
	case "rating", "page_count", "first_published":
		if f, ok := val.(float64); ok {
			return strconv.Itoa(int(f))
		}
	case "tags":
		if arr, ok := val.([]interface{}); ok {
			parts := make([]string, len(arr))
			for i, v := range arr {
				parts[i] = fmt.Sprintf("%v", v)
			}
			return strings.Join(parts, ",")
		}
	}
	return fmt.Sprintf("%v", val)
}
