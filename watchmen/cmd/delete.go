package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [index]",
	Short: "Delete a completed time entry",
	Long: `Delete a completed time entry by index (1=most recent).

Examples:
  watchmen delete 1        # Delete most recent entry (interactive confirmation)
  watchmen delete 1 --yes  # Skip confirmation
  watchmen delete 2        # Delete second most recent entry`,
	RunE: func(cmd *cobra.Command, args []string) error {
		yes, _ := cmd.Flags().GetBool("yes")

		if len(args) == 0 {
			return fmt.Errorf("entry index required (e.g. watchmen delete 1)")
		}

		index, err := strconv.Atoi(args[0])
		if err != nil || index <= 0 {
			fmt.Fprintf(os.Stderr, "invalid entry index: %s\n", args[0])
			os.Exit(2)
		}

		// Get completed entries in reverse chronological order
		entries := store.ListEntries("", nil, nil)
		var completed []int
		for i := len(entries) - 1; i >= 0; i-- {
			if entries[i].Completed {
				completed = append(completed, i)
			}
		}

		if len(completed) == 0 {
			fmt.Fprintln(os.Stderr, "no completed entries to delete")
			os.Exit(1)
		}

		if index > len(completed) {
			fmt.Fprintf(os.Stderr, "entry #%d not found (only %d completed entries)\n", index, len(completed))
			os.Exit(2)
		}

		entryIdx := completed[index-1]
		entry := entries[entryIdx]

		project, _ := store.GetProject(entry.ProjectID)
		projectName := entry.ProjectID
		if project != nil {
			projectName = project.Name
		}

		// Show what will be deleted
		fmt.Printf("Entry #%d:\n", index)
		fmt.Printf("  Project:  %s\n", projectName)
		fmt.Printf("  Date:     %s\n", entry.StartTime().Format("2006-01-02 15:04"))
		fmt.Printf("  Duration: %.2fh\n", entry.Duration().Hours())
		if entry.Note != "" {
			fmt.Printf("  Note:     %q\n", entry.Note)
		}

		if !yes {
			fmt.Print("\nDelete this entry? [y/N] ")
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "Y" {
				fmt.Println("Cancelled")
				return nil
			}
		}

		if err := store.DeleteEntry(entry.ID); err != nil {
			return err
		}

		fmt.Println("Deleted.")
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
}
