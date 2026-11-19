//go:build yaroze

// padcost48 measures, for spec 48, two things the emulator's cost model
// (emu/cpu/cost.go) has from no console row:
//
//   - The clean library's new pads: a load from RAM into $zero with a nop after
//     it ("lw zero nop", LOADS), loads into $zero in a run ("lw zero"), a run of
//     loads into registers in turn ("lw run", LOADRUN and LOADCYC), against the
//     same with a real register ("lw v1 nop") and nops alone. The model has 8, 7,
//     6, 8 and 1 cycles a pattern.
//   - A multiply's time by the size of its operands. cpucost's "mult mflo" (7.8
//     for the two) had small operands; the R3000A is said to take longer for
//     larger ones, by the first operand (rs). Each row is 8 of "mult a2, a3"
//     then "mflo v0", with a2 and a3 set before the timing, so that rs and rt
//     each go from small to the widest, by sign too, and multu with the top bit
//     set. The model has 8 a pair for all of them. Under the model with time by
//     rs (6, 9 or 13 cycles for rs within 11, 20 or 32 bits), Sony's GsSortSprite
//     rotated and RotMatrix come out at the console's figures (155 and 34
//     eighths, the console 156 and 34), where now they are 151 and 28.
//
// It is rowcost with these kernels: machine code the probe writes and calls, a
// loop that reads root counter 2 (system clock), runs the body, reads it again
// and stores the difference, six times, with interrupts off. t0 is a word of
// RAM, the probe's data.
//
// It prints, for each kernel, its name, how many times its pattern is
// repeated, and the six times (the first with the body's lines out of the
// instruction cache), then END, and returns. "empty" is the counter read
// alone: subtract it, and divide by the count, for a pattern's cost.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const reps = 6

// Registers.
const (
	zero = 0
	v0   = 2
	v1   = 3
	a0   = 4
	a1   = 5
	a2   = 6
	a3   = 7
	t0   = 8 // RAM data
	t2   = 10
	t3   = 11
	t4   = 12
	t5   = 13
	t6   = 14
	t7   = 15 // the loop's count
	t8   = 24 // where the times go
	t9   = 25 // 1F800000h: the scratchpad and I/O
	ra   = 31
)

// The body may use these freely.
var free = [...]uint32{v0, v1, a2, a3, t3, t4, t5, t6}

// Instructions.
func iType(op, rs, rt uint32, imm int32) uint32 {
	return op<<26 | rs<<21 | rt<<16 | uint32(imm)&0xffff
}
func rType(rs, rt, rd, fn uint32) uint32 { return rs<<21 | rt<<16 | rd<<11 | fn }

func lui(rt, imm uint32) uint32           { return iType(0x0f, 0, rt, int32(imm)) }
func ori(rt, rs, imm uint32) uint32       { return iType(0x0d, rs, rt, int32(imm)) }
func addiu(rt, rs uint32, i int32) uint32 { return iType(0x09, rs, rt, i) }
func andi(rt, rs, imm uint32) uint32      { return iType(0x0c, rs, rt, int32(imm)) }
func lw(rt, rs uint32, off int32) uint32  { return iType(0x23, rs, rt, off) }
func lh(rt, rs uint32, off int32) uint32  { return iType(0x21, rs, rt, off) }
func lhu(rt, rs uint32, off int32) uint32 { return iType(0x25, rs, rt, off) }
func lbu(rt, rs uint32, off int32) uint32 { return iType(0x24, rs, rt, off) }
func sw(rt, rs uint32, off int32) uint32  { return iType(0x2b, rs, rt, off) }
func sh(rt, rs uint32, off int32) uint32  { return iType(0x29, rs, rt, off) }
func sb(rt, rs uint32, off int32) uint32  { return iType(0x28, rs, rt, off) }
func beq(rs, rt uint32, off int32) uint32 { return iType(0x04, rs, rt, off) }
func bne(rs, rt uint32, off int32) uint32 { return iType(0x05, rs, rt, off) }
func subu(rd, rs, rt uint32) uint32       { return rType(rs, rt, rd, 0x23) }
func jr(rs uint32) uint32                 { return rs<<21 | 0x08 }
func jal(target uint32) uint32            { return 3<<26 | target>>2&0x3ffffff }

const nop = 0

type kernel struct {
	name string
	n    int // times the pattern is repeated
	body func(e *emit)
	pre  []uint32 // before the loop, untimed
}

type emit struct {
	code []uint32
	at   int
}

func (e *emit) put(w ...uint32) {
	for _, x := range w {
		e.code[e.at] = x
		e.at++
	}
}

func (e *emit) pc() uint32 { return uint32(uintptr(unsafe.Pointer(&e.code[e.at]))) }

var (
	code    [24 * 1024]uint32
	data    [5 * 1024]uint32
	times   [64][reps]uint32
	entries [64]uintptr
)

func addr[T any](p *T) uint32 { return uint32(uintptr(unsafe.Pointer(p))) }

// build writes kernel k's code at e's position, 16-byte aligned, timing
// into times[i].
func build(e *emit, i int, k *kernel) uintptr {
	for e.at%4 != 0 {
		e.put(nop)
	}
	start := e.at
	out := addr(&times[i][0])
	d := (addr(&data[0]) + 4095) &^ 4095
	e.put(
		lui(t8, out>>16), ori(t8, t8, out&0xffff),
		lui(t0, d>>16), ori(t0, t0, d&0xffff),
		lui(t9, 0x1f80),
		lui(t2, 0xbfc0),
		addiu(t3, zero, 1000),
		addiu(t4, zero, 7),
		addiu(t7, zero, reps),
	)
	e.put(k.pre...)
	loop := e.at
	e.put(lw(a0, t9, 0x1120))
	k.body(e)
	e.put(
		lw(a1, t9, 0x1120),
		nop,
		subu(a1, a1, a0),
		andi(a1, a1, 0xffff),
		sw(a1, t8, 0),
		addiu(t8, t8, 4),
		addiu(t7, t7, -1),
	)
	e.put(bne(t7, zero, int32(loop-(e.at+1))), nop)
	e.put(jr(ra), nop)
	return uintptr(unsafe.Pointer(&e.code[start]))
}

func repeat(n int, f func(e *emit, i int)) func(e *emit) {
	return func(e *emit) {
		for i := range n {
			f(e, i)
		}
	}
}

func reg(i int) uint32 { return free[i%len(free)] }

func mult(rs, rt uint32) uint32  { return rType(rs, rt, 0, 0x18) }
func multu(rs, rt uint32) uint32 { return rType(rs, rt, 0, 0x19) }
func mflo(rd uint32) uint32      { return rType(0, 0, rd, 0x12) }

// set loads r with v.
func set(r, v uint32) []uint32 { return []uint32{lui(r, v>>16), ori(r, r, v&0xffff)} }

// operands sets a2 (rs) and a3 (rt).
func operands(rs, rt uint32) []uint32 { return append(set(a2, rs), set(a3, rt)...) }

// mults is n of mult (or multu) a2, a3 and mflo v0.
func mults(n int, unsigned bool) func(e *emit) {
	return repeat(n, func(e *emit, _ int) {
		op := mult(a2, a3)
		if unsigned {
			op = multu(a2, a3)
		}
		e.put(op, mflo(v0))
	})
}

func pairsOf(n int, a, b uint32) func(e *emit) {
	return repeat(n, func(e *emit, _ int) { e.put(a, b) })
}

var kernels = []kernel{
	{name: "empty", body: func(*emit) {}},
	// The pads.
	{"nop", 32, repeat(32, func(e *emit, _ int) { e.put(nop) }), nil},
	{"lw zero nop", 16, pairsOf(16, lw(zero, t0, 0), nop), nil},
	{"lw v1 nop", 16, pairsOf(16, lw(v1, t0, 0), nop), nil},
	{"lw zero", 16, repeat(16, func(e *emit, _ int) { e.put(lw(zero, t0, 0)) }), nil},
	{"lw run", 16, repeat(16, func(e *emit, i int) { e.put(lw(reg(i), t0, 0)) }), nil},
	{"lw run then nop", 16, func(e *emit) {
		repeat(16, func(e *emit, i int) { e.put(lw(reg(i), t0, 0)) })(e)
		e.put(nop)
	}, nil},
	// Multiplies by operand size: rs, then rt.
	{"mult rs 7 rt 7", 8, mults(8, false), operands(7, 7)},
	{"mult rs 7ff rt 7", 8, mults(8, false), operands(0x7ff, 7)},
	{"mult rs 800 rt 7", 8, mults(8, false), operands(0x800, 7)},
	{"mult rs fffff rt 7", 8, mults(8, false), operands(0xfffff, 7)},
	{"mult rs 100000 rt 7", 8, mults(8, false), operands(0x100000, 7)},
	{"mult rs 7fffffff rt 7", 8, mults(8, false), operands(0x7fffffff, 7)},
	{"mult rs -800 rt 7", 8, mults(8, false), operands(0xfffff800, 7)},
	{"mult rs -801 rt 7", 8, mults(8, false), operands(0xfffff7ff, 7)},
	{"mult rs -100000 rt 7", 8, mults(8, false), operands(0xfff00000, 7)},
	{"mult rs -100001 rt 7", 8, mults(8, false), operands(0xffefffff, 7)},
	{"mult rs 7 rt 800", 8, mults(8, false), operands(7, 0x800)},
	{"mult rs 7 rt 100000", 8, mults(8, false), operands(7, 0x100000)},
	{"mult rs 7 rt 7fffffff", 8, mults(8, false), operands(7, 0x7fffffff)},
	{"mult rs 1000 rt 1000", 8, mults(8, false), operands(0x1000, 0x1000)},
	{"multu rs 7ff rt 7", 8, mults(8, true), operands(0x7ff, 7)},
	{"multu rs 800 rt 7", 8, mults(8, true), operands(0x800, 7)},
	{"multu rs 100000 rt 7", 8, mults(8, true), operands(0x100000, 7)},
	{"multu rs 80000000 rt 7", 8, mults(8, true), operands(0x80000000, 7)},
	{"multu rs ffffffff rt 7", 8, mults(8, true), operands(0xffffffff, 7)},
}

func call(p uintptr) {
	var f func()
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, p
	f()
}

func main() {
	println("padcost48")
	yaroze.ResetGraph(0)
	if len(kernels) > len(times) {
		println("too many kernels")
		return
	}
	e := &emit{code: code[:]}
	for i := range kernels {
		entries[i] = build(e, i, &kernels[i])
	}
	print("code words ", e.at, " of ", len(code), "\n")
	if e.at > len(code) {
		return
	}

	yaroze.FlushCache()
	mode := (*uint32)(yaroze.Pointer(0x1f801124))
	saved := volatile.LoadUint32(mode)
	yaroze.EnterCriticalSection()
	volatile.StoreUint32(mode, 0) // system clock, free running
	for i := range kernels {
		call(entries[i])
	}
	volatile.StoreUint32(mode, saved)
	yaroze.ExitCriticalSection()

	for i, k := range kernels {
		print(k.name, " n ", k.n, ":")
		for _, t := range times[i] {
			print(" ", t)
		}
		println()
	}
	println("END")
}
