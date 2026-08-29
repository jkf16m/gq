package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gq",
	Short: "gq is a simple CLI tool",
	Long:  `gq is a simple CLI tool built with Cobra.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("gq")
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}