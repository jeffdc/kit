package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Esc       key.Binding
	Quit      key.Binding
	Help      key.Binding
	Search    key.Binding
	Refresh   key.Binding
	SetStatus key.Binding
	MarkRead  key.Binding
	Drop      key.Binding
	FilterAll key.Binding
	FilterWL  key.Binding
	FilterRd  key.Binding
	CycleFilt key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:        key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:      key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "detail")),
		Esc:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Search:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		SetStatus: key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "cycle status")),
		MarkRead:  key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "mark read")),
		Drop:      key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "drop")),
		FilterAll: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all")),
		FilterWL:  key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "wishlist")),
		FilterRd:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "read")),
		CycleFilt: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "cycle filter")),
	}
}
