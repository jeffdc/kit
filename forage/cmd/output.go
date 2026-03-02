package cmd

import (
	"forage/internal/model"
	"forage/internal/openlibrary"
)

type bookConfirmation struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type bookConfirmationWithLookup struct {
	ID          string                    `json:"id"`
	Title       string                    `json:"title"`
	Status      string                    `json:"status"`
	OpenLibrary *openlibrary.SearchResult `json:"open_library"`
}

func confirm(b *model.Book) bookConfirmation {
	return bookConfirmation{
		ID:     b.ID,
		Title:  b.Title,
		Status: b.Status,
	}
}

func confirmWithLookup(b *model.Book, ol *openlibrary.SearchResult) bookConfirmationWithLookup {
	return bookConfirmationWithLookup{
		ID:          b.ID,
		Title:       b.Title,
		Status:      b.Status,
		OpenLibrary: ol,
	}
}

func stripBodies(books []model.Book) []model.Book {
	out := make([]model.Book, len(books))
	for i, b := range books {
		b.Body = ""
		out[i] = b
	}
	return out
}
