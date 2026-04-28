package jsonl

import "time"

// Record is the common envelope for all JSONL log lines emitted by Claude Code.
type Record struct {
	Type      string    `json:"type"`
	SessionID string    `json:"sessionId"`
	Timestamp time.Time `json:"timestamp"`

	// type == "user" or "assistant"
	UUID       string   `json:"uuid"`
	ParentUUID string   `json:"parentUuid"`
	PromptID   string   `json:"promptId"`
	Message    *Message `json:"message,omitempty"`

	// type == "ai-title" — note field name is "aiTitle" not "title"
	AITitle string `json:"aiTitle,omitempty"`

	// type == "queue-operation"
	Operation string `json:"operation,omitempty"` // "enqueue" | "dequeue"
}

type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []ContentBlock
}
