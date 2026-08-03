// Command epaper-verse renders a bible verse and citation to the ePaper
// display as a one-shot PNG push.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/emeraldwalk/agentdashboard/internal/epaper"
)

const usage = "usage: epaper-verse -addr http://<device> (-text \"...\" -citation \"...\" | -file verses.md)"

func main() {
	addr := flag.String("addr", "", "ePaper device address (e.g. http://192.168.1.50)")
	text := flag.String("text", "", "Verse text (ignored if -file is set)")
	citation := flag.String("citation", "", "Verse citation, e.g. John 3:16 (ignored if -file is set)")
	file := flag.String("file", "", "Markdown file of verses (\"## Citation\" headings, verse text below); each run sends the next queued verse, looping back to the start at the end")
	state := flag.String("state", "", "Queue state file path (default: <file>.state)")
	flag.Parse()

	if *addr == "" {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	var verseText, verseCitation string
	var statePath string
	var nextIdx int

	if *file != "" {
		verses, err := epaper.ParseVersesFile(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read verses: %v\n", err)
			os.Exit(1)
		}

		statePath = *state
		if statePath == "" {
			statePath = *file + ".state"
		}

		idx, err := epaper.LoadQueueIndex(statePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read state: %v\n", err)
			os.Exit(1)
		}
		idx %= len(verses)

		v := verses[idx]
		verseText, verseCitation = v.Text, v.Citation
		nextIdx = (idx + 1) % len(verses)
	} else {
		if *text == "" || *citation == "" {
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(1)
		}
		verseText, verseCitation = *text, *citation
	}

	img := epaper.Renderer{}.RenderVerse(verseText, verseCitation)
	data, err := epaper.EncodePNG(epaper.Dither(img))
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, *addr+"/image", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "build request error: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "image/png")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "POST error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "device returned %d\n", resp.StatusCode)
		os.Exit(1)
	}

	// Only advance the queue once the send has actually succeeded, so a
	// failed push retries the same verse next run instead of skipping it.
	if statePath != "" {
		if err := epaper.SaveQueueIndex(statePath, nextIdx); err != nil {
			fmt.Fprintf(os.Stderr, "save state: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("sent:", verseCitation)
}
