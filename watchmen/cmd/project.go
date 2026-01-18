package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"watchmen/internal/model"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
}

var projectAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rate, _ := cmd.Flags().GetFloat64("rate")
		desc, _ := cmd.Flags().GetString("description")

		p, err := store.AddProject(args[0], rate, desc)
		if err != nil {
			return err
		}
		fmt.Printf("Created project: %s (ID: %s)\n", p.Name, p.ID)
		if rate > 0 {
			fmt.Printf("  Hourly rate: $%.2f\n", rate)
		}
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		projects := store.ListProjects()
		if len(projects) == 0 {
			fmt.Println("No projects yet. Create one with: watchmen project add <name>")
			return nil
		}
		fmt.Printf("%-16s %-20s %10s  %s\n", "ID", "NAME", "RATE", "DESCRIPTION")
		fmt.Println("-------------------------------------------------------------------------------")
		for _, p := range projects {
			rate := "-"
			if p.HourlyRate > 0 {
				rate = fmt.Sprintf("$%.2f/hr", p.HourlyRate)
			}
			fmt.Printf("%-16s %-20s %10s  %s\n", p.ID, p.Name, rate, p.Description)
		}
		return nil
	},
}

var projectBillingCmd = &cobra.Command{
	Use:   "billing <project>",
	Short: "Set billing contact and PO for a project",
	Long: `Set the billing contact information and purchase order for a project.

Examples:
  watchmen project billing myproject --name "John Doe" --company "Acme Inc"
  watchmen project billing myproject --name "Jane" --email "jane@acme.com" --address "123 Main St"
  watchmen project billing myproject --po "PO-2026-001"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		company, _ := cmd.Flags().GetString("company")
		address, _ := cmd.Flags().GetString("address")
		phone, _ := cmd.Flags().GetString("phone")
		email, _ := cmd.Flags().GetString("email")
		po, _ := cmd.Flags().GetString("po")

		project, err := store.GetProject(args[0])
		if err != nil {
			return fmt.Errorf("project %q not found", args[0])
		}

		if name == "" && company == "" && address == "" && phone == "" && email == "" && po == "" {
			// Show current billing info
			hasBilling := project.BillingContact != nil
			hasPO := project.PurchaseOrder != ""
			if !hasBilling && !hasPO {
				fmt.Printf("No billing info set for %s\n", project.Name)
				return nil
			}
			fmt.Printf("Billing info for %s:\n", project.Name)
			if hasPO {
				fmt.Printf("  PO #: %s\n", project.PurchaseOrder)
			}
			if hasBilling {
				printContactInfo(project.BillingContact)
			}
			return nil
		}

		// Get existing contact or create new
		contact := project.BillingContact
		if contact == nil {
			contact = &model.ContactInfo{}
		}

		if name != "" {
			contact.Name = name
		}
		if company != "" {
			contact.Company = company
		}
		if address != "" {
			contact.Address = address
		}
		if phone != "" {
			contact.Phone = phone
		}
		if email != "" {
			contact.Email = email
		}

		err = store.UpdateProject(args[0], func(p *model.Project) {
			if name != "" || company != "" || address != "" || phone != "" || email != "" {
				p.BillingContact = contact
			}
			if po != "" {
				p.PurchaseOrder = po
			}
		})
		if err != nil {
			return err
		}

		fmt.Printf("Billing info updated for %s:\n", project.Name)
		if po != "" {
			fmt.Printf("  PO #: %s\n", po)
		}
		if name != "" || company != "" || address != "" || phone != "" || email != "" {
			printContactInfo(contact)
		}
		return nil
	},
}

var projectShowCmd = &cobra.Command{
	Use:   "show <project>",
	Short: "Show project details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := store.GetProject(args[0])
		if err != nil {
			return fmt.Errorf("project %q not found", args[0])
		}

		fmt.Printf("Project: %s\n", project.Name)
		fmt.Printf("  ID:   %s\n", project.ID)
		if project.HourlyRate > 0 {
			fmt.Printf("  Rate: $%.2f/hr\n", project.HourlyRate)
		}
		if project.Description != "" {
			fmt.Printf("  Desc: %s\n", project.Description)
		}
		if project.PurchaseOrder != "" {
			fmt.Printf("  PO #: %s\n", project.PurchaseOrder)
		}
		if project.BillingContact != nil {
			fmt.Println("  Billing Contact:")
			if project.BillingContact.Name != "" {
				fmt.Printf("    Name:    %s\n", project.BillingContact.Name)
			}
			if project.BillingContact.Company != "" {
				fmt.Printf("    Company: %s\n", project.BillingContact.Company)
			}
			if project.BillingContact.Address != "" {
				fmt.Printf("    Address: %s\n", project.BillingContact.Address)
			}
			if project.BillingContact.Phone != "" {
				fmt.Printf("    Phone:   %s\n", project.BillingContact.Phone)
			}
			if project.BillingContact.Email != "" {
				fmt.Printf("    Email:   %s\n", project.BillingContact.Email)
			}
		}
		return nil
	},
}

func init() {
	projectAddCmd.Flags().Float64P("rate", "r", 0, "Hourly rate for the project")
	projectAddCmd.Flags().StringP("description", "d", "", "Project description")

	projectBillingCmd.Flags().String("name", "", "Contact name")
	projectBillingCmd.Flags().String("company", "", "Company name")
	projectBillingCmd.Flags().String("address", "", "Address")
	projectBillingCmd.Flags().String("phone", "", "Phone number")
	projectBillingCmd.Flags().String("email", "", "Email address")
	projectBillingCmd.Flags().String("po", "", "Purchase order number")

	projectCmd.AddCommand(projectAddCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectBillingCmd)
	projectCmd.AddCommand(projectShowCmd)
}
