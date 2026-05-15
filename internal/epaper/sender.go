package epaper

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"time"
)

// SummaryProvider is implemented by whatever builds a SessionSummary from live state.
type SummaryProvider interface {
	Summary() SessionSummary
}

// SenderConfig holds tuning parameters for Sender.
type SenderConfig struct {
	DeviceAddr  string        // e.g. "http://192.168.1.50" — empty disables the sender
	MinInterval time.Duration // minimum time between sends (default 60s)
	MaxInterval time.Duration // force resend even without a change (default 5m)
}

// Sender watches for session changes, throttles, and POSTs PNG images to the ePaper device.
type Sender struct {
	cfg      SenderConfig
	provider SummaryProvider
	renderer Renderer
	notify   chan struct{}
	client   *http.Client
}

// NewSender creates a Sender. If cfg.DeviceAddr is empty the Sender is a no-op.
func NewSender(cfg SenderConfig, provider SummaryProvider, r Renderer) *Sender {
	if cfg.MinInterval <= 0 {
		cfg.MinInterval = 60 * time.Second
	}
	if cfg.MaxInterval <= 0 {
		cfg.MaxInterval = 5 * time.Minute
	}
	return &Sender{
		cfg:      cfg,
		provider: provider,
		renderer: r,
		notify:   make(chan struct{}, 1),
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// NotifyChange signals that session state may have changed.
// Non-blocking; rapid calls within MinInterval are coalesced.
func (s *Sender) NotifyChange() {
	if s.cfg.DeviceAddr == "" {
		return
	}
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// Start begins the send loop. Blocks until ctx is cancelled.
func (s *Sender) Start(ctx context.Context) {
	if s.cfg.DeviceAddr == "" {
		return
	}

	var (
		lastSent    time.Time
		lastSummary SessionSummary
		scheduled   <-chan time.Time // deferred send timer
	)

	maxTicker := time.NewTicker(s.cfg.MaxInterval)
	defer maxTicker.Stop()

	trySend := func(forced bool) {
		summary := s.provider.Summary()
		if !forced && summaryEqual(summary, lastSummary) {
			return
		}
		img := s.renderer.Render(summary)
		data, err := EncodePNG(img)
		if err != nil {
			log.Printf("epaper: encode error: %v", err)
			return
		}
		url := s.cfg.DeviceAddr + "/image"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			log.Printf("epaper: build request error: %v", err)
			return
		}
		req.Header.Set("Content-Type", "image/png")
		resp, err := s.client.Do(req)
		if err != nil {
			log.Printf("epaper: POST error: %v", err)
			return
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Printf("epaper: device returned %d", resp.StatusCode)
			return
		}
		lastSent = time.Now()
		lastSummary = summary
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-maxTicker.C:
			trySend(true)
			scheduled = nil

		case <-s.notify:
			now := time.Now()
			if lastSent.IsZero() || now.Sub(lastSent) >= s.cfg.MinInterval {
				trySend(false)
				scheduled = nil
			} else {
				// Schedule a deferred send at the earliest allowed time.
				delay := s.cfg.MinInterval - now.Sub(lastSent)
				scheduled = time.After(delay)
			}

		case <-scheduledChan(scheduled):
			scheduled = nil
			trySend(false)
		}
	}
}

func summaryEqual(a, b SessionSummary) bool {
	if a.ActiveSessions != b.ActiveSessions ||
		a.PendingSessions != b.PendingSessions ||
		a.DoneSessions != b.DoneSessions ||
		len(a.RecentProjects) != len(b.RecentProjects) {
		return false
	}
	for i := range a.RecentProjects {
		if a.RecentProjects[i] != b.RecentProjects[i] {
			return false
		}
	}
	return true
}

// scheduledChan returns the channel, handling nil (never fires).
func scheduledChan(ch <-chan time.Time) <-chan time.Time {
	if ch == nil {
		return make(chan time.Time) // never fires
	}
	return ch
}
