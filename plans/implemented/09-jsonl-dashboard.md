# 09 — JSONL-Based Conversation Dashboard

## Checklist

- [x] Delete `internal/otlp/` package entirely
- [x] Replace `internal/session/` with `internal/conversation/` (new model + store)
- [x] Write `internal/jsonl/` — JSONL record types + parser
- [x] Write `internal/watcher/` — host `~/.claude` filesystem watcher
- [x] Write `internal/docker/` — Docker socket volume discovery + exec-based file tailing
- [x] Update `internal/dashboard/server.go` — swap session store for conversation store, update API routes
- [x] Update `frontend/` — new `Conversation` type, updated card and app
- [x] Update `cmd/agentdashboard/main.go` — remove OTLP, wire new packages
- [x] Update `scripts/dev-go.sh` — remove `--otlp-addr` flag
- [x] Update `CLAUDE.md` — remove OTLP references, document new flags
- [x] Update `plans/plan.md`

---

## Context for a cold agent

This plan replaces the existing OTLP-based session tracking with a JSONL log file reader. The dashboard was originally designed to receive OpenTelemetry (OTLP) telemetry pushed from Claude Code agents. Investigation showed OTLP lacks conversation-level structure and the wrong schema assumptions were baked in.

Claude Code writes rich JSONL session logs to `~/.claude/projects/<session-id>.jsonl` (and subagents to `~/.claude/projects/<session-id>/subagents/agent-<id>.jsonl`). These files contain full conversation turns, tool calls, AI-generated titles, and status signals — everything needed for the dashboard.

The dashboard runs on the **host machine** (not inside a devcontainer). It needs to read `.jsonl` files from:
1. The host's own `~/.claude/projects/`
2. Named Docker volumes inside devcontainers — each devcontainer mounts a volume named `claude-code-config-<project>` at `/home/vscode/.claude`. These volumes are not directly accessible as host filesystem paths on Mac + Docker Desktop, so they are accessed via the Docker Engine API (`/var/run/docker.sock`).

The goal is to show a flat list of conversations sorted by most-recent activity, with project name as a label, and live status (running / waiting / stopped / failed).

---

## What to delete

| Path | Action |
|---|---|
| `internal/otlp/` | Delete entire directory |
| `internal/session/` | Delete entire directory |

The `internal/dashboard/events.go` SSE broker is kept unchanged. The `internal/dashboard/server.go` HTTP server structure is kept; only the store type and API handlers change.

---

## Existing touchpoints

| File | Change |
|---|---|
| `internal/session/` | Deleted — replaced by `internal/conversation/` |
| `internal/otlp/` | Deleted entirely |
| `internal/dashboard/server.go` | Replace `session.Store` with `conversation.Store`; update `/api/sessions` → `/api/conversations`; remove `/api/raw-events` |
| `internal/dashboard/server_test.go` | Update mock store to implement `conversation.Store` |
| `cmd/agentdashboard/main.go` | Remove OTLP mux and handler; wire `watcher` and `docker` packages |
| `scripts/dev-go.sh` | Remove `--otlp-addr` flag |
| `CLAUDE.md` | Update ports table (remove 4318), update dev instructions |
| `frontend/src/types.ts` | Replace `Session` with `Conversation` |
| `frontend/src/App.tsx` | Fetch `/api/conversations`, SSE event `conversation-update` |
| `frontend/src/components/SessionCard.tsx` | Rename to `ConversationCard.tsx`, update props |

---

## Contracts

### `internal/conversation/model.go`

```go
package conversation

import "time"

type Status string

const (
    StatusRunning Status = "running"       // actively receiving events
    StatusWaiting Status = "waiting_input" // last event was user_prompt with no assistant response yet
    StatusStopped Status = "stopped"       // no activity, last turn completed cleanly
    StatusFailed  Status = "failed"        // last event was internal_error with no recovery
)

type Conversation struct {
    ID          string    `json:"id"`          // session ID from the JSONL filename (UUID)
    Project     string    `json:"project"`     // derived from volume/path name (e.g. "agentdashboard")
    Title       string    `json:"title"`       // from ai-title record; empty if not yet generated
    Status      Status    `json:"status"`
    StartedAt   time.Time `json:"startedAt"`
    LastEventAt time.Time `json:"lastEventAt"`
}
```

**Status derivation rules** (applied when ingesting JSONL records, in priority order):
- If the most recent record type is `user` → `StatusWaiting`
- If the most recent record type is `assistant` → `StatusStopped`
- If a `queue-operation` with `operation: "dequeue"` arrived within the last 30s → `StatusRunning`
- If the only records are `internal_error` with no subsequent `user`/`assistant` → `StatusFailed`
- Default → `StatusStopped`

### `internal/conversation/store.go`

```go
package conversation

type Store interface {
    Upsert(c Conversation) error
    List() ([]Conversation, error)
    Close() error
}

func NewSQLiteStore(path string) (Store, error)
```

SQLite schema:

```sql
CREATE TABLE IF NOT EXISTS conversations (
    id            TEXT PRIMARY KEY,
    project       TEXT NOT NULL,
    title         TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL,
    started_at    DATETIME NOT NULL,
    last_event_at DATETIME NOT NULL
);
```

`Upsert` uses `INSERT OR IGNORE` to set `started_at` only on first insert, then `UPDATE` to set `project`, `title`, `status`, `last_event_at`. `List` returns all rows `ORDER BY last_event_at DESC`.

### `internal/jsonl/`

#### `internal/jsonl/record.go`

JSONL record types emitted by Claude Code. Each line in a `.jsonl` file is one of these.

All record types observed in the wild, with their actual top-level JSON fields:

| type | Key fields |
|---|---|
| `queue-operation` | `type`, `operation` (`"enqueue"`\|`"dequeue"`), `timestamp`, `sessionId` |
| `user` | `type`, `uuid`, `parentUuid`, `promptId`, `timestamp`, `sessionId`, `message.role`, `message.content` |
| `assistant` | `type`, `uuid`, `parentUuid`, `timestamp`, `sessionId`, `message.role`, `message.content`, `message.stop_reason` |
| `ai-title` | `type`, `sessionId`, `aiTitle` (note: **`aiTitle`**, not `title`) |
| `attachment` | `type`, `parentUuid`, `attachment.type` — ignored |
| `file-history-snapshot` | `type`, `messageId` — ignored |
| `last-prompt` | `type`, `sessionId`, `lastPrompt`, `leafUuid` — ignored |

```go
package jsonl

import "time"

// Record is the common envelope for all JSONL log lines.
// All record types have a top-level timestamp field.
type Record struct {
    Type      string    `json:"type"`
    SessionID string    `json:"sessionId"`
    Timestamp time.Time `json:"timestamp"`

    // type == "user" or "assistant"
    UUID       string   `json:"uuid"`
    ParentUUID string   `json:"parentUuid"`
    PromptID   string   `json:"promptId"`
    Message    *Message `json:"message,omitempty"`

    // type == "ai-title" — note field name is "aiTitle" not "title"
    AITitle string `json:"aiTitle,omitempty"`

    // type == "queue-operation"
    Operation string `json:"operation,omitempty"` // "enqueue" | "dequeue"
}

type Message struct {
    Role    string `json:"role"`
    Content any    `json:"content"` // string or []ContentBlock — leave as any, not needed for status
}
```

Only these `type` values are acted on:
- `"user"` — sets status to `waiting_input`; use `Record.Timestamp` for `LastEventAt`
- `"assistant"` — sets status to `stopped`; use `Record.Timestamp` for `LastEventAt`
- `"queue-operation"` with `operation == "dequeue"` — sets status to `running`; use `Record.Timestamp`
- `"ai-title"` — updates `Conversation.Title` from `Record.AITitle`

All other types (`attachment`, `file-history-snapshot`, `last-prompt`) are parsed but ignored for status purposes.

#### `internal/jsonl/parser.go`

`internal/jsonl` imports `internal/conversation` for the `Status` type. This is a one-way dependency: `conversation` does not import `jsonl`.

```go
package jsonl

import (
    "io"
    "github.com/emeraldwalk/agentdashboard/internal/conversation"
)

// Parse reads all complete JSON lines from r and returns them as Records.
// Incomplete trailing lines (no newline) are not returned — they may be mid-write.
func Parse(r io.Reader) ([]Record, error)

// ParseFile reads and parses a complete JSONL file at path.
func ParseFile(path string) ([]Record, error)

// DeriveStatus returns the conversation status from an ordered slice of records
// (oldest first). Uses the rules defined in the Status derivation section above.
func DeriveStatus(records []Record) conversation.Status

// DeriveLastEventAt returns the timestamp of the most recent actionable record
// (user, assistant, or queue-operation). Returns zero time if no records.
func DeriveLastEventAt(records []Record) time.Time
```

### `internal/watcher/`

Watches the host `~/.claude/projects/` directory for new and modified `.jsonl` files, parses them, and publishes `conversation.Conversation` updates.

```go
package watcher

import (
    "context"
    "github.com/emeraldwalk/agentdashboard/internal/conversation"
)

type Handler interface {
    OnConversation(c conversation.Conversation)
}

type Watcher struct {
    root    string // e.g. ~/.claude/projects
    handler Handler
}

func New(root string, handler Handler) (*Watcher, error)

// Run watches root for JSONL changes until ctx is cancelled.
// Calls handler.OnConversation for each new or updated conversation.
func (w *Watcher) Run(ctx context.Context) error
```

Use `github.com/fsnotify/fsnotify` for filesystem events. On startup, do an initial scan of all existing `.jsonl` files before watching for new ones.

**Project name extraction from host paths:**
- Path pattern: `~/.claude/projects/<session-id>.jsonl`
- No project name is available from the host path alone — use `"local"` as the project label for host-sourced conversations.

**Subagent files** (`~/.claude/projects/<session-id>/subagents/agent-<id>.jsonl`) are watched but treated as separate conversations with the same project label. Their session ID is the `agent-<id>` portion.

### `internal/docker/`

Discovers devcontainer volumes via the Docker socket and tails `.jsonl` files inside them.

```go
package docker

import (
    "context"
    "github.com/emeraldwalk/agentdashboard/internal/watcher"
)

// Source discovers and tails Claude JSONL logs from Docker containers.
type Source struct {
    socketPath string // default: /var/run/docker.sock
    handler    watcher.Handler
}

func New(socketPath string, handler watcher.Handler) *Source

// Run discovers containers with claude-code-config-* volumes, tails their
// ~/.claude/projects/ JSONL files, and calls handler.OnConversation.
// Periodically re-discovers to pick up newly started containers.
// Runs until ctx is cancelled.
func (s *Source) Run(ctx context.Context) error
```

**Discovery algorithm:**
1. Call Docker API `GET /volumes` — filter names matching `claude-code-config-*`.
2. For each matching volume, call `GET /containers/json` to find a running container that has the volume mounted at `/home/vscode/.claude`.
3. Extract project name from volume name: `claude-code-config-<project>` → `<project>`.
4. Use Docker exec API to run `find /home/vscode/.claude/projects -name "*.jsonl"` in the container to get the file list.
5. For each file, use Docker exec to `cat <path>` and pass the output to `jsonl.Parse`. Record the byte offset (file size after read).
6. On each re-discovery tick, re-run `cat` and skip the first N bytes already seen (or re-parse the full file — JSONL re-parse is idempotent since `Upsert` is idempotent).

**Do not use `tail -f` via Docker exec.** Streaming stdout from a long-running exec is complex and fragile. Instead, poll: re-read each known file on every re-discovery tick (every 30 seconds). New files found during re-discovery are added to the tracked set.

**Docker client:** use `github.com/docker/docker/client` (official Go SDK). Add to `go.mod` via `go get github.com/docker/docker/client`.

**Re-discovery interval:** every 30 seconds, re-run steps 1–6 to detect newly started containers and pick up new records in existing files.

**Graceful degradation:** if `/var/run/docker.sock` does not exist or is not accessible, log a warning and skip Docker discovery. The host watcher still runs.

### `internal/dashboard/server.go` changes

Replace the `session.Store` field with `conversation.Store`. Update:

- Route `GET /api/sessions` → `GET /api/conversations`
- Remove `GET /api/raw-events`
- SSE event name: `session-update` → `conversation-update`
- JSON response shape: `Conversation` not `Session`

New JSON shape for `/api/conversations`:

```json
[
  {
    "id": "be4bdbc4-ac4b-4e4f-9228-f12b2ff778c8",
    "project": "agentdashboard",
    "title": "Fix JSONL parser bug",
    "status": "waiting_input",
    "startedAt": "2026-04-28T02:47:14Z",
    "lastEventAt": "2026-04-28T03:09:39Z"
  }
]
```

### `cmd/agentdashboard/main.go` changes

Remove:
- OTLP mux and `otlp.NewHandler`
- `--otlp-addr` flag

Add:
- `--claude-dir` flag, default `~/.claude/projects` — path to host Claude projects directory
- `--docker-socket` flag, default `/var/run/docker.sock` — Docker socket path

New startup sequence:
1. Parse flags, expand paths.
2. Open SQLite store: `conversation.NewSQLiteStore(dbPath)`.
3. Create broker: `dashboard.NewBroker()`.
4. Start `broker.Run(ctx)` goroutine.
5. Define an `ingestHandler` inline in `main.go` that implements `watcher.Handler`:
   ```go
   type ingestHandler struct {
       store  conversation.Store
       broker *dashboard.Broker
   }
   func (h *ingestHandler) OnConversation(c conversation.Conversation) {
       if err := h.store.Upsert(c); err != nil {
           log.Printf("upsert error: %v", err)
           return
       }
       data, _ := json.Marshal(c)
       h.broker.Publish(data)
   }
   ```
   `broker.Publish` receives the JSON-encoded `Conversation`. The frontend SSE event carries this as the `data` field of a `conversation-update` event. The JSON keys must be camelCase (use struct tags on `conversation.Conversation`).
6. Create and start `watcher.New(claudeDir, handler)` in a goroutine.
7. Create and start `docker.New(dockerSocket, handler)` in a goroutine (skip if socket missing).
8. Create and start `dashboard.NewServer(store, broker, dashboardAddr)`.
9. Wait for SIGINT/SIGTERM, cancel context, close store.

### Frontend changes

`frontend/src/types.ts`:
```ts
export interface Conversation {
  id: string;
  project: string;
  title: string;
  status: "running" | "waiting_input" | "stopped" | "failed";
  startedAt: string;
  lastEventAt: string;
}
```

`frontend/src/App.tsx`:
- Fetch `/api/conversations` on mount
- Listen for SSE event `conversation-update`
- Sort by `lastEventAt` descending (most recent on top)

`frontend/src/components/ConversationCard.tsx` (rename from `SessionCard.tsx`):
- Props: `conversation: Conversation`
- Display: `project` as a small label, `title` (or `id.slice(0,8)` if empty), `status` badge (same colors as before), `lastEventAt` relative time

---

## New dependencies

Add via `go get` — do not hand-edit `go.mod`:

| Package | Purpose |
|---|---|
| `github.com/fsnotify/fsnotify` | Filesystem watching for host watcher |
| `github.com/docker/docker/client` | Docker Engine API client |

Remove (no longer needed — `go mod tidy` will clean them up after deleting `internal/otlp/`):
- `go.opentelemetry.io/proto/otlp`
- `google.golang.org/protobuf`

---

## Ports

After this plan, only one port remains:

| Port | Purpose |
|---|---|
| 8080 | Dashboard HTTP server |

Remove port 4318 from `CLAUDE.md` ports table.

---

## .gitignore

No changes needed.

---

## Concept boundaries

- **`internal/jsonl`** parses raw record lines and derives status. It does not touch the database or broker.
- **`internal/watcher`** and **`internal/docker`** are both sources — they both call `watcher.Handler.OnConversation`. The handler interface is defined in `internal/watcher` and reused by `internal/docker` to avoid a new package.
- **`internal/conversation`** owns the model and SQLite store. Neither `watcher` nor `docker` import `dashboard`.
- **`internal/dashboard`** imports `conversation` for the store. It does not import `watcher` or `docker`.
- Do not confuse `conversation.ID` (the JSONL filename stem / session UUID) with `prompt.id` (a per-turn identifier seen in OTLP data). The dashboard tracks at the session level, not the turn level.
- The SSE event name changes from `session-update` to `conversation-update`. The frontend must match exactly.
