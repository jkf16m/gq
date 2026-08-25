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
	if _, err := config.Load("", home, filepath.Dir(applicationPath)); err != nil {
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

	messages := []message{{Role: "user", Content: input}}
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
				continue
			}
			output, exitCode := runCommand(args.Command)
			messages = append(messages, message{Role: "tool", ToolCallID: call.ID, Content: fmt.Sprintf("exit code: %d\n%s", exitCode, output)})
		}
	}
	return fmt.Errorf("agent exceeded maximum tool steps")
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
func init()          { rootCmd.AddCommand(versionCmd); rootCmd.AddCommand(modelCmd) }
