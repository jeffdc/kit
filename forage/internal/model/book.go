package model

var validStatuses = map[string]bool{
	"wishlist": true,
	"reading":  true,
	"read":     true,
	"dropped":  true,
}

func ValidStatus(s string) bool {
	return validStatuses[s]
}

func IsTerminal(s string) bool {
	return s == "dropped"
}

type Book struct {
	ID        string   `yaml:"-" json:"id"`
	Filename  string   `yaml:"-" json:"file"`
	Title     string   `yaml:"title" json:"title"`
	Author    string   `yaml:"author" json:"author"`
	Status    string   `yaml:"status" json:"status"`
	Tags      []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Rating    int      `yaml:"rating,omitempty" json:"rating,omitempty"`
	DateAdded string   `yaml:"date_added" json:"date_added"`
	DateRead  string   `yaml:"date_read,omitempty" json:"date_read,omitempty"`
	Body      string   `yaml:"-" json:"body,omitempty"`
}
