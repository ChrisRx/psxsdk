package emu

import (
	"fmt"
	"math"
)

// VRAM dimensions, in 16-bit pixels.
const (
	vramWidth  = 1024
	vramHeight = 512
)

// GPU timing, in GPU cycles, for NTSC. psxe does not emulate PAL timing.
const (
	gpuHDrawCycles    float32 = 2560
	gpuScanlineCycles float32 = 3413
	gpuVDrawLines             = 240
	gpuFrameLines             = 263
	gpuClockMHz       float32 = 53.693175
	cpuClockMHz       float32 = 33.8688
	gpuCyclesPerCPU           = gpuClockMHz / cpuClockMHz
	gpustatDisplayOff         = 1 << 23
	gpustatOddLine            = 1 << 31
	gpustatField              = 1 << 13    // interlace field
	gpustatReadyBits          = 0x1c000000 // ready for commands, for VRAM reads, for DMA
)

// GP0 command states.
const (
	gpuRecvCmd = iota
	gpuRecvArgs
	gpuRecvData
)

// gpu is the graphics processor at 1F801810: GP0 and GPUREAD at offset 0,
// GP1 and GPUSTAT at offset 4.
//
// This is psxe's command processing and timing without its rasterizer:
// commands that draw polygons, lines and rectangles are parsed, and their
// side effects on the GPU's state applied, but nothing is drawn. Commands
// that move pixels around VRAM directly, and transfers to and from the CPU,
// are emulated.
type gpu struct {
	vram [vramWidth * vramHeight]uint16

	state         int
	buf           [16]uint32
	bufIndex      int
	argsRemaining int
	recvData      uint32

	// A transfer from the CPU into VRAM (GP0 A0h).
	xpos, ypos, xsiz, ysiz uint32
	tsiz                   uint32
	xcnt, ycnt             uint32

	// A transfer from VRAM to the CPU (GP0 C0h), read through GPUREAD.
	c0Addr       uint32
	c0Xcnt       uint32
	c0Ycnt       uint32
	c0Xsiz       int
	c0Ysiz       int
	c0Tsiz       int
	gp1InfoReq   uint32
	gpustat      uint32
	displayMode  uint32
	texpX, texpY uint32
	texpD        uint32

	texwMX, texwMY, texwOX, texwOY uint32
	drawX1, drawY1, drawX2, drawY2 uint32
	offX, offY                     int32
	dispX, dispY                   uint32
	dispX1, dispX2, dispY1, dispY2 uint32

	cycles   float32
	scanline int
	frames   uint64 // vertical blanks so far

	// hblanking is whether the clock was in horizontal blanking after the
	// last update, and edge the earliest it can be on the other side.
	// Nothing but update moves the clock, so until it reaches edge, an
	// update has nothing more to do.
	hblanking bool
	edge      float32

	ic     *ic
	timers *timers
	stop   func(error)
}

// newGPU returns a GPU with its display off, reporting an error that stops
// the run to stop.
func newGPU(ic *ic, stop func(error)) *gpu {
	return &gpu{
		state:       gpuRecvCmd,
		gpustat:     gpustatDisplayOff,
		displayMode: 1,
		ic:          ic,
		stop:        stop,
	}
}

func (g *gpu) Read32(off uint32) uint32 {
	switch off {
	case 0:
		return g.gpuread()
	case 4:
		return g.gpustat | gpustatReadyBits
	}
	return 0
}

func (g *gpu) Read16(uint32) uint16   { return 0 }
func (g *gpu) Read8(uint32) uint8     { return 0 }
func (g *gpu) Write16(uint32, uint16) {}
func (g *gpu) Write8(uint32, uint8)   {}

// vramAt reads VRAM at a linear index. psxe does not bound the index for a
// download running off the end of VRAM; it wraps here.
func (g *gpu) vramAt(i uint32) uint32 {
	return uint32(g.vram[i&(vramWidth*vramHeight-1)])
}

// gpuread returns the next word of a VRAM download, or the answer to a GP1
// 10h information request, which replaces it.
func (g *gpu) gpuread() uint32 {
	var data uint32
	if g.c0Tsiz != 0 {
		next := func() uint32 {
			v := g.vramAt(g.c0Addr + g.c0Xcnt + g.c0Ycnt*vramWidth)
			g.c0Xcnt++
			if int(g.c0Xcnt) == g.c0Xsiz {
				g.c0Ycnt++
				g.c0Xcnt = 0
			}
			return v
		}
		data = next()
		data |= next() << 16
		g.c0Tsiz -= 2
	}

	if g.gp1InfoReq != 0 {
		switch g.gp1InfoReq & 7 {
		case 2:
			data = (g.texwOY/8)<<15 | (g.texwOX/8)<<10 | (g.texwMY/8)<<5 | g.texwMX/8
		case 3:
			data = g.drawY1<<10 | g.drawX1
		case 4:
			data = g.drawY2<<10 | g.drawX2
		case 5:
			data = uint32(g.offY<<10 | g.offX)
		}
		g.gp1InfoReq = 0
	}
	return data
}

func (g *gpu) Write32(off, v uint32) {
	switch off {
	case 0:
		g.gp0(v)
	case 4:
		g.gp1(v)
	}
}

func (g *gpu) gp0(v uint32) {
	switch g.state {
	case gpuRecvCmd:
		g.bufIndex = 0
		g.push(v)
	case gpuRecvArgs:
		g.push(v)
		g.argsRemaining--
	case gpuRecvData:
		g.recvData = v
	}
	g.command()
}

// push adds a word to the command buffer. psxe's buffer holds 16 words and
// is not bounded; only a polyline can run past it, which psxe then lets
// overwrite the GPU state after the buffer. That is not reproduced.
func (g *gpu) push(v uint32) {
	if g.bufIndex >= len(g.buf) {
		g.stop(fmt.Errorf("GPU polyline longer than psxe's %d-word command buffer", len(g.buf)))
		g.state = gpuRecvCmd
		return
	}
	g.buf[g.bufIndex] = v
	g.bufIndex++
}

func (g *gpu) gp1(v uint32) {
	switch v >> 24 {
	case 0x03: // display enable
		g.gpustat = g.gpustat&^gpustatDisplayOff | (v<<23)&gpustatDisplayOff
	case 0x05: // start of display area in VRAM
		g.dispX = v & 0x3ff
		g.dispY = v >> 10 & 0x1ff
	case 0x06: // horizontal display range
		g.dispX1 = v & 0xfff
		g.dispX2 = v >> 12 & 0xfff
	case 0x07: // vertical display range
		g.dispY1 = v & 0x1ff
		g.dispY2 = v >> 10 & 0x1ff
	case 0x08: // display mode
		g.displayMode = v & 0xffffff
	case 0x10: // information request
		g.gp1InfoReq = v & 7
	}
	// psxe does not implement the rest, including the resets (00h and
	// 01h) and the interrupt acknowledge (02h).
}

// command acts on the command buffer, as psxe's psx_gpu_update_cmd does
// after every GP0 write.
func (g *gpu) command() {
	cmd := g.buf[0] >> 24

	// Every command from 20h to 7Fh draws: polygons, lines, rectangles.
	switch cmd >> 5 {
	case 1:
		g.polygon()
		return
	case 2:
		g.line()
		return
	case 3:
		g.rectangle()
		return
	}

	switch cmd {
	case 0x02:
		g.fill()
	case 0x80:
		g.copyVRAM()
	case 0xa0:
		g.upload()
	case 0xc0:
		g.download()
	case 0xe1: // draw mode
		g.gpustat = g.gpustat&0xfffff800 | g.buf[0]&0x7ff
		g.texpX = (g.gpustat & 0xf) << 6
		g.texpY = (g.gpustat & 0x10) << 4
		g.texpD = g.gpustat >> 7 & 3
	case 0xe2: // texture window
		g.texwMX = (g.buf[0] >> 0 & 0x1f) << 3
		g.texwMY = (g.buf[0] >> 5 & 0x1f) << 3
		g.texwOX = (g.buf[0] >> 10 & 0x1f) << 3
		g.texwOY = (g.buf[0] >> 15 & 0x1f) << 3
	case 0xe3: // drawing area top left
		g.drawX1 = g.buf[0] & 0x3ff
		g.drawY1 = g.buf[0] >> 10 & 0x1ff
	case 0xe4: // drawing area bottom right
		g.drawX2 = g.buf[0] & 0x3ff
		g.drawY2 = g.buf[0] >> 10 & 0x1ff
	case 0xe5: // drawing offset, two signed 11-bit values
		g.offX = int32(g.buf[0]&0x7ff<<21) >> 21
		g.offY = int32(g.buf[0]>>11&0x7ff<<21) >> 21
	}
	// The rest, nop and cache clear included, do nothing in psxe.
}

// expect starts a command that takes n argument words, and reports whether
// they have all arrived.
func (g *gpu) expect(n int) bool {
	if g.state == gpuRecvCmd {
		g.state = gpuRecvArgs
		g.argsRemaining = n
		return false
	}
	return g.state == gpuRecvArgs && g.argsRemaining == 0
}

func (g *gpu) polygon() {
	shaded := g.buf[0]&0x10000000 != 0
	quad := g.buf[0]&0x08000000 != 0
	textured := g.buf[0]&0x04000000 != 0

	perVertex := 1 + b2i(shaded) + b2i(textured)
	vertices := 3 + b2i(quad)
	if !g.expect(perVertex*vertices - b2i(shaded)) {
		return
	}

	// The words of each vertex: an optional colour, the position, and an
	// optional texture coordinate. The first vertex's colour is the
	// command word itself.
	s, t := b2i(shaded), b2i(textured)
	colorOff := s * (2 + t)
	vertOff := 1 + (t | s) + (t & s)
	texcOff := t * (2 + s)
	texpOff := t * (4 + s)

	d := polyData{
		attrib: uint8(g.buf[0] >> 24),
		clut:   uint16(g.buf[2] >> 16),
		texp:   uint16(g.buf[texpOff] >> 16),
	}

	// A textured polygon carries a texture page, which psxe applies to the
	// GPU's state as E1h would.
	if textured {
		texp := uint32(d.texp)
		g.texpX = (texp & 0xf) << 6
		g.texpY = (texp & 0x10) << 4
		g.texpD = texp >> 7 & 3
		g.gpustat = g.gpustat&0xfffffe00 | texp&0x1ff
	}

	// psxe reads a fourth vertex for a triangle too, from whatever the
	// buffer holds; it is not drawn.
	for i := range d.v {
		pos := g.buf[1+i*vertOff]
		texc := g.buf[2+i*texcOff]
		d.v[i] = vertex{
			c:  g.buf[i*colorOff] & 0xffffff,
			x:  se11(pos & 0xffff),
			y:  se11(pos >> 16),
			tx: uint8(texc),
			ty: uint8(texc >> 8),
		}
	}

	g.triangle(d.v[0], d.v[1], d.v[2], d)
	if quad {
		g.triangle(d.v[1], d.v[2], d.v[3], d)
	}
	g.state = gpuRecvCmd
}

func (g *gpu) line() {
	shaded := g.buf[0]&0x10000000 != 0
	polyline := g.buf[0]&0x08000000 != 0

	if g.state == gpuRecvCmd {
		g.state = gpuRecvArgs
		switch {
		case polyline:
			g.argsRemaining = -1 // until the terminator
		case shaded:
			g.argsRemaining = 3
		default:
			g.argsRemaining = 2
		}
		return
	}
	if g.state != gpuRecvArgs {
		return
	}
	// psxe parses polylines but does not draw them.
	if polyline {
		if g.buf[g.bufIndex-1]&0xf000f000 == 0x50005000 {
			g.state = gpuRecvCmd
		}
		return
	}
	if g.argsRemaining != 0 {
		return
	}
	// Positions are not sign-extended from 11 bits here as they are for
	// polygons, and a shaded line is drawn in its first colour.
	p0 := g.buf[1]
	p1 := g.buf[2]
	if shaded {
		p1 = g.buf[3]
	}
	g.drawLine(
		vertex{x: int16(p0), y: int16(p0 >> 16)},
		vertex{x: int16(p1), y: int16(p1 >> 16)},
		bgr555(g.buf[0]&0xffffff),
	)
	g.state = gpuRecvCmd
}

func (g *gpu) rectangle() {
	size := g.buf[0] >> 27 & 3
	textured := g.buf[0]&0x04000000 != 0
	if !g.expect(1 + b2i(size == 0) + b2i(textured)) {
		return
	}
	d := rectData{
		attrib: uint8(g.buf[0] >> 24),
		v0: vertex{
			c:  g.buf[0] & 0xffffff,
			x:  se11(g.buf[1] & 0xffff),
			y:  se11(g.buf[1] >> 16),
			tx: uint8(g.buf[2]),
			ty: uint8(g.buf[2] >> 8),
		},
		clut: uint16(g.buf[2] >> 16),
	}
	dims := g.buf[2+b2i(textured)]
	d.width, d.height = uint16(dims), uint16(dims>>16)
	// An unmodulated textured rectangle is modulated by the neutral colour,
	// which leaves the texel as it was apart from its top bit.
	if textured && d.attrib&attrRaw != 0 {
		d.v0.c = 0x808080
	}
	g.drawRect(d)
	g.state = gpuRecvCmd
}

// fill fills a rectangle of VRAM with a colour, in units of 16 pixels
// across.
func (g *gpu) fill() {
	if !g.expect(2) {
		return
	}
	x0 := int(g.buf[1] & 0xffff & 0x3f0)
	y0 := int(g.buf[1] >> 16 & 0x1ff)
	w := int((g.buf[2]&0x3ff + 0x0f) & 0xfffffff0)
	h := int(g.buf[2] >> 16 & 0x1ff)
	color := bgr555(g.buf[0] & 0xffffff)
	// What falls outside VRAM is not drawn.
	x1 := min(x0+w, vramWidth)
	for y := y0; y < min(y0+h, vramHeight); y++ {
		if x0 < x1 {
			fill16(g.vram[y*vramWidth+x0:y*vramWidth+x1], color)
		}
	}
	g.state = gpuRecvCmd
}

// copyVRAM copies a rectangle within VRAM. Pixels that would come from or
// go to outside it are skipped rather than wrapped.
func (g *gpu) copyVRAM() {
	if !g.expect(3) {
		return
	}
	srcX, srcY := g.buf[1]&0xffff, g.buf[1]>>16
	dstX, dstY := g.buf[2]&0xffff, g.buf[2]>>16
	w, h := g.buf[3]&0xffff, g.buf[3]>>16
	for y := range h {
		for x := range w {
			if dstX+x < vramWidth && dstY+y < vramHeight && srcX+x < vramWidth && srcY+y < vramHeight {
				g.vram[(dstX+x)+(dstY+y)*vramWidth] = g.vram[(srcX+x)+(srcY+y)*vramWidth]
			}
		}
	}
	g.state = gpuRecvCmd
}

// upload receives a rectangle of pixels from the CPU, two to a word.
func (g *gpu) upload() {
	switch g.state {
	case gpuRecvCmd, gpuRecvArgs:
		if !g.expect(2) {
			return
		}
		g.state = gpuRecvData
		g.xpos = g.buf[1] & 0x3ff
		g.ypos = g.buf[1] >> 16 & 0x1ff
		g.xsiz = ((g.buf[2]&0xffff - 1) & 0x3ff) + 1
		g.ysiz = ((g.buf[2]>>16 - 1) & 0x1ff) + 1
		g.tsiz = (g.xsiz*g.ysiz + 1) &^ 1
		g.xcnt, g.ycnt = 0, 0
	case gpuRecvData:
		g.uploadPixel(uint16(g.recvData))
		g.uploadPixel(uint16(g.recvData >> 16))
		g.tsiz -= 2
		if g.tsiz == 0 {
			g.xcnt, g.ycnt = 0, 0
			g.state = gpuRecvCmd
		}
	}
}

func (g *gpu) uploadPixel(p uint16) {
	x := (g.xpos + g.xcnt) & 0x3ff
	y := (g.ypos + g.ycnt) & 0x1ff
	g.vram[x+y*vramWidth] = p
	g.xcnt++
	if g.xcnt == g.xsiz {
		g.ycnt++
		g.xcnt = 0
	}
}

// download sets up a rectangle of VRAM to be read through GPUREAD.
func (g *gpu) download() {
	if !g.expect(2) {
		return
	}
	g.c0Xcnt, g.c0Ycnt = 0, 0
	x := g.buf[1] & 0xffff & 0x3ff
	y := g.buf[1] >> 16 & 0x1ff
	g.c0Xsiz = int(((g.buf[2]&0xffff)-1)&0x3ff) + 1
	g.c0Ysiz = int(((g.buf[2]>>16)-1)&0x1ff) + 1
	g.c0Tsiz = (g.c0Xsiz*g.c0Ysiz + 1) &^ 1
	g.c0Addr = x + y*vramWidth
	g.state = gpuRecvCmd
}

func bgr555(c uint32) uint16 {
	r := c >> 3 & 0x1f
	gr := c >> 11 & 0x1f
	b := c >> 19 & 0x1f
	return uint16(r | gr<<5 | b<<10)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// update advances the GPU's clock by the CPU cycles a step took, firing the
// blanking events as it crosses them.
//
// psxe keeps the clock as a float within the current scanline: horizontal
// blanking is the stretch from the end of the visible part to the end of
// the line, and the line ends when the clock passes it. The float
// arithmetic is reproduced exactly, conversion by conversion, since the
// point an interrupt is raised depends on it.
func (g *gpu) update(cpuCycles uint32) {
	g.cycles = float32(g.cycles + float32(float32(cpuCycles)*gpuCyclesPerCPU))
	if g.cycles >= g.edge {
		g.crossEdge()
	}
}

// crossEdge fires the blanking event the clock has crossed, if any. Short of
// the line's end, the clock only moves into blanking, at gpuHDrawCycles;
// once in it, only out of it, past gpuScanlineCycles.
func (g *gpu) crossEdge() {
	prev := g.hblanking
	curr := g.inHBlank()

	switch {
	case curr && !prev:
		g.timers.hblankStart()
		g.hblank()
	case prev && !curr:
		g.timers.hblankEnd()
		g.cycles = float32(g.cycles - gpuScanlineCycles)
	}

	g.hblanking = g.inHBlank()
	g.edge = gpuHDrawCycles
	if g.hblanking {
		g.edge = math.Nextafter32(gpuScanlineCycles, math.MaxFloat32)
	}
}

func (g *gpu) inHBlank() bool {
	return g.cycles >= gpuHDrawCycles && g.cycles <= gpuScanlineCycles
}

// hblank moves to the next scanline, raising the vertical blanking
// interrupt as the visible lines end.
func (g *gpu) hblank() {
	if g.scanline < gpuVDrawLines && g.scanline&1 != 0 {
		g.gpustat |= gpustatOddLine
	} else {
		g.gpustat &^= gpustatOddLine
	}

	g.scanline++
	switch g.scanline {
	case gpuVDrawLines:
		g.timers.vblankStart()
		g.frames++
		g.ic.raise(irqVBlank)
	case gpuFrameLines:
		g.timers.vblankEnd()
		// The interlace field alternates every field while interlaced
		// output is selected, and reads as 1 otherwise. psxe does not do
		// this; nix/psxe-interlace-field.patch adds it, since the OpenBIOS
		// shell waits on the field rather than the vertical blank.
		if g.displayMode&0x20 != 0 {
			g.gpustat ^= gpustatField
		} else {
			g.gpustat |= gpustatField
		}
		g.scanline = 0
	}
}
