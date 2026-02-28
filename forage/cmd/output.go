package cmd

import "forage/internal/model"

type bookConfirmation struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func confirm(b *model.Book) bookConfirmation {
	return bookConfirmation{
		ID:     b.ID,
		Title:  b.Title,
		Status: b.Status,
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
