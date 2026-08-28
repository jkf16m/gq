package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ChatMessage is a message in the format accepted by the chat completions API.
// One ChatMessage is encoded as one JSON object (and therefore one JSONL line).
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"`
	ToolCalls  []ChatCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ChatCall is an assistant tool call.
type ChatCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ChatFunction `json:"function"`
}

// ChatFunction contains the name and JSON arguments for a tool call.
type ChatFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// LastPath returns the path used for the most recent gq session.
func LastPath(home string) string {
	return filepath.Join(home, ".gq", "sessions", "last.jsonl")
}

// Load reads a JSONL session and returns the messages in their original order.
// Empty lines are ignored. The line number is included in errors to make a
// damaged session straightforward to diagnose.
func Load(path string) ([]ChatMessage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open session %q: %w", path, err)
	}
	defer file.Close()

	var messages []ChatMessage
	scanner := bufio.NewScanner(file)
	// Tool arguments and message content can be large. The default Scanner
	// limit (64 KiB) is unnecessarily restrictive for a session file.
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var message ChatMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			return nil, fmt.Errorf("decode session %q line %d: %w", path, line, err)
		}
		messages = append(messages, message)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read session %q: %w", path, err)
	}
	return messages, nil
}

// LoadLast reads the most recent session from home.
func LoadLast(home string) ([]ChatMessage, error) {
	return Load(LastPath(home))
}

// Save writes messages as JSONL, replacing path. The parent directory is
// created with private permissions and the session itself is private.
func Save(path string, messages []ChatMessage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open session %q for writing: %w", path, err)
	}
	// OpenFile does not change permissions when the file already exists.
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return fmt.Errorf("set session permissions: %w", err)
	}
	encoder := json.NewEncoder(file)
	for i, message := range messages {
		if err := encoder.Encode(message); err != nil {
			file.Close()
			return fmt.Errorf("encode session message %d: %w", i+1, err)
		}
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close session %q: %w", path, err)
	}
	return nil
}

// SaveLast writes the most recent session under home.
func SaveLast(home string, messages []ChatMessage) error {
	return Save(LastPath(home), messages)
}
