package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCSV(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDetectFormatGoodreads(t *testing.T) {
	headers := []string{"Title", "Author", "My Rating", "Exclusive Shelf", "Bookshelves", "Date Added"}
	f, err := detectFormat(headers)
	if err != nil {
		t.Fatal(err)
	}
	if f != formatGoodreads {
		t.Fatalf("expected Goodreads format, got %d", f)
	}
}

func TestDetectFormatStoryGraph(t *testing.T) {
	headers := []string{"Title", "Authors", "Star Rating", "Read Status", "Tags", "Date Added"}
	f, err := detectFormat(headers)
	if err != nil {
		t.Fatal(err)
	}
	if f != formatStoryGraph {
		t.Fatalf("expected StoryGraph format, got %d", f)
	}
}

func TestDetectFormatUnknown(t *testing.T) {
	headers := []string{"Name", "Writer"}
	_, err := detectFormat(headers)
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestParseGoodreadsCSV(t *testing.T) {
	dir := t.TempDir()
	csv := `Title,Author,My Rating,Exclusive Shelf,Bookshelves,Date Added,Date Read
The Left Hand of Darkness,Ursula K. Le Guin,5,read,"read, sci-fi, classic",2020/01/15,2020/02/28
Dune,Frank Herbert,0,to-read,to-read,2021/03/10,
Parable of the Sower,Octavia Butler,4,currently-reading,"currently-reading, afrofuturism",2022/06/01,
Abandoned Book,Some Author,0,abandoned,abandoned,2023/01/01,
`
	path := writeCSV(t, dir, "goodreads.csv", csv)

	books, err := parseCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 4 {
		t.Fatalf("expected 4 books, got %d", len(books))
	}

	// Check first book
	b := books[0]
	if b.Title != "The Left Hand of Darkness" {
		t.Errorf("title = %q", b.Title)
	}
	if b.Author != "Ursula K. Le Guin" {
		t.Errorf("author = %q", b.Author)
	}
	if b.Rating != 5 {
		t.Errorf("rating = %d", b.Rating)
	}
	if b.Status != "read" {
		t.Errorf("status = %q", b.Status)
	}
	if len(b.Tags) != 2 || b.Tags[0] != "sci-fi" || b.Tags[1] != "classic" {
		t.Errorf("tags = %v", b.Tags)
	}
	if b.DateAdded != "2020-01-15" {
		t.Errorf("date_added = %q", b.DateAdded)
	}
	if b.DateRead != "2020-02-28" {
		t.Errorf("date_read = %q", b.DateRead)
	}

	// Check to-read maps to wishlist
	if books[1].Status != "wishlist" {
		t.Errorf("to-read should map to wishlist, got %q", books[1].Status)
	}
	if books[1].Rating != 0 {
		t.Errorf("unrated should be 0, got %d", books[1].Rating)
	}

	// Check currently-reading maps to reading, tags stripped
	if books[2].Status != "reading" {
		t.Errorf("currently-reading should map to reading, got %q", books[2].Status)
	}
	if len(books[2].Tags) != 1 || books[2].Tags[0] != "afrofuturism" {
		t.Errorf("tags should be [afrofuturism], got %v", books[2].Tags)
	}

	// Check abandoned maps to dropped
	if books[3].Status != "dropped" {
		t.Errorf("abandoned should map to dropped, got %q", books[3].Status)
	}
}

func TestParseStoryGraphCSV(t *testing.T) {
	dir := t.TempDir()
	csv := `Title,Authors,Star Rating,Read Status,Tags,Date Added,Last Date Read
The Left Hand of Darkness,Ursula K. Le Guin,4.0,read,sci-fi,2019/12/01,2020/01/15
Dune,Frank Herbert,,to-read,,2021/03/10,
Kindred,Octavia Butler,3.5,did-not-finish,,2022/05/01,
Paused Book,Some Author,,paused,abandoned,2023/01/01,
`
	path := writeCSV(t, dir, "storygraph.csv", csv)

	books, err := parseCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 4 {
		t.Fatalf("expected 4 books, got %d", len(books))
	}

	b := books[0]
	if b.Rating != 4 {
		t.Errorf("rating = %d, want 4", b.Rating)
	}
	if b.Status != "read" {
		t.Errorf("status = %q", b.Status)
	}
	if len(b.Tags) != 1 || b.Tags[0] != "sci-fi" {
		t.Errorf("tags = %v", b.Tags)
	}

	// did-not-finish -> dropped
	if books[2].Status != "dropped" {
		t.Errorf("did-not-finish should map to dropped, got %q", books[2].Status)
	}
	// 3.5 rounds to 4
	if books[2].Rating != 4 {
		t.Errorf("3.5 should round to 4, got %d", books[2].Rating)
	}

	// paused -> paused, "abandoned" tag should be stripped
	if books[3].Status != "paused" {
		t.Errorf("paused should map to paused, got %q", books[3].Status)
	}
	if len(books[3].Tags) != 0 {
		t.Errorf("abandoned tag should be stripped, got %v", books[3].Tags)
	}
}

func TestMergeBooks(t *testing.T) {
	a := csvBook{
		Title:     "The Left Hand of Darkness",
		Author:    "Ursula K. Le Guin",
		Rating:    0,
		Status:    "wishlist",
		Tags:      []string{"sci-fi"},
		DateAdded: "2020-01-15",
		DateRead:  "",
	}
	b := csvBook{
		Title:     "The Left Hand of Darkness",
		Author:    "Ursula K. Le Guin",
		Rating:    5,
		Status:    "read",
		Tags:      []string{"sci-fi", "classic"},
		DateAdded: "2021-03-10",
		DateRead:  "2021-04-01",
	}

	merged := mergeBooks(a, b)

	if merged.Rating != 5 {
		t.Errorf("rating = %d, want 5 (higher wins)", merged.Rating)
	}
	if merged.Status != "read" {
		t.Errorf("status = %q, want read (more specific wins)", merged.Status)
	}
	if merged.DateAdded != "2020-01-15" {
		t.Errorf("date_added = %q, want 2020-01-15 (earlier wins)", merged.DateAdded)
	}
	if merged.DateRead != "2021-04-01" {
		t.Errorf("date_read = %q, want 2021-04-01 (later wins)", merged.DateRead)
	}
	if len(merged.Tags) != 2 {
		t.Errorf("tags = %v, want [sci-fi classic]", merged.Tags)
	}
}

func TestNormalizeKey(t *testing.T) {
	// Case and whitespace
	k1 := normalizeKey("The Left Hand of Darkness", "Ursula K. Le Guin")
	k2 := normalizeKey("the left hand of darkness", "  ursula k. le guin  ")
	if k1 != k2 {
		t.Errorf("case/whitespace: keys should match: %q vs %q", k1, k2)
	}

	// Middle initials stripped: "Stephen W. Hawking" == "Stephen Hawking"
	k3 := normalizeKey("A Brief History of Time", "Stephen W. Hawking")
	k4 := normalizeKey("A Brief History of Time", "Stephen Hawking")
	if k3 != k4 {
		t.Errorf("middle initials: keys should match: %q vs %q", k3, k4)
	}

	// Multi-author: first author used for key
	k5 := normalizeKey("Dune", "Frank Herbert")
	k6 := normalizeKey("Dune", "Frank Herbert, Brian Herbert")
	if k5 != k6 {
		t.Errorf("multi-author: keys should match: %q vs %q", k5, k6)
	}

	// Trailing series parenthetical stripped
	k7 := normalizeKey("Reaper Man", "Terry Pratchett")
	k8 := normalizeKey("Reaper Man (Discworld, #11; Death, #2)", "Terry Pratchett")
	if k7 != k8 {
		t.Errorf("series parens: keys should match: %q vs %q", k7, k8)
	}

	// Subtitle after colon stripped
	k9 := normalizeKey("The Tipping Point", "Malcolm Gladwell")
	k10 := normalizeKey("The Tipping Point: How Little Things Can Make a Big Difference", "Malcolm Gladwell")
	if k9 != k10 {
		t.Errorf("subtitle: keys should match: %q vs %q", k9, k10)
	}

	// Punctuation normalized: hyphens vs spaces
	k11 := normalizeKey("Zot!: The Complete Black and White Collection", "Scott McCloud")
	k12 := normalizeKey("Zot!: The Complete Black-and-White Collection", "Scott McCloud")
	if k11 != k12 {
		t.Errorf("punctuation: keys should match: %q vs %q", k11, k12)
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Reaper Man (Discworld, #11; Death, #2)", "reaper man"},
		{"The Tipping Point: How Little Things Can Make a Big Difference", "the tipping point"},
		{"Zot!: The Complete Black-and-White Collection: 1987-1991", "zot"},
		{"Mind of the Raven", "mind of the raven"},
		{"Mr. Palomar (English and Italian Edition)", "mr palomar"},
		{"  Effective Java  ", "effective java"},
	}
	for _, tt := range tests {
		got := normalizeTitle(tt.input)
		if got != tt.want {
			t.Errorf("normalizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeAuthor(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Stephen W. Hawking", "stephen hawking"},
		{"Stephen Hawking", "stephen hawking"},
		{"Bernard F. Schutz", "bernard schutz"},
		{"Frank Herbert, Brian Herbert", "frank herbert"},
		{"  David  Lindsay  ", "david lindsay"},
		{"Ray S. Jackendoff", "ray jackendoff"},
	}
	for _, tt := range tests {
		got := normalizeAuthor(tt.input)
		if got != tt.want {
			t.Errorf("normalizeAuthor(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeDate(t *testing.T) {
	if got := normalizeDate("2020/01/15"); got != "2020-01-15" {
		t.Errorf("normalizeDate = %q, want 2020-01-15", got)
	}
	if got := normalizeDate(""); got != "" {
		t.Errorf("normalizeDate empty = %q, want empty", got)
	}
}
