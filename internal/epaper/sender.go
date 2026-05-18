package epaper

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
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
	MaxInterval time.Duration // resend if changed but not yet sent (default 1m)
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
		cfg.MinInterval = 10 * time.Second
	}
	if cfg.MaxInterval <= 0 {
		cfg.MaxInterval = 1 * time.Minute
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

	// Align the time ticker to the next whole minute on the wall clock.
	now := time.Now()
	timeAlign := time.NewTimer(now.Truncate(time.Minute).Add(time.Minute).Sub(now))
	var timeTicker *time.Ticker
	defer func() {
		timeAlign.Stop()
		if timeTicker != nil {
			timeTicker.Stop()
		}
	}()

	trySend := func(forced bool) {
		summary := s.provider.Summary()
		if !forced && summaryEqual(summary, lastSummary) {
			return
		}
		img := s.renderer.Render(summary)
		data, err := EncodePNG(Dither(img))
		if err != nil {
			log.Printf("epaper: encode error: %v", err)
			return
		}
		const imgPath = "epaper-images/latest.png"
		_ = os.MkdirAll("epaper-images", 0o755)
		if err := os.WriteFile(imgPath, data, 0o644); err != nil {
			log.Printf("epaper: save image: %v", err)
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
			trySend(false)
			scheduled = nil

		case <-timeAlign.C:
			s.sendTimePatch(ctx)
			timeTicker = time.NewTicker(time.Minute)

		case <-tickerChan(timeTicker):
			s.sendTimePatch(ctx)

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

func (s *Sender) sendTimePatch(ctx context.Context) {
	img := s.renderer.RenderTimePatch()
	data, err := EncodePNG(Dither(img))
	if err != nil {
		log.Printf("epaper: time patch encode: %v", err)
		return
	}
	url := s.cfg.DeviceAddr + "/image"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		log.Printf("epaper: time patch request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("X-Position-X", strconv.Itoa(TimePatchX))
	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("epaper: time patch POST: %v", err)
		return
	}
	resp.Body.Close()
}

func summaryEqual(a, b SessionSummary) bool {
	if a.PendingSessions != b.PendingSessions ||
		a.DoneSessions != b.DoneSessions ||
		a.ArchivedSessions != b.ArchivedSessions {
		return false
	}
	for _, pair := range [][2][]string{
		{a.PendingProjects, b.PendingProjects},
		{a.DoneProjects, b.DoneProjects},
		{a.ArchivedProjects, b.ArchivedProjects},
	} {
		if len(pair[0]) != len(pair[1]) {
			return false
		}
		for i := range pair[0] {
			if pair[0][i] != pair[1][i] {
				return false
			}
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

// tickerChan returns the ticker's channel, handling nil (never fires).
func tickerChan(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return make(chan time.Time) // never fires
	}
	return t.C
}
