package model

type DocketEntry struct {
	ID   string `yaml:"id" json:"id"`
	Note string `yaml:"note,omitempty" json:"note,omitempty"`
}
