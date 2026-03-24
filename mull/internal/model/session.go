package model

import "time"

type Session struct {
	Date     time.Time `yaml:"date" json:"date"`
	Matters  []string  `yaml:"matters,omitempty" json:"matters,omitempty"`
	Filename string    `yaml:"-" json:"file"`
	Body     string    `yaml:"-" json:"body,omitempty"`
}
