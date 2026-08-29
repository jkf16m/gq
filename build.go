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
		fmt.Println("Commands:")
		fmt.Println("  install - Install gq to $GOPATH/bin/")
		fmt.Println("  build   - Build gq binary to bin/")
		fmt.Println("  clean   - Remove bin/ directory")
		os.Exit(1)
	}

	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	binDir := filepath.Join(root, "bin")

	switch os.Args[1] {
	case "install":
		fmt.Println("Installing gq...")
		cmd := exec.Command("go", "install", ".")
		cmd.Dir = filepath.Join(root, "gq")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error installing gq: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Installed to $GOPATH/bin/gq")

	case "build":
		if err := os.MkdirAll(binDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating bin directory: %v\n", err)
			os.Exit(1)
		}
		buildGq(root, binDir)
		fmt.Println("\nBuild complete. Binary in bin/")

	case "clean":
		if err := os.RemoveAll(binDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing bin directory: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Cleaned bin/")

	case "test":
		fmt.Println("Running integration tests...")
		cmd := exec.Command("go", "test", "-v", "-count=1")
		cmd.Dir = filepath.Join(root, "gq")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running tests: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Tests complete.")

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func buildGq(root, binDir string) {
	fmt.Println("Building gq...")
	cmd := exec.Command("go", "build", "-o", filepath.Join(binDir, "gq"), ".")
	cmd.Dir = filepath.Join(root, "gq")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error building gq: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  -> bin/gq")
}
