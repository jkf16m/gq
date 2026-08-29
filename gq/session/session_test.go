package session

import (
	"os"
	"testing"

	"github.com/user/gq/config"
)

func TestNewAndLoad(t *testing.T) {
	tmp := t.TempDir()
	os.Chdir(tmp)
	
	cfg := &config.Config{
		SessionDir: ".gq/sessions",
	}
	
	// Create new session
	sess, err := New(cfg, "test-1")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	
	if sess.ID != "test-1" {
		t.Errorf("expected ID test-1, got %s", sess.ID)
	}
	if len(sess.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(sess.Messages))
	}
	
	// Load it back
	loaded, err := Load(cfg, "test-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	
	if loaded.ID != "test-1" {
		t.Errorf("expected ID test-1, got %s", loaded.ID)
	}
}

func TestList(t *testing.T) {
	tmp := t.TempDir()
	os.Chdir(tmp)
	
	cfg := &config.Config{
		SessionDir: ".gq/sessions",
	}
	
	// Empty list
	ids, err := List(cfg)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(ids))
	}
	
	// Create sessions
	New(cfg, "session-1")
	New(cfg, "session-2")
	
	ids, err = List(cfg)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(ids))
	}
}

func TestAddMessage(t *testing.T) {
	tmp := t.TempDir()
	os.Chdir(tmp)
	
	cfg := &config.Config{
		SessionDir: ".gq/sessions",
	}
	
	sess, _ := New(cfg, "msg-test")
	
	// Add message
	err := AddMessage(cfg, sess, Message{
		Role:    "user",
		Content: "Hello",
	})
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	
	// Load and verify
	loaded, _ := Load(cfg, "msg-test")
	if len(loaded.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(loaded.Messages))
	}
	if loaded.Messages[0].Content != "Hello" {
		t.Errorf("expected Hello, got %s", loaded.Messages[0].Content)
	}
}

func TestLast(t *testing.T) {
	tmp := t.TempDir()
	os.Chdir(tmp)
	
	cfg := &config.Config{
		SessionDir: ".gq/sessions",
	}
	
	// No sessions
	sess, err := Last(cfg)
	if err != nil {
		t.Fatalf("Last: %v", err)
	}
	if sess != nil {
		t.Errorf("expected nil, got %v", sess)
	}
	
	// Create sessions
	New(cfg, "old")
	New(cfg, "new")
	
	sess, err = Last(cfg)
	if err != nil {
		t.Fatalf("Last: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session, got nil")
	}
}