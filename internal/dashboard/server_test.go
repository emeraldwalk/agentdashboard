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

	"github.com/emeraldwalk/agentdashboard/internal/conversation"
)

// mockStore implements conversation.Store for testing.
type mockStore struct {
	convs []conversation.Conversation
}

func (m *mockStore) Upsert(c conversation.Conversation) error         { return nil }
func (m *mockStore) Close() error                                      { return nil }
func (m *mockStore) List() ([]conversation.Conversation, error)        { return m.convs, nil }

func TestHandleConversations(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	store := &mockStore{
		convs: []conversation.Conversation{
			{
				ID:          "conv-1",
				Project:     "agentdashboard",
				Title:       "Fix JSONL parser",
				Status:      conversation.StatusRunning,
				StartedAt:   now,
				LastEventAt: now,
			},
		},
	}

	broker := NewBroker()
	srv := NewServer(store, broker, ":0")

	req := httptest.NewRequest("GET", "/api/conversations", nil)
	w := httptest.NewRecorder()

	srv.handleConversations(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	ct := res.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected application/json content type, got %q", ct)
	}

	var got []conversation.Conversation
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(got))
	}

	c := got[0]
	if c.ID != "conv-1" {
		t.Errorf("ID: got %q, want %q", c.ID, "conv-1")
	}
	if c.Project != "agentdashboard" {
		t.Errorf("Project: got %q, want %q", c.Project, "agentdashboard")
	}
	if c.Status != conversation.StatusRunning {
		t.Errorf("Status: got %q, want %q", c.Status, conversation.StatusRunning)
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

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		srv.handleEvents(w, req)
	}()

	time.Sleep(50 * time.Millisecond)

	payload := `{"id":"conv-1","project":"agentdashboard","title":"Fix JSONL parser","status":"running","startedAt":"2026-04-26T12:00:00Z","lastEventAt":"2026-04-26T12:00:00Z"}`
	go broker.Publish([]byte(payload))

	select {
	case <-w.flushed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE flush")
	}

	reqCancel()
	<-handlerDone

	body := w.Body.String()
	scanner := bufio.NewScanner(strings.NewReader(body))
	var foundEvent, foundData bool
	for scanner.Scan() {
		line := scanner.Text()
		if line == "event: conversation-update" {
			foundEvent = true
		}
		if line == "data: "+payload {
			foundData = true
		}
	}

	if !foundEvent {
		t.Errorf("SSE body missing 'event: conversation-update' line; body was:\n%s", body)
	}
	if !foundData {
		t.Errorf("SSE body missing expected data line; body was:\n%s", body)
	}
}
