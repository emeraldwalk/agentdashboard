## Plan 08 — Raw Event Capture

### Goal

Store every incoming OTLP payload as a JSON blob in SQLite so we can inspect the full attribute schema regardless of whether the session parser understands it. Existing session parsing is unchanged.

---

### Implementation steps

- [x] Add `raw_events` table to SQLite schema in `internal/session/store.go`
- [x] Add `AppendRawEvent` method to `Store` interface and `sqliteStore`
- [x] Add `RawEventStore` interface (subset of `Store`) to `internal/otlp/handler.go` or a new file, to avoid coupling
- [x] In each OTLP handler, proto-decode the body to JSON and call `AppendRawEvent` before parsing sessions
- [x] Add `GET /api/raw-events` endpoint to `internal/dashboard/server.go` with optional `?signal=traces|metrics|logs` filter
- [x] Update `plan.md`

---

### Existing touchpoints

| File | Change |
|---|---|
| `internal/session/store.go` | Add `raw_events` schema, `AppendRawEvent(signal, json string) error` to interface and impl |
| `internal/session/model.go` | Add `RawEvent` struct |
| `internal/otlp/handler.go` | Accept a `RawStore` interface; call it in all three handlers |
| `internal/otlp/handler_test.go` | Add/update tests for raw capture |
| `internal/dashboard/server.go` | Add `GET /api/raw-events` handler |
| `cmd/agentdashboard/main.go` | Pass store to `otlp.NewHandler` (already passes `store`; just needs updated signature if changed) |

---

### Contracts

#### `raw_events` table

```sql
CREATE TABLE IF NOT EXISTS raw_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    signal      TEXT NOT NULL,   -- "traces" | "metrics" | "logs"
    received_at DATETIME NOT NULL,
    payload     TEXT NOT NULL    -- JSON-encoded protobuf message
);
```

#### `Store` interface addition

```go
AppendRawEvent(signal, payload string) error
```

#### `RawEvent` model

```go
type RawEvent struct {
    ID         int64
    Signal     string
    ReceivedAt time.Time
    Payload    string
}
```

#### `otlp.Handler` raw store interface

```go
type RawStore interface {
    AppendRawEvent(signal, payload string) error
}
```

`NewHandler` signature becomes:

```go
func NewHandler(store session.Store, raw RawStore, broker BrokerPublisher) *Handler
```

Since `session.Store` will embed `AppendRawEvent`, callers can pass the same store for both — but keeping a narrow interface on the OTLP side avoids tight coupling.

#### Proto-to-JSON conversion

Use `google.golang.org/protobuf/encoding/protojson` — already an indirect dep via the proto packages:

```go
import "google.golang.org/protobuf/encoding/protojson"

jsonBytes, err := protojson.Marshal(protoMsg)
```

#### `GET /api/raw-events` response

```json
[
  {
    "id": 1,
    "signal": "logs",
    "received_at": "2026-04-27T19:53:00Z",
    "payload": "{ ... }"
  }
]
```

Optional query param `?signal=logs` filters by signal type. Returns newest-first (ORDER BY id DESC). Limit 200 rows max to avoid blowing up the browser.

---

### .gitignore

No new artifacts.
