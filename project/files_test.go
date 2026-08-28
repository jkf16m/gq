package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsBinaryFile(t *testing.T) {
	dir := t.TempDir()
	textPath := filepath.Join(dir, "text.txt")
	binaryPath := filepath.Join(dir, "binary.dat")
	invalidUTF8Path := filepath.Join(dir, "invalid.dat")

	if err := os.WriteFile(textPath, []byte("hello\nworld\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("header\x00payload"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalidUTF8Path, []byte{0xff, 0xfe, 0xfd}, 0600); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		path string
		want bool
	}{
		{"text", textPath, false},
		{"nul byte", binaryPath, true},
		{"invalid UTF-8", invalidUTF8Path, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := IsBinaryFile(test.path)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("IsBinaryFile() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestLoadFilesOrdersOldestFirst(t *testing.T) {
	root := t.TempDir()
	files := []struct {
		name string
		when int64
	}{
		{"new.txt", 300},
		{"old.txt", 100},
		{"middle.txt", 200},
	}
	for _, file := range files {
		path := filepath.Join(root, file.name)
		if err := os.WriteFile(path, []byte(file.name), 0600); err != nil {
			t.Fatal(err)
		}
		when := time.Unix(file.when, 0)
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
	}
	got, err := LoadFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Path != "old.txt" || got[1].Path != "middle.txt" || got[2].Path != "new.txt" {
		t.Fatalf("files ordered as %v, want oldest to newest", got)
	}
}

func TestLoadFilesSkipsBinaryAndGitignoredFiles(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		".gitignore":   "*.log\nbuild/\nsecret.txt\n",
		"README.md":    "read me",
		"notes.log":    "do not load",
		"secret.txt":   "do not load",
		"build/output": "do not load",
		"data.bin":     "binary\x00data",
		"main.go":      "package main",
		"src/main.go":  "nested file should not load",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	files, err := LoadFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, file := range files {
		got = append(got, file.Path)
	}
	joined := strings.Join(got, ",")
	for _, want := range []string{"README.md", ".gitignore", "main.go"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("files = %v, missing %q", got, want)
		}
	}
	for _, unwanted := range []string{"notes.log", "secret.txt", "build/output", "data.bin", "src/main.go"} {
		if strings.Contains(joined, unwanted) {
			t.Fatalf("files = %v, unexpectedly included %q", got, unwanted)
		}
	}
}
