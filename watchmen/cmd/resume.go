package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume a paused time entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		entry, err := store.ResumeEntry()
		if err != nil {
			return err
		}

		project, _ := store.GetProject(entry.ProjectID)
		projectName := entry.ProjectID
		if project != nil {
			projectName = project.Name
		}

		duration := entry.Duration()
		fmt.Printf("Resumed tracking on %s\n", projectName)
		fmt.Printf("  Resumed at: %s\n", time.Now().Format("3:04 PM"))
		fmt.Printf("  Previous time: %s (%.2f hours)\n", formatDuration(duration), duration.Hours())
		fmt.Printf("  Segments: %d\n", len(entry.Segments))
		if entry.Note != "" {
			fmt.Printf("  Note: %s\n", entry.Note)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}
