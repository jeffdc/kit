package mull

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Tours give paths (/trees/quercus-alba), never full URLs: a path resolves
// against the item's plan's preview, the URL that serves the plan's target
// branch, so a tour still works after the worker's own dev server is gone.
// mull serve and report --text resolve them when they show a tour; the
// report keeps the steps as written.

// tourPath finds a path in a tour step: a "/" at the start of the step or
// after a space, an opening bracket or a quote, up to the next space or
// closing bracket or quote. A path inside a full URL is never matched, since
// its "/" follows the host.
var tourPath = regexp.MustCompile("(^|[\\s(\\[\"'`])(/[^\\s)\\]\"'`<>]*)")

// tourURL finds a full URL in a tour step, or a scheme-less local address
// with a port (localhost:4000/trees). An IPv6 host keeps its brackets
// (http://[::1]:4000/).
var tourURL = regexp.MustCompile("(?i)(?:https?://(?:\\[[0-9a-f:.]+\\][^\\s)\\]\"'`<>]*|[^\\s)\\]\"'`<>]+)|\\b(?:localhost|127\\.0\\.0\\.1|0\\.0\\.0\\.0):\\d+[^\\s)\\]\"'`<>]*)")

// trailing is punctuation that ends a sentence rather than a path or URL.
const trailing = ".,;:!?"

// TourPart is a piece of a tour step: its text, and for a path, the URL it
// resolves to against the plan's preview.
type TourPart struct {
	Text string `json:"text"`
	URL  string `json:"url,omitempty"`
}

// previewBase is what a tour path is appended to: the preview's scheme,
// host and path, without a trailing slash, query or fragment. A preview
// served under a path (https://example.com/app) keeps it.
func previewBase(preview string) string {
	u, err := url.Parse(preview)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + strings.TrimRight(u.Path, "/")
}

// tourParts splits a step into text and the paths in it, each resolved
// against preview. With no preview, the step is one part with no URL.
func tourParts(step, preview string) []TourPart {
	base := previewBase(preview)
	if base == "" {
		return []TourPart{{Text: step}}
	}
	var parts []TourPart
	at := 0
	for _, m := range tourPath.FindAllStringSubmatchIndex(step, -1) {
		start, end := m[4], m[5]
		path := strings.TrimRight(step[start:end], trailing)
		if strings.HasPrefix(path, "//") {
			continue // protocol-relative, not a path
		}
		end = start + len(path)
		if start > at {
			parts = append(parts, TourPart{Text: step[at:start]})
		}
		parts = append(parts, TourPart{Text: path, URL: base + path})
		at = end
	}
	if at < len(step) || len(parts) == 0 {
		parts = append(parts, TourPart{Text: step[at:]})
	}
	return parts
}

// resolveTourStep is a step with each path replaced by its URL against
// preview, for text: a full URL is what a terminal makes clickable.
func resolveTourStep(step, preview string) string {
	var b strings.Builder
	for _, p := range tourParts(step, preview) {
		if p.URL != "" {
			b.WriteString(p.URL)
		} else {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

// resolveTour returns tour's steps resolved against preview, for a review
// packet: nil for no tour.
func resolveTour(tour []string, preview string) [][]TourPart {
	if len(tour) == 0 {
		return nil
	}
	out := make([][]TourPart, len(tour))
	for k, step := range tour {
		out[k] = tourParts(step, preview)
	}
	return out
}

// resolveReportText resolves the tour steps in a report's markdown against
// preview, at read time: the steps are listed under "### UI tour" as
// "1. step", one per line, as renderReport writes them. Only those lines
// change, so a report rendered by an older mull keeps the rest as written.
func resolveReportText(text string, tour []string, preview string) string {
	if previewBase(preview) == "" || len(tour) == 0 {
		return text
	}
	heading := "### UI tour\n"
	start := strings.Index(text, heading)
	if start < 0 {
		heading = "### Look at\n" // reports since the redesign
		if start = strings.Index(text, heading); start < 0 {
			return text
		}
	}
	start += len(heading)
	end := len(text)
	if k := strings.Index(text[start:], "\n### "); k >= 0 {
		end = start + k + 1
	}
	section := text[start:end]
	for k, step := range tour {
		line := fmt.Sprintf("%d. %s\n", k+1, step)
		section = strings.Replace(section, line, fmt.Sprintf("%d. %s\n", k+1, resolveTourStep(step, preview)), 1)
	}
	return text[:start] + section + text[end:]
}

// localHosts are the hosts a worker's own dev server answers on.
var localHosts = map[string]bool{"localhost": true, "127.0.0.1": true, "0.0.0.0": true, "::1": true}

// tourWarnings are done's soft warnings about a tour: a step that gives a
// full local URL (http://localhost:4001/trees) that isn't on the plan's
// preview. Such a URL usually points at the worker's own dev server, which
// is gone by the time the tour is walked; a path resolves against the
// preview instead. Other URLs (a staging site, the docs) are left alone.
func tourWarnings(tour []string, preview string) []string {
	var want *url.URL
	if preview != "" {
		want, _ = url.Parse(preview)
	}
	var out []string
	for k, step := range tour {
		for _, raw := range tourURL.FindAllString(step, -1) {
			raw = strings.TrimRight(raw, trailing)
			full := raw
			if !strings.Contains(strings.ToLower(raw), "://") {
				full = "http://" + raw
			}
			u, err := url.Parse(full)
			if err != nil {
				continue
			}
			host := strings.ToLower(u.Hostname())
			if !localHosts[host] && !strings.HasSuffix(host, ".localhost") {
				continue
			}
			if want != nil && strings.EqualFold(u.Scheme, want.Scheme) && strings.EqualFold(u.Host, want.Host) {
				continue
			}
			path := u.EscapedPath()
			if path == "" {
				path = "/"
			}
			if u.RawQuery != "" {
				path += "?" + u.RawQuery
			}
			if u.Fragment != "" {
				path += "#" + u.EscapedFragment()
			}
			if want != nil {
				out = append(out, fmt.Sprintf("tour step %d gives %s, a local URL that isn't the plan's preview (%s): give the path (%s) instead, which resolves against the preview", k+1, raw, preview, path))
			} else {
				out = append(out, fmt.Sprintf("tour step %d gives %s, a local URL, and the item's plan has no preview: give the path (%s) instead, which resolves once the plan has one", k+1, raw, path))
			}
		}
	}
	return out
}
