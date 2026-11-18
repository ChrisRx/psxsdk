package cpu

// The CPU charges 2 cycles for every instruction, as psxe does, plus what
// the bus charges for the fetch. Under SetCost it charges what a console's
// instructions cost instead, as two probes measured them on a console
// (internal/libps/spec/35-hardware-session.md, "2026-11-10: what the CPU's
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
	// ports not named below are taken to cost the same.
	costIOLoad = 4

	// costSPULoad is a load from the SPU's registers beyond its own cycle,
	// and costSPUAgain what one straight after another costs on top: 20.9 a
	// halfword in a run (cpucost4, "lh spu"), but 21.0 for a load and a nop
	// ("lh spu nop"), so the nop costs nothing there, and the clean sound
	// tick's 24 reads, each after other work, cost 20 each: its loop with
	// nops for its stores takes 36.2 a read on a console, 20 and its 16
	// other instructions (padshape, "readLoopNops"; spec 46).
	costSPULoad  = 19
	costSPUAgain = 1

	// costIRQLoad is a load from the interrupt controller beyond its own
	// cycle: 4.0 for I_STAT in a run (cpucost4, "lw istat").
	costIRQLoad = 3

	// costAfterStores is how long RAM stays busy after the write buffer's
	// last store to RAM leaves it, for a load from RAM, which waits until
	// then. A store to a port doesn't hold it up: a store to the SPU or
	// I_STAT and a load from RAM cost 8.3 the pair, with the store's 7.5
	// drain not waited for (cpucost4, "sh spu lw ram", "sw istat lw ram").
	// A load from a port waits for every store ("sh spu lh spu", 29.1). A
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
	// a port (cpucost, "sw gp0 nop", 1.6 a store). The ports not named
	// below are taken to drain as fast as GP0. A store to the SPU takes
	// 7.5 (cpucost4, "sh spu", 7.5 a store in a run), and one to the
	// interrupt controller 2 ("sw istat").
	//
	// A store to RAM in another row of the DRAM than the store to RAM
	// before it starts to leave the buffer no sooner than wbRowHalves after
	// that one left: a row is ramRowShift's 1 KB (rowcost, "sw alt +512" 2.2
	// a store and "+1024" 5.6, and 4 bytes apart across a 1 KB boundary
	// 5.7), a run of stores taking turns between two rows costs 5.7 a store
	// where a run to one row costs 2 (cpucost5, "sw ram rows"), each change
	// of row in a run of 16 costs 4 (rowcost, "sw A x8 B x8", "x8 B A x7",
	// "x4 B x4 A x4 B x4": 4, 8 and 12 to 22 over "sw A x16"), and stores
	// to two rows a nop or two apart still cost 5 each, four nops apart
	// nothing ("sw A nop1 sw B nop1" to "nop8"). A store to another row after
	// RAM has been idle costs nothing more. A load from a port does not wait
	// for the last store's change of row ("sw A x15 B" 2 under "sw A x16");
	// the model has it wait for the store as if it had changed none, which
	// is 2 over that row and 1.4 under cpucost5's "sw x10 same word sh ram lh
	// spu". Loads show no rows ("lw ram rows", "sw then lw rows"). The clean
	// sound tick stored each envelope it read to one row and its ten pad
	// stores to another, 24 times a blank, until spec 46.
	wbRAMHalves = 4
	wbRowHalves = 7
	ramRowShift = 10
	wbIOHalves  = 3
	wbSPUHalves = 15
	wbIRQHalves = 4

	// costMult and costDiv are when HI and LO are ready after a multiply
	// or divide starts: mult then mflo cost 7.8, div then mflo 37.8
	// (cpucost, "mult mflo", "div mflo"). The wait is charged to mfhi or
	// mflo.
	costMult = 7
	costDiv  = 37

	// costMultMid and costMultWide are what a multiply takes beyond
	// costMult when its first operand, rs, is wider than 11 bits, and wider
	// than 20: 8 of mult and mflo take 7.6, 10.6 and 14.6 cycles a pair by
	// rs alone, whatever rt is (padcost48, spec 48). A signed rs counts by
	// its magnitude less one, so that -800h is narrow and -801h not; multu
	// counts rs unsigned.
	costMultMid  = 3
	costMultWide = 7

	// costMissAfterStore and costMissAfterLoad are how long RAM stays busy
	// for an instruction fetch, a cache miss's fill, after the write
	// buffer's last store to RAM leaves it, and after a load from RAM. A
	// line missed with stores draining costs 5 to 7 cycles more than a line
	// of nops on a console (cpucost, cold less warm a line: "sw ram nop3"
	// 10.2 against "nop 256" 5.1, "sw ram nop" 11.8, "sw ram 128" 11.9;
	// spec 46), and one missed after loads some 3 more ("lw ram" 8.1, "lw
	// ram nop" 8.1, "sw then lw ram" 8.1). The same 3 after a store's drain
	// as after a load gives the store rows within half a cycle a line (4
	// gave a cycle over, 6 three). The clean library's handler pads with
	// stores and runs cold at every blank, so without these it costs less
	// here than on a console.
	costMissAfterStore = 3
	costMissAfterLoad  = 3

	// costBlockStoreTenths is how long before the end of a block's open
	// part, in tenths of a cycle, a store has to start leaving the write
	// buffer while a DMA runs in blocks (HoldBlocks).
	costBlockStoreTenths = 14

	// costDMALoad is a load from the DMA controller's registers beyond its
	// own cycle: 4.0 for DICR and DPCR (cpucost5, "lw dicr", "lw dpcr").
	costDMALoad = 3
)

// region is what an address reaches, as the model costs it.
type region uint8

const (
	regionRAM region = iota
	regionScratch
	regionIO
	regionROM
	regionSPU
	regionIRQ
	regionDMA
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
	case a >= 0x1f801c00 && a < 0x1f802000:
		return regionSPU
	case a >= 0x1f801070 && a < 0x1f801080:
		return regionIRQ
	case a >= 0x1f801080 && a < 0x1f801100:
		return regionDMA
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
	// wbAt the oldest, wbLast when the last does, and wbLastRAM when the
	// last store to RAM did or does.
	wb        [wbDepth]uint64
	wbAt      int
	wbLast    uint64
	wbLastRAM uint64

	// wbRow is the DRAM row of the last store to RAM, and wbPort when a
	// load from a port may go ahead: wbLast, less what the last store waited
	// to change rows.
	wbRow  uint32
	wbPort uint64

	// spuFree is when a load from the SPU straight after another may
	// start.
	spuFree uint64

	// ramFree is when RAM is free for a cache miss's fill, after the last
	// store to RAM has drained or the last load from RAM has read.
	ramFree uint64

	// hiloReady is when HI and LO hold the last multiply or divide's
	// result.
	hiloReady uint64

	// busFree is when a DMA lets the bus go, and holdNext how long one
	// will hold it from the end of the next step.
	busFree, holdNext uint64

	// A transfer in blocks (HoldBlocks) holds the bus for the start of
	// each block and leaves it open for the rest: from blkStart to blkEnd,
	// in cycles, a block every blkPeriod tenths of a cycle, the last
	// blkOpen tenths of each open. blkNext is the blocks of one that
	// starts at the end of the next step.
	blkStart, blkEnd            uint64
	blkPeriod, blkOpen, blkNext uint64

	// now is when the last step ended, for steady.
	now uint64

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

// HoldBlocks has a DMA that runs in blocks, as an image's does on its way to
// the GPU, hold the bus under the cost model for the first part of each of
// its blocks, from the end of the next step: a block every period tenths of
// a cycle, with the bus open to the CPU for the last open tenths of each.
// Code in the instruction cache runs on throughout. An access to the bus
// (a load, a store leaving the write buffer, a cache miss's fill) may start
// only while the bus is open, and waits for that otherwise.
//
// On a console (probes/dmahold, internal/libps/spec/50-image-console-time.md)
// an image of 64 blocks of 16 words takes 1,800 cycles to go to the GPU,
// 28.1 a block. While it goes, 256 cached nops take 259 cycles as they do
// alone, and a counter read after them some 20 more. Loads from RAM go one
// to a block, in a run (64 in 1,770 cycles) or with a nop or seven between
// (16 in 470). Reads of a port go two to a block in a run (32 in 480) and
// one to a block seven nops apart (16 in 475). Stores leave the write buffer
// about three to a block (64 in 690). So the bus is open for more than a
// port read's 5 cycles and no more than a load's 6. Three stores' drains
// of 2 cycles would start in that, every block, and the console has fewer,
// about 2.6: the model has a store start to leave the buffer no later than
// costBlockStoreTenths before the open part ends, which makes it two in
// the blocks whose open part starts late in a cycle and three in the rest.
//
// How far a store's word lies from the image's makes no difference: 32
// stores take 331 to 334 cycles with the blocks going whether they go 4
// bytes from the image's first word, 256 KB from it or to the stack
// (dmahold, round 2). Stores in twos and fours with nops between cost some
// 4 cycles a store over what they cost alone, and a load and a store in
// turn 535 cycles for 16 of each, a load a block and some.
func (c *CPU) HoldBlocks(blocks, period, open uint64) {
	if m := c.cost; m != nil && blocks != 0 && open < period {
		m.blkNext, m.blkPeriod, m.blkOpen = blocks, period, open
	}
}

// bus returns when an access to the bus wanted at t may start: after a DMA
// that holds it, and in the open part of a block of one that runs in
// blocks.
func (m *cost) bus(t uint64) uint64 { return m.busFor(t, 0) }

// busFor is bus for an access that has to start short tenths of a cycle
// before a block's open part ends.
func (m *cost) busFor(t, short uint64) uint64 {
	t = max(t, m.busFree)
	if t < m.blkStart || t >= m.blkEnd {
		return t
	}
	held := m.blkPeriod - m.blkOpen
	switch phase := (t - m.blkStart) * 10 % m.blkPeriod; {
	case phase < held:
		t += (held - phase + 9) / 10
	case phase+short >= m.blkPeriod:
		// Too late in this block: the next one's open part.
		t += (m.blkPeriod - phase + held + 9) / 10
	}
	return t
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
		// A load into $zero after one into $zero costs what one into
		// another register does: 16 in a run take 6 each, as a run into
		// registers in turn (padcost48, "lw zero", spec 48).
		if m.loadReg == gteReg || !isLoad(op) || op>>16&31 == m.loadReg && m.loadReg != 0 {
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
	if m.blkNext != 0 {
		m.blkStart, m.blkEnd = t, t+(m.blkNext*m.blkPeriod+9)/10
		m.blkNext = 0
	}
	m.now = t
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
			m.hiloReady = t + costMult + multWidth(c.R[op>>21&31], op&1 != 0)
		case 0x1a, 0x1b: // div, divu
			m.hiloReady = t + costDiv
		}
	case 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x32: // the loads, lwc2
		addr := c.R[op>>21&31] + uint32(int32(int16(op)))
		r := regionOf(addr)
		if r == regionScratch {
			break
		}
		ready := (m.wbPort + 1) / 2
		if r == regionRAM {
			ready = 0
			if m.wbLastRAM != 0 {
				ready = (m.wbLastRAM+1)/2 + costAfterStores
			}
		}
		if r == regionSPU {
			ready = max(ready, m.spuFree)
		}
		t = m.wait(t, m.bus(max(t, ready)))
		t = m.fetch(pc, t)
		switch r {
		case regionRAM:
			t += costRAMLoad
			m.ramFree = t + costMissAfterLoad
			m.loadNext, m.loadReg = true, op>>16&31
			if code == 0x32 {
				m.loadReg = gteReg
			}
		case regionROM:
			t += costROM
		case regionSPU:
			t += costSPULoad
			m.spuFree = t + costSPUAgain
		case regionIRQ:
			t += costIRQLoad
		case regionDMA:
			t += costDMALoad
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
		drain, held := uint64(wbIOHalves), uint64(0)
		start := max(2*t, m.wbLast)
		start = max(start, 2*m.busFor(start/2, costBlockStoreTenths))
		switch r {
		case regionRAM:
			drain = wbRAMHalves
			row := addr & 0x1fffffff >> ramRowShift
			if row != m.wbRow && m.wbLastRAM != 0 && m.wbLastRAM+wbRowHalves > start {
				held = m.wbLastRAM + wbRowHalves - start
				start += held
			}
			m.wbRow = row
		case regionSPU:
			drain = wbSPUHalves
		case regionIRQ:
			drain = wbIRQHalves
		}
		done := start + drain
		m.wb[m.wbAt], m.wbLast, m.wbPort = done, done, done-held
		if r == regionRAM {
			m.wbLastRAM = done
			m.ramFree = (done+1)/2 + costMissAfterStore
		}
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
		t = m.wait(t, m.bus(t))
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

	// The fill reads RAM, so it waits for a DMA holding the bus and for
	// RAM to be free of the last store or load.
	t = m.wait(t, m.bus(max(t, m.ramFree)))
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
		before.blkNext == 0 && m.now >= m.blkEnd &&
		m.streamNext == before.streamNext &&
		m.loadNext == before.loadNext && (!m.loadNext || m.loadReg == before.loadReg)
}

// multWidth is what a multiply by rs takes beyond costMult: nothing for rs
// within 11 bits, costMultMid within 20, costMultWide wider. Signed, a
// negative rs counts as its complement.
func multWidth(rs uint32, unsigned bool) uint64 {
	if !unsigned && int32(rs) < 0 {
		rs = ^rs
	}
	switch {
	case rs < 1<<11:
		return 0
	case rs < 1<<20:
		return costMultMid
	}
	return costMultWide
}
