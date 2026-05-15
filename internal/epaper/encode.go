package epaper

import (
	"bytes"
	"image"
	"image/png"
)

// EncodePNG encodes img to a PNG byte slice.
func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
