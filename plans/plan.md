# Agent Dashboard — Master Plan

## Ready to Implement

| Plan | Description |
|---|---|
| [03 — OTLP HTTP Receiver](pending/03-otlp-receiver.md) | `internal/otlp` package: HTTP handlers for traces, metrics, logs |
| [04 — SSE Broker](pending/04-sse-broker.md) | `internal/dashboard/events.go`: fan-out broker wired to OTLP handler |
| [05 — Dashboard HTTP Server](pending/05-dashboard-server.md) | `internal/dashboard/server.go`: static SPA serving, `/api/sessions`, `/api/events` |
| [06 — SolidJS Frontend](pending/06-frontend.md) | Vite + SolidJS scaffold, `App.tsx`, `SessionCard.tsx`, SSE integration |
| [07 — Main Entry Point](pending/07-main.md) | `cmd/agentdashboard/main.go`: wires all packages, CLI flags, graceful shutdown |

## Completed

| Plan | Description | Status |
|---|---|---|
| [01 — Project Scaffold](implemented/01-project-scaffold.md) | Go module, directory layout, Makefile, golangci-lint config, CLAUDE.md | ✅ Done |
| [02 — Session Model & SQLite Store](implemented/02-session-store.md) | `internal/session` package: model, SQLite store, migrations | ✅ Done |
