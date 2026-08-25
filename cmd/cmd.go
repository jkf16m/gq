package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"gq/config"
)

const openRouterURL = "https://openrouter.ai/api/v1/chat/completions"

var rootCmd = &cobra.Command{
	Use: "gq", Short: "An agentic CLI tool",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return cmd.Help()
		}
		return askSinglePrompt()
	},
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Tools    []tool    `json:"tools,omitempty"`
}
type message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content,omitempty"`
	ToolCalls  []toolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}
type tool struct {
	Type     string             `json:"type"`
	Function functionDefinition `json:"function"`
}
type functionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}
type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}
type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func askSinglePrompt() error {
	applicationPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find application path: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	loadedConfig, err := config.Load("", home, filepath.Dir(applicationPath))
	if err != nil {
		return err
	}

	apiKey, err := getOpenRouterAPIKey()
	if err != nil {
		return err
	}
	fmt.Print("\033[1;36mgq\033[0m \033[1;36m›\033[0m ")
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(input) == 0 {
		return fmt.Errorf("read prompt: %w", err)
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	messages := make([]message, 0, 2)
	if loadedConfig.Context != "" {
		messages = append(messages, message{Role: "system", Content: loadedConfig.Context})
	}
	messages = append(messages, message{Role: "user", Content: input})
	return runConversation(apiKey, messages, false)
}

func runConversation(apiKey string, messages []message, continuing bool) error {
	if err := saveSession(messages, !continuing); err != nil {
		return err
	}
	for step := 0; step < 20; step++ {
		response, err := complete(apiKey, messages)
		if err != nil {
			return err
		}
		if len(response.Choices) == 0 {
			return fmt.Errorf("OpenRouter returned no choices")
		}
		assistant := response.Choices[0].Message
		if len(assistant.ToolCalls) == 0 {
			if text, ok := assistant.Content.(string); ok {
				printPadded(text)
			}
			return nil
		}
		messages = append(messages, assistant)
		if err := saveSession(messages, false); err != nil {
			return err
		}
		for _, call := range assistant.ToolCalls {
			if call.Function.Name != "cmd" {
				return fmt.Errorf("unsupported tool: %s", call.Function.Name)
			}
			var args struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				return fmt.Errorf("invalid cmd arguments: %w", err)
			}
			fmt.Printf("\n \033[2m$ %s\033[0m\n", args.Command)
			approved, err := approveToolCall()
			if err != nil {
				return err
			}
			if !approved {
				messages = append(messages, message{Role: "tool", ToolCallID: call.ID, Content: "Tool call rejected by user."})
				if err := saveSession(messages, false); err != nil {
					return err
				}
				continue
			}
			output, exitCode := runCommand(args.Command)
			messages = append(messages, message{Role: "tool", ToolCallID: call.ID, Content: fmt.Sprintf("exit code: %d\n%s", exitCode, output)})
			if err := saveSession(messages, false); err != nil {
				return err
			}
		}
	}
	return fmt.Errorf("agent exceeded maximum tool steps")
}

func continueSession() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".gq", "sessions", "last.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read last session: %w", err)
	}
	var messages []message
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var msg message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return fmt.Errorf("parse session: %w", err)
		}
		messages = append(messages, msg)
	}
	if len(messages) == 0 {
		return fmt.Errorf("last session is empty")
	}
	messages = repairSession(messages)
	fmt.Print("\033[1;36mgq\033[0m \033[1;36m›\033[0m ")
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(input) == 0 {
		return err
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	messages = append(messages, message{Role: "user", Content: input})
	apiKey, err := getOpenRouterAPIKey()
	if err != nil {
		return err
	}
	return runConversation(apiKey, messages, true)
}

func repairSession(messages []message) []message {
	var repaired []message
	for i, msg := range messages {
		repaired = append(repaired, msg)
		if msg.Role != "assistant" || len(msg.ToolCalls) == 0 {
			continue
		}
		seen := make(map[string]bool)
		for j := i + 1; j < len(messages) && messages[j].Role == "tool"; j++ {
			seen[messages[j].ToolCallID] = true
		}
		for _, call := range msg.ToolCalls {
			if !seen[call.ID] {
				repaired = append(repaired, message{Role: "tool", ToolCallID: call.ID, Content: "Tool call was not completed in the previous session and was rejected."})
			}
		}
	}
	return repaired
}

func saveSession(messages []message, replace bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".gq", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "last.jsonl")
	flag := os.O_CREATE | os.O_WRONLY
	if replace {
		flag |= os.O_TRUNC
	} else {
		flag |= os.O_APPEND
	}
	file, err := os.OpenFile(path, flag, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if replace {
		for _, msg := range messages {
			data, _ := json.Marshal(msg)
			fmt.Fprintln(file, string(data))
		}
	} else {
		// Rewrite to avoid duplicating messages as the in-memory conversation grows.
		if err := file.Close(); err != nil {
			return err
		}
		data, _ := json.Marshal(messages)
		_ = data
		return writeSession(path, messages)
	}
	return nil
}

func writeSession(path string, messages []message) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, msg := range messages {
		data, _ := json.Marshal(msg)
		if _, err := fmt.Fprintln(file, string(data)); err != nil {
			return err
		}
	}
	return nil
}

func complete(apiKey string, messages []message) (chatResponse, error) {
	body, err := json.Marshal(chatRequest{Model: "openai/gpt-5.6-luna", Messages: messages, Tools: []tool{{Type: "function", Function: functionDefinition{Name: "cmd", Description: "Run one bash command in the current project. Return the command as a bash string.", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"command": map[string]string{"type": "string", "description": "The bash command to run"}}, "required": []string{"command"}}}}}})
	if err != nil {
		return chatResponse{}, fmt.Errorf("encode request: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterURL, bytes.NewReader(body))
	if err != nil {
		return chatResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return chatResponse{}, fmt.Errorf("send request: %w", err)
	}
	defer res.Body.Close()
	var response chatResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return response, fmt.Errorf("decode response: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if response.Error != nil {
			return response, fmt.Errorf("OpenRouter: %s", response.Error.Message)
		}
		return response, fmt.Errorf("OpenRouter returned HTTP %s", res.Status)
	}
	return response, nil
}

func approveToolCall() (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, fmt.Errorf("tool approval requires an interactive terminal")
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return false, fmt.Errorf("enable tool approval input: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	fmt.Print("Approve? Press Enter twice within 2s, or Backspace to reject: ")
	enters := 0
	var firstEnter time.Time
	fd := int(os.Stdin.Fd())
	for {
		var readfds syscall.FdSet
		readfds.Bits[fd/64] |= 1 << (uint(fd) % 64)
		timeout := syscall.Timeval{Sec: 0, Usec: 100000}
		_, err := syscall.Select(fd+1, &readfds, nil, nil, &timeout)
		if err != nil {
			return false, fmt.Errorf("wait for tool approval: %w", err)
		}
		if enters == 1 && time.Since(firstEnter) >= 2*time.Second {
			enters = 0
		}
		if readfds.Bits[fd/64]&(1<<(uint(fd)%64)) == 0 {
			continue
		}
		var b [1]byte
		if _, err := os.Stdin.Read(b[:]); err != nil {
			return false, fmt.Errorf("read tool approval: %w", err)
		}
		switch b[0] {
		case 8, 127:
			fmt.Println(" rejected")
			fmt.Println()
			return false, nil
		case '\r', '\n':
			if enters == 0 {
				enters = 1
				firstEnter = time.Now()
				continue
			}
			if time.Since(firstEnter) < 2*time.Second {
				fmt.Println(" approved")
				fmt.Println()
				return true, nil
			}
			enters = 1
			firstEnter = time.Now()
		}
	}
}

func printPadded(text string) {
	width := 80
	if terminalWidth, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && terminalWidth > 10 {
		width = terminalWidth - 2 // one padding character on each side
	}

	fmt.Println()
	for _, line := range wrapText(strings.TrimRight(text, "\n"), width) {
		fmt.Printf(" %s\n", line)
	}
	fmt.Println()
}

func wrapText(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	var wrapped []string
	for _, paragraph := range strings.Split(text, "\n") {
		if strings.TrimSpace(paragraph) == "" {
			wrapped = append(wrapped, "")
			continue
		}

		words := strings.Fields(paragraph)
		line := ""
		for _, word := range words {
			for len([]rune(word)) > width {
				part := string([]rune(word)[:width])
				if line != "" {
					wrapped = append(wrapped, line)
					line = ""
				}
				wrapped = append(wrapped, part)
				word = string([]rune(word)[width:])
			}
			if line == "" {
				line = word
			} else if len([]rune(line))+1+len([]rune(word)) <= width {
				line += " " + word
			} else {
				wrapped = append(wrapped, line)
				line = word
			}
		}
		if line != "" {
			wrapped = append(wrapped, line)
		}
	}
	return wrapped
}

func runCommand(command string) (string, int) {
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(output), exitErr.ExitCode()
	}
	return string(output) + "\n" + err.Error(), 1
}

func getOpenRouterAPIKey() (string, error) {
	output, err := exec.Command("pass", "show", "pi/openrouter").Output()
	if err != nil {
		return "", fmt.Errorf("get API key from pass: %w", err)
	}
	key := strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])
	if key == "" {
		return "", fmt.Errorf("empty API key from pass")
	}
	return key, nil
}

func Execute() error { return rootCmd.Execute() }
func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(modelCmd)
	rootCmd.AddCommand(&cobra.Command{Use: "c", Aliases: []string{"continue"}, Short: "Continue the last session", RunE: func(cmd *cobra.Command, args []string) error { return continueSession() }})
}
