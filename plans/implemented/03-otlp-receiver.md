# 03 — OTLP HTTP Receiver

## Checklist
- [x] Write `internal/otlp/handler.go`
- [x] Write `internal/otlp/parser.go`
- [x] Write `internal/otlp/handler_test.go`

## Context

Receives OTLP data from Claude Code and GitHub Copilot over HTTP on port 4318. Decodes protobuf payloads, extracts session identity, derives status, and writes to the session store. Also notifies the SSE broker so connected browsers update instantly.

No existing code. This package is constructed in `cmd/agentdashboard/main.go` (plan 07) and registered on the OTLP HTTP mux.

## Existing Touchpoints

Imports:
- `internal/session` — calls `Store.Upsert`
- `internal/dashboard` — calls `Broker.Publish`

Imported by:
- `cmd/agentdashboard` (plan 07) — constructs `Handler`, registers routes on OTLP mux

## Contracts

### `internal/otlp/handler.go`

```go
package otlp

import "net/http"

type Handler struct {
    store  session.Store
    broker *dashboard.Broker
}

func NewHandler(store session.Store, broker *dashboard.Broker) *Handler

// RegisterRoutes registers the three OTLP endpoints on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux)
```

Routes registered:
| Path | Method | Handler method |
|---|---|---|
| `/v1/traces` | POST | `h.handleTraces` |
| `/v1/metrics` | POST | `h.handleMetrics` |
| `/v1/logs` | POST | `h.handleLogs` |

Each handler:
1. Reads body (limit 32 MB).
2. Checks `Content-Type: application/x-protobuf`; returns 415 otherwise.
3. Calls the corresponding parser function.
4. Calls `h.store.Upsert` for each session derived from the payload.
5. Calls `h.broker.Publish` with a JSON-encoded `session.Session` for each upsert.
6. Returns 200 with an empty protobuf response body (`ExportTraceServiceResponse{}` etc.).

### `internal/otlp/parser.go`

```go
package otlp

import (
    tracepb  "go.opentelemetry.io/proto/otlp/collector/trace/v1"
    metricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
    logpb    "go.opentelemetry.io/proto/otlp/collector/logs/v1"
)

// ParseTraces decodes a raw protobuf body into a list of Sessions.
func ParseTraces(body []byte) ([]session.Session, error)

// ParseMetrics decodes metrics payload; returns sessions with status "running" as a heartbeat.
func ParseMetrics(body []byte) ([]session.Session, error)

// ParseLogs decodes logs payload; returns sessions inferred from log records.
func ParseLogs(body []byte) ([]session.Session, error)
```

**Session identity extraction** (same logic for all three signal types):
- `session.ID` ← resource attribute `session.id`; if absent, use resource attribute `service.instance.id`; if still absent, skip the resource.
- `session.AgentName` ← resource attribute `service.name`; default to `"unknown"` if absent.

**Status derivation from traces** (`ParseTraces`):
- If any span in the resource has `span.status.code == STATUS_CODE_ERROR`, set `StatusFailed`.
- Else if any span name contains `"waiting"` or `"user_input"` (case-insensitive), set `StatusWaiting`.
- Else if the resource has any span whose `end_time_unix_nano == 0` (still open), set `StatusRunning`.
- Else set `StatusStopped`.

**Status derivation from metrics/logs**: always `StatusRunning` (they are heartbeat signals; the store upsert won't downgrade an existing `stopped`/`failed` status — that logic lives in `Store.Upsert`).

**`StartedAt`**: earliest `start_time_unix_nano` across all spans in the resource. For metrics/logs: use current time if no timestamp is available.

**`LastEventAt`**: current wall clock time at parse time.

### `internal/otlp/handler_test.go`

- Test each handler with a hand-crafted protobuf payload (use `proto.Marshal`).
- Verify that `store.Upsert` is called with the expected session fields.
- Verify 200 response and empty protobuf response body.
- Use a mock/stub `session.Store` and `dashboard.Broker`.

## Concept Boundaries

- This package is the **only** place that touches `go.opentelemetry.io/proto` types. All other packages work with `session.Session`.
- Do not confuse `go.opentelemetry.io/proto/otlp` (protobuf schema) with `go.opentelemetry.io/otel` (the SDK for *emitting* telemetry — not used here).
