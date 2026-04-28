package dashboard

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emeraldwalk/agentdashboard/internal/session"
)

// mockStore implements session.Store for testing.
type mockStore struct {
	sessions []session.Session
}

func (m *mockStore) Upsert(s session.Session) error                          { return nil }
func (m *mockStore) Close() error                                             { return nil }
func (m *mockStore) List() ([]session.Session, error)                        { return m.sessions, nil }
func (m *mockStore) AppendRawEvent(signal, payload string) error              { return nil }
func (m *mockStore) ListRawEvents(signal string) ([]session.RawEvent, error) { return nil, nil }

func TestHandleSessions(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	store := &mockStore{
		sessions: []session.Session{
			{
				ID:          "sess-1",
				AgentName:   "test-agent",
				Status:      session.StatusRunning,
				StartedAt:   now,
				LastEventAt: now,
			},
		},
	}

	broker := NewBroker()
	srv := NewServer(store, broker, ":0")

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	w := httptest.NewRecorder()

	srv.handleSessions(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	ct := res.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected application/json content type, got %q", ct)
	}

	var dtos []sessionDTO
	if err := json.NewDecoder(res.Body).Decode(&dtos); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(dtos) != 1 {
		t.Fatalf("expected 1 session, got %d", len(dtos))
	}

	got := dtos[0]
	if got.ID != "sess-1" {
		t.Errorf("ID: got %q, want %q", got.ID, "sess-1")
	}
	if got.AgentName != "test-agent" {
		t.Errorf("AgentName: got %q, want %q", got.AgentName, "test-agent")
	}
	if got.Status != "running" {
		t.Errorf("Status: got %q, want %q", got.Status, "running")
	}
}

// sseRecorder is an httptest.ResponseRecorder that also supports http.Flusher.
type sseRecorder struct {
	*httptest.ResponseRecorder
	flushed chan struct{}
}

func newSSERecorder() *sseRecorder {
	return &sseRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		flushed:          make(chan struct{}, 16),
	}
}

func (r *sseRecorder) Flush() {
	r.ResponseRecorder.Flush()
	select {
	case r.flushed <- struct{}{}:
	default:
	}
}

func TestHandleEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := &mockStore{}
	broker := NewBroker()
	go broker.Run(ctx)

	srv := NewServer(store, broker, ":0")

	w := newSSERecorder()

	reqCtx, reqCancel := context.WithCancel(ctx)
	req := httptest.NewRequest("GET", "/api/events", nil).WithContext(reqCtx)

	// Run the SSE handler in a goroutine.
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		srv.handleEvents(w, req)
	}()

	// Wait for the handler to subscribe (it needs time to send on broker.subscribe channel).
	time.Sleep(50 * time.Millisecond)

	// Publish a message.
	payload := `{"id":"sess-1","agentName":"test-agent","status":"running","lastEventAt":"2026-04-26T12:00:00Z"}`
	go broker.Publish([]byte(payload))

	// Wait for a flush to confirm the event was written.
	select {
	case <-w.flushed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE flush")
	}

	// Stop the handler.
	reqCancel()
	<-handlerDone

	// Inspect the written body for the SSE event.
	body := w.Body.String()
	scanner := bufio.NewScanner(strings.NewReader(body))
	var foundEvent, foundData bool
	for scanner.Scan() {
		line := scanner.Text()
		if line == "event: session-update" {
			foundEvent = true
		}
		if line == "data: "+payload {
			foundData = true
		}
	}

	if !foundEvent {
		t.Errorf("SSE body missing 'event: session-update' line; body was:\n%s", body)
	}
	if !foundData {
		t.Errorf("SSE body missing expected data line; body was:\n%s", body)
	}
}
