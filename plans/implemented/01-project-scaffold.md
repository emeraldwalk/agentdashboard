# 01 — Project Scaffold

## Checklist
- [x] Create directory structure
- [x] Write `go.mod` with module `github.com/emeraldwalk/agentdashboard`, go 1.22
- [x] Write `scripts/build.sh`, `scripts/dev-go.sh`, `scripts/lint.sh`, `scripts/test.sh`, `scripts/clean.sh`
- [x] Write `.golangci.yml`
- [x] Write root `CLAUDE.md`
- [x] Write `.gitignore`
- [x] Write `frontend/.oxlintrc.json`
- [x] Write `frontend/.oxfmtrc.json`

## Context

Bootstrap the repo layout. No existing files exist. This plan creates the skeleton that all other plans build on.

## Directory Layout

```
agentdashboard/
├── cmd/agentdashboard/        # entry point (created in plan 07)
├── internal/
│   ├── otlp/                  # OTLP receiver (plan 03)
│   ├── session/               # session model + store (plan 02)
│   └── dashboard/             # HTTP server + SSE broker (plans 04, 05)
├── frontend/                  # SolidJS app (plan 06)
├── docs/
│   └── ARCHITECTURE.md
├── plans/
├── scripts/
│   ├── build.sh
│   ├── dev-go.sh
│   ├── lint.sh
│   ├── test.sh
│   └── clean.sh
├── go.mod
├── .golangci.yml
├── .gitignore
└── CLAUDE.md
```

## Contracts

### `go.mod`

```
module github.com/emeraldwalk/agentdashboard

go 1.22
```

Dependencies added via `go get` (do not hand-edit `go.sum`):
- `go.opentelemetry.io/proto/otlp` — protobuf types for OTLP payloads
- `google.golang.org/protobuf` — proto decode/encode
- `modernc.org/sqlite` — pure-Go SQLite driver (no CGo)
- `github.com/rs/cors` — CORS middleware

### `scripts/`

All scripts must be `chmod +x` and start with `#!/usr/bin/env bash` and `set -euo pipefail`.

| Script | What it does |
|---|---|
| `build.sh` | `cd frontend && npm ci && npm run build`, then `go build -o bin/agentdashboard ./cmd/agentdashboard` |
| `dev-go.sh` | `go run ./cmd/agentdashboard --db /tmp/agentdashboard-dev.db` |
| `lint.sh` | `golangci-lint run ./...` and `cd frontend && npx oxlint . && npx oxfmt --check .` |
| `test.sh` | `go test ./...` |
| `clean.sh` | `rm -rf bin/ frontend/dist` |

Frontend dev server is started directly: `cd frontend && npm run dev`.

### `.golangci.yml`

Enable linters: `errcheck`, `govet`, `staticcheck`, `goimports`, `revive`.

### `frontend/.oxlintrc.json`

```json
{
  "$schema": "./node_modules/oxlint/configuration_schema.json",
  "categories": {
    "correctness": "error",
    "suspicious": "warn"
  }
}
```

Do not enable `--react-plugin` — its rules assume React's virtual DOM model and will produce false positives on SolidJS code. TypeScript strict mode (in `tsconfig.json`) is the primary correctness gate for SolidJS-specific patterns.

### `frontend/.oxfmtrc.json`

```json
{
  "$schema": "./node_modules/oxfmt/configuration_schema.json"
}
```

Default Prettier-compatible settings. Adjust `printWidth`, `tabWidth`, `singleQuote` here if needed.

### `.gitignore`

Ignore: `bin/`, `frontend/node_modules/`, `frontend/dist/`, `*.db`.

### `CLAUDE.md` (repo root)

Cover:
- Build: `./scripts/build.sh` (builds frontend then Go binary with embedded assets)
- Dev: two terminals — `cd frontend && npm run dev` (Vite on :5173) and `./scripts/dev-go.sh` (daemon on :4318 OTLP, :8080 dashboard)
- Lint: `./scripts/lint.sh`
- Test: `./scripts/test.sh` or `go test ./...` for a single package
- OTLP port: 4318 (HTTP only). Dashboard port: 8080.
- SQLite DB: `~/.agentdashboard/sessions.db` (override with `--db` flag)
- Protobuf decode pattern: body → `proto.Unmarshal` into `otlp/collector/*/v1` request type
