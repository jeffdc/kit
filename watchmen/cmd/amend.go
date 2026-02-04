package cmd

import (
	"bufio"
	"fmt"
	"io"
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

Non-interactive mode: When --note or --clear is provided, skips all prompts
and defaults to the most recent entry (index 1) if no index is specified.

Examples:
  watchmen amend                # Interactive mode
  watchmen amend 1              # Edit most recent entry interactively
  watchmen amend -n "note"      # Update most recent entry (non-interactive)
  watchmen amend --last -n "x"  # Explicit: update most recent entry
  watchmen amend 2 -n "note"    # Update second most recent entry
  watchmen amend --clear        # Clear note from most recent entry
  echo "note" | watchmen amend  # Read note from stdin`,
	RunE: func(cmd *cobra.Command, args []string) error {
		note, _ := cmd.Flags().GetString("note")
		clear, _ := cmd.Flags().GetBool("clear")
		last, _ := cmd.Flags().GetBool("last")

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
			fmt.Fprintln(os.Stderr, "no completed entries to amend")
			os.Exit(1)
		}

		// Determine if we're in non-interactive mode
		// Non-interactive when: --note, --clear, --last, or stdin is not a TTY
		stdinIsTerminal := isTerminal(os.Stdin)
		nonInteractive := note != "" || clear || last || !stdinIsTerminal

		// Determine the index
		var index int
		if len(args) > 0 {
			var err error
			index, err = strconv.Atoi(args[0])
			if err != nil || index <= 0 {
				fmt.Fprintf(os.Stderr, "invalid entry index: %s\n", args[0])
				os.Exit(2)
			}
		} else if last || nonInteractive {
			// Default to most recent entry in non-interactive mode
			index = 1
		} else {
			// Interactive mode - no args, no flags
			return runAmendInteractive(entries, completed)
		}

		if index > len(completed) {
			fmt.Fprintf(os.Stderr, "entry #%d not found (only %d completed entries)\n", index, len(completed))
			os.Exit(2)
		}

		// Get the entry details
		entryIdx := completed[index-1]
		entry := entries[entryIdx]
		project, _ := store.GetProject(entry.ProjectID)
		projectName := entry.ProjectID
		if project != nil {
			projectName = project.Name
		}

		// Determine the new note
		var newNote string
		if clear {
			newNote = ""
		} else if note != "" {
			newNote = note
		} else if !stdinIsTerminal {
			// Read note from stdin
			input, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading from stdin: %w", err)
			}
			newNote = strings.TrimSpace(string(input))
			if newNote == "" {
				fmt.Fprintln(os.Stderr, "empty note from stdin")
				os.Exit(1)
			}
		} else {
			// Interactive prompt for note
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
		_, err := store.AmendEntry(index, newNote)
		if err != nil {
			return err
		}

		// Output: minimal in non-interactive mode, verbose otherwise
		if nonInteractive && stdinIsTerminal {
			// Non-interactive with terminal - single line confirmation
			if newNote != "" {
				fmt.Printf("amended entry #%d: %q\n", index, newNote)
			} else {
				fmt.Printf("amended entry #%d: (cleared)\n", index)
			}
		} else if !stdinIsTerminal {
			// Piped input - even more minimal
			fmt.Printf("amended entry #%d\n", index)
		} else {
			// Interactive mode - verbose output
			fmt.Printf("\nUpdated entry #%d:\n", index)
			fmt.Printf("  Project: %s\n", projectName)
			fmt.Printf("  Duration: %.2fh\n", entry.Duration().Hours())
			if newNote != "" {
				fmt.Printf("  Note: %q\n", newNote)
			} else {
				fmt.Printf("  Note: (cleared)\n")
			}
		}

		return nil
	},
}

// isTerminal returns true if the file is a terminal
func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
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
	amendCmd.Flags().BoolP("last", "1", false, "Amend the most recent entry (index 1)")
}
