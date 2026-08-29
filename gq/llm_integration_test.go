package main

import (
	"strings"
	"testing"
)

const testModel = "@preset/mimo"

func TestGetAPIKey(t *testing.T) {
	apiKey, err := GetAPIKey()
	if err != nil {
		t.Fatalf("GetAPIKey failed: %v", err)
	}
	if apiKey == "" {
		t.Fatal("GetAPIKey returned empty key")
	}
	t.Logf("Got API key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
}

func TestChat(t *testing.T) {
	apiKey, err := GetAPIKey()
	if err != nil {
		t.Fatalf("GetAPIKey failed: %v", err)
	}

 response, err := Chat(apiKey, testModel, "Say exactly: test ok")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if response == "" {
		t.Fatal("Chat returned empty response")
	}

	t.Logf("Response: %s", response)
}

func TestChatWithHistory(t *testing.T) {
	apiKey, err := GetAPIKey()
	if err != nil {
		t.Fatalf("GetAPIKey failed: %v", err)
	}

	messages := []Message{
		{Role: "user", Content: "My name is Alice."},
		{Role: "assistant", Content: "Hello Alice!"},
		{Role: "user", Content: "What is my name?"},
	}

	response, err := ChatWithHistory(apiKey, testModel, messages)
	if err != nil {
		t.Fatalf("ChatWithHistory failed: %v", err)
	}

	if response == "" {
		t.Fatal("ChatWithHistory returned empty response")
	}

 lower := strings.ToLower(response)
	if !strings.Contains(lower, "alice") {
		t.Errorf("Expected response to mention 'Alice', got: %s", response)
	}

	t.Logf("Response: %s", response)
}

func TestChatInvalidKey(t *testing.T) {
	_, err := Chat("invalid-key-12345", testModel, "Hello")
	if err == nil {
		t.Fatal("Expected error for invalid API key, got nil")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected 401 error, got: %v", err)
	}

	t.Logf("Got expected error: %v", err)
}
