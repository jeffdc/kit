package model

// StatusList is the canonical ordered list of valid statuses.
var StatusList = []string{"wishlist", "owned", "reading", "paused", "read", "dropped"}

var validStatuses = func() map[string]bool {
	m := make(map[string]bool, len(StatusList))
	for _, s := range StatusList {
		m[s] = true
	}
	return m
}()

func ValidStatus(s string) bool {
	return validStatuses[s]
}

func IsTerminal(s string) bool {
	return s == "dropped"
}

type Bookseller struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Book struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Author    string   `json:"author"`
	Status    string   `json:"status"`
	Tags      []string `json:"tags,omitempty"`
	Rating    int      `json:"rating,omitempty"`
	DateAdded string   `json:"date_added"`
	DateRead  string   `json:"date_read,omitempty"`
	Body      string   `json:"body,omitempty"`
}
