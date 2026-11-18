//go:build yaroze

// stagemap finds, from outside, which of the instruction cache's 256 lines the
// vertical blank interrupt has taken by each of four points in it, for spec 46:
// on a console the kernel's own code, its entry before any handler and its way
// out after the library's, runs some 70 cycles longer under the clean library
// than under Sony's (irqsplit's gap less whole, and its callback to poll), with
// the one clean image whose handler sat elsewhere showing neither. The
// candidate is the clean handler's code on the cache lines of the console
// kernel's entry and exit, which are not the emulator kernel's.
//
// It is icachemap with the measurement moved into the interrupt. A map, 4 KB
// of code aligned to 4 KB, has one routine of four instructions per cache line.
// A harness, machine code, waits for an interrupt, calls every map line, which
// fills the cache with the map, arms a stage and waits for the next interrupt.
// At the armed stage a hook calls every map line again, each between two reads
// of timer 2: a line still cached is quick, one the interrupt has taken by then
// some 4 cycles slower on a console (5 or more in the emulator). The stages:
//
//   - entry: first in the kernel's interrupt chain of priority 0 (SysEnqIntRP,
//     C(02h)): the lines of the kernel's entry;
//   - kernel: first in the chain of priority 3: those and the kernel's own
//     handlers';
//   - callback: the library's VSyncCallback: those, the kernel's way to the
//     library and the library's handler up to its callback;
//   - poll: the program's first code after the interrupt, as icachemap's
//     "blank": the whole interrupt;
//   - control: no interrupt between the fill and the measurement.
//
// The harness and its hooks are on about 30 cache lines, whose map lines they
// can't measure, so they are written twice, at 400h and C00h in a 4 KB page,
// and every stage is measured from both. Before it measures, the measuring
// loop runs once over a routine of its own, so that its own misses are not
// charged to line 0.
//
// It prints, for each stage, TIMES for the first run of each harness (a
// character a line: cycles less 4, 0-9 then a-v), RUN for each run and MAP:
// the taken lines as 64 hex digits, line 0 the top bit of the first, how many,
// and the lines as ranges. A stage's lines less the stage before's are what
// ran between the two. Then END.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timer2Mod = 0x1f801124
	reps      = 4
	lines     = 256
)

// Scratchpad words the harness uses.
const (
	spTimes = 0x000 // 256 halfwords: each map line's time
	spGap1  = 0x200 // the measured interrupt's gap
	spGapS  = 0x204 // the starting gap
	spJunk  = 0x208 // the warm-up pass's time
	spRA    = 0x210
	spMap   = 0x214
	spStage = 0x218
	spArm   = 0x21c // the stage whose hook measures at the next interrupt
)

// Stages. The hooks are 1 to 3.
const (
	stageControl = 0
	stageEntry   = 1
	stageKernel  = 2
	stageCall    = 3
	stagePoll    = 4
)

// Registers.
const (
	zero = 0
	v0   = 2
	a0   = 4
	a1   = 5
	a2   = 6
	t0   = 8
	t1   = 9
	t2   = 10
	t3   = 11
	t4   = 12
	t5   = 13
	t6   = 14
	t7   = 15
	t8   = 24
	t9   = 25
	ra   = 31
)

func iType(op, rs, rt uint32, imm int32) uint32 {
	return op<<26 | rs<<21 | rt<<16 | uint32(imm)&0xffff
}
func rType(rs, rt, rd, fn uint32) uint32   { return rs<<21 | rt<<16 | rd<<11 | fn }
func lui(rt, imm uint32) uint32            { return iType(0x0f, 0, rt, int32(imm)) }
func ori(rt, rs, imm uint32) uint32        { return iType(0x0d, rs, rt, int32(imm)) }
func addiu(rt, rs uint32, i int32) uint32  { return iType(0x09, rs, rt, i) }
func andi(rt, rs, imm uint32) uint32       { return iType(0x0c, rs, rt, int32(imm)) }
func sltiu(rt, rs uint32, i int32) uint32  { return iType(0x0b, rs, rt, i) }
func lw(rt, rs uint32, off int32) uint32   { return iType(0x23, rs, rt, off) }
func sw(rt, rs uint32, off int32) uint32   { return iType(0x2b, rs, rt, off) }
func sh(rt, rs uint32, off int32) uint32   { return iType(0x29, rs, rt, off) }
func addu(rd, rs, rt uint32) uint32        { return rType(rs, rt, rd, 0x21) }
func subu(rd, rs, rt uint32) uint32        { return rType(rs, rt, rd, 0x23) }
func jr(rs uint32) uint32                  { return rs<<21 | 0x08 }
func jalr(rd, rs uint32) uint32            { return rs<<21 | rd<<11 | 0x09 }
func beqOff(rs, rt uint32, off int) uint32 { return iType(0x04, rs, rt, int32(off)) }
func bneOff(rs, rt uint32, off int) uint32 { return iType(0x05, rs, rt, int32(off)) }

const nop = 0

// asm assembles into code from at, with labels for branches.
type asm struct {
	code   []uint32
	at     int
	labels map[string]int
	fix    []fixup
}

type fixup struct {
	at    int
	label string
	beq   bool
	rs    uint32
	rt    uint32
}

func (a *asm) put(w ...uint32) {
	for _, x := range w {
		a.code[a.at] = x
		a.at++
	}
}

func (a *asm) label(name string) { a.labels[name] = a.at }

func (a *asm) beq(rs, rt uint32, l string) {
	a.fix = append(a.fix, fixup{a.at, l, true, rs, rt})
	a.put(0)
}

func (a *asm) bne(rs, rt uint32, l string) {
	a.fix = append(a.fix, fixup{a.at, l, false, rs, rt})
	a.put(0)
}

func (a *asm) resolve() {
	for _, f := range a.fix {
		off := a.labels[f.label] - (f.at + 1)
		if f.beq {
			a.code[f.at] = beqOff(f.rs, f.rt, off)
		} else {
			a.code[f.at] = bneOff(f.rs, f.rt, off)
		}
	}
}

// wait waits for an interrupt: a read of timer 2 more than 2,000 cycles
// after the one before, or about 4 seconds of polls. It stores the jump at
// off in the scratchpad. t4 holds 1F80h<<16.
func (a *asm) wait(name string, off int32) {
	a.put(
		lui(t5, 0x10),
		lw(t2, t4, 0x1120),
	)
	a.label(name)
	a.put(
		lw(t0, t4, 0x1120),
		addiu(t5, t5, -1),
		subu(t1, t0, t2),
		andi(t1, t1, 0xffff),
		sltiu(t3, t1, 2000),
		addu(t2, t0, zero),
		beqOff(t5, zero, 2),
		nop,
	)
	a.bne(t3, zero, name)
	a.put(
		nop,
		sw(t1, t4, off),
	)
}

// measure writes the measuring loop: a pass over the harness's own routine at
// stub, then every map line between two reads of timer 2, the times to the
// scratchpad. t4 holds 1F80h<<16.
func (a *asm) measure(pre string, stub uintptr) {
	a.put(
		addiu(t3, zero, 1),
		lui(t9, uint32(stub>>16)),
		ori(t9, t9, uint32(stub&0xffff)),
		addiu(t5, zero, 1),
		addiu(t6, t4, spJunk),
	)
	a.label(pre + "m")
	a.put(
		lw(t0, t4, 0x1120),
		jalr(t8, t9),
		nop,
		lw(t1, t4, 0x1120),
		addiu(t9, t9, 16),
		subu(t1, t1, t0),
		sh(t1, t6, spTimes),
		addiu(t5, t5, -1),
	)
	a.bne(t5, zero, pre+"m")
	a.put(addiu(t6, t6, 2))
	a.beq(t3, zero, pre+"done")
	a.put(
		nop,
		addu(t3, zero, zero),
		lw(t9, t4, spMap),
		addiu(t5, zero, lines),
	)
	a.beq(zero, zero, pre+"m")
	a.put(addu(t6, t4, zero))
	a.label(pre + "done")
}

// harness writes the harness and its hooks at code, which is at p. The
// harness is entered with a0 the map and a1 the stage. It returns the words
// written and the three hooks' addresses.
func harness(code []uint32, p uintptr) (int, [3]uintptr) {
	a := &asm{code: code, labels: map[string]int{}}
	// The routine the measuring loop warms up on.
	stub := p
	a.put(nop, nop, jr(t8), nop)
	a.put(
		lui(t4, 0x1f80),
		sw(ra, t4, spRA),
		sw(a0, t4, spMap),
		sw(a1, t4, spStage),
		sw(zero, t4, spArm),
	)
	a.wait("w0", spGapS)
	// Fill.
	a.put(
		lw(t9, t4, spMap),
		addiu(t5, zero, lines),
	)
	a.label("fill")
	a.put(
		jalr(t8, t9),
		nop,
		addiu(t5, t5, -1),
	)
	a.bne(t5, zero, "fill")
	a.put(addiu(t9, t9, 16))
	a.put(lw(t7, t4, spStage))
	a.put(addiu(t0, zero, stagePoll))
	a.beq(t7, zero, "now")
	a.put(nop)
	a.beq(t7, t0, "blank")
	a.put(nop)
	a.put(sw(t7, t4, spArm))
	a.label("blank")
	a.wait("w1", spGap1)
	a.put(
		lw(t7, t4, spStage),
		addiu(t0, zero, stagePoll),
	)
	a.bne(t7, t0, "ret")
	a.put(nop)
	a.label("now")
	a.measure("p", stub)
	a.label("ret")
	a.put(
		lw(ra, t4, spRA),
		nop,
		jr(ra),
		nop,
	)
	// The hooks: each measures if its stage is armed, and returns 0.
	var hooks [3]uintptr
	for i := range hooks {
		hooks[i] = p + uintptr(4*a.at)
		a.put(
			lui(t4, 0x1f80),
			lw(t0, t4, spArm),
			addiu(t1, zero, int32(stageEntry+i)),
		)
		a.bne(t0, t1, "out")
		a.put(nop)
		a.beq(zero, zero, "hook")
		a.put(nop)
	}
	a.label("hook")
	a.put(sw(zero, t4, spArm))
	a.measure("h", stub)
	a.label("out")
	a.put(
		jr(ra),
		addu(v0, zero, zero),
	)
	a.resolve()
	return a.at, hooks
}

// 6 pages: one for alignment, the map, and one for each harness.
var codeBuf [6 * 1024]uint32

// nodes are the chain elements: next, second function, first function, 0.
var nodes [2][4]uint32

var kernBuf [32]uint32

// kernelC writes code that calls the kernel's C(fn)(prio, &nodes[k]).
func kernelC(code []uint32, fn, prio int32, k int) uintptr {
	n := uint32(uintptr(unsafe.Pointer(&nodes[k])))
	copy(code, []uint32{
		lui(a1, n>>16),
		ori(a1, a1, n&0xffff),
		addiu(a0, zero, prio),
		addiu(t2, zero, 0xc0),
		jr(t2),
		addiu(t1, zero, fn),
	})
	return uintptr(unsafe.Pointer(&code[0]))
}

func call(p uintptr) {
	var f func()
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, p
	f()
}

func run(h, mapAddr uintptr, stage uint32) {
	var f func(uintptr, uint32)
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, h
	f(mapAddr, stage)
}

func load16(off uintptr) uint32 {
	return uint32(volatile.LoadUint16((*uint16)(yaroze.Pointer(0x1f800000 + off))))
}

func load32(off uintptr) uint32 {
	return volatile.LoadUint32((*uint32)(yaroze.Pointer(0x1f800000 + off)))
}

// warm is the time of a cached map line: the time the control runs' lines
// took most often, from warmHist. The shortest time would do on a console,
// but one line a little quicker than the rest, which the emulator's model
// shows now and then, would then make every other line look taken.
var (
	warm     uint32 = 0xffff
	warmHist [64]uint32
)

// taken reads the last run's times into bits: a line is taken when it took
// more than warm plus 2. A control run's times go into warm first. It
// returns the shortest and longest times, and the times as one character
// each, 0-9 then a-v, for 0 to 31 cycles past 4 (w for more).
func taken(bits *[lines]bool, control bool) (lo, hi uint32, chars string) {
	lo, hi = 0xffff, 0
	var t [lines]uint32
	c := make([]byte, lines)
	const digits = "0123456789abcdefghijklmnopqrstuv"
	for i := range t {
		t[i] = load16(spTimes + uintptr(2*i))
		lo, hi = min(lo, t[i]), max(hi, t[i])
		if control && t[i] < uint32(len(warmHist)) {
			warmHist[t[i]]++
		}
		switch d := int(t[i]) - 4; {
		case d < 0:
			c[i] = '-'
		case d < len(digits):
			c[i] = digits[d]
		default:
			c[i] = 'w'
		}
	}
	if control {
		most := uint32(0)
		for v, n := range warmHist {
			if n > most {
				most, warm = n, uint32(v)
			}
		}
	}
	for i := range t {
		bits[i] = t[i] > warm+2
	}
	return lo, hi, string(c)
}

func hexMap(bits *[lines]bool) string {
	const digits = "0123456789abcdef"
	b := make([]byte, lines/4)
	for i := range b {
		v := 0
		for j := 0; j < 4; j++ {
			if bits[4*i+j] {
				v |= 8 >> j
			}
		}
		b[i] = digits[v]
	}
	return string(b)
}

// printRanges prints the set lines as hex ranges.
func printRanges(bits *[lines]bool) {
	const digits = "0123456789ABCDEF"
	hex := func(i int) { print(string(digits[i>>4]), string(digits[i&15])) }
	for i := 0; i < lines; i++ {
		if !bits[i] {
			continue
		}
		j := i
		for j+1 < lines && bits[j+1] {
			j++
		}
		print(" ")
		hex(i)
		if j > i {
			print("-")
			hex(j)
		}
		i = j
	}
}

// cover reports whether a harness at page offset off, n words long, leaves
// map line i alone.
func cover(off, n, i int) bool { return i < off/16 || i >= (off+4*n+15)/16 }

type harnessAt struct {
	entry uintptr
	hooks [3]uintptr
	off   int
	words int
}

// use points the chains and the callback at h's hooks.
func use(h *harnessAt) {
	volatile.StoreUint32(&nodes[0][2], uint32(h.hooks[0]))
	volatile.StoreUint32(&nodes[1][2], uint32(h.hooks[1]))
	yaroze.VSyncCallback(h.hooks[2])
}

func stage(name string, hs *[2]harnessAt, mapAddr uintptr, st uint32) {
	var all [lines]int // runs that saw each line taken, of those that can see it
	var seen [lines]int
	for r := 0; r < reps; r++ {
		for k := range hs {
			h := &hs[k]
			use(h)
			run(h.entry, mapAddr, st)
			var bits [lines]bool
			lo, hi, chars := taken(&bits, st == stageControl)
			if r == 0 {
				println("TIMES", name, "harness", k, chars)
			}
			n := 0
			for i := range bits {
				if !cover(h.off, h.words, i) {
					continue
				}
				seen[i]++
				if bits[i] {
					all[i]++
					n++
				}
			}
			print("RUN ", name, " rep ", r, " harness ", k, " line time ", lo, " to ", hi, " cached ", warm, " taken ", n)
			if st != stageControl {
				print(" gap ", load32(spGap1)&0xffff, " armed left ", load32(spArm))
			}
			print("\n")
		}
	}
	var bits [lines]bool
	n, odd := 0, 0
	for i := range bits {
		bits[i] = all[i]*2 > seen[i]
		if bits[i] {
			n++
		}
		if all[i] != 0 && all[i] != seen[i] {
			odd++
		}
	}
	print("MAP ", name, " ", hexMap(&bits), " taken ", n, " unsteady ", odd, " lines")
	printRanges(&bits)
	print("\n")
}

func display(name string, hs *[2]harnessAt, page uintptr) {
	stage(name+" control", hs, page, stageControl)
	stage(name+" entry", hs, page, stageEntry)
	stage(name+" kernel", hs, page, stageKernel)
	stage(name+" callback", hs, page, stageCall)
	stage(name+" poll", hs, page, stagePoll)
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	saved := volatile.LoadUint32((*uint32)(yaroze.Pointer(timer2Mod)))
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), 0) // system clock

	base := uintptr(unsafe.Pointer(&codeBuf[0]))
	page := (base + 4095) &^ 4095
	words := func(p uintptr, n int) []uint32 {
		i := int(p-base) / 4
		return codeBuf[i : i+n]
	}
	m := words(page, 1024)
	for i := 0; i < lines; i++ {
		copy(m[4*i:], []uint32{nop, nop, jr(t8), nop})
	}
	var hs [2]harnessAt
	for k, off := range [2]int{0x400, 0xc00} {
		p := page + 4096 + uintptr(off)
		n, hooks := harness(words(p, 256), p)
		hs[k] = harnessAt{
			entry: p + 16,
			hooks: hooks,
			off:   off,
			words: n,
		}
	}
	enq0, enq3 := kernelC(kernBuf[0:], 2, 0, 0), kernelC(kernBuf[8:], 2, 3, 1)
	deq0, deq3 := kernelC(kernBuf[16:], 3, 0, 0), kernelC(kernBuf[24:], 3, 3, 1)
	yaroze.FlushCache()
	println("map", page, "harnesses", hs[0].entry, hs[1].entry, "words", hs[0].words)

	use(&hs[0])
	yaroze.EnterCriticalSection()
	call(enq0)
	call(enq3)
	yaroze.ExitCriticalSection()

	yaroze.GsInitGraph(320, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	yaroze.VSync(0)
	display("240p", &hs, page)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.VSync(0)
	display("480i", &hs, page)

	yaroze.VSyncCallback(0)
	yaroze.EnterCriticalSection()
	call(deq3)
	call(deq0)
	yaroze.ExitCriticalSection()
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), saved)
	println("END")
}
