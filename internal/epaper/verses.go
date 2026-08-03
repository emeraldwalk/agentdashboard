package epaper

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Verse is a single scripture passage with its citation.
type Verse struct {
	Citation string
	Text     string
}

// ParseVersesFile reads a markdown file listing verses as "## Citation"
// headings followed by the verse text (one or more lines, joined with a
// single space) up to the next heading or end of file.
func ParseVersesFile(path string) ([]Verse, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var verses []Verse
	var citation string
	var textLines []string

	flush := func() {
		if citation != "" {
			verses = append(verses, Verse{
				Citation: citation,
				Text:     strings.TrimSpace(strings.Join(textLines, " ")),
			})
		}
		textLines = nil
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if after, ok := strings.CutPrefix(line, "## "); ok {
			flush()
			citation = strings.TrimSpace(after)
			continue
		}
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			textLines = append(textLines, trimmed)
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(verses) == 0 {
		return nil, fmt.Errorf("no verses found in %s (expected \"## Citation\" headings)", path)
	}
	return verses, nil
}

// queueState is the on-disk record of a verse queue's position.
type queueState struct {
	Index int `json:"index"`
}

// LoadQueueIndex reads the next queue index from statePath.
// Returns 0 if statePath doesn't exist yet.
func LoadQueueIndex(statePath string) (int, error) {
	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var s queueState
	if err := json.Unmarshal(data, &s); err != nil {
		return 0, err
	}
	return s.Index, nil
}

// SaveQueueIndex writes the next queue index to statePath.
func SaveQueueIndex(statePath string, index int) error {
	data, err := json.Marshal(queueState{Index: index})
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, data, 0o644)
}
