package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the current time entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		note, _ := cmd.Flags().GetString("note")

		entry, err := store.StopEntry(note)
		if err != nil {
			return err
		}

		project, _ := store.GetProject(entry.ProjectID)
		projectName := entry.ProjectID
		if project != nil {
			projectName = project.Name
		}

		duration := entry.Duration()
		hours := duration.Hours()

		fmt.Printf("Stopped tracking time on %s\n", projectName)
		fmt.Printf("  Duration: %s (%.2f hours)\n", formatDuration(duration), hours)
		if entry.Note != "" {
			fmt.Printf("  Note: %s\n", entry.Note)
		}
		return nil
	},
}

func init() {
	stopCmd.Flags().StringP("note", "n", "", "Add note when stopping")
}
