package export

import (
	"bytes"
	"strings"
	"testing"

	"forage/internal/model"
)

func TestGenerateContainsExpectedElements(t *testing.T) {
	books := []model.Book{
		{
			ID: "a1b2", Title: "Dune", Author: "Frank Herbert",
			Status: "wishlist", Tags: []string{"sci-fi"}, Rating: 5,
			DateAdded: "2026-02-28",
		},
		{
			ID: "c3d4", Title: "Blood Meridian", Author: "Cormac McCarthy",
			Status: "read", DateAdded: "2026-01-15", DateRead: "2026-02-01",
			Body: "A dark western novel.",
		},
	}

	var buf bytes.Buffer
	if err := Generate(books, &buf); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	html := buf.String()

	// Should contain basic HTML structure
	for _, want := range []string{
		"<!DOCTYPE html>",
		"<html",
		"</html>",
		"Forage Library",
		// Book data should be embedded
		"Dune",
		"Frank Herbert",
		"Blood Meridian",
		"Cormac McCarthy",
		// Should have search/filter controls
		"search",
		"filter",
	} {
		if !strings.Contains(strings.ToLower(html), strings.ToLower(want)) {
			t.Errorf("HTML missing expected content: %q", want)
		}
	}
}

func TestGenerateEmptyLibrary(t *testing.T) {
	var buf bytes.Buffer
	if err := Generate(nil, &buf); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	html := buf.String()
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("empty library should still produce valid HTML")
	}
}

func TestGenerateBookDataEmbedded(t *testing.T) {
	books := []model.Book{
		{
			ID: "x1y2", Title: "Test Book", Author: "Test Author",
			Status: "wishlist", Tags: []string{"tag1", "tag2"},
			Rating: 3, DateAdded: "2026-02-28",
		},
	}

	var buf bytes.Buffer
	Generate(books, &buf)
	html := buf.String()

	// JSON data should be embedded in a script tag
	if !strings.Contains(html, `"id":"x1y2"`) {
		t.Error("book ID not found in embedded data")
	}
	if !strings.Contains(html, `"tags":["tag1","tag2"]`) {
		t.Error("tags not found in embedded data")
	}
}
