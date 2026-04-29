# Debugging & Health Checks

## Docker socket — volume & container discovery

All commands talk to `/var/run/docker.sock` the same way the app does.

```bash
# All claude-code-config-* volumes
curl -s --unix-socket /var/run/docker.sock \
  http://docker/volumes \
  | jq '[.Volumes[] | select(.Name | startswith("claude-code-config-")) | .Name]'

# Running containers + which volumes they mount (name, destination)
curl -s --unix-socket /var/run/docker.sock \
  'http://docker/containers/json' \
  | jq '[.[] | {id: .Id[:12], mounts: [.Mounts[] | {name: .Name, dest: .Destination}]}]'

# Running containers that have a claude-code-config-* volume at /home/vscode/.claude
# (exactly what the app filters on)
curl -s --unix-socket /var/run/docker.sock \
  'http://docker/containers/json' \
  | jq '[.[] | select(.Mounts[] | (.Name // "" | startswith("claude-code-config-")) and .Destination == "/home/vscode/.claude") | {id: .Id[:12], project: (.Mounts[] | select(.Name // "" | startswith("claude-code-config-")).Name | ltrimstr("claude-code-config-"))}]'

# JSONL files inside a specific container (replace CONTAINER_ID)
CONTAINER_ID=abc123
curl -s --unix-socket /var/run/docker.sock \
  -X POST http://docker/containers/$CONTAINER_ID/exec \
  -H 'Content-Type: application/json' \
  -d '{"AttachStdout":true,"AttachStderr":false,"Cmd":["find","/home/vscode/.claude/projects","-name","*.jsonl"]}' \
  | jq -r '.Id' \
  | xargs -I{} curl -s --unix-socket /var/run/docker.sock \
      -X POST http://docker/exec/{}/start \
      -H 'Content-Type: application/json' \
      -d '{"Detach":false,"Tty":true}'
```

> The `find` exec output uses Docker's multiplexed stream format when `Tty:false`. Use `Tty:true` for readable raw output in a one-liner; the app itself strips the mux headers.

---

## Host JSONL files

```bash
# List all session JSONL files on the host
ls -lt ~/.claude/projects/*.jsonl 2>/dev/null | head -20

# Pretty-print the last 20 records from a file (replace path)
tail -20 ~/.claude/projects/<uuid>.jsonl | jq .

# Show only the types present in a file
jq -r '.type' ~/.claude/projects/<uuid>.jsonl | sort | uniq -c | sort -rn

# Show only actionable records (those that affect status/title)
jq 'select(.type == "user" or .type == "assistant" or .type == "queue-operation" or .type == "ai-title")' \
  ~/.claude/projects/<uuid>.jsonl

# Check what the app would derive as status (last actionable type)
jq -r 'select(.type == "user" or .type == "assistant" or .type == "queue-operation") | .type' \
  ~/.claude/projects/<uuid>.jsonl | tail -1
```

---

## SQLite database

Default dev DB: `/tmp/agentdashboard-dev.db`

```bash
# All conversations, most recent first
sqlite3 /tmp/agentdashboard-dev.db \
  "SELECT id, project, status, title, last_event_at FROM conversations ORDER BY last_event_at DESC;"

# Count by status
sqlite3 /tmp/agentdashboard-dev.db \
  "SELECT status, count(*) FROM conversations GROUP BY status;"

# Conversations updated in the last hour
sqlite3 /tmp/agentdashboard-dev.db \
  "SELECT id, project, status, last_event_at FROM conversations WHERE last_event_at > datetime('now', '-1 hour') ORDER BY last_event_at DESC;"

# Wipe and start fresh (triggers a full re-ingest on next app start)
rm /tmp/agentdashboard-dev.db
```

---

## HTTP API (app running on :8080)

```bash
# All conversations (pretty-printed, sorted by lastEventAt)
curl -s http://localhost:8080/api/conversations | jq 'sort_by(.lastEventAt) | reverse'

# Just IDs + status
curl -s http://localhost:8080/api/conversations | jq '[.[] | {id: .id[:8], project, status, title}]'

# SSE stream — watch live updates (Ctrl-C to stop)
curl -N -H 'Accept: text/event-stream' http://localhost:8080/api/events

# SSE stream filtered to conversation-update events only
curl -N -H 'Accept: text/event-stream' http://localhost:8080/api/events \
  | grep '^data:' | jq -R 'ltrimstr("data: ") | fromjson'
```
