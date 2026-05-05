package watcher

import (
	"context"
	"log"
	"os"
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
	// Initial scan: <root>/<project>/*.jsonl
	projMatches, err := filepath.Glob(filepath.Join(w.root, "*", "*.jsonl"))
	if err == nil {
		for _, path := range projMatches {
			w.processFile(path)
		}
	}
	// Initial scan: <root>/<project>/subagents/*.jsonl
	subMatches, err := filepath.Glob(filepath.Join(w.root, "*", "subagents", "*.jsonl"))
	if err == nil {
		for _, path := range subMatches {
			w.processFile(path)
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

	// Watch every project subdirectory and its subagents dir.
	w.addSubdirs(fw)

	ticker := time.NewTicker(10 * time.Second)
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
				w.processFile(event.Name)
			}
		case err, ok := <-fw.Errors:
			if !ok {
				return nil
			}
			log.Printf("watcher: fsnotify error: %v", err)
		case <-ticker.C:
			// Periodically re-add any new subdirs and re-scan all files to
			// correct stale "running" statuses when no fsnotify event fires.
			w.addSubdirs(fw)
			w.scanAll()
		}
	}
}

func (w *Watcher) scanAll() {
	if matches, err := filepath.Glob(filepath.Join(w.root, "*", "*.jsonl")); err == nil {
		for _, path := range matches {
			w.processFile(path)
		}
	}
	if matches, err := filepath.Glob(filepath.Join(w.root, "*", "subagents", "*.jsonl")); err == nil {
		for _, path := range matches {
			w.processFile(path)
		}
	}
}

func (w *Watcher) addSubdirs(fw *fsnotify.Watcher) {
	projDirs, _ := filepath.Glob(filepath.Join(w.root, "*"))
	for _, d := range projDirs {
		_ = fw.Add(d)
		_ = fw.Add(filepath.Join(d, "subagents"))
	}
}

func (w *Watcher) processFile(path string) {
	records, err := jsonl.ParseFile(path)
	if err != nil || len(records) == 0 {
		return
	}

	sessionID := sessionIDFromPath(path)
	if sessionID == "" {
		return
	}

	project := projectFromPath(w.root, path)

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
		Source:      conversation.SourceHost,
		StartedAt:   startedAt,
		LastEventAt: lastEventAt,
	}
	w.handler.OnConversation(c)
}

// projectFromPath returns a human-readable project name for a JSONL file.
// Claude encodes the working directory path as the project dir name by replacing
// '/' with '-', producing e.g. "-Users-bingles-code-tools-agentdashboard".
// We strip the leading '-' then strip the encoded home directory prefix so that
// only the path relative to home remains, e.g. "code-tools-agentdashboard".
func projectFromPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	dir := strings.SplitN(rel, string(filepath.Separator), 2)[0]

	// Strip leading '-' produced by the leading '/' in the absolute path.
	dir = strings.TrimPrefix(dir, "-")

	// Encode the home directory the same way Claude does (replace '/' with '-')
	// and strip it from the front of dir.
	if home, err := os.UserHomeDir(); err == nil {
		encodedHome := strings.TrimPrefix(strings.ReplaceAll(home, "/", "-"), "-")
		dir = strings.TrimPrefix(dir, encodedHome)
		dir = strings.TrimLeft(dir, "-")
	}

	return dir
}

// sessionIDFromPath extracts the session/agent ID from a JSONL file path.
func sessionIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".jsonl")
}
