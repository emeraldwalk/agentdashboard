#!/usr/bin/env bash
set -euo pipefail

cd frontend && npm ci && npm run build
cd ..
go build -o bin/agentdashboard ./cmd/agentdashboard
