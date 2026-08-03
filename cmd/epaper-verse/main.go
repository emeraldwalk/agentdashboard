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

func main() {
	addr := flag.String("addr", "", "ePaper device address (e.g. http://192.168.1.50)")
	text := flag.String("text", "", "Verse text")
	citation := flag.String("citation", "", "Verse citation (e.g. John 3:16)")
	flag.Parse()

	if *addr == "" || *text == "" || *citation == "" {
		fmt.Fprintln(os.Stderr, "usage: epaper-verse -addr http://<device> -text \"...\" -citation \"...\"")
		os.Exit(1)
	}

	img := epaper.Renderer{}.RenderVerse(*text, *citation)
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

	fmt.Println("sent")
}
