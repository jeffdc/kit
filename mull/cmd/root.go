package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"mull/internal/storage"
)

var store *storage.Store

var rootCmd = &cobra.Command{
	Use:   "mull",
	Short: "Track ideas and features for solo projects",
	Long:  `A CLI tool for tracking ideas and features ("matters") for solo projects. All output is JSON.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		store, err = storage.New(".")
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
