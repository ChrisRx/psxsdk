//go:build yaroze

package main

import (
	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

var (
	gp0     = (*uint32)(yaroze.Pointer(0x1f801810))
	gp1     = (*uint32)(yaroze.Pointer(0x1f801814))
	gpuStat = gp1

	// Root counter 1, which counts the display's lines: the monitor's clock.
	rcnt1Count = (*uint32)(yaroze.Pointer(0x1f801110))
	rcnt1Mode  = (*uint32)(yaroze.Pointer(0x1f801114))
)

// gpuReady is GPUSTAT's bit for a GP0 command word being taken. On a console
// it's clear while a command's parameters are still owed, so it's waited for
// only before a command's first word.
const gpuReady = 1 << 26

// gpuWaitLimit is how many times gpuCommand reads GPUSTAT before it gives up
// on the GPU. The screen is a help, and must never stop the monitor starting.
const gpuWaitLimit = 100000

// gpuOK is cleared once the GPU fails to take a command, and the rest of the
// screen is skipped.
var gpuOK bool

// gpuSettle reads GPUSTAT n times, as a pause the GPU can count on.
func gpuSettle(n int) {
	for range n {
		volatile.LoadUint32(gpuStat)
	}
}

// gpuCommand sends a GP0 command's first word once the GPU will take one.
// While it waits, which on a console can be a while after a fill or a copy,
// it takes in what the serial port has received: the port holds only 8
// bytes, and the host may be sending. It is called from many places, and
// kept out of line, which keeps the monitor within its memory.
//
//go:noinline
func gpuCommand(w uint32) {
	if !gpuOK {
		return
	}
	for range gpuWaitLimit {
		line.poll()
		if volatile.LoadUint32(gpuStat)&gpuReady != 0 {
			volatile.StoreUint32(gp0, w)
			return
		}
	}
	gpuOK = false
}

// gpuParam sends a parameter of the command gpuCommand started, which goes
// into the GPU's command buffer without a wait.
func gpuParam(w uint32) {
	if gpuOK {
		volatile.StoreUint32(gp0, w)
	}
}

// Where the screen keeps things in video memory: two buffers, the console
// in the first and the logo in both, a frame in one while the other is
// shown, and the font, 4-bit, with its palette under it.
const (
	bufferY = 256 // the second buffer's top
	fontX   = 512
	fontY   = 0
	clutX   = fontX
	clutY   = 48

	// texpage is GP0 E1h for the font: its page, 4-bit, and drawing to the
	// displayed area allowed.
	texpage = 0xe1000000 | 1<<10 | fontX/64
	clut    = clutY<<6 | clutX/16
)

// Colors, as GP0 takes them, blue in the high byte. Text is the font's
// white made dimmer by the color it is drawn with, 0x80 being as it is.
const (
	textColor   = 0x505050 // a mid gray
	accentColor = 0x203860 // amber, for the monitor's name
	ruleColor   = 0x202020
	barColor    = 0x304830
)

// clockRaw reads the monitor's clock, the display's lines, 16 bits.
func clockRaw() uint16 { return uint16(volatile.LoadUint32(rcnt1Count)) }

// showScreen puts the monitor's console up, from scratch: the GPU reset and
// set to 320 by 240, the font loaded, and everything drawn. It talks to the
// GPU directly rather than through the library, whose state a program then
// starts from as the disc left it, and resets the GPU first, since a program
// that returned may have left it in any state. It starts the clock too:
// root counter 1 counting the display's lines.
func showScreen() {
	volatile.StoreUint32(rcnt1Mode, 0x0100)
	con.raw = clockRaw()
	activity = true
	con.state = stateReady
	con.full, con.status = true, true

	volatile.StoreUint32(gp1, 0x00000000) // reset
	gpuSettle(1000)
	volatile.StoreUint32(gp1, 0x01000000) // reset the command buffer
	gpuSettle(1000)
	displayMode()
	gpuOK = true
	loadFont()
	con.tick(clockRaw())
	drawConsole()
	if !gpuOK {
		// Leave the GPU able to take a program's commands, with the display
		// off as the reset left it, rather than showing whatever is in VRAM.
		volatile.StoreUint32(gp1, 0x01000000)
		return
	}
	volatile.StoreUint32(gp1, 0x03000000) // display on
}

// displayMode sets the display to 320 by 240, from 0,0.
func displayMode() {
	volatile.StoreUint32(gp1, 0x08000001)                         // 320 by 240, NTSC
	volatile.StoreUint32(gp1, 0x06000000|(0x260+320*8)<<12|0x260) // horizontal range
	volatile.StoreUint32(gp1, 0x07000000|(16+240)<<10|16)         // vertical range
	volatile.StoreUint32(gp1, 0x05000000)                         // display from 0,0
	shown = 0
}

// loadFont puts the font into video memory, a glyph to each 8 by 8 cell,
// sixteen to a row, a dot of color 1, and its palette: 0 clear, 1 white.
func loadFont() {
	gpuCommand(0x01000000) // clear the texture cache
	gpuCommand(0xa0000000)
	gpuParam(fontY<<16 | fontX)
	gpuParam(48<<16 | 32) // 128 by 48 dots of 4 bits, in 16-bit units
	for y := range 48 {
		for col := range 16 {
			// A word is a row of a cell: eight dots, the leftmost in the
			// low nibble, the glyph in the middle five.
			var w uint32
			if g, r := y/8*16+col, y%8; g < fontCount && r < 7 {
				bits := fontRows[g][r]
				for x := range 5 {
					if bits&(0x10>>x) != 0 {
						w |= 1 << (4 * (x + 1))
					}
				}
			}
			gpuParam(w)
		}
	}
	gpuCommand(0xa0000000)
	gpuParam(clutY<<16 | clutX)
	gpuParam(1<<16 | 16)
	gpuParam(0x7fff << 16)
	for range 7 {
		gpuParam(0)
	}
}

// drawTo directs drawing at the buffer from line top.
func drawTo(top int) {
	gpuCommand(0xe3000000 | uint32(top)<<10)
	gpuCommand(0xe4000000 | uint32(top+239)<<10 | 319)
	gpuCommand(0xe5000000 | uint32(top)<<11)
}

// gpuFill paints a rectangle of video memory, in absolute coordinates, x and w
// to 16 dots.
func gpuFill(x, y, w, h int, color uint32) {
	gpuCommand(0x02000000 | color)
	gpuParam(uint32(y)<<16 | uint32(x))
	gpuParam(uint32(h)<<16 | uint32(w))
}

// gpuRect draws a rectangle in the buffer drawTo set.
func gpuRect(x, y, w, h int, color uint32) {
	gpuCommand(0x60000000 | color)
	gpuParam(uint32(y)<<16 | uint32(x&0xffff))
	gpuParam(uint32(h)<<16 | uint32(w))
}

// gpuText draws s at x, y in cells step dots apart, in color. Between
// characters it takes in what the serial port has received, so that a
// line drawn while the host sends loses nothing.
//
//go:noinline
func gpuText(x, y int, s []byte, step int, color uint32) {
	gpuCommand(texpage)
	for i, c := range s {
		line.poll()
		if c <= ' ' || c > '~' {
			continue
		}
		g := int(c) - fontFirst
		gpuCommand(0x74000000 | color)
		gpuParam(uint32(y)<<16 | uint32(x+i*step))
		gpuParam(clut<<16 | uint32(g/16*8)<<8 | uint32(g%16*8))
	}
}

// screenTick is the console's turn, while the monitor waits for the host:
// the clock read, and whatever is due drawn.
func screenTick() {
	if con.state == stateRun || con.state == stateRDB {
		return
	}
	con.tick(clockRaw())
	drawConsole()
}

// statusTick draws the status lines during a transfer, at most four times a
// second.
func statusTick() {
	con.tick(clockRaw())
	if con.saver || con.clock-con.statusAt < statusLines {
		return
	}
	drawStatus()
}

// shown is which buffer is on the screen, 0 or bufferY.
var shown int

// drawConsole draws what has changed, or the logo's next step.
func drawConsole() {
	if !gpuOK || con.held {
		return
	}
	if con.remode {
		con.remode = false
		displayMode()
		volatile.StoreUint32(gp1, 0x03000000) // display on
	}
	if con.saver {
		// Paced by vertical blanks, not the clock: the library counts them
		// in its interrupt, and reading the count waits for nothing.
		con.full = false
		flip, next := con.logo.pace(yaroze.VSync(-1))
		if flip {
			showLogo()
		}
		if next {
			drawLogo()
		}
		return
	}
	if shown != 0 {
		shown = 0
		volatile.StoreUint32(gp1, 0x05000000)
	}
	drawTo(0)
	ox, oy := con.origin()
	switch {
	case con.full:
		con.full = false
		gpuFill(0, 0, 320, 240, 0)
		gpuRect(conX+ox, conBar+oy, conCols*8, 1, ruleColor)
		con.status = true
		con.scrolls = 0
		for r := range conLogRows {
			con.dirty[r] = true
		}
	case con.scrolls >= conLogRows:
		con.scrolls = 0
		gpuFill(0, conLogY+oy, 320, conLogRows*8, 0)
		for r := range conLogRows {
			con.dirty[r] = true
		}
	case con.scrolls > 0:
		// The log moves up in video memory, and only the new lines at the
		// bottom are drawn.
		n := con.scrolls
		con.scrolls = 0
		gpuCommand(0x80000000)
		gpuParam(uint32(conLogY+oy+n*8)<<16 | uint32(conX+ox))
		gpuParam(uint32(conLogY+oy)<<16 | uint32(conX+ox))
		gpuParam(uint32((conLogRows-n)*8)<<16 | conCols*8)
		for r := conLogRows - n; r < conLogRows; r++ {
			con.dirty[r] = true
		}
	}
	if con.status {
		drawStatus()
	}
	for r := range conLogRows {
		if !con.dirty[r] {
			continue
		}
		con.dirty[r] = false
		y := conLogY + oy + r*8
		gpuFill(0, y, 320, 8, 0)
		gpuText(conX+ox, y, con.line(r), 8, textColor)
	}
}

// statusLines are the status lines as last written out.
var statusA, statusB [conCols]byte

// drawStatus draws the two status lines and, during a transfer, its
// progress on the rule under them.
func drawStatus() {
	if !gpuOK {
		return
	}
	con.status = false
	con.statusAt = con.clock
	drawTo(0)
	ox, oy := con.origin()
	con.statusText(&statusA, &statusB)
	gpuFill(0, conY+oy, 320, 17, 0)
	gpuText(conX+ox, conY+oy, statusA[:13], 8, accentColor)
	gpuText(conX+ox+13*8, conY+oy, statusA[13:], 8, textColor)
	gpuText(conX+ox, conY+oy+9, statusB[:], 8, textColor)
	gpuRect(conX+ox, conBar+oy, conCols*8, 1, ruleColor)
	if con.state == stateReceive || con.state == stateSend {
		if w := con.progress(conCols * 8); w > 0 {
			gpuRect(conX+ox, conBar+oy, w, 1, barColor)
		}
	}
}

// discWidths are the half-widths of the logo's disc, a row each.
var discWidths = [logoDisc]int{20, 34, 41, 47, 50, 52, 52, 52, 50, 47, 41, 34, 20}

// logoTop is the buffer drawLogo drew the logo's next frame into.
var logoTop int

// showLogo shows the frame drawLogo drew last.
func showLogo() {
	shown = logoTop
	volatile.StoreUint32(gp1, 0x05000000|uint32(logoTop)<<10)
}

// drawLogo draws the logo where it is, in the buffer not on the screen, for
// showLogo to show.
func drawLogo() {
	top := bufferY - shown
	logoTop = top
	drawTo(top)
	gpuFill(0, top, 320, 240, 0)
	l := &con.logo
	color := logoColors[l.color]
	for i := range len(logoWord) {
		g := fontRows[logoWord[i]-fontFirst]
		for y := range 7 {
			for x := range 5 {
				if g[y]&(0x10>>x) != 0 {
					gpuRect(l.x+(i*6+x)*3, l.y+y*3, 3, 3, color)
				}
			}
		}
	}
	dy := l.y + logoText + logoGap
	for y, hw := range discWidths {
		gpuRect(l.x+logoW/2-hw, dy+y, 2*hw+1, 1, color)
	}
	// MONITOR, cut out of the disc in black.
	const name = "MONITOR"
	var b [len(name)]byte
	copy(b[:], name)
	gpuText(l.x+(logoW-len(name)*6)/2-1, dy+3, b[:], 6, 0)
}
