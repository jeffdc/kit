package cmd

import (
	"encoding/json"
	"os"

	"forage/internal/model"

	"github.com/spf13/cobra"
)

type primeBook struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Author string   `json:"author"`
	Status string   `json:"status"`
	Tags   []string `json:"tags,omitempty"`
	Rating int      `json:"rating,omitempty"`
}

type primeOutput struct {
	Books  []primeBook    `json:"books"`
	Counts map[string]int `json:"counts"`
}

var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Token-efficient JSON snapshot for LLM context",
	Long: `Token-efficient JSON snapshot of your library for LLM context windows.
Lists non-dropped books with minimal fields, plus status counts.

Use "forage guide" for a full command/field reference.

Example:
  forage prime

Output: {"books": [{"id": "...", "title": "...", ...}], "counts": {"reading": 1, ...}}`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		books, err := store.ListBooks(nil)
		if err != nil {
			return err
		}

		out := primeOutput{
			Books:  []primeBook{},
			Counts: make(map[string]int),
		}

		for _, b := range books {
			out.Counts[b.Status]++

			if model.IsTerminal(b.Status) {
				continue
			}

			out.Books = append(out.Books, primeBook{
				ID:     b.ID,
				Title:  b.Title,
				Author: b.Author,
				Status: b.Status,
				Tags:   b.Tags,
				Rating: b.Rating,
			})
		}

		return json.NewEncoder(os.Stdout).Encode(out)
	},
}

func init() {
	rootCmd.AddCommand(primeCmd)
}
