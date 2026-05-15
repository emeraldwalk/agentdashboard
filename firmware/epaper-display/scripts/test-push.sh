#!/usr/bin/env bash
# Test script for epaper-display firmware
#
# Usage:
#   DEVICE_IP=192.168.x.x ./scripts/test-push.sh [test]
#
# Tests:
#   full        — push full 800x480 test image (default)
#   top         — push black 800x240 image to top half (Y=0)
#   bottom      — push white 800x240 image to bottom half (Y=240)
#   quadrant    — push checkerboard 400x240 to top-right quadrant (X=400, Y=0)
#   composite   — run top + bottom + quadrant in sequence (tests non-clobbering)
#   clear       — wipe screen to white
#   custom      — push a custom file: IMG=path/to/file.png [X=0] [Y=0] ./scripts/test-push.sh custom

set -e

IP=${DEVICE_IP:?Usage: DEVICE_IP=192.168.x.x ./scripts/test-push.sh [test]}
DIR="$(dirname "$0")"
TEST=${1:-full}

push_image() {
    local file=$1 x=${2:-0} y=${3:-0}
    echo "→ POST /image  file=$(basename "$file")  x=$x y=$y"
    curl -sf -X POST "http://$IP/image" \
        -H "Content-Type: image/png" \
        -H "X-Position-X: $x" \
        -H "X-Position-Y: $y" \
        --data-binary "@$file"
    echo " OK"
}

case "$TEST" in
    full)
        push_image "$DIR/test-800x480.png" 0 0
        ;;
    top)
        push_image "$DIR/test-top-black.png" 0 0
        ;;
    bottom)
        push_image "$DIR/test-bottom-white.png" 0 240
        ;;
    quadrant)
        push_image "$DIR/test-checkerboard-400x240.png" 400 0
        ;;
    composite)
        echo "--- Composite test: top + bottom + top-right quadrant ---"
        echo "Step 1: black top half"
        push_image "$DIR/test-top-black.png" 0 0
        sleep 3
        echo "Step 2: white bottom half (top should still be black)"
        push_image "$DIR/test-bottom-white.png" 0 240
        sleep 3
        echo "Step 3: checkerboard top-right quadrant (other regions unchanged)"
        push_image "$DIR/test-checkerboard-400x240.png" 400 0
        echo "--- Done. Screen should show: checkerboard top-right, black top-left, white bottom ---"
        ;;
    clear)
        echo "→ POST /clear"
        curl -sf -X POST "http://$IP/clear"
        echo " OK"
        ;;
    custom)
        IMG=${IMG:?Usage: IMG=path/to/file.png [X=0] [Y=0] DEVICE_IP=... ./scripts/test-push.sh custom}
        push_image "$IMG" "${X:-0}" "${Y:-0}"
        ;;
    *)
        echo "Unknown test: $TEST"
        echo "Valid tests: full top bottom quadrant composite clear custom"
        exit 1
        ;;
esac
