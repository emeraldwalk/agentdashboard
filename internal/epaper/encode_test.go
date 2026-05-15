package epaper

import (
	"bytes"
	"image/png"
	"testing"
)

func TestEncodePNG_RoundTrip(t *testing.T) {
	r := Renderer{}
	img := r.Render(SessionSummary{ActiveSessions: 1, PendingSessions: 2, DoneSessions: 3})

	data, err := EncodePNG(img)
	if err != nil {
		t.Fatalf("EncodePNG error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("EncodePNG returned empty slice")
	}

	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png.Decode error: %v", err)
	}

	bounds := decoded.Bounds()
	if bounds.Dx() != Width || bounds.Dy() != Height {
		t.Errorf("round-trip dimensions mismatch: got %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), Width, Height)
	}
}
