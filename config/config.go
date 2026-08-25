package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const configName = "config.json"

type Result struct {
	Values  map[string]interface{}
	Files   []string
	Context string
}

// Load discovers and merges configuration for the directory containing cwd.
// Defaults are applied first; home and project configuration override them.
func Load(cwd, home, applicationDir string) (Result, error) {
	result := Result{Values: make(map[string]interface{})}

	// The application default is the lowest-precedence layer.
	for _, path := range []string{
		filepath.Join(applicationDir, configName),
		filepath.Join("/usr/lib/gq", configName),
		filepath.Join("/usr/share/gq", configName),
	} {
		if err := mergeFile(&result, path); err != nil {
			return Result{}, err
		}
		if contains(result.Files, path) {
			break
		}
	}

	// The user's home configuration overrides application defaults.
	if home != "" {
		if err := mergeFile(&result, filepath.Join(home, ".gq", configName)); err != nil {
			return Result{}, err
		}
	}

	// Discover from the execution directory outward. Reverse the discovered
	// paths so parent configuration is merged before child configuration.
	paths, err := discover(cwd)
	if err != nil {
		return Result{}, err
	}
	for i := len(paths) - 1; i >= 0; i-- {
		if err := mergeFile(&result, paths[i]); err != nil {
			return Result{}, err
		}
	}
	if err := loadContextFiles(&result, paths); err != nil {
		return Result{}, err
	}

	delete(result.Values, "keepWalking")
	delete(result.Values, "contextFiles")
	return result, nil
}

func discover(cwd string) ([]string, error) {
	cwd, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	var paths []string
	for {
		path := filepath.Join(cwd, ".gq", configName)
		data, err := os.ReadFile(path)
		if err == nil {
			var values map[string]interface{}
			if err := json.Unmarshal(data, &values); err != nil {
				return nil, fmt.Errorf("parse %s: %w", path, err)
			}
			paths = append(paths, path)
			keepWalking, defined := values["keepWalking"].(bool)
			if !defined || !keepWalking {
				break
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return paths, nil
}

func loadContextFiles(result *Result, paths []string) error {
	var sections []string
	for i := len(paths) - 1; i >= 0; i-- {
		data, err := os.ReadFile(paths[i])
		if err != nil {
			return fmt.Errorf("read %s: %w", paths[i], err)
		}
		var values map[string]interface{}
		if err := json.Unmarshal(data, &values); err != nil {
			return fmt.Errorf("parse %s: %w", paths[i], err)
		}
		names, _ := values["contextFiles"].([]interface{})
		base := filepath.Dir(filepath.Dir(paths[i]))
		for _, rawName := range names {
			name, ok := rawName.(string)
			if !ok || name == "" {
				continue
			}
			path := filepath.Join(base, name)
			content, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return fmt.Errorf("read context file %s: %w", path, err)
			}
			sections = append(sections, fmt.Sprintf("--- %s ---\n%s", path, strings.TrimSpace(string(content))))
		}
	}
	result.Context = strings.Join(sections, "\n\n")
	return nil
}

func mergeFile(result *Result, path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var values map[string]interface{}
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	merge(result.Values, values)
	result.Files = append(result.Files, path)
	return nil
}

func merge(dst, src map[string]interface{}) {
	for key, value := range src {
		srcMap, ok := value.(map[string]interface{})
		if !ok {
			dst[key] = value
			continue
		}
		dstMap, ok := dst[key].(map[string]interface{})
		if !ok {
			dstMap = make(map[string]interface{})
			dst[key] = dstMap
		}
		merge(dstMap, srcMap)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
