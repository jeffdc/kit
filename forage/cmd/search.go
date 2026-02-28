package cmd

import (
	"encoding/json"
	"os"

	"forage/internal/model"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search books by title, author, tags, or notes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		books, err := store.SearchBooks(args[0])
		if err != nil {
			return err
		}

		results := stripBodies(books)
		if results == nil {
			results = []model.Book{}
		}

		return json.NewEncoder(os.Stdout).Encode(results)
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
