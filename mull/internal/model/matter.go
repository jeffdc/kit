package model

import "time"

type Matter struct {
	// Core fields
	ID       string `yaml:"-" json:"id"`
	Filename string `yaml:"-" json:"file"`
	Title    string `yaml:"-" json:"title"`

	// Required metadata
	Status  string `yaml:"status" json:"status"`
	Created string `yaml:"created" json:"created"`
	Updated string `yaml:"updated" json:"updated"`

	// Optional metadata
	Tags   []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Effort string   `yaml:"effort,omitempty" json:"effort,omitempty"`
	Plan   string   `yaml:"plan,omitempty" json:"plan,omitempty"`

	// Relationships
	Relates []string `yaml:"relates,omitempty" json:"relates,omitempty"`
	Blocks  []string `yaml:"blocks,omitempty" json:"blocks,omitempty"`
	Needs   []string `yaml:"needs,omitempty" json:"needs,omitempty"`
	Parent  string   `yaml:"parent,omitempty" json:"parent,omitempty"`

	// Extra arbitrary key-value pairs
	Extra map[string]interface{} `yaml:"-" json:"extra,omitempty"`

	// Body (markdown content after frontmatter)
	Body string `yaml:"-" json:"body,omitempty"`
}

func Today() string {
	return time.Now().Format("2006-01-02")
}
