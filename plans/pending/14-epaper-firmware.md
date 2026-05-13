# Plan 14 — ePaper Device Firmware

## Checklist

- [ ] Create PlatformIO project at `firmware/epaper-display/`
- [ ] Configure `platformio.ini` for XIAO-ESP32S3 with GxEPD2 dependency
- [ ] Implement Wi-Fi provisioning (captive portal on first boot, NVS persistence)
- [ ] Implement HTTP server with `POST /image` endpoint
- [ ] Implement image decode + GxEPD2 render pipeline
- [ ] Implement serial logging for all key events
- [ ] Write test script at `firmware/epaper-display/scripts/test-push.sh`
- [ ] Update `seeed-studio-ref.md` with confirmed display constructor params

## Context

Target device: Seeed Studio reTerminal E1001 — 7.5" ePaper, 800×480, XIAO-ESP32S3 based.

Goal: device acts as an HTTP server. Any client POSTs an 800×480 PNG and the device renders it immediately. No app, no cloud, no polling.

Reference reading (not a fork target):
- `omeriko9/E1001-reTerminal-Photo-Album` — similar HTTP POST approach; read for patterns
- `Handy4ndy/Handy-reTerminal-E1001` — working GxEPD2 sketches for E1001

## Toolchain

**PlatformIO** — not Arduino IDE. VS Code extension + `pio` CLI, fully headless.

Install: `pip install platformio` or via VS Code PlatformIO extension.

Key commands:
```bash
pio run -e xiao-esp32s3          # compile
pio run -e xiao-esp32s3 -t upload  # compile + flash (USB-C)
pio device monitor               # serial output
pio test -e native               # host-side unit tests
```

Restoring stock firmware at any time: browser flasher at `trmnl.com/flash` or `sensecraft.seeed.cc` — no tools required.

## Project layout

```
firmware/epaper-display/
├── platformio.ini
├── src/
│   └── main.cpp
├── lib/
│   └── (PlatformIO auto-downloads)
├── test/
│   └── test_image_decode.cpp    # host-side Unity tests
└── scripts/
    └── test-push.sh             # curl-based integration test
```

## `platformio.ini`

```ini
[env:xiao-esp32s3]
platform = espressif32
board = seeed_xiao_esp32s3
framework = arduino
monitor_speed = 115200
lib_deps =
    zinggjm/GxEPD2 @ ^1.5.0
    bblanchon/ArduinoJson @ ^7.0.0

[env:native]
platform = native
test_framework = unity
```

The `native` env runs host-side unit tests (no device required).

## Contracts

### HTTP endpoint

```
POST /image
Content-Type: image/png
Body: raw PNG bytes, must be 800×480

Response 200 OK     — accepted and rendering
Response 400        — wrong dimensions or bad format
```

No auth. LAN-only. Any HTTP client (curl, Go's http.Post) can drive it.

### Serial log events (115200 baud)

Key lines an agent or dev can grep for:
```
[WIFI] Connected: 192.168.x.x
[HTTP] Listening on :80
[IMG]  Received N bytes
[IMG]  Decoded 800x480
[IMG]  Render complete
[ERR]  <message>
```

### Wi-Fi provisioning

- On first boot (no saved creds): device starts AP `epaper-display` (no password), serves captive portal at `192.168.4.1`.
- Captive portal: HTML form to enter SSID + password. On submit, saves to NVS and reboots.
- On subsequent boots: connects to saved network. Falls back to AP mode if connection fails after 10s.
- NVS keys: `wifi_ssid`, `wifi_pass` (namespace `epaper`).

## Image pipeline

1. Receive POST body into a `uint8_t` buffer (heap-allocated, max 400KB).
2. Decode PNG to RGB using a lightweight C decoder — recommend `pngle` (single-header) or `libpng` if available in PlatformIO registry. Verify dimensions == 800×480; return 400 if not.
3. Convert RGB → 1-bit (threshold at 128, or simple Floyd–Steinberg dither).
4. Hand the 1-bit bitmap to `GxEPD2_BW` for full refresh.

GxEPD2 constructor for E1001 — **verify against `Handy4ndy` examples before coding**; exact pin assignments are hardware-specific and must be confirmed.

## Testing

### Without device (host)

Unit test `test/test_image_decode.cpp` using Unity + `env:native`:
- Feed a known 800×480 PNG → assert decoded dimensions
- Feed a non-PNG → assert error return
- Feed wrong dimensions → assert error return

Run: `pio test -e native`

### With device (integration)

`scripts/test-push.sh`:
```bash
#!/usr/bin/env bash
# Usage: DEVICE_IP=192.168.x.x ./scripts/test-push.sh [path/to/image.png]
set -e
IP=${DEVICE_IP:?set DEVICE_IP}
IMG=${1:-test-800x480.png}
curl -f -X POST "http://$IP/image" \
  -H "Content-Type: image/png" \
  --data-binary "@$IMG"
echo "Pushed $IMG to $IP"
```

Include a `test-800x480.png` (solid grey, 800×480) in `scripts/` as a safe default test image.

## `.gitignore` additions

```
firmware/epaper-display/.pio/
firmware/epaper-display/.vscode/
```
