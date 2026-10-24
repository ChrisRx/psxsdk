// Command gen draws the timers example's sprite, a 16 by 16 ball in sixteen
// colors, which png2tim turns into a 4-bit TIM.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
)

func main() {
	pal := color.Palette{color.NRGBA{}}
	for i := range 15 {
		v := uint8(80 + i*11)
		pal = append(pal, color.NRGBA{v, v / 2, 255 - v/2, 255})
	}
	img := image.NewPaletted(image.Rect(0, 0, 16, 16), pal)
	for y := range 16 {
		for x := range 16 {
			dx, dy := float64(x)-7.5, float64(y)-7.5
			d := dx*dx + dy*dy
			if d < 64 {
				// Lighter towards the top left.
				l := 15 - int((dx+dy+11)/22*14)
				img.SetColorIndex(x, y, uint8(max(1, min(15, l))))
			}
		}
	}
	f, err := os.Create(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	f.Close()
}
