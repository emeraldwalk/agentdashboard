#!/usr/bin/env bash
set -euo pipefail

go run ./cmd/agentdashboard --db /tmp/agentdashboard-dev.db
