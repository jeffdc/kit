package cmd

import (
	"encoding/json"
	"os"

	"forage/internal/pwa"

	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Generate a PWA directory for web deployment",
	Long: `Generate a PWA directory for web deployment.
The PWA syncs data from the server API at runtime.

Examples:
  forage export
  forage export -o my-pwa-dir

Output: {"exported": "forage-pwa", "pwa": true}`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")

		if err := pwa.Generate(output); err != nil {
			return err
		}

		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"exported": "./" + output,
			"pwa":      true,
		})
	},
}

func init() {
	exportCmd.Flags().StringP("output", "o", "forage-pwa", "Output directory path")
	// Keep --pwa flag for backward compatibility (accepted but ignored)
	exportCmd.Flags().Bool("pwa", false, "Ignored (PWA is now the only export mode)")
	_ = exportCmd.Flags().MarkHidden("pwa")
	rootCmd.AddCommand(exportCmd)
}
