package resolve

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	reSpaces        = regexp.MustCompile(`\s+`)
	reTrailingParen = regexp.MustCompile(`\s*\([^)]*\)\s*$`)
	reSiteSuffix    = regexp.MustCompile(`\s+(?:\||-|–|—|·)\s+`)
	reBy            = regexp.MustCompile(`(?i)\s+by\s+`)
)

// junkTitles are page titles from bot checks and error pages; they say
// nothing about the book.
var junkTitles = []string{
	"client challenge", "just a moment", "attention required", "access denied", "robot check",
	"captcha", "are you a robot", "verify you are human", "verify_human", "human verification", "page not found",
	"403 forbidden", "not found", "sign in", "log in",
}

func isJunkTitle(t string) bool {
	lt := strings.ToLower(strings.TrimSpace(t))
	if lt == "" || lt == "amazon.com" {
		return true
	}
	for _, j := range junkTitles {
		if strings.Contains(lt, j) {
			return true
		}
	}
	return false
}

func collapse(s string) string {
	return strings.TrimSpace(reSpaces.ReplaceAllString(s, " "))
}

// stripSeries removes a trailing parenthetical: "Dune (Dune, #1)" -> "Dune",
// "Piranesi (novel)" -> "Piranesi".
func stripSeries(t string) string {
	for {
		s := reTrailingParen.ReplaceAllString(t, "")
		if s == t || s == "" {
			return t
		}
		t = s
	}
}

// cleanPageTitle turns a page or link-card title into a search query:
// "Dune (Dune, #1) by Frank Herbert | Goodreads" -> ("Dune", "Frank Herbert").
func cleanPageTitle(raw string) (title, author string) {
	t := collapse(html.UnescapeString(raw))
	if parts := reSiteSuffix.Split(t, 2); len(parts) > 1 {
		t = parts[0]
	}
	if loc := reBy.FindAllStringIndex(t, -1); len(loc) > 0 {
		last := loc[len(loc)-1]
		title, author = t[:last[0]], t[last[1]:]
	} else {
		title = t
	}
	return collapse(stripSeries(title)), collapse(author)
}

// slugQuery builds a search query from the most descriptive path segment
// of a URL, e.g. bookshop.org/p/books/piranesi-susanna-clarke/14399474 ->
// "piranesi susanna clarke". Returns "" if no segment looks like words.
func slugQuery(u *url.URL) string {
	best := ""
	for _, seg := range strings.Split(u.Path, "/") {
		seg, _ = url.PathUnescape(seg)
		for _, ext := range []string{".html", ".htm", ".php", ".aspx"} {
			seg = strings.TrimSuffix(seg, ext)
		}
		// "234225.Dune" (Goodreads) -> "Dune"
		if i := strings.IndexByte(seg, '.'); i > 0 && isDigits(seg[:i]) {
			seg = seg[i+1:]
		}
		if !strings.ContainsAny(seg, "-_") && !strings.Contains(seg, " ") {
			continue
		}
		words := strings.FieldsFunc(seg, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
		var kept []string
		letters := 0
		for _, w := range words {
			if isDigits(w) {
				continue
			}
			kept = append(kept, w)
			for _, r := range w {
				if unicode.IsLetter(r) {
					letters++
				}
			}
		}
		cand := strings.Join(kept, " ")
		if letters >= 4 && len(cand) > len(best) {
			best = cand
		}
	}
	return collapse(stripSeries(best))
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// splitBy splits free text like "Piranesi by Susanna Clarke".
func splitBy(text string) (title, author string) {
	if loc := reBy.FindAllStringIndex(text, -1); len(loc) > 0 {
		last := loc[len(loc)-1]
		return collapse(text[:last[0]]), collapse(text[last[1]:])
	}
	return collapse(text), ""
}

// --- Normalization for matching against the library ---

var foldAccents = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

func fold(s string) string {
	out, _, err := transform.String(foldAccents, s)
	if err != nil {
		return s
	}
	return out
}

func wordsOnly(s string) string {
	s = strings.ReplaceAll(s, "&", " and ")
	return collapse(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, fold(s)))
}

// NormalizeTitle reduces a title to a comparison key: no accents, case,
// punctuation, subtitle, series parenthetical, or leading article.
func NormalizeTitle(t string) string {
	t = stripSeries(collapse(t))
	if i := strings.IndexByte(t, ':'); i > 0 {
		t = t[:i]
	}
	t = wordsOnly(t)
	for _, article := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(t, article) && len(t) > len(article) {
			t = t[len(article):]
			break
		}
	}
	return t
}

// Surname returns a comparison key for an author's last name. It handles
// "Frank Herbert", "Herbert, Frank", initials, and Jr./Sr./roman suffixes.
func Surname(author string) string {
	a := collapse(author)
	if i := strings.IndexByte(a, ','); i > 0 {
		before, after := a[:i], strings.TrimSpace(a[i+1:])
		// "Herbert, Frank" puts the surname first; "Martin Luther King, Jr." does not.
		if !isNameSuffix(after) {
			return wordsOnly(before)
		}
		a = before
	}
	words := strings.Fields(wordsOnly(a))
	for len(words) > 1 && isNameSuffix(words[len(words)-1]) {
		words = words[:len(words)-1]
	}
	if len(words) == 0 {
		return ""
	}
	return words[len(words)-1]
}

func isNameSuffix(w string) bool {
	switch strings.Trim(strings.ToLower(w), ". ") {
	case "jr", "sr", "ii", "iii", "iv", "phd", "md":
		return true
	}
	return false
}

// fillerWords carry no identifying information in a search query.
var fillerWords = map[string]bool{
	"the": true, "a": true, "an": true, "of": true, "and": true, "by": true,
	"novel": true, "book": true, "books": true, "paperback": true, "hardcover": true, "edition": true,
}

func wordSet(s string) map[string]bool {
	set := map[string]bool{}
	for _, w := range strings.Fields(wordsOnly(s)) {
		if !fillerWords[w] {
			set[w] = true
		}
	}
	return set
}

func subset(a, b map[string]bool) bool {
	if len(a) == 0 {
		return false
	}
	for w := range a {
		if !b[w] {
			return false
		}
	}
	return true
}

// relevant reports whether a candidate plausibly is what a loosely derived
// query (URL slug, page title) describes: every word of its title appears in
// the query, or every query word appears in its title or author.
func relevant(query string, c Candidate) bool {
	q := wordSet(query)
	t := wordSet(NormalizeTitle(c.Title))
	if subset(t, q) {
		return true
	}
	ta := wordSet(c.Title + " " + c.Author)
	return subset(q, ta)
}
