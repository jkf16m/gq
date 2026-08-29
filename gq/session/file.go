package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// loadSessionFile reads and parses a session JSON file.
func loadSessionFile(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}

	return &sess, nil
}

// listSessionFiles returns all session file paths in the given directory.
func listSessionFiles(sessionDir string) ([]string, error) {
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".json" {
			paths = append(paths, filepath.Join(sessionDir, entry.Name()))
		}
	}

	return paths, nil
}

// saveSessionFile writes a session to a JSON file.
func saveSessionFile(sess *Session, path string) error {
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// newSession creates a new session with the given ID.
func newSession(id string) *Session {
	return &Session{
		ID:       id,
		Created:  time.Now(),
		Updated:  time.Now(),
		Messages: []Message{},
	}
}