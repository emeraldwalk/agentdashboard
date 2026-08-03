package epaper

import (
	"fmt"
	"image"
	"time"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

const (
	Width  = 800
	Height = 480

	TimePatchX = 0   // left edge of the time strip in display coordinates
	timePatchW = 160 // wide enough for any HH:MM string at 24pt
	timePatchH = 60  // full header bar height
)

type SessionSummary struct {
	PendingSessions  int
	DoneSessions     int
	ArchivedSessions int
	PendingProjects  []string
	DoneProjects     []string
	ArchivedProjects []string
}

type Renderer struct{}

func loadFont(points float64) font.Face {
	tt, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil
	}
	face, err := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    points,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil
	}
	return face
}

// Render draws a session summary onto an 800×480 image.
// Caller owns the returned image.
func (r Renderer) Render(s SessionSummary) *image.RGBA {
	dc := gg.NewContext(Width, Height)

	// White background
	dc.SetRGB(1, 1, 1)
	dc.Clear()

	// Header bar (black)
	dc.SetRGB(0, 0, 0)
	dc.DrawRectangle(0, 0, Width, 60)
	dc.Fill()

	// Header text (white on black)
	dc.SetRGB(1, 1, 1)
	if f := loadFont(24); f != nil {
		dc.SetFontFace(f)
	}
	ts := time.Now().Format("15:04")
	dc.DrawString(ts, 20, 40)

	// Count badges row
	badges := []struct {
		label string
		count int
	}{
		{"Pending", s.PendingSessions},
		{"Done", s.DoneSessions},
		{"Archived", s.ArchivedSessions},
	}

	const badgeW = 160.0
	const badgeH = 80.0
	const gap = 40.0
	totalW := badgeW*float64(len(badges)) + gap*float64(len(badges)-1)
	startX := (float64(Width) - totalW) / 2
	const badgeY = 90.0

	for i, b := range badges {
		x := startX + float64(i)*(badgeW+gap)

		// Badge outline (black border, white fill)
		dc.SetRGB(0, 0, 0)
		dc.DrawRoundedRectangle(x, badgeY, badgeW, badgeH, 10)
		dc.SetLineWidth(2)
		dc.Stroke()

		dc.SetRGB(0, 0, 0)

		if f := loadFont(36); f != nil {
			dc.SetFontFace(f)
		}
		countStr := fmt.Sprintf("%d", b.count)
		cw, _ := dc.MeasureString(countStr)
		dc.DrawString(countStr, x+badgeW/2-cw/2, badgeY+50)

		if f := loadFont(14); f != nil {
			dc.SetFontFace(f)
		}
		lw, _ := dc.MeasureString(b.label)
		dc.DrawString(b.label, x+badgeW/2-lw/2, badgeY+72)
	}

	// Divider
	dc.SetRGB(0, 0, 0)
	dc.DrawRectangle(20, 195, float64(Width)-40, 1)
	dc.Fill()

	// Three project columns: Pending | Done | Archived
	const colY = 210.0
	const colRowH = 28.0
	const maxRows = 8
	const colW = float64(Width) / 3

	cols := []struct {
		label    string
		projects []string
	}{
		{"PENDING", s.PendingProjects},
		{"DONE", s.DoneProjects},
		{"ARCHIVED", s.ArchivedProjects},
	}

	if f := loadFont(13); f != nil {
		dc.SetFontFace(f)
	}
	for i, col := range cols {
		x := float64(i)*colW + 16

		dc.SetRGB(0, 0, 0)
		dc.DrawString(col.label, x, colY+14)

		// Underline the column header
		dc.DrawRectangle(x, colY+18, colW-20, 1)
		dc.Fill()

		if f := loadFont(14); f != nil {
			dc.SetFontFace(f)
		}
		projects := col.projects
		if len(projects) > maxRows {
			projects = projects[:maxRows]
		}
		for j, p := range projects {
			dc.DrawString(p, x, colY+18+float64(j+1)*colRowH)
		}
	}

	return dc.Image().(*image.RGBA)
}

// RenderVerse draws body text with a right-aligned citation onto an 800×480
// image, sized to fill the display. Body text is word-wrapped and centered.
func (r Renderer) RenderVerse(text, citation string) *image.RGBA {
	dc := gg.NewContext(Width, Height)

	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)

	const margin = 60.0
	const citationH = 50.0
	maxWidth := float64(Width) - 2*margin

	bodySize := fitFontSize(dc, text, maxWidth, float64(Height)-citationH-2*margin)
	if f := loadFont(bodySize); f != nil {
		dc.SetFontFace(f)
	}
	dc.DrawStringWrapped(text, float64(Width)/2, (float64(Height)-citationH)/2,
		0.5, 0.5, maxWidth, 1.4, gg.AlignCenter)

	if f := loadFont(20); f != nil {
		dc.SetFontFace(f)
	}
	cw, _ := dc.MeasureString(citation)
	dc.DrawString(citation, float64(Width)-margin-cw, float64(Height)-margin+10)

	return dc.Image().(*image.RGBA)
}

// fitFontSize picks the largest font size (within a fixed range) whose
// word-wrapped rendering of text fits within maxHeight at the given width.
func fitFontSize(dc *gg.Context, text string, maxWidth, maxHeight float64) float64 {
	const maxSize = 48.0
	const minSize = 18.0
	const lineSpacing = 1.4

	for size := maxSize; size >= minSize; size -= 2 {
		if f := loadFont(size); f != nil {
			dc.SetFontFace(f)
		}
		lines := dc.WordWrap(text, maxWidth)
		_, lineH := dc.MeasureString("Ag")
		totalH := float64(len(lines)) * lineH * lineSpacing
		if totalH <= maxHeight {
			return size
		}
	}
	return minSize
}

// RenderTimePatch renders just the time text on a black background, sized to
// overwrite the top-right corner of the header bar. The returned image is
// positioned at x=TimePatchX, y=0 in display coordinates.
func (r Renderer) RenderTimePatch() *image.RGBA {
	dc := gg.NewContext(timePatchW, timePatchH)
	dc.SetRGB(0, 0, 0)
	dc.Clear()
	dc.SetRGB(1, 1, 1)
	if f := loadFont(24); f != nil {
		dc.SetFontFace(f)
	}
	ts := time.Now().Format("15:04")
	dc.DrawString(ts, 20, 40)
	return dc.Image().(*image.RGBA)
}
