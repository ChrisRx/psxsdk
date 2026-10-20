// Time three ways: a root counter counting the screen's lines, a clock kept
// by counting frames, and how much of each frame the program's work takes,
// with sprites to add until it takes too much.
//
// A port of the SCEE tutorial's chapter 5, its three examples in one
// program. The first started root counter 1, which counts the television's
// horizontal lines, and counted each time it passed 65,250. The second kept
// a clock in minutes and seconds by counting frames, fifty to a second on
// the European consoles it was written for. The third drew sprites, more or
// fewer with up and down, and printed how many lines had passed since the
// last vertical blank, which is how long the frame's work took.
//
// The root counters are the CPU's four timers: one counts the GPU's dot
// clock, one the horizontal lines, one the system clock divided by eight,
// and one the vertical blanks, which the library keeps for VSync. Each
// counts to 65,535 and round again, so a program measures a span by the
// difference of two readings. The frame count gives a clock without them:
// VSync(-1) returns the fields since the program started, sixty a second
// on an NTSC console and fifty on a PAL one. And VSync(1) returns the lines
// since the last vertical blank without waiting, which, read when a frame's
// work is done, is how much of the frame it took: a television draws 263
// lines a field on NTSC and 313 on PAL, and work that takes more misses the
// next vertical blank and halves the frame rate.
//
// Differences from the tutorial: the clock reads the console's own rate,
// rather than taking fifty frames to a second; the line count is drawn as a
// bar against the frame as well as printed; and the sprites move, a few
// hundred at most.
//
// # Controls
//
// Up and down add and take away sprites, ten at a time while held.
package main

//go:generate go run ./gen gen/ball.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 640,0 --clut 0,481 -o assets/BALL.TIM gen/ball.png
//go:generate go run go.chrisrx.dev/psx/tools/discgen assets

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

const maxBalls = 400

type ball struct {
	x, y, dx, dy int16
}

var (
	screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 20, G: 20, B: 40}, Depth: 1, Packets: 32 << 10}
	pad    yaroze.Pad

	sprite yaroze.GsSPRITE
	balls  [maxBalls]ball
	// count is how many balls there are, and lines how many lines the last
	// frame's work took, for a test to read.
	count int32 = 20
	lines int32

	// wraps counts the times the line counter passed passing, as the
	// tutorial's first example did.
	wraps   int32
	counter int32

	frameLines int32 = 263
	rate       int32 = 60

	loadBar, frameBar yaroze.GsBOXF

	text    yaroze.Buffer
	textBuf [512]byte
)

// passing is where the tutorial counted the line counter as passing.
const passing = 65250

func main() {
	screen.Open()
	screen.Font(16, 16, 288, 80, 300)
	text.Init(textBuf[:])
	if yaroze.GetVideoMode() == yaroze.MODE_PAL {
		frameLines, rate = 313, 50
	}

	if f := disc.LoadAll(300, files...); f == nil {
		sprite = yaroze.TIM(ballTIM.Data()).Load().Sprite()
	}
	yaroze.Srand(7)
	for i := range balls {
		b := &balls[i]
		b.x = int16(yaroze.Rand() % 300)
		b.y = int16(116 + yaroze.Rand()%100)
		b.dx = int16(yaroze.Rand()%5 - 2)
		b.dy = int16(yaroze.Rand()%5 - 2)
		if b.dx == 0 {
			b.dx = 1
		}
	}

	yaroze.StartRCnt(yaroze.RCntCNT1)
	yaroze.ResetRCnt(yaroze.RCntCNT1)
	frameBar = yaroze.GsBOXF{X: 16, Y: 92, W: 288, H: 6, R: 60, G: 60, B: 90}
	loadBar = yaroze.GsBOXF{X: 16, Y: 92, H: 6}

	for ot := range screen.Frames() {
		pad.Update()
		if n := pad.Duration(yaroze.PADLup); n == 1 || n > 15 {
			count = min(count+10, maxBalls)
		}
		if n := pad.Duration(yaroze.PADLdown); n == 1 || n > 15 {
			count = max(count-10, 0)
		}

		// The line counter, round and round.
		counter = yaroze.GetRCnt(yaroze.RCntCNT1)
		if counter > passing {
			wraps++
			yaroze.ResetRCnt(yaroze.RCntCNT1)
		}

		// The bar shows the last frame's work against the whole frame,
		// red once it is more than the frame holds. It is sorted first
		// so that it is drawn last, over the sprites: at one depth, the
		// last thing sorted is drawn first.
		loadBar.W = uint16(min(288, 288*lines/frameLines))
		loadBar.R, loadBar.G, loadBar.B = 80, 220, 120
		if lines > frameLines {
			loadBar.R, loadBar.G, loadBar.B = 240, 60, 40
		}
		loadBar.Sort(ot, 0)
		frameBar.Sort(ot, 0)

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
			sprite.X, sprite.Y = b.x, b.y
			sprite.Sort(ot, 0)
		}

		seconds := yaroze.VSync(-1) / rate
		text.Reset()
		text.WriteString("~c990LINE COUNTER~c999 ").WriteInt(counter)
		text.WriteString("\nPASSED ").WriteInt(passing).WriteString(" ").WriteInt(wraps).WriteString(" TIMES\n")
		text.WriteString("\n~c990CLOCK~c999 ").WriteInt(seconds / 60).WriteString(":")
		if seconds%60 < 10 {
			text.WriteString("0")
		}
		text.WriteInt(seconds % 60)
		text.WriteString("\n\n~c990FRAME~c999 ").WriteInt(count).WriteString(" SPRITES, ")
		text.WriteInt(lines).WriteString(" OF ").WriteInt(frameLines).WriteString(" LINES\n")
		text.Print()

		// The lines since the last vertical blank: this frame's work,
		// the sorting included, done.
		lines = yaroze.VSync(1)
	}
}
