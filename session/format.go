// Package session parses the human-readable .gq session format.
package session

import (
	"fmt"
	"strconv"
	"strings"
)

type Message struct {
	Role       string
	Content    string
	ToolCallID string
	ToolCalls  []ToolCall
}

type ToolCall struct {
	ID        string
	Type      string
	Name      string
	Arguments string
}

// Escape encodes characters that would otherwise break the one-record-per-line
// format. Backslashes are escaped first so the operation is reversible.
func Escape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	value = strings.ReplaceAll(value, "\t", `\t`)
	return value
}

// Unescape reverses Escape for supported .gq escape sequences.
func Unescape(value string) (string, error) {
	var result strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' {
			result.WriteByte(value[i])
			continue
		}
		if i+1 >= len(value) {
			return "", fmt.Errorf("trailing escape")
		}
		i++
		switch value[i] {
		case 'n':
			result.WriteByte('\n')
		case 't':
			result.WriteByte('\t')
		case '\\':
			result.WriteByte('\\')
		default:
			return "", fmt.Errorf("unknown escape \\\\%c", value[i])
		}
	}
	return result.String(), nil
}

// Parse parses a .gq session. The first space separates the record type from
// its payload; message contents are escaped with Escape rather than quoted.
func Parse(data []byte) ([]Message, error) {
	var messages []Message
	for lineNumber, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		message, err := ParseLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber+1, err)
		}
		if message.Role == "tool_call" {
			if len(messages) == 0 || messages[len(messages)-1].Role != "assistant" {
				messages = append(messages, Message{Role: "assistant"})
			}
			messages[len(messages)-1].ToolCalls = append(messages[len(messages)-1].ToolCalls, message.ToolCalls...)
		} else {
			messages = append(messages, message)
		}
	}
	return messages, nil
}

// ParseLine parses one record. User and assistant payloads are everything
// after the first space, so spaces in the message are preserved.
func tokenFields(input string) []string {
	var result []string
	for input = strings.TrimSpace(input); input != ""; {
		start, quote, escaped := 0, false, false
		for i := 0; i < len(input); i++ {
			switch {
			case escaped:
				escaped = false
			case input[i] == '\\' && quote:
				escaped = true
			case input[i] == '"':
				quote = !quote
			case input[i] == ' ' && !quote:
				result = append(result, input[start:i])
				input = strings.TrimSpace(input[i:])
				start = -1
			}
			if start == -1 {
				break
			}
		}
		if start == -1 {
			continue
		}
		result = append(result, input)
		break
	}
	return result
}

func firstField(input string) (field, rest string, ok bool, err error) {
	input = strings.TrimLeft(input, " ")
	if input == "" {
		return "", "", false, nil
	}
	if input[0] != '"' {
		field, rest, ok = strings.Cut(input, " ")
		if !ok {
			return input, "", true, nil
		}
		return field, strings.TrimLeft(rest, " "), true, nil
	}
	for i := 1; i < len(input); i++ {
		if input[i] == '"' && input[i-1] != '\\' {
			field, err = strconv.Unquote(input[:i+1])
			if err != nil {
				return "", "", false, err
			}
			return field, strings.TrimLeft(input[i+1:], " "), true, nil
		}
	}
	return "", "", false, fmt.Errorf("unterminated quoted id")
}

func ParseLine(line string) (Message, error) {
	kind, payload, ok := strings.Cut(line, " ")
	if !ok || payload == "" {
		return Message{}, fmt.Errorf("expected a message type and payload")
	}
	switch kind {
	case "a", "u":
		content, err := Unescape(payload)
		if err != nil {
			return Message{}, fmt.Errorf("message: %w", err)
		}
		role := "assistant"
		if kind == "u" {
			role = "user"
		}
		return Message{Role: role, Content: content}, nil
	case "t":
		id, remainder, ok, err := firstField(payload)
		if err != nil {
			return Message{}, fmt.Errorf("tool call id: %w", err)
		}
		if !ok || id == "" {
			return Message{}, fmt.Errorf("tool call requires an id and tool name")
		}
		name, attributes, ok, err := firstField(remainder)
		if err != nil {
			return Message{}, fmt.Errorf("tool name: %w", err)
		}
		if !ok || name == "" {
			return Message{}, fmt.Errorf("tool call requires an id and tool name")
		}
		values := make(map[string]string)
		for _, attribute := range tokenFields(attributes) {
			key, value, ok := strings.Cut(attribute, "=")
			if !ok || key == "" {
				return Message{}, fmt.Errorf("invalid tool-call attribute %q", attribute)
			}
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				value, err = strconv.Unquote(value)
				if err != nil {
					return Message{}, fmt.Errorf("attribute %s: %w", key, err)
				}
			}
			value, err = Unescape(value)
			if err != nil {
				return Message{}, fmt.Errorf("attribute %s: %w", key, err)
			}
			values[key] = value
		}
		return Message{Role: "tool_call", ToolCalls: []ToolCall{{ID: id, Type: "function", Name: name, Arguments: values["arguments"]}}}, nil
	case "tr":
		id, result, ok, err := firstField(payload)
		if err != nil {
			return Message{}, fmt.Errorf("tool result id: %w", err)
		}
		if !ok || id == "" || result == "" {
			return Message{}, fmt.Errorf("tool result requires an id and result")
		}
		result, err = Unescape(result)
		if err != nil {
			return Message{}, fmt.Errorf("tool result: %w", err)
		}
		return Message{Role: "tool", ToolCallID: id, Content: result}, nil
	default:
		return Message{}, fmt.Errorf("unknown message type %q", kind)
	}
}
