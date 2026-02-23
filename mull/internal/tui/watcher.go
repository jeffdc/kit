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
			// Can't watch — trigger a one-time reload and retry later
			time.Sleep(5 * time.Second)
			return dataReloadMsg{}
		}
		defer watcher.Close()

		_ = watcher.Add(mullDir + "/matters")
		_ = watcher.Add(mullDir)

		// Debounce: wait for 100ms of quiet before sending reload
		var timer *time.Timer
		for {
			select {
			case _, ok := <-watcher.Events:
				if !ok {
					// Channel closed — watcher died, trigger reload to restart
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
					// Error channel closed — trigger reload to restart watcher
					return dataReloadMsg{}
				}
				// Transient error — trigger reload to restart watcher
				return dataReloadMsg{}
			}
		}
	}
}
