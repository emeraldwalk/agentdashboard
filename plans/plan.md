# Agent Dashboard — Master Plan

## Ready to Implement

| Plan | Description | Status |
|------|-------------|--------|

## Completed

| Plan                                                                 | Description                                                                           | Status  |
| -------------------------------------------------------------------- | ------------------------------------------------------------------------------------- | ------- |
| [01 — Project Scaffold](implemented/01-project-scaffold.md)          | Go module, directory layout, Makefile, golangci-lint config, CLAUDE.md                | ✅ Done |
| [02 — Session Model & SQLite Store](implemented/02-session-store.md) | `internal/session` package: model, SQLite store, migrations                           | ✅ Done |
| [03 — OTLP HTTP Receiver](implemented/03-otlp-receiver.md)           | `internal/otlp` package: HTTP handlers for traces, metrics, logs                      | ✅ Done |
| [04 — SSE Broker](implemented/04-sse-broker.md)                      | `internal/dashboard/events.go`: fan-out broker wired to OTLP handler                  | ✅ Done |
| [05 — Dashboard HTTP Server](implemented/05-dashboard-server.md)     | `internal/dashboard/server.go`: static SPA serving, `/api/sessions`, `/api/events`    | ✅ Done |
| [06 — SolidJS Frontend](implemented/06-frontend.md)                  | Vite + SolidJS scaffold, `App.tsx`, `SessionCard.tsx`, SSE integration                | ✅ Done |
| [07 — Main Entry Point](implemented/07-main.md)                      | `cmd/agentdashboard/main.go`: wires all packages, CLI flags, graceful shutdown        | ✅ Done |
| [08 — Raw Event Capture](implemented/08-raw-event-capture.md)        | Store all OTLP payloads as JSON in `raw_events` table; `GET /api/raw-events` endpoint | ✅ Done |
| [09 — JSONL-Based Conversation Dashboard](implemented/09-jsonl-dashboard.md) | Replace OTLP with JSONL log file reader; Docker socket discovery; conversation model | ✅ Done |
