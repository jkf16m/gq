package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gq",
	Short: "An agentic CLI tool",
	Long:  `gq is an agentic CLI that helps you with tasks using AI models.`,
	Run: func(cmd *cobra.Command, args []string) {
		// When no subcommand is provided, launch TUI
		// This will be handled by bubbletea later
		cmd.Println("Welcome to gq! (TUI will launch here)")
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Add subcommands here
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(modelCmd)
}
