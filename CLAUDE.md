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
./agentdashboard --help   # flags: --db, --otlp-addr, --dashboard-addr
```

Do not run the binary from `bin/` directly — always use `./agentdashboard`.

### Dev

Run two terminals:

```bash
# Terminal 1 — frontend (Vite on :5173)
cd frontend && npm run dev

# Terminal 2 — Go daemon (OTLP on :4318, dashboard on :8080)
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
go test ./internal/session/...
```

## Ports

| Port | Purpose |
|---|---|
| 4318 | OTLP HTTP receiver (agent telemetry ingress, HTTP only) |
| 8080 | Dashboard HTTP server (browser + API) |
| 5173 | Vite dev server (frontend development only) |

## SQLite Database

Default location: `~/.agentdashboard/sessions.db`

Override with the `--db` flag:

```bash
./scripts/dev-go.sh  # uses --db /tmp/agentdashboard-dev.db
```

## Protobuf Decode Pattern

```go
body, _ := io.ReadAll(r.Body)
var req collectorv1.ExportTraceServiceRequest
proto.Unmarshal(body, &req)
```

Import the request type from `go.opentelemetry.io/proto/otlp/collector/trace/v1` (or `metrics`, `logs`). Use `google.golang.org/protobuf/proto` for `proto.Unmarshal`.
