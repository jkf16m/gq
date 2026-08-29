package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// confirm prompts the user to accept or reject a command.
// Returns true if user presses y twice within 2 seconds, false otherwise.
func confirm(command string) bool {
	fmt.Printf("$ %s\n", command)

	scanner := bufio.NewScanner(os.Stdin)

	// First y
	fmt.Print("y/n: ")
	if !scanner.Scan() {
		return false
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "n" || input == "N" {
		return false
	}
	if input != "y" && input != "Y" {
		return false
	}

	// Second y within 2 seconds
	fmt.Print("y/n: ")

	done := make(chan bool, 1)
	go func() {
		if scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			done <- (input == "y" || input == "Y")
		} else {
			done <- false
		}
	}()

	select {
	case result := <-done:
		return result
	case <-time.After(2 * time.Second):
		fmt.Println("\ntimeout")
		return false
	}
}

// executeCommand runs a bash command and returns the output.
func executeCommand(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
