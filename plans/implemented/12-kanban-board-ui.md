# 12 — Kanban Board UI

## Goal

Replace the current single-column project list with a three-column kanban board: **Pending**, **Done**, and **Archived**. A project card can appear in multiple columns simultaneously, showing only the sessions relevant to that column. Cards within a column are sorted newest-to-oldest by the most recent session in that bucket.

## Column Definitions

| Column | Included sessions | Status values |
|--------|-------------------|---------------|
| **Pending** | Active or waiting sessions | `running`, `waiting_input` |
| **Done** | Finished sessions ≤ 2 days old | `stopped`, `failed` where `lastEventAt` is within 48 hours |
| **Archived** | Any session > 2 days old | any status where `lastEventAt` is older than 48 hours |

A single session belongs to exactly one column. The 48-hour boundary is evaluated client-side at render time (comparing `lastEventAt` to `Date.now()`).

A project appears in a column only if it has ≥ 1 session in that column. The same project name may produce a card in Pending, Done, and/or Archived simultaneously, each card showing only the sessions for that column.

Within a column, cards are sorted descending by the `lastEventAt` of their most-recent session in the column.

## Existing Touchpoints

| File | Role | Change |
|------|------|--------|
| `frontend/src/App.tsx` | Root component; builds `projectGroups` memo and renders cards | Replace single list with three-column layout and per-column grouping logic |
| `frontend/src/components/ProjectCard.tsx` | Renders one project's conversations | Receives a filtered `conversations` list; no structural change needed to the component itself |
| `frontend/src/types.ts` | `Conversation` type | No change; `status` and `lastEventAt` fields already exist |

No backend changes are required. The column assignment is pure frontend logic derived from existing `Conversation.status` and `Conversation.lastEventAt`.

## Contracts

### Column assignment function

```typescript
// Returns which column a conversation belongs to.
// "archived" wins over status — age check runs first.
function conversationColumn(conv: Conversation, now: number): "pending" | "done" | "archived" {
  const age = now - new Date(conv.lastEventAt).getTime();
  if (age > 48 * 60 * 60 * 1000) return "archived";
  if (conv.status === "running" || conv.status === "waiting_input") return "pending";
  return "done";
}
```

### Per-column grouping

```typescript
// Produces ProjectGroup[] for one column.
// columnConvs: only conversations that belong to this column.
// Groups by project, sorts each group's conversations newest-first,
// then sorts groups by their most-recent conversation newest-first.
function buildColumnGroups(columnConvs: Conversation[]): ProjectGroup[]
```

`ProjectGroup` is the existing interface from `frontend/src/components/ProjectCard.tsx` — no change to its shape.

### Layout

`App.tsx` renders three sibling column `<div>`s inside a flex container:

```
<main style="display: flex; gap: 16px; align-items: flex-start;">
  <KanbanColumn title="Pending" groups={pendingGroups()} />
  <KanbanColumn title="Done"    groups={doneGroups()} />
  <KanbanColumn title="Archived" groups={archivedGroups()} />
</main>
```

Each column takes equal width (`flex: 1`). A new `KanbanColumn` component (in `frontend/src/components/KanbanColumn.tsx`) renders the column title and a `<For>` loop of `<ProjectCard>` instances.

### KanbanColumn component

```typescript
// frontend/src/components/KanbanColumn.tsx
interface KanbanColumnProps {
  title: string;
  groups: ProjectGroup[];
}
function KanbanColumn(props: KanbanColumnProps): JSX.Element
```

Renders:
- A column header (`<h2>` or `<h3>`) with the column title and a count badge showing the number of cards.
- A scrollable list of `<ProjectCard group={…} />` instances.
- An empty-state message when `groups` is empty (e.g., "No sessions").

## Concept Boundaries

- **Column** — the three kanban buckets (Pending / Done / Archived). Not to be confused with CSS `flex-direction: column`, which is used inside each column for stacking cards.
- **ProjectGroup** — unchanged existing type; within a column context it only holds sessions belonging to that column.
- **Archived vs. Done** — age (>48 h) takes precedence over status. A `stopped` session that is 1 hour old is Done; the same session 3 days later is Archived. A `running` session > 48 h old goes to Archived, not Pending. This is intentional: stale active sessions are surfaced in Archived rather than polluting Pending.

## .gitignore

No new build artifacts or generated files. No `.gitignore` changes required.
