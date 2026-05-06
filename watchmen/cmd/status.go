package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current tracking status",
	RunE: func(cmd *cobra.Command, args []string) error {
		outputJSON, _ := cmd.Flags().GetBool("json")

		entry := store.ActiveEntry()
		if entry == nil {
			if outputJSON {
				fmt.Println("{}")
				return nil
			}
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

		if outputJSON {
			output := map[string]interface{}{
				"running":         entry.IsRunning(),
				"paused":          entry.IsPaused(),
				"status":          status,
				"project_id":      entry.ProjectID,
				"project_name":    projectName,
				"started_at":      entry.StartTime(),
				"elapsed_seconds": int(duration.Seconds()),
				"hours":           duration.Hours(),
				"segments":        len(entry.Segments),
				"note":            entry.Note,
			}
			jsonData, err := json.Marshal(output)
			if err != nil {
				return err
			}
			fmt.Println(string(jsonData))
			return nil
		}

		fmt.Printf("Currently tracking: %s (%s)\n", projectName, status)
		fmt.Printf("  Started: %s\n", formatStartedAt(entry.StartTime(), time.Now()))
		fmt.Printf("  Accumulated: %s (%.2f hours)\n", formatDuration(duration), duration.Hours())
		fmt.Printf("  Segments: %d\n", len(entry.Segments))
		if entry.Note != "" {
			fmt.Printf("  Note: %s\n", entry.Note)
		}
		return nil
	},
}

func init() {
	statusCmd.Flags().Bool("json", false, "Output status as JSON")
}

func formatStartedAt(startedAt, now time.Time) string {
	if startedAt.IsZero() {
		return ""
	}

	nowInStartLoc := now.In(startedAt.Location())
	if sameCalendarDate(startedAt, nowInStartLoc) {
		return startedAt.Format("3:04 PM")
	}

	return startedAt.Format("Jan 2, 2006 3:04 PM")
}

func sameCalendarDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
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
