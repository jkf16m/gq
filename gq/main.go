package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jkf16m/gq/llm"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gq",
	Short: "gq is a simple CLI tool",
	Long:  `gq is a simple CLI tool built with Cobra.`,
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		apiKey, err := llm.GetAPIKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Check if there's piped input
		info, _ := os.Stdin.Stat()
		isPiped := (info.Mode() & os.ModeCharDevice) == 0

		var prompt string
		var tty bool
		if isPiped {
			prompt = readStdin()
			tty = false
		} else if len(args) > 0 {
			prompt = strings.Join(args, " ")
			tty = false
		} else {
			prompt = readTTY()
			tty = true
		}

		if prompt == "" {
			fmt.Fprintln(os.Stderr, "No prompt provided")
			os.Exit(1)
		}

		response, pendingCommands := AgentLoop(apiKey, "@preset/mimo", prompt, tty)

		// Show pending commands (non-TTY modes)
		if len(pendingCommands) > 0 {
			fmt.Println("\n--- Pending Commands ---")
			for i, c := range pendingCommands {
				fmt.Printf("%d. %s\n", i+1, c.Command)
			}
			fmt.Println("--- Use 'gq c' to review and execute ---")
		}

		// Show response
		if response != "" {
			fmt.Println(response)
		}
	},
}

var resumeCmd = &cobra.Command{
	Use:   "c",
	Short: "Resume the last session and execute pending commands",
	Long:  `Review and execute pending commands from the last session.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Resuming last session...")
		// TODO: Load session and show pending commands
		apiKey, err := llm.GetAPIKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		runAgentLoop(apiKey)
	},
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runAgentLoop(apiKey string) {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			break
		}

		response, pendingCommands := AgentLoop(apiKey, "@preset/mimo", input, true)

		if len(pendingCommands) > 0 {
			fmt.Println("\n--- Pending Commands ---")
			for i, c := range pendingCommands {
				fmt.Printf("%d. %s\n", i+1, c.Command)
			}
			fmt.Println("--- Use 'gq c' to review and execute ---")
		}

		if response != "" {
			fmt.Println(response)
		}
	}
}

func readStdin() string {
	scanner := bufio.NewScanner(os.Stdin)
	var input strings.Builder
	for scanner.Scan() {
		input.WriteString(scanner.Text())
		input.WriteString("\n")
	}
	return strings.TrimSpace(input.String())
}

func readTTY() string {
	fmt.Print("> ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}
