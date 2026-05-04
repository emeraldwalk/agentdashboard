package jsonl

import (
	"encoding/json"
	"time"
)

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
	Role    string           `json:"role"`
	Content json.RawMessage  `json:"content"` // string or []ContentBlock
}

type ContentBlock struct {
	Type string `json:"type"`
}

// IsHumanTurn returns true when the user record represents a human typing a
// message (first content block is "text"), as opposed to a tool result
// returned by the agent (first content block is "tool_result").
func (r Record) IsHumanTurn() bool {
	if r.Type != "user" || r.Message == nil {
		return false
	}
	var blocks []ContentBlock
	if err := json.Unmarshal(r.Message.Content, &blocks); err != nil || len(blocks) == 0 {
		return false
	}
	return blocks[0].Type == "text"
}
