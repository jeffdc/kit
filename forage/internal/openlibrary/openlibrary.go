package openlibrary

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const baseURL = "https://openlibrary.org"

type SearchResult struct {
	Title          string   `json:"title"`
	Author         string   `json:"author"`
	FirstPublished int      `json:"first_published,omitempty"`
	PageCount      int      `json:"page_count,omitempty"`
	ISBN           string   `json:"isbn,omitempty"`
	Subjects       []string `json:"subjects,omitempty"`
}

type searchResponse struct {
	NumFound int         `json:"numFound"`
	Docs     []searchDoc `json:"docs"`
}

type searchDoc struct {
	Title            string   `json:"title"`
	AuthorName       []string `json:"author_name"`
	FirstPublishYear int      `json:"first_publish_year"`
	PagesMedian      int      `json:"number_of_pages_median"`
	ISBN             []string `json:"isbn"`
	Subject          []string `json:"subject"`
}

func Search(title, author string) (*SearchResult, error) {
	return searchWithBase(baseURL, title, author)
}

func searchWithBase(base, title, author string) (*SearchResult, error) {
	params := url.Values{}
	params.Set("title", title)
	if author != "" {
		params.Set("author", author)
	}
	params.Set("limit", "1")
	params.Set("fields", "title,author_name,first_publish_year,number_of_pages_median,isbn,subject")

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/search.json?%s", base, params.Encode()), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "forage-cli/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	if sr.NumFound == 0 || len(sr.Docs) == 0 {
		return nil, nil
	}

	doc := sr.Docs[0]
	result := &SearchResult{
		Title:          doc.Title,
		FirstPublished: doc.FirstPublishYear,
		PageCount:      doc.PagesMedian,
		Subjects:       doc.Subject,
	}
	if len(doc.AuthorName) > 0 {
		result.Author = doc.AuthorName[0]
	}
	// Prefer ISBN-13 (starts with 978/979, 13 digits)
	for _, isbn := range doc.ISBN {
		if len(isbn) == 13 {
			result.ISBN = isbn
			break
		}
	}
	if result.ISBN == "" && len(doc.ISBN) > 0 {
		result.ISBN = doc.ISBN[0]
	}

	return result, nil
}
