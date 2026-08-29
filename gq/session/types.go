package session

import (
	"time"
)

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Session represents a conversation session.
type Session struct {
	ID       string    `json:"id"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
	Messages []Message `json:"messages"`
}