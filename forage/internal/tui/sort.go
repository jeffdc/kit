package tui

import (
	"sort"
	"strings"

	"forage/internal/model"
)

type sortMode int

const (
	sortTitle  sortMode = iota
	sortDate            // newest first
	sortStatus          // lifecycle order
	sortAuthor
)

var sortModes = []sortMode{sortTitle, sortDate, sortStatus, sortAuthor}

var sortLabels = map[sortMode]string{
	sortTitle:  "title",
	sortDate:   "date",
	sortStatus: "status",
	sortAuthor: "author",
}

var statusOrder = func() map[string]int {
	m := make(map[string]int, len(model.StatusList))
	for i, s := range model.StatusList {
		m[s] = i
	}
	return m
}()

func sortBooks(books []model.Book, mode sortMode) {
	sort.SliceStable(books, func(i, j int) bool {
		a, b := books[i], books[j]
		switch mode {
		case sortDate:
			return a.DateAdded > b.DateAdded
		case sortStatus:
			return statusOrder[a.Status] < statusOrder[b.Status]
		case sortAuthor:
			return strings.ToLower(a.Author) < strings.ToLower(b.Author)
		default: // sortTitle
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		}
	})
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
