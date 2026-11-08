package resolve

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// reBlueskyPost matches https://bsky.app/profile/{handle or did}/post/{rkey}.
var reBlueskyPost = regexp.MustCompile(`^/profile/([^/]+)/post/([A-Za-z0-9]+)/?$`)

type blueskyPost struct {
	Text      string
	Links     []string // link card first, then links in the text
	CardTitle string
}

type bskyExternal struct {
	URI   string `json:"uri"`
	Title string `json:"title"`
}

type bskyThread struct {
	Thread struct {
		Post struct {
			Record struct {
				Text   string `json:"text"`
				Facets []struct {
					Features []struct {
						URI string `json:"uri"`
					} `json:"features"`
				} `json:"facets"`
			} `json:"record"`
			Embed struct {
				External *bskyExternal `json:"external"`
				Media    *struct {
					External *bskyExternal `json:"external"`
				} `json:"media"`
			} `json:"embed"`
		} `json:"post"`
	} `json:"thread"`
}

func isBlueskyPost(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	return (host == "bsky.app" || host == "www.bsky.app") && reBlueskyPost.MatchString(u.Path)
}

// fetchBlueskyPost reads a public post through Bluesky's public API.
func (r *Resolver) fetchBlueskyPost(ctx context.Context, u *url.URL) (*blueskyPost, error) {
	m := reBlueskyPost.FindStringSubmatch(u.Path)
	if m == nil {
		return nil, fmt.Errorf("not a post URL")
	}
	atURI := "at://" + m[1] + "/app.bsky.feed.post/" + m[2]
	q := url.Values{"uri": {atURI}, "depth": {"0"}, "parentHeight": {"0"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.BlueskyURL+"/xrpc/app.bsky.feed.getPostThread?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", apiUserAgent)
	resp, err := r.apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bluesky: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bluesky: HTTP %d", resp.StatusCode)
	}
	var t bskyThread
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return nil, fmt.Errorf("bluesky: %w", err)
	}

	p := t.Thread.Post
	post := &blueskyPost{Text: p.Record.Text}
	seen := map[string]bool{}
	add := func(link string) {
		lu, err := url.Parse(link)
		if err != nil || (lu.Scheme != "http" && lu.Scheme != "https") || seen[link] {
			return
		}
		if h := strings.ToLower(lu.Hostname()); h == "bsky.app" || strings.HasSuffix(h, ".bsky.app") {
			return
		}
		seen[link] = true
		post.Links = append(post.Links, link)
	}
	card := p.Embed.External
	if card == nil && p.Embed.Media != nil {
		card = p.Embed.Media.External
	}
	if card != nil {
		add(card.URI)
		post.CardTitle = card.Title
	}
	for _, f := range p.Record.Facets {
		for _, feat := range f.Features {
			if feat.URI != "" {
				add(feat.URI)
			}
		}
	}
	return post, nil
}
