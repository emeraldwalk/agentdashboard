# Plan 10 — Stopped Container Discovery

## Checklist

- [ ] Extend `findContainerForVolume` to query stopped containers via `?all=1`
- [ ] Add `readFilesViaTemporaryContainer` to spin up a short-lived container for stopped containers
- [ ] Wire stopped-container path into `discover`
- [ ] Skip volumes already handled by a running container

## Background

Currently `discover` in `internal/docker/source.go` only finds running containers (`GET /containers/json`). If a devcontainer is stopped, its `claude-code-config-*` volume exists but has no running container to exec into, so its sessions are never ingested.

## Approach

For stopped containers, exec is unavailable. The standard Docker API alternative is to create a temporary container with the volume mounted, run `find` + `cat` inside it, then remove it. The temporary container needs only a minimal image (`alpine` or `busybox`) that has `find` and `cat`.

The existing `execInContainer` / `processContainerFile` logic is reused unchanged — the only difference is the container is ephemeral and removed after use.

## Existing touchpoints

| File | Role |
|------|------|
| `internal/docker/source.go` | All changes are here |

No other files are modified.

## Contracts

### `findContainerForVolume` — changed signature (internal only)

Current: returns `(containerID string, err error)` — empty string means not found.

Change: add a `running bool` return value so the caller knows whether exec is available or a temporary container is needed.

```go
func (s *Source) findContainerForVolume(ctx context.Context, volumeName string) (containerID string, running bool, err error)
```

Query `http://docker/containers/json?all=1` instead of `http://docker/containers/json`. The Docker API returns a `State` field on each container object; use it to set `running`.

Container JSON shape (relevant fields only):
```json
{ "Id": "...", "State": "running", "Mounts": [{ "Name": "...", "Destination": "..." }] }
```

`State == "running"` → running container, exec is safe.  
Any other state → stopped/paused/exited, use temporary container path.

### `discover` — updated call site

```go
containerID, running, err := s.findContainerForVolume(ctx, vol)
if err != nil || containerID == "" {
    // no container has this volume at all — skip
    continue
}
if running {
    s.tailFilesInContainer(ctx, containerID, project)
} else {
    s.readFilesViaTemporaryContainer(ctx, vol, project)
}
```

### `readFilesViaTemporaryContainer` — new method

```go
func (s *Source) readFilesViaTemporaryContainer(ctx context.Context, volumeName, project string)
```

Steps:
1. `POST /containers/create` — body mounts `volumeName` at `/data`, uses image `alpine`, cmd `["sleep", "30"]`. Record the returned container ID.
2. `POST /containers/{id}/start`
3. Call `tailFilesInContainer(ctx, id, project)` with path `/data/projects` instead of `/home/vscode/.claude/projects`.
4. `DELETE /containers/{id}?force=true` — always, even on error.

**Container create body:**
```json
{
  "Image": "alpine",
  "Cmd": ["sleep", "30"],
  "HostConfig": {
    "Binds": ["<volumeName>:/data"],
    "AutoRemove": false
  }
}
```

`AutoRemove: false` — we issue the explicit DELETE ourselves so we can confirm cleanup in logs.

**Path difference:** the volume root is `/data` not `/home/vscode/.claude`, so the `find` path passed to `tailFilesInContainer` must be `/data/projects`.

Extract the find-path from `tailFilesInContainer` into a parameter:

```go
// current (implicit path):
func (s *Source) tailFilesInContainer(ctx context.Context, containerID, project string)
// find path hardcoded as "/home/vscode/.claude/projects"

// new (explicit path):
func (s *Source) tailFilesInContainer(ctx context.Context, containerID, project, findRoot string)
```

Existing call site passes `"/home/vscode/.claude/projects"`; temporary container call site passes `"/data/projects"`.

### Deduplication

`discover` is called every 30 seconds. A volume with no running container will re-trigger `readFilesViaTemporaryContainer` on every tick. This is harmless because `handler.OnConversation` is idempotent (upserts by session ID), but it wastes resources.

Track processed volumes in a `map[string]struct{}` field on `Source` (named `ingestedVolumes`). After successfully reading a stopped volume, add it. Skip on subsequent ticks. Running containers are not added — they may receive new sessions while running.

```go
type Source struct {
    socketPath      string
    handler         watcher.Handler
    client          *http.Client
    ingestedVolumes map[string]struct{}  // volumes fully read (stopped containers only)
}
```

Initialize in `New`. Access is single-goroutine (only `discover` touches it), no mutex needed.

## Concept boundaries

- `tailFilesInContainer` — currently implies a running container. After this plan it works for any container (running or temporary). The name stays; the assumption changes.
- "temporary container" — the ephemeral alpine container created solely to read a volume. Not the same as the devcontainer that owns the volume.

## .gitignore

No new build artifacts or generated files.
