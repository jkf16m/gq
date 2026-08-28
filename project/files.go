// Package project discovers text files in a working directory for use as
// project context.
package project

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const binaryProbeSize = 8192

// File is a text file discovered beneath a project root.
type File struct {
	Path    string
	Content string
}

// IsBinaryFile reports whether path appears to contain binary data. It checks
// a bounded prefix, which avoids loading an entire large file just to decide
// whether it should be skipped. NUL bytes and invalid UTF-8 are treated as
// binary; an empty file is text.
func IsBinaryFile(path string) (bool, error) {
	return isBinaryFile(path)
}

func isBinaryFile(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	probe := make([]byte, binaryProbeSize)
	n, err := file.Read(probe)
	if err != nil && n == 0 && err != io.EOF {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	probe = probe[:n]
	return bytes.IndexByte(probe, 0) >= 0 || !utf8.Valid(probe), nil
}

// LoadFiles returns non-binary, non-ignored regular files directly inside
// root. It deliberately does not descend into subdirectories: gq sessions are
// scoped to the directory from which gq was invoked. Paths in File are
// relative to root, using slash separators.
func LoadFiles(root string) ([]File, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	ignores, err := loadIgnores(root)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read project directory: %w", err)
	}
	type discoveredFile struct {
		file    File
		modTime time.Time
	}
	var discovered []discoveredFile
	for _, entry := range entries {
		// Directories are intentionally not traversed. This also keeps .git
		// out of context without relying on a particular ignore file.
		if entry.IsDir() || !entry.Type().IsRegular() {
			continue
		}
		rel := filepath.ToSlash(entry.Name())
		if ignores.match(rel) {
			continue
		}
		fullPath := filepath.Join(root, entry.Name())
		binary, err := IsBinaryFile(fullPath)
		if err != nil {
			return nil, err
		}
		if binary {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", fullPath, err)
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", fullPath, err)
		}
		discovered = append(discovered, discoveredFile{
			file:    File{Path: rel, Content: string(content)},
			modTime: info.ModTime(),
		})
	}
	// Context is ordered chronologically so newer files appear later, nearest
	// the model's most recent context window.
	sort.SliceStable(discovered, func(i, j int) bool {
		if discovered[i].modTime.Equal(discovered[j].modTime) {
			return discovered[i].file.Path < discovered[j].file.Path
		}
		return discovered[i].modTime.Before(discovered[j].modTime)
	})
	files := make([]File, 0, len(discovered))
	for _, item := range discovered {
		files = append(files, item.file)
	}
	return files, nil
}

// Context renders files exactly as they are presented to the model. Discovery
// metadata, including modification times, is intentionally omitted.
func Context(files []File) string {
	sections := make([]string, 0, len(files))
	for _, file := range files {
		sections = append(sections, fmt.Sprintf("--- %s ---\n%s", file.Path, strings.TrimRight(file.Content, "\n")))
	}
	return strings.Join(sections, "\n\n")
}

// ContextCharacters returns the number of characters in Context(files).
func ContextCharacters(files []File) int {
	return len(Context(files))
}

type ignorePattern struct {
	regex   *regexp.Regexp
	negated bool
}

type ignoreRules []ignorePattern

func loadIgnores(root string) (ignoreRules, error) {
	paths := []string{filepath.Join(root, ".gitignore"), filepath.Join(root, ".gq", ".gitignore")}
	var rules ignoreRules
	for _, ignorePath := range paths {
		file, err := os.Open(ignorePath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", ignorePath, err)
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			negated := strings.HasPrefix(line, "!")
			if negated {
				line = line[1:]
			}
			line = strings.TrimPrefix(line, "/")
			directoryOnly := strings.HasSuffix(line, "/")
			line = strings.TrimSuffix(line, "/")
			if line == "" {
				continue
			}
			pattern := globRegex(line, strings.Contains(line, "/"))
			if directoryOnly {
				pattern += `(?:/.*)?`
			}
			rules = append(rules, ignorePattern{regex: regexp.MustCompile("^" + pattern + "$"), negated: negated})
		}
		if err := scanner.Err(); err != nil {
			file.Close()
			return nil, fmt.Errorf("read %s: %w", ignorePath, err)
		}
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("close %s: %w", ignorePath, err)
		}
	}
	return rules, nil
}

func (rules ignoreRules) match(name string) bool {
	ignored := false
	for _, rule := range rules {
		if rule.regex.MatchString(name) {
			ignored = !rule.negated
		}
	}
	return ignored
}

// globRegex converts the useful .gitignore glob subset to a path-aware regex.
func globRegex(pattern string, rooted bool) string {
	var b strings.Builder
	if !rooted {
		b.WriteString(`(?:.*/)?`)
	}
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				b.WriteString(`.*`)
			} else {
				b.WriteString(`[^/]*`)
			}
		case '?':
			b.WriteString(`[^/]`)
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	return b.String()
}
