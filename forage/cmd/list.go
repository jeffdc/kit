package cmd

import (
	"encoding/json"
	"os"

	"forage/internal/model"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List books in your library",
	Long: `List books in your library. Dropped books are excluded by default.

Examples:
  forage list
  forage list --status reading
  forage list --tag sci-fi
  forage list --author "Frank Herbert"
  forage list --all                       # include dropped books

Output: JSON array of books (body field stripped).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filters := make(map[string]string)

		status, _ := cmd.Flags().GetString("status")
		tag, _ := cmd.Flags().GetString("tag")
		author, _ := cmd.Flags().GetString("author")
		all, _ := cmd.Flags().GetBool("all")

		if status != "" {
			filters["status"] = status
		}
		if tag != "" {
			filters["tag"] = tag
		}
		if author != "" {
			filters["author"] = author
		}

		books, err := store.ListBooks(filters)
		if err != nil {
			return err
		}

		// Exclude terminal statuses by default
		if !all && status == "" {
			var filtered []model.Book
			for _, b := range books {
				if !model.IsTerminal(b.Status) {
					filtered = append(filtered, b)
				}
			}
			books = filtered
		}

		books = stripBodies(books)

		// Return [] not null
		if books == nil {
			books = []model.Book{}
		}

		return json.NewEncoder(os.Stdout).Encode(books)
	},
}

func init() {
	listCmd.Flags().String("status", "", "Filter by status")
	listCmd.Flags().String("tag", "", "Filter by tag")
	listCmd.Flags().String("author", "", "Filter by author")
	listCmd.Flags().Bool("all", false, "Include dropped books")
	rootCmd.AddCommand(listCmd)
}
