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
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"gq/config"
	"gq/project"
	gqsession "gq/session"
)

const (
	openRouterURL = "https://openrouter.ai/api/v1/chat/completions"
	defaultModel  = "@preset/mimo"
)

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
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("find working directory: %w", err)
	}
	projectFiles, err := project.LoadFiles(workingDirectory)
	if err != nil {
		return fmt.Errorf("load project files: %w", err)
	}

	apiKey, err := getOpenRouterAPIKey()
	if err != nil {
		return err
	}
	fmt.Print("\033[1;36mgq\033[0m \033[1;36m›\033[0m ")
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(input) == 0 {
		return err
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	messages := make([]message, 0, 2)
	contextText := loadedConfig.Context
	projectContext := project.Context(projectFiles)
	if projectContext != "" {
		if contextText != "" {
			contextText += "\n\n"
		}
		contextText += projectContext
	}
	if contextText != "" {
		messages = append(messages, message{Role: "system", Content: contextText})
	}
	messages = append(messages, message{Role: "user", Content: input})
	return runConversation(apiKey, messages, false, modelFromConfig(loadedConfig))
}

func modelFromConfig(cfg config.Result) string {
	if m, ok := cfg.Values["model"].(string); ok && m != "" {
		return m
	}
	return defaultModel
}

func runConversation(apiKey string, messages []message, continuing bool, model string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := saveSession(messages, !continuing); err != nil {
		return err
	}
	if continuing {
		if err := handlePendingTools(ctx, messages); err != nil {
			return err
		}
	}
	for step := 0; step < 20; step++ {
		response, err := complete(ctx, apiKey, messages, model)
		if err != nil {
			return err
		}
		if len(response.Choices) == 0 {
			return fmt.Errorf("OpenRouter returned no choices")
		}
		assistant := response.Choices[0].Message
		if len(assistant.ToolCalls) == 0 {
			if text, ok := assistant.Content.(string); ok {
				printMarkdown(text)
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
			printToolCommand(args.Command)
			approved, err := approveToolCall(ctx)
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
			output, exitCode := runCommand(ctx, args.Command)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			printToolResult(args.Command, output, exitCode)
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
	path := filepath.Join(home, ".gq", "sessions", "last.gq")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read last session: %w", err)
	}
	messages, err := parseGQSession(data)
	if err != nil {
		return fmt.Errorf("parse session: %w", err)
	}
	if len(messages) == 0 {
		return fmt.Errorf("last session is empty")
	}
	// Leave incomplete tool calls intact. runConversation will present them to
	// the user for approval and append their results before asking the model to
	// continue.
	fmt.Print("\033[1;36mgq\033[0m \033[1;36m›\033[0m ")
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(input) == 0 {
		return fmt.Errorf("read prompt: %w", err)
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
	applicationPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find application path: %w", err)
	}
	loadedConfig, err := config.Load("", home, filepath.Dir(applicationPath))
	if err != nil {
		return err
	}
	return runConversation(apiKey, messages, true, modelFromConfig(loadedConfig))
}

func handlePendingTools(ctx context.Context, messages []message) error {
	completed := make(map[string]bool)
	for _, msg := range messages {
		if msg.Role == "tool" {
			completed[msg.ToolCallID] = true
		}
	}
	for _, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}
		for _, call := range msg.ToolCalls {
			if completed[call.ID] {
				continue
			}
			if call.Function.Name != "cmd" {
				return fmt.Errorf("unsupported tool: %s", call.Function.Name)
			}
			var args struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				return err
			}
			printToolCommand(args.Command)
			approved, err := approveToolCall(ctx)
			if err != nil {
				return err
			}
			var content string
			if !approved {
				content = "Tool call rejected by user."
			} else {
				output, code := runCommand(ctx, args.Command)
				if ctx.Err() != nil {
					return ctx.Err()
				}
				printToolResult(args.Command, output, code)
				content = fmt.Sprintf("exit code: %d\n%s", code, output)
			}
			messages = append(messages, message{Role: "tool", ToolCallID: call.ID, Content: content})
			if err := saveSession(messages, false); err != nil {
				return err
			}
			completed[call.ID] = true
		}
	}
	return nil
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
	path := filepath.Join(dir, "last.gq")
	if replace {
		return writeSession(path, messages)
	}
	// Rewrite to avoid duplicating messages as the in-memory conversation grows.
	return writeSession(path, messages)
}

func writeGQMessage(file *os.File, msg message) error {
	content, _ := msg.Content.(string)
	var lines []string
	switch msg.Role {
	case "assistant":
		lines = append(lines, "a "+gqsession.Escape(content))
		for _, call := range msg.ToolCalls {
			id := call.ID
			if strings.ContainsAny(id, " \t") {
				id = strconv.Quote(id)
			}
			lines = append(lines, fmt.Sprintf("t %s %s arguments=%s", id, call.Function.Name, strconv.Quote(gqsession.Escape(call.Function.Arguments))))
		}
	case "user":
		lines = append(lines, "u "+gqsession.Escape(content))
	case "tool":
		id := msg.ToolCallID
		if strings.ContainsAny(id, " \t") {
			id = strconv.Quote(id)
		}
		lines = append(lines, "tr "+id+" "+gqsession.Escape(content))
	case "system":
		// System context is reconstructed from configuration for new prompts.
		// Continuation sessions do not persist it in the compact format.
		return nil
	default:
		return fmt.Errorf("unsupported session message role %q", msg.Role)
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(file, line); err != nil {
			return err
		}
	}
	return nil
}

func parseGQSession(data []byte) ([]message, error) {
	parsed, err := gqsession.Parse(data)
	if err != nil {
		return nil, err
	}
	messages := make([]message, 0, len(parsed))
	for _, msg := range parsed {
		converted := message{Role: msg.Role, Content: msg.Content, ToolCallID: msg.ToolCallID}
		for _, call := range msg.ToolCalls {
			convertedCall := toolCall{ID: call.ID, Type: call.Type}
			convertedCall.Function.Name = call.Name
			convertedCall.Function.Arguments = call.Arguments
			converted.ToolCalls = append(converted.ToolCalls, convertedCall)
		}
		messages = append(messages, converted)
	}
	return messages, nil
}

func writeSession(path string, messages []message) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, msg := range messages {
		if err := writeGQMessage(file, msg); err != nil {
			return err
		}
	}
	return nil
}

func complete(ctx context.Context, apiKey string, messages []message, model string) (chatResponse, error) {
	body, err := json.Marshal(chatRequest{Model: model, Messages: messages, Tools: []tool{{Type: "function", Function: functionDefinition{Name: "cmd", Description: "Run one bash command in the current project. Return the command as a bash string.", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"command": map[string]string{"type": "string", "description": "The bash command to run"}}, "required": []string{"command"}}}}}})
	if err != nil {
		return chatResponse{}, fmt.Errorf("encode request: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	stopSpinner := startSpinner(ctx)
	defer stopSpinner()
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

func approveToolCall(ctx context.Context) (bool, error) {
	tty, err := openApprovalTTY()
	if err != nil {
		return false, err
	}
	if tty != os.Stdin {
		defer tty.Close()
	}

	fd := int(tty.Fd())
	if !term.IsTerminal(fd) {
		return false, fmt.Errorf("tool approval requires an interactive terminal")
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return false, fmt.Errorf("enable tool approval input: %w", err)
	}
	defer term.Restore(fd, oldState)

	// Reading in a goroutine lets the timer expire without platform-specific
	// polling APIs. Closing a fallback TTY on return releases that reader.
	input := make(chan byte, 1)
	readErr := make(chan error, 1)
	go func() {
		var b [1]byte
		for {
			if _, err := tty.Read(b[:]); err != nil {
				readErr <- err
				return
			}
			input <- b[0]
		}
	}()

	enters := 0
	var firstEnter time.Time
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case err := <-readErr:
			return false, fmt.Errorf("read tool approval: %w", err)
		case b := <-input:
			if b == 3 { // Ctrl-C
				return false, context.Canceled
			}
			if b == 'c' || b == 'C' {
				return false, nil
			}
			if b == 'o' || b == 'O' {
				if enters == 1 && time.Since(firstEnter) < 2*time.Second {
					return true, nil
				}
				enters = 1
				firstEnter = time.Now()
				continue
			}
			// Only consecutive o keypresses approve. Any other key resets
			// the pending approval sequence.
			enters = 0
		}
	}
}

func openApprovalTTY() (*os.File, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return os.Stdin, nil
	}
	for _, path := range []string{"/dev/tty", "CONIN$"} {
		tty, err := os.OpenFile(path, os.O_RDWR, 0)
		if err == nil {
			return tty, nil
		}
	}
	return nil, fmt.Errorf("tool approval requires an interactive terminal")
}

func printMarkdown(text string) {
	renderer, err := glamour.NewTermRenderer(glamour.WithAutoStyle())
	if err == nil {
		if rendered, renderErr := renderer.Render(text); renderErr == nil {
			text = rendered
		}
	}
	fmt.Println()
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			fmt.Println()
			continue
		}
		fmt.Printf(" %s\n", strings.TrimRight(line, " \t"))
	}
	fmt.Println()
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

func runCommand(ctx context.Context, command string) (string, int) {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
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

func startSpinner(ctx context.Context) func() {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		frames := []string{"|", "/", "-", "\\"}
		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-ticker.C:
				fmt.Printf("\rWaiting for response %s (Ctrl-C to cancel)", frames[i%len(frames)])
				i++
			case <-done:
				fmt.Print("\r\033[K")
				return
			case <-ctx.Done():
				fmt.Print("\r\033[K")
				return
			}
		}
	}()
	return func() {
		select {
		case <-done:
		default:
			close(done)
		}
		<-stopped
	}
}

func printToolCommand(command string) {
	fmt.Printf("\n \033[2m$ %s\033[0m\n", command)
	fmt.Print(" Press o twice to approve, c to cancel: ")
}

func printToolResult(command, output string, exitCode int) {
	_ = command
	fmt.Printf(" exit code: %d\n", exitCode)
	if output != "" {
		for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
			fmt.Printf(" %s\n", line)
		}
	}
	fmt.Println()
}

func showInfo() error {
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
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("find working directory: %w", err)
	}
	files, err := project.LoadFiles(workingDirectory)
	if err != nil {
		return fmt.Errorf("load project files: %w", err)
	}
	contextText := loadedConfig.Context
	projectContext := project.Context(files)
	if projectContext != "" {
		if contextText != "" {
			contextText += "\n\n"
		}
		contextText += projectContext
	}
	fmt.Println("Files loaded into system prompt:")
	if loadedConfig.Context != "" {
		fmt.Println("  configured context")
	}
	for _, file := range files {
		fmt.Printf("  %s\n", file.Path)
	}
	fmt.Printf("\nTotal characters: %d\n", len(contextText))
	return nil
}

func showConfig() error {
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
	fmt.Println("\033[1;36mgq\033[0m configuration:")
	fmt.Println()
	if len(loadedConfig.Values) == 0 {
		fmt.Println("  No configuration values set.")
	} else {
		keys := make([]string, 0, len(loadedConfig.Values))
		for key := range loadedConfig.Values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := loadedConfig.Values[key]
			source := loadedConfig.Sources[key]
			fmt.Printf("  \033[1m%s\033[0m = %v\n", key, value)
			fmt.Printf("    source: %s\n", source)
		}
	}
	fmt.Println()
	fmt.Println("\033[1mConfig files loaded:\033[0m")
	if len(loadedConfig.Files) == 0 {
		fmt.Println("  None")
	} else {
		for _, f := range loadedConfig.Files {
			fmt.Printf("  %s\n", f)
		}
	}
	return nil
}

func Execute() error { return rootCmd.Execute() }
func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(modelCmd)
	rootCmd.AddCommand(&cobra.Command{Use: "c", Aliases: []string{"continue"}, Short: "Continue the last session", RunE: func(cmd *cobra.Command, args []string) error { return continueSession() }})
	rootCmd.AddCommand(&cobra.Command{Use: "config", Short: "Show current configuration and its sources", RunE: func(cmd *cobra.Command, args []string) error { return showConfig() }})
	rootCmd.AddCommand(&cobra.Command{Use: "info", Aliases: []string{"i"}, Short: "Show the context sent to the model", RunE: func(cmd *cobra.Command, args []string) error { return showInfo() }})
}
