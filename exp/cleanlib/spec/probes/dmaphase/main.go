//go:build yaroze

// dmaphase maps, for spec 54, when an access to the bus waits while an
// image goes to or from the GPU by DMA, and for how long. dmahold (spec 50)
// timed runs of one kind of access, which a model with the bus open a fixed
// part of each block fits for an image going to the GPU and doesn't for one
// coming from it.
//
// It is dmahold's frame (see there). A row's pre starts channel 2 on 64
// blocks from the GPU, as StoreImage does, or to it, as LoadImage does. The
// body, timed, is d nops, which run from the instruction cache and touch no
// bus, then one access, then 16 nops. A row with no access gives the same
// d's time without it, so the difference is what the one access cost, d
// cycles after the counter's first read.
//
// It prints each row's name, its count and the six times, then END, and
// returns.
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
func subu(rd, rs, rt uint32) uint32       { return rType(rs, rt, rd, 0x23) }
func and(rd, rs, rt uint32) uint32        { return rType(rs, rt, rd, 0x24) }
func jr(rs uint32) uint32                 { return rs<<21 | 0x08 }

const nop = 0

// I/O ports, from 1F800000h.
const (
	iStat  = 0x1070
	d2MADR = 0x10a0
	d2BCR  = 0x10a4
	d2CHCR = 0x10a8
	timer2 = 0x1120
	gp0    = 0x1810
	gp1    = 0x1814
)

type kernel struct {
	name string
	n    int
	pre  func(e *emit) // before the first counter read
	body func(e *emit)
	post func(e *emit) // after the time is stored
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
	codeBuf [80 * 1024]uint32
	data    [4096 + 1024]uint32
	times   [320][reps]uint32
	entries [320]uintptr
)

func addr[T any](p *T) uint32 { return uint32(uintptr(unsafe.Pointer(p))) }

// aligned returns buf from its first 16-byte aligned word.
func aligned(buf []uint32) []uint32 {
	for i := range buf {
		if uintptr(unsafe.Pointer(&buf[i]))%16 == 0 {
			return buf[i:]
		}
	}
	return buf
}

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
		addiu(t7, zero, reps),
	)
	loop := e.at
	if k.pre != nil {
		k.pre(e)
	}
	e.put(lw(a0, t9, timer2))
	k.body(e)
	e.put(
		lw(a1, t9, timer2),
		nop,
		subu(a1, a1, a0),
		andi(a1, a1, 0xffff),
		sw(a1, t8, 0),
		addiu(t8, t8, 4),
	)
	if k.post != nil {
		k.post(e)
	}
	e.put(addiu(t7, t7, -1))
	e.put(bne(t7, zero, int32(loop-(e.at+1))), nop)
	e.put(jr(ra), nop)
	return uintptr(unsafe.Pointer(&e.code[start]))
}

// set puts v in r.
func set(e *emit, r, v uint32) { e.put(lui(r, v>>16), ori(r, r, v&0xffff)) }

// out writes v to the port at off.
func out(e *emit, off int32, v uint32) {
	set(e, v0, v)
	e.put(sw(v0, t9, off))
}

// waitBit polls the port at off until its bits in mask are all clear
// (want false) or not all clear (want true), a million times at most, so
// that the probe returns whatever the hardware does.
func waitBit(e *emit, off int32, mask uint32, want bool) {
	set(e, v1, mask)
	e.put(lui(t6, 0x10))
	l := e.at
	e.put(lw(v0, t9, off), addiu(t6, t6, -1), and(v0, v0, v1), beq(t6, zero, 3), nop)
	if want {
		e.put(beq(v0, zero, int32(l-(e.at+1))), nop)
	} else {
		e.put(bne(v0, zero, int32(l-(e.at+1))), nop)
	}
}

// size is the GPU's size word for an image of n blocks of 16 words: 32
// pixels wide, n high.
func size(n int) uint32 { return uint32(n)<<16 | 32 }

const place = 0<<16 | 640

// load starts channel 2 on an image of n blocks to the GPU, as LoadImage
// does.
func load(n int) func(e *emit) {
	return func(e *emit) {
		waitBit(e, gp1, 1<<26, true)
		out(e, gp1, 0x04000000)
		out(e, gp0, 0x01000000)
		out(e, gp0, 0xa0000000)
		out(e, gp0, place)
		out(e, gp0, size(n))
		out(e, gp1, 0x04000002)
		out(e, d2MADR, addr(&data[0])&0xffffff)
		out(e, d2BCR, uint32(n)<<16|16)
		out(e, d2CHCR, 0x01000201)
	}
}

// store starts channel 2 on an image of n blocks from the GPU, as
// StoreImage does.
func store(n int) func(e *emit) {
	return func(e *emit) {
		waitBit(e, gp1, 1<<26, true)
		out(e, gp1, 0x04000000)
		out(e, gp0, 0x01000000)
		out(e, gp0, 0xc0000000)
		out(e, gp0, place)
		out(e, gp0, size(n))
		waitBit(e, gp1, 1<<27, true)
		out(e, gp1, 0x04000003)
		out(e, d2MADR, addr(&data[0])&0xffffff)
		out(e, d2BCR, uint32(n)<<16|16)
		out(e, d2CHCR, 0x01000200)
	}
}

// done waits for the channel to end and the GPU to be ready for a command,
// and turns the GPU's DMA off.
func done(e *emit) {
	waitBit(e, d2CHCR, 1<<24, false)
	waitBit(e, gp1, 1<<26, true)
	out(e, gp1, 0x04000000)
}

func run(n int, w ...uint32) func(e *emit) {
	return func(e *emit) {
		for range n {
			e.put(w...)
		}
	}
}

// loads is n loads from RAM into registers in turn.
func loads(n int) func(e *emit) {
	return func(e *emit) {
		for i := range n {
			e.put(lw(free[i%len(free)], t0, 0))
		}
	}
}

// spin is n turns of a loop of a load, an add and a store, as a program
// waits: about the loop loadsync's wait is.
func spin(n int32) func(e *emit) {
	return func(e *emit) {
		e.put(addiu(t3, zero, n))
		l := e.at
		e.put(lw(t4, t0, 0), addiu(t3, t3, -1), addiu(t4, t4, 1), sw(t4, t0, 0))
		e.put(bne(t3, zero, int32(l-(e.at+1))), nop)
	}
}

func alone(e *emit) { waitBit(e, d2CHCR, 1<<24, false) }

const sp = 29

func loadsStack(e *emit) {
	for range 16 {
		e.put(lw(v0, sp, -4096))
	}
}

func rType2(rs, rt, fn uint32) uint32 { return rs<<21 | rt<<16 | fn }

type body struct {
	name string
	n    int
	f    func(e *emit)
}

// accesses are the one access each row makes, or none.
var accesses = []struct {
	name string
	w    []uint32
}{
	{"none", nil},
	{"lw ram", []uint32{lw(v0, t0, 0)}},
	{"lw gpustat", []uint32{lw(v0, t9, gp1)}},
	{"sw lw", []uint32{sw(zero, t0, 4), lw(v0, t0, 0)}},
}

// phase is d nops, the access, and 16 nops.
func phase(d int, w []uint32) func(e *emit) {
	return func(e *emit) {
		run(d, nop)(e)
		e.put(w...)
		run(16, nop)(e)
	}
}

var kernels []kernel

func itoa(v uint32) string {
	if v == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for ; v != 0; v /= 10 {
		i--
		b[i] = byte('0' + v%10)
	}
	return string(b[i:])
}

func call(p uintptr) {
	var f func()
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, p
	f()
}

func main() {
	println("dmaphase")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	yaroze.VSync(0)

	for _, x := range []struct {
		name string
		pre  func(e *emit)
	}{{"store", store(64)}, {"load", load(64)}} {
		for d := 0; d <= 70; d += 2 {
			for _, a := range accesses {
				kernels = append(kernels, kernel{name: x.name + " " + a.name, n: d, pre: x.pre, body: phase(d, a.w), post: done})
			}
		}
	}
	if len(kernels) > len(times) {
		println("too many kernels", len(kernels))
		println("END")
		return
	}
	e := &emit{code: aligned(codeBuf[:])}
	for i := range kernels {
		entries[i] = build(e, i, &kernels[i])
	}
	print("code words ", e.at, " of ", len(e.code), "\n")
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
