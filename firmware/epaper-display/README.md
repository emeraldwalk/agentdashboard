# epaper-display firmware

PlatformIO/Arduino firmware for the Seeed Studio reTerminal E1001 (7.5" ePaper, 800×480, XIAO-ESP32S3).

The device acts as an HTTP server. Any client on the LAN can POST a PNG to a specific position on screen. Multiple clients can share the display by targeting non-overlapping regions — the firmware composites each push into a persistent framebuffer without clearing other regions.

## Hardware

- **Device:** Seeed Studio reTerminal E1001
- **Display:** 7.5" monochrome ePaper, 800×480
- **MCU:** XIAO-ESP32S3

Pin assignments (confirmed from [Handy4ndy/Handy-reTerminal-E1001](https://github.com/Handy4ndy/Handy-reTerminal-E1001)):

| Signal | GPIO |
| ------ | ---- |
| SCK    | 7    |
| MOSI   | 9    |
| CS     | 10   |
| DC     | 11   |
| RST    | 12   |
| BUSY   | 13   |

## API

### `POST /image`

Blits a PNG into the framebuffer at the given position and refreshes the display.

| Header         | Default | Description                              |
| -------------- | ------- | ---------------------------------------- |
| `X-Position-X` | `0`     | Left edge of the image in display pixels |
| `X-Position-Y` | `0`     | Top edge of the image in display pixels  |
| `Content-Type` | —       | `image/png`                              |

Body: raw PNG bytes. Any size is accepted as long as the image fits within 800×480 at the given origin. Returns `400` if out of bounds or not a valid PNG.

The framebuffer persists across requests — only the pixels covered by this image are changed. Other regions are untouched.

```bash
# Full screen (800×480)
curl -X POST http://$DEVICE_IP/image \
  -H "Content-Type: image/png" \
  --data-binary "@image.png"

# Bottom-right quadrant (400×240 image starting at x=400, y=240)
curl -X POST http://$DEVICE_IP/image \
  -H "Content-Type: image/png" \
  -H "X-Position-X: 400" \
  -H "X-Position-Y: 240" \
  --data-binary "@bottom-right.png"
```

### `POST /clear`

Fills the framebuffer with white and refreshes the display. No body required.

```bash
curl -X POST http://$DEVICE_IP/clear
```

## Serial log events

Connect at 115200 baud (`pio device monitor`). Key lines to grep for:

```
[BOOT] ePaper display firmware starting
[WIFI] Connected: 192.168.x.x
[WIFI] Starting AP: epaper-display
[HTTP] Listening on :80
[IMG]  Received N bytes
[IMG]  Decoded WxH at (X,Y)
[IMG]  Render complete
[IMG]  Screen cleared
[ERR]  <message>
```

## Wi-Fi provisioning

- **First boot** (no saved credentials): device starts an AP named `epaper-display` (no password) and serves a captive portal at `192.168.4.1`.
- Connect to the AP, browse to `http://192.168.4.1`, enter SSID + password, save. Device reboots and connects to your network.
- **Subsequent boots:** connects to saved network. Falls back to AP mode if connection fails after 10 seconds.
- Credentials are stored in NVS (survives power cycles). Reflash or full erase to reset.

## Toolchain setup

### Host (required for flashing)

Install the [PlatformIO IDE VS Code extension](https://marketplace.visualstudio.com/items?itemName=platformio.platformio-ide) — it bundles PlatformIO Core and esptool. No separate install needed.

USB drivers:

- **macOS:** nothing needed
- **Linux:** add your user to the `dialout` group: `sudo usermod -a -G dialout $USER`
- **Windows:** install the CH340/CP2102 driver for the XIAO's USB chip

### Devcontainer (compile + test, no flashing)

`pip install platformio` is run automatically in `postCreateCommand.sh`. The `pio` CLI is available in the terminal for everything except USB flashing.

## Key commands

```bash
cd firmware/epaper-display

# Compile
pio run -e xiao-esp32s3

# Compile + flash via USB-C (host only)
~/.platformio/penv/bin/pio run -e xiao-esp32s3 -t upload

# Serial monitor (host only, device must be connected)
~/.platformio/penv/bin/pio device monitor \
 --port /dev/cu.usbserial-1440 \
 --baud 115200


curl -X POST http://192.168.68.54/clear


curl -X POST http://192.168.68.54/image \
  -H "Content-Type: image/png" \
  --data-binary "@firmware/epaper-display/scripts/test-800x480.png"


curl -X POST http://192.168.68.54/image \
  -H "Content-Type: image/png" \
  --data-binary "@firmware/epaper-display/scripts/test-stripes-800x480.png"


# Host-side unit tests (no device required)
pio test -e native
```

## Testing

### Host-side unit tests (no device)

```bash
cd firmware/epaper-display
pio test -e native
```

Tests PNG header parsing: valid 800×480 accepted, wrong dimensions rejected, non-PNG rejected.

### Integration tests (device required)

Set `DEVICE_IP` to your device's LAN address, then run any of the named tests:

```bash
cd firmware/epaper-display

# Push a full 800×480 test image
DEVICE_IP=192.168.x.x ./scripts/test-push.sh full

# Push black 800×240 image to top half
DEVICE_IP=192.168.x.x ./scripts/test-push.sh top

# Push white 800×240 image to bottom half
DEVICE_IP=192.168.x.x ./scripts/test-push.sh bottom

# Push checkerboard 400×240 to top-right quadrant
DEVICE_IP=192.168.x.x ./scripts/test-push.sh quadrant

# Composite test: three sequential pushes to non-overlapping regions.
# Expected result: black top-left, checkerboard top-right, white bottom.
# Verifies that each push only affects its own region.
DEVICE_IP=192.168.x.x ./scripts/test-push.sh composite

# Clear screen to white
DEVICE_IP=192.168.x.x ./scripts/test-push.sh clear

# Push a custom image to a specific position
IMG=myimage.png X=0 Y=240 DEVICE_IP=192.168.x.x ./scripts/test-push.sh custom
```

### Test images

Pre-generated images in `scripts/`:

| File                            | Size    | Content                                    |
| ------------------------------- | ------- | ------------------------------------------ |
| `test-800x480.png`              | 800×480 | Solid grey — safe default full-screen test |
| `test-top-black.png`            | 800×240 | Solid black — top half                     |
| `test-bottom-white.png`         | 800×240 | Solid white — bottom half                  |
| `test-checkerboard-400x240.png` | 400×240 | Checkerboard — top-right quadrant          |
| `test-stripes-800x480.png`      | 800×480 | Diagonal stripes — full screen pattern     |

## Restoring stock firmware

Browser-based flasher, no tools required: [trmnl.com/flash](https://trmnl.com/flash) or [sensecraft.seeed.cc](https://sensecraft.seeed.cc)

## Project layout

```
firmware/epaper-display/
├── platformio.ini          # Build config: xiao-esp32s3 + native envs
├── src/
│   └── main.cpp            # All firmware: Wi-Fi, HTTP server, PNG decode, compositing
├── test/
│   └── test_image_decode.cpp  # Unity host-side tests
└── scripts/
    ├── test-push.sh        # curl-based integration tests
    ├── test-800x480.png
    ├── test-top-black.png
    ├── test-bottom-white.png
    ├── test-checkerboard-400x240.png
    └── test-stripes-800x480.png
```
