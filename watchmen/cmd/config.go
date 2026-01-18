package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"watchmen/internal/model"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration settings",
	Long:  `Manage watchmen configuration settings like your contact info for invoices.`,
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set your contact info for invoices",
	Long: `Set your contact information that appears in the "From" section of invoices.

Examples:
  watchmen config set --name "John Doe" --email "john@example.com"
  watchmen config set --name "Jane Smith" --company "Smith LLC" --address "123 Main St" --phone "555-1234" --email "jane@smith.com"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		company, _ := cmd.Flags().GetString("company")
		address, _ := cmd.Flags().GetString("address")
		phone, _ := cmd.Flags().GetString("phone")
		email, _ := cmd.Flags().GetString("email")

		if name == "" && company == "" && address == "" && phone == "" && email == "" {
			return fmt.Errorf("provide at least one field to set")
		}

		// Get existing contact info to preserve unset fields
		settings := store.GetSettings()
		contact := settings.UserContact
		if contact == nil {
			contact = &model.ContactInfo{}
		}

		// Update only provided fields
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

		if err := store.SetUserContact(contact); err != nil {
			return err
		}

		fmt.Println("Contact info updated:")
		printContactInfo(contact)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show your contact info",
	RunE: func(cmd *cobra.Command, args []string) error {
		settings := store.GetSettings()
		if settings.UserContact == nil {
			fmt.Println("No contact info configured. Use 'watchmen config set' to add your info.")
			return nil
		}
		fmt.Println("Your contact info:")
		printContactInfo(settings.UserContact)
		return nil
	},
}

func printContactInfo(c *model.ContactInfo) {
	if c.Name != "" {
		fmt.Printf("  Name:    %s\n", c.Name)
	}
	if c.Company != "" {
		fmt.Printf("  Company: %s\n", c.Company)
	}
	if c.Address != "" {
		fmt.Printf("  Address: %s\n", c.Address)
	}
	if c.Phone != "" {
		fmt.Printf("  Phone:   %s\n", c.Phone)
	}
	if c.Email != "" {
		fmt.Printf("  Email:   %s\n", c.Email)
	}
}

func init() {
	configSetCmd.Flags().String("name", "", "Your name")
	configSetCmd.Flags().String("company", "", "Your company name")
	configSetCmd.Flags().String("address", "", "Your address")
	configSetCmd.Flags().String("phone", "", "Your phone number")
	configSetCmd.Flags().String("email", "", "Your email address")

	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowCmd)
}
