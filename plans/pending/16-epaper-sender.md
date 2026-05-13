# Plan 16 — ePaper Sender (Go)

## Checklist

- [ ] Create `internal/epaper/sender.go`
- [ ] Implement `Sender` with throttle + change detection
- [ ] Wire `Sender` into the existing session change notification path
- [ ] Add `--epaper-addr` CLI flag to `cmd/agentdashboard/main.go`
- [ ] Write unit tests for throttle logic
- [ ] Document config in `--help` output

## Context

This plan wires the image generator (Plan 15) to the firmware endpoint (Plan 14). It watches for session state changes, throttles sends to respect ePaper refresh limits, and POSTs the generated PNG to the device.

Depends on Plan 15 (`internal/epaper` render + encode) and Plan 14 (device endpoint).

## Existing touchpoints

| File | Role |
|---|---|
| `cmd/agentdashboard/main.go` | Add `--epaper-addr` flag; construct and start `Sender` |
| `internal/dashboard/server.go` | Source of session state; `Sender` reads from the same store |
| `internal/session/store.go` | `Store.All()` or equivalent — used to build `SessionSummary` |
| `internal/epaper/render.go` | `Renderer.Render()` — called by `Sender` on state change |
| `internal/epaper/encode.go` | `EncodePNG()` — called by `Sender` before POST |

## Package layout

New file only:

```
internal/epaper/
└── sender.go        # Sender: throttle, change detect, HTTP POST
    sender_test.go
```

## Contracts

### `Sender`

```go
// internal/epaper/sender.go

type SenderConfig struct {
    DeviceAddr  string        // e.g. "http://192.168.1.50" or "http://reterminal.local"
    MinInterval time.Duration // minimum time between sends (default 60s)
    MaxInterval time.Duration // force resend even if no change (default 5m)
}

type Sender struct { /* unexported fields */ }

func NewSender(cfg SenderConfig, store *session.Store, r Renderer) *Sender

// Start begins the send loop. Blocks until ctx is cancelled.
func (s *Sender) Start(ctx context.Context)

// NotifyChange signals that session state may have changed.
// Non-blocking; coalesces rapid calls within MinInterval.
func (s *Sender) NotifyChange()
```

### Send loop behavior

```
on NotifyChange():
    if now - lastSent >= MinInterval:
        send immediately
    else:
        schedule send at lastSent + MinInterval (coalesce multiple calls)

every MaxInterval:
    send regardless of change (keeps display from going stale)

on send:
    summary = store.Summary()
    if summary == lastSummary and not forced:
        skip
    img = renderer.Render(summary)
    body = EncodePNG(img)
    POST DeviceAddr/image with body
    lastSent = now
    lastSummary = summary
```

### CLI flag

```
--epaper-addr string   ePaper device address (e.g. http://192.168.1.50); omit to disable
```

If `--epaper-addr` is empty, `Sender` is not started. No ePaper-related code runs.

### POST request

```
POST <DeviceAddr>/image
Content-Type: image/png
Body: PNG bytes from EncodePNG()
```

On non-200 response: log error, do not retry immediately. Normal throttle applies to next attempt.
On network error: log error, same behavior.

### `session.Store` addition

Add to `internal/session/store.go` (or a new method file):

```go
// Summary returns aggregate counts for the ePaper renderer.
func (s *Store) Summary() epaper.SessionSummary
```

Counts active/pending/done sessions and returns the 5 most recently updated project names.

**Concept boundary:** `SessionSummary` lives in `internal/epaper` (not `internal/session`) to keep the dependency direction correct — `session` should not import `epaper`. `Store.Summary()` returns `epaper.SessionSummary` directly, so `session` imports `epaper`. If that feels wrong, use a plain struct in `session` and adapt in `sender.go` — implementer's call.

## Wiring in `main.go`

```go
if cfg.EpaperAddr != "" {
    sender := epaper.NewSender(epaper.SenderConfig{
        DeviceAddr:  cfg.EpaperAddr,
        MinInterval: 60 * time.Second,
        MaxInterval: 5 * time.Minute,
    }, store, epaper.Renderer{})
    go sender.Start(ctx)
    // pass sender.NotifyChange to wherever session updates are broadcast
}
```

`NotifyChange` should be called wherever sessions are updated — the same place that currently triggers SSE fan-out.

## Testing

`sender_test.go`:
- Throttle: call `NotifyChange` 10× rapidly → assert HTTP POST called exactly once within `MinInterval`.
- MaxInterval: advance mock clock past `MaxInterval` with no `NotifyChange` → assert POST fired.
- No-op on identical summary: send fires, summary unchanged → assert second send skipped.
- Disabled: empty `DeviceAddr` → assert no HTTP calls made.

Use an `httptest.Server` as the fake device endpoint.

## `.gitignore`

No additions needed.
