// The check sample, in Go.
//
// A port of sdk/sample/check/main.c: bouncing balls over a cleared
// background, the count adjustable with the d-pad, with the sound data read
// off the disc. It exists to show the Go bindings carrying a real program
// rather than a hello world, and to put the CD path, the controller and a
// few hundred sprites through the same code the C sample uses.
//
// Two deliberate differences from the original:
//
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
	"unsafe"

	"go.chrisrx.dev/psx/disc"
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

	// Where the sound data is loaded. These addresses are below the
	// program and above the resident library, which is where the SDK puts
	// scratch data.
	vhAddr  = 0x80090000
	vbAddr  = 0x800a0000
	seqAddr = 0x80110000

	// Frames to wait for one CD read before giving up. Short enough that
	// three files cannot add up to a wait long enough to look like a hang.
	readTimeout = 180
)

// Ordering tables and GPU packet space, one set per display buffer.
var (
	worldOT [2]yaroze.GsOT
	otTags  [2][1 << otLength]yaroze.GsOT_TAG

	// The library builds GPU commands in here. It has to be large enough
	// for every primitive in a frame.
	gpuPacketArea [2][maxObj * 24]yaroze.PACKET

	sprites [maxObj]yaroze.GsSPRITE
)

// A pos is a ball's position and velocity.
type pos struct {
	x, y   uint16
	dx, dy uint16
}

var positions [maxObj]pos

// Controller state. GetPadBuf hands back pointers into the library's own
// receive buffers, which are refreshed every frame.
var (
	padBuf0 *uint8
	padBuf1 *uint8
)

// Button bits, from the SDK's pad.h. PadRead assembles them from four bytes
// of the two receive buffers and inverts them, since the hardware reports
// buttons active low.
const (
	padLUp    = 1 << 12
	padLDown  = 1 << 14
	padL1     = 1 << 2
	padSelect = 1 << 8
)

// padRaw reproduces the sample's PadRead: two bytes from each of the two
// receive buffers, assembled into one word and inverted.
func padRaw() uint32 {
	if padBuf0 == nil || padBuf1 == nil {
		return 0
	}
	b0, b1 := byteAt(padBuf0, 3), byteAt(padBuf0, 2)
	b2, b3 := byteAt(padBuf1, 3), byteAt(padBuf1, 2)
	return ^(uint32(b0) | uint32(b1)<<8 | uint32(b2)<<16 | uint32(b3)<<24)
}

// The files the sample loads off the disc.
// The sound data, registered with mkpsxiso by these declarations: see
// package disc.
var dataFiles = [...]struct {
	disc.File
	addr uintptr
}{
	{disc.Register("DATA/SOUND/STD0.VH", "data/STD0.VH"), vhAddr},
	{disc.Register("DATA/SOUND/STD0.VB", "data/STD0.VB"), vbAddr},
	{disc.Register("DATA/SOUND/SAMPLE1.SEQ", "data/SAMPLE1.SEQ"), seqAddr},
}

var (
	pathBuf [64]byte
	lineBuf [128]byte
	line    yaroze.Buffer

	loadNote [len(dataFiles)]string

	vab int16 = -1
	seq int16 = -1
)

// The sound data is loaded at addresses the SDK fixes, so these conversions
// from integer to pointer are deliberate rather than the mistake go vet
// normally warns about.

// at turns a fixed load address into a pointer the library can write to.
func at(addr uintptr) *uint32 {
	return (*uint32)(unsafe.Pointer(addr))
}

// bytesAt is the same for a byte pointer.
func bytesAt(addr uintptr) *uint8 {
	return (*uint8)(unsafe.Pointer(addr))
}

// byteAt reads the nth byte after p. The controller buffers are plain memory
// the library refreshes, and Go has no array type to describe them.
func byteAt(p *uint8, n uintptr) uint8 {
	return *(*uint8)(unsafe.Add(unsafe.Pointer(p), n))
}

// loadDataFiles reads the sound data. It reports whether every file
// arrived, so the caller can skip sound rather than feed the sequencer an
// empty buffer.
func loadDataFiles() bool {
	ok := true
	for i := range dataFiles {
		f := &dataFiles[i]
		status("reading sound data\n")

		// From the sector mkpsxiso recorded, or by name in an executable
		// it did not place.
		if !f.Read(at(f.addr), pathBuf[:]) {
			loadNote[i] = "not found"
			ok = false
			continue
		}

		// Bounded, unlike the C original, and with the wait visible so a
		// drive that never finishes shows up as a stalled counter rather
		// than a hang.
		done := false
		for w := 0; w < readTimeout; w++ {
			if yaroze.CdReadSync(1, nil) <= 0 {
				done = true
				break
			}
			yaroze.VSync(0)
		}
		if !done {
			loadNote[i] = "read timed out"
			ok = false
			continue
		}
		loadNote[i] = "ok"
	}
	return ok
}

func initSound() {
	vab = yaroze.SsVabTransfer(bytesAt(vhAddr), bytesAt(vbAddr), -1, 1)
	if vab < 0 {
		return
	}
	seq = yaroze.SsSeqOpen(at(seqAddr), vab)
}

func playSound() {
	if vab < 0 || seq < 0 {
		return
	}
	yaroze.SsSetMVol(mvol, mvol)
	yaroze.SsSeqSetVol(seq, svol, svol)
	yaroze.SsSeqPlay(seq, yaroze.SSPLAY_PLAY, yaroze.SSPLAY_INFINITY)
}

func stopSound() {
	if vab < 0 || seq < 0 {
		return
	}
	yaroze.SsSeqStop(seq)
	yaroze.VSync(0)
	yaroze.VSync(0)
	yaroze.SsSeqClose(seq)
	yaroze.SsVabClose(vab)
}

// initPrim loads the ball texture into video memory and sets up the sprites.
func initPrim() {
	rect := yaroze.RECT{X: 640, Y: 0, W: 16 / 4, H: 16}
	yaroze.LoadImage(&rect, &ball16x16[0])
	tpage := yaroze.GetTPage(0, 0, 640, 0)

	// 32 palettes, one row of video memory each, so each ball can have its
	// own shade without a second texture.
	for i := range ballcolor {
		r := yaroze.RECT{X: 0, Y: int16(480 + i), W: 256, H: 1}
		yaroze.LoadImage(&r, &ballcolor[i][0])
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

// padRead adjusts the ball count, returning zero to stop.
func padRead(n int) int {
	pad := padRaw()

	if pad&padLUp != 0 {
		n += 4
	}
	if pad&padLDown != 0 {
		n -= 4
	}
	if pad&padL1 != 0 {
		// Pause: hold here until the button comes back up.
		for padRaw()&padL1 != 0 {
			yaroze.VSync(0)
		}
	}
	if pad&padSelect != 0 {
		return 0
	}

	return min(max(n, 1), maxObj-1)
}

// frame draws one complete frame of flat colour, with an optional caption.
func frame(caption *byte) {
	active := yaroze.GsGetActiveBuff()
	yaroze.GsSetWorkBase(&gpuPacketArea[active][0])
	yaroze.GsClearOt(0, 0, &worldOT[active])

	yaroze.DrawSync(0)
	yaroze.VSync(0)
	yaroze.GsSwapDispBuff()

	yaroze.GsSortClear(60, 120, 120, &worldOT[active])
	yaroze.GsDrawOt(&worldOT[active])

	if caption != nil {
		yaroze.FntPrint(caption)
		yaroze.FntFlush(-1)
	}
}

// status shows a caption for about half a second, so that a slow step has
// something to show for itself rather than leaving a blank screen.
//
// It draws complete frames. That matters: FntFlush alone writes into the
// buffer that is not currently displayed, so text without the surrounding
// clear, draw and swap never appears.
func status(msg string) {
	for range 24 {
		line.Reset()
		line.WriteString(msg)
		frame(line.CString())
	}
}

func main() {
	line.Init(lineBuf[:])

	// Graphics come up before anything touches the drive, unlike the C
	// original, which loads its data first and so has nothing on screen if
	// that takes a while or does not finish. Bringing the display up first
	// costs nothing and turns a black screen into a report.
	yaroze.SetVideoMode(yaroze.MODE_NTSC)
	yaroze.GsInitGraph(frameX, frameY, 4, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, frameY)

	for i := range worldOT {
		worldOT[i].Length = otLength
		worldOT[i].Org = &otTags[i][0]
	}

	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(16, 16, 256, 200, 0, 512)

	status("loading\n")
	yaroze.GetPadBuf(&padBuf0, &padBuf1)

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

		active := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(&gpuPacketArea[active][0])
		yaroze.GsClearOt(0, 0, &worldOT[active])

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
			yaroze.GsSortFastSprite(&sprites[i], &worldOT[active], 0)
		}

		yaroze.DrawSync(0)
		cnt := yaroze.VSync(0)
		yaroze.GsSwapDispBuff()

		yaroze.GsSortClear(60, 120, 120, &worldOT[active])
		yaroze.GsDrawOt(&worldOT[active])

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
