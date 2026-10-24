// Command gen writes jumpcube's pictures, the files an artist would make
// in a paint program: the ground's tiles, the hills and a cloud far off,
// and the cube. Each is an indexed PNG of a few flat colors, color 0
// see-through, which png2tim makes a 4-bit TIM of, and which the
// playground's sprite editor can paint on. The level itself is
// scenes/level.json, made in the playground's scene editor.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
)

func main() {
	dir := os.Args[1]
	must(os.MkdirAll(dir, 0o755))
	write(filepath.Join(dir, "tiles.png"), tiles())
	write(filepath.Join(dir, "far.png"), far())
	write(filepath.Join(dir, "hero.png"), cube())
}

// The ground's cells, eight across, in the order the level's flags are
// given to them: grass, dirt, a block, a plank stood on from above,
// spikes, a coin and the goal's flag. The last is left empty for more.
var tilePalette = color.Palette{
	color.NRGBA{0, 0, 0, 0},         // see-through
	color.NRGBA{40, 36, 48, 255},    // outline
	color.NRGBA{108, 192, 76, 255},  // grass
	color.NRGBA{150, 98, 60, 255},   // dirt
	color.NRGBA{120, 76, 48, 255},   // dirt, darker
	color.NRGBA{170, 150, 140, 255}, // block
	color.NRGBA{200, 150, 90, 255},  // plank
	color.NRGBA{210, 214, 224, 255}, // spike
	color.NRGBA{250, 210, 60, 255},  // gold
	color.NRGBA{240, 70, 70, 255},   // flag
}

const (
	clear, outline, grass, dirt, dirtDark, block, plank, spike, gold, red = 0, 1, 2, 3, 4, 5, 6, 7, 8, 9
)

func tiles() *image.Paletted {
	im := image.NewPaletted(image.Rect(0, 0, 128, 16), tilePalette)
	cell(im, 0, func(x, y int) uint8 { // grass over dirt
		if y < 4 {
			return grass
		}
		return dirt
	})
	cell(im, 1, func(x, y int) uint8 { // dirt, with a dot here and there
		if (x*7+y*13)%23 == 0 {
			return dirtDark
		}
		return dirt
	})
	cell(im, 2, func(x, y int) uint8 { // a block, outlined
		if x == 0 || y == 0 || x == 15 || y == 15 {
			return outline
		}
		return block
	})
	cell(im, 3, func(x, y int) uint8 { // a plank across the top
		switch {
		case y > 5:
			return clear
		case y == 0 || y == 5:
			return outline
		}
		return plank
	})
	cell(im, 4, func(x, y int) uint8 { // two spikes
		tip := x % 8
		if tip > 3 {
			tip = 7 - tip
		}
		if y >= 15-4*tip {
			return spike
		}
		return clear
	})
	cell(im, 5, func(x, y int) uint8 { // a coin
		if d := (x-8)*(x-8)*4 + (y-8)*(y-8)*3; d <= 150 {
			return gold
		}
		return clear
	})
	cell(im, 6, func(x, y int) uint8 { // a flag on a pole
		switch {
		case x == 3:
			return outline
		case x > 3 && x < 14 && y > 1 && y < 8:
			return red
		}
		return clear
	})
	return im
}

// cell paints cell k of a sheet eight cells across, a pixel at a time.
func cell(im *image.Paletted, k int, f func(x, y int) uint8) {
	x0, y0 := k%8*16, k/8*16
	for y := range 16 {
		for x := range 16 {
			im.SetColorIndex(x0+x, y0+y, f(x, y))
		}
	}
}

// The hills and the cloud far off: the cloud, two cells by two at cells
// 0, 1, 8 and 9, and the sides and the top of a hill and the ground in
// it, in flat colors.
var farPalette = color.Palette{
	color.NRGBA{0, 0, 0, 0},         // see-through
	color.NRGBA{248, 250, 255, 255}, // cloud
	color.NRGBA{120, 184, 160, 255}, // hill
}

func far() *image.Paletted {
	im := image.NewPaletted(image.Rect(0, 0, 128, 32), farPalette)
	// The cloud: three circles over the top left two cells by two.
	for y := range 29 {
		for x := range 32 {
			if within(x, y, 10, 20, 9) || within(x, y, 19, 13, 10) || within(x, y, 25, 21, 7) {
				im.SetColorIndex(x, y, 1)
			}
		}
	}
	// The hill's left side, its top, its right side and the ground in it.
	cell(im, 2, func(x, y int) uint8 {
		if y < 15-x {
			return clear
		}
		return 2
	})
	cell(im, 3, func(x, y int) uint8 { return 2 })
	cell(im, 4, func(x, y int) uint8 {
		if y < x {
			return clear
		}
		return 2
	})
	cell(im, 5, func(x, y int) uint8 { return 2 })
	return im
}

func within(x, y, cx, cy, r int) bool { return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r }

// The cube: one frame, sixteen pixels square, a square of fourteen in one
// color with a dark border a pixel wide round the outside, the same each
// way round, so that it looks the same after each quarter turn.
var cubePalette = color.Palette{
	color.NRGBA{0, 0, 0, 0},        // see-through
	color.NRGBA{40, 36, 48, 255},   // border
	color.NRGBA{240, 140, 40, 255}, // cube
}

func cube() *image.Paletted {
	im := image.NewPaletted(image.Rect(0, 0, 16, 16), cubePalette)
	for y := range 16 {
		for x := range 16 {
			switch min(x, y, 15-x, 15-y) { // how far in from the frame's edge
			case 0:
				im.SetColorIndex(x, y, 0)
			case 1:
				im.SetColorIndex(x, y, 1)
			default:
				im.SetColorIndex(x, y, 2)
			}
		}
	}
	return im
}

func write(path string, im image.Image) {
	f, err := os.Create(path)
	must(err)
	must(png.Encode(f, im))
	must(f.Close())
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
