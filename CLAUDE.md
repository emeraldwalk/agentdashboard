# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Verified API references

`llms/` contains full documentation dumps for libraries used in this project. Before writing any library API calls (CLI flags, config keys, function signatures) into a plan, verify them against the relevant file:

| File | Covers |
|---|---|
| `llms/oxc-llms-full.txt` | oxlint, oxfmt — CLI flags, config schema, VS Code integration |

Do not write API details into a plan that have not been verified against these files.

## Tasks

Tasks are stored in `.tasks/` (mounted from the host at container start). Use the `run_task_loop` shell function to process tasks:

```bash
run_task_loop
```

This function is defined in `~/.bashrc` inside the devcontainer and invokes the task-tracking skill.

## Build, Dev, Lint, Test

### Build

```bash
./scripts/build.sh
```

Builds the frontend (`npm ci && npm run build`) then Go binaries for all platforms into `bin/`. Do not use `go build` directly.

### Run

```bash
./agentdashboard          # detects OS/arch, runs the matching bin/agentdashboard-{os}-{arch}
./agentdashboard --help   # flags: --db, --dashboard-addr, --claude-dir, --docker-socket
```

Do not run the binary from `bin/` directly — always use `./agentdashboard`.

### Dev

Run two terminals:

```bash
# Terminal 1 — frontend (Vite on :5173)
cd frontend && npm run dev

# Terminal 2 — Go daemon (dashboard on :8080)
./scripts/dev-go.sh
```

### Lint

```bash
./scripts/lint.sh
```

Runs `golangci-lint run ./...` for Go and `npx oxlint . && npx oxfmt --check .` for the frontend.

### Test

```bash
./scripts/test.sh
# or for a single package:
go test ./internal/conversation/...
```

## Ports

| Port | Purpose |
|---|---|
| 8080 | Dashboard HTTP server (browser + API) |
| 5173 | Vite dev server (frontend development only) |

## SQLite Database

Default location: `~/.agentdashboard/sessions.db`

Override with the `--db` flag:

```bash
./scripts/dev-go.sh  # uses --db /tmp/agentdashboard-dev.db
```

## JSONL Source

The dashboard reads Claude Code session logs from:
1. **Host filesystem**: `~/.claude/projects/*.jsonl` (override with `--claude-dir`)
2. **Docker volumes**: containers with `claude-code-config-*` volumes mounted at `/home/vscode/.claude` (override socket with `--docker-socket`, default `/var/run/docker.sock`)
