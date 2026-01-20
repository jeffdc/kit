package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"watchmen/internal/invoice"
)

var reportCmd = &cobra.Command{
	Use:   "report <project>",
	Short: "Generate a stakeholder report for a project",
	Long: `Generate a markdown report summarizing work completed on a project.

Examples:
  watchmen report myproject --week              # This week's work
  watchmen report myproject --month             # This month's work
  watchmen report myproject --since 2026-01-01  # Since a specific date
  watchmen report myproject --invoice INV-foo-123 -o report.md`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sinceStr, _ := cmd.Flags().GetString("since")
		untilStr, _ := cmd.Flags().GetString("until")
		thisWeek, _ := cmd.Flags().GetBool("week")
		thisMonth, _ := cmd.Flags().GetBool("month")
		invoiceRef, _ := cmd.Flags().GetString("invoice")
		outputFile, _ := cmd.Flags().GetString("output")

		project, err := store.GetProject(args[0])
		if err != nil {
			return fmt.Errorf("project %q not found", args[0])
		}

		var from, to time.Time
		now := time.Now()

		if thisWeek {
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			from = now.AddDate(0, 0, -weekday+1)
			from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.Local)
			to = now
		} else if thisMonth {
			from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
			to = now
		} else if sinceStr != "" {
			from, err = time.Parse("2006-01-02", sinceStr)
			if err != nil {
				return fmt.Errorf("invalid date format for --since, use YYYY-MM-DD")
			}
			if untilStr != "" {
				to, err = time.Parse("2006-01-02", untilStr)
				if err != nil {
					return fmt.Errorf("invalid date format for --until, use YYYY-MM-DD")
				}
			} else {
				to = now
			}
		} else {
			// Default to this month
			from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
			to = now
		}

		fromPtr := &from
		toPtr := &to
		entries := store.ListEntries(project.ID, fromPtr, toPtr)

		if len(entries) == 0 {
			return fmt.Errorf("no entries found for %s in the specified period", project.Name)
		}

		// Calculate total hours
		var totalHours float64
		for _, e := range entries {
			totalHours += e.Duration().Hours()
		}

		data := &invoice.ReportData{
			ProjectName: project.Name,
			From:        from,
			To:          to,
			TotalHours:  totalHours,
			InvoiceRef:  invoiceRef,
			Entries:     entries,
		}

		var out *os.File
		if outputFile != "" {
			out, err = os.Create(outputFile)
			if err != nil {
				return fmt.Errorf("failed to create output file: %v", err)
			}
			defer out.Close()
		} else {
			out = os.Stdout
		}

		if err := invoice.GenerateReport(out, data); err != nil {
			return fmt.Errorf("failed to generate report: %v", err)
		}

		if outputFile != "" {
			fmt.Fprintf(os.Stderr, "Report generated: %s\n", outputFile)
		}

		return nil
	},
}

func init() {
	reportCmd.Flags().String("since", "", "Start date (YYYY-MM-DD)")
	reportCmd.Flags().String("until", "", "End date (YYYY-MM-DD)")
	reportCmd.Flags().BoolP("week", "w", false, "This week")
	reportCmd.Flags().BoolP("month", "m", false, "This month")
	reportCmd.Flags().StringP("invoice", "i", "", "Invoice number to reference in header")
	reportCmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")
}
