package main

// The monitor's screen is a console: what the monitor sends the host,
// scrolling up, under two status lines that say what the monitor is doing.
// It is drawn dim on black, and moved a little every minute, so that a CRT
// left on the monitor for hours doesn't keep a picture of it. After five
// minutes with nothing on the cable the console gives way to a logo that
// bounces around the screen, and the next byte brings the console back.
//
// The console is drawn only while the monitor waits for the host, between
// the bytes it reads, so that drawing never holds up a transfer. While a
// program runs the screen is the program's, and nothing is drawn; what the
// program prints still goes into the console, for when it returns.

// The console's size and place: inside a CRT's overscan, in 8 by 8 cells.
const (
	conCols    = 36
	conLogRows = 24
	conX       = 16 // the left of the text
	conY       = 12 // the top of the first status line
	conLogY    = conY + 22
	conBar     = conY + 18 // the line under the status lines, an upload's progress bar
	conHeight  = 240
)

// Times, in lines of the display, which the monitor's clock counts: about
// 15,734 a second.
const (
	lineRate = 15734
	// idleLines is how long before the logo: 30 seconds for now, while it
	// is tried out on a console, and 5 minutes after.
	idleLines   = 30 * lineRate
	shiftLines  = 60 * lineRate // between moves of the console
	logoLines   = 2 * 263       // between steps of the logo, every other frame
	tenthLines  = lineRate / 10
	statusLines = lineRate / 4 // between redraws of the status during an upload
)

// shifts are where the console is drawn from, in turn, a minute each.
var shifts = [4][2]int{{0, 0}, {2, 0}, {2, 2}, {0, 2}}

// The monitor's states, which the second status line names.
const (
	stateReady = iota
	stateReceive
	stateSend
	stateRun
	stateRDB
)

// upload is a transfer the status line follows.
type upload struct {
	addr, size, done uint32
	start            uint32 // the clock when it began
	retries          uint32
}

// console is the screen's text and state.
type console struct {
	// kept is set, which puts the console in .data rather than .bss, so
	// that it lives through the monitor starting again after a program
	// returns.
	kept bool

	// started is set once the monitor has started, so that it can tell
	// starting again after a program from its first start.
	started bool

	text [conLogRows][conCols]byte // the lines, a ring: text[last] is the newest
	last int
	col  int

	binary bool // a transfer's bytes are going by, which aren't text

	scrolls int              // lines scrolled since the log was drawn
	dirty   [conLogRows]bool // rows of text changed since
	full    bool             // everything to be drawn again
	status  bool             // the status lines to be drawn again

	state    int
	up       upload // the transfer under way
	session  upload // the transfers of one bwr or brd, all told
	lastUp   upload // the last session, done counting its bytes and start how long it took
	upDone   bool
	overruns uint32 // bytes the serial port lost

	clock    uint32 // lines of the display since the monitor started counting
	raw      uint16 // the counter as last read
	active   uint32 // the clock at the last byte either way
	shiftAt  uint32
	shift    int
	statusAt uint32

	saver bool
	logo  logo

	// held keeps the screen as a command left it, such as cb's color
	// bars, until the host sends a byte, when the console comes back, with
	// the display set up again (remode).
	held, remode bool
}

var con = console{kept: true, full: true, status: true}

// activity is set by the serial port when a byte goes either way, and
// received when one comes from the host, and taken by the console when it
// next looks.
var activity, received bool

// hold keeps the screen as it is until the host sends a byte.
func (c *console) hold() {
	c.held = true
	received = false
}

// put takes a byte the monitor sends the host, or a program prints.
//
//go:noinline
func (c *console) put(b byte) {
	if c.binary {
		return
	}
	switch {
	case b == '\r':
		c.col = 0
	case b == '\n':
		c.newline()
	case b == '\b':
		if c.col > 0 {
			c.col--
		}
	case b == '\t':
		c.char(' ')
		for c.col%8 != 0 && c.col < conCols {
			c.char(' ')
		}
	case b < ' ':
	case b > '~':
		c.char('?')
	default:
		c.char(b)
	}
}

// char puts a printable character where the line is. One past the end of
// the line goes on the next, unless it's a space: the monitor's prompt
// clears the line with more spaces than the console is wide, and the
// spaces at the end of a line show nothing anyway.
func (c *console) char(b byte) {
	if c.col >= conCols {
		if b == ' ' {
			return
		}
		c.newline()
	}
	if c.text[c.last][c.col] != b {
		c.text[c.last][c.col] = b
		c.dirty[c.row(c.last)] = true
	}
	c.col++
}

// newline starts a line, scrolling the log up.
func (c *console) newline() {
	c.last = (c.last + 1) % conLogRows
	c.text[c.last] = [conCols]byte{}
	c.col = 0
	c.scrolls++
}

// row is where text line i is on the screen, 0 at the top of the log.
func (c *console) row(i int) int {
	return (i - c.last - 1 + 2*conLogRows) % conLogRows
}

// restarted marks the monitor starting again, after a program returned or
// the debugger ended it, with a line across the log, and reports whether it
// had started before.
func (c *console) restarted() bool {
	if !c.started {
		c.started = true
		return false
	}
	if c.col > 0 {
		c.newline()
	}
	for i := range len(divider) {
		c.char(divider[i])
	}
	c.newline()
	return true
}

// divider is the line between a program and the monitor starting again.
const divider = "---------- program ended ---------"

// line returns the text on screen row r of the log.
func (c *console) line(r int) []byte {
	return c.text[(c.last+1+r)%conLogRows][:]
}

// setState changes what the status line says.
func (c *console) setState(s int) {
	c.state = s
	c.status = true
}

// startUpload begins following a transfer of size bytes at addr, the
// first of a session or another in it.
func (c *console) startUpload(state int, addr, size uint32) {
	if c.state != stateReceive && c.state != stateSend {
		c.session = upload{start: c.clock}
	}
	c.binary = true
	c.up = upload{addr: addr, size: size, start: c.clock}
	c.setState(state)
}

// block counts n bytes of a transfer as done.
func (c *console) block(n uint32) {
	c.up.done += n
	c.session.done += n
	c.status = true
}

// retry counts a block sent again.
func (c *console) retry() {
	c.up.retries++
	c.status = true
}

// endUpload ends a session of transfers.
func (c *console) endUpload() {
	c.binary = false
	if c.state == stateReceive || c.state == stateSend {
		c.lastUp = c.session
		c.lastUp.start = c.clock - c.session.start // how long it took
		c.upDone = true
	}
	c.setState(stateReady)
}

// tick advances the clock by the counter's reading, a 16-bit count of the
// display's lines, which it has to be given within 4 seconds of the last.
func (c *console) tick(raw uint16) {
	c.clock += uint32(raw - c.raw)
	c.raw = raw
	if activity {
		activity = false
		c.active = c.clock
		if c.saver {
			c.saver = false
			c.full = true
		}
	}
	if received {
		received = false
		if c.held {
			c.held = false
			c.full, c.remode = true, true
		}
	}
	switch {
	case c.saver:
	case c.clock-c.active >= idleLines:
		c.saver = true
		c.logo.start(c.clock)
	case c.clock-c.shiftAt >= shiftLines:
		c.shiftAt = c.clock
		c.shift = (c.shift + 1) % len(shifts)
		c.full = true
	}
}

// origin is where the console is drawn from, this minute.
func (c *console) origin() (x, y int) {
	s := shifts[c.shift]
	return s[0], s[1]
}

// statusText writes the two status lines into a and b, space-padded.
//
//go:noinline
func (c *console) statusText(a, b *[conCols]byte) {
	*a = [conCols]byte{}
	*b = [conCols]byte{}
	put(a[:], 0, "CLEAN MONITOR")
	putDecimal(a[:], conCols-4-digits10(line.baud), line.baud)
	put(a[:], conCols-4, " bps")
	switch c.state {
	case stateReady:
		at := put(b[:], 0, "ready")
		if c.upDone {
			u := c.lastUp
			at = put(b[:], at, ", loaded ")
			at = putSize(b[:], at, u.done)
			at = put(b[:], at, " in ")
			at = putTenths(b[:], at, u.start)
			at = put(b[:], at, "s")
		}
		if c.overruns > 0 {
			at = put(b[:], at, ", lost ")
			putDecimal(b[:], at, c.overruns)
		}
	case stateReceive, stateSend:
		at := put(b[:], 0, "receiving ")
		if c.state == stateSend {
			at = put(b[:], 0, "sending ")
		}
		at = putHex(b[:], at, c.up.addr)
		at = put(b[:], at, " ")
		at = putSize(b[:], at, c.up.done)
		at = put(b[:], at, "/")
		at = putSize(b[:], at, c.up.size)
		if c.up.retries > 0 {
			at = put(b[:], at, " ")
			at = putDecimal(b[:], at, c.up.retries)
			put(b[:], at, " again")
		}
	case stateRun:
		at := put(b[:], 0, "running ")
		putHex(b[:], at, regs[regEPC])
	case stateRDB:
		put(b[:], 0, "remote debugger")
	}
}

// progress is the share of a transfer done, out of w.
func (c *console) progress(w int) int {
	if c.up.size == 0 {
		return 0
	}
	return int(uint64(min(c.up.done, c.up.size)) * uint64(w) / uint64(c.up.size))
}

// The console writes numbers straight into its lines, and takes nothing
// from the monitor's heap, which is never collected.

// put writes s into line from column at, as much as fits, and returns the
// column after it.
//
//go:noinline
func put(line []byte, at int, s string) int {
	for i := range len(s) {
		if at >= len(line) {
			break
		}
		line[at] = s[i]
		at++
	}
	return at
}

// digits10 is how many decimal digits v takes.
func digits10(v uint32) int {
	n := 1
	for v >= 10 {
		v /= 10
		n++
	}
	return n
}

// putDecimal writes v in decimal into line from column at, as much as
// fits, and returns the column after it.
//
//go:noinline
func putDecimal(line []byte, at int, v uint32) int {
	n := digits10(v)
	for i := n - 1; i >= 0; i-- {
		if at+i < len(line) {
			line[at+i] = byte('0' + v%10)
		}
		v /= 10
	}
	return min(at+n, len(line))
}

//
//go:noinline
func putHex(line []byte, at int, v uint32) int {
	for i := 7; i >= 0; i-- {
		if at < len(line) {
			line[at] = "0123456789abcdef"[v>>(4*i)&15]
		}
		at++
	}
	return min(at, len(line))
}

// putSize writes a size in bytes, or kilobytes from 10 KB on.
func putSize(line []byte, at int, v uint32) int {
	if v >= 10*1024 {
		at = putDecimal(line, at, v/1024)
		return put(line, at, "K")
	}
	return putDecimal(line, at, v)
}

// putTenths writes a time in lines of the display as seconds, to a tenth.
func putTenths(line []byte, at int, lines uint32) int {
	t := lines / tenthLines
	at = putDecimal(line, at, t/10)
	at = put(line, at, ".")
	return putDecimal(line, at, t%10)
}

// logo is the screen saver: a mark that moves across the screen, a pixel a
// step, and bounces off the edges, taking another color at each bounce.
type logo struct {
	x, y, dx, dy int
	color        int
	at           uint32 // the clock at the last step
	old          [2]int // where it was drawn, to be cleared
}

// The logo's size: PSXSDK in letters of 3 by 3 dots, over a flattened
// disc as wide with MONITOR in it.
const (
	logoWord = "PSXSDK"
	logoW    = len(logoWord)*6*3 - 3
	logoText = 7 * 3
	logoGap  = 3
	logoDisc = 13
	logoH    = logoText + logoGap + logoDisc

	// logoMargin keeps the logo inside a CRT's overscan.
	logoMargin = 8
)

// logoColors are the logo's colors, dim, as GP0 takes them: blue in the
// high byte.
var logoColors = [6]uint32{0x703070, 0x606020, 0x105070, 0x306830, 0x784030, 0x403070}

func (l *logo) start(clock uint32) {
	l.x, l.y = (320-logoW)/2, (conHeight-logoH)/2
	l.dx, l.dy = 1, 1
	l.at = clock
}

// step moves the logo as many steps as the clock says are due, and reports
// whether it moved.
func (l *logo) step(clock uint32) bool {
	moved := false
	for clock-l.at >= logoLines {
		l.at += logoLines
		l.x += l.dx
		l.y += l.dy
		bounced := false
		if l.x <= logoMargin || l.x >= 320-logoMargin-logoW {
			l.dx = -l.dx
			bounced = true
		}
		if l.y <= logoMargin || l.y >= conHeight-logoMargin-logoH {
			l.dy = -l.dy
			bounced = true
		}
		if bounced {
			l.color = (l.color + 1) % len(logoColors)
		}
		moved = true
	}
	return moved
}
