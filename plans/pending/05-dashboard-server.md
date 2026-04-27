# 05 — Dashboard HTTP Server

## Checklist
- [ ] Write `internal/dashboard/server.go`
- [ ] Write `internal/dashboard/server_test.go`

## Context

HTTP server on port 8080. Serves the embedded SolidJS SPA, a JSON sessions API, and an SSE event stream. The frontend `dist/` directory is embedded at compile time — it must exist before `go build` runs (produced by `make build-frontend`).

No existing code. Constructed and started in `cmd/agentdashboard/main.go` (plan 07).

## Existing Touchpoints

Imports:
- `internal/session` — calls `Store.List` for `/api/sessions`
- `internal/dashboard` (same package) — uses `Broker` for `/api/events`

Imported by:
- `cmd/agentdashboard` (plan 07) — constructs `Server`, calls `server.Start(ctx)`

## Contracts

### `internal/dashboard/server.go`

```go
package dashboard

import (
    "context"
    "embed"
    "net/http"
)

//go:embed all:dist
var frontendFS embed.FS

type Server struct {
    store  session.Store
    broker *Broker
    addr   string
}

func NewServer(store session.Store, broker *Broker, addr string) *Server

// Start registers routes and serves until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error
```

**Route table:**

| Pattern | Handler |
|---|---|
| `GET /api/sessions` | JSON array of `session.Session`; `Content-Type: application/json` |
| `GET /api/events` | SSE stream (see below) |
| `GET /assets/` | Static files from embedded `dist/assets/` |
| `GET /` (catch-all) | Serve `dist/index.html` (SPA fallback) |

**SSE handler (`/api/events`):**
1. Sets headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`.
2. Calls `b.Subscribe()`, defers `b.Unsubscribe(ch)`.
3. Loops: on each received `[]byte`, writes `event: session-update\ndata: <payload>\n\n` and flushes.
4. Returns when client disconnects (`r.Context().Done()`).

**SSE event format:**
```
event: session-update
data: {"id":"...","agentName":"...","status":"running","lastEventAt":"2026-04-26T12:00:00Z"}

```
(JSON keys are camelCase to match frontend expectations.)

**Embed path:** `dist/` is at `internal/dashboard/dist/` in the repo (symlinked or copied from `frontend/dist/` by the Makefile). The `//go:embed all:dist` directive in this file refers to that path relative to the source file.

Alternative — to avoid a symlink, the Makefile `build-go` target can copy `frontend/dist` to `internal/dashboard/dist` before running `go build`. Document whichever approach is chosen in `CLAUDE.md`.

### `internal/dashboard/server_test.go`

- Test `/api/sessions` returns valid JSON with a mocked store.
- Test `/api/events` receives a published broker message within a timeout.

## Concept Boundaries

- The `dist/` directory embedded here is the **built** frontend output, not the `frontend/src/` source. Do not confuse the two.
- `/api/` prefix is reserved for Go handlers. The SPA must not define client-side routes beginning with `/api/`.
