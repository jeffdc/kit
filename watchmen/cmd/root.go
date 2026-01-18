package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"watchmen/internal/storage"
)

var store *storage.Store
var dataPath string

var rootCmd = &cobra.Command{
	Use:   "watchmen",
	Short: "Track work hours and generate invoices",
	Long:  `A CLI tool to track time spent on projects with notes, and generate invoices.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		path := dataPath
		if path == "" {
			var err error
			path, err = storage.DefaultPath()
			if err != nil {
				return err
			}
		}
		var err error
		store, err = storage.New(path)
		return err
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dataPath, "data", "", "Path to data file (default: ~/.watchmen/data.json)")

	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(invoiceCmd)
	rootCmd.AddCommand(configCmd)
}
