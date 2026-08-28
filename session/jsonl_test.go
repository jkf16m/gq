package session

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveLoadJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "last.jsonl")
	want := []ChatMessage{
		{Role: "system", Content: "be concise"},
		{Role: "user", Content: "show me\nREADME.md"},
		{Role: "assistant", ToolCalls: []ChatCall{{
			ID: "call 1", Type: "function",
			Function: ChatFunction{Name: "cmd", Arguments: `{"command":"cat README.md"}`},
		}}},
		{Role: "tool", ToolCallID: "call 1", Content: "contents\nwith newlines"},
	}

	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded messages differ\n got: %#v\nwant: %#v", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("session permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestLoadReportsLineNumber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.jsonl")
	if err := os.WriteFile(path, []byte("{\"role\":\"user\",\"content\":\"ok\"}\nnot json\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !contains(err.Error(), "line 2") {
		t.Fatalf("Load error = %v, want line number", err)
	}
}

func TestLastPath(t *testing.T) {
	got := LastPath("/home/example")
	want := filepath.Join("/home/example", ".gq", "sessions", "last.jsonl")
	if got != want {
		t.Fatalf("LastPath() = %q, want %q", got, want)
	}
}

func contains(value, substring string) bool {
	for i := 0; i+len(substring) <= len(value); i++ {
		if value[i:i+len(substring)] == substring {
			return true
		}
	}
	return false
}
