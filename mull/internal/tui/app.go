package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"mull/internal/storage"
)

type App struct {
	store *storage.Store
}

func NewApp(store *storage.Store) App {
	return App{store: store}
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
	}
	return a, nil
}

func (a App) View() string {
	return "mull-tui - press q to quit\n"
}
