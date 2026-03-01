package main

import (
	"fmt"
	"os"

	"forage/internal/storage"
	"forage/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	store, err := storage.New(storage.DefaultRoot())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	p := tea.NewProgram(tui.NewApp(store), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
