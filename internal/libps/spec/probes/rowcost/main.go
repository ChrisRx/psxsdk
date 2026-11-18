//go:build yaroze

// rowcost measures, for spec 46, what a store to RAM costs a console when it
// goes to another row of the DRAM than the store before it. cpucost5 found a
// run of stores taking turns between two words 4 KB apart at 5.7 cycles a
// store, where a run to one row costs 2, and one such store at the end of a
// run only 1.5 more than a store in the row. The clean sound tick stored each
// envelope it read to one row and its ten pad stores to another, 24 times a
// blank, and cost a console some 4 cycles a read more than the emulator's
// model had it. The model now has a rule for it (emu/cpu/cost.go, wbRowHalves
// and wbRowHoldHalves) fitted to those two rows; this probe's rows say whether
// the rule is right, how big a row is, and what the tick's loop costs in its
// old shape and its new one.
//
// It is cpucost5 with other kernels: machine code the probe writes and calls,
// a loop that reads root counter 2 (system clock), runs the body, reads it
// again and stores the difference, six times, with interrupts off. A is a
// word at a 4 KB boundary of the probe's data, B one 4 KB on.
//
//   - "sw alt +N": 32 stores taking turns between A and A+N, for the row's
//     size: 2 a store while both are in a row, 5.7 once they are not. "sw alt
//     X Y" takes turns between A+X and A+Y, 4 bytes apart across a 1 KB, 2 KB
//     or 4 KB boundary.
//   - "sw A x16" and the runs with one or two changes of row in them, for
//     what one change costs and where: at the run's end, in its middle, at
//     its start after RAM has been idle.
//   - "sw A nopK sw B nopK": pairs of stores K nops apart, to two rows and
//     (the "same" rows) to one, for how close two stores must be to pay.
//   - "read ...": the sound tick's envelope read loop, 8 times: a halfword
//     from the SPU, a nop, the halfword stored, then ten stores to another
//     row ("far", the old loop) or the same ("near"), with a nop between the
//     two ("far nop1"), and the new loop's seven nops and three loads, into
//     one register and into three.
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

// B is A's offset 4 KB on, in another DRAM row whatever the row's size.
const rowB = 4096

func reg(i int) uint32 { return free[i%len(free)] }

// alt is n stores taking turns between offsets x and y.
func alt(n int, x, y int32) func(e *emit) {
	return repeat(n, func(e *emit, i int) {
		off := x
		if i%2 != 0 {
			off = y
		}
		e.put(sw(zero, t0, off))
	})
}

// run is stores to the offsets in offs, in order.
func run(offs ...int32) func(e *emit) {
	return func(e *emit) {
		for _, off := range offs {
			e.put(sw(zero, t0, off))
		}
	}
}

// times16 is 16 offsets: A but for B at the indexes in bs.
func times16(bs ...int) []int32 {
	offs := make([]int32, 16)
	for _, b := range bs {
		offs[b] = rowB
	}
	return offs
}

// span is 16 offsets: A, and B from index from to index to.
func span(from, to int) []int32 {
	offs := make([]int32, 16)
	for i := from; i < to; i++ {
		offs[i] = rowB
	}
	return offs
}

// pairs is 8 pairs of stores k nops apart, to A and to second.
func pairs(k int, second int32) func(e *emit) {
	return repeat(8, func(e *emit, _ int) {
		e.put(sw(zero, t0, 0))
		for range k {
			e.put(nop)
		}
		e.put(sw(zero, t0, second))
		for range k {
			e.put(nop)
		}
	})
}

// read is the sound tick's read loop, 8 times: the envelope read, a nop, its
// store at offset keep, gap nops, and then the pad.
func read(keep int32, gap int, pad func(e *emit)) func(e *emit) {
	return repeat(8, func(e *emit, i int) {
		e.put(lh(v0, t9, 0x1c0c), nop, sh(v0, t0, keep+int32(2*i)))
		for range gap {
			e.put(nop)
		}
		pad(e)
	})
}

func stores10(e *emit) {
	for range 10 {
		e.put(sw(zero, t0, 0))
	}
}

// nopsLoads is the new loop's pad: seven nops and three loads, into one
// register or three.
func nopsLoads(same bool) func(e *emit) {
	return func(e *emit) {
		for range 7 {
			e.put(nop)
		}
		for i := range 3 {
			r := uint32(v1)
			if !same {
				r = reg(i + 1)
			}
			e.put(lw(r, t0, 0))
		}
	}
}

var kernels = []kernel{
	{"empty", 0, func(*emit) {}},
	// The row's size.
	{"sw alt +4", 32, alt(32, 0, 4)},
	{"sw alt +64", 32, alt(32, 0, 64)},
	{"sw alt +256", 32, alt(32, 0, 256)},
	{"sw alt +512", 32, alt(32, 0, 512)},
	{"sw alt +1024", 32, alt(32, 0, 1024)},
	{"sw alt +2048", 32, alt(32, 0, 2048)},
	{"sw alt +4096", 32, alt(32, 0, 4096)},
	{"sw alt 1020 1024", 32, alt(32, 1020, 1024)},
	{"sw alt 2044 2048", 32, alt(32, 2044, 2048)},
	{"sw alt 4092 4096", 32, alt(32, 4092, 4096)},
	// One change of row, and two, in a run of 16.
	{"sw A x16", 16, run(times16()...)},
	{"sw A x15 B", 16, run(times16(15)...)},
	{"sw A x8 B x8", 16, run(span(8, 16)...)},
	{"sw A x8 B A x7", 16, run(times16(8)...)},
	{"sw B A x15", 16, run(times16(0)...)},
	{"sw A x4 B x4 A x4 B x4", 16, run(append(append(append(make([]int32, 4), rowB, rowB, rowB, rowB), 0, 0, 0, 0), rowB, rowB, rowB, rowB)...)},
	// How close two stores must be.
	{"sw A nop1 sw B nop1", 8, pairs(1, rowB)},
	{"sw A nop1 sw A nop1 same", 8, pairs(1, 4)},
	{"sw A nop2 sw B nop2", 8, pairs(2, rowB)},
	{"sw A nop2 sw A nop2 same", 8, pairs(2, 4)},
	{"sw A nop4 sw B nop4", 8, pairs(4, rowB)},
	{"sw A nop4 sw A nop4 same", 8, pairs(4, 4)},
	{"sw A nop8 sw B nop8", 8, pairs(8, rowB)},
	{"sw A nop8 sw A nop8 same", 8, pairs(8, 4)},
	// The sound tick's read loop.
	{"read far", 8, read(rowB, 0, stores10)},
	{"read near", 8, read(64, 0, stores10)},
	{"read far nop1", 8, read(rowB, 1, stores10)},
	{"read nops loads same reg", 8, read(rowB, 0, nopsLoads(true))},
	{"read nops loads three regs", 8, read(rowB, 0, nopsLoads(false))},
}

func call(p uintptr) {
	var f func()
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, p
	f()
}

func main() {
	println("rowcost")
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
