package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Esc     key.Binding
	Quit    key.Binding
	Help    key.Binding
	Tab1    key.Binding
	Tab2    key.Binding
	Open    key.Binding
	Closed  key.Binding
	All     key.Binding
	Status  key.Binding
	Epic    key.Binding
	Search  key.Binding
	Prompt  key.Binding
	Docket  key.Binding
	Refresh key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "detail")),
		Esc:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Tab1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "matters")),
		Tab2:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "docket")),
		Open:    key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open only")),
		Closed:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "closed only")),
		All:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all")),
		Status:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "cycle status")),
		Epic:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "filter epic")),
		Search:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Prompt:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "copy prompt")),
		Docket:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "toggle docket")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}
