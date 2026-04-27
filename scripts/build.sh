#!/usr/bin/env bash
set -euo pipefail

APP_NAME="agentdashboard"
PLATFORMS=("darwin/amd64" "darwin/arm64" "linux/amd64" "linux/arm64" "windows/amd64")

cd frontend && npm ci && npm run build
cd ..

# Copy frontend dist to internal/dashboard for embedding
rm -rf internal/dashboard/dist
cp -r frontend/dist internal/dashboard/dist

mkdir -p bin

for PLATFORM in "${PLATFORMS[@]}"; do
    OS=$(echo $PLATFORM | cut -d'/' -f1)
    ARCH=$(echo $PLATFORM | cut -d'/' -f2)
    OUTPUT="bin/${APP_NAME}-${OS}-${ARCH}"

    if [ "$OS" = "windows" ]; then
        OUTPUT="${OUTPUT}.exe"
    fi

    echo "Building for $OS/$ARCH..."
    GOOS=$OS GOARCH=$ARCH go build -o "$OUTPUT" ./cmd/agentdashboard
done

find bin/ -not -name "*.exe" -exec chmod +x {} +
