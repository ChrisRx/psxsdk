//go:build yaroze

// cpucost measures what the CPU's instructions and memory accesses cost, for
// the emulator's cost model (spec 35, "The emulator's cost model is far from
// a console's"). No library code is timed: each kernel is machine code the
// probe writes into RAM and calls, a loop that reads root counter 2 (system
// clock), runs the kernel's body, reads it again and stores the difference,
// six times. The first time is with the body's lines out of the instruction
// cache (cold), the rest with them in, unless the body is larger than the
// cache or runs uncached (through KSEG1).
//
// The kernels follow the C calling convention: they use only registers a
// caller doesn't expect kept (at is left alone), and return with jr ra, so
// the probe calls each through a func value. Interrupts are off around
// each call. It prints, for each kernel, its name, how many times its
// pattern is repeated, and the six times, then END, and returns.
//
// Each body's time includes the second counter read. "empty" is that alone:
// subtract it, and divide by the count, for a pattern's cost.
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
func sw(rt, rs uint32, off int32) uint32  { return iType(0x2b, rs, rt, off) }
func beq(rs, rt uint32, off int32) uint32 { return iType(0x04, rs, rt, off) }
func bne(rs, rt uint32, off int32) uint32 { return iType(0x05, rs, rt, off) }
func addu(rd, rs, rt uint32) uint32       { return rType(rs, rt, rd, 0x21) }
func subu(rd, rs, rt uint32) uint32       { return rType(rs, rt, rd, 0x23) }
func mult(rs, rt uint32) uint32           { return rType(rs, rt, 0, 0x18) }
func div(rs, rt uint32) uint32            { return rType(rs, rt, 0, 0x1a) }
func mflo(rd uint32) uint32               { return rType(0, 0, rd, 0x12) }
func jr(rs uint32) uint32                 { return rs<<21 | 0x08 }
func mtc2(rt, rd uint32) uint32           { return 0x48800000 | rt<<16 | rd<<11 }
func mfc2(rt, rd uint32) uint32           { return 0x48000000 | rt<<16 | rd<<11 }

const nop = 0

// GTE commands, as the clean library's target assembly has them.
const (
	rtps  = 0x4a180001
	rtpt  = 0x4a280030
	nclip = 0x4b400006
	avsz3 = 0x4b58002d
	ncct  = 0x4b18043f
)

type kernel struct {
	name     string
	n        int  // times the pattern is repeated
	uncached bool // called through KSEG1
	body     func(e *emit)
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
	p := uintptr(unsafe.Pointer(&e.code[start]))
	if k.uncached {
		p |= 0x20000000
	}
	return p
}

func repeat(n int, f func(e *emit, i int)) func(e *emit) {
	return func(e *emit) {
		for i := range n {
			f(e, i)
		}
	}
}

func nops(n int) func(e *emit) { return repeat(n, func(e *emit, _ int) { e.put(nop) }) }

func stores(n int) func(e *emit) {
	return repeat(n, func(e *emit, i int) { e.put(sw(zero, t0, int32(4*i))) })
}

func gte(cmd, reg uint32) func(e *emit) {
	return repeat(8, func(e *emit, _ int) { e.put(cmd, mfc2(v0, reg), nop) })
}

var kernels = []kernel{
	{"empty", 0, false, func(*emit) {}},
	{"nop", 256, false, nops(256)},
	{"nop uncached", 256, true, nops(256)},
	{"nop 896", 896, false, nops(896)},
	{"nop 2048", 2048, false, nops(2048)},
	{"lw ram", 64, false, repeat(64, func(e *emit, i int) { e.put(lw(free[i%len(free)], t0, int32(4*i))) })},
	{"lw ram then use", 64, false, repeat(64, func(e *emit, i int) { e.put(lw(v0, t0, int32(4*i)), addu(v1, v0, v0)) })},
	{"lw ram nop", 64, false, repeat(64, func(e *emit, i int) { e.put(lw(v0, t0, int32(4*i)), nop) })},
	{"sw ram 1", 1, false, stores(1)},
	{"sw ram 2", 2, false, stores(2)},
	{"sw ram 4", 4, false, stores(4)},
	{"sw ram 8", 8, false, stores(8)},
	{"sw ram 16", 16, false, stores(16)},
	{"sw ram 32", 32, false, stores(32)},
	{"sw ram 64", 64, false, stores(64)},
	{"sw ram 128", 128, false, stores(128)},
	{"sw ram nop", 64, false, repeat(64, func(e *emit, i int) { e.put(sw(zero, t0, int32(4*i)), nop) })},
	{"sw ram nop3", 64, false, repeat(64, func(e *emit, i int) { e.put(sw(zero, t0, int32(4*i)), nop, nop, nop) })},
	{"sw then lw ram", 32, false, repeat(32, func(e *emit, i int) { e.put(sw(zero, t0, int32(8*i)), lw(v0, t0, int32(8*i+4))) })},
	{"lw scratch", 64, false, repeat(64, func(e *emit, i int) { e.put(lw(free[i%len(free)], t9, int32(4*i))) })},
	{"sw scratch", 64, false, repeat(64, func(e *emit, i int) { e.put(sw(zero, t9, int32(4*i))) })},
	{"lw gpustat", 16, false, repeat(16, func(e *emit, _ int) { e.put(lw(v0, t9, 0x1814)) })},
	{"sw gp0 nop", 16, false, repeat(16, func(e *emit, _ int) { e.put(sw(zero, t9, 0x1810)) })},
	{"lw timer", 16, false, repeat(16, func(e *emit, _ int) { e.put(lw(v0, t9, 0x1120)) })},
	{"lw rom", 16, false, repeat(16, func(e *emit, i int) { e.put(lw(v0, t2, int32(0x100+4*i))) })},
	{"mult mflo", 16, false, repeat(16, func(e *emit, _ int) { e.put(mult(t3, t4), mflo(v0)) })},
	{"div mflo", 16, false, repeat(16, func(e *emit, _ int) { e.put(div(t3, t4), mflo(v0)) })},
	{"mtc2", 16, false, repeat(16, func(e *emit, _ int) { e.put(mtc2(t3, 0)) })},
	{"mfc2 nop", 16, false, repeat(16, func(e *emit, _ int) { e.put(mfc2(v0, 7), nop) })},
	{"rtps mfc2", 8, false, gte(rtps, 14)},
	{"rtpt mfc2", 8, false, gte(rtpt, 14)},
	{"nclip mfc2", 8, false, gte(nclip, 24)},
	{"avsz3 mfc2", 8, false, gte(avsz3, 7)},
	{"ncct mfc2", 8, false, gte(ncct, 22)},
	{"branch taken", 64, false, repeat(64, func(e *emit, _ int) { e.put(beq(zero, zero, 1), nop, nop) })},
	{"branch not taken", 64, false, repeat(64, func(e *emit, _ int) { e.put(bne(zero, zero, 1), nop, nop) })},
}

func call(p uintptr) {
	var f func()
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, p
	f()
}

func main() {
	println("cpucost")
	yaroze.ResetGraph(0)
	yaroze.GsInit3D()
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
