package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a matter by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		md, _ := cmd.Flags().GetBool("md")
		if md {
			raw, err := store.ReadMatterRaw(args[0])
			if err != nil {
				return err
			}
			fmt.Print(raw)
			return nil
		}

		m, err := store.GetMatter(args[0])
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(m)
	},
}

func init() {
	showCmd.Flags().Bool("md", false, "output as raw markdown instead of JSON")
	rootCmd.AddCommand(showCmd)
}
