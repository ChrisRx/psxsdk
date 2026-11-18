//go:build yaroze

// irqstamp is irqsplit with the clean library's own stamps read as well, for
// spec 46: on a console the clean vertical blank interrupt runs some 110 to
// 180 cycles longer than Sony's (irqsplit: 25 in the kernel's part, 43 before
// the callback, 85 after it), which the emulator's cost model does not show.
// An image of the clean library built with the irqstamp tag
// (TGOFLAGS=-tags=irqstamp exp/cleanlib/image/make.sh; for the measurement
// only, never shipped) stores timer 2 in the scratchpad's last 16 words,
// 1F8003C0h on, at twelve points inside its handler (exp/cleanlib/irqstamp.go
// lists them). This probe makes irqsplit's four stamps (the kernel's chains of
// priority 0 and 3, VSyncCallback, the program's poll), clears the inner
// words before each blank, and prints, for 32 blanks at each display:
//
//   - BLANK: irqsplit's line, the kernel's part, the library's to the
//     callback, the callback to the poll, the whole and the gap;
//   - INNER, when the image left stamps: each stretch between consecutive
//     stamps, from the kernel's priority 3 stamp to the handler's start (k3-s0),
//     then s0-s1 and so on, the callback stamp between s3 and s4 (s3-cb, cb-s4),
//     and the handler's end to the poll (s11-poll).
//
// Then SPLIT lines with each part's shortest, mean and longest. The same ELF
// runs on Sony's library (no INNER lines), on the shipped clean image (none)
// and on the stamped image. Run on a console and in the emulator with the
// costs on, each INNER part's console time over the emulator's says which
// stretch of the clean handler the console prices otherwise than the model.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timer2Mod = 0x1f801124
	blanks    = 32

	innerBase  = 0x1f8003c0
	innerCount = 12
)

const (
	zero = 0
	v0   = 2
	a0   = 4
	a1   = 5
	t0   = 8
	t1   = 9
	t2   = 10
	t3   = 11
	ra   = 31
)

func iType(op, rs, rt uint32, imm int32) uint32 {
	return op<<26 | rs<<21 | rt<<16 | uint32(imm)&0xffff
}
func lui(rt, imm uint32) uint32           { return iType(0x0f, 0, rt, int32(imm)) }
func ori(rt, rs, imm uint32) uint32       { return iType(0x0d, rs, rt, int32(imm)) }
func addiu(rt, rs uint32, i int32) uint32 { return iType(0x09, rs, rt, i) }
func lw(rt, rs uint32, off int32) uint32  { return iType(0x23, rs, rt, off) }
func sw(rt, rs uint32, off int32) uint32  { return iType(0x2b, rs, rt, off) }
func jr(rs uint32) uint32                 { return rs<<21 | 0x08 }
func addu(rd, rs, rt uint32) uint32       { return rs<<21 | rt<<16 | rd<<11 | 0x21 }

const nop = 0

// stamps: priority 0, priority 3, the callback, and how many interrupts
// priority 0 saw.
var stamps [4]uint32

// nodes are the chain elements: next, second function, first function, 0.
var nodes [2][4]uint32

var codeBuf [256]uint32

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

func (e *emit) start() uintptr {
	for uintptr(unsafe.Pointer(&e.code[e.at]))%16 != 0 {
		e.put(nop)
	}
	return uintptr(unsafe.Pointer(&e.code[e.at]))
}

func addr(p unsafe.Pointer) (uint32, uint32) {
	a := uint32(uintptr(p))
	return a >> 16, a & 0xffff
}

// stamp writes code that reads timer 2 into stamps[i] (and counts, for
// priority 0), and returns 0.
func stamp(e *emit, i int32, count bool) uintptr {
	p := e.start()
	hi, lo := addr(unsafe.Pointer(&stamps))
	e.put(
		lui(t0, 0x1f80),
		lw(t2, t0, 0x1120),
		lui(t1, hi),
		ori(t1, t1, lo),
		sw(t2, t1, 4*i),
	)
	if count {
		e.put(
			lw(t3, t1, 12),
			nop,
			addiu(t3, t3, 1),
			sw(t3, t1, 12),
		)
	}
	e.put(
		jr(ra),
		addu(v0, zero, zero),
	)
	return p
}

// kernelC writes code that calls the kernel's C(fn)(prio, &nodes[k]).
func kernelC(e *emit, fn, prio int32, k int) uintptr {
	p := e.start()
	hi, lo := addr(unsafe.Pointer(&nodes[k]))
	e.put(
		lui(a1, hi),
		ori(a1, a1, lo),
		addiu(a0, zero, prio),
		addiu(t2, zero, 0xc0),
		jr(t2),
		addiu(t1, zero, fn),
	)
	return p
}

func call(p uintptr) {
	var f func()
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, p
	f()
}

func timer() uint32 {
	return volatile.LoadUint32((*uint32)(yaroze.Pointer(0x1f801120))) & 0xffff
}

func st(i int) uint32 { return volatile.LoadUint32(&stamps[i]) & 0xffff }

func inner(i int) uint32 {
	return volatile.LoadUint32((*uint32)(yaroze.Pointer(uintptr(innerBase + 4*i))))
}

func clearInner() {
	for i := range innerCount {
		volatile.StoreUint32((*uint32)(yaroze.Pointer(uintptr(innerBase+4*i))), 0)
	}
}

type part struct {
	lo, hi, sum uint32
	n           uint32
}

func (p *part) add(v uint32) {
	if p.n == 0 || v < p.lo {
		p.lo = v
	}
	if v > p.hi {
		p.hi = v
	}
	p.sum += v
	p.n++
}

// The inner stretches, in order: from the kernel's priority 3 stamp to stamp
// 0, each stamp to the next, with the callback between 3 and 4, and stamp 11
// to the poll.
var innerNames = [...]string{
	"k3-s0", "s0-s1", "s1-s2", "s2-s3", "s3-cb", "cb-s4", "s4-s5", "s5-s6",
	"s6-s7", "s7-s8", "s8-s9", "s9-s10", "s10-s11", "s11-poll",
}

func record(name string) {
	var kern, lib, after, whole, gap part
	var inners [len(innerNames)]part
	for b := 0; b < blanks; b++ {
		clearInner()
		seen := volatile.LoadUint32(&stamps[3])
		prev := timer()
		var now uint32
		for n := 0; n < 2000000; n++ {
			now = timer()
			if (now-prev)&0xffff > 2000 {
				break
			}
			prev = now
		}
		if volatile.LoadUint32(&stamps[3]) == seen {
			println("SPLIT", name, b, "no stamp")
			continue
		}
		s0, s3, sc := st(0), st(1), st(2)
		k, l, a, w, g := (s3-s0)&0xffff, (sc-s3)&0xffff, (now-sc)&0xffff, (now-s0)&0xffff, (now-prev)&0xffff
		println("BLANK", name, b, "kernel", k, "library to callback", l, "callback to poll", a, "whole", w, "gap", g)
		kern.add(k)
		lib.add(l)
		after.add(a)
		whole.add(w)
		gap.add(g)
		if inner(0) == 0 {
			continue
		}
		// The points in order, the callback stamp between inner 3 and 4.
		var pts [innerCount + 3]uint32
		pts[0] = s3
		for i := 0; i < 4; i++ {
			pts[1+i] = inner(i) & 0xffff
		}
		pts[5] = sc
		for i := 4; i < innerCount; i++ {
			pts[2+i] = inner(i) & 0xffff
		}
		pts[innerCount+2] = now
		print("INNER ", name, " ", b)
		for i := range innerNames {
			d := (pts[i+1] - pts[i]) & 0xffff
			print(" ", innerNames[i], " ", d)
			inners[i].add(d)
		}
		println()
	}
	for _, p := range []struct {
		name string
		p    *part
	}{{"kernel", &kern}, {"library to callback", &lib}, {"callback to poll", &after}, {"whole", &whole}, {"gap", &gap}} {
		if p.p.n > 0 {
			println("SPLIT", name, p.name, p.p.lo, p.p.sum/p.p.n, p.p.hi)
		}
	}
	for i := range innerNames {
		if p := &inners[i]; p.n > 0 {
			println("SPLIT", name, innerNames[i], p.lo, p.sum/p.n, p.hi)
		}
	}
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	saved := volatile.LoadUint32((*uint32)(yaroze.Pointer(timer2Mod)))
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), 0) // system clock

	e := &emit{code: codeBuf[:]}
	h0 := stamp(e, 0, true)
	h3 := stamp(e, 1, false)
	cb := stamp(e, 2, false)
	enq0, enq3 := kernelC(e, 2, 0, 0), kernelC(e, 2, 3, 1)
	deq0, deq3 := kernelC(e, 3, 0, 0), kernelC(e, 3, 3, 1)
	nodes[0][2], nodes[1][2] = uint32(h0), uint32(h3)
	yaroze.FlushCache()

	yaroze.EnterCriticalSection()
	call(enq0)
	call(enq3)
	yaroze.ExitCriticalSection()
	yaroze.VSyncCallback(cb)

	yaroze.GsInitGraph(320, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	yaroze.VSync(0)
	record("240p")
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.VSync(0)
	record("480i")

	yaroze.VSyncCallback(0)
	yaroze.EnterCriticalSection()
	call(deq3)
	call(deq0)
	yaroze.ExitCriticalSection()
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), saved)
	println("END")
}
