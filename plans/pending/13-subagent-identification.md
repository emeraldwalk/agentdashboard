# Plan 13 — Sub-agent Identification and Filtering

## Implementation checklist

- [ ] Add `IsSubagent bool` field to `conversation.Conversation` model
- [ ] Add `is_subagent` column to SQLite schema with migration
- [ ] Detect sub-agents in `watcher.processFile` (host path pattern)
- [ ] Detect sub-agents in `docker.processContainerFile` (path + ID pattern)
- [ ] Add `IsSidechain bool` and `AgentID string` fields to `jsonl.Record`
- [ ] Update `conversationColumn` in `App.tsx` to map sub-agents to `done` instead of `pending`
- [ ] Expose `isSubagent` in the JSON API response and `Conversation` TS type
- [ ] Update `ConversationCard` to visually distinguish sub-agents (badge or label)

---

## Problem

Sub-agents spawned by the Claude Code SDK are written as separate JSONL files and
ingested as top-level `Conversation` rows. When a sub-agent ends in `waiting_input`
(e.g. orphaned after its parent session stopped), it appears as a live card in the
**Pending** column — misleading the user into thinking a real session needs attention.

---

## Identification approach

There are two reliable, complementary signals. Both should be checked; if either matches
the row is a sub-agent.

### Signal 1 — file path nesting (preferred, explicit)

For **host** sessions, Claude Code places sub-agent JSONL files under:

```
<root>/<project>/<parent-session-uuid>/subagents/<agent-id>.jsonl
```

The parent session UUID directory is a UUID (matches
`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`) and the file sits
under a `subagents/` leaf. The existing `watcher.processFile` and `watcher.scanAll`
already pick these files up via the glob `*/<project>/subagents/*.jsonl` — but nothing
currently marks them as sub-agents.

For **docker** sessions the same directory structure exists inside the container volume
(`/home/vscode/.claude/projects/<project>/<parent-uuid>/subagents/<agent-id>.jsonl`).
`docker.processContainerFile` receives the full `path` string, so the same substring
check works.

Detection function (Go):

```go
// isSubagentPath reports whether path is a sub-agent JSONL file by checking
// for a /subagents/ directory component.
func isSubagentPath(path string) bool {
    return strings.Contains(filepath.ToSlash(path), "/subagents/")
}
```

### Signal 2 — session ID prefix + JSONL content (fallback)

For **docker** sessions that were ingested before the path structure was captured
(or whose parent path cannot be determined), a session whose ID matches `^agent-[a-f0-9]+$`
AND whose first JSONL record contains `"isSidechain": true` is definitively a sub-agent.

Add `IsSidechain bool` and `AgentID string` to `jsonl.Record`:

```go
IsSidechain bool   `json:"isSidechain"`
AgentID     string `json:"agentId"`
```

Detection function:

```go
func IsSubagentRecords(id string, records []jsonl.Record) bool {
    if !strings.HasPrefix(id, "agent-") {
        return false
    }
    for _, r := range records {
        if r.IsSidechain {
            return true
        }
    }
    return false
}
```

Apply Signal 2 as a secondary check in both `watcher.processFile` and
`docker.processContainerFile` when Signal 1 is not conclusive.

---

## Touchpoints

### `internal/jsonl/record.go`

Add two new fields to `Record`:

```go
IsSidechain bool   `json:"isSidechain"`
AgentID     string `json:"agentId"`
```

No changes to `Parse`, `DeriveStatus`, or `DeriveLastEventAt`.

### `internal/conversation/model.go`

Add field:

```go
IsSubagent bool `json:"isSubagent"`
```

### `internal/conversation/store.go`

- Add `is_subagent INTEGER NOT NULL DEFAULT 0` to the `CREATE TABLE` statement.
- Add a migration constant `migration02` that runs `ALTER TABLE conversations ADD COLUMN is_subagent INTEGER NOT NULL DEFAULT 0;` (same pattern as `migration01`).
- Update `Upsert` to include `is_subagent` in both the `INSERT` column list and the `ON CONFLICT DO UPDATE SET` clause.
- Update `List`'s `SELECT` and `rows.Scan` to include `is_subagent`.

### `internal/watcher/watcher.go`

In `processFile`, after deriving `sessionID` from `path`, check:

```go
isSubagent := isSubagentPath(path) || jsonl.IsSubagentRecords(sessionID, records)
```

Add a package-local helper `isSubagentPath(path string) bool` as described above.
Pass `isSubagent` into the `Conversation` struct.

### `internal/docker/source.go`

In `processContainerFile`, after building `sessionID`, check:

```go
isSubagent := isSubagentPath(path) || jsonl.IsSubagentRecords(sessionID, records)
```

`isSubagentPath` can be duplicated here or extracted to a shared internal helper
package — either is acceptable.
Pass `isSubagent` into the `Conversation` struct.

### `internal/jsonl/parser.go` (new exported helper)

Add:

```go
// IsSubagentRecords returns true when id has the "agent-" prefix and at least
// one record carries isSidechain: true.
func IsSubagentRecords(id string, records []Record) bool {
    if !strings.HasPrefix(id, "agent-") {
        return false
    }
    for _, r := range records {
        if r.IsSidechain {
            return true
        }
    }
    return false
}
```

### `frontend/src/types.ts`

Add field:

```ts
isSubagent: boolean;
```

### `frontend/src/App.tsx` — `conversationColumn`

Change the column mapping so a sub-agent is never placed in `pending`, regardless of
its status:

```ts
function conversationColumn(conv: Conversation, now: number): "pending" | "done" | "archived" {
  const age = now - new Date(conv.lastEventAt).getTime();
  if (age > 48 * 60 * 60 * 1000) return "archived";
  if (conv.isSubagent) return "done";   // sub-agents always go to Done
  if (conv.status === "running" || conv.status === "waiting_input") return "pending";
  return "done";
}
```

### `frontend/src/components/ConversationCard.tsx`

Add a visual indicator for sub-agents. The display name should already work (no title →
ID prefix), but add a small "sub-agent" label so users can distinguish them:

```tsx
{props.conversation.isSubagent && (
  <span style="font-size: 0.7em; opacity: 0.6;">sub-agent</span>
)}
```

---

## Concept boundary note

The existing `agent-a*` ID format is **not** the same as a regular Claude conversation
UUID. Do not confuse `isSubagent` (SDK-spawned sub-agent) with `source === "docker"`
(any conversation from a devcontainer). A regular user-initiated conversation inside a
Docker devcontainer has `source: "docker"` but `isSubagent: false`.

---

## .gitignore

No new build artifacts produced by this plan.
