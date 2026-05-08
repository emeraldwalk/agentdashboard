# Plan 13 — Sub-agent Identification and Filtering

## Implementation checklist

- [x] Add `IsSidechain bool` and `AgentID string` fields to `jsonl.Record`
- [x] Add `IsSubagent bool` and `ParentID string` fields to `conversation.Conversation` model
- [x] Add `is_subagent` and `parent_id` columns to SQLite schema with migration
- [x] Detect sub-agents and extract `ParentID` in `watcher.processFile` (host path pattern + JSONL)
- [x] Detect sub-agents and extract `ParentID` in `docker.processContainerFile` (path + JSONL)
- [x] Add `IsSubagentRecords` and `ParentIDFromRecords` helpers to `internal/jsonl/parser.go`
- [x] Update `conversationColumn` in `App.tsx` to follow parent column instead of hardcoding `done`
- [x] Expose `isSubagent` and `parentId` in the JSON API response and `Conversation` TS type
- [x] Update `ConversationCard` to visually distinguish sub-agents (small label)

---

## Problem

Sub-agents spawned by the Claude Code SDK are written as separate JSONL files and
ingested as top-level `Conversation` rows. When a sub-agent ends in `waiting_input`
(e.g. orphaned after its parent session stopped), it appears as a live card in the
**Pending** column — misleading the user into thinking a real session needs attention.

Sub-agents should follow their parent's column:
- Parent in Pending → sub-agent in Pending
- Parent in Done → sub-agent in Done
- Parent in Archived → sub-agent in Archived
- Orphaned sub-agent (parent not found) → use sub-agent's own `lastEventAt` for age-based placement, never promote to Pending based on status

---

## Identification approach

There are two reliable, complementary signals. Both should be checked; if either matches
the row is a sub-agent.

### Signal 1 — file path nesting (preferred, explicit)

For **host** sessions, Claude Code places sub-agent JSONL files under:

```
<root>/<project>/<parent-session-uuid>/subagents/<agent-id>.jsonl
```

The parent session UUID directory is a UUID and the file sits under a `subagents/` leaf.
The existing `watcher.processFile` and `watcher.scanAll` already pick these files up via
the glob `*/<project>/subagents/*.jsonl` — but nothing currently marks them as sub-agents.

For **docker** sessions the same directory structure exists inside the container volume
(`/home/vscode/.claude/projects/<project>/<parent-uuid>/subagents/<agent-id>.jsonl`).
`docker.processContainerFile` receives the full `path` string, so the same substring
check works.

Detection + parent extraction (Go):

```go
// isSubagentPath reports whether path is a sub-agent JSONL file.
func isSubagentPath(path string) bool {
    return strings.Contains(filepath.ToSlash(path), "/subagents/")
}

// parentIDFromPath extracts the parent session UUID from a subagent path.
// Path form: .../projects/<project>/<parent-uuid>/subagents/<agent-id>.jsonl
// Returns "" if the path is not a subagent path.
func parentIDFromPath(path string) string {
    slash := filepath.ToSlash(path)
    idx := strings.Index(slash, "/subagents/")
    if idx == -1 {
        return ""
    }
    before := slash[:idx]
    parts := strings.Split(before, "/")
    return parts[len(parts)-1] // the directory immediately above /subagents/
}
```

### Signal 2 — session ID prefix + JSONL content (fallback)

For **docker** sessions whose path structure alone may not surface the parent UUID,
the first JSONL record contains `"isSidechain": true` and `"sessionId": "<parent-uuid>"`.

Add `IsSidechain bool` and `AgentID string` to `jsonl.Record`:

```go
IsSidechain bool   `json:"isSidechain"`
AgentID     string `json:"agentId"`
```

Note: `sessionId` on a sub-agent record refers to the **parent** session UUID — verified
against a live JSONL file:
```json
{"isSidechain":true,"agentId":"a9df8a3097dbbbc01","sessionId":"03077f08-4bc5-465e-a9c2-1bd34f65c705",...}
```

Helpers in `internal/jsonl/parser.go`:

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

// ParentIDFromRecords returns the parent session ID embedded in the first
// sub-agent record's sessionId field. Returns "" if records is empty or
// isSidechain is not set.
func ParentIDFromRecords(records []Record) string {
    for _, r := range records {
        if r.IsSidechain && r.SessionID != "" {
            return r.SessionID
        }
    }
    return ""
}
```

Apply Signal 2 as a secondary check; prefer Signal 1 when both are available.
The `ParentID` should be the same value from either signal — take whichever is non-empty.

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

Add fields:

```go
IsSubagent bool   `json:"isSubagent"`
ParentID   string `json:"parentId"`
```

### `internal/conversation/store.go`

- Add `is_subagent INTEGER NOT NULL DEFAULT 0` and `parent_id TEXT NOT NULL DEFAULT ''`
  to the `CREATE TABLE` statement.
- Add a migration constant `migration02`:
  ```go
  const migration02 = `ALTER TABLE conversations ADD COLUMN is_subagent INTEGER NOT NULL DEFAULT 0;
  ALTER TABLE conversations ADD COLUMN parent_id TEXT NOT NULL DEFAULT '';`
  ```
  Run it the same way as `migration01` (ignore error if column already exists).
- Update `Upsert` to include both columns in the `INSERT` list and `ON CONFLICT DO UPDATE SET`.
- Update `List`'s `SELECT` and `rows.Scan` to include both columns.

### `internal/watcher/watcher.go`

In `processFile`, after deriving `sessionID`:

```go
isSubagent := isSubagentPath(path) || jsonl.IsSubagentRecords(sessionID, records)
parentID := parentIDFromPath(path)
if parentID == "" {
    parentID = jsonl.ParentIDFromRecords(records)
}
```

Add package-local helpers `isSubagentPath` and `parentIDFromPath` as described above.
Pass both into the `Conversation` struct.

### `internal/docker/source.go`

In `processContainerFile`, after building `sessionID`:

```go
isSubagent := isSubagentPath(path) || jsonl.IsSubagentRecords(sessionID, records)
parentID := parentIDFromPath(path)
if parentID == "" {
    parentID = jsonl.ParentIDFromRecords(records)
}
```

`isSubagentPath` and `parentIDFromPath` can be duplicated here or extracted to a shared
internal helper package — either is acceptable.
Pass both into the `Conversation` struct.

### `internal/jsonl/parser.go` (new exported helpers)

Add `IsSubagentRecords` and `ParentIDFromRecords` as specified in the identification
section above.

### `frontend/src/types.ts`

Add fields:

```ts
isSubagent: boolean;
parentId: string;
```

### `frontend/src/App.tsx` — `conversationColumn`

Sub-agents follow their parent's column. The parent lookup must happen against the full
conversation list, so `conversationColumn` needs access to it. Change the signature and
call site:

```ts
function conversationColumn(
  conv: Conversation,
  now: number,
  allConvs: Conversation[],
): "pending" | "done" | "archived" {
  const age = now - new Date(conv.lastEventAt).getTime();
  if (age > 48 * 60 * 60 * 1000) return "archived";

  if (conv.isSubagent) {
    const parent = conv.parentId
      ? allConvs.find((c) => c.id === conv.parentId)
      : undefined;
    if (parent) {
      // Recurse once — parent is never itself a sub-agent in practice.
      return conversationColumn(parent, now, allConvs);
    }
    // Orphaned sub-agent: age-based placement only, never Pending.
    return "done";
  }

  if (conv.status === "running" || conv.status === "waiting_input") return "pending";
  return "done";
}
```

Update the call site in `columnGroups` memo to pass `conversations()` as the third argument.

### `frontend/src/components/ConversationCard.tsx`

Add a small "sub-agent" label so users can distinguish them from top-level sessions:

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

The `sessionId` field in a sub-agent JSONL record refers to the **parent** session UUID,
not the sub-agent's own ID. The sub-agent's own ID is the filename stem (e.g.
`agent-a9df8a3097dbbbc01`) and is also present as `agentId` in the record.

---

## .gitignore

No new build artifacts produced by this plan.
