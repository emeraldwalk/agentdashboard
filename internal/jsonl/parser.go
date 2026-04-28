package jsonl

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/emeraldwalk/agentdashboard/internal/conversation"
)

// Parse reads all complete JSON lines from r and returns them as Records.
// Incomplete trailing lines (no newline) are not returned.
func Parse(r io.Reader) ([]Record, error) {
	var records []Record
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			continue // skip malformed lines
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

// ParseFile reads and parses a complete JSONL file at path.
func ParseFile(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// DeriveStatus returns the conversation status from an ordered slice of records (oldest first).
func DeriveStatus(records []Record) conversation.Status {
	var lastType string
	var lastDequeue time.Time

	for _, r := range records {
		switch r.Type {
		case "user":
			lastType = "user"
		case "assistant":
			lastType = "assistant"
		case "queue-operation":
			if r.Operation == "dequeue" {
				lastDequeue = r.Timestamp
			}
		case "internal_error":
			if lastType == "" {
				lastType = "internal_error"
			}
		}
	}

	if lastType == "user" {
		return conversation.StatusWaiting
	}
	if lastType == "assistant" {
		if !lastDequeue.IsZero() && time.Since(lastDequeue) < 30*time.Second {
			return conversation.StatusRunning
		}
		return conversation.StatusStopped
	}
	if !lastDequeue.IsZero() && time.Since(lastDequeue) < 30*time.Second {
		return conversation.StatusRunning
	}
	if lastType == "internal_error" {
		return conversation.StatusFailed
	}
	return conversation.StatusStopped
}

// DeriveLastEventAt returns the timestamp of the most recent actionable record.
func DeriveLastEventAt(records []Record) time.Time {
	var t time.Time
	for _, r := range records {
		switch r.Type {
		case "user", "assistant", "queue-operation":
			if r.Timestamp.After(t) {
				t = r.Timestamp
			}
		}
	}
	return t
}
