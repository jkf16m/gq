package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompound(t *testing.T) {
	// Create temp directory structure
	tmp := t.TempDir()
	subdir := filepath.Join(tmp, "project")
	os.MkdirAll(subdir, 0755)

	// Create .gq/config.json in subdir
	gqDir := filepath.Join(subdir, ".gq")
	os.MkdirAll(gqDir, 0755)
	cfgContent := `{"keepWalking": true, "contextFiles": ["AGENTS.md", "CLAUDE.md"]}`
	os.WriteFile(filepath.Join(gqDir, "config.json"), []byte(cfgContent), 0644)

	// Load config from subdir
	cfg := Load(subdir)

	// Should have values from .gq/config.json
	if cfg.KeepWalking != true {
		t.Errorf("expected KeepWalking=true, got %v", cfg.KeepWalking)
	}
	if len(cfg.ContextFiles) != 2 {
		t.Errorf("expected 2 context files, got %d", len(cfg.ContextFiles))
	}
}

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.SessionDir != ".gq/sessions" {
		t.Errorf("expected .gq/sessions, got %s", cfg.SessionDir)
	}
	if cfg.KeepWalking != false {
		t.Errorf("expected KeepWalking=false, got %v", cfg.KeepWalking)
	}
}

func TestWalkUp(t *testing.T) {
	// Create nested directory
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "a", "b", "c")
	os.MkdirAll(nested, 0755)

	// Walk from nested should find nothing
	paths := WalkUp(nested)
	if len(paths) != 0 {
		t.Errorf("expected no paths, got %d", len(paths))
	}

	// Create .gq/config.json in parent
	gqDir := filepath.Join(tmp, "a", "b", ".gq")
	os.MkdirAll(gqDir, 0755)
	os.WriteFile(filepath.Join(gqDir, "config.json"), []byte(`{}`), 0644)

	// Walk from nested should find one path
	paths = WalkUp(nested)
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d", len(paths))
	}
}