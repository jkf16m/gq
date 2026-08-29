package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/glamour"
	"golang.org/x/term"
)

const (
	apiURL  = "https://openrouter.ai/api/v1/chat/completions"
	m       = "@preset/mimo"
	passKey = "pi/openrouter"
)

// Config

type Config struct {
	KeepWalking  *bool    `json:"keepWalking"`
	Include      []string `json:"include"`
	Exclude      []string `json:"exclude"`
	UseGitignore bool     `json:"useGitignore"`
}

// configPatch preserves whether a setting was present in a file. This is
// important for precedence: an omitted setting in the nearest config must not
// overwrite an explicit setting from a parent config.
type configPatch struct {
	KeepWalking  *bool     `json:"keepWalking"`
	Include      *[]string `json:"include"`
	Exclude      *[]string `json:"exclude"`
	UseGitignore *bool     `json:"useGitignore"`
}

func mergeConfigPatch(selected *configPatch, patch configPatch) {
	if selected.KeepWalking == nil {
		selected.KeepWalking = patch.KeepWalking
	}
	if selected.Include == nil {
		selected.Include = patch.Include
	}
	if selected.Exclude == nil {
		selected.Exclude = patch.Exclude
	}
	if selected.UseGitignore == nil {
		selected.UseGitignore = patch.UseGitignore
	}
}

func configFromPatch(patch configPatch) Config {
	cfg := defaultConfig()
	if patch.KeepWalking != nil {
		cfg.KeepWalking = patch.KeepWalking
	}
	if patch.Include != nil {
		cfg.Include = *patch.Include
	}
	if patch.Exclude != nil {
		cfg.Exclude = *patch.Exclude
	}
	if patch.UseGitignore != nil {
		cfg.UseGitignore = *patch.UseGitignore
	}
	return cfg
}

func defaultConfig() Config {
	return Config{
		KeepWalking:  boolPtr(false),
		Include:      []string{"*"},
		Exclude:      []string{},
		UseGitignore: true,
	}
}

func boolPtr(b bool) *bool {
	return &b
}

// Types

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type ExternalTool struct {
	Name string
	Path string
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
}

type ChatResponse struct {
	Choices []struct {
		FinishReason string  `json:"finish_reason"`
		Message      Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Tools

var tools = []Tool{
	{
		Type: "function",
		Function: ToolFunction{
			Name:        "cmd",
			Description: "Execute a bash command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The bash command to execute",
					},
				},
				"required": []string{"command"},
			},
		},
	},
}

func discoverTools(startDir string) []ExternalTool {
	var result []ExternalTool
	seen := make(map[string]bool)
	current := startDir

	for {
		toolDir := filepath.Join(current, ".gq", "tools")
		entries, _ := os.ReadDir(toolDir)
		for _, entry := range entries {
			if entry.IsDir() || seen[entry.Name()] {
				continue
			}
			info, err := entry.Info()
			if err != nil || info.Mode()&0111 == 0 {
				continue
			}
			seen[entry.Name()] = true
			result = append(result, ExternalTool{
				Name: entry.Name(),
				Path: filepath.Join(toolDir, entry.Name()),
			})
		}

		configData, err := os.ReadFile(filepath.Join(current, ".gq", "config.json"))
		if err != nil {
			break
		}
		patch := configPatch{}
		if json.Unmarshal(configData, &patch) != nil || patch.KeepWalking == nil || !*patch.KeepWalking {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	// Precedence is resolved above; this only makes --help execution order
	// deterministic across tools from different directory levels.
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func mustGetwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func loadToolHelp(external []ExternalTool) string {
	var prompt strings.Builder
	for _, tool := range external {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		output, err := exec.CommandContext(ctx, tool.Path, "--help").CombinedOutput()
		cancel()

		prompt.WriteString("\n\n--- command: ")
		prompt.WriteString(tool.Name)
		prompt.WriteString(" ---\n")
		if len(output) == 0 {
			if err != nil {
				prompt.WriteString("--help failed: ")
				prompt.WriteString(err.Error())
			} else {
				prompt.WriteString("(no help output)")
			}
		} else {
			prompt.Write(output)
		}
	}
	return prompt.String()
}

// LLM

func getAPIKey() string {
	// Keep credentials out of the source and let pass handle its own errors.
	out, err := exec.Command("pass", "show", passKey).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get API key from pass: %v\n", err)
		os.Exit(1)
	}
	key := strings.TrimSpace(string(out))
	if key == "" {
		fmt.Fprintln(os.Stderr, "Error: pass returned an empty API key")
		os.Exit(1)
	}
	return key
}

func startSpinner() func() {
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return func() {}
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r%s Thinking...", frames[i%len(frames)])
			}
		}
	}()

	return func() {
		close(stop)
		wg.Wait()
		// Erase the spinner so it cannot remain after the response is printed.
		fmt.Fprint(os.Stderr, "\r\033[2K")
	}
}

func chat(apiKey string, messages []Message, availableTools []Tool) (*ChatResponse, error) {
	body, err := json.Marshal(ChatRequest{
		Model:    m,
		Messages: messages,
		Tools:    availableTools,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	spinnerDone := startSpinner()
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		spinnerDone()
		return nil, err
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	spinnerDone()
	if readErr != nil {
		return nil, readErr
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, err
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	return &chatResp, nil
}

// Command execution

func executeCommand(command string) string {
	return executeCommandWithTools(command, nil)
}

func executeCommandWithTools(command string, external []ExternalTool) string {
	cmd := exec.Command("bash", "-c", command)
	if len(external) > 0 {
		dirs := make([]string, 0, len(external))
		seen := make(map[string]bool)
		for _, tool := range external {
			dir := filepath.Dir(tool.Path)
			if !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
		}
		cmd.Env = append(os.Environ(), "PATH="+strings.Join(dirs, ":")+":"+os.Getenv("PATH"))
	}
	output, _ := cmd.CombinedOutput()
	return string(output)
}

func printToolOutput(output string) {
	fmt.Fprintln(os.Stdout)
	if output == "" {
		output = "(no output)"
	}

	if term.IsTerminal(int(os.Stdout.Fd())) {
		// Green identifies text produced by the command rather than the LLM.
		fmt.Fprint(os.Stdout, "\033[38;5;114m[cmd output]\033[0m\n")
		fmt.Fprint(os.Stdout, "\033[38;5;114m", output)
		if !strings.HasSuffix(output, "\n") {
			fmt.Fprintln(os.Stdout)
		}
		fmt.Fprint(os.Stdout, "\033[0m")
		return
	}

	fmt.Fprintln(os.Stdout, "[cmd output]")
	fmt.Fprint(os.Stdout, output)
	if !strings.HasSuffix(output, "\n") {
		fmt.Fprintln(os.Stdout)
	}
}

func confirm(command string) bool {
	fmt.Printf("$ %s\n", command)
	fmt.Print("[press y twice to accept, n to reject] ")

	fd := int(os.Stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Println("\nunable to read terminal input")
		return false
	}
	defer term.Restore(fd, state)

	accepted := 0
	var key [1]byte
	for {
		if _, err := os.Stdin.Read(key[:]); err != nil {
			fmt.Println()
			return false
		}

		switch key[0] {
		case 'n':
			fmt.Println("n")
			return false
		case 'y':
			accepted++
			fmt.Print("y")
			if accepted == 2 {
				fmt.Println()
				return true
			}
		}
	}
}

// Context loading

func loadContext() string {
	dir, _ := os.Getwd()
	configs, files := walkAndCollect(dir)

	var context strings.Builder
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(dir, f)
		context.WriteString(fmt.Sprintf("--- %s ---\n", rel))
		context.Write(content)
		context.WriteString("\n\n")
	}

	_ = configs // configs loaded, used for walking decisions
	return context.String()
}

func walkAndCollect(startDir string) ([]Config, []string) {
	var configs []Config
	var allFiles []string
	var selected configPatch

	current := startDir
	for {
		configPath := filepath.Join(current, ".gq", "config.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			// The current directory is always part of the context, even without a
			// config. Only parent walking requires an explicit config.
			if len(configs) == 0 {
				allFiles = append(allFiles, collectFiles(current, configFromPatch(selected))...)
			}
			break
		}

		var patch configPatch
		if err := json.Unmarshal(data, &patch); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid %s: %v\n", configPath, err)
			break
		}

		// Configs are visited nearest-first. Keep the first occurrence of every
		// field, making the nearest config win over every parent config.
		mergeConfigPatch(&selected, patch)
		effective := configFromPatch(selected)
		configs = append(configs, effective)
		allFiles = append(allFiles, collectFiles(current, effective)...)

		if effective.KeepWalking == nil || !*effective.KeepWalking {
			break
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	// Sort by modification time (oldest first), with a stable path tie-breaker.
	sort.SliceStable(allFiles, func(i, j int) bool {
		fi, errI := os.Stat(allFiles[i])
		fj, errJ := os.Stat(allFiles[j])
		if errI != nil || errJ != nil {
			return allFiles[i] < allFiles[j]
		}
		if fi.ModTime().Equal(fj.ModTime()) {
			return allFiles[i] < allFiles[j]
		}
		return fi.ModTime().Before(fj.ModTime())
	})

	return configs, allFiles
}

func collectFiles(dir string, cfg Config) []string {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}

	// Load gitignore if needed
	var gitignorePatterns []string
	if cfg.UseGitignore {
		gitignorePatterns = loadGitignore(dir)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		path := filepath.Join(dir, name)

		// Do not recurse: automatic context contains only files directly in this
		// directory, never files inside folders.

		// Match both the simple filename and the path relative to the configured
		// directory. The latter makes patterns such as "vendor/**" useful.
		rel, _ := filepath.Rel(dir, path)
		if matchesAny(name, cfg.Exclude) || matchesAny(rel, cfg.Exclude) {
			continue
		}

		// Check gitignore patterns
		if cfg.UseGitignore && (matchesAny(name, gitignorePatterns) || matchesAny(rel, gitignorePatterns)) {
			continue
		}

		if isBinaryFile(path) {
			continue
		}

		// Check include patterns
		if matchesAny(name, cfg.Include) || matchesAny(rel, cfg.Include) {
			files = append(files, path)
		}
	}

	return files
}

func isBinaryFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	return bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data)
}

func loadGitignore(dir string) []string {
	path := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

func matchesAny(name string, patterns []string) bool {
	for _, p := range patterns {
		matched, _ := filepath.Match(p, name)
		if matched {
			return true
		}
		// Also check with path
		matched, _ = filepath.Match(p, filepath.Base(name))
		if matched {
			return true
		}
	}
	return false
}

// Agent loop

const systemPrompt = `You are a helpful AI assistant. Technically, you have exactly one tool: cmd.

Commands are first-class citizens in gq. User-defined commands are executable programs discovered from .gq/tools and documented below. To use one, call cmd with the command name and its arguments. Prefer a specialized command when one matches the task; use other bash commands only when no specialized command applies.

The command help below is authoritative. When the task is complete, respond with text.`

func agentLoop(apiKey string, userMessage string) string {
	context := loadContext()
	externalTools := discoverTools(mustGetwd())
	toolHelp := loadToolHelp(externalTools)

	messages := []Message{
		{Role: "system", Content: systemPrompt + toolHelp + "\n\nContext:\n" + context},
		{Role: "user", Content: userMessage},
	}

	for {
		response, err := chat(apiKey, messages, tools)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}

		if len(response.Choices) == 0 {
			return "No response from LLM"
		}

		choice := response.Choices[0]
		msg := choice.Message

		// Tool calls can be accompanied by different finish_reason values across
		// OpenAI-compatible providers, so inspect the message as the source of truth.
		if len(msg.ToolCalls) > 0 {
			messages = append(messages, msg)
			rejected := false

			for _, tc := range msg.ToolCalls {
				var args struct {
					Command string `json:"command"`
				}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil || strings.TrimSpace(args.Command) == "" {
					messages = append(messages, Message{
						Role: "tool", ToolCallID: tc.ID,
						Content: "Invalid cmd arguments",
					})
					continue
				}

				if !confirm(args.Command) {
					rejected = true
					messages = append(messages, Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    "Command rejected by user",
					})
					break
				}

				output := executeCommandWithTools(args.Command, externalTools)
				printToolOutput(output)
				// Tool messages must contain a non-empty content field. In
				// particular, commands such as `echo ... > file` normally produce
				// no stdout, and omitempty would otherwise remove content entirely.
				toolContent := output
				if toolContent == "" {
					toolContent = "(command completed with no output)"
				}

				messages = append(messages, Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    toolContent,
				})
			}

			if rejected {
				messages = append(messages, Message{
					Role:    "user",
					Content: "The user rejected the command. Please respond.",
				})
				finalResp, err := chat(apiKey, messages, tools)
				if err != nil {
					return "Command rejected."
				}
				if len(finalResp.Choices) > 0 {
					return finalResp.Choices[0].Message.Content
				}
				return "Command rejected."
			}

			continue
		}

		return msg.Content
	}
}

// Main

func renderMarkdown(content string) string {
	rendered, err := glamour.Render(content, "dark")
	if err != nil {
		return content
	}
	return rendered
}

func main() {
	// A gq process handles exactly one user turn. Sessions are intentionally
	// short-lived: after the model returns text (or a command is rejected), the
	// process exits instead of handing control back to another prompt.
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	if !scanner.Scan() {
		return
	}

	input := strings.TrimSpace(scanner.Text())
	if input == "" || input == "exit" || input == "quit" {
		return
	}

	apiKey := getAPIKey()
	fmt.Print(renderMarkdown(agentLoop(apiKey, input)))
}
