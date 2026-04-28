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
	"strings"
	"syscall"

	"github.com/emeraldwalk/agentdashboard/internal/conversation"
	"github.com/emeraldwalk/agentdashboard/internal/dashboard"
	"github.com/emeraldwalk/agentdashboard/internal/docker"
	"github.com/emeraldwalk/agentdashboard/internal/watcher"
)

type ingestHandler struct {
	store  conversation.Store
	broker *dashboard.Broker
}

func (h *ingestHandler) OnConversation(c conversation.Conversation) {
	if err := h.store.Upsert(c); err != nil {
		log.Printf("upsert error: %v", err)
		return
	}
	data, _ := json.Marshal(c)
	h.broker.Publish(data)
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

	handler := &ingestHandler{store: store, broker: broker}

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
