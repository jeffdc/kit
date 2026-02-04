package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"watchmen/internal/model"
)

var amendCmd = &cobra.Command{
	Use:   "amend [index]",
	Short: "Amend the note on a completed time entry",
	Long: `Amend the note on a completed time entry.

With no arguments, displays an interactive list of recent entries.
With an index (1=most recent), directly edits that entry.

Examples:
  watchmen amend              # Interactive mode
  watchmen amend 1            # Edit most recent entry
  watchmen amend 2 -n "note"  # Update second entry directly
  watchmen amend 1 --clear    # Clear note from most recent entry`,
	RunE: func(cmd *cobra.Command, args []string) error {
		note, _ := cmd.Flags().GetString("note")
		clear, _ := cmd.Flags().GetBool("clear")

		if clear && note != "" {
			return fmt.Errorf("cannot use both --note and --clear")
		}

		// Get all completed entries
		entries := store.ListEntries("", nil, nil)
		var completed []int // indices in reverse order
		for i := len(entries) - 1; i >= 0; i-- {
			if entries[i].Completed {
				completed = append(completed, i)
			}
		}

		if len(completed) == 0 {
			fmt.Println("No completed entries to amend")
			return nil
		}

		// Interactive mode - no args
		if len(args) == 0 {
			return runAmendInteractive(entries, completed)
		}

		// Direct mode - with index
		index, err := strconv.Atoi(args[0])
		if err != nil || index <= 0 {
			return fmt.Errorf("invalid entry index: %s", args[0])
		}

		if index > len(completed) {
			return fmt.Errorf("entry #%d not found (only %d completed entries)", index, len(completed))
		}

		// Get the entry details for display
		entryIdx := completed[index-1]
		entry := entries[entryIdx]
		project, _ := store.GetProject(entry.ProjectID)
		projectName := entry.ProjectID
		if project != nil {
			projectName = project.Name
		}

		// Show entry details
		fmt.Printf("Entry #%d:\n", index)
		fmt.Printf("  Project: %s\n", projectName)
		fmt.Printf("  Date: %s\n", entry.StartTime().Format("2006-01-02 15:04"))
		fmt.Printf("  Duration: %.2fh\n", entry.Duration().Hours())
		if entry.Note != "" {
			fmt.Printf("  Current note: %q\n", entry.Note)
		} else {
			fmt.Printf("  Current note: (none)\n")
		}
		fmt.Println()

		var newNote string
		if clear {
			newNote = ""
		} else if note != "" {
			newNote = note
		} else {
			// Prompt for new note
			fmt.Print("Enter new note (or press Enter to keep current): ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)

			if input == "" {
				fmt.Println("Note unchanged")
				return nil
			}
			newNote = input
		}

		// Update the entry
		updated, err := store.AmendEntry(index, newNote)
		if err != nil {
			return err
		}

		// Show confirmation
		fmt.Printf("\nUpdated entry #%d:\n", index)
		fmt.Printf("  Project: %s\n", projectName)
		fmt.Printf("  Duration: %.2fh\n", updated.Duration().Hours())
		if newNote != "" {
			fmt.Printf("  Note: %q\n", newNote)
		} else {
			fmt.Printf("  Note: (cleared)\n")
		}

		return nil
	},
}

func runAmendInteractive(entries []model.Entry, completed []int) error {
	fmt.Println("\nRecent time entries:")

	// Show up to 20 entries
	displayCount := len(completed)
	if displayCount > 20 {
		displayCount = 20
	}

	for i := 0; i < displayCount; i++ {
		entryIdx := completed[i]
		entry := entries[entryIdx]

		project, _ := store.GetProject(entry.ProjectID)
		projectName := entry.ProjectID
		if project != nil {
			projectName = project.Name
		}
		if len(projectName) > 15 {
			projectName = projectName[:12] + "..."
		}

		dateStr := entry.StartTime().Format("2006-01-02 15:04")
		duration := entry.Duration().Hours()

		note := entry.Note
		if note == "" {
			note = "(no note)"
		}
		if len(note) > 40 {
			note = note[:37] + "..."
		}

		fmt.Printf("  %2d. %-16s %-15s %4.1fh  %s\n", i+1, dateStr, projectName, duration, note)
	}

	fmt.Print("\nEnter entry number to amend (or 'q' to quit): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "q" || input == "" {
		return nil
	}

	index, err := strconv.Atoi(input)
	if err != nil || index <= 0 || index > displayCount {
		return fmt.Errorf("invalid selection")
	}

	// Get the entry
	entryIdx := completed[index-1]
	entry := entries[entryIdx]

	project, _ := store.GetProject(entry.ProjectID)
	projectName := entry.ProjectID
	if project != nil {
		projectName = project.Name
	}

	// Show current details
	fmt.Printf("\nEntry #%d:\n", index)
	fmt.Printf("  Project: %s\n", projectName)
	fmt.Printf("  Date: %s\n", entry.StartTime().Format("2006-01-02 15:04"))
	fmt.Printf("  Duration: %.2fh\n", entry.Duration().Hours())
	if entry.Note != "" {
		fmt.Printf("  Current note: %q\n", entry.Note)
	} else {
		fmt.Printf("  Current note: (none)\n")
	}

	// Prompt for new note
	fmt.Print("\nEnter new note: ")
	newNote, _ := reader.ReadString('\n')
	newNote = strings.TrimSpace(newNote)

	if newNote == "" {
		fmt.Println("Note unchanged")
		return nil
	}

	// Update the entry
	updated, err := store.AmendEntry(index, newNote)
	if err != nil {
		return err
	}

	// Show confirmation
	fmt.Printf("\nUpdated entry #%d:\n", index)
	fmt.Printf("  Project: %s\n", projectName)
	fmt.Printf("  Duration: %.2fh\n", updated.Duration().Hours())
	fmt.Printf("  Note: %q\n", newNote)

	return nil
}

func init() {
	amendCmd.Flags().StringP("note", "n", "", "New note text (skips prompt)")
	amendCmd.Flags().Bool("clear", false, "Clear the note (set to empty)")
}
