# Agent Dashboard — Master Plan

## Ready to Implement

| Plan | Description | Status |
| ---- | ----------- | ------ |
| [15 — ePaper Image Generator](pending/15-epaper-image-generator.md) | Go `internal/epaper` package: render session summary to 800×480 PNG using `fogleman/gg` | Ready |
| [16 — ePaper Sender](pending/16-epaper-sender.md) | Go sender: throttle + change detection, POST PNG to device; `--epaper-addr` CLI flag | Ready (after 15) |

## Completed

| Plan                                                                 | Description                                                                           | Status  |
| -------------------------------------------------------------------- | ------------------------------------------------------------------------------------- | ------- |
| [14 — ePaper Firmware](implemented/14-epaper-firmware.md) | PlatformIO Arduino firmware for reTerminal E1001: Wi-Fi provisioning, HTTP POST `/image` endpoint, GxEPD2 render | ✅ Done |
| [13 — Sub-agent Identification](implemented/13-subagent-identification.md) | Detect sub-agents via path nesting + JSONL fields; route them to Done, never Pending | ✅ Done |
| [12 — Kanban Board UI](implemented/12-kanban-board-ui.md) | Three-column kanban (Pending / Done / Archived); project cards split by session bucket | ✅ Done |
| [11 — Project Status Cards UI](implemented/11-project-status-cards-ui.md) | Group by project into status cards; host vs. docker indicator | ✅ Done |
| [10 — Stopped Container Discovery](implemented/10-stopped-container-discovery.md) | Ingest JSONL from stopped devcontainers via ephemeral alpine container | ✅ Done |
| [01 — Project Scaffold](implemented/01-project-scaffold.md)          | Go module, directory layout, Makefile, golangci-lint config, CLAUDE.md                | ✅ Done |
| [02 — Session Model & SQLite Store](implemented/02-session-store.md) | `internal/session` package: model, SQLite store, migrations                           | ✅ Done |
| [03 — OTLP HTTP Receiver](implemented/03-otlp-receiver.md)           | `internal/otlp` package: HTTP handlers for traces, metrics, logs                      | ✅ Done |
| [04 — SSE Broker](implemented/04-sse-broker.md)                      | `internal/dashboard/events.go`: fan-out broker wired to OTLP handler                  | ✅ Done |
| [05 — Dashboard HTTP Server](implemented/05-dashboard-server.md)     | `internal/dashboard/server.go`: static SPA serving, `/api/sessions`, `/api/events`    | ✅ Done |
| [06 — SolidJS Frontend](implemented/06-frontend.md)                  | Vite + SolidJS scaffold, `App.tsx`, `SessionCard.tsx`, SSE integration                | ✅ Done |
| [07 — Main Entry Point](implemented/07-main.md)                      | `cmd/agentdashboard/main.go`: wires all packages, CLI flags, graceful shutdown        | ✅ Done |
| [08 — Raw Event Capture](implemented/08-raw-event-capture.md)        | Store all OTLP payloads as JSON in `raw_events` table; `GET /api/raw-events` endpoint | ✅ Done |
| [09 — JSONL-Based Conversation Dashboard](implemented/09-jsonl-dashboard.md) | Replace OTLP with JSONL log file reader; Docker socket discovery; conversation model | ✅ Done |
