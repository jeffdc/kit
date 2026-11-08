package resolve

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"forage/internal/model"
)

// --- Fakes ---

type fakeBook struct {
	key, title, author string
	year               int
	isbns              []string
}

var catalog = []fakeBook{
	{"/works/OL1W", "Dune", "Frank Herbert", 1965, []string{"9780441172719", "0441172717"}},
	{"/works/OL2W", "Piranesi", "Susanna Clarke", 2020, []string{"9781635577808"}},
	{"/works/OL3W", "Babel", "R.F. Kuang", 2022, []string{"9780063021426"}},
}

// fakeOpenLibrary answers search.json like Open Library: isbn exact,
// title/author case-insensitive substring, q all-words.
func fakeOpenLibrary(t *testing.T) (*httptest.Server, *[]url.Values) {
	var mu sync.Mutex
	var queries []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		mu.Lock()
		queries = append(queries, q)
		mu.Unlock()
		if r.URL.Path != "/search.json" || q.Get("fields") == "" {
			http.Error(w, "bad request", 400)
			return
		}
		var docs []map[string]any
		for _, b := range catalog {
			hay := strings.ToLower(b.title + " " + b.author)
			ok := true
			if v := q.Get("isbn"); v != "" {
				ok = false
				for _, i := range b.isbns {
					ok = ok || i == v
				}
			}
			if v := q.Get("title"); v != "" {
				ok = ok && strings.Contains(strings.ToLower(b.title), strings.ToLower(v))
			}
			if v := q.Get("author"); v != "" {
				ok = ok && strings.Contains(strings.ToLower(b.author), strings.ToLower(v))
			}
			if v := q.Get("q"); v != "" {
				for _, w := range strings.Fields(strings.ToLower(v)) {
					ok = ok && strings.Contains(hay, w)
				}
			}
			if ok {
				docs = append(docs, map[string]any{"key": b.key, "title": b.title, "author_name": []string{b.author}, "first_publish_year": b.year})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"docs": docs})
	}))
	t.Cleanup(srv.Close)
	return srv, &queries
}

func fixtureServer(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	serveFile := func(path, file string, status int) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			data, err := os.ReadFile("testdata/" + file)
			if err != nil {
				t.Fatal(err)
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(status)
			w.Write(data)
		})
	}
	serveFile("/book/show/234225.Dune", "jsonld-book.html", 200)
	serveFile("/shop/babel", "jsonld-graph.html", 200)
	serveFile("/ogonly", "og-only.html", 200)
	serveFile("/w/dune_frank-herbert/250051/", "challenge.html", 200)
	serveFile("/p/books/piranesi-susanna-clarke/14399474", "challenge.html", 403)
	serveFile("/wiki/Piranesi_(novel)", "wikipedia.html", 200)
	serveFile("/nothing-useful-here", "challenge.html", 200)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testResolver(t *testing.T) (*Resolver, string, *[]url.Values) {
	ol, queries := fakeOpenLibrary(t)
	pages := fixtureServer(t)
	r := New()
	r.OpenLibraryURL = ol.URL
	r.pageClient = guardedClient(true) // fixtures live on localhost
	return r, pages.URL, queries
}

func titles(res *Result) []string {
	var out []string
	for _, c := range res.Candidates {
		out = append(out, c.Title)
	}
	return out
}

func wantTitle(t *testing.T, input string, res *Result, err error, kind, title string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", input, err)
	}
	if res.Kind != kind || len(res.Candidates) == 0 || res.Candidates[0].Title != title {
		t.Errorf("%s: kind=%s candidates=%v query=%q, want %s %q", input, res.Kind, titles(res), res.Query, kind, title)
	}
}

// --- Resolve ---

func TestResolveISBN(t *testing.T) {
	r, _, _ := testResolver(t)
	for _, in := range []string{"9780441172719", "978-0-441-17271-9", "ISBN: 0441172717"} {
		res, err := r.Resolve(context.Background(), in)
		wantTitle(t, in, res, err, "isbn", "Dune")
		if res != nil && len(res.Candidates) > 0 && res.Candidates[0].ISBN == "" {
			t.Errorf("%s: ISBN not carried onto candidate", in)
		}
	}
}

func TestResolveURLs(t *testing.T) {
	r, base, queries := testResolver(t)
	cases := []struct{ input, title, why string }{
		{base + "/book/show/234225.Dune", "Dune", "JSON-LD Book with ISBN"},
		{base + "/shop/babel", "Babel", "@graph Book, ISBN in workExample"},
		{base + "/ogonly", "Piranesi", "og:title 'X by Y | Site'"},
		{base + "/w/dune_frank-herbert/250051/", "Dune", "bot challenge -> URL slug"},
		{base + "/p/books/piranesi-susanna-clarke/14399474", "Piranesi", "403 -> URL slug"},
		{base + "/wiki/Piranesi_(novel)", "Piranesi", "Wikipedia title"},
		{"https://www.amazon.invalid/Dune-Frank-Herbert/dp/0441172717/ref=sr_1_1", "Dune", "ISBN-10 in Amazon URL (no fetch needed)"},
		{"https://bookshop.invalid/a/123/9781635577808", "Piranesi", "ISBN-13 in URL"},
		{"Look at this! " + base + "/ogonly.", "Piranesi", "URL inside shared text"},
	}
	for _, c := range cases {
		res, err := r.Resolve(context.Background(), c.input)
		t.Run(c.why, func(t *testing.T) { wantTitle(t, c.input, res, err, "url", c.title) })
	}
	if len(*queries) == 0 {
		t.Fatal("no Open Library queries recorded")
	}
}

func TestResolveURLWithNothingUseful(t *testing.T) {
	r, base, _ := testResolver(t)
	res, err := r.Resolve(context.Background(), base+"/nothing-useful-here")
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "url" || len(res.Candidates) != 0 || res.Candidates == nil {
		t.Errorf("got %+v, want url kind with empty (non-nil) candidates", res)
	}

	// Words shared alongside a useless link are used instead.
	res, err = r.Resolve(context.Background(), "Piranesi by Susanna Clarke "+base+"/nothing-useful-here")
	wantTitle(t, "text+link", res, err, "url", "Piranesi")
}

func TestResolveText(t *testing.T) {
	r, _, _ := testResolver(t)
	for in, want := range map[string]string{
		"Piranesi by Susanna Clarke": "Piranesi",
		"babel kuang":                "Babel",
		"dune":                       "Dune",
	} {
		res, err := r.Resolve(context.Background(), in)
		wantTitle(t, in, res, err, "text", want)
	}
	if _, err := r.Resolve(context.Background(), "   "); !errors.Is(err, ErrEmptyInput) {
		t.Errorf("blank input: %v", err)
	}
}

func TestResolveBluesky(t *testing.T) {
	r, base, _ := testResolver(t)
	var gotURI string
	posts := map[string]string{
		// link card to a book page
		"card": `{"thread":{"post":{"record":{"text":"Loved this one"},"embed":{"$type":"app.bsky.embed.external#view","external":{"uri":"` + base + `/book/show/234225.Dune","title":"Dune"}}}}}`,
		// link only in the text (facet), alongside a bsky link that must be skipped
		"facet": `{"thread":{"post":{"record":{"text":"read this","facets":[{"features":[{"uri":"https://bsky.app/profile/x"}]},{"features":[{"uri":"` + base + `/shop/babel"}]}]},"embed":null}}}`,
		// card whose page is blocked: fall back to the card title
		"cardtitle": `{"thread":{"post":{"record":{"text":"new fave"},"embed":{"$type":"app.bsky.embed.recordWithMedia#view","media":{"external":{"uri":"` + base + `/nothing-useful-here","title":"Piranesi by Susanna Clarke | Bookshop.org"}}}}}}`,
		// no links at all: return the text for manual entry
		"textonly": `{"thread":{"post":{"record":{"text":"Just finished an amazing novel about a house of statues"}}}}`,
	}
	bsky := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.Query().Get("uri")
		rkey := gotURI[strings.LastIndex(gotURI, "/")+1:]
		body, ok := posts[rkey]
		if !ok {
			http.Error(w, `{"error":"NotFound"}`, 400)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(bsky.Close)
	r.BlueskyURL = bsky.URL

	res, err := r.Resolve(context.Background(), "https://bsky.app/profile/someone.bsky.social/post/card")
	wantTitle(t, "card", res, err, "bluesky", "Dune")
	if gotURI != "at://someone.bsky.social/app.bsky.feed.post/card" {
		t.Errorf("AT URI = %q", gotURI)
	}
	res, err = r.Resolve(context.Background(), "https://bsky.app/profile/did:plc:abc/post/facet")
	wantTitle(t, "facet", res, err, "bluesky", "Babel")
	res, err = r.Resolve(context.Background(), "https://bsky.app/profile/someone/post/cardtitle")
	wantTitle(t, "cardtitle", res, err, "bluesky", "Piranesi")

	res, err = r.Resolve(context.Background(), "https://bsky.app/profile/someone/post/textonly")
	if err != nil || len(res.Candidates) != 0 || !strings.Contains(res.Text, "house of statues") {
		t.Errorf("text-only post: %+v %v", res, err)
	}
	if _, err := r.Resolve(context.Background(), "https://bsky.app/profile/someone/post/missing"); err == nil {
		t.Error("missing post: want error")
	}
}

func TestOpenLibraryFailureIsAnError(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(down.Close)
	r := New()
	r.OpenLibraryURL = down.URL
	if _, err := r.Resolve(context.Background(), "9780441172719"); err == nil {
		t.Error("want error when Open Library is down")
	}
}

// --- Safety ---

func TestGuardedClientRefusesInternalAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<title>secret</title>"))
	}))
	t.Cleanup(srv.Close)

	r := New() // production guard
	for _, u := range []string{srv.URL, "http://localhost:80/", "http://[::1]/", "http://169.254.169.254/latest/meta-data"} {
		if _, err := r.fetchPage(context.Background(), u); err == nil || !errors.Is(err, errBlockedAddress) {
			t.Errorf("fetch %s: err = %v, want blocked", u, err)
		}
	}

	for ip, want := range map[string]bool{
		"8.8.8.8": true, "2606:4700::1111": true,
		"127.0.0.1": false, "10.1.2.3": false, "192.168.1.1": false, "172.16.0.1": false,
		"100.64.1.1": false, "169.254.1.1": false, "0.0.0.0": false, "::1": false,
		"fdaa::1": false, "fe80::1": false, "::ffff:127.0.0.1": false,
	} {
		if got := publicAddress(parseIP(t, ip)); got != want {
			t.Errorf("publicAddress(%s) = %v, want %v", ip, got, want)
		}
	}
}

// --- Pieces ---

func TestNormalizeISBN(t *testing.T) {
	for in, want := range map[string]string{
		"9780441172719": "9780441172719", "978-0-441-17271-9": "9780441172719",
		"0441172717": "0441172717", "isbn 0-441-17271-7": "0441172717", "080442957X": "080442957X",
		"9780441172718": "", "1234567890": "", "dune": "", "B08XYZ1234": "",
	} {
		if got := NormalizeISBN(in); got != want {
			t.Errorf("NormalizeISBN(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestISBNFromURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://www.amazon.com/dp/0441172717":                             "0441172717",
		"https://www.amazon.com/Dune/dp/0441172717/ref=x":                  "0441172717",
		"https://www.amazon.com/gp/product/0441172717?x=1":                 "0441172717",
		"https://www.amazon.com/dp/B00B7NPRY8":                             "",
		"https://bookshop.org/p/books/piranesi/14399474?ean=9781635577808": "9781635577808",
		"https://example.com/order/9780441172718":                          "",
		"https://example.com/posts/12345":                                  "",
	} {
		if got := isbnFromURL(in); got != want {
			t.Errorf("isbnFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanPageTitle(t *testing.T) {
	cases := []struct{ in, title, author string }{
		{"Dune (Dune, #1) by Frank Herbert | Goodreads", "Dune", "Frank Herbert"},
		{"Piranesi (novel) - Wikipedia", "Piranesi", ""},
		{"Stand by Me: A Memoir by Ann Author — Shop", "Stand by Me: A Memoir", "Ann Author"},
		{"The Overstory", "The Overstory", ""},
		{"  Babel &amp; Other   Stories  ", "Babel & Other Stories", ""},
	}
	for _, c := range cases {
		title, author := cleanPageTitle(c.in)
		if title != c.title || author != c.author {
			t.Errorf("cleanPageTitle(%q) = (%q, %q), want (%q, %q)", c.in, title, author, c.title, c.author)
		}
	}
}

func TestSlugQuery(t *testing.T) {
	for in, want := range map[string]string{
		"https://bookshop.org/p/books/piranesi-susanna-clarke/14399474":               "piranesi susanna clarke",
		"https://www.thriftbooks.com/w/dune_frank-herbert/250051/":                    "dune frank herbert",
		"https://www.goodreads.com/book/show/234225.Dune":                             "",
		"https://www.goodreads.com/book/show/50202953-piranesi":                       "piranesi",
		"https://www.penguinrandomhouse.com/books/622418/piranesi-by-susanna-clarke/": "piranesi by susanna clarke",
		"https://en.wikipedia.org/wiki/Piranesi_(novel)":                              "Piranesi",
		"https://example.com/": "",
	} {
		u, _ := url.Parse(in)
		if got := slugQuery(u); got != want {
			t.Errorf("slugQuery(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeTitleAndSurname(t *testing.T) {
	for in, want := range map[string]string{
		"The Left Hand of Darkness": "left hand of darkness", "Dune (Dune, #1)": "dune",
		"Babel: Or the Necessity of Violence": "babel", "Cien años de soledad": "cien anos de soledad",
		"Tom & Jerry": "tom and jerry", "A": "a",
	} {
		if got := NormalizeTitle(in); got != want {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"Frank Herbert": "herbert", "Herbert, Frank": "herbert", "R. F. Kuang": "kuang", "R.F. Kuang": "kuang",
		"Martin Luther King, Jr.": "king", "Kurt Vonnegut Jr.": "vonnegut", "Gabriel García Márquez": "marquez",
		"qntm": "qntm", "": "",
	} {
		if got := Surname(in); got != want {
			t.Errorf("Surname(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMarkLibraryMatches(t *testing.T) {
	books := []model.Book{
		{ID: "a1", Title: "Dune (Dune, #1)", Author: "Herbert, Frank", Status: "read", Rating: 5, DateRead: "2019-05-01"},
		{ID: "b2", Title: "Babel: Or the Necessity of Violence", Author: "R. F. Kuang", Status: "wishlist"},
		{ID: "c3", Title: "Dune", Author: "Someone Else", Status: "read"},
	}
	cands := []Candidate{
		{Title: "Dune", Author: "Frank Herbert"},
		{Title: "Babel", Author: "R.F. Kuang"},
		{Title: "Piranesi", Author: "Susanna Clarke"},
	}
	MarkLibraryMatches(cands, books)
	if m := cands[0].Match; m == nil || m.BookID != "a1" || m.DateRead != "2019-05-01" || m.Rating != 5 {
		t.Errorf("Dune match = %+v", m)
	}
	if m := cands[1].Match; m == nil || m.BookID != "b2" || m.Status != "wishlist" {
		t.Errorf("Babel match = %+v", m)
	}
	if cands[2].Match != nil {
		t.Errorf("Piranesi should not match: %+v", cands[2].Match)
	}
}

func TestExtractMetaFixtures(t *testing.T) {
	cases := map[string]pageMeta{
		"jsonld-book.html":  {Title: "Dune", Author: "Frank Herbert", ISBN: "9780441172719"},
		"jsonld-graph.html": {Title: "Babel", Author: "R. F. Kuang", ISBN: "9780063021426"},
		"og-only.html":      {Title: "Piranesi", Author: "Susanna Clarke & more"},
		"og-isbn.html":      {Title: "Something", ISBN: "9781635577808"},
		"challenge.html":    {},
		"wikipedia.html":    {Title: "Piranesi"},
	}
	for file, want := range cases {
		data, err := os.ReadFile("testdata/" + file)
		if err != nil {
			t.Fatal(err)
		}
		if got := extractMeta(string(data)); got != want {
			t.Errorf("%s: %+v, want %+v", file, got, want)
		}
	}
}

func parseIP(t *testing.T, s string) net.IP {
	t.Helper()
	ip := net.ParseIP(s)
	if ip == nil {
		t.Fatalf("bad IP %q", s)
	}
	return ip
}

func TestRelevant(t *testing.T) {
	cases := []struct {
		query, title, author string
		want                 bool
	}{
		{"dune frank herbert", "Dune", "Frank Herbert", true},
		{"piranesi", "Piranesi", "Susanna Clarke", true},
		{"piranesi", "Piranesi's Rome", "Someone", true},
		{"piranesi susanna clarke", "Piranesi: A Novel", "Susanna Clarke", true},
		{"latine heritage month", "Confessions", "Augustine of Hippo", false},
		{"latest meta data", "Universal meta data models", "David Marco", false},
		{"the", "The", "", false},
	}
	for _, c := range cases {
		if got := relevant(c.query, Candidate{Title: c.title, Author: c.author}); got != c.want {
			t.Errorf("relevant(%q, %q/%q) = %v, want %v", c.query, c.title, c.author, got, c.want)
		}
	}
}

func TestSlugQueryDropsFileExtensions(t *testing.T) {
	u, _ := url.Parse("https://www.nytimes.com/2024/01/01/books/review/piranesi-susanna-clarke.html")
	if got := slugQuery(u); got != "piranesi susanna clarke" {
		t.Errorf("slugQuery = %q", got)
	}
}
