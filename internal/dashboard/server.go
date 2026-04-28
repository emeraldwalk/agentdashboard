package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/emeraldwalk/agentdashboard/internal/conversation"
)

//go:embed all:dist
var frontendFS embed.FS

// Server serves the dashboard HTTP API and SPA.
type Server struct {
	store  conversation.Store
	broker *Broker
	addr   string
}

// NewServer creates a new Server.
func NewServer(store conversation.Store, broker *Broker, addr string) *Server {
	return &Server{
		store:  store,
		broker: broker,
		addr:   addr,
	}
}

// Start registers routes and serves until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/conversations", s.handleConversations)
	mux.HandleFunc("GET /api/events", s.handleEvents)

	assetsFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		return err
	}
	mux.Handle("GET /assets/", http.FileServerFS(assetsFS))
	mux.HandleFunc("GET /", s.handleSPA)

	srv := &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background()) //nolint:errcheck
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	convs, err := s.store.List()
	if err != nil {
		http.Error(w, "failed to list conversations", http.StatusInternalServerError)
		return
	}

	if convs == nil {
		convs = []conversation.Conversation{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(convs) //nolint:errcheck
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := s.broker.Subscribe()
	defer s.broker.Unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			_, err := w.Write([]byte("event: conversation-update\ndata: " + string(data) + "\n\n"))
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	indexHTML, err := fs.ReadFile(frontendFS, "dist/index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML) //nolint:errcheck
}
