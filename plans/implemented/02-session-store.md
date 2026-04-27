# 02 — Session Model & SQLite Store

## Checklist
- [x] Write `internal/session/model.go`
- [x] Write `internal/session/store.go` (interface + SQLite implementation)
- [x] Write `internal/session/store_test.go`

## Context

All agent session state lives here. The OTLP receiver (plan 03) calls `Store.Upsert` on every incoming span batch. The dashboard server (plan 05) calls `Store.List` to serve `/api/sessions`. There is no existing code — create the package from scratch.

## Existing Touchpoints

None yet. This package is imported by:
- `internal/otlp` (plan 03) — calls `Upsert`
- `internal/dashboard` (plan 05) — calls `List`
- `cmd/agentdashboard` (plan 07) — constructs the store, passes it to both above

## Contracts

### `internal/session/model.go`

```go
package session

import "time"

type Status string

const (
    StatusRunning  Status = "running"
    StatusWaiting  Status = "waiting_input"
    StatusStopped  Status = "stopped"
    StatusFailed   Status = "failed"
)

type Session struct {
    ID          string
    AgentName   string
    Status      Status
    StartedAt   time.Time
    LastEventAt time.Time
}
```

### `internal/session/store.go`

```go
package session

type Store interface {
    Upsert(s Session) error
    List() ([]Session, error)
    Close() error
}

func NewSQLiteStore(path string) (Store, error)
```

`NewSQLiteStore` opens (or creates) a SQLite database at `path` using `modernc.org/sqlite` and runs the schema migration below.

### SQLite Schema

```sql
CREATE TABLE IF NOT EXISTS sessions (
    id            TEXT PRIMARY KEY,
    agent_name    TEXT NOT NULL,
    status        TEXT NOT NULL,
    started_at    DATETIME NOT NULL,
    last_event_at DATETIME NOT NULL
);
```

`Upsert` uses `INSERT OR REPLACE`. `List` returns all rows ordered by `last_event_at DESC`.

### `internal/session/store_test.go`

Test `NewSQLiteStore` with an in-memory path (`:memory:`), exercise `Upsert` and `List` round-trip, and verify status transitions update correctly.

## Concept Boundaries

- `Status` values are lowercase strings matching what the JSON API and SSE events emit — do not use integer enum constants.
- `started_at` is set only on the first insert (upsert must not overwrite it if the row already exists). Use `INSERT OR IGNORE` + a separate `UPDATE` or an `ON CONFLICT` clause to preserve `started_at`.
