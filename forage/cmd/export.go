package cmd

import (
	"encoding/json"
	"os"

	"forage/internal/export"
	"forage/internal/model"
	"forage/internal/pwa"

	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Generate a self-contained HTML file of your library",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")
		isPWA, _ := cmd.Flags().GetBool("pwa")

		books, err := store.ListBooks(nil)
		if err != nil {
			return err
		}

		// Exclude dropped
		var included []model.Book
		for _, b := range books {
			if !model.IsTerminal(b.Status) {
				included = append(included, b)
			}
		}

		booksellers, err := store.LoadBooksellers()
		if err != nil {
			return err
		}

		if isPWA {
			if !cmd.Flags().Changed("output") {
				output = "forage-pwa"
			}

			if err := pwa.Generate(included, booksellers, output); err != nil {
				return err
			}

			return json.NewEncoder(os.Stdout).Encode(map[string]any{
				"exported": "./" + output,
				"books":    len(included),
				"pwa":      true,
			})
		}

		f, err := os.Create(output)
		if err != nil {
			return err
		}
		defer f.Close()

		if err := export.Generate(included, booksellers, f); err != nil {
			return err
		}

		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"exported": output,
			"books":    len(included),
		})
	},
}

func init() {
	exportCmd.Flags().StringP("output", "o", "forage-library.html", "Output file path")
	exportCmd.Flags().Bool("pwa", false, "Generate a PWA directory instead of a single HTML file")
	rootCmd.AddCommand(exportCmd)
}
