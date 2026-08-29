package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultAPIURL = "https://openrouter.ai/api/v1/chat/completions"

// client handles HTTP communication with the OpenRouter API.
type client struct {
	apiKey     string
	httpClient *http.Client
}

// newClient creates a new API client.
func newClient(apiKey string) *client {
	return &client{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// chatCompletion sends a chat completion request and returns the response.
func (c *client) chatCompletion(req ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", defaultAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (raw: %s)", err, string(respBody))
	}

	if chatResp.Error != nil {
		return nil, chatResp.Error
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &chatResp, nil
}
