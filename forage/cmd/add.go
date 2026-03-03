package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"forage/internal/openlibrary"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a book to your library",
	Long: `Add a book to your library. Status defaults to "wishlist".

Valid statuses: wishlist, reading, paused, read, dropped.
Rating: 1-5 (0 or omitted = unrated).

Examples:
  forage add "Dune" --author "Frank Herbert"
  forage add "Neuromancer" --author "William Gibson" --tag sci-fi --tag classic
  forage add "Babel" --author "R.F. Kuang" --status reading --rating 4

Output: {"id": "a3f2", "title": "...", "status": "wishlist"}`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]

		author, _ := cmd.Flags().GetString("author")
		if author == "" {
			return fmt.Errorf("--author is required")
		}

		lookup, _ := cmd.Flags().GetBool("lookup")
		var olResult *openlibrary.SearchResult
		if lookup {
			var err error
			olResult, err = openlibrary.Search(title, author)
			if err != nil {
				return fmt.Errorf("open library lookup failed: %w", err)
			}
			if olResult != nil {
				title = olResult.Title
				author = olResult.Author
			}
		}

		meta := make(map[string]string)

		if s, _ := cmd.Flags().GetString("status"); s != "" {
			meta["status"] = s
		}
		if tags, _ := cmd.Flags().GetStringSlice("tag"); len(tags) > 0 {
			meta["tags"] = joinTags(tags)
		}
		if r, _ := cmd.Flags().GetInt("rating"); r > 0 {
			meta["rating"] = fmt.Sprintf("%d", r)
		}
		if b, _ := cmd.Flags().GetString("body"); b != "" {
			meta["body"] = b
		}

		book, err := store.CreateBook(title, author, meta)
		if err != nil {
			return err
		}

		if lookup {
			return json.NewEncoder(os.Stdout).Encode(confirmWithLookup(book, olResult))
		}
		return json.NewEncoder(os.Stdout).Encode(confirm(book))
	},
}

func joinTags(tags []string) string {
	result := ""
	for i, t := range tags {
		if i > 0 {
			result += ","
		}
		result += t
	}
	return result
}

func init() {
	addCmd.Flags().String("author", "", "Book author (required)")
	addCmd.Flags().String("status", "", "Status (default: wishlist)")
	addCmd.Flags().StringSlice("tag", nil, "Tags (repeatable)")
	addCmd.Flags().Int("rating", 0, "Rating (1-5)")
	addCmd.Flags().String("body", "", "Notes about the book")
	addCmd.Flags().Bool("lookup", false, "Look up book via Open Library")
	rootCmd.AddCommand(addCmd)
}
