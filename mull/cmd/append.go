package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"mull/internal/model"

	"github.com/spf13/cobra"
)

var replaceBody bool

var appendCmd = &cobra.Command{
	Use:   "append <id> [text | -]",
	Short: "Append text to a matter's body",
	Long: `Append text to a matter's body. Text can be provided as a positional
argument, or piped via stdin using "-" as the text argument.

Use --replace to overwrite the body instead of appending.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		var text string
		if len(args) == 2 && args[1] != "-" {
			text = args[1]
		} else {
			// Read from stdin (either no text arg, or "-")
			info, err := os.Stdin.Stat()
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeCharDevice != 0 {
				return fmt.Errorf("no text argument and stdin is a terminal; pipe input or pass text as an argument")
			}
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			text = string(data)
		}

		var m *model.Matter
		var err error
		if replaceBody {
			m, err = store.ReplaceBody(id, text)
		} else {
			m, err = store.AppendBody(id, text)
		}
		if err != nil {
			return err
		}

		return json.NewEncoder(os.Stdout).Encode(confirmAppend(m))
	},
}

func init() {
	appendCmd.Flags().BoolVar(&replaceBody, "replace", false, "Replace the body instead of appending")
	rootCmd.AddCommand(appendCmd)
}
