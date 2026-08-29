package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
)

const defaultAPIURL = "https://openrouter.ai/api/v1/chat/completions"

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the request body for the OpenRouter chat completions API.
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// ChatChoice represents a single choice in the API response.
type ChatChoice struct {
	Message Message `json:"message"`
}

// ChatResponse is the response from the OpenRouter chat completions API.
type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
	Error   *APIError    `json:"error,omitempty"`
}

// APIError represents an error returned by the API.
type APIError struct {
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: %s", e.Message)
}

// GetAPIKey retrieves the OpenRouter API key from pass.
func GetAPIKey() (string, error) {
	out, err := exec.Command("bash", "-c", "pass show pi/openrouter").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get API key from pass: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Chat sends a single prompt to the LLM and returns the response text.
func Chat(apiKey, model, prompt string) (string, error) {
	return ChatWithHistory(apiKey, model, []Message{
		{Role: "user", Content: prompt},
	})
}

// ChatWithHistory sends a conversation history to the LLM and returns the response text.
func ChatWithHistory(apiKey, model string, messages []Message) (string, error) {
	reqBody := ChatRequest{
		Model:    model,
		Messages: messages,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", defaultAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w (raw: %s)", err, string(respBody))
	}

	if chatResp.Error != nil {
		return "", chatResp.Error
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}
