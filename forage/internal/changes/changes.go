// Package changes applies the change log sent by the original PWA
// (POST /api/changes). It translates that older, book-only format onto the
// current schema, where read history lives in reads. The new sync API
// (POST /api/ops) replaces it once the PWA is rewritten.
package changes

import (
	"errors"
	"strconv"
	"strings"

	"forage/internal/model"
	"forage/internal/storage"
)

// Changelog represents a JSON changelog exported from the PWA.
type Changelog struct {
	Version  int     `json:"version"`
	Exported string  `json:"exported"`
	Changes  []Entry `json:"changes"`
}

// Entry represents a single operation in a changelog.
type Entry struct {
	Op     string         `json:"op"`
	Book   *model.Book    `json:"book,omitempty"`   // for create
	ID     string         `json:"id,omitempty"`     // for update/delete
	Fields map[string]any `json:"fields,omitempty"` // for update
	Ts     string         `json:"ts"`
}

// Summary is the JSON output for a changes operation.
type Summary struct {
	Applied int `json:"applied"`
	Skipped int `json:"skipped"`
	Errors  int `json:"errors"`
}

// Apply applies a changelog. Each entry is applied on its own; a failed entry
// is counted as skipped and the rest still go through.
//
// Read history: the old PWA has no read-date field, only the "read" status.
//   - Creating a book as read records one undated read (the date is unknown).
//   - Changing a book's status to read records a read finished today.
//   - An explicit date_read sets the latest read's date.
func Apply(s *storage.Store, cl Changelog) Summary {
	var summary Summary

	for _, c := range cl.Changes {
		var err error
		switch c.Op {
		case "create":
			err = applyCreate(s, c)
		case "update":
			err = applyUpdate(s, c)
		case "delete":
			if c.ID == "" {
				summary.Errors++
				continue
			}
			err = s.DeleteBook(c.ID)
		default:
			summary.Errors++
			continue
		}
		if err == errMalformed {
			summary.Errors++
		} else if err != nil {
			summary.Skipped++
		} else {
			summary.Applied++
		}
	}

	return summary
}

var errMalformed = errors.New("malformed change")

func applyCreate(s *storage.Store, c Entry) error {
	if c.Book == nil {
		return errMalformed
	}
	b := c.Book
	meta := map[string]string{}
	set := func(k, v string) {
		if v != "" {
			meta[k] = v
		}
	}
	set("status", b.Status)
	set("tags", strings.Join(b.Tags, ","))
	set("date_added", b.DateAdded)
	set("body", b.Body)
	set("isbn", b.ISBN)
	if b.Rating > 0 {
		meta["rating"] = strconv.Itoa(b.Rating)
	}
	if b.PageCount > 0 {
		meta["page_count"] = strconv.Itoa(b.PageCount)
	}
	if b.FirstPublished > 0 {
		meta["first_published"] = strconv.Itoa(b.FirstPublished)
	}
	if _, err := s.CreateBookWithID(b.ID, b.Title, b.Author, meta); err != nil {
		return err
	}
	if b.Status == "read" || b.DateRead != "" {
		return addRead(s, b.ID, b.DateRead)
	}
	return nil
}

func applyUpdate(s *storage.Store, c Entry) error {
	if c.ID == "" || len(c.Fields) == 0 {
		return errMalformed
	}
	before, err := s.GetBook(c.ID)
	if err != nil {
		return err
	}

	fields := make(map[string]string, len(c.Fields))
	for k, v := range c.Fields {
		fields[k] = storage.FieldString(v)
	}
	dateRead, hasDateRead := fields["date_read"]
	delete(fields, "date_read")

	after := before
	if len(fields) > 0 {
		if after, err = s.UpdateBook(c.ID, fields); err != nil {
			return err
		}
	}

	switch {
	case hasDateRead && dateRead != "":
		return s.SetLatestReadDate(c.ID, dateRead)
	case before.Status != "read" && after.Status == "read":
		return addRead(s, c.ID, storage.Today())
	}
	return nil
}

func addRead(s *storage.Store, bookID, date string) error {
	id, err := storage.NewID()
	if err != nil {
		return err
	}
	_, err = s.CreateRead(model.Read{ID: id, BookID: bookID, DateFinished: date})
	return err
}
