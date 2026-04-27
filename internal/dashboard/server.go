package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"time"

	"github.com/emeraldwalk/agentdashboard/internal/session"
)

//go:embed all:dist
var frontendFS embed.FS

// sessionDTO is a JSON-friendly representation of session.Session with camelCase keys.
type sessionDTO struct {
	ID          string    `json:"id"`
	AgentName   string    `json:"agentName"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"startedAt"`
	LastEventAt time.Time `json:"lastEventAt"`
}

func toDTO(s session.Session) sessionDTO {
	return sessionDTO{
		ID:          s.ID,
		AgentName:   s.AgentName,
		Status:      string(s.Status),
		StartedAt:   s.StartedAt,
		LastEventAt: s.LastEventAt,
	}
}

// Server serves the dashboard HTTP API and SPA.
type Server struct {
	store  session.Store
	broker *Broker
	addr   string
}

// NewServer creates a new Server.
func NewServer(store session.Store, broker *Broker, addr string) *Server {
	return &Server{
		store:  store,
		broker: broker,
		addr:   addr,
	}
}

// Start registers routes and serves until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// /api/sessions — JSON array of sessions
	mux.HandleFunc("GET /api/sessions", s.handleSessions)

	// /api/events — SSE stream
	mux.HandleFunc("GET /api/events", s.handleEvents)

	// /assets/ — static files from embedded dist/assets/
	assetsFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		return err
	}
	mux.Handle("GET /assets/", http.FileServerFS(assetsFS))

	// Catch-all — serve SPA index.html
	mux.HandleFunc("GET /", s.handleSPA)

	srv := &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	// Shut down when ctx is cancelled.
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background()) //nolint:errcheck
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.store.List()
	if err != nil {
		http.Error(w, "failed to list sessions", http.StatusInternalServerError)
		return
	}

	dtos := make([]sessionDTO, len(sessions))
	for i, sess := range sessions {
		dtos[i] = toDTO(sess)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dtos) //nolint:errcheck
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
			_, err := w.Write([]byte("event: session-update\ndata: " + string(data) + "\n\n"))
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
