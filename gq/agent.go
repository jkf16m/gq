package main

import (
	"encoding/json"
	"fmt"

	"github.com/jkf16m/gq/llm"
)

const systemPrompt = `You are a helpful AI assistant. You have one tool available:

- cmd: Execute a bash command. Use it when the user asks you to run commands or perform system operations.

When the user asks you to run a command, use the cmd tool. Otherwise, respond with text.`

// Tools available to the agent
var tools = []llm.Tool{
	{
		Type: "function",
		Function: llm.ToolFunction{
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

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	Command string
	Output  string
}

// AgentLoop runs the agent loop until the LLM stops calling tools.
// In TTY mode, it prompts for confirmation on each tool call.
// Returns the final text response and any pending tool calls (for non-TTY modes).
func AgentLoop(apiKey, model, userMessage string, tty bool) (string, []ToolResult) {
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	var allResults []ToolResult

	for {
		response, err := llm.ChatWithHistory(apiKey, model, messages, tools)
		if err != nil {
			return fmt.Sprintf("Error: %v", err), allResults
		}

		if len(response.Choices) == 0 {
			return "No response from LLM", allResults
		}

		choice := response.Choices[0]
		msg := choice.Message

		// Check if LLM wants to call tools
		if choice.FinishReason == "tool_calls" && len(msg.ToolCalls) > 0 {
			// Add assistant message with tool calls
			messages = append(messages, msg)

			rejected := false

			// Execute each tool call
			for _, tc := range msg.ToolCalls {
				var args struct {
					Command string `json:"command"`
				}
				json.Unmarshal([]byte(tc.Function.Arguments), &args)

				if tty {
					// TTY mode: prompt for confirmation
					if !confirm(args.Command) {
						rejected = true
						// Add rejection to messages
						messages = append(messages, llm.Message{
							Role:       "tool",
							ToolCallID: tc.ID,
							Content:    "Command rejected by user",
						})
						break
					}

					// Execute command
					output, _ := executeCommand(args.Command)
					fmt.Println(output)

					// Add result to messages
					messages = append(messages, llm.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    output,
					})
				} else {
					// Non-TTY mode: queue command
					allResults = append(allResults, ToolResult{
						Command: args.Command,
					})

					messages = append(messages, llm.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    fmt.Sprintf("Command queued: %s", args.Command),
					})
				}
			}

			// If rejected in TTY mode, break the loop
			if rejected && tty {
				break
			}

			// Continue loop
			continue
		}

		// LLM is done - return final response
		return msg.Content, allResults
	}

	// If we broke out of loop (rejected), get final response from agent
	// Add a message indicating rejection
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: "The user rejected the command. Please respond to the user.",
	})

	finalResponse, err := llm.ChatWithHistory(apiKey, model, messages, nil)
	if err != nil {
		return "Command rejected.", allResults
	}

	if len(finalResponse.Choices) > 0 {
		return finalResponse.Choices[0].Message.Content, allResults
	}

	return "Command rejected.", allResults
}
