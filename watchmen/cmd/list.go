package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List time entries",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		projectFilter, _ := cmd.Flags().GetString("project")
		sinceStr, _ := cmd.Flags().GetString("since")
		untilStr, _ := cmd.Flags().GetString("until")
		thisWeek, _ := cmd.Flags().GetBool("week")
		thisMonth, _ := cmd.Flags().GetBool("month")
		showSegments, _ := cmd.Flags().GetBool("segments")

		var from, to *time.Time

		if thisWeek {
			now := time.Now()
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			start := now.AddDate(0, 0, -weekday+1)
			start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.Local)
			from = &start
		} else if thisMonth {
			now := time.Now()
			start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
			from = &start
		} else {
			if sinceStr != "" {
				t, err := time.Parse("2006-01-02", sinceStr)
				if err != nil {
					return fmt.Errorf("invalid date format for --since, use YYYY-MM-DD")
				}
				from = &t
			}
			if untilStr != "" {
				t, err := time.Parse("2006-01-02", untilStr)
				if err != nil {
					return fmt.Errorf("invalid date format for --until, use YYYY-MM-DD")
				}
				t = t.Add(24*time.Hour - time.Second)
				to = &t
			}
		}

		entries := store.ListEntries(projectFilter, from, to)
		if len(entries) == 0 {
			fmt.Println("No entries found")
			return nil
		}

		var totalDuration time.Duration
		fmt.Printf("%-12s %-20s %8s  %s\n", "DATE", "PROJECT", "HOURS", "NOTE")
		fmt.Println("-------------------------------------------------------------------------------")

		for _, e := range entries {
			project, _ := store.GetProject(e.ProjectID)
			projectName := e.ProjectID
			if len(projectName) > 8 {
				projectName = projectName[:8]
			}
			if project != nil {
				projectName = project.Name
				if len(projectName) > 20 {
					projectName = projectName[:17] + "..."
				}
			}

			duration := e.Duration()
			totalDuration += duration

			status := ""
			if e.IsRunning() {
				status = " (running)"
			} else if e.IsPaused() {
				status = " (paused)"
			}

			note := e.Note
			if len(note) > 40 {
				note = note[:37] + "..."
			}

			fmt.Printf("%-12s %-20s %8.2f  %s%s\n",
				e.StartTime().Format("Jan 2"),
				projectName,
				duration.Hours(),
				note,
				status)

			if showSegments && len(e.Segments) > 1 {
				for i, seg := range e.Segments {
					endStr := "running"
					if seg.End != nil {
						endStr = seg.End.Format("3:04 PM")
					}
					fmt.Printf("             segment %d: %s - %s (%.2fh)\n",
						i+1,
						seg.Start.Format("3:04 PM"),
						endStr,
						seg.Duration().Hours())
				}
			}
		}

		fmt.Println("-------------------------------------------------------------------------------")
		fmt.Printf("%-12s %-20s %8.2f\n", "TOTAL", "", totalDuration.Hours())

		return nil
	},
}

func init() {
	listCmd.Flags().StringP("project", "p", "", "Filter by project name or ID")
	listCmd.Flags().String("since", "", "Show entries since date (YYYY-MM-DD)")
	listCmd.Flags().String("until", "", "Show entries until date (YYYY-MM-DD)")
	listCmd.Flags().BoolP("week", "w", false, "Show this week's entries")
	listCmd.Flags().BoolP("month", "m", false, "Show this month's entries")
	listCmd.Flags().BoolP("segments", "s", false, "Show individual time segments")
}
