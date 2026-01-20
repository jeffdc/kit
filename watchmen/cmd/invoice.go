package cmd

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"watchmen/internal/invoice"
	"watchmen/internal/model"
)

var invoiceCmd = &cobra.Command{
	Use:   "invoice <project>",
	Short: "Generate an invoice for a project",
	Long: `Generate an invoice for time entries on a project.

By default, generates a condensed invoice with a single line item (requires --desc).

Examples:
  watchmen invoice myproject -d "Software development"  # Condensed invoice (default)
  watchmen invoice myproject --detailed                 # Detailed invoice with all entries
  watchmen invoice myproject --week -d "Weekly dev"     # This week's entries
  watchmen invoice myproject --since 2024-01-01 --until 2024-01-31 -d "Jan work"
  watchmen invoice myproject --pdf invoice.pdf -d "Dev" # Generate PDF
  watchmen invoice myproject --one-shot -d "Dev"        # Auto-generate invoice + report`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sinceStr, _ := cmd.Flags().GetString("since")
		untilStr, _ := cmd.Flags().GetString("until")
		thisWeek, _ := cmd.Flags().GetBool("week")
		thisMonth, _ := cmd.Flags().GetBool("month")
		pdfFile, _ := cmd.Flags().GetString("pdf")
		invoiceNum, _ := cmd.Flags().GetString("number")
		poNumber, _ := cmd.Flags().GetString("po")
		markdown, _ := cmd.Flags().GetBool("markdown")
		outputFile, _ := cmd.Flags().GetString("output")
		condensed, _ := cmd.Flags().GetBool("condensed")
		detailed, _ := cmd.Flags().GetBool("detailed")
		condensedDesc, _ := cmd.Flags().GetString("desc")
		noSave, _ := cmd.Flags().GetBool("no-save")
		oneShot, _ := cmd.Flags().GetBool("one-shot")

		// --detailed overrides --condensed
		if detailed {
			condensed = false
		}

		if condensed && condensedDesc == "" {
			return fmt.Errorf("--desc is required for condensed invoices")
		}

		project, err := store.GetProject(args[0])
		if err != nil {
			return fmt.Errorf("project %q not found", args[0])
		}

		var from, to time.Time
		now := time.Now()

		if oneShot {
			// One-shot mode: auto-calculate date range
			from, err = getOneShotStartDate(project.ID)
			if err != nil {
				return err
			}

			// End date is 2 weeks from start, or today if sooner
			twoWeeksOut := from.AddDate(0, 0, 14)
			today := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.Local)
			if today.Before(twoWeeksOut) {
				to = today
			} else {
				to = twoWeeksOut
			}
		} else if thisWeek {
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

		if invoiceNum == "" {
			invoiceNum = fmt.Sprintf("INV-%s-%s", project.Name[:min(3, len(project.Name))], now.Format("20060102"))
		}

		// Get contact info
		settings := store.GetSettings()

		// Use project PO as default, override with --po flag
		po := project.PurchaseOrder
		if poNumber != "" {
			po = poNumber
		}

		data := &invoice.InvoiceData{
			InvoiceNumber:        invoiceNum,
			PurchaseOrder:        po,
			Date:                 now,
			Project:              *project,
			Entries:              entries,
			From:                 from,
			To:                   to,
			FromContact:          settings.UserContact,
			BillToContact:        project.BillingContact,
			Condensed:            condensed,
			CondensedDescription: condensedDesc,
		}

		if oneShot {
			// One-shot mode: generate PDF, markdown invoice, and report
			pdfFileName := invoiceNum + ".pdf"
			mdFileName := invoiceNum + ".md"
			reportFileName := fmt.Sprintf("report-%s-%s.md", project.Name, now.Format("20060102"))

			// Generate PDF invoice
			if err := invoice.GeneratePDF(pdfFileName, data); err != nil {
				return fmt.Errorf("failed to generate PDF: %v", err)
			}
			fmt.Printf("Invoice PDF: %s\n", pdfFileName)

			// Generate markdown invoice
			mdFile, err := os.Create(mdFileName)
			if err != nil {
				return fmt.Errorf("failed to create markdown file: %v", err)
			}
			invoice.GenerateMarkdown(mdFile, data)
			mdFile.Close()
			fmt.Printf("Invoice MD:  %s\n", mdFileName)

			// Generate stakeholder report
			var totalHours float64
			for _, e := range entries {
				totalHours += e.Duration().Hours()
			}

			reportData := &invoice.ReportData{
				ProjectName: project.Name,
				From:        from,
				To:          to,
				TotalHours:  totalHours,
				InvoiceRef:  invoiceNum,
				Entries:     entries,
			}

			reportFile, err := os.Create(reportFileName)
			if err != nil {
				return fmt.Errorf("failed to create report file: %v", err)
			}
			invoice.GenerateReport(reportFile, reportData)
			reportFile.Close()
			fmt.Printf("Report:      %s\n", reportFileName)

			fmt.Printf("\nTotal hours: %.2f\n", data.TotalHours())
			fmt.Printf("Total due:   $%.2f\n", data.TotalAmount())
		} else if pdfFile != "" {
			if err := invoice.GeneratePDF(pdfFile, data); err != nil {
				return fmt.Errorf("failed to generate PDF: %v", err)
			}
			fmt.Printf("Invoice generated: %s\n", pdfFile)
			fmt.Printf("  Total hours: %.2f\n", data.TotalHours())
			fmt.Printf("  Total due:   $%.2f\n", data.TotalAmount())
		} else if markdown {
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
			invoice.GenerateMarkdown(out, data)
			if outputFile != "" {
				fmt.Printf("Invoice generated: %s\n", outputFile)
			}
		} else {
			invoice.GenerateText(os.Stdout, data)
		}

		// Save invoice record unless --no-save is set
		if !noSave {
			invRecord := &model.Invoice{
				ID:          invoiceNum,
				ProjectID:   project.ID,
				ProjectName: project.Name,
				PeriodStart: from,
				PeriodEnd:   to,
				Hours:       data.TotalHours(),
				Rate:        project.HourlyRate,
				Amount:      data.TotalAmount(),
				Description: condensedDesc,
				Condensed:   condensed,
			}
			if err := store.SaveInvoice(invRecord); err != nil {
				return fmt.Errorf("failed to save invoice record: %v", err)
			}
		}

		return nil
	},
}

// getOneShotStartDate returns the start date for one-shot mode.
// If there's a previous invoice, returns the day after its end date.
// Otherwise, returns the date of the first entry for the project.
func getOneShotStartDate(projectID string) (time.Time, error) {
	// Get all invoices for this project
	invoices := store.ListInvoices(projectID, "")

	if len(invoices) > 0 {
		// Sort by PeriodEnd descending to find the most recent
		sort.Slice(invoices, func(i, j int) bool {
			return invoices[i].PeriodEnd.After(invoices[j].PeriodEnd)
		})

		lastInvoice := invoices[0]
		// Start from the day after the last invoice's end date
		nextDay := lastInvoice.PeriodEnd.AddDate(0, 0, 1)
		return time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 0, 0, 0, 0, time.Local), nil
	}

	// No previous invoice - find first entry date
	entries := store.ListEntries(projectID, nil, nil)
	if len(entries) == 0 {
		return time.Time{}, fmt.Errorf("no entries found for project")
	}

	// Find the earliest entry
	earliest := entries[0].StartTime()
	for _, e := range entries[1:] {
		if e.StartTime().Before(earliest) {
			earliest = e.StartTime()
		}
	}

	return time.Date(earliest.Year(), earliest.Month(), earliest.Day(), 0, 0, 0, 0, time.Local), nil
}

func init() {
	invoiceCmd.Flags().String("since", "", "Start date (YYYY-MM-DD)")
	invoiceCmd.Flags().String("until", "", "End date (YYYY-MM-DD)")
	invoiceCmd.Flags().BoolP("week", "w", false, "This week")
	invoiceCmd.Flags().BoolP("month", "m", false, "This month")
	invoiceCmd.Flags().String("pdf", "", "Output PDF file")
	invoiceCmd.Flags().StringP("number", "n", "", "Invoice number")
	invoiceCmd.Flags().String("po", "", "Purchase order number")
	invoiceCmd.Flags().Bool("markdown", false, "Output as markdown")
	invoiceCmd.Flags().StringP("output", "o", "", "Output file (for markdown)")
	invoiceCmd.Flags().BoolP("condensed", "c", true, "Generate condensed invoice with single line item (default)")
	invoiceCmd.Flags().Bool("detailed", false, "Generate detailed invoice with all time entries")
	invoiceCmd.Flags().StringP("desc", "d", "", "Description for condensed invoice line item (required for condensed)")
	invoiceCmd.Flags().Bool("no-save", false, "Don't save invoice record (preview only)")
	invoiceCmd.Flags().Bool("one-shot", false, "Generate invoice + report, auto-calculating dates from last invoice")
}
