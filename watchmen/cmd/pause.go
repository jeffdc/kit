package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause the current time entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		entry, err := store.PauseEntry()
		if err != nil {
			return err
		}

		project, _ := store.GetProject(entry.ProjectID)
		projectName := entry.ProjectID
		if project != nil {
			projectName = project.Name
		}

		duration := entry.Duration()
		fmt.Printf("Paused tracking on %s\n", projectName)
		fmt.Printf("  Accumulated: %s (%.2f hours)\n", formatDuration(duration), duration.Hours())
		fmt.Printf("  Segments: %d\n", len(entry.Segments))
		if entry.Note != "" {
			fmt.Printf("  Note: %s\n", entry.Note)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pauseCmd)
}
