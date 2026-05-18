package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/emeraldwalk/agentdashboard/internal/conversation"
	"github.com/emeraldwalk/agentdashboard/internal/dashboard"
	"github.com/emeraldwalk/agentdashboard/internal/docker"
	"github.com/emeraldwalk/agentdashboard/internal/epaper"
	"github.com/emeraldwalk/agentdashboard/internal/watcher"
)

type ingestHandler struct {
	ctx    context.Context
	store  conversation.Store
	broker *dashboard.Broker
	cache  map[string]conversation.Conversation
	notify func()
}

func (h *ingestHandler) OnConversation(c conversation.Conversation) {
	if prev, ok := h.cache[c.ID]; ok && prev == c {
		return
	}
	h.cache[c.ID] = c
	if err := h.store.Upsert(c); err != nil {
		log.Printf("upsert error: %v", err)
		return
	}
	data, _ := json.Marshal(c)
	h.broker.Publish(h.ctx, data)
	if h.notify != nil {
		h.notify()
	}
}

// conversationSummaryProvider implements epaper.SummaryProvider using the conversation store.
type conversationSummaryProvider struct {
	store conversation.Store
}

func (p *conversationSummaryProvider) Summary() epaper.SessionSummary {
	convs, err := p.store.List()
	if err != nil {
		return epaper.SessionSummary{}
	}
	now := time.Now()
	const archiveAge = 48 * time.Hour

	// Track the most recent LastEventAt per project per column.
	// A project can appear in multiple columns (e.g. has both done and archived conversations).
	type projectTime = map[string]time.Time
	pendingProjects := make(projectTime)
	doneProjects := make(projectTime)
	archivedProjects := make(projectTime)

	for _, c := range convs {
		if c.IsSubagent {
			continue
		}
		var col projectTime
		if now.Sub(c.LastEventAt) >= archiveAge {
			col = archivedProjects
		} else if c.Status == conversation.StatusRunning || c.Status == conversation.StatusWaiting {
			col = pendingProjects
		} else {
			col = doneProjects
		}
		if t, ok := col[c.Project]; !ok || c.LastEventAt.After(t) {
			col[c.Project] = c.LastEventAt
		}
	}

	var summary epaper.SessionSummary
	summary.PendingSessions = len(pendingProjects)
	summary.DoneSessions = len(doneProjects)
	summary.ArchivedSessions = len(archivedProjects)

	summary.PendingProjects = projectsByRecency(pendingProjects, 8)
	summary.DoneProjects = projectsByRecency(doneProjects, 8)
	summary.ArchivedProjects = projectsByRecency(archivedProjects, 8)
	return summary
}

func projectsByRecency(m map[string]time.Time, limit int) []string {
	type entry struct {
		name string
		t    time.Time
	}
	entries := make([]entry, 0, len(m))
	for name, t := range m {
		entries = append(entries, entry{name, t})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].t.After(entries[j].t)
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	return names
}

func expandHome(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func main() {
	dbFlag := flag.String("db", "~/.agentdashboard/sessions.db", "SQLite database file path")
	dashboardAddr := flag.String("dashboard-addr", ":8080", "Dashboard HTTP listen address")
	claudeDir := flag.String("claude-dir", "~/.claude/projects", "Path to host Claude projects directory")
	dockerSocket := flag.String("docker-socket", "/var/run/docker.sock", "Docker socket path")
	epaperAddr := flag.String("epaper-addr", "", "ePaper device address (e.g. http://192.168.1.50); omit to disable")
	flag.Parse()

	dbPath, err := expandHome(*dbFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get home directory: %v\n", err)
		os.Exit(1)
	}

	claudePath, err := expandHome(*claudeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to expand claude-dir: %v\n", err)
		os.Exit(1)
	}

	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create directory %s: %v\n", dbDir, err)
		os.Exit(1)
	}

	store, err := conversation.NewSQLiteStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open store: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	broker := dashboard.NewBroker()
	go broker.Run(ctx)

	handler := &ingestHandler{ctx: ctx, store: store, broker: broker, cache: make(map[string]conversation.Conversation)}

	if *epaperAddr != "" {
		sender := epaper.NewSender(epaper.SenderConfig{
			DeviceAddr:  *epaperAddr,
			MinInterval: 10 * time.Second,
			MaxInterval: 1 * time.Minute,
		}, &conversationSummaryProvider{store: store}, epaper.Renderer{})
		go sender.Start(ctx)
		handler.notify = sender.NotifyChange
	}

	// Start host filesystem watcher.
	w, err := watcher.New(claudePath, handler)
	if err != nil {
		log.Printf("watcher init error: %v", err)
	} else {
		go func() {
			if err := w.Run(ctx); err != nil {
				log.Printf("watcher error: %v", err)
			}
		}()
	}

	// Start Docker source if socket is available.
	if _, err := os.Stat(*dockerSocket); err == nil {
		src := docker.New(*dockerSocket, handler)
		go func() {
			if err := src.Run(ctx); err != nil {
				log.Printf("docker source error: %v", err)
			}
		}()
	} else {
		log.Printf("docker socket %s not available, skipping Docker discovery", *dockerSocket)
	}

	server := dashboard.NewServer(store, broker, *dashboardAddr)
	go func() {
		if err := server.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "dashboard server error: %v\n", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	cancel()

	if err := store.Close(); err != nil {
		log.Printf("error closing store: %v", err)
	}
	log.Println("shutting down")
}
