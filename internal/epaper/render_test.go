package epaper

import (
	"testing"
)

func TestRender_Dimensions(t *testing.T) {
	r := Renderer{}
	s := SessionSummary{
		ActiveSessions:  3,
		PendingSessions: 7,
		DoneSessions:    42,
		RecentProjects:  []string{"proj-a", "proj-b", "proj-c"},
	}
	img := r.Render(s)
	if img == nil {
		t.Fatal("Render returned nil")
	}
	bounds := img.Bounds()
	if bounds.Dx() != Width || bounds.Dy() != Height {
		t.Errorf("expected %dx%d, got %dx%d", Width, Height, bounds.Dx(), bounds.Dy())
	}
}

func TestRender_ZeroValues(t *testing.T) {
	r := Renderer{}
	img := r.Render(SessionSummary{})
	if img == nil {
		t.Fatal("Render returned nil for zero-value summary")
	}
}

func TestRender_TruncatesProjects(t *testing.T) {
	r := Renderer{}
	s := SessionSummary{
		RecentProjects: []string{"a", "b", "c", "d", "e", "f", "g"},
	}
	img := r.Render(s)
	if img == nil {
		t.Fatal("Render returned nil")
	}
	bounds := img.Bounds()
	if bounds.Dx() != Width || bounds.Dy() != Height {
		t.Errorf("expected %dx%d, got %dx%d", Width, Height, bounds.Dx(), bounds.Dy())
	}
}
