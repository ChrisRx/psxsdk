package cpu

// The CPU charges 2 cycles for every instruction, as psxe does, plus what
// the bus charges for the fetch. Under SetCost it charges what a console's
// instructions cost instead, as two probes measured them on a console
// (exp/cleanlib/spec/35-hardware-session.md, "2026-11-10: what the CPU's
// accesses cost", probes/cpucost and probes/cpucost2): the instruction
// cache, the memory a load reads, the write buffer stores go through, the
// HI/LO wait after a multiply or divide, and the bus a DMA holds. Each
// figure below cites the probe's row it comes from.
//
// The model is what the probes show, simplified, rather than the R3000A's
// pipeline: a step's cost is worked out from the instruction before it runs,
// and charged before it runs, so that what it reads from a device, a root
// counter above all, is read after any wait. Where the console's behavior is
// more subtle than the data shows, the model follows the data, and says so.
//
// Off, nothing of it runs but a test at each step, and nothing about the
// emulation changes.

// Costs in cycles, unless named for half cycles.
const (
	// icacheLines is the instruction cache's lines: 4 KB, direct mapped,
	// 16-byte lines (cpucost2, "map 2KB" to "map 8KB": two blocks run in
	// turn evict each other 4 KB and 8 KB apart, and not 2 KB or 6 KB).
	icacheLines = 256

	// costMissLine and costMissWordHalves are what a cache miss costs: a
	// line is filled from the word entered to the line's end, about 3
	// cycles a line and 1.5 a word (cpucost2, "cold 2 of 4 at 0" 10.7 and
	// "at 8" 7.7 a line; "cold nop 256" 9.1 a line; cpucost, "nop 2048",
	// 2.3 an instruction). The instruction missed is charged the whole
	// fill, and the words after it in its line, run straight after it,
	// arrive with the fill and cost nothing more.
	costMissLine       = 3
	costMissWordHalves = 3

	// costMissJump is what a miss entered by a jump or branch costs on top:
	// jumping into lines costs about 1.6 a line more than running into them
	// ("cold 2 of 4" against "cold nop 256", per word filled).
	costMissJump = 2

	// costUncached is an instruction fetched through KSEG1 from RAM, 6.1
	// (cpucost, "nop uncached").
	costUncached = 6

	// costROM is an access to the BIOS ROM beyond the instruction's own
	// cycle: a load from it costs 33 (cpucost, "lw rom"). The BIOS runs
	// its ROM code through KSEG1, so its fetches cost the same. The
	// expansion region, unmeasured, is taken to cost what the ROM does.
	costROM = 32

	// costRAMLoad is a load from RAM beyond its own cycle: 6.1 a load in a
	// run of loads into different registers (cpucost, "lw ram").
	costRAMLoad = 5

	// costRAMLoadNext is what the instruction after a load from RAM costs
	// on top, unless it is a load into a different register: a load and
	// a nop, or a load and an instruction using it, cost 8.1 (cpucost,
	// "lw ram nop", "lw ram then use"), and a run of loads into the same
	// register 7 each (cpucost2, "lw same", "lw stride 4"). After lwc2 the
	// next costs it whatever it is: a run of lwc2 costs 7.1 each, and lwc2
	// and a nop 8.1 (cpucost3, "lwc2 ram", "lwc2 ram nop"). On an R3000A
	// this is the pipeline; the model has only what the data shows.
	costRAMLoadNext = 1

	// costIOLoad is a load from an I/O port beyond its own cycle: 4.9 for
	// GPUSTAT or a root counter (cpucost, "lw gpustat", "lw timer"). The
	// other ports are taken to cost the same.
	costIOLoad = 4

	// costAfterStores is how long RAM stays busy after the write buffer's
	// last store leaves it, for a load from RAM, which waits until then: a
	// store then a load cost 13.3 for the two (cpucost, "sw then lw ram"),
	// 3 more than the drain and the load alone come to. Three instructions
	// between don't hide it: a load and a store cost 12.3 a pair, with three
	// nops after the store or without (cpucost3, "lw ram sw ram", "lw ram sw
	// ram nop3"). A load from a port shows none ("sw ram 1", whose counter
	// read waits for the store).
	costAfterStores = 3

	// wbDepth is the write buffer's entries. A store costs its cycle if
	// there is room, and waits for an entry otherwise.
	wbDepth = 4

	// wbRAMHalves and wbIOHalves are the half cycles a store takes to
	// leave the write buffer, one after another: 2 for RAM (cpucost, "sw
	// ram 1" to "sw ram 128": 3, 5, 9, 17, 33 for 1, 2, 4, 8, 16 stores
	// with the counter read after them, which waits for the last), 1.5 for
	// a port (cpucost, "sw gp0 nop", 1.6 a store). Only GP0 was measured;
	// the other ports are taken to drain as fast.
	wbRAMHalves = 4
	wbIOHalves  = 3

	// costMult and costDiv are when HI and LO are ready after a multiply
	// or divide starts: mult then mflo cost 7.8, div then mflo 37.8
	// (cpucost, "mult mflo", "div mflo"). The wait is charged to mfhi or
	// mflo. A console's multiply takes longer for larger operands; the
	// probe's are small, and the model has the one time.
	costMult = 7
	costDiv  = 37
)

// region is what an address reaches, as the model costs it.
type region uint8

const (
	regionRAM region = iota
	regionScratch
	regionIO
	regionROM
)

// regionOf returns the region addr is in.
func regionOf(addr uint32) region {
	if addr >= 0xc0000000 {
		return regionIO // KSEG2: the cache control register
	}
	switch a := addr & 0x1fffffff; {
	case a < 0x00800000:
		return regionRAM
	case a >= 0x1fc00000 && a < 0x1fc80000, a >= 0x1f000000 && a < 0x1f800000:
		return regionROM
	case a >= 0x1f800000 && a < 0x1f800400:
		return regionScratch
	}
	return regionIO
}

// gteReg is loadReg after lwc2, which no CPU register's number matches.
const gteReg = 32

// cost is the cost model's state.
type cost struct {
	// lines is the instruction cache: for each line, the physical address
	// of the 16 bytes it holds, with the low four bits saying which of its
	// words are valid.
	lines [icacheLines]uint32

	// lastFetch is the physical address last fetched, to tell a fetch that
	// runs on from one that jumps.
	lastFetch uint32

	// streamNext is the physical address of the next word the last miss
	// filled, which costs nothing if it is fetched next, or 0.
	streamNext uint32

	// loadNext is whether the last step loaded from RAM, into loadReg,
	// which costs the next step costRAMLoadNext. loadReg is gteReg for
	// lwc2.
	loadNext bool
	loadReg  uint32

	// wb is when each write buffer entry leaves it, in half cycles, with
	// wbAt the oldest, and wbLast when the last does.
	wb     [wbDepth]uint64
	wbAt   int
	wbLast uint64

	// hiloReady is when HI and LO hold the last multiply or divide's
	// result.
	hiloReady uint64

	// busFree is when a DMA lets the bus go, and holdNext how long one
	// will hold it from the end of the next step.
	busFree, holdNext uint64

	// events counts the steps that waited or missed, which change what
	// the same steps cost the next time, for TryPass.
	events uint64
}

// SetCost turns the cost model described above on or off. It is off unless
// a caller turns it on, and with it off nothing about the emulation
// changes. Turning it on starts with the instruction cache empty. It
// survives a reset, which empties the cache.
func (c *CPU) SetCost(on bool) {
	if on {
		c.cost, c.base = new(cost), 0
		return
	}
	c.cost, c.base = nil, 2
}

// HoldBus has a DMA hold the bus for cycles, under the cost model, from the
// end of the next step: an access to anything but the instruction cache and
// the scratchpad waits until it ends, and a store made meanwhile leaves the
// write buffer only after it.
//
// On a console, channel 6 clearing 1,024 words takes 1,097 cycles, and a
// counter read straight after the transfer starts gets through first
// (cpucost2, "otc alone"). Loads from RAM after it wait for the transfer to
// end ("otc lw 64": the transfer's time and the loads'), and so does the
// counter read after 256 cached instructions ("otc nop 256": 1,088, where
// alone they take 255). Whether cached code runs on during the transfer
// or stops, which spec 35 reads it as, the data can't tell apart; the model
// lets it run, up to its next access to the bus.
func (c *CPU) HoldBus(cycles uint64) {
	if c.cost != nil {
		c.cost.holdNext = cycles
	}
}

// step returns what the step about to run costs: the instruction is fetched,
// as Run left it in Opcode from PC, and not yet executed, so the registers
// are what it reads. Waits come before the fetch.
func (m *cost) step(c *CPU) uint32 {
	start := c.TotalCycles
	t := start
	op, pc := c.Opcode, c.PC

	if m.loadNext {
		m.loadNext = false
		if m.loadReg == gteReg || !isLoad(op) || op>>16&31 == m.loadReg {
			t += costRAMLoadNext
		}
	}

	if c.irqPending() {
		// The instruction is fetched, and the exception taken in its
		// place.
		t = m.fetch(pc, t)
	} else {
		t = m.execute(c, op, pc, t)
	}

	if m.holdNext != 0 {
		m.busFree = t + m.holdNext
		m.holdNext = 0
	}
	return uint32(t - start)
}

// execute is step's cost for an instruction that runs, from t.
func (m *cost) execute(c *CPU, op, pc uint32, t uint64) uint64 {
	switch code := op >> 26; code {
	case 0x00:
		switch op & 0x3f {
		case 0x10, 0x12: // mfhi, mflo
			t = m.wait(t, m.hiloReady)
		case 0x18, 0x19: // mult, multu
			m.hiloReady = t + costMult
		case 0x1a, 0x1b: // div, divu
			m.hiloReady = t + costDiv
		}
	case 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x32: // the loads, lwc2
		addr := c.R[op>>21&31] + uint32(int32(int16(op)))
		r := regionOf(addr)
		if r == regionScratch {
			break
		}
		ready := (m.wbLast + 1) / 2
		if r == regionRAM && m.wbLast != 0 {
			ready += costAfterStores
		}
		t = m.wait(t, max(ready, m.busFree))
		t = m.fetch(pc, t)
		switch r {
		case regionRAM:
			t += costRAMLoad
			m.loadNext, m.loadReg = true, op>>16&31
			if code == 0x32 {
				m.loadReg = gteReg
			}
		case regionROM:
			t += costROM
		default:
			t += costIOLoad
		}
		return t
	case 0x28, 0x29, 0x2a, 0x2b, 0x2e, 0x3a: // the stores, swc2
		addr := c.R[op>>21&31] + uint32(int32(int16(op)))
		r := regionOf(addr)
		if c.COP0[SR]&srIsc != 0 {
			// With the cache isolated and swapped, as the BIOS flushes
			// it, a store reaches the instruction cache's line instead,
			// which the model takes as emptying it.
			if c.COP0[SR]&srSwC != 0 {
				m.lines[addr>>4&(icacheLines-1)] = 0
			}
			break
		}
		if r == regionScratch {
			break
		}
		t = m.wait(t, (m.wb[m.wbAt]+1)/2)
		t = m.fetch(pc, t)
		drain := uint64(wbIOHalves)
		if r == regionRAM {
			drain = wbRAMHalves
		}
		done := max(2*t, m.wbLast, 2*m.busFree) + drain
		m.wb[m.wbAt], m.wbLast = done, done
		m.wbAt = (m.wbAt + 1) % wbDepth
		return t
	}
	return m.fetch(pc, t)
}

// isLoad reports whether op is one of the CPU's loads.
func isLoad(op uint32) bool { return op>>26 >= 0x20 && op>>26 <= 0x26 }

// wait returns when a step at t that has to wait until ready starts.
func (m *cost) wait(t, ready uint64) uint64 {
	if ready <= t {
		return t
	}
	m.events++
	return ready
}

// fetch returns when the instruction at pc, fetched at t, has run, but for
// what its data access costs.
func (m *cost) fetch(pc uint32, t uint64) uint64 {
	phys := pc & 0x1fffffff
	jumped := phys != m.lastFetch+4
	m.lastFetch = phys

	// Only RAM through KUSEG and KSEG0 is cached.
	if r := regionOf(pc); r != regionRAM || pc>>29 == 5 {
		t = m.wait(t, m.busFree)
		if r == regionROM {
			return t + 1 + costROM
		}
		return t + costUncached
	}

	i, word := phys>>4&(icacheLines-1), phys>>2&3
	line := m.lines[i]
	if line&^0xf == phys&^0xf && line>>word&1 != 0 {
		if phys == m.streamNext {
			m.streamNext = (phys + 4) * b2u((phys+4)&0xf != 0)
			return t
		}
		m.streamNext = 0
		return t + 1
	}

	t = m.wait(t, m.busFree)
	m.events++
	words := 4 - word
	t += costMissLine + (costMissWordHalves*uint64(words)+1)/2
	if jumped {
		t += costMissJump
	}
	if line&^0xf != phys&^0xf {
		line = phys &^ 0xf
	}
	m.lines[i] = line | 0xf<<word&0xf
	m.streamNext = (phys + 4) * b2u(word < 3)
	return t
}

// steady reports whether a pass that started with the model as before and
// ended with it as m costs the same every time it runs again: it missed
// nothing and waited for nothing, and leaves the model where it found it.
func (m *cost) steady(before *cost) bool {
	return m.events == before.events && before.holdNext == 0 &&
		m.streamNext == before.streamNext &&
		m.loadNext == before.loadNext && (!m.loadNext || m.loadReg == before.loadReg)
}
