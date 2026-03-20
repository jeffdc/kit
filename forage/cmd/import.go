package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"forage/internal/changes"

	"github.com/spf13/cobra"
)

// csvBook is an intermediate representation of a book parsed from CSV.
type csvBook struct {
	Title     string
	Author    string
	Rating    int
	Status    string
	Tags      []string
	DateAdded string
	DateRead  string
}

// middleInitialRe matches single-letter middle initials like "W." or "K."
var middleInitialRe = regexp.MustCompile(`\b[A-Za-z]\.\s*`)

// trailingParenRe matches trailing parentheticals like "(Series, #2)" or "(English and Italian Edition)".
var trailingParenRe = regexp.MustCompile(`\s*\([^)]*\)\s*$`)

// normalizeAuthor collapses whitespace, strips middle initials, and takes
// only the first author (StoryGraph lists multiple, Goodreads uses one).
func normalizeAuthor(author string) string {
	// Take first author only (before first comma)
	if i := strings.IndexByte(author, ','); i >= 0 {
		author = author[:i]
	}
	author = strings.ToLower(strings.TrimSpace(author))
	// Strip middle initials like "w." or "f."
	author = middleInitialRe.ReplaceAllString(author, "")
	// Collapse whitespace
	return strings.Join(strings.Fields(author), " ")
}

// normalizeTitle strips trailing parentheticals (series info), subtitles
// after the first colon, and normalizes punctuation and whitespace.
// Goodreads appends "(Series, #N)", StoryGraph often omits subtitles,
// and minor punctuation differs ("Black and White" vs "Black-and-White").
func normalizeTitle(title string) string {
	t := strings.TrimSpace(title)
	// Strip trailing parenthetical: "Reaper Man (Discworld, #11)" -> "Reaper Man"
	t = trailingParenRe.ReplaceAllString(t, "")
	// Strip subtitle after colon: "The Tipping Point: How Little..." -> "The Tipping Point"
	if i := strings.IndexByte(t, ':'); i >= 0 {
		t = t[:i]
	}
	t = strings.ToLower(t)
	// Normalize hyphens and punctuation to spaces
	t = strings.Map(func(r rune) rune {
		if r == '-' || r == '\'' || r == ',' || r == '.' || r == '!' || r == '?' {
			return ' '
		}
		return r
	}, t)
	return strings.Join(strings.Fields(t), " ")
}

func normalizeKey(title, author string) string {
	return normalizeTitle(title) + "\t" + normalizeAuthor(author)
}

// statusPriority returns a numeric priority for merging (higher = more specific).
var statusPriority = map[string]int{
	"wishlist": 1,
	"paused":   2,
	"reading":  3,
	"dropped":  4,
	"read":     5,
}

var goodreadsStatusMap = map[string]string{
	"read":              "read",
	"to-read":           "wishlist",
	"currently-reading": "reading",
	"abandoned":         "dropped",
	"stuck":             "paused",
}

var storygraphStatusMap = map[string]string{
	"read":              "read",
	"to-read":           "wishlist",
	"currently-reading": "reading",
	"did-not-finish":    "dropped",
	"paused":            "paused",
}

// statusLikeValues are bookshelves/tags that should be stripped, not treated as real tags.
var statusLikeValues = map[string]bool{
	"read": true, "to-read": true, "currently-reading": true,
	"abandoned": true, "stuck": true, "did-not-finish": true, "paused": true,
}

type csvFormat int

const (
	formatGoodreads csvFormat = iota
	formatStoryGraph
)

func detectFormat(headers []string) (csvFormat, error) {
	headerSet := make(map[string]bool)
	for _, h := range headers {
		headerSet[h] = true
	}
	if headerSet["Exclusive Shelf"] && headerSet["Bookshelves"] {
		return formatGoodreads, nil
	}
	if headerSet["Read Status"] && headerSet["Star Rating"] {
		return formatStoryGraph, nil
	}
	return 0, fmt.Errorf("unrecognized CSV format: expected Goodreads or StoryGraph headers")
}

func parseCSV(path string) ([]csvBook, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file is empty or has no data rows")
	}

	headers := records[0]
	format, err := detectFormat(headers)
	if err != nil {
		return nil, err
	}

	colIndex := make(map[string]int)
	for i, h := range headers {
		colIndex[h] = i
	}

	var books []csvBook
	for _, row := range records[1:] {
		var b csvBook
		switch format {
		case formatGoodreads:
			b = parseGoodreadsRow(row, colIndex)
		case formatStoryGraph:
			b = parseStoryGraphRow(row, colIndex)
		}
		if b.Title == "" || b.Author == "" {
			continue
		}
		books = append(books, b)
	}
	return books, nil
}

func col(row []string, colIndex map[string]int, name string) string {
	i, ok := colIndex[name]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func parseGoodreadsRow(row []string, colIndex map[string]int) csvBook {
	shelf := col(row, colIndex, "Exclusive Shelf")
	status := goodreadsStatusMap[shelf]
	if status == "" {
		status = "wishlist"
	}

	ratingStr := col(row, colIndex, "My Rating")
	rating := 0
	if n, err := strconv.Atoi(ratingStr); err == nil && n > 0 && n <= 5 {
		rating = n
	}

	bookshelves := col(row, colIndex, "Bookshelves")
	var tags []string
	if bookshelves != "" {
		for _, t := range strings.Split(bookshelves, ",") {
			t = strings.TrimSpace(t)
			if t != "" && !statusLikeValues[t] && t != shelf {
				tags = append(tags, t)
			}
		}
	}

	dateAdded := normalizeDate(col(row, colIndex, "Date Added"))
	dateRead := normalizeDate(col(row, colIndex, "Date Read"))

	return csvBook{
		Title:     col(row, colIndex, "Title"),
		Author:    col(row, colIndex, "Author"),
		Rating:    rating,
		Status:    status,
		Tags:      tags,
		DateAdded: dateAdded,
		DateRead:  dateRead,
	}
}

func parseStoryGraphRow(row []string, colIndex map[string]int) csvBook {
	readStatus := col(row, colIndex, "Read Status")
	status := storygraphStatusMap[readStatus]
	if status == "" {
		status = "wishlist"
	}

	ratingStr := col(row, colIndex, "Star Rating")
	rating := 0
	if ratingStr != "" {
		if f, err := strconv.ParseFloat(ratingStr, 64); err == nil && f > 0 {
			rating = int(math.Round(f))
			if rating > 5 {
				rating = 5
			}
		}
	}

	tagStr := col(row, colIndex, "Tags")
	var tags []string
	if tagStr != "" && !statusLikeValues[tagStr] {
		tags = append(tags, tagStr)
	}

	dateAdded := normalizeDate(col(row, colIndex, "Date Added"))
	dateRead := normalizeDate(col(row, colIndex, "Last Date Read"))

	return csvBook{
		Title:     col(row, colIndex, "Title"),
		Author:    col(row, colIndex, "Authors"),
		Rating:    rating,
		Status:    status,
		Tags:      tags,
		DateAdded: dateAdded,
		DateRead:  dateRead,
	}
}

// normalizeDate converts YYYY/MM/DD to YYYY-MM-DD.
func normalizeDate(d string) string {
	return strings.ReplaceAll(d, "/", "-")
}

func mergeBooks(a, b csvBook) csvBook {
	merged := a

	// Prefer higher rating (non-zero wins)
	if b.Rating > merged.Rating {
		merged.Rating = b.Rating
	}

	// Prefer more specific status
	if statusPriority[b.Status] > statusPriority[merged.Status] {
		merged.Status = b.Status
	}

	// Merge tags
	tagSet := make(map[string]bool)
	for _, t := range merged.Tags {
		tagSet[t] = true
	}
	for _, t := range b.Tags {
		if !tagSet[t] {
			merged.Tags = append(merged.Tags, t)
		}
	}

	// Prefer earlier date_added
	if b.DateAdded != "" && (merged.DateAdded == "" || b.DateAdded < merged.DateAdded) {
		merged.DateAdded = b.DateAdded
	}

	// Prefer latest date_read
	if b.DateRead != "" && b.DateRead > merged.DateRead {
		merged.DateRead = b.DateRead
	}

	return merged
}

type importSummary struct {
	Imported        int `json:"imported"`
	SkippedExisting int `json:"skipped_existing"`
	SkippedDupe     int `json:"skipped_duplicate"`
}

var importCmd = &cobra.Command{
	Use:   "import <file> [file...]",
	Short: "Import books from Goodreads or StoryGraph CSV exports",
	Long: `Import books from Goodreads or StoryGraph CSV exports. Auto-detects format
by header inspection. Multiple files are merged with deduplication.

Also supports JSON changelog import from PWA sync with --changes.

Examples:
  forage import goodreads.csv
  forage import goodreads.csv storygraph.csv    # merge and deduplicate
  forage import --dry-run export.csv            # preview without writing
  forage import changes.json --changes          # apply PWA changelog

CSV output:  {"imported": 42, "skipped_existing": 3, "skipped_duplicate": 1}
JSON output: {"applied": 10, "skipped": 1, "errors": 0}`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		changes, _ := cmd.Flags().GetBool("changes")
		if changes {
			return runImportChanges(args[0])
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")

		// Parse all CSV files and merge by normalized key
		merged := make(map[string]csvBook)
		var mergeOrder []string

		for _, path := range args {
			books, err := parseCSV(path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}
			for _, b := range books {
				key := normalizeKey(b.Title, b.Author)
				if existing, ok := merged[key]; ok {
					merged[key] = mergeBooks(existing, b)
				} else {
					mergeOrder = append(mergeOrder, key)
					merged[key] = b
				}
			}
		}

		// Load existing forage library
		existing, err := store.ListBooks(nil)
		if err != nil {
			return err
		}
		existingSet := make(map[string]bool)
		for _, b := range existing {
			existingSet[normalizeKey(b.Title, b.Author)] = true
		}

		summary := importSummary{
			SkippedDupe: len(merged) - len(mergeOrder),
		}

		for _, key := range mergeOrder {
			b := merged[key]
			if existingSet[key] {
				summary.SkippedExisting++
				continue
			}

			if dryRun {
				summary.Imported++
				continue
			}

			meta := make(map[string]string)
			meta["status"] = b.Status
			if b.Rating > 0 {
				meta["rating"] = strconv.Itoa(b.Rating)
			}
			if len(b.Tags) > 0 {
				meta["tags"] = strings.Join(b.Tags, ",")
			}
			if b.DateAdded != "" {
				meta["date_added"] = b.DateAdded
			}
			if b.DateRead != "" {
				meta["date_read"] = b.DateRead
			}

			_, err := store.CreateBook(b.Title, b.Author, meta)
			if err != nil {
				return fmt.Errorf("creating book %q: %w", b.Title, err)
			}
			summary.Imported++
		}

		return json.NewEncoder(os.Stdout).Encode(summary)
	},
}

func runImportChanges(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading changelog: %w", err)
	}

	var cl changes.Changelog
	if err := json.Unmarshal(data, &cl); err != nil {
		return fmt.Errorf("parsing changelog: %w", err)
	}

	summary := changes.Apply(store, cl)
	return json.NewEncoder(os.Stdout).Encode(summary)
}

func init() {
	importCmd.Flags().Bool("dry-run", false, "Show what would be imported without writing anything")
	importCmd.Flags().Bool("changes", false, "Import a JSON changelog (from PWA sync) instead of CSV")
	rootCmd.AddCommand(importCmd)
}
