package emu

import "fmt"

// CD-ROM commands.
const (
	cdGetstat    = 0x01
	cdSetloc     = 0x02
	cdPlay       = 0x03
	cdForward    = 0x04
	cdBackward   = 0x05
	cdReadN      = 0x06
	cdMotorOn    = 0x07
	cdStop       = 0x08
	cdPause      = 0x09
	cdInit       = 0x0a
	cdMute       = 0x0b
	cdDemute     = 0x0c
	cdSetfilter  = 0x0d
	cdSetmode    = 0x0e
	cdGetparam   = 0x0f
	cdGetlocL    = 0x10
	cdGetlocP    = 0x11
	cdSetSession = 0x12
	cdGetTN      = 0x13
	cdGetTD      = 0x14
	cdSeekL      = 0x15
	cdSeekP      = 0x16
	cdTest       = 0x19
	cdGetID      = 0x1a
	cdReadS      = 0x1b
	cdReset      = 0x1c
	cdGetQ       = 0x1d
	cdReadTOC    = 0x1e
	cdVideoCD    = 0x1f
)

// Status bits.
const (
	cdStatPlay      = 0x80
	cdStatRead      = 0x20
	cdStatShellOpen = 0x10
	cdStatIDError   = 0x08
	cdStatSeekError = 0x04
	cdStatSpindle   = 0x02
	cdStatError     = 0x01
)

// Error codes.
const (
	cdErrSeekFailed  = 0x04
	cdErrInvalidSub  = 0x10
	cdErrWrongParams = 0x20
	cdErrInvalidCmd  = 0x40
	cdErrNoDisc      = 0x80
)

// Mode bits.
const (
	cdModeSectorSize = 0x20
	cdModeXAADPCM    = 0x40
	cdModeSpeed      = 0x80
)

// Delays, in CPU cycles.
const (
	cdDelay1ms      = 33869
	cdDelayFirst    = 50401 // to the first response
	cdDelayInit     = 81102 // to Init's first response
	cdDelayReadSS   = 33868800 / 75
	cdDelayReadDS   = 33868800 / (2 * 75)
	cdVersionC0A    = 1
	cdRegionAmerica = 2
)

// Drive states.
const (
	cdIdle = iota
	cdResp1
	cdResp2
	cdReading
	cdPlaying
)

// cdQueue is one of the drive's FIFOs. psxe's do not wrap: they fill up to
// their size, drop what does not fit, and start again from the beginning
// once emptied.
type cdQueue struct {
	buf  []byte
	r, w int
}

func newCDQueue(size int) *cdQueue { return &cdQueue{buf: make([]byte, size)} }

func (q *cdQueue) empty() bool { return q.r == q.w }
func (q *cdQueue) full() bool  { return q.w == len(q.buf) }
func (q *cdQueue) size() int   { return q.w - q.r }
func (q *cdQueue) reset()      { q.r, q.w = 0, 0 }

func (q *cdQueue) push(v byte) {
	if !q.full() {
		q.buf[q.w] = v
		q.w++
	}
}

func (q *cdQueue) pop() byte {
	if q.empty() {
		return 0
	}
	v := q.buf[q.r]
	q.r++
	if q.empty() {
		q.reset()
	}
	return v
}

// clear empties the queue and zeroes what it held.
func (q *cdQueue) clear() {
	clear(q.buf[:q.w])
	q.reset()
}

// cdrom is the CD-ROM controller at 1F801800, ported from psxe: four
// registers whose meaning depends on an index, a command that answers after
// a delay with one or two responses, each raising an interrupt, and reads
// that deliver a sector at a time into the data FIFO.
//
// CD audio, CD-DA and XA-ADPCM, is in cdaudio.go. With audioOn off, as for
// tracing, the drive does what psxe does while tracing: Play plays nothing,
// and XA sectors are read as data.
type cdrom struct {
	disc     *Disc
	discKind int
	ic       *ic
	stop     func(error)

	index      uint8
	ier, ifr   uint8
	volPending [4]uint8
	vol        [4]uint8
	xaMute     bool
	mode       uint8
	dataReq    bool

	data       *cdQueue
	response   *cdQueue
	parameters *cdQueue

	pendingCommand uint8
	busy           bool
	state          int
	prevState      int
	delay          int

	// psxe counts the delay down every step, but nothing sees it until it
	// has run out, so the drive is only brought up to date then: cycles is
	// the machine's cycle count, synced the count the delay was last
	// counted down to, and due the count at which update next has
	// something to do.
	cycles *uint64
	synced uint64
	due    uint64
	wake   *bool // set when due changes

	pendingLBA, lba   uint32
	speedSwitchDelay  int
	seekPrecision     bool
	mute              bool
	xaFile, xaChannel uint8
	xaPlaying         bool
	cddaPlaying       bool
	readOngoing       bool

	audioOn bool
	audio   cdAudio
}

// newCDROM returns a drive with nothing in it, counting cycles from the
// given clock, setting wake when it changes what it has due, and reporting
// an error that stops the run to stop.
func newCDROM(ic *ic, cycles *uint64, wake *bool, stop func(error)) *cdrom {
	c := &cdrom{
		ic:            ic,
		stop:          stop,
		cycles:        cycles,
		due:           ^uint64(0),
		wake:          wake,
		data:          newCDQueue(sectorSize),
		response:      newCDQueue(32),
		parameters:    newCDQueue(32),
		vol:           [4]uint8{0x80, 0, 0x80, 0},
		pendingLBA:    150,
		lba:           150,
		seekPrecision: true,
	}
	c.audio.xa.sixStep = 6
	return c
}

func (c *cdrom) insert(d *Disc) {
	c.disc = d
	c.discKind = d.kind()
}

func (c *cdrom) stat() uint8 {
	var s uint8 = cdStatSpindle
	if c.cddaPlaying {
		s |= cdStatPlay
	}
	if c.readOngoing {
		s |= cdStatRead
	}
	if c.mode&0x10 != 0 {
		s |= cdStatIDError
	}
	if c.disc == nil {
		s |= cdStatShellOpen
	}
	return s
}

func (c *cdrom) respond(irq uint8, values ...uint8) {
	c.ifr = irq
	for _, v := range values {
		c.response.push(v)
	}
}

func (c *cdrom) fail(stat, code uint8) {
	c.ifr = 5
	c.parameters.reset()
	c.response.reset()
	if stat&(cdStatIDError|cdStatSeekError) != 0 {
		c.response.push(stat)
	} else {
		c.response.push(cdStatError | stat)
	}
	c.response.push(code)
	c.prevState = cdIdle
	c.state = cdIdle
	c.pendingCommand = 0
	c.busy = false
}

func (c *cdrom) processSetloc() {
	if c.pendingLBA == 0 {
		return
	}
	c.lba = c.pendingLBA
	c.pendingLBA = 0
}

func (c *cdrom) readDelay() int {
	if c.mode&cdModeSpeed != 0 {
		return cdDelayReadDS
	}
	return cdDelayReadSS
}

func (c *cdrom) pauseDelay() int {
	if !c.readOngoing {
		return 7000
	}
	if c.mode&cdModeSpeed != 0 {
		return cdDelay1ms * 35
	}
	return cdDelay1ms * 70
}

func (c *cdrom) seekDelay(kind int) int {
	d := c.speedSwitchDelay
	c.speedSwitchDelay = 0
	switch kind {
	case sectorFar:
		d = 650 * cdDelay1ms
	case sectorAudio:
		d = 4000 * cdDelay1ms
	}
	return d
}

// Register access. Only 8-bit accesses mean anything; psxe asserts on the
// others.

func (c *cdrom) Read32(uint32) uint32 {
	c.stop(fmt.Errorf("32-bit CD-ROM read, which psxe asserts on"))
	return 0
}

func (c *cdrom) Read16(uint32) uint16 {
	c.stop(fmt.Errorf("16-bit CD-ROM read, which psxe asserts on"))
	return 0
}

func (c *cdrom) Write32(uint32, uint32) {
	c.stop(fmt.Errorf("32-bit CD-ROM write, which psxe asserts on"))
}

func (c *cdrom) Write16(uint32, uint16) {
	c.stop(fmt.Errorf("16-bit CD-ROM write, which psxe asserts on"))
}

func (c *cdrom) Read8(off uint32) uint8 {
	switch off {
	case 0:
		return c.status()
	case 1:
		return c.response.pop()
	case 2:
		if !c.dataReq {
			return 0
		}
		return c.data.pop()
	case 3:
		if c.index&1 != 0 {
			return 0xe0 | c.ifr
		}
		return c.ier
	}
	return 0
}

func (c *cdrom) status() uint8 {
	dataEmpty := c.data.empty() || !c.dataReq
	return c.index |
		uint8(b2i(c.xaPlaying))<<2 |
		uint8(b2i(c.parameters.empty()))<<3 |
		uint8(b2i(!c.parameters.full()))<<4 |
		uint8(b2i(!c.response.empty()))<<5 |
		uint8(b2i(!dataEmpty))<<6 |
		uint8(b2i(c.busy))<<7
}

func (c *cdrom) Write8(off uint32, v uint8) {
	// A command replaces the delay, so what has run off it so far is
	// counted first; and the rest of the writes leave it counting.
	c.sync()
	defer c.schedule()
	switch uint32(c.index)<<2 | off {
	case 0, 4, 8, 12:
		c.index = v & 3
	case 1:
		c.command(v)
	case 2:
		c.parameters.push(v)
	case 3:
		c.dataReq = v&0x80 != 0
	case 6:
		c.ier = v
	case 7:
		if v&0x40 != 0 {
			c.parameters.clear()
		}
		c.ifr &^= v & 0x1f
	case 10, 11:
		c.volPending[off-2] = v
	case 13, 14:
		c.volPending[off+1] = v
	case 15:
		c.xaMute = v&1 != 0
		c.vol = c.volPending
	}
}

// command starts a command. Its first response comes after a delay.
func (c *cdrom) command(cmd uint8) {
	c.prevState = c.state
	c.state = cdResp1
	c.pendingCommand = cmd
	if cmd == cdInit {
		c.delay = cdDelayInit
	} else {
		c.delay = cdDelayFirst
	}
}

// sync counts the cycles since the drive was last brought up to date off
// the delay. They cannot have run it out, or update would have been called
// at the step they did.
func (c *cdrom) sync() {
	n := *c.cycles - c.synced
	c.synced = *c.cycles
	if c.delay > 0 {
		c.delay -= int(n)
	}
}

// schedule works out when update next has something to do: when the delay
// runs out, or straight away with none and a response or sector to
// deliver. Idle, or playing, which the drive does on its own, it has
// nothing to do until a command.
func (c *cdrom) schedule() {
	*c.wake = true
	switch {
	case c.delay > 0:
		c.due = c.synced + uint64(c.delay)
	case c.state == cdIdle || c.state == cdPlaying:
		c.due = ^uint64(0)
	default:
		c.due = c.synced
	}
}

// update advances the drive by the cycles since it was last brought up to
// date, which the machine calls for when they reach due. psxe advances it
// every step; counting a step's cycles off the delay at a time comes to
// the same as counting them all off at once, so the drive acts on the
// step psxe would.
func (c *cdrom) update() {
	n := *c.cycles - c.synced
	c.synced = *c.cycles
	if c.delay > int(n) {
		c.delay -= int(n)
	} else if c.delay|c.state != 0 {
		c.advance(n)
	}
	c.schedule()
}

func (c *cdrom) advance(cycles uint64) {
	if c.delay > 0 {
		c.delay -= int(cycles)
		if c.delay > 0 {
			return
		}
	}
	c.delay = 0

	if c.state == cdIdle || c.state == cdPlaying {
		return
	}
	// Nothing more is delivered until the last interrupt is acknowledged.
	if c.ifr&0x1f != 0 {
		c.delay = 2
		return
	}

	switch c.state {
	case cdResp1:
		c.firstResponse()
		c.enterRead()
	case cdResp2:
		c.execute()
		c.enterRead()
	case cdReading:
		c.readSector()
	}

	if c.ifr&c.ier != 0 {
		c.ic.raise(irqCDROM)
	}
}

// enterRead sets the delay to the first sector once a command has started
// a read. It replaces the delay the command set, which included any seek.
func (c *cdrom) enterRead() {
	if c.state != cdReading {
		return
	}
	c.processSetloc()
	c.state = cdReading
	c.prevState = cdReading
	c.delay = c.readDelay() + cdDelay1ms*4
}

// noDisc is the commands that answer "no disc" when there is none.
var noDisc = map[uint8]bool{
	cdSetloc: true, cdPlay: true, cdForward: true, cdBackward: true,
	cdReadN: true, cdMotorOn: true, cdStop: true, cdPause: true,
	cdMute: true, cdDemute: true, cdSetfilter: true, cdGetlocL: true,
	cdGetlocP: true, cdSetSession: true, cdGetTN: true, cdGetTD: true,
	cdSeekL: true, cdSeekP: true, cdGetID: true, cdReadS: true, cdGetQ: true,
}

// paramCount is how many parameters each valid command takes; -1 for Play,
// which takes none or one. Anything missing is an invalid command.
var paramCount = map[uint8]int{
	cdGetstat: 0, cdForward: 0, cdBackward: 0, cdReadN: 0, cdMotorOn: 0,
	cdStop: 0, cdPause: 0, cdInit: 0, cdMute: 0, cdDemute: 0, cdGetparam: 0,
	cdGetlocL: 0, cdGetlocP: 0, cdGetTN: 0, cdSeekL: 0, cdSeekP: 0, cdGetID: 0,
	cdReadS: 0, cdReset: 0, cdReadTOC: 0,
	cdSetloc: 3, cdPlay: -1, cdSetfilter: 2, cdGetQ: 2,
	cdSetmode: 1, cdGetTD: 1, cdTest: 1, cdVideoCD: 6,
	// SetSession is in psxe's command table but not its list of valid
	// commands, so it is rejected as invalid before it runs.
}

// firstResponse checks a command and runs it.
func (c *cdrom) firstResponse() {
	c.busy = false
	cmd := c.pendingCommand

	if c.disc == nil && noDisc[cmd] {
		c.fail(cdStatShellOpen, cdErrNoDisc)
		return
	}
	// The C0A firmware psxe emulates lacks GetQ and ReadTOC.
	if cmd == cdGetQ || cmd == cdReadTOC {
		c.fail(cdStatSpindle, cdErrInvalidCmd)
		return
	}
	n, ok := paramCount[cmd]
	switch {
	case !ok:
		c.fail(cdStatSpindle, cdErrInvalidCmd)
		return
	case n == -1 && c.parameters.size() > 1, n >= 0 && c.parameters.size() != n:
		c.fail(cdStatSpindle, cdErrWrongParams)
		return
	}
	c.execute()
}

// restore returns to reading or playing after a command, if that is what
// the drive was doing before it.
func (c *cdrom) restore() {
	c.state = cdIdle
	if c.prevState == cdPlaying || c.prevState == cdReading {
		c.state = c.prevState
	}
	c.pendingCommand = 0
}

func (c *cdrom) pause() {
	c.prevState = cdIdle
	c.state = cdIdle
	c.pendingCommand = 0
	c.busy = false
	c.cddaPlaying = false
	c.xaPlaying = false
	c.readOngoing = false
}

// twoPart runs the first half of a command that answers twice: the first
// response now, the second after delay. It reports whether this was the
// first half.
func (c *cdrom) twoPart(delay int) bool {
	if c.state != cdResp1 {
		return false
	}
	c.respond(3, c.stat())
	c.delay = delay
	c.state = cdResp2
	return true
}

func (c *cdrom) execute() {
	switch c.pendingCommand {
	case cdGetstat:
		c.respond(3, c.stat())
		c.restore()
	case cdSetloc:
		m, s, f := c.parameters.pop(), c.parameters.pop(), c.parameters.pop()
		if !validMSF(m, s, f) {
			c.fail(cdStatSpindle, cdErrInvalidSub)
			return
		}
		c.respond(3, c.stat())
		c.pendingLBA = btoi(m)*4500 + btoi(s)*75 + btoi(f)
		c.restore()
	case cdPlay:
		c.respond(3, c.stat())
		c.processSetloc()
		if !c.parameters.empty() {
			if track := btoi(c.parameters.pop()); track != 0 {
				c.lba = c.disc.trackLBA(int(track))
			}
		}
		c.prevState = cdPlaying
		c.state = cdPlaying
		c.pendingCommand = 0
		c.cddaPlaying = true
		if c.audioOn {
			c.playStart()
		}
		c.restore()
	case cdForward:
		c.respond(3, c.stat())
		c.lba += 35
		c.restore()
	case cdBackward:
		c.respond(3, c.stat())
		c.lba -= 35
		if c.lba < 150 {
			c.lba = 150
		}
		c.restore()
	case cdReadN, cdReadS:
		c.respond(3, c.stat())
		c.processSetloc()
		kind := c.disc.read(c.lba, c.data.buf)
		if c.mode&cdModeXAADPCM != 0 {
			c.xaPlaying = true
		}
		c.state = cdReading
		c.prevState = cdReading
		c.delay = c.readDelay() + c.seekDelay(kind)
		c.readOngoing = true
	case cdMotorOn:
		if !c.twoPart(cdDelayFirst) {
			c.respond(2, c.stat())
			c.restore()
		}
	case cdStop:
		if !c.twoPart(cdDelayFirst) {
			c.respond(2, c.stat())
			c.pendingLBA = 150
			c.processSetloc()
			c.pause()
		}
	case cdPause:
		if !c.twoPart(c.pauseDelay()) {
			c.respond(2, cdStatSpindle)
			c.pause()
		}
	case cdInit:
		if !c.twoPart(cdDelay1ms) {
			c.respond(2, c.stat())
			c.pause()
		}
	case cdMute:
		c.respond(3, c.stat())
		c.mute = true
		c.restore()
	case cdDemute:
		c.respond(3, c.stat())
		c.mute = false
		c.restore()
	case cdSetfilter:
		c.respond(3, c.stat())
		c.xaFile = c.parameters.pop()
		c.xaChannel = c.parameters.pop()
		c.restore()
	case cdSetmode:
		c.respond(3, c.stat())
		prev := c.mode & cdModeSpeed
		c.mode = c.parameters.pop()
		if prev != c.mode&cdModeSpeed {
			c.speedSwitchDelay = 650 * cdDelay1ms
		}
		c.pause()
	case cdGetparam:
		c.respond(3, c.stat(), c.mode, 0, c.xaFile, c.xaChannel)
		c.restore()
	case cdGetlocL:
		// The header of the last sector read.
		c.respond(3, c.data.buf[0x0c:0x14]...)
		c.restore()
	case cdGetlocP:
		c.getlocP()
	case cdGetTN:
		c.respond(3, c.stat(), 1, itob(len(c.disc.tracks)))
		c.restore()
	case cdGetTD:
		bcd := c.parameters.pop()
		if !validBCD(bcd) {
			c.fail(cdStatSpindle, cdErrInvalidSub)
			return
		}
		f := c.disc.trackLBA(int(btoi(bcd)))
		if f == sectorFar {
			c.fail(cdStatSpindle, cdErrInvalidSub)
			return
		}
		c.respond(3, c.stat(), itob(int(f/4500)), itob(int(f%4500/75)))
		c.restore()
	case cdSeekL:
		if c.twoPart(cdDelay1ms) {
			return
		}
		switch c.disc.query(c.pendingLBA) {
		case sectorFar:
			c.fail(cdStatSpindle|cdStatSeekError, cdErrInvalidSub)
			return
		case sectorAudio:
			c.fail(cdStatSpindle|cdStatSeekError, cdErrSeekFailed)
			return
		}
		c.processSetloc()
		c.seekPrecision = true
		c.respond(2, c.stat())
		c.restore()
	case cdSeekP:
		if c.twoPart(cdDelay1ms) {
			c.busy = true
			return
		}
		c.respond(2, c.stat())
		c.processSetloc()
		c.seekPrecision = false
		c.restore()
	case cdTest:
		c.test(c.parameters.pop())
	case cdGetID:
		if c.twoPart(cdDelayFirst) {
			return
		}
		id := [4][4]uint8{
			{0x08, 0x40, 0x00, 0x00}, // no disc
			{0x02, 0x00, 0x20, 0x00}, // licensed
			{0x0a, 0x90, 0x00, 0x00}, // audio
			{0x0a, 0x80, 0x00, 0x00}, // unlicensed
		}[c.discKind]
		c.respond(2, id[:]...)
		if c.discKind == discLicensed {
			c.respond(2, 'S', 'C', 'E', "IEA"[cdRegionAmerica])
		} else {
			c.respond(2, 0, 0, 0, 0)
		}
		c.restore()
	case cdReset, cdGetQ, cdVideoCD:
		// psxe does nothing for these, not even respond, so the drive
		// runs them again on every step until another command arrives.
	}
}

func (c *cdrom) getlocP() {
	lba := int32(c.lba)
	track := c.disc.trackNumber(uint32(lba))
	trackLBA := int32(c.disc.trackLBA(track))
	if !c.seekPrecision {
		lba -= 25
	}
	diff := lba - trackLBA
	if diff < 0 {
		diff = -diff
	}
	index := 0
	if c.disc.query(uint32(lba)) != sectorPregap {
		index = 1
	}
	c.respond(3,
		itob(track), itob(index),
		itob(int(diff/4500)), itob(int(diff%4500/75)), itob(int(diff%4500%75)),
		itob(int(lba/4500)), itob(int(lba%4500/75)), itob(int(lba%4500%75)),
	)
	c.restore()
}

func (c *cdrom) test(sub uint8) {
	switch sub {
	case 0x04:
		c.respond(3, cdStatSpindle)
	case 0x05:
		c.respond(3, 0, 0)
	case 0x20:
		// The firmware version and date: C0A, 19 September 1994.
		c.respond(3, 0x94, 0x09, 0x19, 0xc0)
	default:
		c.fail(cdStatSpindle, cdErrInvalidSub)
		return
	}
	c.restore()
}

// readSector delivers the next sector of a read into the data FIFO, which
// the program then drains through the data register or by DMA.
func (c *cdrom) readSector() {
	c.processSetloc()
	switch c.disc.query(c.lba) {
	case sectorFar:
		c.fail(cdStatSpindle|cdStatSeekError, cdErrInvalidSub)
		return
	case sectorAudio:
		c.fail(cdStatSpindle|cdStatSeekError, cdErrSeekFailed)
		return
	}
	if c.audioOn {
		// An XA-ADPCM sector goes to the decoder, or is dropped by the
		// filter, rather than to the CPU, which may still be reading the
		// last sector out of the FIFO.
		var sector [sectorSize]byte
		c.disc.read(c.lba, sector[:])
		if c.xaSector(sector[:]) {
			c.pendingLBA = c.lba + 1
			c.delay = c.readDelay()
			return
		}
		copy(c.data.buf, sector[:])
	} else {
		c.disc.read(c.lba, c.data.buf)
	}
	c.respond(1, c.stat())
	// The FIFO holds the data part of the sector, or with the sector size
	// bit set, everything after the sync pattern.
	if c.mode&cdModeSectorSize != 0 {
		c.data.r, c.data.w = 12, 12+0x924
	} else {
		c.data.r, c.data.w = 24, 24+0x800
	}
	c.pendingLBA = c.lba + 1
	c.delay = c.readDelay()
}

// btoi converts a BCD byte to binary. psxe uses a table that does the same
// sum for invalid digits.
func btoi(b uint8) uint32 {
	return uint32(b>>4)*10 + uint32(b&0xf)
}

// itob converts to BCD. psxe's table covers up to 255, with the tens digit
// running past 9 to 15 and then starting again.
func itob(i int) uint8 {
	i = i & 0xff % 160
	return uint8(i/10<<4 | i%10)
}

func validBCD(b uint8) bool {
	return b&0xf0 <= 0x90 && b&0xf <= 9
}

func validMSF(m, s, f uint8) bool {
	return validBCD(m) && validBCD(s) && validBCD(f) && f < 0x75 && s < 0x60
}
