package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show full details of a book",
	Long: `Show full details of a book by its 4-char hex ID, including body/notes.

Example:
  forage show a3f2

Output: full book JSON with all fields (id, title, author, status, tags, rating, date_added, date_read, body).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		book, err := store.GetBook(args[0])
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(book)
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
