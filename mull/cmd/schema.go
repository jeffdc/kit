package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
	"mull/internal/model"
)

type fieldSchema struct {
	Required bool     `json:"required"`
	Type     string   `json:"type"`
	Values   []string `json:"values,omitempty"`
}

type schemaOutput struct {
	Statuses []string               `json:"statuses"`
	Fields   map[string]fieldSchema `json:"fields"`
	Links    []string               `json:"links"`
}

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Show valid fields, statuses, and relationship types",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		statuses := make([]string, 0, len(model.ValidStatuses))
		for s := range model.ValidStatuses {
			statuses = append(statuses, s)
		}

		out := schemaOutput{
			Statuses: []string{"raw", "refined", "planned", "done", "dropped"},
			Fields: map[string]fieldSchema{
				"title":  {Required: true, Type: "string"},
				"status": {Required: true, Type: "enum", Values: []string{"raw", "refined", "planned", "done", "dropped"}},
				"tags":   {Required: false, Type: "string[]"},
				"effort": {Required: false, Type: "string"},
				"epic":   {Required: false, Type: "string"},
				"plan":   {Required: false, Type: "string"},
				"docs":   {Required: false, Type: "string[]"},
			},
			Links: []string{"relates", "blocks", "needs", "parent"},
		}

		return json.NewEncoder(os.Stdout).Encode(out)
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
