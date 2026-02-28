package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

type dataReloadMsg struct{}

func watchFiles(booksDir string) tea.Cmd {
	return func() tea.Msg {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			time.Sleep(5 * time.Second)
			return dataReloadMsg{}
		}
		defer watcher.Close()

		_ = watcher.Add(booksDir)

		var timer *time.Timer
		for {
			select {
			case _, ok := <-watcher.Events:
				if !ok {
					return dataReloadMsg{}
				}
				if timer != nil {
					timer.Stop()
				}
				timer = time.NewTimer(100 * time.Millisecond)
				<-timer.C
				return dataReloadMsg{}
			case _, ok := <-watcher.Errors:
				if !ok {
					return dataReloadMsg{}
				}
				return dataReloadMsg{}
			}
		}
	}
}
