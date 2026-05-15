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
	ActiveSessions  int
	PendingSessions int
	DoneSessions    int
	RecentProjects  []string // up to 5, truncated to fit
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

	// Black background
	dc.SetRGB(0, 0, 0)
	dc.Clear()

	// Header bar
	dc.SetRGB(0.15, 0.15, 0.15)
	dc.DrawRectangle(0, 0, Width, 60)
	dc.Fill()

	// Header text
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
		{"Active", s.ActiveSessions},
		{"Pending", s.PendingSessions},
		{"Done", s.DoneSessions},
	}

	const badgeW = 160.0
	const badgeH = 80.0
	const gap = 40.0
	totalW := badgeW*float64(len(badges)) + gap*float64(len(badges)-1)
	startX := (float64(Width) - totalW) / 2
	const badgeY = 90.0

	for i, b := range badges {
		x := startX + float64(i)*(badgeW+gap)

		dc.SetRGB(0.2, 0.2, 0.2)
		dc.DrawRoundedRectangle(x, badgeY, badgeW, badgeH, 10)
		dc.Fill()

		dc.SetRGB(1, 1, 1)

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
	dc.SetRGB(0.3, 0.3, 0.3)
	dc.DrawRectangle(20, 195, float64(Width)-40, 1)
	dc.Fill()

	// Recent projects heading
	dc.SetRGB(0.6, 0.6, 0.6)
	if f := loadFont(14); f != nil {
		dc.SetFontFace(f)
	}
	dc.DrawString("Recent Projects", 20, 222)

	// Project list
	dc.SetRGB(1, 1, 1)
	if f := loadFont(18); f != nil {
		dc.SetFontFace(f)
	}
	projects := s.RecentProjects
	if len(projects) > 5 {
		projects = projects[:5]
	}
	for i, p := range projects {
		y := 255.0 + float64(i)*42
		dc.DrawString(p, 30, y)
	}

	return dc.Image().(*image.RGBA)
}
