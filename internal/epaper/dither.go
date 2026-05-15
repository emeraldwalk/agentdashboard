package epaper

import (
	"image"
	"image/color"
)

// Dither converts an RGBA image to a 1-bit grayscale image
// using Floyd-Steinberg dithering.
func Dither(src *image.RGBA) *image.Gray {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Work in float32 luminance to accumulate error
	lum := make([]float32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			// Rec. 601 luminance, values are 0–65535
			l := 0.299*float32(r) + 0.587*float32(g) + 0.114*float32(b)
			lum[y*w+x] = l / 257 // scale to 0–255
		}
	}

	dst := image.NewGray(bounds)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			old := lum[y*w+x]
			var newVal float32
			if old < 128 {
				newVal = 0
			} else {
				newVal = 255
			}
			dst.SetGray(bounds.Min.X+x, bounds.Min.Y+y, color.Gray{Y: uint8(newVal)})

			err := old - newVal
			// Floyd-Steinberg diffusion
			if x+1 < w {
				lum[y*w+(x+1)] += err * 7 / 16
			}
			if y+1 < h {
				if x-1 >= 0 {
					lum[(y+1)*w+(x-1)] += err * 3 / 16
				}
				lum[(y+1)*w+x] += err * 5 / 16
				if x+1 < w {
					lum[(y+1)*w+(x+1)] += err * 1 / 16
				}
			}
		}
	}
	return dst
}
