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
	dc.DrawString("Agent Dashboard", 20, 40)
	ts := time.Now().Format("2006-01-02 15:04")
	tw, _ := dc.MeasureString(ts)
	dc.DrawString(ts, float64(Width)-tw-20, 40)

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
