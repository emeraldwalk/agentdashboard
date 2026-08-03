# epaper-verse

Standalone CLI that renders a bible verse + citation to an 800×480 1-bit PNG and pushes it to the [ePaper display](../firmware/epaper-display/README.md) via `POST /image`. It runs independently of the main `agentdashboard` daemon — a one-shot push, not a background process.

## Build

```bash
./scripts/build.sh
```

Builds `bin/epaper-verse-{os}-{arch}` alongside the main binaries.

## Single verse

```bash
./bin/epaper-verse-linux-amd64 \
  -addr http://192.168.1.50 \
  -text "For God so loved the world, that he gave his only begotten Son, that whosoever believeth in him should not perish, but have everlasting life." \
  -citation "John 3:16"
```

Body text is word-wrapped and auto-sized (48pt down to 18pt) to fit the display; the citation is right-aligned at the bottom.

## Verse queue from a markdown file

Use `-file` to cycle through a list instead of passing `-text`/`-citation` directly:

```bash
./bin/epaper-verse-linux-amd64 -addr http://192.168.1.50 -file verses.md
```

Each run sends the next verse in the file and remembers its position, looping back to the first verse once the list is exhausted. This makes it easy to schedule (cron, launchd, etc.) for a new verse on a timer.

### File format

Verses are `## Citation` headings followed by the verse text (one or more lines, joined into a single paragraph, up to the next heading or end of file):

```markdown
## John 3:16
For God so loved the world, that he gave his only begotten Son,
that whosoever believeth in him should not perish, but have
everlasting life.

## Romans 8:28
And we know that all things work together for good to them that
love God, to them who are the called according to his purpose.

## Psalm 23:1
The LORD is my shepherd; I shall not want.
```

### Queue state

The current position is stored as JSON in `<file>.state` (e.g. `verses.md.state`) next to the markdown file:

```json
{"index":1}
```

- Override the location with `-state /path/to/state.json`.
- The index only advances after a successful send to the device — if the POST fails (device offline, network error), the same verse is retried on the next run instead of being skipped.
- Delete the state file to restart the queue from the beginning.

## Flags

```
-addr       ePaper device address, e.g. http://192.168.1.50 (required)
-text       Verse text (ignored if -file is set)
-citation   Verse citation, e.g. "John 3:16" (ignored if -file is set)
-file       Markdown file of verses; sends the next queued verse each run
-state      Queue state file path (default: <file>.state)
```

Either `-text`/`-citation` or `-file` is required.

## Scheduling example

Push a new verse every morning at 7am via cron:

```cron
0 7 * * * /path/to/bin/epaper-verse-linux-amd64 -addr http://192.168.1.50 -file /path/to/verses.md >> /var/log/epaper-verse.log 2>&1
```
