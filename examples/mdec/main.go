// Full-color video through the motion decoder, driven by its registers.
//
// The GPU draws in 15-bit color, but it can show 24-bit pictures, and the
// MDEC, the PlayStation's motion decoder, is how games made them: it takes
// pictures compressed much as JPEG compresses them, a 16x16 macroblock at a
// time, and turns them into pixels, which a program copies into VRAM. Games
// played their video this way.
//
// The Net Yaroze library has no MDEC functions, so this drives the MDEC and
// its two DMA channels through their registers. The movie, a ray-traced
// loop that gen renders and format/mdec encodes, is read off the disc whole
// and played at 20 frames a second. Each frame is decoded a column of
// macroblocks at a time, while the other is on screen.
//
// # Controls
//
// Hold Start and Select together to quit.
package main

//go:generate go run ./gen -o assets/MOVIE.MDC

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

// The movie, read into the program's own memory, whole, in a buffer of
// whole sectors.
var (
	movieData [(647180 + 2047) / 2048 * 512]uint32
	movieFile = disc.Register("ASSETS/MOVIE.MDC", "assets/MOVIE.MDC").Into(movieData[:])
	files     = []*disc.File{&movieFile}
)

// The movie's layout; see format/mdec.
const (
	movieMagic  = 'M' | 'D'<<8 | 'E'<<16 | 'C'<<24
	tableWords  = 2 * 33 // the commands that load the tables, with them
	headerWords = 3 + tableWords
)

// MDEC and DMA registers.
var (
	mdecData = reg(0x1f801820) // commands and their parameters in, pixels out
	mdecCtrl = reg(0x1f801824)

	dma0MADR = reg(0x1f801080) // MDEC in
	dma0BCR  = reg(0x1f801084)
	dma0CHCR = reg(0x1f801088)
	dma1MADR = reg(0x1f801090) // MDEC out
	dma1BCR  = reg(0x1f801094)
	dma1CHCR = reg(0x1f801098)
	dpcr     = reg(0x1f8010f0)
)

func reg(addr uintptr) *volatile.Register32 {
	return (*volatile.Register32)(yaroze.Pointer(addr))
}

const (
	mdecReset     = 1 << 31
	mdecEnableDMA = 1<<30 | 1<<29 // in and out

	dmaBusy       = 1 << 24
	dmaToDevice   = 1 << 0
	dmaSyncBlocks = 1 << 9 // as the device asks for them
	dmaBlock      = 32     // words; the movie pads each frame to a multiple

	// Each channel's priority and enable bits in DPCR.
	dpcrMDEC = 0x8<<0 | 0x8<<4
)

// A column of macroblocks, 16 pixels wide and the screen high, in 24-bit
// pixels: 24 halfwords of VRAM a row.
const (
	columnWidth = 16 * 3 / 2
	maxHeight   = 240
	columnWords = 16 * maxHeight * 3 / 4
)

// column is where a column of pixels goes on its way from the MDEC to VRAM.
var column [columnWords]uint32

// The movie, as read off the disc.
var (
	movieWidth, movieHeight int16
	movieFrames             int
	frameAt                 [64]*uint32 // each frame's length word

	// shown counts the pictures shown, which a test reads. The loop counts
	// on it directly, so that the compiler keeps it.
	shown int
)

// openMovie checks the movie, finds its frames and loads the MDEC's tables.
func openMovie() bool {
	header := movieData[:headerWords]
	if header[0] != movieMagic {
		return false
	}
	movieWidth, movieHeight = int16(header[1]), int16(header[1]>>16)
	movieFrames = int(header[2] & 0xffff)
	if movieHeight > maxHeight || movieWidth > 1024/3*2 || movieFrames > len(frameAt) {
		return false
	}
	p := uintptr(unsafe.Pointer(&movieData[headerWords]))
	for i := range movieFrames {
		frameAt[i] = yaroze.At(p)
		p += 4 * uintptr(1+*frameAt[i])
	}

	mdecCtrl.Set(mdecReset)
	mdecCtrl.Set(mdecEnableDMA)
	for _, w := range header[3:] {
		mdecData.Set(w)
	}
	dpcr.Set(dpcr.Get() | dpcrMDEC)
	return true
}

// decode decodes frame i into VRAM with its top left corner at (0, y).
func decode(i int, y int16) {
	// The decode command and its codes go in on channel 0, which feeds the
	// MDEC as it asks. It is left running: the MDEC only takes more as its
	// pixels are taken out.
	n := *frameAt[i]
	dma0MADR.Set(uint32(uintptr(unsafe.Pointer(frameAt[i]))) + 4)
	dma0BCR.Set(n/dmaBlock<<16 | dmaBlock)
	dma0CHCR.Set(dmaBusy | dmaSyncBlocks | dmaToDevice)

	// The pixels come out a macroblock at a time, column by column, so a
	// column's worth is a strip of VRAM.
	rect := yaroze.RECT{Y: y, W: columnWidth, H: movieHeight}
	words := uint32(movieHeight) * 16 * 3 / 4
	for x := int16(0); x < movieWidth/16; x++ {
		dma1MADR.Set(uint32(uintptr(unsafe.Pointer(&column[0]))))
		dma1BCR.Set(words/dmaBlock<<16 | dmaBlock)
		dma1CHCR.Set(dmaBusy | dmaSyncBlocks)
		for dma1CHCR.Get()&dmaBusy != 0 {
		}
		rect.X = x * columnWidth
		yaroze.LoadImage(&rect, &column[0])
		yaroze.DrawSync(0)
	}
}

// show puts the 24-bit picture at (0, y) on screen.
func show(y int16) {
	env := yaroze.DISPENV{
		Disp:    yaroze.RECT{X: 0, Y: y, W: movieWidth, H: movieHeight},
		Isrgb24: 1,
	}
	yaroze.PutDispEnv(&env)
}

var (
	lineBuf [96]byte
	line    yaroze.Buffer
)

// say shows text for a frame, in the 15-bit display the library sets up.
func say(text, detail string) {
	line.Reset()
	line.WriteString(text).WriteString(detail)
	line.Print()
	yaroze.DrawSync(0)
	yaroze.VSync(0)
	yaroze.GsSwapDispBuff()
	screen := yaroze.RECT{W: 320, H: 512}
	yaroze.ClearImage(&screen, 0, 0, 0)
	yaroze.FntFlush(-1)
}

func quitting() bool {
	return yaroze.PadRead()&(yaroze.PADstart|yaroze.PADselect) == yaroze.PADstart|yaroze.PADselect
}

// fail shows what went wrong until the program is quit.
func fail(text, detail string) {
	for !quitting() {
		say(text, detail)
	}
	yaroze.ResetGraph(1)
}

func main() {
	line.Init(lineBuf[:])
	yaroze.ResetGraph(0)
	yaroze.PadInit()

	yaroze.GsInitGraph(320, 240, 4, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 256)
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(16, 16, 288, 200, 0, 256)

	if missing := demo.Load(files, func(string) { say("loading\n", "") }); missing != "" {
		fail("could not read\n", missing)
		return
	}
	if !openMovie() {
		fail("not a movie this program can play\n", "")
		return
	}

	// Two pictures, one above the other: the one on screen, and the one
	// being decoded into.
	buffers := [2]int16{0, 256}
	decode(0, buffers[0])
	show(buffers[0])
	yaroze.VSync(0)

	for shown = 1; ; shown++ {
		back := buffers[shown&1]
		decode(shown%movieFrames, back)
		yaroze.VSync(3) // 20 frames a second
		show(back)
		if quitting() {
			break
		}
	}
	yaroze.ResetGraph(1)
}
