package llm

import "fmt"

// Message represents a chat message.
type Message struct {
	Role       string      `json:"role"`
	Content    string      `json:"content,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool call from the LLM.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool represents a tool definition.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction represents a tool function definition.
type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ChatRequest is the request body for the OpenRouter chat completions API.
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
}

// ChatChoice represents a single choice in the API response.
type ChatChoice struct {
	FinishReason string  `json:"finish_reason"`
	Message      Message `json:"message"`
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
