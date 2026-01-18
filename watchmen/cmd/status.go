package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current tracking status",
	RunE: func(cmd *cobra.Command, args []string) error {
		entry := store.ActiveEntry()
		if entry == nil {
			fmt.Println("No active time entry")
			return nil
		}

		project, _ := store.GetProject(entry.ProjectID)
		projectName := entry.ProjectID
		if project != nil {
			projectName = project.Name
		}

		duration := entry.Duration()
		status := "running"
		if entry.IsPaused() {
			status = "paused"
		}

		fmt.Printf("Currently tracking: %s (%s)\n", projectName, status)
		fmt.Printf("  Started: %s\n", entry.StartTime().Format("3:04 PM"))
		fmt.Printf("  Accumulated: %s (%.2f hours)\n", formatDuration(duration), duration.Hours())
		fmt.Printf("  Segments: %d\n", len(entry.Segments))
		if entry.Note != "" {
			fmt.Printf("  Note: %s\n", entry.Note)
		}
		return nil
	},
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
