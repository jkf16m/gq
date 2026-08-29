package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// configFile represents a .gq/config.json file with its path and parsed content.
type configFile struct {
	path string
	cfg  Config
}

// walkFromDir collects all .gq/config.json paths starting from dir,
// walking outward until the filesystem root is reached.
// It returns the paths in order from nearest to farthest.
func walkFromDir(dir string) []string {
	var paths []string

	// Walk outward from the given directory
	current := dir
	for {
		configPath := filepath.Join(current, ".gq", "config.json")

		// Check if the config file exists
		if _, err := os.Stat(configPath); err == nil {
			paths = append(paths, configPath)
		}

		// Move to parent directory
		parent := filepath.Dir(current)
		if parent == current {
			break // Reached root
		}
		current = parent
	}

	return paths
}

// loadConfigFile reads and parses a .gq/config.json file.
// It returns the parsed Config or nil if the file cannot be read or parsed.
func loadConfigFile(path string) *Config {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}

	return &cfg
}