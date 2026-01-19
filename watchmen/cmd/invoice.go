package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"watchmen/internal/invoice"
)

var invoiceCmd = &cobra.Command{
	Use:   "invoice <project>",
	Short: "Generate an invoice for a project",
	Long: `Generate an invoice for time entries on a project.

Examples:
  watchmen invoice myproject --month              # This month's entries (text)
  watchmen invoice myproject --week --markdown    # This week's entries (markdown)
  watchmen invoice myproject --since 2024-01-01 --until 2024-01-31
  watchmen invoice myproject --pdf invoice.pdf    # Generate PDF
  watchmen invoice myproject --markdown -o inv.md # Save markdown to file
  watchmen invoice myproject --condensed -d "Software development services"`,
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
		condensedDesc, _ := cmd.Flags().GetString("desc")

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

		if pdfFile != "" {
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

		return nil
	},
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
	invoiceCmd.Flags().BoolP("condensed", "c", false, "Generate condensed invoice with single line item")
	invoiceCmd.Flags().StringP("desc", "d", "", "Description for condensed invoice line item")
}
