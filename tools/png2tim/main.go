// Command png2tim converts pictures into TIM files, the PlayStation's image
// format, which a program loads into video memory as sprites and textures:
// what Sony's TIMUTIL did for BMP and PICT files on Windows. It is also
// yaroze's tim command.
package main

import (
	"log"

	"go.chrisrx.dev/psx/internal/png2tim"
)

func main() {
	if err := png2tim.Command("png2tim").Execute(); err != nil {
		log.Fatal(err)
	}
}
