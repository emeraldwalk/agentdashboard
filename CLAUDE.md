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
