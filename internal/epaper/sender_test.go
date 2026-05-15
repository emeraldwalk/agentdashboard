package epaper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type fakeSummaryProvider struct {
	s SessionSummary
}

func (f *fakeSummaryProvider) Summary() SessionSummary { return f.s }

func newTestServer(t *testing.T, count *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
}

// TestSender_ThrottleCoalesces verifies that 10 rapid NotifyChange calls result in a
// single POST within MinInterval.
func TestSender_ThrottleCoalesces(t *testing.T) {
	var count atomic.Int32
	srv := newTestServer(t, &count)
	defer srv.Close()

	const minInterval = 200 * time.Millisecond
	provider := &fakeSummaryProvider{s: SessionSummary{ActiveSessions: 1}}
	sender := NewSender(SenderConfig{
		DeviceAddr:  srv.URL,
		MinInterval: minInterval,
		MaxInterval: 10 * time.Second,
	}, provider, Renderer{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sender.Start(ctx)

	// Fire 10 rapid notifications.
	for i := 0; i < 10; i++ {
		sender.NotifyChange()
	}

	// Wait just past MinInterval for the deferred send to fire.
	time.Sleep(minInterval + 100*time.Millisecond)

	got := count.Load()
	if got > 2 {
		t.Errorf("expected at most 2 POSTs (1 immediate + 1 deferred), got %d", got)
	}
	if got == 0 {
		t.Error("expected at least 1 POST, got 0")
	}
}

// TestSender_MaxInterval verifies a forced resend fires after MaxInterval with no NotifyChange.
func TestSender_MaxInterval(t *testing.T) {
	var count atomic.Int32
	srv := newTestServer(t, &count)
	defer srv.Close()

	const maxInterval = 200 * time.Millisecond
	provider := &fakeSummaryProvider{s: SessionSummary{ActiveSessions: 2}}
	sender := NewSender(SenderConfig{
		DeviceAddr:  srv.URL,
		MinInterval: 10 * time.Second,
		MaxInterval: maxInterval,
	}, provider, Renderer{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sender.Start(ctx)

	// Wait for two MaxInterval ticks.
	time.Sleep(maxInterval*2 + 100*time.Millisecond)

	got := count.Load()
	if got < 2 {
		t.Errorf("expected at least 2 forced POSTs from MaxInterval, got %d", got)
	}
}

// TestSender_SkipsIdenticalSummary verifies that a second send is skipped when the
// summary has not changed.
func TestSender_SkipsIdenticalSummary(t *testing.T) {
	var count atomic.Int32
	srv := newTestServer(t, &count)
	defer srv.Close()

	const minInterval = 50 * time.Millisecond
	provider := &fakeSummaryProvider{s: SessionSummary{ActiveSessions: 3}}
	sender := NewSender(SenderConfig{
		DeviceAddr:  srv.URL,
		MinInterval: minInterval,
		MaxInterval: 10 * time.Second,
	}, provider, Renderer{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sender.Start(ctx)

	// First notification — should send.
	sender.NotifyChange()
	time.Sleep(minInterval + 50*time.Millisecond)

	before := count.Load()
	if before == 0 {
		t.Fatal("expected first POST, got none")
	}

	// Second notification — summary unchanged, should be skipped.
	sender.NotifyChange()
	time.Sleep(minInterval + 50*time.Millisecond)

	after := count.Load()
	if after > before {
		t.Errorf("expected no additional POST for identical summary, got %d total (was %d)", after, before)
	}
}

// TestSender_DisabledWhenNoAddr verifies that no HTTP calls are made when DeviceAddr is empty.
func TestSender_DisabledWhenNoAddr(t *testing.T) {
	var count atomic.Int32
	srv := newTestServer(t, &count)
	defer srv.Close()

	provider := &fakeSummaryProvider{}
	sender := NewSender(SenderConfig{
		DeviceAddr:  "",
		MinInterval: 10 * time.Millisecond,
		MaxInterval: 20 * time.Millisecond,
	}, provider, Renderer{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sender.Start(ctx)

	sender.NotifyChange()
	time.Sleep(100 * time.Millisecond)

	if count.Load() != 0 {
		t.Errorf("expected 0 POSTs with empty DeviceAddr, got %d", count.Load())
	}
}
