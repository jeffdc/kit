package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <id> <key> <value>",
	Short: "Update a field on a book",
	Long: `Update a field on a book.

Valid keys: title, author, status, rating, tags, date_read.
Valid statuses: wishlist, reading, paused, read, dropped.
For tags, use comma-separated values.

Examples:
  forage set a3f2 status reading
  forage set a3f2 rating 4
  forage set a3f2 tags "sci-fi,classic"
  forage set a3f2 date_read 2025-01-15

Output: {"id": "a3f2", "title": "...", "status": "..."}`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		book, err := store.UpdateBook(args[0], args[1], args[2])
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(confirm(book))
	},
}

func init() {
	rootCmd.AddCommand(setCmd)
}
