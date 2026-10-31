// Time three ways: a root counter counting the screen's lines, a clock kept
// by counting frames, and how much of each frame the program's work takes,
// with sprites to add until it takes too much.
//
// examples/timers on the exp/ps API: the root counters are ps.Counter
// values, the field count and the lines since the last blank are
// functions, and the sprites are a ps.Sprite drawn once per ball.
//
// # Controls
//
// Up and down add and take away sprites, ten at a time while held.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/exp/ps"
)

const maxBalls = 400

type ball struct {
	x, y, dx, dy int
}

var (
	ballTIM = disc.Register("ASSETS/BALL.TIM", "../../../examples/timers/assets/BALL.TIM")

	screen = ps.Screen{
		Background: ps.Color{R: 20, G: 20, B: 40},
		Depth:      1,
		Packets:    32 << 10,
	}
	pad ps.Controller

	sprite ps.Sprite
	balls  [maxBalls]ball
	// count is how many balls there are, and lines how many lines the last
	// frame's work took, for a test to read.
	count = 20
	lines int

	// wraps counts the times the line counter passed passing, as the
	// tutorial's first example did.
	wraps   int
	counter int

	frameLines = 263
	rate       = 60

	loadBar, frameBar ps.Box

	text ps.Text
)

// passing is where the tutorial counted the line counter as passing.
const passing = 65250

func main() {
	screen.Open()
	text = screen.Font(16, 16, 288, 80, 300)
	if screen.Video() == ps.PAL {
		frameLines, rate = 313, 50
	}

	if f := disc.LoadAll(300); f == nil {
		sprite = ps.LoadTIM(ballTIM.Data()).Sprite()
	}
	ps.Seed(7)
	for i := range balls {
		b := &balls[i]
		b.x = ps.Rand() % 300
		b.y = 116 + ps.Rand()%100
		b.dx = ps.Rand()%5 - 2
		b.dy = ps.Rand()%5 - 2
		if b.dx == 0 {
			b.dx = 1
		}
	}

	ps.Lines.Start()
	ps.Lines.Reset()
	frameBar = ps.Box{
		X:     16,
		Y:     92,
		W:     288,
		H:     6,
		Color: ps.Color{R: 60, G: 60, B: 90},
	}
	loadBar = ps.Box{
		X: 16,
		Y: 92,
		H: 6,
	}

	for frame := range screen.Frames() {
		pad.Update()
		if n := pad.Duration(ps.Up); n == 1 || n > 15 {
			count = min(count+10, maxBalls)
		}
		if n := pad.Duration(ps.Down); n == 1 || n > 15 {
			count = max(count-10, 0)
		}

		// The line counter, round and round.
		counter = ps.Lines.Read()
		if counter > passing {
			wraps++
			ps.Lines.Reset()
		}

		// The bar shows the last frame's work against the whole frame,
		// red once it is more than the frame holds. It is drawn first so
		// that it is drawn last, over the sprites: at one depth, the last
		// thing drawn is drawn first.
		loadBar.W = uint16(min(288, 288*lines/frameLines))
		loadBar.Color = ps.Color{R: 80, G: 220, B: 120}
		if lines > frameLines {
			loadBar.Color = ps.Color{R: 240, G: 60, B: 40}
		}
		loadBar.Draw(frame, 0)
		frameBar.Draw(frame, 0)

		for i := range count {
			b := &balls[i]
			b.x += b.dx
			b.y += b.dy
			if b.x < 0 || b.x > 304 {
				b.dx = -b.dx
			}
			if b.y < 116 || b.y > 224 {
				b.dy = -b.dy
			}
			sprite.MoveTo(b.x, b.y)
			sprite.Draw(frame, 0)
		}

		seconds := ps.FieldCount() / rate
		text.Print("~c990LINE COUNTER~c999 ").Int(counter)
		text.Print("\nPASSED ").Int(passing).Print(" ").Int(wraps).Print(" TIMES\n")
		text.Print("\n~c990CLOCK~c999 ").Int(seconds / 60).Print(":")
		if seconds%60 < 10 {
			text.Print("0")
		}
		text.Int(seconds % 60)
		text.Print("\n\n~c990FRAME~c999 ").Int(count).Print(" SPRITES, ")
		text.Int(lines).Print(" OF ").Int(frameLines).Print(" LINES\n")

		// The lines since the last vertical blank: this frame's work,
		// the drawing included, done.
		lines = ps.LinesSinceBlank()
	}
}
