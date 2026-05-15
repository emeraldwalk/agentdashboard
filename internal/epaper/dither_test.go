package epaper

import (
	"image"
	"image/color"
	"testing"
)

func makeRGBA(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestDither_AllWhite(t *testing.T) {
	src := makeRGBA(10, 10, color.RGBA{255, 255, 255, 255})
	dst := Dither(src)
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			if dst.GrayAt(x, y).Y != 255 {
				t.Fatalf("expected white at (%d,%d), got %d", x, y, dst.GrayAt(x, y).Y)
			}
		}
	}
}

func TestDither_AllBlack(t *testing.T) {
	src := makeRGBA(10, 10, color.RGBA{0, 0, 0, 255})
	dst := Dither(src)
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			if dst.GrayAt(x, y).Y != 0 {
				t.Fatalf("expected black at (%d,%d), got %d", x, y, dst.GrayAt(x, y).Y)
			}
		}
	}
}

func TestDither_Checkerboard(t *testing.T) {
	const size = 20
	src := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if (x+y)%2 == 0 {
				src.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			} else {
				src.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			}
		}
	}
	dst := Dither(src)

	var black, white int
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if dst.GrayAt(x, y).Y == 0 {
				black++
			} else {
				white++
			}
		}
	}
	total := size * size
	// Allow 20% tolerance around 50/50
	low := total * 30 / 100
	high := total * 70 / 100
	if black < low || black > high {
		t.Errorf("checkerboard: expected roughly half black, got black=%d white=%d total=%d", black, white, total)
	}
}
