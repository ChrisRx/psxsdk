// Command gen writes the quest example's placeholder art: a flagstone floor
// texture, a stone wall texture, a grass texture, a log wall and a
// shingled roof for the house, a bed, a treasure chest, the sky's backdrop
// of clouds and hills, a sun, the text box's letters, and two sprite
// sheets, Hero's and the villager's, eight ways to face by four frames of
// walking. Hand-made art of the same names and sizes replaces the
// pictures.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

func main() {
	dir := os.Args[1]
	must(os.MkdirAll(dir, 0o755))

	writePNG(filepath.Join(dir, "floor.png"), floor())
	writePNG(filepath.Join(dir, "wall.png"), wall())
	writePNG(filepath.Join(dir, "grass.png"), grass())
	writePNG(filepath.Join(dir, "log.png"), logs())
	writePNG(filepath.Join(dir, "roof.png"), roof())
	writePNG(filepath.Join(dir, "sky.png"), sky())
	writePNG(filepath.Join(dir, "sun.png"), sun())
	writePNG(filepath.Join(dir, "bed.png"), bed())
	writePNG(filepath.Join(dir, "chest.png"), chest())
	writePNG(filepath.Join(dir, "font.png"), font())
	writePNG(filepath.Join(dir, "hero.png"), sheet(heroPalette))
	writePNG(filepath.Join(dir, "npc.png"), sheet(npcPalette))
}

// chest is a 64x64 texture in bands for a treasure chest: the lid's top,
// rows 0-15, planks across with two iron straps; the front, rows 16-39,
// planks with the straps and a lock in the middle; a side, rows 40-55;
// and the dark inside, rows 56-63.
func chest() image.Image {
	pal := color.Palette{
		color.NRGBA{140, 90, 40, 255},
		color.NRGBA{100, 60, 25, 255},
		color.NRGBA{170, 115, 55, 255},
		color.NRGBA{70, 70, 80, 255},
		color.NRGBA{110, 110, 125, 255},
		color.NRGBA{230, 190, 60, 255},
		color.NRGBA{150, 120, 30, 255},
		color.NRGBA{25, 15, 10, 255},
		color.NRGBA{45, 30, 20, 255},
	}
	img := image.NewPaletted(image.Rect(0, 0, 64, 64), pal)
	for y := range 64 {
		for x := range 64 {
			var c uint8
			strap := x >= 11 && x <= 15 || x >= 48 && x <= 52
			switch {
			case y >= 56:
				c = 7
				if hash(x, y, 30)%7 == 0 {
					c = 8
				}
			case y >= 40:
				c = 0
				if (y-40)%8 == 0 {
					c = 1
				} else if (y-40)%8 == 1 {
					c = 2
				}
			default:
				c = 0
				switch {
				case strap:
					c = 3
					if x == 11 || x == 48 || (y%16 == 2 || y%16 == 9) && x%3 == 0 {
						c = 4
					}
				case y%8 == 0:
					c = 1
				case y%8 == 1:
					c = 2
				case hash(x, y, 31)%9 == 0:
					c = 1
				}
				// The lock, on the front, under the lid's edge.
				if y >= 18 && y <= 29 && x >= 27 && x <= 36 {
					c = 5
					if y == 29 || x == 36 || y >= 23 && y <= 25 && x >= 31 && x <= 32 {
						c = 6
					}
				}
			}
			img.SetColorIndex(x, y, c)
		}
	}
	return img
}

// font is the text box's letters: the printable ASCII characters from the
// space on, in sixteen columns, each drawn from a 5x8 glyph doubled to
// 10x16 in a cell of 12x18, white on nothing, bold as Dragon Quest's
// letters were, and in the last cell, for character 127, the arrow that
// says there is more to read.
func font() image.Image {
	pal := color.Palette{color.NRGBA{0, 0, 0, 0}, color.NRGBA{255, 255, 255, 255}}
	img := image.NewPaletted(image.Rect(0, 0, 16*12, 6*18), pal)
	for c := 32; c <= 127; c++ {
		g, ok := glyphs[rune(c)]
		if !ok {
			g = glyphs['?']
		}
		x0, y0 := c%16*12, c/16*18-36
		for y, row := range g {
			for x, p := range row {
				if p != '#' {
					continue
				}
				for dy := range 2 {
					for dx := range 2 {
						img.SetColorIndex(x0+2*x+dx, y0+2*y+dy, 1)
					}
				}
			}
		}
	}
	return img
}

// glyphs are the letters, 5 across and up to 8 down, the capitals on the
// first seven rows and the descenders on the eighth.
var glyphs = map[rune][]string{
	' ':  {},
	'!':  {"..#..", "..#..", "..#..", "..#..", "..#..", ".....", "..#.."},
	'"':  {".#.#.", ".#.#."},
	'\'': {"..#..", "..#.."},
	'(':  {"...#.", "..#..", ".#...", ".#...", ".#...", "..#..", "...#."},
	')':  {".#...", "..#..", "...#.", "...#.", "...#.", "..#..", ".#..."},
	',':  {".....", ".....", ".....", ".....", ".....", "..#..", "..#..", ".#..."},
	'-':  {".....", ".....", ".....", "#####"},
	'.':  {".....", ".....", ".....", ".....", ".....", ".##..", ".##.."},
	':':  {".....", ".....", ".##..", ".##..", ".....", ".##..", ".##.."},
	';':  {".....", ".....", ".##..", ".##..", ".....", "..#..", "..#..", ".#..."},
	'?':  {".###.", "#...#", "....#", "..##.", "..#..", ".....", "..#.."},
	'0':  {".###.", "#...#", "#..##", "#.#.#", "##..#", "#...#", ".###."},
	'1':  {"..#..", ".##..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'2':  {".###.", "#...#", "....#", "...#.", "..#..", ".#...", "#####"},
	'3':  {"#####", "...#.", "..#..", "...#.", "....#", "#...#", ".###."},
	'4':  {"...#.", "..##.", ".#.#.", "#..#.", "#####", "...#.", "...#."},
	'5':  {"#####", "#....", "####.", "....#", "....#", "#...#", ".###."},
	'6':  {"..##.", ".#...", "#....", "####.", "#...#", "#...#", ".###."},
	'7':  {"#####", "....#", "...#.", "..#..", ".#...", ".#...", ".#..."},
	'8':  {".###.", "#...#", "#...#", ".###.", "#...#", "#...#", ".###."},
	'9':  {".###.", "#...#", "#...#", ".####", "....#", "...#.", ".##.."},
	'A':  {".###.", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'B':  {"####.", "#...#", "####.", "#...#", "#...#", "#...#", "####."},
	'C':  {".###.", "#...#", "#....", "#....", "#....", "#...#", ".###."},
	'D':  {"####.", "#...#", "#...#", "#...#", "#...#", "#...#", "####."},
	'E':  {"#####", "#....", "#....", "####.", "#....", "#....", "#####"},
	'F':  {"#####", "#....", "#....", "####.", "#....", "#....", "#...."},
	'G':  {".###.", "#...#", "#....", "#.###", "#...#", "#...#", ".####"},
	'H':  {"#...#", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'I':  {"#####", "..#..", "..#..", "..#..", "..#..", "..#..", "#####"},
	'J':  {"..###", "...#.", "...#.", "...#.", "...#.", "#..#.", ".##.."},
	'K':  {"#...#", "#..#.", "#.#..", "##...", "#.#..", "#..#.", "#...#"},
	'L':  {"#....", "#....", "#....", "#....", "#....", "#....", "#####"},
	'M':  {"#...#", "##.##", "#.#.#", "#.#.#", "#...#", "#...#", "#...#"},
	'N':  {"#...#", "##..#", "#.#.#", "#..##", "#...#", "#...#", "#...#"},
	'O':  {".###.", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'P':  {"####.", "#...#", "#...#", "####.", "#....", "#....", "#...."},
	'Q':  {".###.", "#...#", "#...#", "#...#", "#.#.#", "#..#.", ".##.#"},
	'R':  {"####.", "#...#", "#...#", "####.", "#.#..", "#..#.", "#...#"},
	'S':  {".###.", "#...#", "#....", ".###.", "....#", "#...#", ".###."},
	'T':  {"#####", "..#..", "..#..", "..#..", "..#..", "..#..", "..#.."},
	'U':  {"#...#", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'V':  {"#...#", "#...#", "#...#", "#...#", ".#.#.", ".#.#.", "..#.."},
	'W':  {"#...#", "#...#", "#...#", "#.#.#", "#.#.#", "##.##", "#...#"},
	'X':  {"#...#", "#...#", ".#.#.", "..#..", ".#.#.", "#...#", "#...#"},
	'Y':  {"#...#", "#...#", ".#.#.", "..#..", "..#..", "..#..", "..#.."},
	'Z':  {"#####", "....#", "...#.", "..#..", ".#...", "#....", "#####"},
	'a':  {".....", ".....", ".###.", "....#", ".####", "#...#", ".####"},
	'b':  {"#....", "#....", "####.", "#...#", "#...#", "#...#", "####."},
	'c':  {".....", ".....", ".###.", "#....", "#....", "#...#", ".###."},
	'd':  {"....#", "....#", ".####", "#...#", "#...#", "#...#", ".####"},
	'e':  {".....", ".....", ".###.", "#...#", "#####", "#....", ".###."},
	'f':  {"..##.", ".#..#", ".#...", "###..", ".#...", ".#...", ".#..."},
	'g':  {".....", ".....", ".####", "#...#", "#...#", ".####", "....#", ".###."},
	'h':  {"#....", "#....", "####.", "#...#", "#...#", "#...#", "#...#"},
	'i':  {"..#..", ".....", ".##..", "..#..", "..#..", "..#..", ".###."},
	'j':  {"...#.", ".....", "..##.", "...#.", "...#.", "...#.", "#..#.", ".##.."},
	'k':  {"#....", "#....", "#..#.", "#.#..", "##...", "#.#..", "#..#."},
	'l':  {".##..", "..#..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'm':  {".....", ".....", "##.#.", "#.#.#", "#.#.#", "#.#.#", "#...#"},
	'n':  {".....", ".....", "####.", "#...#", "#...#", "#...#", "#...#"},
	'o':  {".....", ".....", ".###.", "#...#", "#...#", "#...#", ".###."},
	'p':  {".....", ".....", "####.", "#...#", "#...#", "####.", "#....", "#...."},
	'q':  {".....", ".....", ".####", "#...#", "#...#", ".####", "....#", "....#"},
	'r':  {".....", ".....", "#.###", "##...", "#....", "#....", "#...."},
	's':  {".....", ".....", ".####", "#....", ".###.", "....#", "####."},
	't':  {".#...", ".#...", "###..", ".#...", ".#...", ".#..#", "..##."},
	'u':  {".....", ".....", "#...#", "#...#", "#...#", "#..##", ".##.#"},
	'v':  {".....", ".....", "#...#", "#...#", "#...#", ".#.#.", "..#.."},
	'w':  {".....", ".....", "#...#", "#...#", "#.#.#", "#.#.#", ".#.#."},
	'x':  {".....", ".....", "#...#", ".#.#.", "..#..", ".#.#.", "#...#"},
	'y':  {".....", ".....", "#...#", "#...#", "#...#", ".####", "....#", ".###."},
	'z':  {".....", ".....", "#####", "...#.", "..#..", ".#...", "#####"},
	127:  {".....", ".....", "#####", ".###.", "..#.."},
}

// bed is a 64x64 texture in three bands: a quilt of checks for the top
// of a bed, a pillow, and planks for its sides.
func bed() image.Image {
	pal := color.Palette{
		color.NRGBA{60, 80, 170, 255},
		color.NRGBA{200, 205, 235, 255},
		color.NRGBA{40, 50, 110, 255},
		color.NRGBA{240, 240, 245, 255},
		color.NRGBA{190, 190, 200, 255},
		color.NRGBA{120, 80, 40, 255},
		color.NRGBA{80, 50, 25, 255},
		color.NRGBA{150, 105, 55, 255},
	}
	img := image.NewPaletted(image.Rect(0, 0, 64, 64), pal)
	for y := range 64 {
		for x := range 64 {
			var c uint8
			switch {
			case y < 32:
				c = uint8((x/8 + y/8) % 2)
				if x%8 == 0 || y%8 == 0 {
					c = 2
				}
			case y < 48:
				c = 3
				if x < 3 || x > 60 || y < 34 || y > 45 {
					c = 4
				}
			default:
				c = 5
				switch {
				case y%8 == 0 || (x+y/8*20)%32 == 0:
					c = 6
				case y%8 == 1:
					c = 7
				}
			}
			img.SetColorIndex(x, y, c)
		}
	}
	return img
}

// logs is a 64x64 log wall: four logs, each rounded, lit along its top
// and shaded under, with a dark gap between, grained along its length,
// wrapping both ways.
func logs() image.Image {
	pal, shade := ramp(30, 170, [3]float64{1, 0.72, 0.45})
	img := image.NewPaletted(image.Rect(0, 0, 64, 64), pal)
	for y := range 64 {
		for x := range 64 {
			iy := y % 16
			var b int
			switch {
			case iy < 2:
				b = 35 + hash(x, y, 14)%12
			default:
				// Brightest a little above the log's middle, falling
				// off to its edges, with grain along x.
				m := iy - 8
				b = 130 - m*m*2 + hash(x/6, y, 15)%24 - 12 + hash(x, y, 16)%10 - 5
			}
			img.SetColorIndex(x, y, shade(b))
		}
	}
	return img
}

// roof is 64x64 of shingles: rows of 16, each shifted half a shingle,
// a dark line under each row and between shingles, in a dark red.
func roof() image.Image {
	pal, shade := ramp(40, 150, [3]float64{1, 0.5, 0.4})
	img := image.NewPaletted(image.Rect(0, 0, 64, 64), pal)
	for y := range 64 {
		for x := range 64 {
			row := y / 16
			sx := (x + row%2*8) % 16
			iy := y % 16
			var b int
			switch {
			case iy > 13 || sx < 1:
				b = 45 + hash(x, y, 17)%10
			default:
				b = 105 + hash((x+row%2*8)/16, row, 18)%30 - 15 + hash(x, y, 19)%16 - 8 - iy
			}
			img.SetColorIndex(x, y, shade(b))
		}
	}
	return img
}

// sky is the backdrop, 256x64 that wraps across: clouds in the top half,
// white above and gray below, on nothing, and in the bottom half two
// lines of hills, the farther paler, solid to the bottom.
func sky() image.Image {
	pal := color.Palette{
		color.NRGBA{0, 0, 0, 0},
		color.NRGBA{255, 255, 255, 255},
		color.NRGBA{190, 200, 220, 255},
		color.NRGBA{90, 140, 150, 255},
		color.NRGBA{50, 110, 80, 255},
	}
	img := image.NewPaletted(image.Rect(0, 0, 256, 64), pal)
	// Each cloud is a row of discs along a line, flatter below.
	for n := range 5 {
		cx, cy := 20+n*50+hash(n, 0, 11)%20, 8+hash(n, 1, 11)%10
		for k := range 5 {
			x, y := cx+k*7-14, cy+hash(n, k, 12)%3
			r := 4 + hash(n, k, 13)%4
			for dy := -r; dy <= r/2; dy++ {
				for dx := -r; dx <= r; dx++ {
					if dx*dx+dy*dy*2 > r*r {
						continue
					}
					c := uint8(1)
					if dy > 0 {
						c = 2
					}
					img.SetColorIndex((x+dx+256)%256, y+dy, c)
				}
			}
		}
	}
	// The hills: heights from sines whose periods divide 256, so they
	// wrap, the far line above the near one.
	for x := range 256 {
		t := float64(x) * 2 * math.Pi / 256
		far := 40 - 5*math.Sin(3*t) - 3*math.Sin(7*t+1)
		near := 50 - 6*math.Sin(2*t+2) - 3*math.Sin(5*t)
		for y := 32; y < 64; y++ {
			switch {
			case float64(y) >= near:
				img.SetColorIndex(x, y, 4)
			case float64(y) >= far:
				img.SetColorIndex(x, y, 3)
			}
		}
	}
	return img
}

// sun is a 32x32 sun: a pale yellow disc with a brighter middle.
func sun() image.Image {
	pal := color.Palette{
		color.NRGBA{0, 0, 0, 0},
		color.NRGBA{255, 250, 200, 255},
		color.NRGBA{255, 225, 120, 255},
	}
	img := image.NewPaletted(image.Rect(0, 0, 32, 32), pal)
	for y := range 32 {
		for x := range 32 {
			d := (x-16)*(x-16) + (y-16)*(y-16)
			switch {
			case d <= 7*7:
				img.SetColorIndex(x, y, 1)
			case d <= 11*11:
				img.SetColorIndex(x, y, 2)
			}
		}
	}
	return img
}

// grass is 64x64 of grass: green speckled by the pixel, with short dark
// blades scattered over it, wrapping both ways.
func grass() image.Image {
	pal, shade := ramp(50, 170, [3]float64{0.55, 1, 0.4})
	img := image.NewPaletted(image.Rect(0, 0, 64, 64), pal)
	for y := range 64 {
		for x := range 64 {
			img.SetColorIndex(x, y, shade(110+hash(x, y, 7)%40-20+hash(x/4, y/4, 8)%20-10))
		}
	}
	for n := range 90 {
		x, y, h := hash(n, 0, 9)%64, hash(n, 1, 9)%64, 2+hash(n, 2, 9)%3
		for k := range h {
			img.SetColorIndex(x, (y+k)%64, shade(60+hash(n, k, 10)%20))
		}
	}
	return img
}

// hash is a small integer hash of a point, for noise that is the same
// every run.
func hash(x, y, seed int) int {
	h := uint32(x)*374761393 + uint32(y)*668265263 + uint32(seed)*1274126177
	h = (h ^ h>>13) * 1274126177
	h ^= h >> 16
	return int(h & 0xff)
}

// ramp makes a palette of 16 shades from dark to light in a tint, and a
// function that picks the nearest shade for a brightness.
func ramp(lo, hi int, tint [3]float64) (color.Palette, func(b int) uint8) {
	var p color.Palette
	for i := range 16 {
		v := float64(lo) + float64(hi-lo)*float64(i)/15
		p = append(p, color.NRGBA{uint8(v * tint[0]), uint8(v * tint[1]), uint8(v * tint[2]), 255})
	}
	return p, func(b int) uint8 {
		i := (b - lo) * 15 / (hi - lo)
		return uint8(min(max(i, 0), 15))
	}
}

// floor is a 64x64 flagstone floor: four stones of 32 with mortar between,
// each its own shade, speckled, darker at its edges, wrapping at the
// texture's edges so that tiles join.
func floor() image.Image {
	pal, shade := ramp(50, 190, [3]float64{1, 0.94, 0.84})
	img := image.NewPaletted(image.Rect(0, 0, 64, 64), pal)
	for y := range 64 {
		for x := range 64 {
			sx, sy := x/32, y/32
			ix, iy := x%32, y%32
			var b int
			switch {
			case ix < 2 || iy < 2:
				b = 60 + hash(x, y, 1)%20
			default:
				b = 150 + hash(sx, sy, 2)%40 - 20 + hash(x, y, 3)%24 - 12
				if ix < 4 || iy < 4 || ix > 29 || iy > 29 {
					b -= 18
				}
			}
			img.SetColorIndex(x, y, shade(b))
		}
	}
	return img
}

// wall is a 64x64 wall of stone blocks in rows of 16, each row staggered
// by half a block, each block its own shade with a lit top and a shaded
// bottom, wrapping both ways.
func wall() image.Image {
	pal, shade := ramp(40, 180, [3]float64{0.88, 0.9, 1})
	img := image.NewPaletted(image.Rect(0, 0, 64, 64), pal)
	for y := range 64 {
		for x := range 64 {
			row := y / 16
			bx := (x + row%2*16) % 64
			ix, iy := bx%32, y%16
			var b int
			switch {
			case ix < 2 || iy < 2:
				b = 45 + hash(x, y, 4)%16
			default:
				b = 125 + hash(bx/32, row, 5)%44 - 22 + hash(x, y, 6)%20 - 10
				if iy < 4 || ix < 4 {
					b += 16
				}
				if iy > 13 || ix > 29 {
					b -= 16
				}
			}
			img.SetColorIndex(x, y, shade(b))
		}
	}
	return img
}

// The hero's colors, by index: 0 is see-through.
const (
	clear = iota
	outline
	skin
	hair
	tunic
	tunicLight
	legs
	white
)

var heroPalette = color.Palette{
	color.NRGBA{0, 0, 0, 0},
	color.NRGBA{16, 16, 24, 255},
	color.NRGBA{240, 200, 160, 255},
	color.NRGBA{120, 70, 30, 255},
	color.NRGBA{40, 90, 220, 255},
	color.NRGBA{90, 140, 255, 255},
	color.NRGBA{60, 50, 80, 255},
	color.NRGBA{255, 255, 255, 255},
}

// npcPalette colors a figure the same way in a green tunic with gray
// hair.
var npcPalette = color.Palette{
	color.NRGBA{0, 0, 0, 0},
	color.NRGBA{16, 16, 24, 255},
	color.NRGBA{240, 200, 160, 255},
	color.NRGBA{150, 150, 150, 255},
	color.NRGBA{60, 140, 70, 255},
	color.NRGBA{110, 190, 110, 255},
	color.NRGBA{70, 60, 50, 255},
	color.NRGBA{255, 255, 255, 255},
}

// sheet is a figure's sheet: a column of frames for each of the eight
// ways to face, from up on the screen clockwise in eighths of a turn, and
// a row for each frame of walking: standing, a step with the left foot
// forward, standing, a step with the right. Each frame is 32x48 with the
// feet on row 38. The figure is seen from behind when it faces up the
// screen, from the front when it faces down, and from the side between.
func sheet(pal color.Palette) image.Image {
	img := image.NewPaletted(image.Rect(0, 0, 8*32, 4*48), pal)
	for k := range 8 {
		for f := range 4 {
			frame(img, 32*k, 48*f, k, f)
		}
	}
	return img
}

// frame draws one frame at x0, y0: facing k, walking frame f.
func frame(img *image.Paletted, x0, y0, k, f int) {
	set := func(x, y int, c uint8) { img.SetColorIndex(x0+x, y0+y, c) }
	at := func(x, y int) uint8 { return img.ColorIndexAt(x0+x, y0+y) }
	box := func(x1, y1, x2, y2 int, c uint8) {
		for y := y1; y <= y2; y++ {
			for x := x1; x <= x2; x++ {
				set(x, y, c)
			}
		}
	}
	// outlined draws a box with a dark edge a pixel round it.
	outlined := func(x1, y1, x2, y2 int, c uint8) {
		box(x1-1, y1-1, x2+1, y2+1, outline)
		box(x1, y1, x2, y2, c)
	}
	disc := func(cx, cy, r int, c uint8) {
		for y := cy - r; y <= cy+r; y++ {
			for x := cx - r; x <= cx+r; x++ {
				if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r {
					set(x, y, c)
				}
			}
		}
	}

	// The step frames lift the body a pixel and put one foot forward,
	// drawn a pixel longer, and the other back, drawn shorter.
	bob, left, right := 0, 37, 37
	switch f {
	case 1:
		bob, left, right = 1, 38, 35
	case 3:
		bob, left, right = 1, 35, 38
	}
	outlined(12, 32, 14, left, legs)
	outlined(17, 32, 19, right, legs)

	// The body: narrower from the side, with the lighter front of the
	// tunic towards the way it faces.
	side := k == 2 || k == 6
	toward := k >= 3 && k <= 5
	bx1, bx2 := 11, 20
	if side {
		bx1, bx2 = 12, 19
	}
	outlined(bx1, 22-bob, bx2, 31-bob, tunic)
	switch {
	case toward:
		box(14, 23-bob, 17, 30-bob, tunicLight)
	case k == 2:
		box(17, 23-bob, 19, 30-bob, tunicLight)
	case k == 6:
		box(12, 23-bob, 14, 30-bob, tunicLight)
	}

	// The head: a skin disc under hair that covers all of it from behind,
	// the top and the far side from the side, and the top from the front;
	// eyes where the face shows.
	cy := 17 - bob
	disc(16, cy, 6, outline)
	disc(16, cy, 5, skin)
	for y := cy - 6; y <= cy+6; y++ {
		for x := 10; x <= 22; x++ {
			if at(x, y) != skin {
				continue
			}
			var covered bool
			switch {
			case toward:
				covered = y <= cy-3
			case k == 2:
				covered = y <= cy-2 || x <= 14
			case k == 6:
				covered = y <= cy-2 || x >= 18
			default:
				covered = y <= cy+3
			}
			if covered {
				set(x, y, hair)
			}
		}
	}
	switch k {
	case 3:
		set(15, cy+1, white)
		set(19, cy+1, white)
	case 4:
		set(14, cy+1, white)
		set(18, cy+1, white)
	case 5:
		set(13, cy+1, white)
		set(17, cy+1, white)
	case 2:
		set(19, cy+1, white)
	case 6:
		set(13, cy+1, white)
	}
}

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	must(err)
	must(png.Encode(f, img))
	must(f.Close())
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
