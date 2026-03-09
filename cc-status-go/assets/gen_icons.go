//go:build ignore

package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const size = 64

type icon struct {
	name string
	r, g, b uint8
}

func main() {
	icons := []icon{
		{"idle.png", 180, 180, 180},
		{"active.png", 107, 143, 173},
		{"waiting.png", 232, 156, 77},
		{"done.png", 122, 176, 110},
	}
	for _, ic := range icons {
		img := genCircle(ic.r, ic.g, ic.b)
		f, err := os.Create(ic.name)
		if err != nil {
			panic(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			panic(err)
		}
		f.Close()
	}
}

func genCircle(r, g, b uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(size)/2, float64(size)/2
	radius := float64(size)/2 - 2

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			dist := math.Sqrt(dx*dx + dy*dy)

			if dist <= radius-0.5 {
				// Fully inside
				img.SetNRGBA(x, y, color.NRGBA{r, g, b, 255})
			} else if dist <= radius+0.5 {
				// Anti-aliased edge (1px band)
				alpha := uint8(255 * (radius + 0.5 - dist))
				img.SetNRGBA(x, y, color.NRGBA{r, g, b, alpha})
			}
			// else: transparent (zero value)
		}
	}
	return img
}
