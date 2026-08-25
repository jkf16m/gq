package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Configure or show the current AI model",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Model configuration:")
		fmt.Println("  No model configured.")
		fmt.Println("  Use 'gq model set <model>' to configure.")
	},
}
