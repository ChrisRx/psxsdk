// The check sample, in Go.
//
// A port of sdk/sample/check/main.c: bouncing balls over a cleared
// background, the count adjustable with the d-pad, with the sound data read
// off the disc. It exists to show the Go bindings carrying a real program
// rather than a hello world, and to put the CD path, the controller and a
// few hundred sprites through the same code the C sample uses.
//
// Three deliberate differences from the original:
//
//   - The sound data is read into the program's own memory, declared here,
//     rather than to addresses chosen by hand, and the build checks that
//     each file fits where it goes.
//   - The CD waits are bounded. The C sample spins in
//     "while ((cnt = CdReadSync(1, 0)) > 0)" with no limit, so running it
//     without the files present hangs before it has drawn anything.
//   - Sound is only started if the VAB actually loaded. The C sample calls
//     SsSeqSetVol and SsSeqPlay with whatever handle it has, including the
//     zero left behind when the transfer failed.
//
// Nothing allocates after startup. With no collector running, anything
// allocated in the frame loop would never be reclaimed.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	otLength = 1   // ordering table size, as a power of two
	maxObj   = 256 // how many balls can be on screen
	frameX   = 320 // display width
	frameY   = 240 // display height
	wallX    = frameX - 16
	wallY    = frameY - 16

	mvol = 127 // main volume
	svol = 127 // sequence volume

	// Frames to wait for one CD read before giving up. Short enough that
	// three files cannot add up to a wait long enough to look like a hang.
	readTimeout = 180

	// The sound files' sizes in bytes, from which their buffers are sized
	// in whole sectors, since that is what the drive delivers. The build
	// checks the files fit, so a change to the data shows up there.
	vhSize  = 60448
	vbSize  = 401360
	seqSize = 39616
)

// The sound data: the bank's header and waveforms, and the sequence, each
// in a buffer of whole sectors, 512 words each, enough for its file. Each
// is registered with mkpsxiso by its declaration, which also says where it
// is read into; see package disc.
var (
	vhData  [(vhSize + 2047) / 2048 * 512]uint32
	vbData  [(vbSize + 2047) / 2048 * 512]uint32
	seqData [(seqSize + 2047) / 2048 * 512]uint32

	vh  = disc.Register("DATA/SOUND/STD0.VH", "data/STD0.VH").Into(vhData[:])
	vb  = disc.Register("DATA/SOUND/STD0.VB", "data/STD0.VB").Into(vbData[:])
	seq = disc.Register("DATA/SOUND/SAMPLE1.SEQ", "data/SAMPLE1.SEQ").Into(seqData[:])

	dataFiles = [...]*disc.File{&vh, &vb, &seq}
)

// The frame: two ordering tables and two packet areas, one of each per
// display buffer, which the screen takes in halves. The packet area has to
// be large enough for every primitive in a frame.
var (
	screen     yaroze.Screen
	otTags     [2 << otLength]yaroze.GsOT_TAG
	packetArea [2 * maxObj * 24 / 4]uint32

	sprites [maxObj]yaroze.GsSPRITE
)

// A pos is a ball's position and velocity.
type pos struct {
	x, y   uint16
	dx, dy uint16
}

var positions [maxObj]pos

var (
	lineBuf [128]byte
	line    yaroze.Buffer

	loadNote [len(dataFiles)]string

	vab  int16 = -1
	song int16 = -1
)

// loadDataFiles reads the sound data. It reports whether every file
// arrived, so the caller can skip sound rather than feed the sequencer an
// empty buffer.
func loadDataFiles() bool {
	ok := true
	for i, f := range dataFiles {
		status("reading sound data\n")

		// From the sector mkpsxiso recorded, or by name in an executable
		// it did not place.
		if !f.Load() {
			loadNote[i] = "not found"
			ok = false
			continue
		}

		// Bounded, unlike the C original, so that a drive that never
		// finishes shows up as a report rather than a hang.
		if !disc.Wait(readTimeout) {
			loadNote[i] = "read timed out"
			ok = false
			continue
		}
		loadNote[i] = "ok"
	}
	return ok
}

func initSound() {
	vab = yaroze.SsVabTransfer(vh.Bytes(), vb.Bytes(), -1, 1)
	if vab < 0 {
		return
	}
	song = yaroze.SsSeqOpen(&seqData[0], vab)
}

func playSound() {
	if vab < 0 || song < 0 {
		return
	}
	yaroze.SsSetMVol(mvol, mvol)
	yaroze.SsSeqSetVol(song, svol, svol)
	yaroze.SsSeqPlay(song, yaroze.SSPLAY_PLAY, yaroze.SSPLAY_INFINITY)
}

func stopSound() {
	if vab < 0 || song < 0 {
		return
	}
	yaroze.SsSeqStop(song)
	yaroze.VSync(0)
	yaroze.VSync(0)
	yaroze.SsSeqClose(song)
	yaroze.SsVabClose(vab)
}

// initPrim loads the ball texture into video memory and sets up the sprites.
func initPrim() {
	rect := yaroze.RECT{X: 640, Y: 0, W: 16 / 4, H: 16}
	rect.Load(&demo.Ball16x16[0])
	tpage := yaroze.GetTPage(0, 0, 640, 0)

	// 32 palettes, one row of video memory each, so each ball can have its
	// own shade without a second texture.
	for i := range demo.BallColor {
		r := yaroze.RECT{X: 0, Y: int16(480 + i), W: 256, H: 1}
		r.Load(&demo.BallColor[i][0])
	}

	for i := range sprites {
		sp := &sprites[i]
		sp.Attribute = 0
		sp.X, sp.Y = 0, 0
		sp.W, sp.H = 16, 16
		sp.Tpage = tpage
		sp.U, sp.V = 0, 0
		sp.Cx, sp.Cy = 0, int16(480+(i%32))
		sp.R, sp.G, sp.B = 0x80, 0x80, 0x80
		sp.Mx, sp.My = 0, 0
		sp.Scalex, sp.Scaley = yaroze.ONE, yaroze.ONE
		sp.Rotate = 0
	}
}

// initPoints gives every ball a different starting place and speed.
func initPoints() {
	for i := range positions {
		positions[i].x = uint16(i * 7 % wallX)
		positions[i].y = uint16(i * 13 % wallY)
		positions[i].dx = uint16(i%7 + 1)
		positions[i].dy = uint16(i%5 + 1)
	}
}

// padRead adjusts the ball count, returning zero to stop. It is kept out of
// line because the tests run the program until it is first called, which
// is when the main loop has begun.
//
//go:noinline
func padRead(n int) int {
	pad := yaroze.PadRead()

	if pad&yaroze.PADLup != 0 {
		n += 4
	}
	if pad&yaroze.PADLdown != 0 {
		n -= 4
	}
	if pad&yaroze.PADL1 != 0 {
		// Pause: hold here until the button comes back up.
		for yaroze.PadRead()&yaroze.PADL1 != 0 {
			yaroze.VSync(0)
		}
	}
	if pad&yaroze.PADselect != 0 {
		return 0
	}

	return min(max(n, 1), maxObj-1)
}

// status shows a caption for about half a second, so that a slow step has
// something to show for itself rather than leaving a blank screen.
//
// It draws complete frames. That matters: FntFlush alone writes into the
// buffer that is not currently displayed, so text without the surrounding
// clear, draw and swap never appears.
func status(msg string) {
	for range 24 {
		screen.Begin()
		screen.End(60, 120, 120)
		line.Reset()
		line.WriteString(msg)
		line.Print()
		yaroze.FntFlush(-1)
	}
}

func main() {
	line.Init(lineBuf[:])

	// Graphics come up before anything touches the drive, unlike the C
	// original, which loads its data first and so has nothing on screen if
	// that takes a while or does not finish. Bringing the display up first
	// costs nothing and turns a black screen into a report.
	yaroze.SetVideoMode(yaroze.MODE_NTSC)
	yaroze.GsInitGraph(frameX, frameY, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, frameY)
	screen.Init(otTags[:], packetArea[:])

	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(16, 16, 256, 200, 0, 512)

	status("loading\n")
	yaroze.PadInit()

	haveSound := loadDataFiles()

	initPrim()
	initPoints()
	if haveSound {
		initSound()
		playSound()
	}

	nobj := 16
	for {
		nobj = padRead(nobj)
		if nobj <= 0 {
			break
		}

		ot := screen.Begin()
		for i := 0; i < nobj; i++ {
			p := &positions[i]
			p.x += p.dx
			p.y += p.dy

			// Bounce by folding the position back on itself, which is
			// what the original does with a modulo of twice the wall.
			x := int(p.x) % (wallX * 2)
			if x >= wallX {
				x = wallX*2 - x
			}
			y := int(p.y) % (wallY * 2)
			if y >= wallY {
				y = wallY*2 - y
			}

			sprites[i].X = int16(x)
			sprites[i].Y = int16(y)
			sprites[i].SortFast(ot, 0)
		}
		cnt := screen.End(60, 120, 120)

		line.Reset()
		line.WriteString("sprites = ").WriteInt(nobj).
			WriteString("\ntotal time = ").WriteInt(int(cnt)).
			WriteString("\n\nUP    : INCREASE\nDOWN  : DECREASE\n")
		if !haveSound {
			line.WriteString("\nsound data:\n")
			for i := range dataFiles {
				line.WriteString("  ").WriteString(loadNote[i]).WriteString("\n")
			}
		}
		line.WriteString("\nwritten in Go!!!\n")
		line.Print()
		yaroze.FntFlush(-1)
	}

	stopSound()
}
