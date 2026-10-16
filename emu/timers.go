package emu

// timer is one of the three root counters at 1F801100.
//
// psxe keeps the count as a float, so that the fractional clock sources can
// add a fraction of a tick, and reads truncate it.
type timer struct {
	counter float32
	target  uint32

	syncEnable    bool
	syncMode      uint32
	resetTarget   bool
	irqTarget     bool
	irqMax        bool
	irqRepeat     bool
	irqToggle     bool
	clkSource     uint32
	irq           bool // the mode register's interrupt bit: 1 when not requesting
	targetReached bool
	maxReached    bool
	irqFired      bool
	paused        bool
	blankOnce     bool
}

// timers is the three of them, with the blanking state the GPU reports.
//
// psxe advances the counters every step, but nothing sees them between the
// points where a counter passes its target or 65535 and the accesses that
// read or change them, so they are brought up to date only then: steps is
// the machine's count of steps, synced the step the counters were last
// advanced to, and due the step at which one next needs checking.
type timers struct {
	t      [3]timer
	hblank bool
	vblank bool
	gpu    *gpu
	ic     *ic

	steps  *uint64
	synced uint64
	due    uint64
	wake   *bool // set when due changes
}

func (tm *timers) mode(i int) uint16 {
	t := &tm.t[i]
	v := uint16(b2i(t.syncEnable)) |
		uint16(t.syncMode)<<1 |
		uint16(b2i(t.resetTarget))<<3 |
		uint16(b2i(t.irqTarget))<<4 |
		uint16(b2i(t.irqMax))<<5 |
		uint16(b2i(t.irqRepeat))<<6 |
		uint16(b2i(t.irqToggle))<<7 |
		uint16(t.clkSource)<<8 |
		uint16(b2i(t.irq))<<10 |
		uint16(b2i(t.targetReached))<<11 |
		uint16(b2i(t.maxReached))<<12
	// Reading the mode clears the reached flags.
	t.targetReached = false
	t.maxReached = false
	return v
}

// setMode writes a mode register, which also resets the counter.
func (tm *timers) setMode(i int, v uint16) {
	t := &tm.t[i]
	t.syncEnable = v&1 != 0
	t.syncMode = uint32(v>>1) & 3
	t.resetTarget = v>>3&1 != 0
	t.irqTarget = v>>4&1 != 0
	t.irqMax = v>>5&1 != 0
	t.irqRepeat = v>>6&1 != 0
	t.irqToggle = v>>7&1 != 0
	t.clkSource = uint32(v>>8) & 3
	t.targetReached = false
	t.maxReached = false
	t.irq = true
	t.irqFired = false
	t.counter = 0
	t.blankOnce = false
	t.paused = false

	switch i {
	case 0:
		if t.syncMode == 1 || t.syncMode == 2 || !t.syncEnable {
			return
		}
		t.paused = tm.hblank || t.syncMode == 3
	case 1:
		if t.syncMode == 1 || t.syncMode == 2 || !t.syncEnable {
			return
		}
		t.paused = tm.vblank || t.syncMode == 3
	case 2:
		if !t.syncEnable {
			return
		}
		t.paused = t.syncMode == 0 || t.syncMode == 3
	}
}

func (tm *timers) read(off uint32) (uint32, bool) {
	i := int(off >> 4)
	if i > 2 {
		return 0, false
	}
	tm.sync(*tm.steps)
	switch off & 0xf {
	case 0:
		return uint32(tm.t[i].counter), true
	case 4:
		// Reading clears the reached flags, which a settled timer has
		// to set again.
		defer tm.schedule()
		return uint32(tm.mode(i)), true
	case 8:
		return tm.t[i].target, true
	}
	return 0, false
}

func (tm *timers) Read32(off uint32) uint32 {
	v, _ := tm.read(off)
	return v
}

func (tm *timers) Read16(off uint32) uint16 {
	v, _ := tm.read(off)
	return uint16(v)
}

func (tm *timers) Read8(uint32) uint8 { return 0 }

func (tm *timers) write(off uint32, v uint16) {
	i := int(off >> 4)
	if i > 2 {
		return
	}
	tm.sync(*tm.steps)
	defer tm.schedule()
	switch off & 0xf {
	case 0:
		tm.t[i].counter = float32(v)
	case 4:
		tm.setMode(i, v)
	case 8:
		tm.t[i].target = uint32(v)
	}
	// psxe checks for the target and overflow after any write, even to an
	// offset that is not a register.
	tm.check(i)
}

func (tm *timers) Write32(off, v uint32)        { tm.write(off, uint16(v)) }
func (tm *timers) Write16(off uint32, v uint16) { tm.write(off, v) }
func (tm *timers) Write8(uint32, uint8)         {}

// due reports whether check has anything to do: it does nothing unless the
// counter is past its target or past 65535. Testing that first keeps the
// call off the path every step takes.
func (t *timer) due() bool {
	return t.counter > float32(t.target) || t.counter > 65535
}

// check handles a counter having passed its target or overflowed, and
// raises the timer's interrupt if it is due.
//
// psxe counts the target as reached once the counter is past it, not at it.
func (tm *timers) check(i int) {
	t := &tm.t[i]
	irq := false

	if t.counter > float32(t.target) {
		t.targetReached = true
		if t.resetTarget {
			t.counter = 0
		}
		if t.irqTarget {
			irq = true
		}
	}
	if t.counter > 65535 {
		t.counter = 0
		t.maxReached = true
		if t.irqMax {
			irq = true
		}
	}
	if !irq {
		return
	}

	if t.irqToggle {
		t.irq = !t.irq
	} else {
		t.irq = false
	}
	trigger := !t.irq
	if !t.irqRepeat {
		if !trigger || t.irqFired {
			return
		}
		t.irqFired = true
	}
	t.irq = true
	if trigger {
		tm.ic.raise(irqTimer0 << i)
	}
}

// dotclockDiv is how far timer 0 advances per CPU cycle when it counts the
// dot clock, which depends on the horizontal resolution. The divisions are
// done one at a time in float, as psxe's are.
func (tm *timers) dotclockDiv() float32 {
	ratio := float32(float32(11) / float32(7))
	if tm.gpu.displayMode&0x40 != 0 {
		return float32(ratio / 7)
	}
	return float32(ratio / [4]float32{10, 8, 5, 4}[tm.gpu.displayMode&3])
}

// stepCycles is how far psxe advances the counters a step, whatever the
// step cost.
const stepCycles = 2

// increment returns what timer i adds in a step, and whether adding it n
// times comes to adding n times it at once. The system clock's 2 and its
// eighth's 0.25 do: the counters only ever hold multiples of a quarter
// below 2^18, which a float32 holds exactly. The dot clock's fraction does
// not, so a timer counting it is advanced a step at a time.
func (tm *timers) increment(i int) (inc float32, exact bool) {
	t := &tm.t[i]
	switch i {
	case 0:
		if t.clkSource&1 != 0 {
			return float32(float32(stepCycles) * tm.dotclockDiv()), false
		}
	case 1:
		// Counting hblanks happens in hblank instead.
		if t.clkSource&1 != 0 {
			return 0, true
		}
	case 2:
		if t.clkSource > 1 {
			return float32(float32(stepCycles) / 8), true
		}
	}
	return stepCycles, true
}

// sync advances the counters to the given step. Nothing changes whether
// a timer is paused, or what it counts, without syncing first, so they
// hold for every step in between.
func (tm *timers) sync(to uint64) {
	n := to - tm.synced
	if n == 0 {
		return
	}
	tm.synced = to
	for i := range tm.t {
		t := &tm.t[i]
		if t.paused {
			continue
		}
		inc, exact := tm.increment(i)
		if exact {
			t.counter = float32(float64(t.counter) + float64(n)*float64(inc))
			continue
		}
		for range n {
			t.counter = float32(t.counter + inc)
		}
	}
}

// settled reports whether checking a counter that is past its target, but
// not 65535, would change nothing: it does not reset at the target, has
// already flagged it, and either raises no interrupt there or has raised
// its one interrupt and is not toggling the bit.
func (t *timer) settled() bool {
	if t.counter > 65535 || t.resetTarget || !t.targetReached {
		return false
	}
	return !t.irqTarget || !t.irqToggle && !t.irqRepeat && t.irqFired && !t.irq
}

// schedule works out the step at which update next has a timer to check:
// the first at which checking a counter changes something. That is when it
// passes its target, or, once past and settled there, 65535. A timer still
// changing past its target, or advanced a step at a time, is checked at the
// next step.
func (tm *timers) schedule() {
	*tm.wake = true
	tm.due = ^uint64(0)
	for i := range tm.t {
		t := &tm.t[i]
		if t.paused {
			continue
		}
		inc, exact := tm.increment(i)
		// The limit the counter has to pass, in quarters, in which
		// everything is a whole number. The target is at most 65535.
		var limit uint64
		switch {
		case !exact:
			tm.due = min(tm.due, tm.synced+1)
			continue
		case !t.due():
			limit = uint64(t.target) * 4
		case t.settled():
			limit = 65535 * 4
		default:
			tm.due = min(tm.due, tm.synced+1)
			continue
		}
		if inc == 0 {
			continue
		}
		q, step := uint64(t.counter*4), uint64(inc*4)
		tm.due = min(tm.due, tm.synced+(limit-q)/step+1)
	}
}

// update advances the counters to the given step, at which one is due to
// be checked, and checks those that are.
func (tm *timers) update(now uint64) {
	tm.sync(now)
	for i := range tm.t {
		if t := &tm.t[i]; !t.paused && t.due() {
			tm.check(i)
		}
	}
	tm.schedule()
}

// The GPU calls these as it crosses the blanking periods, before the step's
// update. Timer 0 can synchronise to horizontal blanking, timer 1 can count
// it or synchronise to vertical blanking.

func (tm *timers) hblankStart() {
	tm.sync(*tm.steps)
	tm.hblank = true
	if t := &tm.t[1]; t.clkSource&1 != 0 && !t.paused {
		t.counter = float32(t.counter + 1)
		if t.due() {
			tm.check(1)
		}
	}
	tm.blankSync(&tm.t[0])
	tm.schedule()
}

func (tm *timers) hblankEnd() {
	tm.sync(*tm.steps)
	tm.hblank = false
	tm.blankEndSync(&tm.t[0])
	tm.schedule()
}

func (tm *timers) vblankStart() {
	tm.sync(*tm.steps)
	tm.vblank = true
	tm.blankSync(&tm.t[1])
	tm.schedule()
}

func (tm *timers) vblankEnd() {
	tm.sync(*tm.steps)
	tm.vblank = false
	tm.blankEndSync(&tm.t[1])
	tm.schedule()
}

// blankSync applies a synchronisation mode at the start of blanking: pause
// during it, reset at it, reset at it and run only during it, or wait for
// it once and then run freely.
func (tm *timers) blankSync(t *timer) {
	if !t.syncEnable {
		return
	}
	switch t.syncMode {
	case 0:
		t.paused = true
	case 1:
		t.counter = 0
	case 2:
		t.counter = 0
		t.paused = false
	case 3:
		if !t.blankOnce {
			t.blankOnce = true
			t.syncEnable = false
			t.paused = false
		}
	}
}

func (tm *timers) blankEndSync(t *timer) {
	if !t.syncEnable {
		return
	}
	switch t.syncMode {
	case 0:
		t.paused = false
	case 2:
		t.paused = true
	}
}
