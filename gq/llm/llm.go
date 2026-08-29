package llm

// Chat sends a single prompt to the LLM and returns the response.
func Chat(apiKey, model, prompt string) (*ChatResponse, error) {
	return ChatWithHistory(apiKey, model, []Message{
		{Role: "user", Content: prompt},
	}, nil)
}

// ChatWithHistory sends a conversation history to the LLM and returns the response.
func ChatWithHistory(apiKey, model string, messages []Message, tools []Tool) (*ChatResponse, error) {
	c := newClient(apiKey)

	return c.chatCompletion(ChatRequest{
		Model:    model,
		Messages: messages,
		Tools:    tools,
	})
}
