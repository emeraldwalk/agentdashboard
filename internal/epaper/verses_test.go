package epaper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseVersesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "verses.md")
	mustWriteFile(t, path, `## John 3:16
For God so loved the world, that he gave
his only begotten Son.

## Romans 8:28
And we know that all things work together for good.
`)

	verses, err := ParseVersesFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(verses) != 2 {
		t.Fatalf("got %d verses, want 2", len(verses))
	}
	if verses[0].Citation != "John 3:16" {
		t.Errorf("citation = %q", verses[0].Citation)
	}
	if verses[0].Text != "For God so loved the world, that he gave his only begotten Son." {
		t.Errorf("text = %q", verses[0].Text)
	}
	if verses[1].Citation != "Romans 8:28" {
		t.Errorf("citation = %q", verses[1].Citation)
	}
}

func TestParseVersesFileEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "verses.md")
	mustWriteFile(t, path, "just some text, no headings\n")

	if _, err := ParseVersesFile(path); err == nil {
		t.Fatal("expected error for file with no headings")
	}
}

func TestQueueIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "verses.md.state")

	idx, err := LoadQueueIndex(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if idx != 0 {
		t.Fatalf("initial index = %d, want 0", idx)
	}

	if err := SaveQueueIndex(statePath, 3); err != nil {
		t.Fatal(err)
	}
	idx, err = LoadQueueIndex(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if idx != 3 {
		t.Fatalf("index = %d, want 3", idx)
	}
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
