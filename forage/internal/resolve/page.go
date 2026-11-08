package resolve

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
)

const maxPageBytes = 2 << 20

// pageUserAgent identifies the app without any personal contact details.
const pageUserAgent = "Mozilla/5.0 (compatible; Forage/1.0; personal book list)"

// pageMeta is what a web page says about the book it describes.
type pageMeta struct {
	Title  string
	Author string
	ISBN   string
}

var (
	reMetaTag   = regexp.MustCompile(`(?is)<meta\s[^>]*>`)
	reAttr      = regexp.MustCompile(`(?is)([a-z:_-]+)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	reTitleTag  = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reLDJSONTag = regexp.MustCompile(`(?is)<script[^>]*type\s*=\s*["']application/ld\+json["'][^>]*>(.*?)</script>`)
)

// fetchPage downloads an HTML page with the guarded client.
func (r *Resolver) fetchPage(ctx context.Context, u string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", pageUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en")
	resp, err := r.pageClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); mt != "" && mt != "text/html" && mt != "application/xhtml+xml" {
		return "", fmt.Errorf("not HTML: %s", mt)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes))
	return string(body), err
}

// extractMeta reads book details from a page: JSON-LD Book data first,
// then Open Graph book tags, then the page title.
func extractMeta(page string) pageMeta {
	var m pageMeta

	for _, match := range reLDJSONTag.FindAllStringSubmatch(page, -1) {
		var doc any
		if json.Unmarshal([]byte(strings.TrimSpace(match[1])), &doc) != nil {
			continue
		}
		if b := findLDBook(doc); b != nil {
			m.Title = collapse(html.UnescapeString(ldString(b["name"])))
			m.Author = ldAuthor(b["author"])
			m.ISBN = NormalizeISBN(ldString(b["isbn"]))
			if m.ISBN == "" {
				m.ISBN = ldWorkExampleISBN(b["workExample"])
			}
			break
		}
	}

	metas := map[string]string{}
	for _, tag := range reMetaTag.FindAllString(page, -1) {
		attrs := map[string]string{}
		for _, a := range reAttr.FindAllStringSubmatch(tag, -1) {
			attrs[strings.ToLower(a[1])] = a[2] + a[3]
		}
		key := strings.ToLower(attrs["property"] + attrs["name"])
		if key != "" && attrs["content"] != "" {
			if _, seen := metas[key]; !seen {
				metas[key] = html.UnescapeString(attrs["content"])
			}
		}
	}
	if m.ISBN == "" {
		m.ISBN = NormalizeISBN(firstNonEmpty(metas["books:isbn"], metas["book:isbn"]))
	}
	if m.Author == "" {
		if a := firstNonEmpty(metas["books:author"], metas["book:author"]); a != "" && !strings.HasPrefix(a, "http") {
			m.Author = collapse(a)
		}
	}
	if m.Title == "" {
		title := firstNonEmpty(metas["og:title"], metas["twitter:title"])
		if title == "" {
			if t := reTitleTag.FindStringSubmatch(page); t != nil {
				title = t[1]
			}
		}
		if !isJunkTitle(title) {
			t, a := cleanPageTitle(title)
			m.Title = t
			if m.Author == "" {
				m.Author = a
			}
		}
	}
	m.Title = stripSeries(m.Title)
	return m
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// findLDBook walks a JSON-LD document (object, array, or @graph) for an
// object whose @type is or includes "Book".
func findLDBook(doc any) map[string]any {
	switch v := doc.(type) {
	case []any:
		for _, item := range v {
			if b := findLDBook(item); b != nil {
				return b
			}
		}
	case map[string]any:
		if isLDType(v["@type"], "Book") {
			return v
		}
		if g, ok := v["@graph"]; ok {
			return findLDBook(g)
		}
		if e, ok := v["mainEntity"]; ok {
			return findLDBook(e)
		}
	}
	return nil
}

func isLDType(t any, want string) bool {
	switch v := t.(type) {
	case string:
		return v == want
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

func ldString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprintf("%.0f", x)
	case []any:
		if len(x) > 0 {
			return ldString(x[0])
		}
	}
	return ""
}

// ldAuthor reads an author given as a string, a Person, or a list of either.
func ldAuthor(v any) string {
	switch x := v.(type) {
	case string:
		return collapse(html.UnescapeString(x))
	case map[string]any:
		return collapse(html.UnescapeString(ldString(x["name"])))
	case []any:
		if len(x) > 0 {
			return ldAuthor(x[0])
		}
	}
	return ""
}

func ldWorkExampleISBN(v any) string {
	switch x := v.(type) {
	case map[string]any:
		return NormalizeISBN(ldString(x["isbn"]))
	case []any:
		for _, e := range x {
			if isbn := ldWorkExampleISBN(e); isbn != "" {
				return isbn
			}
		}
	}
	return ""
}
