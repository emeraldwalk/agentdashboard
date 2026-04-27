# 04 — SSE Broker

## Checklist
- [ ] Write `internal/dashboard/events.go`
- [ ] Write `internal/dashboard/events_test.go`

## Context

Fan-out broker that receives session-update events from the OTLP handler and streams them to all connected browser SSE clients. Single goroutine owns the subscriber map to avoid races.

No existing code. Constructed in `cmd/agentdashboard/main.go` (plan 07), passed to both `otlp.Handler` (plan 03) and `dashboard.Server` (plan 05).

## Existing Touchpoints

Imported by:
- `internal/otlp` (plan 03) — calls `Broker.Publish`
- `internal/dashboard` (plan 05) — calls `Broker.Subscribe` / `Broker.Unsubscribe` in the SSE HTTP handler
- `cmd/agentdashboard` (plan 07) — calls `broker.Run(ctx)` in a goroutine

## Contracts

### `internal/dashboard/events.go`

```go
package dashboard

import "context"

type Broker struct {
    subscribe   chan chan []byte
    unsubscribe chan chan []byte
    publish     chan []byte
}

func NewBroker() *Broker

// Run processes subscribe/unsubscribe/publish until ctx is cancelled.
// Must be called in its own goroutine.
func (b *Broker) Run(ctx context.Context)

// Subscribe returns a channel that will receive published payloads.
func (b *Broker) Subscribe() chan []byte

// Unsubscribe removes the channel from the broker.
func (b *Broker) Unsubscribe(ch chan []byte)

// Publish sends data to all current subscribers (non-blocking per subscriber).
func (b *Broker) Publish(data []byte)
```

Implementation notes:
- `Run` maintains a `map[chan []byte]struct{}` of subscribers.
- `Publish` sends to each subscriber channel with a `select` default to drop if the channel is full (prevents a slow client from blocking the broker).
- Subscriber channel buffer size: 16.

### `internal/dashboard/events_test.go`

- Verify that `Publish` delivers to all subscribers.
- Verify that after `Unsubscribe`, the channel no longer receives messages.
- Verify that a full subscriber channel does not block `Publish`.

## Concept Boundaries

- `Broker` lives in package `dashboard` (same package as the HTTP server) to avoid a circular import with `otlp`. The `otlp` package receives a `*dashboard.Broker`; the dashboard package does not import `otlp`.
