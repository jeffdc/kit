package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start <project>",
	Short: "Start tracking time on a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		note, _ := cmd.Flags().GetString("note")

		// Resolve project by name or ID
		project, err := store.GetProject(args[0])
		if err != nil {
			return fmt.Errorf("project %q not found", args[0])
		}

		entry, err := store.StartEntry(project.ID, note)
		if err != nil {
			return err
		}

		fmt.Printf("Started tracking time on %s\n", project.Name)
		fmt.Printf("  Started: %s\n", entry.StartTime().Format("3:04 PM"))
		if note != "" {
			fmt.Printf("  Note: %s\n", note)
		}
		return nil
	},
}

func init() {
	startCmd.Flags().StringP("note", "n", "", "Note for this time entry")
}
