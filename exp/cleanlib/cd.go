//go:build yaroze

package cleanlib

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The CD-ROM drive: commands, and reads that go on in the background,
// driven by the drive's interrupt.

// The drive's registers. What the three after the first mean depends on
// the index written to the first, whose read is the drive's status.
var (
	cdIndex = (*volatile.Register8)(unsafe.Pointer(uintptr(0x1f801800)))
	cdReg1  = (*volatile.Register8)(unsafe.Pointer(uintptr(0x1f801801)))
	cdReg2  = (*volatile.Register8)(unsafe.Pointer(uintptr(0x1f801802)))
	cdReg3  = (*volatile.Register8)(unsafe.Pointer(uintptr(0x1f801803)))

	dmaCDAddr    = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010b0)))
	dmaCDBlock   = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010b4)))
	dmaCDControl = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010b8)))
	dmaInterrupt = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010f4)))

	// The memory controller's delay and bus size for the drive, and its
	// common delay, which a sector's transfer changes and puts back.
	cdDelaySize = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f801018)))
	comDelay    = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f801020)))

	// The SPU's main volume, its CD audio volume and its control
	// register, which the drive's sound goes through.
	spuMainLeft  = (*volatile.Register16)(unsafe.Pointer(uintptr(0x1f801d80)))
	spuMainRight = (*volatile.Register16)(unsafe.Pointer(uintptr(0x1f801d82)))
	spuCDLeft    = (*volatile.Register16)(unsafe.Pointer(uintptr(0x1f801db0)))
	spuCDRight   = (*volatile.Register16)(unsafe.Pointer(uintptr(0x1f801db2)))
	spuControl   = (*volatile.Register16)(unsafe.Pointer(uintptr(0x1f801daa)))
)

const (
	cdStatusResponse = 1 << 5 // the response FIFO holds bytes
	cdStatusData     = 1 << 6 // the data FIFO holds bytes
	cdStatusBusy     = 1 << 7 // a command is being sent

	dmaCDEnable = 1 << 15    // DPCR: channel 3 enabled
	dmaFromCD   = 0x11000000 // start, at once, into memory

	// DICR: channel 3's interrupt enable, and its flag, written as 1 to
	// acknowledge it.
	dicrCDEnable = 1 << 19
	dicrCDFlag   = 1 << 27

	// The drive's commands the library sends itself.
	cdlGetstat = 0x01
	cdlSetloc  = 0x02
	cdlPlay    = 0x03
	cdlReadN   = 0x06
	cdlPause   = 0x09
	cdlInit    = 0x0a
	cdlDemute  = 0x0c
	cdlSetmode = 0x0e
	cdlReadS   = 0x1b

	// The values the memory controller's registers take: the drive's
	// delay and size, and the common delay during a sector's transfer and
	// after it.
	cdDelayValue = 0x00020943
	comDelayDMA  = 0x1323
	comDelayIdle = 0x1325

	// pauseWait is how many horizontal lines the handler waits for the
	// first answer to the Pause it sends after a read, about a sixtieth
	// of a second.
	pauseWait = 263

	// What Sony's library spends, in cycles, beyond the work the clean
	// library does the same way, measured on readsync against Sony's
	// (spec 08, "Waiting on the drive"). Its own state and calls cost it
	// more than the clean library's.
	//
	// watchBegin and watchCheck: from the time read to the stores, in a
	// watch's begin (50 cycles in Sony's) and check (56).
	watchBegin = 24
	watchCheck = 36
	// sendEntry: from CdControl's call to its watch (194 cycles);
	// sendRest: from the watch's check to the command (148 for ReadN,
	// 168 for Setmode, 274 for Setloc); controlReturn: from the last
	// check of the wait to the return (136).
	sendEntry     = 98
	sendRest      = 60
	controlReturn = 102
	// CdReadSync: its time reads 76 and 110 cycles apart, and its return
	// 106 after the check, 476 cycles a call in all.
	syncRead1  = 60
	syncRead2  = 80
	syncReturn = 60
	// CdRead: its time read 114 cycles from the call, its look at the
	// drive 144 after that, its watch 344 after the common delay is put
	// back, its time read 86 after ReadN and its return 66 after that.
	readEntry  = 70
	readAsk    = 110
	readSend   = 228
	readSent   = 72
	readReturn = 24
	// answerRest: in the handler, from acknowledging a command's answer
	// to its event (494 cycles from the cause read for Setloc's);
	// deliverRest: after the event, before the next look at the cause
	// (Sony's returns from the handler 602 cycles after delivering it,
	// the kernel's delivery taking 72 cycles longer than for the clean
	// library).
	answerRest  = 96
	deliverRest = 132
	// pauseRest: the read's last sector to the Pause, 1,642 cycles from
	// the cause read in Sony's library, which spends about 500 cycles
	// longer than the clean library on the sector itself; pauseReturn
	// and pauseAfter: after Pause's first answer, its event 662 cycles
	// from the cause read and the handler's return 1,496.
	pauseRest   = 672
	pauseReturn = 60
	pauseAfter  = 240

	cdlModeSpeed = 0x80 // double speed
	cdlModeSize  = 0x20 // 2340 bytes a sector rather than 2048

	sectorSize = 2048
)

// cdTwoPart marks the commands that answer twice, a first response at
// once and a second when the drive has done what was asked. The next
// command waits for the second.
var cdTwoPart = [32]bool{
	0x07: true, 0x08: true, cdlPause: true, cdlInit: true,
	0x15: true, 0x16: true, 0x1a: true, 0x1e: true,
}

// cdParams is how many parameter bytes the library sends with each
// command; the rest it sends with none. Test (19h) is one of those, as
// with Sony's library, which the drive refuses with error 20h, the wrong
// number of parameters (probe spec/probes/cdsearch).
var cdParams = [32]uint8{
	cdlSetloc: 3, 0x0d: 2, cdlSetmode: 1, 0x14: 1,
}

// cdPositioned marks the commands whose parameter is the position to go
// to, a CdlLOC: the library sends it first as a Setloc of its own, waits
// for its response, then sends the command without parameters (probes
// spec/probes/cdlimits, "case control", and cdmore: SeekL, SeekP, ReadN,
// Play and ReadS; Play with a nil parameter is sent alone).
var cdPositioned = [32]bool{cdlPlay: true, cdlReadN: true, 0x15: true, 0x16: true, cdlReadS: true}

// cdTries is how many times a command the drive refuses is sent before
// its error is returned, by CdControl and CdControlB alike (probes
// spec/probes/cdlimits and cdmore).
const cdTries = 4

// How a command's answers are announced with kernel events (see deliver):
// cdQuiet for the library's own commands; cdErrors for the Setloc sent
// before a positioned command, 40h when the drive refuses it and nothing
// otherwise; cdComplete for a command the program sent, 20h when it has
// fully answered (its first response, or its second when it answers twice)
// and 40h then 20h when the drive refuses it or answers it second with an
// error; cdPauseEv for the Pause the handler sends at a read's end, 40h at
// its first response and 20h at its second (probe spec/probes/cdmore, and
// spec 05).
const (
	cdQuiet = iota
	cdErrors
	cdComplete
	cdPauseEv
)

// cdDriver is the drive's state, shared between the program and the
// interrupt handler: the fields the handler writes are read through
// volatile loads.
type cdDriver struct {
	started bool

	// busy is set while a command waits for its first response, and resp
	// is that response, n bytes of it, with failed set for an error. raw
	// is the last answer's response as it was read.
	busy   uint32
	resp   [8]uint8
	raw    [8]uint8
	n      int
	failed bool

	// com is the command last sent, and second is set from its first
	// response to its second when it answers twice; resp2 is that second
	// response, and secondFailed is set when it is an error.
	com          uint8
	second       uint32
	resp2        [8]uint8
	secondFailed uint32

	// notify is how the command in flight is announced with kernel events
	// (cdQuiet to cdPauseEv, see there); notify2 is set from its first
	// response to its second when it answers twice and is announced.
	notify  int
	notify2 bool

	// mode is the drive mode the last Setmode set.
	mode uint8

	// A read: the sectors still to come, where the next goes, its length
	// in words, and whether the drive reported an error.
	remaining uint32
	buf       unsafe.Pointer
	words     uint32
	readErr   uint32

	// pausing is set from a read's last sector until the drive
	// acknowledges the Pause sent after it: the read counts as done only
	// then. count is what CdReadSync reports: the sectors still to come,
	// except that the last leaves 1 until that acknowledge. It is one
	// word so that a poll never sees it rise.
	pausing uint32
	count   uint32

	// asked is the time CdRead was last called, sent the time it sent
	// ReadN and ended the time the read's last sector came, in lines;
	// raised is what the drive had raised when CdRead looked.
	asked, sent, ended uint32
	raised             uint8

	// loc is the position the last Setloc sent.
	loc [4]uint8

	// next is the sector after the last CdReadFile read, for one that
	// carries on.
	next uint32

	// header is set when a read takes each sector's header into head
	// before its data.
	header bool
	head   [3]uint32

	// dir is where CdReadFile reads the disc's directories.
	dir [sectorSize / 4]uint32
}

var cd cdDriver

// cdWatch times a wait on the drive, as Sony's library times each of its
// waits (spec 08, "Waiting on the drive"): it reads the time when the wait
// begins and on each turn, and keeps when it began, the time it last read
// and how long the wait may take.
type cdWatch struct {
	start, last, limit uint32
}

var watch cdWatch

// cdNoLimit is the limit of a wait that does not give up: no spec gives
// the drive's waits a limit, except the handler's wait for Pause.
const cdNoLimit = 0xffff

// cdNow returns the time, in horizontal lines, read as Sony's library
// reads it: GPUSTAT, then root counter 1.
func cdNow() uint32 {
	gp1.Get()
	return lineCount.Get() & 0xffff
}

// begin starts timing a wait of up to limit lines.
//
//go:noinline
func (w *cdWatch) begin(limit uint32) {
	t := cdNow()
	spinCycles(watchBegin)
	volatile.StoreUint32(&w.start, t)
	volatile.StoreUint32(&w.last, t)
	volatile.StoreUint32(&w.limit, limit)
}

// check reads the time and reports whether the wait has taken longer than
// its limit.
//
//go:noinline
func (w *cdWatch) check() bool {
	t := cdNow()
	spinCycles(watchCheck)
	volatile.StoreUint32(&w.last, t)
	return (t-volatile.LoadUint32(&w.start))&0xffff > volatile.LoadUint32(&w.limit)
}

// start takes the drive over from the kernel, the first time it is used:
// it starts the library if nothing has, enables the drive's interrupts,
// prints the reference's notice (the address of its own CD state, kept so
// that the line is the same length), resets the drive (Getstat, then
// Init), unmutes the drive and turns up the SPU's and the drive's volume
// for CD audio.
// The notice is the time the reference takes before its first command.
func (c *cdDriver) start() {
	if c.started {
		return
	}
	c.started = true
	libStart()
	EnterCriticalSection()
	cdIndex.Set(1)
	cdReg3.Set(0x1f) // acknowledge anything pending
	cdReg2.Set(0x1f) // enable every interrupt
	cdIndex.Set(0)
	dmaPriority.Set(dmaPriority.Get() | dmaCDEnable)
	// The kernel reads by DMA with the channel's interrupt on; the
	// library waits for its transfers itself, and nothing would
	// acknowledge the interrupt.
	dmaInterrupt.Set(dmaInterrupt.Get()&0x00ffffff&^dicrCDEnable | dicrCDFlag)
	iStat.Set(^uint32(irqCD | irqDMA))
	iMask.Set(iMask.Get() | irqCD)
	ExitCriticalSection()
	// The reference sends Getstat straight after the line.
	ttyPrintf("addr=%08x\n", 0x8004d724, 0)
	c.control(cdlGetstat, nil, nil)
	c.control(cdlInit, nil, nil)
	c.mode = 0
	c.control(cdlDemute, nil, nil)
	// The drive's audio plays through the SPU, at full volume, with the
	// SPU on and unmuted, as the reference library leaves it.
	spuMainLeft.Set(0x3fff)
	spuMainRight.Set(0x3fff)
	spuCDLeft.Set(0x3fff)
	spuCDRight.Set(0x3fff)
	spuControl.Set(0xc001)
	// The drive's own audio volume: each side to its own at 80h, nothing
	// across, then the apply bit.
	cdIndex.Set(2)
	cdReg2.Set(0x80) // left to left
	cdReg3.Set(0x00) // left to right
	cdIndex.Set(3)
	cdReg1.Set(0x80) // right to right
	cdReg2.Set(0x00) // right to left
	cdReg3.Set(0x20)
}

// send waits, timed by the watch, until no command is in flight, then
// writes a command and its parameters to the drive, marking it in flight,
// announced by events as notify says. In the handler (inHandler), where
// the drive's interrupt cannot come, each turn of the wait handles what the
// drive has raised itself, as Sony's library does. Interrupts need not be
// off: the handler puts the register index back as it found it.
func (c *cdDriver) send(com uint8, param *uint8, notify int, inHandler bool) {
	spinCycles(sendEntry)
	watch.begin(cdNoLimit)
	for {
		watch.check()
		if inHandler {
			c.dispatch(false)
		}
		if volatile.LoadUint32(&c.busy) == 0 && volatile.LoadUint32(&c.second) == 0 {
			break
		}
	}
	volatile.StoreUint32(&c.busy, 1)
	volatile.StoreUint32(&c.secondFailed, 0)
	c.com = com
	c.notify = notify
	if com == cdlSetloc && param != nil {
		// The position is kept, a byte at a time, as Sony's library
		// keeps it.
		for i := range c.loc {
			volatile.StoreUint8(&c.loc[i], *(*uint8)(unsafe.Add(unsafe.Pointer(param), i)))
		}
	}
	// The drive's own busy bit is not waited on: Sony's library does not
	// read it (spec 08), and the emulator leaves it set after a SeekP
	// until the next command.
	spinCycles(sendRest)
	cdIndex.Set(0)
	if param != nil {
		for i := range cdParams[com&31] {
			cdReg2.Set(*(*uint8)(unsafe.Add(unsafe.Pointer(param), i)))
		}
	}
	cdReg1.Set(com)
}

// interrupt handles the drive's interrupt: each response, sector or error
// the drive has raised, until it has no more, as Sony's library does.
func (c *cdDriver) interrupt() { c.dispatch(false) }

// dispatch handles each answer the drive has raised until it has no more,
// with the register index put back as it was. Each answer's events are
// delivered at once, or, with defer set, the last answer's are returned for
// the caller to deliver.
func (c *cdDriver) dispatch(deferEv bool) (ev uintptr) {
	index := cdIndex.Get() & 3
	for {
		cdIndex.Set(1)
		if cdReg3.Get()&7 == 0 {
			break
		}
		if e := c.answer(); e != 0 {
			if !deferEv {
				deliverAll(e)
				spinCycles(deliverRest)
			} else {
				ev = e
			}
		}
	}
	cdIndex.Set(index)
	return ev
}

// answer handles one of the drive's interrupts, a response to a command, a
// sector of a read or an error, with the register index at 1, and returns
// the events to deliver for it, cdEvAck and cdEvComplete as bits, or 0.
func (c *cdDriver) answer() (ev uintptr) {
	kind := cdReg3.Get() & 7
	reading := volatile.LoadUint32(&c.remaining) > 0
	// The response goes to an 8-byte buffer, its rest zero-filled, a
	// byte at a time, as Sony's library does.
	n := 0
	for cdIndex.Get()&cdStatusResponse != 0 {
		v := cdReg1.Get()
		if n < len(c.raw) {
			volatile.StoreUint8(&c.raw[n], v)
			n++
		}
	}
	for i := n; i < len(c.raw); i++ {
		volatile.StoreUint8(&c.raw[i], 0)
	}
	// Acknowledged at once, and the drive's interrupts enabled again,
	// before a sector is taken: the drive keeps the sector in its buffer
	// until the next comes, a sector's time later.
	cdReg3.Set(7)
	cdReg2.Set(7)
	done := false
	if kind == 1 && reading {
		cdIndex.Set(0)
		cdReg3.Set(0) // the data FIFO emptied, of anything not taken before
		if c.header {
			// The sector's header and subheader first, then its
			// data; the rest of the sector is left in the drive.
			cdTransfer(unsafe.Pointer(&c.head[0]), uint32(len(c.head)))
		}
		cdTransfer(c.buf, c.words)
		c.buf = unsafe.Add(c.buf, 4*c.words)
		left := volatile.LoadUint32(&c.remaining) - 1
		volatile.StoreUint32(&c.remaining, left)
		done = left == 0
		if !done {
			volatile.StoreUint32(&c.count, left)
		}
	}
	switch {
	case (kind == 3 || kind == 5) && volatile.LoadUint32(&c.busy) != 0:
		// Copied to the result, a byte at a time, as Sony's library
		// does, after the bookkeeping it does first.
		spinCycles(answerRest)
		for i := range c.resp {
			volatile.StoreUint8(&c.resp[i], volatile.LoadUint8(&c.raw[i]))
		}
		c.n, c.failed = n, kind == 5
		if c.com == cdlReadN && kind == 5 {
			// CdRead does not wait for ReadN's answer: a refusal
			// ends the read with an error.
			volatile.StoreUint32(&c.remaining, 0)
			volatile.StoreUint32(&c.count, 0)
			volatile.StoreUint32(&c.readErr, 1)
		}
		if c.com == cdlPause && volatile.LoadUint32(&c.pausing) != 0 {
			volatile.StoreUint32(&c.pausing, 0)
			volatile.StoreUint32(&c.count, 0)
		}
		twoPart := kind == 3 && cdTwoPart[c.com&31]
		if twoPart {
			volatile.StoreUint32(&c.second, 1)
		}
		volatile.StoreUint32(&c.busy, 0)
		c.notify2 = twoPart && c.notify >= cdComplete
		switch {
		case kind == 5 && c.notify >= cdErrors:
			// Refused: 40h, and 20h too for the program's own command.
			ev = cdEvAck
			if c.notify >= cdComplete {
				ev |= cdEvComplete
			}
		case twoPart && c.notify == cdPauseEv:
			ev = cdEvAck
		case kind == 3 && !twoPart && c.notify >= cdComplete:
			ev = cdEvComplete
		}
	case kind == 2:
		spinCycles(answerRest)
		c.keepSecond()
		volatile.StoreUint32(&c.second, 0)
		if c.notify2 {
			c.notify2 = false
			ev = cdEvComplete
		}
	case kind == 5 && volatile.LoadUint32(&c.second) != 0:
		// The second response an error, as a seek past the disc's end
		// or into an audio track ends: 40h then 20h for the program's
		// command.
		c.keepSecond()
		volatile.StoreUint32(&c.secondFailed, 1)
		volatile.StoreUint32(&c.second, 0)
		if c.notify2 {
			c.notify2 = false
			ev = cdEvAck | cdEvComplete
		}
	case kind == 5:
		volatile.StoreUint32(&c.readErr, 1)
	case kind == 4:
		c.resp[0] = c.raw[0]
		cdAudioEnd()
	case kind == 1:
		// A sector with no read in progress, after a ReadN or ReadS the
		// program sent itself: the data request is cleared and 40h
		// announced, as Sony's library does; the sector is not taken.
		cdIndex.Set(0)
		cdReg3.Set(0)
		ev = cdEvAck
	}
	if done {
		ev = c.pause()
	}
	return ev
}

// keepSecond keeps the second response of a command that answers twice,
// as the handler read it, for CdControlB's result.
func (c *cdDriver) keepSecond() {
	for i := range c.resp2 {
		volatile.StoreUint8(&c.resp2[i], volatile.LoadUint8(&c.raw[i]))
	}
}

// pause pauses the drive at the end of a read, from the handler, which
// would otherwise read on. As Sony's library does, it waits there for the
// drive's first answer, checking for it on each turn of a timed wait, and
// returns its event, delivered once the wait is over.
func (c *cdDriver) pause() (ev uintptr) {
	volatile.StoreUint32(&c.pausing, 1)
	// The time the read ended, then the time once more, as Sony's
	// library reads it before the Pause.
	volatile.StoreUint32(&c.ended, cdNow())
	cdNow()
	spinCycles(pauseRest)
	c.send(cdlPause, nil, cdPauseEv, true)
	watch.begin(pauseWait)
	for volatile.LoadUint32(&c.busy) != 0 {
		late := watch.check()
		ev = c.dispatch(true)
		if late {
			break
		}
	}
	// Sony's library delivers the answer's event once its command has
	// returned, before the rest of the sector's handling.
	spinCycles(pauseReturn)
	if ev != 0 {
		deliverAll(ev)
	}
	spinCycles(pauseAfter)
	return 0
}

// cdTransfer moves words words of the sector in the drive's buffer to p
// by DMA, setting the memory controller's timing for the drive for it as
// Sony's library does. The register index must be 0.
func cdTransfer(p unsafe.Pointer, words uint32) {
	cdReg3.Set(0x80) // the sector to the data FIFO
	cdDelaySize.Set(cdDelayValue)
	comDelay.Set(comDelayDMA)
	dmaPriority.Set(dmaPriority.Get() | dmaCDEnable)
	dmaCDAddr.Set(physical(p))
	dmaCDBlock.Set(1<<16 | words)
	for cdIndex.Get()&cdStatusData == 0 {
	}
	dmaCDControl.Set(dmaFromCD)
	for dmaCDControl.Get()&dmaBusy != 0 {
	}
	comDelay.Set(comDelayIdle)
}

// The kernel events the library delivers for the drive (class F0000003h),
// as Sony's library was seen to (spec 05, and probe spec/probes/cdmore):
// cdEvComplete, 20h, when a command the program sent has fully answered
// (the response of Setloc, Getstat or Play sent alone, the second response
// of SeekL or Pause) and at the second response of the Pause sent at a
// read's end; cdEvAck, 40h, at that Pause's first response, at each sector
// the drive delivers while no read of the library's is in progress (after
// the program's own ReadN or ReadS), and when the drive refuses a command
// or answers it second with an error, 20h following it when the command is
// the program's own. Commands sent as part of a larger operation (Setmode
// and ReadN of a read, CdPlay's GetTN, GetTD and the Setloc before Play,
// the Setloc before a positioned command) and the drive's reset are not
// announced, except that the Setloc before a positioned command gives 40h
// when it is refused.
const (
	cdEventClass     = 0xf0000003
	cdEvComplete     = 0x20
	cdEvAck          = 0x40
	biosDeliverEvent = 0x07 // B0
)

// deliver delivers the drive's event spec to the kernel.
func deliver(spec uintptr) {
	libps_biosB(biosDeliverEvent, cdEventClass, spec, 0, 0)
}

// deliverAll delivers the events in mask, 40h before 20h, as Sony's library
// delivers the two of a refusal (some 560 cycles apart).
func deliverAll(mask uintptr) {
	if mask&cdEvAck != 0 {
		deliver(cdEvAck)
		if mask&cdEvComplete != 0 {
			spinCycles(deliverRest)
		}
	}
	if mask&cdEvComplete != 0 {
		deliver(cdEvComplete)
	}
}

// control sends a command and waits for its first response, which it
// copies to result, and reports whether the drive accepted it. The
// command's answers are not announced; controlEv announces them.
func (c *cdDriver) control(com uint8, param, result *uint8) bool {
	return c.command(com, param, result, cdQuiet)
}

// controlEv is control for a command whose answers the library announces
// with kernel events.
func (c *cdDriver) controlEv(com uint8, param, result *uint8) bool {
	return c.command(com, param, result, cdComplete)
}

// command is CdControl's command: after the Setloc of a command whose
// parameter is a position (cdPositioned), it sends com and waits for its
// first response. A command the drive refuses is sent again, cdTries times
// in all, before its error is returned, the Setloc too, which then ends
// the call with its error and the command is never sent (probe
// spec/probes/cdmore: CdControl sends a refused Setloc four times as
// CdControlB does).
func (c *cdDriver) command(com uint8, param, result *uint8, notify int) bool {
	c.start()
	if cdPositioned[com&31] && param != nil {
		if !c.insist(cdlSetloc, param, result, min(notify, cdErrors)) {
			return false
		}
		param = nil
	}
	return c.insist(com, param, result, notify)
}

// commandB is CdControlB's command: command, then a command that answers
// twice is waited for past its first response, with the second response in
// result (a Pause while reading gives 22h then 02h, and result holds 02h),
// an error among them returned as a refusal (probe spec/probes/cdlimits,
// "case control"). ReadN answers once: its errors come with the data,
// after the return.
func (c *cdDriver) commandB(com uint8, param, result *uint8) bool {
	if !c.command(com, param, result, cdComplete) {
		return false
	}
	if !cdTwoPart[com&31] {
		return true
	}
	watch.begin(cdNoLimit)
	for volatile.LoadUint32(&c.second) != 0 {
		watch.check()
	}
	if result != nil {
		copy(unsafe.Slice(result, len(c.resp2)), c.resp2[:])
	}
	return volatile.LoadUint32(&c.secondFailed) == 0
}

// insist is once, sent again while the drive refuses the command, cdTries
// times in all.
func (c *cdDriver) insist(com uint8, param, result *uint8, notify int) bool {
	for try := 1; ; try++ {
		if c.once(com, param, result, notify) {
			return true
		}
		if try == cdTries {
			return false
		}
	}
}

// once sends one command and waits for its first response, which it copies
// to result, and reports whether the drive accepted it.
func (c *cdDriver) once(com uint8, param, result *uint8, notify int) bool {
	if com == cdlSetmode && param != nil {
		c.mode = *param
	}
	c.send(com, param, notify, false)
	// The first response comes through the handler; the wait is timed
	// all the same, a check on each turn.
	watch.begin(cdNoLimit)
	for volatile.LoadUint32(&c.busy) != 0 {
		watch.check()
	}
	spinCycles(controlReturn)
	// The whole response buffer goes to result, the bytes past the
	// response zero (probe spec/probes/cdsearch).
	if result != nil {
		copy(unsafe.Slice(result, len(c.resp)), c.resp[:])
	}
	return !c.failed
}

// CdControl sends the drive the command com with its parameters at param
// (a position for SeekL, SeekP, ReadN, Play and ReadS, sent first as a
// Setloc), waits for the drive to acknowledge it, and copies the reply to
// result when it is not nil. It returns 1, or 0 if the drive refused the
// command (sent again, four times in all, first).
//
//export CdControl
//go:noinline
func CdControl(com uint8, param, result *uint8) int32 {
	if cd.controlEv(com, param, result) {
		return 1
	}
	return 0
}

// CdControlB is CdControl waiting, past the first response, for the
// second of a command that answers twice. It returns 1, or 0 if the drive
// refused the command (sent again, four times in all, first) or answered
// it with an error second; result receives the command's last response,
// the second when there is one, error or not.
//
//export CdControlB
//go:noinline
func CdControlB(com uint8, param, result *uint8) int32 {
	if cd.commandB(com, param, result) {
		return 1
	}
	return 0
}

// CdlLOC is a position on the disc: minutes, seconds and sectors in BCD.
type CdlLOC struct {
	Minute, Second, Sector, Track uint8
}

// CdIntToPos converts the sector number i to a position, counting the two
// seconds of the first track's pregap, writes it to p and returns p. The
// Track byte is left alone. Each part goes to BCD as its tens times 16 plus
// its units, in 32 bits, cut to a byte, so a negative or overflowing number
// gives what the probe shows (spec/probes/cdsearch).
//
//export CdIntToPos
func CdIntToPos(i int32, p *CdlLOC) *CdlLOC {
	i += 150
	p.Minute = bcd(i / 4500)
	p.Second = bcd(i / 75 % 60)
	p.Sector = bcd(i % 75)
	return p
}

func bcd(v int32) uint8 { return uint8(v/10*16 + v%10) }

// CdPosToInt converts the position p back to its sector number, less the
// two seconds of pregap. Each byte is read as its high digit times 10 plus
// its low one, whether or not it is valid BCD; Track is ignored.
//
//export CdPosToInt
func CdPosToInt(p *CdlLOC) int32 {
	return (unbcd(p.Minute)*60+unbcd(p.Second))*75 + unbcd(p.Sector) - 150
}

// CdRead reads sectors sectors from the position the last CdlSetloc gave
// into buf, in the drive mode mode, and returns 1 once the read has
// started, or 0 if it could not be. CdReadSync waits for it.
//
//export CdRead
//go:noinline
func CdRead(sectors int32, buf *uint32, mode int32) int32 {
	if sectors <= 0 {
		return 0
	}
	// The drive reads whole sectors, less their sync, so that each
	// sector's header comes too, unless the mode asks for that already.
	spinCycles(readEntry)
	m := uint8(mode)
	cd.header = m&cdlModeSize == 0
	m |= cdlModeSize
	cd.buf = unsafe.Pointer(buf)
	cd.words = sectorSize / 4
	if !cd.header {
		cd.words = 2340 / 4
	}
	// As Sony's library does: the time the read is asked for, a look at
	// whether the drive has raised anything, the data FIFO emptied and
	// the common delay put back (spec 08).
	volatile.StoreUint32(&cd.asked, cdNow())
	spinCycles(readAsk)
	cdIndex.Set(1)
	cd.raised = cdReg3.Get() & 7
	cdIndex.Set(0)
	cdReg3.Set(0)
	comDelay.Set(comDelayIdle)
	if m != cd.mode && !cd.control(cdlSetmode, &m, nil) {
		return 0
	}
	spinCycles(readSend)
	volatile.StoreUint32(&cd.readErr, 0)
	volatile.StoreUint32(&cd.remaining, uint32(sectors))
	volatile.StoreUint32(&cd.count, uint32(sectors))
	// CdRead returns once ReadN is sent, without waiting for the drive
	// to acknowledge it, as the reference does, noting the time it did.
	cd.send(cdlReadN, nil, cdQuiet, false)
	spinCycles(readSent)
	volatile.StoreUint32(&cd.sent, cdNow())
	spinCycles(readReturn)
	return 1
}

// CdReadSync reports on the read in progress, which ends when the drive
// acknowledges the Pause sent after its last sector. With mode 0 it waits
// for it and returns 0, and with any other mode it returns at once with the
// sectors still to come. Either returns -1 if the drive reported an
// error. result, if not nil, receives the drive's last status.
//
//export CdReadSync
//go:noinline
func CdReadSync(mode int32, result *uint8) int32 {
	// As Sony's library does on each call: it reads the time twice,
	// then times the wait, with one check even when it does not wait
	// (spec 08). The read's state is taken first, so that a call the
	// drive's interrupt lands in returns what it was before, as Sony's
	// do.
	failed, left := volatile.LoadUint32(&cd.readErr), volatile.LoadUint32(&cd.count)
	cdNow()
	spinCycles(syncRead1)
	cdNow()
	spinCycles(syncRead2)
	watch.begin(cdNoLimit)
	for {
		watch.check()
		if failed != 0 {
			return -1
		}
		// The last sector leaves 1 until the Pause after it is
		// acknowledged.
		if mode != 0 || left == 0 {
			if result != nil {
				*result = cd.resp[0]
			}
			spinCycles(syncReturn)
			return int32(left)
		}
		failed, left = volatile.LoadUint32(&cd.readErr), volatile.LoadUint32(&cd.count)
	}
}

// CdReadFile starts reading the file named file, an absolute path such as
// "\DATA\FILE.DAT;1" (case ignored, the version optional, looked up
// through CdSearchFile's tables as Sony's does, spec 14), into addr: nbyte
// bytes of it, or all of it with nbyte 0. With file nil it carries on from where the last read stopped.
// It returns the bytes it will read, or 0 if the file was not found or
// the read could not start.
//
//export CdReadFile
func CdReadFile(file *byte, addr *uint32, nbyte int32) int32 {
	sector := cd.next
	if file != nil {
		var size uint32
		var ok bool
		if sector, size, ok = cdFiles.search(file); !ok {
			return 0
		}
		if nbyte == 0 {
			nbyte = int32(size)
		}
	}
	n := (nbyte + sectorSize - 1) / sectorSize
	if !cd.readAt(sector, n, addr) {
		return 0
	}
	cd.next = sector + uint32(n)
	return nbyte
}

// readAt starts reading n sectors from sector into buf at double speed.
func (c *cdDriver) readAt(sector uint32, n int32, buf *uint32) bool {
	var pos CdlLOC
	CdIntToPos(int32(sector), &pos)
	return c.controlEv(cdlSetloc, &pos.Minute, nil) && CdRead(n, buf, cdlModeSpeed) != 0
}

// readDir reads one sector into dir and waits for it.
func (c *cdDriver) readDir(sector uint32) bool {
	return c.readAt(sector, 1, &c.dir[0]) && CdReadSync(0, nil) == 0
}

// sameName compares a record's name rec with the name asked for, ignoring
// the case of letters. A name without a version (";1") matches the record
// whatever its version (spec 14, CdReadExec).
func sameName(rec, name []byte) bool {
	if indexByte(name, ';') < 0 {
		if i := indexByte(rec, ';'); i >= 0 {
			rec = rec[:i]
		}
	}
	if len(rec) != len(name) {
		return false
	}
	for i := range rec {
		if upper(rec[i]) != upper(name[i]) {
			return false
		}
	}
	return true
}

// indexByte returns the index of the first c in b, or -1.
func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func upper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
