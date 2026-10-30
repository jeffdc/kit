package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"watchmen/internal/model"
)

var summaryCmd = &cobra.Command{
	Use:   "summary [project]",
	Short: "Show billing summary per project",
	Long: `Show total billed, paid, and outstanding amounts per project.

Examples:
  watchmen summary        # All projects
  watchmen summary iowa   # Single project`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projects := store.ListProjects()
		if len(projects) == 0 {
			fmt.Println("No projects found")
			return nil
		}

		var filtered []model.Project
		if len(args) > 0 {
			p, err := store.GetProject(args[0])
			if err != nil {
				return fmt.Errorf("project %q not found", args[0])
			}
			filtered = append(filtered, *p)
		} else {
			filtered = projects
		}

		fmt.Printf("%-12s %10s %10s %10s\n", "PROJECT", "BILLED", "PAID", "OUTSTANDING")
		fmt.Println("--------------------------------------------")

		var grandBilled, grandPaid, grandOutstanding float64
		for _, p := range filtered {
			invoices := store.ListInvoices(p.ID, "")
			var billed, paid, outstanding float64
			for _, inv := range invoices {
				billed += inv.Amount
				if inv.Status == model.InvoiceStatusPaid {
					paid += inv.Amount
				} else {
					outstanding += inv.Amount
				}
			}

			fmt.Printf("%-12s %10.2f %10.2f %10.2f\n", p.Name, billed, paid, outstanding)

			grandBilled += billed
			grandPaid += paid
			grandOutstanding += outstanding
		}

		fmt.Println("--------------------------------------------")
		if len(args) == 0 {
			fmt.Printf("%-12s %10.2f %10.2f %10.2f\n", "Total", grandBilled, grandPaid, grandOutstanding)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(summaryCmd)
}
