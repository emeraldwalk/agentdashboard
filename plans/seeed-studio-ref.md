# Seeed Studio reTerminal E1001 — Setup Notes

7.5" monochrome ePaper display (800×480), XIAO-ESP32S3 based.

## Goal

Drive the display from a Go CLI that generates an image and pushes it to the device on demand. The device acts as the HTTP endpoint — any tool can POST an image to it. No proprietary app required.

## Architecture: device as HTTP server (push model)

The stock TRMNL firmware uses a **pull model**: the device wakes, polls a server for an image, sleeps. That's the wrong shape for this use case.

The desired model is the inverse:
- **Device listens** on a local HTTP endpoint.
- **Clients push** a prerendered image via `POST /`.
- The Go CLI generates an image only when state changes (throttled) and sends it.

This requires custom firmware.

### Firmware: custom HTTP server on device

The ESP32-S3 is capable enough to run an embedded HTTP server. The recommended stack uses officially-endorsed components:

- **[GxEPD2](https://github.com/ZinggJM/GxEPD2)** — the display library Seeed explicitly recommends for custom E1001 projects. Well-maintained, wide ePaper device support.
- **Arduino-ESP32 WebServer** — built into the Arduino-ESP32 package. Simple `server.on("/image", HTTP_POST, handler)` pattern. Seeed's wiki uses the Arduino path.

The firmware is roughly 200–300 lines of Arduino code:
1. Connect to Wi-Fi (captive portal for provisioning).
2. Start HTTP server, register `POST /image`.
3. Read body bytes → decode → dither → hand to GxEPD2.

**Reference reading** (not a dependency): [omeriko9/E1001-reTerminal-Photo-Album](https://github.com/omeriko9/E1001-reTerminal-Photo-Album) solves the same problem with an HTTP POST endpoint — useful for understanding the structure. Low trust as a fork target (1 star, single contributor, no community review), but fine to read for patterns. [Handy4ndy/Handy-reTerminal-E1001](https://github.com/Handy4ndy/Handy-reTerminal-E1001) has good working GxEPD2 display sketches for the E1001.

**One-time setup:**
1. Flash firmware via Arduino IDE over USB-C.
2. Device comes up as AP with captive portal — set Wi-Fi creds.
3. Note device IP (or configure `.local` via mDNS).

### Push endpoint contract

```
POST http://<device-ip>/image
Content-Type: image/png   (or image/bmp)
Body: raw image bytes, 800×480
```

Response: `200 OK` on success. Device renders immediately.

Any tool that can do an HTTP POST can drive the display. The CLI is just one client.

### mDNS

If the device advertises `reterminal.local` (or similar), clients use that instead of a raw IP.
- macOS: works out of the box (Bonjour).
- Linux: requires `avahi-daemon`.
- Windows: spotty.

For reliability on a home network, set a DHCP reservation and use the IP.

---

## Go CLI integration

The CLI already tracks session/conversation state. The ePaper integration adds:

1. **Image generator** — renders a "dashboard summary" as an 800×480 PNG (not served over HTTP, just generated in memory or to a temp file). No headless browser needed — draw directly with `image/png` + `golang.org/x/image/draw` or a simple layout library.

2. **Change detector + throttle** — watches for meaningful state changes (new session, status change, etc.). Throttles sends to respect ePaper refresh rate (~2s full refresh; design for a minimum cadence of 30–60s).

3. **Push sender** — `POST`s the PNG to the device endpoint. Simple `http.Post` call.

### Image generation approach

Avoid headless Chromium (`chromedp`) for this — it's heavy and unnecessary for a static layout. Instead, render directly in Go:

- [`fogleman/gg`](https://github.com/fogleman/gg) — 2D drawing with text, shapes, easy layout.
- Standard library `image/png` for encoding.
- 1-bit dithering can be done on the device (preferred) or in Go with a simple Floyd–Steinberg pass if the firmware doesn't handle it.

If the firmware accepts pre-dithered 1-bit BMP, convert the PNG after drawing:
1. Draw to `image.RGBA` with `gg`.
2. Dither to `image.Gray` (threshold or Floyd–Steinberg).
3. Encode as 1-bit BMP with `golang.org/x/image/bmp` (or a small custom encoder).

### Throttle design

```
minInterval = 60s   // ePaper full refresh + margin
maxInterval = 5m    // force a refresh even if nothing changed
```

On state change: if `now - lastSent > minInterval`, send immediately; otherwise, schedule send at `lastSent + minInterval`.

---

## Alternatives considered

| Option | Why not |
|---|---|
| TRMNL BYOS (pull model) | Device polls server — wrong shape. We want to push from CLI on state change. |
| TRMNL Webhook Image plugin | Pseudo-push: you POST to TRMNL's cloud, device polls from there. Rate limited to 12/hr. External dependency. |
| Stock SenseCraft HMI firmware | Proprietary app required for provisioning. |
| ESPHome | Primarily sensor-oriented; push model requires Home Assistant integration. |
| Hosted TRMNL service | Dashboard must be internet-reachable. |

---

## References

- [Getting Started with reTerminal E1001 — Seeed Wiki](https://wiki.seeedstudio.com/getting_started_with_reterminal_e1001/)
- [Work with TRMNL — Seeed Wiki](https://wiki.seeedstudio.com/reterminal_e10xx_trmnl/)
- [GxEPD2](https://github.com/ZinggJM/GxEPD2) — Seeed-recommended ePaper display library for E1001 (use this)
- [Handy4ndy/Handy-reTerminal-E1001](https://github.com/Handy4ndy/Handy-reTerminal-E1001) — community GxEPD2 sketches for E1001; good display driver reference
- [omeriko9/E1001-reTerminal-Photo-Album](https://github.com/omeriko9/E1001-reTerminal-Photo-Album) — hobby project with HTTP POST endpoint; read for patterns only (1 star, unreviewed)
- [Frans-Willem/reterminal_e100x](https://github.com/Frans-Willem/reterminal_e100x) — experimental Rust firmware; pull model, E1001 support unconfirmed
- [TRMNL Webhook Image plugin](https://help.trmnl.com/en/articles/13213669-webhook-image) — cloud-relay pseudo-push (rate limited to 12/hr)
- [BYOS docs — TRMNL API](https://docs.trmnl.com/go/diy/byos)
- [BYOD/S protocol reference](https://docs.usetrmnl.com/go/diy/byod-s)
- [ESPHome Cookbook (reTerminal E Series)](https://wiki.seeedstudio.com/reterminal_e10xx_with_esphome/)
- [fogleman/gg — Go 2D drawing library](https://github.com/fogleman/gg)
- [byos_hanami (reference Ruby BYOS server)](https://github.com/usetrmnl/byos_hanami)
