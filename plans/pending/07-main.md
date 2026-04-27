# 07 — Main Entry Point

## Checklist
- [ ] Write `cmd/agentdashboard/main.go`

## Context

Wires all packages together, parses CLI flags, starts the OTLP receiver and dashboard server concurrently, and handles graceful shutdown on SIGINT/SIGTERM.

This is the last plan to implement — depends on all prior plans being complete.

## Existing Touchpoints

Imports (all must exist before implementing this plan):
- `internal/session` (plan 02) — `session.NewSQLiteStore`
- `internal/dashboard` (plan 04, 05) — `dashboard.NewBroker`, `dashboard.NewServer`
- `internal/otlp` (plan 03) — `otlp.NewHandler`

## Contracts

### `cmd/agentdashboard/main.go`

```go
package main

func main()
```

**CLI flags:**

| Flag | Default | Description |
|---|---|---|
| `--db` | `~/.agentdashboard/sessions.db` | SQLite database file path |
| `--otlp-addr` | `:4318` | OTLP HTTP listen address |
| `--dashboard-addr` | `:8080` | Dashboard HTTP listen address |

**Startup sequence:**
1. Parse flags.
2. Expand `--db` path (resolve `~`).
3. Create `~/.agentdashboard/` directory if it does not exist.
4. Open SQLite store: `session.NewSQLiteStore(dbPath)`.
5. Create broker: `dashboard.NewBroker()`.
6. Start `broker.Run(ctx)` in a goroutine.
7. Create OTLP handler: `otlp.NewHandler(store, broker)`.
8. Create dashboard server: `dashboard.NewServer(store, broker, dashboardAddr)`.
9. Start OTLP HTTP server in a goroutine (plain `net/http` mux, no TLS).
10. Start dashboard HTTP server in a goroutine via `server.Start(ctx)`.
11. Wait for SIGINT or SIGTERM.
12. Cancel context, call `store.Close()`, log "shutting down".

**Error handling:** any startup error (store open, bind failure) writes to stderr and exits with code 1.

## Concept Boundaries

- Two separate `http.ServeMux` instances: one for OTLP (:4318), one for the dashboard (:8080). They must not share a mux.
- `context.WithCancel` is the shutdown signal passed to broker and server — do not use `os.Exit` to stop goroutines.
