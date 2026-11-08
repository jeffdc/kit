// Package resolve turns what the user shares — an ISBN, a link to a book
// page or a Bluesky post, or a bit of text — into candidate books from
// Open Library, each marked if it's already in the library.
//
// It is deterministic: structured page data, URL patterns and Open Library
// searches, no LLM.
package resolve

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"forage/internal/model"
)

const apiUserAgent = "Forage/1.0 (personal book list)"

// maxLinksPerPost bounds how many links in a Bluesky post are followed.
const maxLinksPerPost = 3

// Candidate is a book the input might refer to.
type Candidate struct {
	Title   string `json:"title"`
	Author  string `json:"author"`
	Year    int    `json:"first_published,omitempty"`
	Pages   int    `json:"page_count,omitempty"`
	ISBN    string `json:"isbn,omitempty"`
	WorkKey string `json:"work_key,omitempty"`
	Match   *Match `json:"match,omitempty"`
}

// Match describes the library book a candidate corresponds to.
type Match struct {
	BookID   string `json:"book_id"`
	Title    string `json:"title"`
	Author   string `json:"author"`
	Status   string `json:"status"`
	Rating   int    `json:"rating,omitempty"`
	DateRead string `json:"date_read,omitempty"`
}

// Result is the outcome of resolving one input.
type Result struct {
	// Kind is how the input was understood: isbn, url, bluesky or text.
	Kind string `json:"kind"`
	// Query is the search that produced the candidates, for display.
	Query string `json:"query,omitempty"`
	// Text is something to show for manual entry when nothing resolved:
	// the post text or the page title.
	Text       string      `json:"text,omitempty"`
	Candidates []Candidate `json:"candidates"`
}

// ErrEmptyInput is returned for blank input.
var ErrEmptyInput = errors.New("nothing to resolve")

type Resolver struct {
	OpenLibraryURL string
	BlueskyURL     string
	apiClient      *http.Client // fixed, trusted hosts
	pageClient     *http.Client // user-supplied URLs (guarded)
}

// New returns a Resolver using the public Open Library and Bluesky APIs.
func New() *Resolver {
	return &Resolver{
		OpenLibraryURL: "https://openlibrary.org",
		BlueskyURL:     "https://public.api.bsky.app",
		apiClient:      &http.Client{Timeout: 10 * time.Second},
		pageClient:     guardedClient(false),
	}
}

var reURLInText = regexp.MustCompile(`https?://[^\s<>"']+`)

// Resolve works out which book input refers to.
func (r *Resolver) Resolve(ctx context.Context, input string) (*Result, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, ErrEmptyInput
	}

	if isbn := NormalizeISBN(input); isbn != "" {
		cands, err := r.searchISBN(ctx, isbn)
		return &Result{Kind: "isbn", Query: isbn, Candidates: orEmpty(cands)}, err
	}

	if raw := reURLInText.FindString(input); raw != "" {
		raw = trimURLPunctuation(raw)
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			rest := collapse(strings.Replace(input, raw, " ", 1))
			return r.resolveURLInput(ctx, u, rest)
		}
	}

	return r.resolveText(ctx, input)
}

func (r *Resolver) resolveText(ctx context.Context, text string) (*Result, error) {
	title, author := splitBy(text)
	cands, err := r.searchTitle(ctx, title, author)
	return &Result{Kind: "text", Query: collapse(text), Candidates: orEmpty(cands)}, err
}

// resolveURLInput handles a shared URL, plus any text shared alongside it.
func (r *Resolver) resolveURLInput(ctx context.Context, u *url.URL, extraText string) (*Result, error) {
	if isBlueskyPost(u) {
		return r.resolveBluesky(ctx, u)
	}
	res, err := r.resolveURL(ctx, u)
	if err != nil || len(res.Candidates) > 0 || extraText == "" {
		return res, err
	}
	// The link told us nothing; try the words that came with it.
	title, author := splitBy(extraText)
	cands, err := r.looseSearch(ctx, title, author)
	if err != nil {
		return res, err
	}
	if len(cands) > 0 {
		res.Query, res.Candidates = extraText, cands
	} else if res.Text == "" {
		res.Text = extraText
	}
	return res, nil
}

// resolveURL tries, in order: an ISBN in the URL, the page's own book data,
// and the words in the URL's path.
func (r *Resolver) resolveURL(ctx context.Context, u *url.URL) (*Result, error) {
	res := &Result{Kind: "url", Candidates: []Candidate{}}

	if host := strings.ToLower(u.Hostname()); host == "openlibrary.org" || host == "www.openlibrary.org" {
		cands, handled, err := r.openLibraryKeySearch(ctx, u.Path)
		if err != nil {
			return res, err
		}
		if handled && len(cands) > 0 {
			res.Query, res.Candidates = u.Path, cands
			return res, nil
		}
	}

	if isbn := isbnFromURL(u.String()); isbn != "" {
		cands, err := r.searchISBN(ctx, isbn)
		if err != nil {
			return res, err
		}
		if len(cands) > 0 {
			res.Query, res.Candidates = isbn, cands
			return res, nil
		}
	}

	// Page fetch failures (bot checks, 403s, timeouts) are expected; fall through.
	if page, err := r.fetchPage(ctx, u.String()); err == nil && !ctxDone(ctx) {
		meta := extractMeta(page)
		if meta.ISBN != "" {
			cands, err := r.searchISBN(ctx, meta.ISBN)
			if err != nil {
				return res, err
			}
			if len(cands) > 0 {
				res.Query, res.Candidates = meta.ISBN, cands
				return res, nil
			}
		}
		if meta.Title != "" {
			cands, err := r.looseSearch(ctx, meta.Title, meta.Author)
			if err != nil {
				return res, err
			}
			res.Text = strings.TrimSpace(meta.Title + " " + meta.Author)
			if len(cands) > 0 {
				res.Query, res.Candidates = res.Text, cands
				return res, nil
			}
		}
	}

	if q := slugQuery(u); q != "" {
		title, author := splitBy(q)
		cands, err := r.looseSearch(ctx, title, author)
		if err != nil {
			return res, err
		}
		if len(cands) > 0 {
			res.Query, res.Candidates = q, cands
			return res, nil
		}
		if res.Text == "" {
			res.Text = q
		}
	}
	return res, nil
}

// resolveBluesky follows the post's link card and links; failing that it
// searches the link card's title, and failing that returns the post text.
func (r *Resolver) resolveBluesky(ctx context.Context, u *url.URL) (*Result, error) {
	post, err := r.fetchBlueskyPost(ctx, u)
	if err != nil {
		return nil, err
	}
	res := &Result{Kind: "bluesky", Text: post.Text, Candidates: []Candidate{}}

	for i, link := range post.Links {
		if i >= maxLinksPerPost || ctxDone(ctx) {
			break
		}
		lu, err := url.Parse(link)
		if err != nil {
			continue
		}
		lr, err := r.resolveURL(ctx, lu)
		if err != nil {
			return res, err
		}
		if len(lr.Candidates) > 0 {
			res.Query, res.Candidates = lr.Query, lr.Candidates
			return res, nil
		}
	}

	if post.CardTitle != "" && !isJunkTitle(post.CardTitle) {
		title, author := cleanPageTitle(post.CardTitle)
		cands, err := r.looseSearch(ctx, title, author)
		if err != nil {
			return res, err
		}
		if len(cands) > 0 {
			res.Query, res.Candidates = strings.TrimSpace(title+" "+author), cands
		}
	}
	return res, nil
}

// trimURLPunctuation drops sentence punctuation stuck to the end of a URL
// found in text, keeping a closing parenthesis that the URL itself opened
// (e.g. Wikipedia's "Piranesi_(novel)").
func trimURLPunctuation(u string) string {
	for len(u) > 0 {
		last := u[len(u)-1]
		switch {
		case strings.IndexByte(".,;:!?]'\"", last) >= 0:
			u = u[:len(u)-1]
		case last == ')' && strings.Count(u, ")") > strings.Count(u, "("):
			u = u[:len(u)-1]
		default:
			return u
		}
	}
	return u
}

func orEmpty(c []Candidate) []Candidate {
	if c == nil {
		return []Candidate{}
	}
	return c
}

// MarkLibraryMatches sets Match on candidates that are already in the
// library, comparing normalized titles and author surnames.
func MarkLibraryMatches(cands []Candidate, books []model.Book) {
	byTitle := map[string][]model.Book{}
	for _, b := range books {
		t := NormalizeTitle(b.Title)
		byTitle[t] = append(byTitle[t], b)
	}
	for i := range cands {
		t := NormalizeTitle(cands[i].Title)
		s := Surname(cands[i].Author)
		for _, b := range byTitle[t] {
			if s == "" || Surname(b.Author) == s {
				cands[i].Match = &Match{
					BookID: b.ID, Title: b.Title, Author: b.Author,
					Status: b.Status, Rating: b.Rating, DateRead: b.DateRead,
				}
				break
			}
		}
	}
}
