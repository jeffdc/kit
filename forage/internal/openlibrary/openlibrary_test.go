package openlibrary

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearch_MatchFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("title") == "" {
			t.Error("expected title query param")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"numFound": 1,
			"docs": [{
				"title": "House of Suns",
				"author_name": ["Alastair Reynolds"],
				"first_publish_year": 2008,
				"number_of_pages_median": 473,
				"isbn": ["9780575082540", "0575082542"],
				"subject": ["Science fiction", "Space and time", "Fiction"]
			}]
		}`))
	}))
	defer srv.Close()

	result, err := searchWithBase(srv.URL, "House of Suns", "Reynolds")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Title != "House of Suns" {
		t.Errorf("title = %q, want %q", result.Title, "House of Suns")
	}
	if result.Author != "Alastair Reynolds" {
		t.Errorf("author = %q, want %q", result.Author, "Alastair Reynolds")
	}
	if result.FirstPublished != 2008 {
		t.Errorf("first_published = %d, want 2008", result.FirstPublished)
	}
	if result.PageCount != 473 {
		t.Errorf("page_count = %d, want 473", result.PageCount)
	}
	if result.ISBN != "9780575082540" {
		t.Errorf("isbn = %q, want 9780575082540", result.ISBN)
	}
	if len(result.Subjects) != 3 || result.Subjects[0] != "Science fiction" {
		t.Errorf("subjects = %v, want [Science fiction, Space and time, Fiction]", result.Subjects)
	}
}

func TestSearch_NoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"numFound": 0, "docs": []}`))
	}))
	defer srv.Close()

	result, err := searchWithBase(srv.URL, "asdfasdf", "Nobody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

func TestSearch_NoAuthor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"numFound": 1,
			"docs": [{
				"title": "Dune",
				"author_name": [],
				"first_publish_year": 1965
			}]
		}`))
	}))
	defer srv.Close()

	result, err := searchWithBase(srv.URL, "Dune", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Author != "" {
		t.Errorf("author = %q, want empty", result.Author)
	}
}
