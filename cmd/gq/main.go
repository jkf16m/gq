package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Println("gq version 0.1.0")
		case "help":
			fmt.Println("Usage: gq <command>")
			fmt.Println("Commands:")
			fmt.Println("  help     Show this help message")
			fmt.Println("  version  Show version")
		default:
			fmt.Printf("Unknown command: %s\n", os.Args[1])
			fmt.Println("Run 'gq help' for usage")
		}
	} else {
		fmt.Println("Hello from gq!")
	}
}
