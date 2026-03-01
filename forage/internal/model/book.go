package model

var validStatuses = map[string]bool{
	"wishlist": true,
	"reading":  true,
	"paused":   true,
	"read":     true,
	"dropped":  true,
}

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
