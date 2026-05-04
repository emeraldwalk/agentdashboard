# Architecture

## Overview

`agentdashboard` is a Go daemon that reads Claude Code session logs (JSONL files) from the host filesystem and Docker container volumes, then serves a real-time web dashboard showing conversation status.

## Tech Stack

### Backend
| Component | Technology |
|---|---|
| Language | Go 1.22 |
| HTTP server | `net/http` stdlib |
| Frontend embedding | `embed` stdlib |
| SQLite driver | `modernc.org/sqlite` (pure Go, no CGo) |
| CORS middleware | `github.com/rs/cors` |

### Frontend
| Component | Technology |
|---|---|
| Framework | SolidJS (TypeScript) |
| Build tool | Vite |
| Real-time updates | Browser native `EventSource` (SSE) |

### Tooling
| Tool | Purpose |
|---|---|
| `golangci-lint` | Go linting (`errcheck`, `govet`, `staticcheck`, `goimports`, `revive`) |
| `oxlint` | Frontend JS/TS linting (general correctness; no React plugin — incompatible with SolidJS) |
| `oxfmt` | Frontend formatting (Prettier-compatible) |
| TypeScript strict mode | Primary SolidJS correctness gate (`"strict": true` in tsconfig) |
| Shell scripts | Build orchestration (see `scripts/`) |
| npm | Frontend package management |

## Data Flow

```
~/.claude/projects/*.jsonl          Docker container volumes
(host filesystem)                   (claude-code-config-* at /home/vscode/.claude)
        │                                        │
        │  fs notify / poll                      │  Docker API + exec
        ▼                                        ▼
  ┌──────────────┐                    ┌──────────────────┐
  │   Watcher    │                    │  Docker Source   │
  └──────┬───────┘                    └────────┬─────────┘
         │                                     │
         └──────────────┬──────────────────────┘
                        │  conversation.Conversation
                        ▼
               ┌─────────────────┐
               │  JSONL Parser   │  parse → extract ID, status, title
               └────────┬────────┘
                        │ store.Upsert(conversation)
                        ├──────────────────────────────────────┐
                        │                                      │
                        ▼                                      ▼
               ┌─────────────────┐                  ┌─────────────────┐
               │  SQLite Store   │                  │   SSE Broker    │
               │  sessions.db    │                  │  (fan-out chan) │
               └─────────────────┘                  └────────┬────────┘
                                                             │ conversation-update event
                                                             ▼
                                                    ┌─────────────────────┐
                                                    │  Dashboard Server   │  :8080
                                                    │  /api/conversations │◄── GET (initial load)
                                                    │  /api/events        │◄── EventSource (live)
                                                    │  /* (SPA)           │◄── browser
                                                    └─────────────────────┘
```

## Port Map

| Port | Purpose |
|---|---|
| 8080 | Dashboard HTTP server (browser + API) |
| 5173 | Vite dev server (frontend development only) |

## Package Layout

```
internal/conversation/  — Conversation model and SQLite store
internal/jsonl/         — JSONL file parser
internal/watcher/       — Host filesystem watcher (reads ~/.claude/projects/*.jsonl)
internal/docker/        — Docker volume source (reads JSONL from container volumes)
internal/dashboard/     — SSE broker and HTTP server (serves embedded SPA)
cmd/agentdashboard/     — Entry point: wires packages, CLI flags, graceful shutdown
frontend/               — SolidJS source; builds to dist/ which is embedded into the binary
scripts/                — Shell scripts for build, dev, lint, test
```

## Key Design Decisions

- **Single process, embedded frontend.** `//go:embed all:dist` bakes the built SPA into the Go binary — single-file deployment, no separate static server needed in production. Two terminals are only needed during active frontend development (for Vite's hot-reloading).
- **File-based ingestion, not OTLP.** The daemon reads Claude Code's JSONL session logs directly rather than receiving telemetry pushes. This requires no changes to Claude Code configuration.
- **SSE over WebSockets.** The dashboard is read-only; SSE is sufficient and requires no extra library.
- **Pure-Go SQLite.** `modernc.org/sqlite` avoids CGo, making cross-compilation straightforward.
