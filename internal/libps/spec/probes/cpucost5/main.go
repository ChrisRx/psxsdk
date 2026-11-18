//go:build yaroze

// cpucost5 measures, for spec 46, what the console charges for the kinds of
// access the clean vertical blank handler makes and the cost model has not
// been measured on: accesses that change DRAM row between one and the next
// (the handler's pads store to one word, spacer, between its work on the
// voices' shadow, its stack and the SPU, where a run of stores to one row is
// what cpucost measured), halfword and byte loads and stores to RAM (the
// voices' shadow is halfwords and bytes), loads from the DMA controller's
// and I_MASK's registers (the clean handler reads DICR twice an interrupt
// where Sony's reads I_STAT and I_MASK), and a call and return through
// jal and jr. It is cpucost4 with other kernels: machine code the probe
// writes and calls, a loop that reads root counter 2 (system clock), runs
// the body, reads it again and stores the difference, six times, with
// interrupts off. "rows" patterns take turns between data[0] and data[1024],
// 4 KB apart, which is in another DRAM row whatever the row's size; "same
// row" between data[0] and data[1], and "same word" to data[0] every time.
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
	data    [2048]uint32
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
	d := addr(&data[0])
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

// Offsets into data: the same row (consecutive words) and another row, 4 KB on.
const (
	rowA = 0
	rowB = 4096
)

func reg(i int) uint32 { return free[i%len(free)] }

// rows gives offset rowA or rowB by turns.
func rows(i int) int32 {
	if i%2 == 0 {
		return rowA
	}
	return rowB
}

// jalRet writes a stub (jr ra; nop) jumped over, and n calls of it, with
// the kernel's own return address kept in t6 meanwhile.
func jalRet(n int) func(e *emit) {
	return func(e *emit) {
		e.put(rType(ra, zero, t6, 0x21), beq(zero, zero, 2), nop)
		stub := e.pc()
		e.put(jr(ra), nop)
		for range n {
			e.put(jal(stub), nop)
		}
		e.put(rType(t6, zero, ra, 0x21))
	}
}

const (
	iMask = 0x1074
	dicr  = 0x10f4
	dpcr  = 0x10f0
)

var kernels = []kernel{
	{"empty", 0, func(*emit) {}},
	// Loads, as cpucost measured them (consecutive words), and across rows.
	{"lw ram", 32, repeat(32, func(e *emit, i int) { e.put(lw(reg(i), t0, int32(4*i))) })},
	{"lw ram rows", 32, repeat(32, func(e *emit, i int) { e.put(lw(reg(i), t0, rows(i)+int32(4*(i/2)))) })},
	{"lw ram same word", 32, repeat(32, func(e *emit, i int) { e.put(lw(reg(i), t0, 0)) })},
	{"lhu ram", 32, repeat(32, func(e *emit, i int) { e.put(lhu(reg(i), t0, int32(2*i))) })},
	{"lh ram", 32, repeat(32, func(e *emit, i int) { e.put(lh(reg(i), t0, int32(2*i))) })},
	{"lbu ram", 32, repeat(32, func(e *emit, i int) { e.put(lbu(reg(i), t0, int32(i))) })},
	// Stores in a run: consecutive words, one word, across rows, halfwords, bytes.
	{"sw ram", 32, repeat(32, func(e *emit, i int) { e.put(sw(zero, t0, int32(4*i))) })},
	{"sw ram same word", 32, repeat(32, func(e *emit, i int) { e.put(sw(zero, t0, 0)) })},
	{"sw ram rows", 32, repeat(32, func(e *emit, i int) { e.put(sw(zero, t0, rows(i)+int32(4*(i/2)))) })},
	{"sh ram", 32, repeat(32, func(e *emit, i int) { e.put(sh(zero, t0, int32(2*i))) })},
	{"sb ram", 32, repeat(32, func(e *emit, i int) { e.put(sb(zero, t0, int32(i))) })},
	// A store then a load, in the same row and in another.
	{"sw then lw same row", 16, repeat(16, func(e *emit, i int) { e.put(sw(zero, t0, int32(8*i)), lw(reg(i), t0, int32(8*i+4))) })},
	{"sw then lw rows", 16, repeat(16, func(e *emit, i int) { e.put(sw(zero, t0, int32(4*i)), lw(reg(i), t0, rowB+int32(4*i))) })},
	{"lw then sw same row", 16, repeat(16, func(e *emit, i int) { e.put(lw(reg(i), t0, int32(8*i)), sw(zero, t0, int32(8*i+4))) })},
	{"lw then sw rows", 16, repeat(16, func(e *emit, i int) { e.put(lw(reg(i), t0, int32(4*i)), sw(zero, t0, rowB+int32(4*i))) })},
	// The pads' shape: stores to one word, then a load from elsewhere.
	{"sw x4 same word lw rows", 8, repeat(8, func(e *emit, i int) {
		e.put(sw(zero, t0, 0), sw(zero, t0, 0), sw(zero, t0, 0), sw(zero, t0, 0), lw(reg(i), t0, rowB+int32(4*i)))
	})},
	{"sw x4 same word lw same row", 8, repeat(8, func(e *emit, i int) {
		e.put(sw(zero, t0, 0), sw(zero, t0, 0), sw(zero, t0, 0), sw(zero, t0, 0), lw(reg(i), t0, int32(4+4*i)))
	})},
	{"sw x10 same word lh spu", 8, repeat(8, func(e *emit, i int) {
		for range 10 {
			e.put(sw(zero, t0, 0))
		}
		e.put(lh(reg(i), t9, 0x1c0c))
	})},
	{"sw x10 same word sh ram lh spu", 8, repeat(8, func(e *emit, i int) {
		for range 10 {
			e.put(sw(zero, t0, 0))
		}
		e.put(sh(zero, t0, rowB+int32(2*i)), lh(reg(i), t9, 0x1c0c))
	})},
	// Loads from the interrupt and DMA controllers' other registers.
	{"lw imask", 32, repeat(32, func(e *emit, i int) { e.put(lw(reg(i), t9, iMask)) })},
	{"lw dicr", 32, repeat(32, func(e *emit, i int) { e.put(lw(reg(i), t9, dicr)) })},
	{"lw dpcr", 32, repeat(32, func(e *emit, i int) { e.put(lw(reg(i), t9, dpcr)) })},
	{"lw istat", 32, repeat(32, func(e *emit, i int) { e.put(lw(reg(i), t9, 0x1070)) })},
	// A call and its return, the target cached.
	{"jal ret", 16, jalRet(16)},
}

func call(p uintptr) {
	var f func()
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, p
	f()
}

func main() {
	println("cpucost5")
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
