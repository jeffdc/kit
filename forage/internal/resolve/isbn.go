package resolve

import (
	"regexp"
	"strings"
)

// NormalizeISBN returns s as a bare ISBN-10 or ISBN-13 if it is one (with
// optional "ISBN" prefix, spaces and hyphens) and its checksum is valid,
// otherwise "".
func NormalizeISBN(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "ISBN-13")
	s = strings.TrimPrefix(s, "ISBN-10")
	s = strings.TrimPrefix(s, "ISBN")
	s = strings.TrimLeft(s, ": ")
	s = strings.NewReplacer("-", "", " ", "").Replace(s)
	if validISBN13(s) || validISBN10(s) {
		return s
	}
	return ""
}

func validISBN13(s string) bool {
	if len(s) != 13 || !(strings.HasPrefix(s, "978") || strings.HasPrefix(s, "979")) {
		return false
	}
	sum := 0
	for i, r := range s {
		if r < '0' || r > '9' {
			return false
		}
		d := int(r - '0')
		if i%2 == 1 {
			d *= 3
		}
		sum += d
	}
	return sum%10 == 0
}

func validISBN10(s string) bool {
	if len(s) != 10 {
		return false
	}
	sum := 0
	for i, r := range s {
		var d int
		switch {
		case r >= '0' && r <= '9':
			d = int(r - '0')
		case r == 'X' && i == 9:
			d = 10
		default:
			return false
		}
		sum += d * (10 - i)
	}
	return sum%11 == 0
}

var (
	reURLISBN13 = regexp.MustCompile(`(?:^|[^0-9])(97[89][0-9]{10})(?:[^0-9]|$)`)
	reURLISBN10 = regexp.MustCompile(`(?i)/(?:dp|gp/product|isbn|ISBN)/([0-9]{9}[0-9X])(?:[/?]|$)`)
)

// isbnFromURL finds an ISBN in a URL: any 13-digit ISBN (Bookshop, Biblio,
// many publishers) or an Amazon-style /dp/ISBN-10. Kindle ASINs (B0...)
// don't qualify.
func isbnFromURL(raw string) string {
	for _, m := range reURLISBN13.FindAllStringSubmatch(raw, -1) {
		if validISBN13(m[1]) {
			return m[1]
		}
	}
	if m := reURLISBN10.FindStringSubmatch(raw); m != nil && validISBN10(strings.ToUpper(m[1])) {
		return strings.ToUpper(m[1])
	}
	return ""
}
