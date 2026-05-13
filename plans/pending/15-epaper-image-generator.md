# Plan 15 — ePaper Image Generator (Go)

## Checklist

- [ ] Create `internal/epaper/` package
- [ ] Implement `Renderer` — draws session summary to an 800×480 `image.RGBA` using `fogleman/gg`
- [ ] Implement `Dither` — converts RGBA → 1-bit BMP (Floyd–Steinberg)
- [ ] Implement `Encode` — writes final PNG to `[]byte` (or 1-bit BMP if firmware requires it)
- [ ] Write unit tests: render smoke test, dither correctness, encode round-trip
- [ ] Add `go.mod` dependency: `github.com/fogleman/gg`

## Context

The Go CLI already tracks session/conversation state. This plan adds the ability to render a visual summary of that state as an 800×480 PNG — without a browser or HTTP server. The image is generated in memory and handed to the sender (Plan 16).

This plan covers generation only. Triggering and sending are Plan 16.

## Package layout

```
internal/epaper/
├── render.go        # Renderer: session state → image.RGBA
├── render_test.go
├── dither.go        # Floyd–Steinberg RGBA → image.Gray (1-bit)
├── dither_test.go
└── encode.go        # image.RGBA → []byte (PNG)
```

## Contracts

### `Renderer`

```go
// internal/epaper/render.go

type SessionSummary struct {
    ActiveSessions   int
    PendingSessions  int
    DoneSessions     int
    RecentProjects   []string  // up to 5, truncated to fit
}

type Renderer struct{}

// Render draws a session summary onto an 800×480 image.
// Caller owns the returned image.
func (r Renderer) Render(s SessionSummary) *image.RGBA
```

Layout (approximate — implementer has latitude on exact positioning):
- Header bar: "Agent Dashboard" + timestamp, top 60px
- Three count badges: Active / Pending / Done, centered row
- Recent projects list: up to 5 lines, bottom half
- All text white on black background (ePaper renders black=ink, white=paper)

Font: use `gg`'s built-in font or embed a TTF via Go `embed`. Keep it simple — no external font files required unless the built-in is unreadably small.

### `Dither`

```go
// internal/epaper/dither.go

// Dither converts an RGBA image to a 1-bit grayscale image
// using Floyd-Steinberg dithering.
func Dither(src *image.RGBA) *image.Gray
```

Threshold: luminance < 128 → black, >= 128 → white. Floyd–Steinberg for smooth gradients.

### `Encode`

```go
// internal/epaper/encode.go

// EncodePNG encodes img to a PNG byte slice.
func EncodePNG(img image.Image) ([]byte, error)
```

PNG is the wire format for Plan 16 (sender). The firmware (Plan 14) accepts PNG. No BMP conversion needed in Go unless firmware testing reveals otherwise.

## Dependencies

`github.com/fogleman/gg` — 2D drawing (text, shapes, layout). Add to `go.mod`.

No headless browser. No CGo. Pure Go.

## Testing

`render_test.go`:
- Call `Render` with a populated `SessionSummary` → assert returned image is 800×480, non-nil.
- Call `Render` with zero values → assert no panic.

`dither_test.go`:
- All-white RGBA → all-white Gray.
- All-black RGBA → all-black Gray.
- Checkerboard → assert output pixel count is roughly half black.

`encode_test.go`:
- `EncodePNG(Render(...))` → `png.Decode` round-trip → assert dimensions preserved.
