package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

type dataReloadMsg struct{}

func watchFiles(mullDir string) tea.Cmd {
	return func() tea.Msg {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil
		}
		_ = watcher.Add(mullDir + "/matters")
		_ = watcher.Add(mullDir)

		// Debounce: wait for 100ms of quiet before sending reload
		var timer *time.Timer
		for {
			select {
			case _, ok := <-watcher.Events:
				if !ok {
					return nil
				}
				if timer != nil {
					timer.Stop()
				}
				timer = time.NewTimer(100 * time.Millisecond)
				<-timer.C
				return dataReloadMsg{}
			case _, ok := <-watcher.Errors:
				if !ok {
					return nil
				}
			}
		}
	}
}
