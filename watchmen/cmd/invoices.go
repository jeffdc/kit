package cmd

import (
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"
	"watchmen/internal/model"
)

var invoicesCmd = &cobra.Command{
	Use:     "invoices",
	Short:   "List invoices",
	Aliases: []string{"inv"},
	Long: `List all invoices with optional filters.

Examples:
  watchmen invoices                    # List all invoices
  watchmen invoices --outstanding      # List unpaid invoices
  watchmen invoices --project iowa     # List invoices for a project
  watchmen invoices --paid             # List paid invoices`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectFilter, _ := cmd.Flags().GetString("project")
		outstanding, _ := cmd.Flags().GetBool("outstanding")
		paid, _ := cmd.Flags().GetBool("paid")

		var statusFilter model.InvoiceStatus
		if outstanding {
			statusFilter = model.InvoiceStatusPending
		} else if paid {
			statusFilter = model.InvoiceStatusPaid
		}

		invoices := store.ListInvoices(projectFilter, statusFilter)
		if len(invoices) == 0 {
			fmt.Println("No invoices found")
			return nil
		}

		fmt.Printf("%-20s %-12s %10s %10s  %s\n", "INVOICE", "PROJECT", "AMOUNT", "STATUS", "AGE/PAID")
		fmt.Println("-------------------------------------------------------------------------------")

		var totalOutstanding float64
		for _, inv := range invoices {
			projectName := inv.ProjectName
			if len(projectName) > 12 {
				projectName = projectName[:9] + "..."
			}

			status := string(inv.Status)
			ageOrPaid := ""

			if inv.Status == model.InvoiceStatusPending {
				age := time.Since(inv.CreatedAt)
				days := int(math.Round(age.Hours() / 24))
				if days == 0 {
					ageOrPaid = "today"
				} else if days == 1 {
					ageOrPaid = "1 day"
				} else {
					ageOrPaid = fmt.Sprintf("%d days", days)
				}
				totalOutstanding += inv.Amount
			} else if inv.PaidAt != nil {
				ageOrPaid = inv.PaidAt.Format("Jan 2, 2006")
			}

			fmt.Printf("%-20s %-12s %10.2f %10s  %s\n",
				inv.ID,
				projectName,
				inv.Amount,
				status,
				ageOrPaid)
		}

		fmt.Println("-------------------------------------------------------------------------------")
		if outstanding || statusFilter == "" {
			fmt.Printf("Outstanding: $%.2f\n", totalOutstanding)
		}

		return nil
	},
}

var invoicesPaidCmd = &cobra.Command{
	Use:   "paid <invoice-id>",
	Short: "Mark an invoice as paid",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inv, err := store.MarkInvoicePaid(args[0])
		if err != nil {
			return fmt.Errorf("invoice %q not found", args[0])
		}
		fmt.Printf("Marked %s as paid ($%.2f)\n", inv.ID, inv.Amount)
		return nil
	},
}

var invoicesShowCmd = &cobra.Command{
	Use:   "show <invoice-id>",
	Short: "Show invoice details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inv, err := store.GetInvoice(args[0])
		if err != nil {
			return fmt.Errorf("invoice %q not found", args[0])
		}

		fmt.Printf("Invoice:     %s\n", inv.ID)
		fmt.Printf("Project:     %s\n", inv.ProjectName)
		fmt.Printf("Period:      %s - %s\n",
			inv.PeriodStart.Format("Jan 2, 2006"),
			inv.PeriodEnd.Format("Jan 2, 2006"))
		fmt.Printf("Hours:       %.2f\n", inv.Hours)
		fmt.Printf("Rate:        $%.2f/hour\n", inv.Rate)
		fmt.Printf("Amount:      $%.2f\n", inv.Amount)
		fmt.Printf("Status:      %s\n", inv.Status)
		fmt.Printf("Created:     %s\n", inv.CreatedAt.Format("Jan 2, 2006"))
		if inv.PaidAt != nil {
			fmt.Printf("Paid:        %s\n", inv.PaidAt.Format("Jan 2, 2006"))
		}
		if inv.Description != "" {
			fmt.Printf("Description: %s\n", inv.Description)
		}

		return nil
	},
}

var invoicesDeleteCmd = &cobra.Command{
	Use:   "delete <invoice-id>",
	Short: "Delete an invoice record",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inv, err := store.GetInvoice(args[0])
		if err != nil {
			return fmt.Errorf("invoice %q not found", args[0])
		}
		if err := store.DeleteInvoice(args[0]); err != nil {
			return err
		}
		fmt.Printf("Deleted invoice %s ($%.2f)\n", inv.ID, inv.Amount)
		return nil
	},
}

func init() {
	invoicesCmd.Flags().StringP("project", "p", "", "Filter by project name or ID")
	invoicesCmd.Flags().BoolP("outstanding", "o", false, "Show only unpaid invoices")
	invoicesCmd.Flags().Bool("paid", false, "Show only paid invoices")

	invoicesCmd.AddCommand(invoicesPaidCmd)
	invoicesCmd.AddCommand(invoicesShowCmd)
	invoicesCmd.AddCommand(invoicesDeleteCmd)
}
