//go:build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run build.go <command>")
		fmt.Println("Commands: build, clean")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		os.MkdirAll("bin", 0755)
		cmd := exec.Command("go", "build", "-o", "bin/gq", ".")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Build failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Built: gq")
	case "clean":
		os.Remove(filepath.Join(".", "gq"))
		fmt.Println("Cleaned")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
