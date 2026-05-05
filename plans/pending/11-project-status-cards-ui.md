# Plan 11 — Project Status Cards UI

## Implementation Checklist

- [ ] Add `source` field to `Conversation` model, store, and API
- [ ] Populate `source` in watcher and docker source
- [ ] Group conversations by project in frontend (derive `ProjectGroup`)
- [ ] Build `ProjectCard` component (collapsed / expanded conversation list)
- [ ] Build `ConversationRow` subcomponent (replaces flat `ConversationCard`)
- [ ] Update `App.tsx` to render grouped `ProjectCard` list
- [ ] Wire SSE updates into the grouped data structure

---

## Goals

Two UI improvements:

1. **Project status cards** — group conversations by project. Each card shows the project name and up to 3 most-recent conversation rows; an expand toggle reveals the rest. Cards sorted by the most-recent `lastEventAt` within the group.
2. **Source indicator** — visually distinguish conversations that came from a devcontainer (Docker volume) vs. the host filesystem.

No new data is introduced for grouping — conversations are already fetched with a `project` field. Grouping is pure frontend derivation. The only new data field is `source`.

---

## Existing Touchpoints

| File | Role |
|---|---|
| `internal/conversation/model.go` | `Conversation` struct — add `Source` field |
| `internal/conversation/store.go` | SQLite schema + `Upsert`/`List` — add `source` column |
| `internal/watcher/watcher.go` | Host filesystem source — sets `Source: "host"` |
| `internal/docker/source.go` | Docker source — sets `Source: "docker"` |
| `frontend/src/types.ts` | `Conversation` interface — add `source` field |
| `frontend/src/App.tsx` | Replace flat list with grouped `ProjectCard` list |
| `frontend/src/components/ConversationCard.tsx` | Repurpose or replace as `ConversationRow` |

---

## Contracts

### 1. `Conversation.Source` field

**Go** (`internal/conversation/model.go`):
```go
type Source string

const (
    SourceHost   Source = "host"
    SourceDocker Source = "docker"
)

type Conversation struct {
    // existing fields ...
    Source Source `json:"source"`
}
```

**SQLite** — add column to existing schema (new `CREATE TABLE` DDL and an `ALTER TABLE` migration for existing DBs):
```sql
ALTER TABLE conversations ADD COLUMN source TEXT NOT NULL DEFAULT 'host';
```

**Upsert** — include `source` in both `INSERT` and `DO UPDATE SET`.

**List** — include `source` in `SELECT` and `rows.Scan`.

### 2. Watcher and Docker source set `Source`

- `internal/watcher/watcher.go` `processFile`: set `c.Source = conversation.SourceHost`
- `internal/docker/source.go` `processContainerFile`: set `c.Source = conversation.SourceDocker`

### 3. SSE `conversation-update` event

The existing SSE event format carries one `Conversation`. It carries the `source` field automatically once the struct is updated — no handler changes needed.

TypeScript shape update (`frontend/src/types.ts`):
```typescript
export interface Conversation {
  id: string;
  project: string;
  title: string;
  status: "running" | "waiting_input" | "stopped" | "failed";
  source: "host" | "docker";   // NEW
  startedAt: string;
  lastEventAt: string;
}
```

### 4. Frontend — grouped data structure

In `App.tsx`, derive a computed list of `ProjectGroup` values from the flat `conversations` signal:

```typescript
interface ProjectGroup {
  project: string;
  lastEventAt: string;           // max of member conversations, for sorting groups
  conversations: Conversation[]; // sorted by lastEventAt DESC within group
}
```

Grouping logic runs as a derived signal (SolidJS `createMemo`). Groups sorted by `lastEventAt` DESC.

### 5. `ProjectCard` component

New file: `frontend/src/components/ProjectCard.tsx`

Props:
```typescript
interface Props {
  group: ProjectGroup;
}
```

Behavior:
- Renders project name as a header.
- Shows up to 3 `ConversationRow` items when collapsed.
- If `group.conversations.length > 3`, shows an expand/collapse toggle button (e.g. "Show all 7" / "Show less").
- Expanded state is local (`createSignal<boolean>(false)`).

### 6. `ConversationRow` component

Rename / repurpose `ConversationCard.tsx` → `ConversationRow.tsx` (or create `ConversationRow.tsx` and keep `ConversationCard.tsx` as-is if preferred).

Displays per-conversation info in a compact row:
- Status badge (existing color scheme)
- Title / truncated ID
- Relative time
- Source indicator: a small label or icon — "host" vs "docker". Visual treatment is up to the implementer (e.g. a chip, badge, or subtle icon).

### 7. `App.tsx` updated render

Replace the flat `<For each={conversations()}>` with:
```tsx
<For each={projectGroups()}>
  {(group) => <ProjectCard group={group} />}
</For>
```

Where `projectGroups` is a `createMemo` derived from `conversations`.

SSE updates continue to update the flat `conversations` signal; the memo re-derives groups reactively.

---

## Concept Boundaries

- **`ConversationCard`** (existing) vs **`ConversationRow`** (new) — the existing component will be replaced or renamed. Do not introduce both names permanently.
- **`ProjectGroup`** is a frontend-only derived type — it has no backend representation and must not be confused with the `project` string field on `Conversation`.
- **`source`** ("host" | "docker") is distinct from **`project`** (the working-directory name). A host conversation and a docker conversation can share the same `project` string if they reference the same repo.

---

## `.gitignore`

No new artifacts. No changes needed.
