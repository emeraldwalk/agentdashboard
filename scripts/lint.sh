#!/usr/bin/env bash
set -euo pipefail

golangci-lint run ./...
cd frontend && npx oxlint . && npx oxfmt --check .
