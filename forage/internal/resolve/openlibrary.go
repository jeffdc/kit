package resolve

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
)

var reOpenLibraryPath = regexp.MustCompile(`^/(works|books)/(OL[0-9]+[WM])(?:/|$)`)

const searchFields = "key,title,author_name,first_publish_year,number_of_pages_median"

type olSearchResponse struct {
	Docs []struct {
		Key        string   `json:"key"`
		Title      string   `json:"title"`
		AuthorName []string `json:"author_name"`
		FirstYear  int      `json:"first_publish_year"`
		Pages      int      `json:"number_of_pages_median"`
	} `json:"docs"`
}

// searchOpenLibrary runs one Open Library search. params holds isbn, or
// title/author, or q.
func (r *Resolver) searchOpenLibrary(ctx context.Context, params url.Values) ([]Candidate, error) {
	params.Set("fields", searchFields)
	params.Set("limit", "5")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.OpenLibraryURL+"/search.json?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", apiUserAgent)
	resp, err := r.apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open library: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open library: HTTP %d", resp.StatusCode)
	}
	var sr olSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("open library: %w", err)
	}

	cands := []Candidate{}
	for _, d := range sr.Docs {
		if d.Title == "" {
			continue
		}
		c := Candidate{Title: d.Title, Year: d.FirstYear, Pages: d.Pages, WorkKey: d.Key}
		if len(d.AuthorName) > 0 {
			c.Author = d.AuthorName[0]
		}
		cands = append(cands, c)
	}
	return cands, nil
}

// looseSearch is searchTitle for queries derived from URLs and page
// titles: it drops candidates that don't fit the query and puts exact
// title matches first.
func (r *Resolver) looseSearch(ctx context.Context, title, author string) ([]Candidate, error) {
	cands, err := r.searchTitle(ctx, title, author)
	if err != nil {
		return nil, err
	}
	query := title + " " + author
	want := NormalizeTitle(title)
	var exact, rest []Candidate
	for _, c := range cands {
		switch {
		case NormalizeTitle(c.Title) == want:
			exact = append(exact, c)
		case relevant(query, c):
			rest = append(rest, c)
		}
	}
	return append(exact, rest...), nil
}

// openLibraryKeySearch handles links to Open Library itself, whose pages
// are behind a bot check: /works/OL…W and /books/OL…M.
func (r *Resolver) openLibraryKeySearch(ctx context.Context, path string) ([]Candidate, bool, error) {
	m := reOpenLibraryPath.FindStringSubmatch(path)
	if m == nil {
		return nil, false, nil
	}
	q := "key:/works/" + m[2]
	if m[1] == "books" {
		q = "edition_key:" + m[2]
	}
	cands, err := r.searchOpenLibrary(ctx, url.Values{"q": {q}})
	return cands, true, err
}

func (r *Resolver) searchISBN(ctx context.Context, isbn string) ([]Candidate, error) {
	cands, err := r.searchOpenLibrary(ctx, url.Values{"isbn": {isbn}})
	for i := range cands {
		cands[i].ISBN = isbn
	}
	return cands, err
}

// searchTitle searches by title and author, loosening the search until
// something turns up: title+author fields, then both as one query (author
// names vary: "R.F. Kuang" vs "R. F. Kuang"), then the title alone (the
// "author" scraped from a page may be noise).
func (r *Resolver) searchTitle(ctx context.Context, title, author string) ([]Candidate, error) {
	if title == "" {
		return nil, nil
	}
	if author != "" {
		for _, params := range []url.Values{
			{"title": {title}, "author": {author}},
			{"q": {title + " " + author}},
		} {
			cands, err := r.searchOpenLibrary(ctx, params)
			if err != nil || len(cands) > 0 {
				return cands, err
			}
		}
	}
	return r.searchOpenLibrary(ctx, url.Values{"q": {title}})
}
