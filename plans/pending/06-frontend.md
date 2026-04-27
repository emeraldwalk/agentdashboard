# 06 — SolidJS Frontend

## Checklist
- [ ] Scaffold `frontend/` with `npm create vite@latest frontend -- --template solid-ts`
- [ ] Configure `vite.config.ts` with dev proxy
- [ ] Write `src/App.tsx`
- [ ] Write `src/components/SessionCard.tsx`
- [ ] Verify `npm run build` produces `dist/`

## Context

Single-page dashboard showing live agent session status. Bootstrapped from the official Vite + SolidJS TypeScript template. No routing library needed — single view.

Real-time updates come from the Go SSE endpoint (`/api/events`). Initial state is fetched from `/api/sessions` on mount.

## Existing Touchpoints

- `frontend/dist/` — build output consumed by plan 05 (embedded into Go binary)
- Go SSE endpoint `GET /api/events` — emits `session-update` events (plan 05)
- Go REST endpoint `GET /api/sessions` — returns `Session[]` JSON (plan 05)

## Contracts

### JSON shape from `/api/sessions`

```ts
interface Session {
  id: string;
  agentName: string;
  status: "running" | "waiting_input" | "stopped" | "failed";
  startedAt: string;   // ISO 8601
  lastEventAt: string; // ISO 8601
}
```

### `vite.config.ts` dev proxy

```ts
server: {
  proxy: {
    "/api": "http://localhost:8080",
    "/v1":  "http://localhost:4318",
  }
}
```

### `src/App.tsx`

- On mount: `fetch("/api/sessions")` → set signal `sessions`.
- Open `new EventSource("/api/events")`.
- On `session-update` event: parse JSON, upsert into `sessions` signal by `id`.
- Render a list of `<SessionCard>` components, one per session.
- Show a "No sessions" message when the list is empty.

### `src/components/SessionCard.tsx`

Props: `session: Session`

Displays:
- `agentName` — agent name
- `status` — as a colored badge:
  - `running` → green
  - `waiting_input` → yellow
  - `stopped` → gray
  - `failed` → red
- `lastEventAt` — relative time (e.g. "2 min ago"); re-renders every 30 s via `setInterval`

No external date library needed — compute relative time inline.

## Concept Boundaries

- The frontend communicates **only** via `/api/sessions` and `/api/events`. It never posts to `/v1/*` (those are OTLP ingress endpoints for agents, not the browser).
- Do not add a router (e.g. solid-router). The app is a single view.
