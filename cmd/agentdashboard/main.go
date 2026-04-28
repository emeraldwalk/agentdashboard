package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/emeraldwalk/agentdashboard/internal/dashboard"
	"github.com/emeraldwalk/agentdashboard/internal/otlp"
	"github.com/emeraldwalk/agentdashboard/internal/session"
)

func main() {
	dbFlag := flag.String("db", "~/.agentdashboard/sessions.db", "SQLite database file path")
	otlpAddr := flag.String("otlp-addr", ":4318", "OTLP HTTP listen address")
	dashboardAddr := flag.String("dashboard-addr", ":8080", "Dashboard HTTP listen address")
	flag.Parse()

	// Expand ~ to home directory
	dbPath := *dbFlag
	if strings.HasPrefix(dbPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to get home directory: %v\n", err)
			os.Exit(1)
		}
		dbPath = filepath.Join(home, dbPath[2:])
	}

	// Create directory if it does not exist
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create directory %s: %v\n", dbDir, err)
		os.Exit(1)
	}

	// Open SQLite store
	store, err := session.NewSQLiteStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open store: %v\n", err)
		os.Exit(1)
	}

	// Set up root context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create and start broker
	broker := dashboard.NewBroker()
	go broker.Run(ctx)

	// Create OTLP handler
	otlpHandler := otlp.NewHandler(store, store, broker)
	otlpMux := http.NewServeMux()
	otlpHandler.RegisterRoutes(otlpMux)

	// Create dashboard server
	server := dashboard.NewServer(store, broker, *dashboardAddr)

	// Start OTLP HTTP server
	go func() {
		if err := http.ListenAndServe(*otlpAddr, otlpMux); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "OTLP server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Start dashboard HTTP server
	go func() {
		if err := server.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "dashboard server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	cancel()

	if err := store.Close(); err != nil {
		log.Printf("error closing store: %v", err)
	}
	log.Println("shutting down")
}
