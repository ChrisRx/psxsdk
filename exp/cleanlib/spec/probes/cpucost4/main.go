//go:build yaroze

// cpucost4 measures, for spec 45, what the console's accesses to the SPU and
// to I_STAT cost, which the emulator's cost model takes to cost what a root
// counter's do (a load) and what GP0's do (a store's drain), unmeasured: the
// vertical blank interrupt reads the SPU's envelopes and writes its
// registers and I_STAT, and on a console the clean library's interrupt runs
// some 170 cycles longer than Sony's with the same accesses (probe
// spec/probes/irqsplit). It is cpucost3 with other kernels: machine code the
// probe writes and calls, a loop that reads root counter 2 (system clock),
// runs the body, reads it again and stores the difference, six times, with
// interrupts off. The SPU reads are of voice 0's envelope (1F801C0Ch); the
// SPU writes go to voice 23's pitch (1F801D74h), put back after; the I_STAT
// writes are all ones, which acknowledge nothing.
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

func lui(rt, imm uint32) uint32            { return iType(0x0f, 0, rt, int32(imm)) }
func ori(rt, rs, imm uint32) uint32        { return iType(0x0d, rs, rt, int32(imm)) }
func addiu(rt, rs uint32, i int32) uint32  { return iType(0x09, rs, rt, i) }
func andi(rt, rs, imm uint32) uint32       { return iType(0x0c, rs, rt, int32(imm)) }
func lw(rt, rs uint32, off int32) uint32   { return iType(0x23, rs, rt, off) }
func sw(rt, rs uint32, off int32) uint32   { return iType(0x2b, rs, rt, off) }
func beq(rs, rt uint32, off int32) uint32  { return iType(0x04, rs, rt, off) }
func bne(rs, rt uint32, off int32) uint32  { return iType(0x05, rs, rt, off) }
func addu(rd, rs, rt uint32) uint32        { return rType(rs, rt, rd, 0x21) }
func subu(rd, rs, rt uint32) uint32        { return rType(rs, rt, rd, 0x23) }
func mult(rs, rt uint32) uint32            { return rType(rs, rt, 0, 0x18) }
func div(rs, rt uint32) uint32             { return rType(rs, rt, 0, 0x1a) }
func mflo(rd uint32) uint32                { return rType(0, 0, rd, 0x12) }
func jr(rs uint32) uint32                  { return rs<<21 | 0x08 }
func mtc2(rt, rd uint32) uint32            { return 0x48800000 | rt<<16 | rd<<11 }
func mfc2(rt, rd uint32) uint32            { return 0x48000000 | rt<<16 | rd<<11 }
func lwc2(rt, rs uint32, off int32) uint32 { return iType(0x32, rs, rt, off) }
func swc2(rt, rs uint32, off int32) uint32 { return iType(0x3a, rs, rt, off) }

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

func lh(rt, rs uint32, off int32) uint32 { return iType(0x21, rs, rt, off) }
func sh(rt, rs uint32, off int32) uint32 { return iType(0x29, rs, rt, off) }

const (
	spuEnv   = 0x1c0c // voice 0's envelope
	spuPitch = 0x1d74 // voice 23's pitch
	iStat    = 0x1070
)

// ones puts all ones in t6 for the I_STAT writes, as a pattern's first
// instruction.
func ones(e *emit) { e.put(addiu(t6, zero, -1)) }

var kernels = []kernel{
	{"empty", 0, false, func(*emit) {}},
	{"ones", 1, false, ones},
	{"lw ram", 32, false, repeat(32, func(e *emit, i int) { e.put(lw(free[i%len(free)], t0, int32(4*i))) })},
	{"lh spu", 32, false, repeat(32, func(e *emit, i int) { e.put(lh(free[i%len(free)], t9, spuEnv)) })},
	{"lh spu nop", 32, false, repeat(32, func(e *emit, i int) { e.put(lh(free[i%len(free)], t9, spuEnv), nop) })},
	{"lw istat", 32, false, repeat(32, func(e *emit, i int) { e.put(lw(free[i%len(free)], t9, iStat)) })},
	{"sh spu", 32, false, repeat(32, func(e *emit, i int) { e.put(sh(zero, t9, spuPitch)) })},
	{"sh spu nop8", 16, false, repeat(16, func(e *emit, i int) { e.put(sh(zero, t9, spuPitch), nop, nop, nop, nop, nop, nop, nop, nop) })},
	{"sw istat", 32, false, func(e *emit) { ones(e); repeat(32, func(e *emit, i int) { e.put(sw(t6, t9, iStat)) })(e) }},
	{"sw istat nop8", 16, false, func(e *emit) {
		ones(e)
		repeat(16, func(e *emit, i int) { e.put(sw(t6, t9, iStat), nop, nop, nop, nop, nop, nop, nop, nop) })(e)
	}},
	{"sh spu lw ram", 16, false, repeat(16, func(e *emit, i int) { e.put(sh(zero, t9, spuPitch), lw(free[i%len(free)], t0, int32(4*i))) })},
	{"sh spu lh spu", 16, false, repeat(16, func(e *emit, i int) { e.put(sh(zero, t9, spuPitch), lh(free[i%len(free)], t9, spuEnv)) })},
	{"sh spu nop8 lw ram", 16, false, repeat(16, func(e *emit, i int) {
		e.put(sh(zero, t9, spuPitch), nop, nop, nop, nop, nop, nop, nop, nop, lw(free[i%len(free)], t0, int32(4*i)))
	})},
	{"sw istat lw ram", 16, false, func(e *emit) {
		ones(e)
		repeat(16, func(e *emit, i int) { e.put(sw(t6, t9, iStat), lw(free[i%len(free)], t0, int32(4*i))) })(e)
	}},
	{"sw ram lh spu", 16, false, repeat(16, func(e *emit, i int) { e.put(sw(zero, t0, int32(4*i)), lh(free[i%len(free)], t9, spuEnv)) })},
	{"sw ram lw istat", 16, false, repeat(16, func(e *emit, i int) { e.put(sw(zero, t0, int32(4*i)), lw(free[i%len(free)], t9, iStat)) })},
	{"sw ram x4 lh spu", 8, false, repeat(8, func(e *emit, i int) {
		e.put(sw(zero, t0, 0), sw(zero, t0, 4), sw(zero, t0, 8), sw(zero, t0, 12), lh(free[i%len(free)], t9, spuEnv))
	})},
}

func call(p uintptr) {
	var f func()
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, p
	f()
}

func main() {
	println("cpucost4")
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
	pitch := (*uint16)(yaroze.Pointer(0x1f801d74))
	savedPitch := volatile.LoadUint16(pitch)
	mode := (*uint32)(yaroze.Pointer(0x1f801124))
	saved := volatile.LoadUint32(mode)
	yaroze.EnterCriticalSection()
	volatile.StoreUint32(mode, 0) // system clock, free running
	for i := range kernels {
		call(entries[i])
	}
	volatile.StoreUint32(mode, saved)
	volatile.StoreUint16(pitch, savedPitch)
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
