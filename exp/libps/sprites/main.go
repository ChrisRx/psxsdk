// Sprites: the second program of the Go library experiment, a 2D scene
// that needs nothing off the disc. Eight sprites cut from a sheet carried
// in the executable (sheet.go) bounce about, one turning and one
// breathing, a bar at the bottom follows the D-pad, and the frame number
// is printed. Built both ways, against the resident library and against
// exp/libps, it has to draw the same frames.
package main

import "go.chrisrx.dev/psx/yaroze"

var screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 40, G: 40, B: 60}}

// A mover is a sprite on its way across the screen.
type mover struct {
	sprite yaroze.GsSPRITE
	dx, dy int16
}

var (
	movers  [8]mover
	bar     = yaroze.GsBOXF{X: 20, Y: 220, W: 100, H: 8, R: 220, G: 200, B: 60}
	text    yaroze.Buffer
	textBuf [32]byte
	// frameNumber is read by the tests, so it is written where the
	// compiler cannot fold it away.
	frameNumber int32
)

//go:noinline
func countFrame() { frameNumber++ }

func main() {
	screen.Open()
	screen.Font(16, 16, 256, 16, 32)
	text.Init(textBuf[:])
	image := yaroze.TIM(sheetWords[:]).Load()
	yaroze.Srand(7)
	for i := range movers {
		m := &movers[i]
		m.sprite = image.Sprite()
		m.sprite.U, m.sprite.V = uint8(i%4)*16, uint8(i/4)*16
		m.sprite.W, m.sprite.H = 16, 16
		m.sprite.Mx, m.sprite.My = 8, 8
		m.sprite.X = int16(20 + yaroze.Rand()%280)
		m.sprite.Y = int16(40 + yaroze.Rand()%160)
		m.dx = int16(1 + yaroze.Rand()%3)
		m.dy = int16(1 + yaroze.Rand()%2)
		if yaroze.Rand()%2 == 0 {
			m.dx = -m.dx
		}
	}
	for ot := range screen.Frames() {
		pad := yaroze.ReadController(0)
		if pad.Buttons&yaroze.PADLright != 0 && bar.W < 280 {
			bar.W += 2
		}
		if pad.Buttons&yaroze.PADLleft != 0 && bar.W > 10 {
			bar.W -= 2
		}
		for i := range movers {
			m := &movers[i]
			m.sprite.X += m.dx
			m.sprite.Y += m.dy
			if m.sprite.X < 8 || m.sprite.X > 312 {
				m.dx = -m.dx
			}
			if m.sprite.Y < 40 || m.sprite.Y > 210 {
				m.dy = -m.dy
			}
			m.sprite.Sort(ot, uint16(i+1))
		}
		movers[0].sprite.Rotate += 2 << 12
		movers[1].sprite.Scalex = 4096 + int16((frameNumber%60)*48)
		movers[1].sprite.Scaley = movers[1].sprite.Scalex
		bar.Sort(ot, 10)
		text.Reset()
		text.WriteString("FRAME ").WriteInt(frameNumber)
		text.Print()
		countFrame()
	}
}
