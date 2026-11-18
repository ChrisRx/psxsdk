//go:build yaroze

// cpucost2 follows cpucost (spec 35) with what the emulator's cost model
// needs next, timed the same way, with machine code the probe writes and
// calls, six times each, interrupts off:
//
//   - how the instruction cache maps addresses: two blocks of code run in
//     turn, 2, 4, 6 and 8 KB apart, which fit in the cache together or
//     evict each other;
//   - what filling a line costs, with every run cold (the cache emptied by
//     running 4 KB of other code after each): whole lines, two
//     instructions of each line entered at its start, and two entered
//     8 bytes in;
//   - loads from RAM at one address and 128 bytes apart;
//   - what a DMA running alongside does to the CPU: channel 6 (OTC) writing
//     1,024 words while the CPU runs cached code, or loads from RAM.
//
// It prints each kernel's name, count and six times, then END, and
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

func and(rd, rs, rt uint32) uint32 { return rType(rs, rt, rd, 0x24) }
func j(target uintptr) uint32      { return 0x08000000 | uint32(target>>2)&0x3ffffff }

type kernel struct {
	name  string
	n     int
	pre   func(e *emit) // before the first counter read
	body  func(e *emit)
	post  func(e *emit) // after the time is stored
	flush bool          // run 4 KB of other code after each time
}

type emit struct {
	code  []uint32
	at    int
	far   []uint32
	farAt int
}

func (e *emit) put(w ...uint32) {
	for _, x := range w {
		e.code[e.at] = x
		e.at++
	}
}

func (e *emit) here() uintptr { return uintptr(unsafe.Pointer(&e.code[e.at])) }

func farAddr(e *emit, i int) uintptr { return uintptr(unsafe.Pointer(&e.far[i])) }

var (
	codeBuf [16 * 1024]uint32
	farBuf  [16 * 1024]uint32
	data    [2048]uint32
	times   [32][reps]uint32
	entries [32]uintptr
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
		lui(t2, 0xbfc0),
		addiu(t7, zero, reps),
	)
	loop := e.at
	if k.pre != nil {
		k.pre(e)
	}
	e.put(lw(a0, t9, 0x1120))
	k.body(e)
	e.put(
		lw(a1, t9, 0x1120),
		nop,
		subu(a1, a1, a0),
		andi(a1, a1, 0xffff),
		sw(a1, t8, 0),
		addiu(t8, t8, 4),
	)
	if k.post != nil {
		k.post(e)
	}
	if k.flush {
		for range 1100 {
			e.put(nop)
		}
	}
	e.put(addiu(t7, t7, -1))
	e.put(bne(t7, zero, int32(loop-(e.at+1))), nop)
	e.put(jr(ra), nop)
	return uintptr(unsafe.Pointer(&e.code[start]))
}

func nops(n int) func(e *emit) {
	return func(e *emit) {
		for range n {
			e.put(nop)
		}
	}
}

// mapping runs 128 instructions at a far block, then 128 at another d
// words after it, and comes back.
func mapping(d int) func(e *emit) {
	return func(e *emit) {
		e.farAt = (e.farAt + 1023) &^ 1023
		a, b := e.farAt, e.farAt+d
		e.farAt = b + 256
		ret := e.here() + 8
		e.put(j(farAddr(e, a)), nop)
		for i := range 126 {
			e.far[a+i] = nop
		}
		e.far[a+126] = j(farAddr(e, b))
		e.far[a+127] = nop
		for i := range 126 {
			e.far[b+i] = nop
		}
		e.far[b+126] = j(ret)
		e.far[b+127] = nop
	}
}

// lines runs two instructions in each of 16 lines, entered at offset words
// into each.
func lines(offset int) func(e *emit) {
	return func(e *emit) {
		e.farAt = (e.farAt + 3) &^ 3
		base := e.farAt
		e.farAt += 16*4 + 4
		ret := e.here() + 8
		e.put(j(farAddr(e, base+offset)), nop)
		for l := range 16 {
			at := base + 4*l + offset
			next := ret
			if l < 15 {
				next = farAddr(e, base+4*(l+1)+offset)
			}
			e.far[at] = j(next)
			e.far[at+1] = nop
		}
	}
}

// otc starts channel 6 clearing 1,024 words of data, and waitOTC waits
// for it to end.
func otc(e *emit) {
	m := addr(&data[1023])
	e.put(
		lui(v0, m>>16), ori(v0, v0, m&0xffff), sw(v0, t9, 0x10e0),
		addiu(v0, zero, 1024), sw(v0, t9, 0x10e4),
		lui(v0, 0x1100), ori(v0, v0, 2), sw(v0, t9, 0x10e8),
	)
}

func waitOTC(e *emit) {
	e.put(lui(v1, 0x0100))
	l := e.at
	e.put(lw(v0, t9, 0x10e8), nop, and(v0, v0, v1))
	e.put(bne(v0, zero, int32(l-(e.at+1))), nop)
}

func loads(stride int32) func(e *emit) {
	return func(e *emit) {
		for i := range int32(64) {
			e.put(lw(v0, t0, stride*i))
		}
	}
}

var kernels = []kernel{
	{name: "empty", body: func(*emit) {}},
	{name: "map 2KB", n: 256, body: mapping(512)},
	{name: "map 4KB", n: 256, body: mapping(1024)},
	{name: "map 6KB", n: 256, body: mapping(1536)},
	{name: "map 8KB", n: 256, body: mapping(2048)},
	{name: "cold empty", body: func(*emit) {}, flush: true},
	{name: "cold nop 64", n: 64, body: nops(64), flush: true},
	{name: "cold nop 256", n: 256, body: nops(256), flush: true},
	{name: "cold 2 of 4 at 0", n: 32, body: lines(0), flush: true},
	{name: "cold 2 of 4 at 8", n: 32, body: lines(2), flush: true},
	{name: "warm 2 of 4 at 0", n: 32, body: lines(0)},
	{name: "lw same", n: 64, body: loads(0)},
	{name: "lw stride 4", n: 64, body: loads(4)},
	{name: "lw stride 128", n: 64, body: loads(128)},
	{name: "nop 256", n: 256, body: nops(256)},
	{name: "otc nop 256", n: 256, pre: otc, body: nops(256), post: waitOTC},
	{name: "otc lw 64", n: 64, pre: otc, body: loads(4), post: waitOTC},
	{name: "otc alone", n: 1024, pre: otc, body: waitOTC},
}

func call(p uintptr) {
	var f func()
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, p
	f()
}

func main() {
	println("cpucost2")
	yaroze.ResetGraph(0)
	e := &emit{code: aligned(codeBuf[:]), far: aligned(farBuf[:])}
	for i := range kernels {
		entries[i] = build(e, i, &kernels[i])
	}
	print("code words ", e.at, " far words ", e.farAt, "\n")

	mode := (*uint32)(yaroze.Pointer(0x1f801124))
	dpcr := (*uint32)(yaroze.Pointer(0x1f8010f0))
	savedMode, savedDPCR := volatile.LoadUint32(mode), volatile.LoadUint32(dpcr)
	yaroze.EnterCriticalSection()
	volatile.StoreUint32(dpcr, savedDPCR|0x08000000) // channel 6 on
	volatile.StoreUint32(mode, 0)
	for i := range kernels {
		call(entries[i])
	}
	volatile.StoreUint32(mode, savedMode)
	volatile.StoreUint32(dpcr, savedDPCR)
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
