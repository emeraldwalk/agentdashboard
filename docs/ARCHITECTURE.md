# Architecture

## Overview

`agentdashboard` is a Go daemon that receives OpenTelemetry data from AI agent tools (Claude Code, GitHub Copilot) and serves a real-time web dashboard showing session status.

## Tech Stack

### Backend
| Component | Technology |
|---|---|
| Language | Go 1.22 |
| HTTP servers | `net/http` stdlib (two muxes) |
| Frontend embedding | `embed` stdlib |
| OTLP protobuf types | `go.opentelemetry.io/proto/otlp` |
| Protobuf codec | `google.golang.org/protobuf` |
| SQLite driver | `modernc.org/sqlite` (pure Go, no CGo) |
| CORS middleware | `github.com/rs/cors` |

### Frontend
| Component | Technology |
|---|---|
| Framework | SolidJS (TypeScript) |
| Build tool | Vite |
| Real-time updates | Browser native `EventSource` (SSE) |

### Protocol
| Signal | Transport |
|---|---|
| Agent telemetry in | OTLP HTTP (`application/x-protobuf`) on port 4318 |
| Dashboard push | Server-Sent Events on `/api/events` port 8080 |

### Storage
| Component | Technology |
|---|---|
| Session state | SQLite at `~/.agentdashboard/sessions.db` |

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
Claude Code / Copilot
        │
        │  OTLP HTTP POST /v1/traces|metrics|logs
        │  port 4318
        ▼
  ┌─────────────┐
  │ OTLP Handler│  proto.Unmarshal → extract session.ID, status
  └──────┬──────┘
         │ store.Upsert(session)
         ├──────────────────────────────────────────┐
         │                                          │
         ▼                                          ▼
  ┌─────────────┐                         ┌─────────────────┐
  │ SQLite Store│                         │   SSE Broker    │
  │ sessions.db │                         │  (fan-out chan) │
  └─────────────┘                         └────────┬────────┘
                                                   │ session-update event
                                                   ▼
                                          ┌─────────────────┐
                                          │ Dashboard Server │  port 8080
                                          │  /api/sessions  │◄── GET (initial load)
                                          │  /api/events    │◄── EventSource (live)
                                          │  /* (SPA)       │◄── browser
                                          └─────────────────┘
```

## Port Map

| Port | Purpose |
|---|---|
| 4318 | OTLP HTTP receiver (agent telemetry ingress) |
| 8080 | Dashboard HTTP server (browser + API) |
| 5173 | Vite dev server (frontend development only) |

## Package Layout

```
internal/session/    — Session model and SQLite store
internal/otlp/       — OTLP HTTP handlers and protobuf parsers
internal/dashboard/  — SSE broker and HTTP server (serves embedded SPA)
cmd/agentdashboard/  — Entry point: wires packages, CLI flags, graceful shutdown
frontend/            — SolidJS source; builds to dist/ which is embedded into the binary
scripts/             — Shell scripts for build, lint, test
```

## Key Design Decisions

- **Two HTTP servers, one binary.** OTLP ingress (port 4318) and the dashboard (port 8080) run on separate `http.ServeMux` instances to keep the namespaces clean and match standard OTLP port conventions.
- **SSE over WebSockets.** The dashboard is read-only; SSE is sufficient and requires no extra library.
- **Pure-Go SQLite.** `modernc.org/sqlite` avoids CGo, making cross-compilation straightforward.
- **Frontend embedded at compile time.** `//go:embed all:dist` bakes the built SPA into the binary — single-file deployment, no separate static server needed in production.
- **No full OTel Collector.** Only `go.opentelemetry.io/proto/otlp` (protobuf schema) is imported — not the Collector framework — keeping the dependency footprint small.
