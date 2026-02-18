package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"mull/internal/storage"
	"mull/internal/tui"
)

func main() {
	store, err := storage.New(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(tui.NewApp(store), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
