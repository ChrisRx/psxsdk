//go:build yaroze

// icachemap finds, from outside, which of the instruction cache's 256 lines
// (4 KB, direct mapped, 16-byte lines) a vertical blank interrupt takes, and
// which a VSync(0) on a 480-line display takes with the blank it waits for,
// for spec 45: the clean library's code sits on other cache lines than
// Sony's, so a program's lines that survive a blank differ by library.
//
// The probe writes a map, 4 KB of code aligned to 4 KB: one routine of four
// instructions per cache line (nop, nop, jr t8, nop), so that map line i is
// cache line i. A harness, machine code the probe also writes, then runs
// with nothing else between:
//
//  1. waits for an interrupt (a jump of timer 2 over 2,000 cycles), so that
//     each run starts just after one;
//  2. calls every map line, which fills the cache with the map;
//  3. the event: none (control), a vertical blank (wait as in 1), or
//     VSync(0); or, for warm, no fill and a second wait;
//  4. calls every map line again, each between two reads of timer 2. A line
//     still cached takes 9 cycles on a console, one the event took 13 or
//     more (11 and 16 or more in the emulator). A line counts as taken when
//     it took more than the control runs' shortest time plus 2. The first
//     run of each event prints every line's time (TIMES, a character a
//     line: cycles less 4, 0-9 then a-v).
//
// The harness's own code is on about 25 cache lines, whose map lines it
// can't measure, so it is written twice, at 400h and C00h in a 4 KB page,
// and each map is measured from both: a line counts as taken when both
// harnesses that can see it say so.
//
// For each display and event it prints MAP (for warm, which doesn't fill
// the cache with the map, only GAP means anything): the taken lines as 64 hex
// digits, line 0 the top bit of the first, and how many; and for each run
// the threshold. For the blank and warm events it prints GAP: the interrupt's
// length in cycles (to the first read of timer 2 after it), with the cache
// filled with the map (cold: every line the interrupt runs misses) and with
// only the wait between two interrupts (warm: what the last interrupt left).
// Cold less warm is what the interrupt's misses cost; warm against warm is
// what the libraries' interrupts cost without misses.
//
// It prints END and returns.
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
	spGap1  = 0x200 // the event's gap (blank), or the warm second gap
	spGapS  = 0x204 // the starting gap
	spRA    = 0x210
	spMap   = 0x214
	spMode  = 0x218
)

// Modes.
const (
	modeControl = 0
	modeBlank   = 1
	modeVSync   = 2
	modeWarm    = 3
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
func rType(rs, rt, rd, fn uint32) uint32  { return rs<<21 | rt<<16 | rd<<11 | fn }
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

// harness writes the harness at code, entered with a0 the map, a1 the mode
// and a2 VSync's address.
func harness(code []uint32) int {
	a := &asm{code: code, labels: map[string]int{}}
	a.put(
		lui(t4, 0x1f80),
		sw(ra, t4, spRA),
		sw(a0, t4, spMap),
		sw(a1, t4, spMode),
	)
	a.wait("w0", spGapS)
	a.put(
		lw(t7, t4, spMode),
		addiu(t0, zero, modeWarm),
	)
	a.beq(t7, t0, "warm")
	a.put(nop)
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
	// The event.
	a.put(addiu(t0, zero, modeBlank))
	a.beq(t7, t0, "blank")
	a.put(addiu(t0, zero, modeVSync))
	a.beq(t7, t0, "vsync")
	a.put(nop)
	a.beq(zero, zero, "measure")
	a.put(nop)
	a.label("vsync")
	a.put(
		addu(t9, a2, zero),
		jalr(ra, t9),
		addu(a0, zero, zero),
		lui(t4, 0x1f80),
	)
	a.beq(zero, zero, "measure")
	a.put(nop)
	a.label("warm")
	a.wait("w2", spGap1)
	a.beq(zero, zero, "measure")
	a.put(nop)
	a.label("blank")
	a.wait("w1", spGap1)
	// Measure.
	a.label("measure")
	a.put(
		lw(t9, t4, spMap),
		addiu(t5, zero, lines),
		addu(t6, t4, zero),
	)
	a.label("m")
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
	a.bne(t5, zero, "m")
	a.put(
		addiu(t6, t6, 2),
		lw(ra, t4, spRA),
		nop,
		jr(ra),
		nop,
	)
	a.resolve()
	return a.at
}

// 6 pages: one for alignment, the map, and one for each harness.
var codeBuf [6 * 1024]uint32

func run(h, mapAddr, vsync uintptr, mode uint32) {
	var f func(uintptr, uint32, uintptr)
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, h
	f(mapAddr, mode, vsync)
}

func load16(off uintptr) uint32 {
	return uint32(volatile.LoadUint16((*uint16)(yaroze.Pointer(0x1f800000 + off))))
}

func load32(off uintptr) uint32 {
	return volatile.LoadUint32((*uint32)(yaroze.Pointer(0x1f800000 + off)))
}

// warm is the time of a cached map line, the shortest a control run saw.
var warm uint32 = 0xffff

// taken reads the last run's times into bits: a line is taken when it took
// more than warm plus 2. On a console a line missed costs only some 4 or 5
// cycles more than one cached (9 against 13 or more); the emulator charges
// more (11 against 16 or more). It returns the shortest and longest times,
// and the times as one character each, 0-9 then a-v, for 0 to 31 cycles
// past 4 (w for more).
func taken(bits *[lines]bool) (lo, hi uint32, chars string) {
	lo, hi = 0xffff, 0
	var t [lines]uint32
	c := make([]byte, lines)
	const digits = "0123456789abcdefghijklmnopqrstuv"
	for i := range t {
		t[i] = load16(spTimes + uintptr(2*i))
		lo, hi = min(lo, t[i]), max(hi, t[i])
		switch d := int(t[i]) - 4; {
		case d < 0:
			c[i] = '-'
		case d < len(digits):
			c[i] = digits[d]
		default:
			c[i] = 'w'
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

// cover reports whether a harness at page offset off, n words long, leaves
// map line i alone.
func cover(off, n, i int) bool { return i < off/16 || i >= (off+4*n+15)/16 }

func event(name string, harnesses [2]uintptr, offs, sizes [2]int, mapAddr, vsync uintptr, mode uint32) {
	var all [lines]int // runs that saw each line taken, of those that can see it
	var seen [lines]int
	for r := 0; r < reps; r++ {
		for k, h := range harnesses {
			run(h, mapAddr, vsync, mode)
			var bits [lines]bool
			lo, hi, chars := taken(&bits)
			if mode == modeControl {
				warm = min(warm, lo)
			}
			if r == 0 {
				println("TIMES", name, "harness", k, chars)
			}
			gap, start := load32(spGap1)&0xffff, load32(spGapS)&0xffff
			n := 0
			for i := range bits {
				if !cover(offs[k], sizes[k], i) {
					continue
				}
				seen[i]++
				if bits[i] {
					all[i]++
					n++
				}
			}
			print("RUN ", name, " rep ", r, " harness ", k, " line time ", lo, " to ", hi, " taken ", n)
			if mode == modeBlank || mode == modeWarm {
				print(" gap ", gap, " start gap ", start)
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
	println("MAP", name, hexMap(&bits), "taken", n, "unsteady", odd)
}

// vsync0 is VSync(0) for the harness to call: the library's VSync can't be
// taken as a value. Its own lines are printed with the map's address.
//
//go:noinline
func vsync0() { yaroze.VSync(0) }

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
	offs := [2]int{0x400, 0xc00}
	var hs [2]uintptr
	var sizes [2]int
	for k, off := range offs {
		p := page + 4096 + uintptr(off)
		sizes[k] = harness(words(p, 256))
		hs[k] = p
	}
	yaroze.FlushCache()

	vs := vsync0
	vsync := (*[2]uintptr)(unsafe.Pointer(&vs))[1]
	println("map", page, "harnesses", hs[0], hs[1], "words", sizes[0], "vsync0", vsync, "line", (vsync>>4)&0xff)

	yaroze.GsInitGraph(320, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	yaroze.VSync(0)
	event("240p control", hs, offs, sizes, page, vsync, modeControl)
	event("240p blank", hs, offs, sizes, page, vsync, modeBlank)
	event("240p warm", hs, offs, sizes, page, vsync, modeWarm)
	event("240p vsync", hs, offs, sizes, page, vsync, modeVSync)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.VSync(0)
	event("480i control", hs, offs, sizes, page, vsync, modeControl)
	event("480i blank", hs, offs, sizes, page, vsync, modeBlank)
	event("480i warm", hs, offs, sizes, page, vsync, modeWarm)
	event("480i vsync", hs, offs, sizes, page, vsync, modeVSync)

	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), saved)
	println("END")
}
