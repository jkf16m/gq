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
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const openRouterURL = "https://openrouter.ai/api/v1/chat/completions"

var rootCmd = &cobra.Command{
	Use:   "gq",
	Short: "An agentic CLI tool",
	Long:  `gq is an agentic CLI that helps you with tasks using AI models.`,
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
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func askSinglePrompt() error {
	apiKey, err := getOpenRouterAPIKey()
	if err != nil {
		return err
	}

	fmt.Print("Prompt: ")
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(input) == 0 {
		return fmt.Errorf("read prompt: %w", err)
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	body, err := json.Marshal(chatRequest{
		Model:    "openai/gpt-5.6-luna",
		Messages: []message{{Role: "user", Content: input}},
	})
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer res.Body.Close()

	var response chatResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if response.Error != nil {
			return fmt.Errorf("OpenRouter: %s", response.Error.Message)
		}
		return fmt.Errorf("OpenRouter returned HTTP %s", res.Status)
	}
	if len(response.Choices) == 0 {
		return fmt.Errorf("OpenRouter returned no choices")
	}

	fmt.Println(response.Choices[0].Message.Content)
	return nil
}

func getOpenRouterAPIKey() (string, error) {
	output, err := exec.Command("pass", "show", "pi/openrouter").Output()
	if err != nil {
		return "", fmt.Errorf("get API key from pass: %w", err)
	}
	lines := strings.SplitN(string(output), "\n", 2)
	key := strings.TrimSpace(lines[0])
	if key == "" {
		return "", fmt.Errorf("empty API key from pass")
	}
	return key, nil
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(modelCmd)
}
