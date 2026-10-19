// Command gen draws the card example's save icon: 16 by 16 pixels in
// sixteen colours, which png2tim turns into a 4-bit TIM whose pixels and
// palette are the icon's, as the card's header wants them.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
)

func main() {
	pal := color.Palette{
		color.NRGBA{0, 0, 0, 0},
		color.NRGBA{20, 20, 40, 255},
		color.NRGBA{255, 200, 40, 255},
		color.NRGBA{255, 255, 255, 255},
	}
	for len(pal) < 16 {
		pal = append(pal, color.NRGBA{0, 0, 0, 255})
	}
	img := image.NewPaletted(image.Rect(0, 0, 16, 16), pal)
	for y := range 16 {
		for x := range 16 {
			switch {
			case x == 0 || y == 0 || x == 15 || y == 15:
				img.SetColorIndex(x, y, 1)
			case (x-8)*(x-8)+(y-8)*(y-8) < 30:
				img.SetColorIndex(x, y, 2) // a coin
			default:
				img.SetColorIndex(x, y, 1)
			}
		}
	}
	img.SetColorIndex(6, 6, 3)
	img.SetColorIndex(7, 5, 3)
	f, err := os.Create(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	f.Close()
}
