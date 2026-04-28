package watcher

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/emeraldwalk/agentdashboard/internal/conversation"
	"github.com/emeraldwalk/agentdashboard/internal/jsonl"
	"github.com/fsnotify/fsnotify"
)

// Handler receives conversation updates from any source.
type Handler interface {
	OnConversation(c conversation.Conversation)
}

// Watcher watches a directory for JSONL file changes and calls Handler.
type Watcher struct {
	root    string
	handler Handler
}

// New creates a Watcher for the given root directory.
func New(root string, handler Handler) (*Watcher, error) {
	return &Watcher{root: root, handler: handler}, nil
}

// Run scans existing files then watches for changes until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) error {
	// Initial scan of all existing JSONL files.
	matches, err := filepath.Glob(filepath.Join(w.root, "*.jsonl"))
	if err == nil {
		for _, path := range matches {
			w.processFile(path, "local")
		}
	}
	// Also scan subagent files.
	subMatches, err := filepath.Glob(filepath.Join(w.root, "*/subagents/*.jsonl"))
	if err == nil {
		for _, path := range subMatches {
			w.processFile(path, "local")
		}
	}

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fw.Close()

	if err := fw.Add(w.root); err != nil {
		// Root may not exist on this machine — degrade gracefully.
		log.Printf("watcher: cannot watch %s: %v", w.root, err)
		<-ctx.Done()
		return nil
	}

	// Watch every subdirectory for subagent files.
	subdirs, _ := filepath.Glob(filepath.Join(w.root, "*/subagents"))
	for _, d := range subdirs {
		_ = fw.Add(d)
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-fw.Events:
			if !ok {
				return nil
			}
			if strings.HasSuffix(event.Name, ".jsonl") &&
				(event.Op&(fsnotify.Create|fsnotify.Write)) != 0 {
				w.processFile(event.Name, "local")
			}
		case err, ok := <-fw.Errors:
			if !ok {
				return nil
			}
			log.Printf("watcher: fsnotify error: %v", err)
		case <-ticker.C:
			// Periodically re-add any new subdirs that appeared.
			subdirs, _ := filepath.Glob(filepath.Join(w.root, "*/subagents"))
			for _, d := range subdirs {
				_ = fw.Add(d)
			}
		}
	}
}

func (w *Watcher) processFile(path, project string) {
	records, err := jsonl.ParseFile(path)
	if err != nil || len(records) == 0 {
		return
	}

	sessionID := sessionIDFromPath(path)
	if sessionID == "" {
		return
	}

	// Extract title from ai-title records.
	var title string
	for _, r := range records {
		if r.Type == "ai-title" && r.AITitle != "" {
			title = r.AITitle
		}
	}

	startedAt := records[0].Timestamp
	lastEventAt := jsonl.DeriveLastEventAt(records)
	if lastEventAt.IsZero() {
		lastEventAt = startedAt
	}

	c := conversation.Conversation{
		ID:          sessionID,
		Project:     project,
		Title:       title,
		Status:      jsonl.DeriveStatus(records),
		StartedAt:   startedAt,
		LastEventAt: lastEventAt,
	}
	w.handler.OnConversation(c)
}

// sessionIDFromPath extracts the session/agent ID from a JSONL file path.
// For ~/.claude/projects/<uuid>.jsonl → <uuid>
// For ~/.claude/projects/<uuid>/subagents/agent-<id>.jsonl → agent-<id>
func sessionIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".jsonl")
}
