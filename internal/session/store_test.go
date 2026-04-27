package session_test

import (
	"testing"
	"time"

	"github.com/emeraldwalk/agentdashboard/internal/session"
)

func TestNewSQLiteStore_InMemory(t *testing.T) {
	store, err := session.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()
}

func TestUpsertAndList_RoundTrip(t *testing.T) {
	store, err := session.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)

	sess := session.Session{
		ID:          "abc-123",
		AgentName:   "test-agent",
		Status:      session.StatusRunning,
		StartedAt:   now,
		LastEventAt: now,
	}

	if err := store.Upsert(sess); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}

	got := list[0]
	if got.ID != sess.ID {
		t.Errorf("ID: got %q, want %q", got.ID, sess.ID)
	}
	if got.AgentName != sess.AgentName {
		t.Errorf("AgentName: got %q, want %q", got.AgentName, sess.AgentName)
	}
	if got.Status != session.StatusRunning {
		t.Errorf("Status: got %q, want %q", got.Status, session.StatusRunning)
	}
	if !got.StartedAt.Equal(sess.StartedAt) {
		t.Errorf("StartedAt: got %v, want %v", got.StartedAt, sess.StartedAt)
	}
}

func TestUpsert_PreservesStartedAt(t *testing.T) {
	store, err := session.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	originalStart := time.Now().UTC().Truncate(time.Second)
	later := originalStart.Add(5 * time.Minute)

	first := session.Session{
		ID:          "session-1",
		AgentName:   "agent",
		Status:      session.StatusRunning,
		StartedAt:   originalStart,
		LastEventAt: originalStart,
	}

	if err := store.Upsert(first); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}

	// Second upsert with a later started_at and a different status.
	second := session.Session{
		ID:          "session-1",
		AgentName:   "agent",
		Status:      session.StatusStopped,
		StartedAt:   later, // should be ignored
		LastEventAt: later,
	}

	if err := store.Upsert(second); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}

	got := list[0]
	if got.Status != session.StatusStopped {
		t.Errorf("Status: got %q, want %q", got.Status, session.StatusStopped)
	}
	if !got.StartedAt.Equal(originalStart) {
		t.Errorf("StartedAt was overwritten: got %v, want %v", got.StartedAt, originalStart)
	}
	if !got.LastEventAt.Equal(later) {
		t.Errorf("LastEventAt: got %v, want %v", got.LastEventAt, later)
	}
}

func TestList_OrderByLastEventAtDesc(t *testing.T) {
	store, err := session.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	base := time.Now().UTC().Truncate(time.Second)

	sessions := []session.Session{
		{ID: "a", AgentName: "agent", Status: session.StatusRunning, StartedAt: base, LastEventAt: base},
		{ID: "b", AgentName: "agent", Status: session.StatusRunning, StartedAt: base, LastEventAt: base.Add(2 * time.Minute)},
		{ID: "c", AgentName: "agent", Status: session.StatusRunning, StartedAt: base, LastEventAt: base.Add(1 * time.Minute)},
	}

	for _, s := range sessions {
		if err := store.Upsert(s); err != nil {
			t.Fatalf("Upsert %s: %v", s.ID, err)
		}
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(list))
	}

	// Expect order: b (latest), c, a (oldest)
	expectedOrder := []string{"b", "c", "a"}
	for i, id := range expectedOrder {
		if list[i].ID != id {
			t.Errorf("position %d: got %q, want %q", i, list[i].ID, id)
		}
	}
}
