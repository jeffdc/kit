package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"forage/internal/storage"

	"github.com/spf13/cobra"
)

var store *storage.Store

var rootCmd = &cobra.Command{
	Use:   "forage",
	Short: "A personal book library manager",
	Long:  "A CLI tool for managing your personal book library. All output is JSON.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		store, err = storage.New(storage.DefaultRoot())
		return err
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintln(os.Stderr, string(b))
		os.Exit(1)
	}
}
