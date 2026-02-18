package cmd

import (
	"mull/internal/model"
	"strings"
)

// matterConfirmation is a compact response for mutation commands.
type matterConfirmation struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func confirm(m *model.Matter) matterConfirmation {
	return matterConfirmation{ID: m.ID, Title: m.Title, Status: m.Status}
}

// appendConfirmation is a compact response for body mutation commands.
type appendConfirmation struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Lines int    `json:"lines"`
}

func confirmAppend(m *model.Matter) appendConfirmation {
	lines := 0
	if m.Body != "" {
		lines = len(strings.Split(strings.TrimRight(m.Body, "\n"), "\n"))
	}
	return appendConfirmation{ID: m.ID, Title: m.Title, Lines: lines}
}

// stripBodies returns copies of matters with bodies removed, for list output.
func stripBodies(matters []*model.Matter) []*model.Matter {
	out := make([]*model.Matter, len(matters))
	for i, m := range matters {
		cp := *m
		cp.Body = ""
		out[i] = &cp
	}
	return out
}
