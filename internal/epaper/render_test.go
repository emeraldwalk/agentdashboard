package epaper

import (
	"testing"
)

func TestRender_Dimensions(t *testing.T) {
	r := Renderer{}
	s := SessionSummary{
		PendingSessions:  3,
		DoneSessions:     7,
		ArchivedSessions: 42,
		PendingProjects:   []string{"proj-a", "proj-b", "proj-c"},
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
		PendingProjects: []string{"a", "b", "c", "d", "e", "f", "g"},
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
