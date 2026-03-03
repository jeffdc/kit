package tui

import (
	"strings"

	"mull/internal/model"
)

type sortMode int

const (
	sortTitle   sortMode = iota
	sortCreated          // newest first
	sortUpdated          // newest first
	sortStatus           // lifecycle order
)

var sortModes = []sortMode{sortTitle, sortCreated, sortUpdated, sortStatus}

var sortLabels = map[sortMode]string{
	sortTitle:   "title",
	sortCreated: "created",
	sortUpdated: "updated",
	sortStatus:  "status",
}

var statusOrder = map[string]int{
	"raw":     0,
	"refined": 1,
	"planned": 2,
	"active":  3,
	"done":    4,
	"dropped": 5,
}

func sortFunc(mode sortMode, matters []*model.Matter) func(i, j int) bool {
	return func(i, j int) bool {
		a, b := matters[i], matters[j]
		switch mode {
		case sortCreated:
			return a.Created > b.Created
		case sortUpdated:
			return a.Updated > b.Updated
		case sortStatus:
			return statusOrder[a.Status] < statusOrder[b.Status]
		default: // sortTitle
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		}
	}
}

func (a *App) cycleSort() {
	for i, m := range sortModes {
		if m == a.sortMode {
			a.sortMode = sortModes[(i+1)%len(sortModes)]
			return
		}
	}
	a.sortMode = sortTitle
}
