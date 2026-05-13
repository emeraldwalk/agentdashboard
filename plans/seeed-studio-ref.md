# Seeed Studio reTerminal E1001 — Setup Notes

7.5" monochrome ePaper display (800×480), XIAO-ESP32S3 based.

## Goal

Drive the display programmatically from a Go CLI that already serves an HTML dashboard, without using the proprietary SenseCraft HMI app.

## Recommended path: TRMNL firmware + BYOS

TRMNL ("Bring Your Own Server") gives the cleanest split:
- Device firmware handles deep sleep, OTA, Wi-Fi, image fetch loop.
- Your Go CLI just serves a 1-bit BMP over HTTP when the device polls.

No cloud account, no TRMNL service involvement at runtime once configured.

### Firmware setup (one time)

1. **Flash TRMNL firmware** from a Chrome/Edge browser via the Seeed web flasher over USB-C. No desktop app or SDK required.
2. **First boot:** device comes up as a Wi-Fi AP with a captive portal.
3. In the captive portal:
   - Pick your Wi-Fi network.
   - Click **Advanced** → **Custom Server: Yes**.
   - Paste your Go server's URL (no trailing slash).
   - Save.

### Changing the server URL later

- Hold the back button ~5 seconds → captive portal AP comes back.
- Join the AP, re-enter Wi-Fi creds, update Custom Server URL, save.
- No reflash, no cable. Device is offline during the swap.
- Easier pattern for dev/staging/prod: keep one stable URL and swap what the server returns.

### `.local` / mDNS

- Supported — TRMNL's own BYOS docs use `https://byos.local/api/...` in examples.
- Caveats:
  - One open issue in `byos_hanami` reports occasional resolution failures. Not bulletproof.
  - Server host has to advertise mDNS: macOS does this automatically (Bonjour); Linux needs `avahi-daemon`; Windows is spotty.
  - Device and server must share an L2 segment (no VLAN/subnet crossing without a reflector).
- Rule of thumb: `.local` is fine for same-box dev on home Wi-Fi. If it flakes, switch to a DHCP reservation + IP.

## Protocol — what the Go server implements

Device polls on a fixed cadence and sleeps between polls. Two endpoints to implement:

- `GET /api/setup` — initial device registration.
- `GET /api/display` — returns the BMP to render + the next wake interval.

Response is a 1-bit BMP at panel resolution (800×480 for E1001).

### HTML → bitmap pipeline (Go side)

1. **Render the HTML dashboard** to a PNG via headless Chromium (`chromedp`) at 800×480.
2. **Convert to 1-bit BMP** with Floyd–Steinberg dithering if you want grays to look reasonable.
3. **Serve from the HTTP handler** when the device polls.

### ePaper refresh constraints

- Full refresh ~2s — design for cadence in minutes, not seconds.
- Partial refresh looks ghosty.
- Battery life depends on poll interval; longer = better.

## Alternatives considered

| Option | Why not |
|---|---|
| Stock SenseCraft HMI firmware | Pushes you toward proprietary app/cloud for provisioning. |
| TRMNL hosted + private plugin | Dashboard must be internet-reachable; their rendering pipeline in the loop. BYOS keeps it local. |
| ESPHome / Home Assistant | Great for sensor data; awkward for "render this HTML." |
| Arduino / EEZ Studio custom firmware | Reinvents what TRMNL firmware already gives you. |

## References

- [Getting Started with reTerminal E1001 — Seeed Wiki](https://wiki.seeedstudio.com/getting_started_with_reterminal_e1001/)
- [Work with TRMNL — Seeed Wiki](https://wiki.seeedstudio.com/reterminal_e10xx_trmnl/)
- [Connect your Device to Terminus (BYOS)](https://help.trmnl.com/en/articles/12263392-connect-your-device-to-terminus-byos)
- [BYOS docs — TRMNL API](https://docs.trmnl.com/go/diy/byos)
- [BYOD/S protocol reference](https://docs.usetrmnl.com/go/diy/byod-s)
- [ESPHome Cookbook (reTerminal E Series)](https://wiki.seeedstudio.com/reterminal_e10xx_with_esphome/)
- [Arduino Cookbook](https://wiki.seeedstudio.com/reterminal_e10xx_with_arduino/)
- [Work with EEZ Studio](https://wiki.seeedstudio.com/reterminal_e10xx_with_eezstudio/)
- [byos_hanami (reference Ruby server)](https://github.com/usetrmnl/byos_hanami)
- [larapaper (Laravel server)](https://github.com/usetrmnl/larapaper)
- [Issue #118 — device mDNS resolution flake](https://github.com/usetrmnl/byos_hanami/issues/118)