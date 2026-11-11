//go:build yaroze

// irqsplit splits the vertical blank interrupt's time into parts, in system
// clock cycles, for spec 45: on a console the clean library's interrupt
// takes some 170 cycles longer than Sony's, warm and cold alike, with the
// same accesses to the devices. Machine code the probe writes reads timer 2
// into a word of its own at four points:
//
//   - first in the kernel's interrupt chain of priority 0 (SysEnqIntRP,
//     C(02h)): the kernel's entry;
//   - first in the chain of priority 3, after the kernel's own handlers of
//     priorities 0 to 2, the controller driver's among them;
//   - the library's VSyncCallback, which both libraries call from their
//     handler after the count;
//   - and the program's poll, its first read of timer 2 after the interrupt.
//
// Each stamp is the first instruction of its code, and the code is the
// same on both libraries, so differences between the libraries in a part
// are the libraries' (or the kernel's, run from other cache lines). For
// each display it polls timer 2 in a tight loop across 32 blanks and prints
// a line a blank: kernel (priority 0 to 3), library to the callback, from
// the callback to the poll, and the whole from priority 0, with the poll's
// gap. Then SPLIT lines with each part's shortest, mean and longest. The
// chain elements and the callback are taken out before END.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timer2Mod = 0x1f801124
	blanks    = 32
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

func record(name string) {
	var kern, lib, after, whole, gap part
	for b := 0; b < blanks; b++ {
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
	}
	for _, p := range []struct {
		name string
		p    *part
	}{{"kernel", &kern}, {"library to callback", &lib}, {"callback to poll", &after}, {"whole", &whole}, {"gap", &gap}} {
		if p.p.n > 0 {
			println("SPLIT", name, p.name, p.p.lo, p.p.sum/p.p.n, p.p.hi)
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
