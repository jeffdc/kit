package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show full details of a book",
	Args:  cobra.ExactArgs(1),
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
