package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log <project> [\"start | end | note\"]",
	Short: "Log a completed time entry",
	Long: `Log a time entry that has already been completed.

Condensed format (recommended):
  watchmen log myproject "9am | 11am | worked on stuff"
  watchmen log myproject "1400 | 1730 | afternoon meeting"
  watchmen log myproject "9:30am | 12pm | morning work"

Flag-based format:
  watchmen log myproject --duration 2h --note "Fixed bugs"
  watchmen log myproject --duration 1h30m --date 2024-01-15
  watchmen log myproject --start "9:00AM" --end "11:30AM" --note "Meeting"

Time formats supported: 9am, 9AM, 9:30pm, 14:30, 1400, 2330`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		note, _ := cmd.Flags().GetString("note")
		durationStr, _ := cmd.Flags().GetString("duration")
		dateStr, _ := cmd.Flags().GetString("date")
		startStr, _ := cmd.Flags().GetString("start")
		endStr, _ := cmd.Flags().GetString("end")

		project, err := store.GetProject(args[0])
		if err != nil {
			return fmt.Errorf("project %q not found", args[0])
		}

		// Check for condensed format as second argument
		if len(args) == 2 {
			parts := strings.Split(args[1], "|")
			if len(parts) < 2 {
				return fmt.Errorf("condensed format requires at least start and end time separated by |")
			}
			startStr = strings.TrimSpace(parts[0])
			endStr = strings.TrimSpace(parts[1])
			if len(parts) >= 3 {
				note = strings.TrimSpace(parts[2])
			}
		}

		var startTime, endTime time.Time
		baseDate := time.Now()

		// Parse date if provided
		if dateStr != "" {
			parsed, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				return fmt.Errorf("invalid date format, use YYYY-MM-DD")
			}
			baseDate = parsed
		}

		if durationStr != "" {
			// Duration-based entry
			duration, err := time.ParseDuration(durationStr)
			if err != nil {
				return fmt.Errorf("invalid duration: %v", err)
			}
			// If logging for today, end at current time; otherwise end at 5 PM
			if dateStr == "" {
				endTime = time.Now()
			} else {
				endTime = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 17, 0, 0, 0, time.Local)
			}
			startTime = endTime.Add(-duration)
		} else if startStr != "" && endStr != "" {
			// Start/end time based entry
			startTime, err = parseTime(startStr, baseDate)
			if err != nil {
				return fmt.Errorf("invalid start time: %v", err)
			}
			endTime, err = parseTime(endStr, baseDate)
			if err != nil {
				return fmt.Errorf("invalid end time: %v", err)
			}
		} else {
			return fmt.Errorf("provide either --duration or both --start and --end, or use condensed format")
		}

		entry, err := store.LogEntry(project.ID, note, startTime, endTime)
		if err != nil {
			return err
		}

		duration := entry.Duration()
		fmt.Printf("Logged %.2f hours on %s\n", duration.Hours(), project.Name)
		fmt.Printf("  Date: %s\n", startTime.Format("Jan 2, 2006"))
		fmt.Printf("  Time: %s - %s\n", startTime.Format("3:04 PM"), endTime.Format("3:04 PM"))
		if note != "" {
			fmt.Printf("  Note: %s\n", note)
		}
		return nil
	},
}

func parseTime(s string, baseDate time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)

	// Try 24-hour format without colon (1700, 2330, 900)
	if hour, min, ok := parseMilitaryTime(s); ok {
		return time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(),
			hour, min, 0, 0, time.Local), nil
	}

	// Normalize: uppercase AM/PM for consistent parsing
	upper := strings.ToUpper(s)

	// Try various formats
	formats := []string{
		"3:04PM",
		"3:04 PM",
		"3:04pm",
		"3:04 pm",
		"15:04",
		"3PM",
		"3 PM",
		"3pm",
		"3 pm",
	}

	for _, f := range formats {
		// Try with original string
		if t, err := time.Parse(f, s); err == nil {
			return time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(),
				t.Hour(), t.Minute(), 0, 0, time.Local), nil
		}
		// Try with uppercased string (handles am/pm -> AM/PM)
		if t, err := time.Parse(strings.ToUpper(f), upper); err == nil {
			return time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(),
				t.Hour(), t.Minute(), 0, 0, time.Local), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}

// parseMilitaryTime handles formats like 1700, 2330, 900, 0900
func parseMilitaryTime(s string) (hour, min int, ok bool) {
	// Must be all digits
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, 0, false
		}
	}

	switch len(s) {
	case 3: // 900 -> 9:00
		hour, _ = strconv.Atoi(s[:1])
		min, _ = strconv.Atoi(s[1:])
	case 4: // 1700, 0900
		hour, _ = strconv.Atoi(s[:2])
		min, _ = strconv.Atoi(s[2:])
	default:
		return 0, 0, false
	}

	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return 0, 0, false
	}
	return hour, min, true
}

func init() {
	logCmd.Flags().StringP("note", "n", "", "Note for this entry")
	logCmd.Flags().StringP("duration", "d", "", "Duration (e.g., 2h, 1h30m)")
	logCmd.Flags().String("date", "", "Date for the entry (YYYY-MM-DD, default: today)")
	logCmd.Flags().String("start", "", "Start time (e.g., 9:00AM, 14:30)")
	logCmd.Flags().String("end", "", "End time (e.g., 5:00PM, 17:00)")
}
