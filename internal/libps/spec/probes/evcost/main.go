//go:build yaroze

// evcost prices the kernel's open events in the vertical blank interrupt,
// for spec 46's reopening: irqsplit's four stamps (the kernel's chain of
// priority 0, of priority 3, the library's VSyncCallback and the program's
// poll), at 480 lines, with the kernel's event table (EvCB: its address at
// 80000120h, its size in bytes at 80000124h, 28 bytes an event: class,
// status, spec, mode, handler) in four states:
//
//   - found: as the program found it;
//   - closed: every event but number 0 closed (its status 0);
//   - eight: numbers 1 to 8 open as a freshly booted console has them (four
//     of class F4000001h, four of F0000011h, status 2000h), with no handler,
//     and the rest closed;
//   - nine: number 9 open as well, a copy of number 0, for what one more
//     costs;
//   - found again.
//
// It writes the table's words itself, with interrupts off, so that nothing
// is opened or closed through the kernel and the table goes back exactly as
// it was. It prints the table as found and as left (EVENT lines), the
// interrupt mask, and BLANK and SPLIT lines as irqsplit's for each state.
// A part's difference between states is what the open events cost the
// kernel there. The same ELF says what a library and a monitor leave open.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timer2Mod = 0x1f801124
	blanks    = 32

	evcbAddr  = 0xa0000120
	evcbSize  = 0xa0000124
	evWords   = 7
	maxEvents = 64
)

func word(a uintptr) *uint32 { return (*uint32)(yaroze.Pointer(a)) }

// table is the kernel's event table's address (in kseg1) and its events.
func table() (uintptr, int) {
	base := uintptr(volatile.LoadUint32(word(evcbAddr)))&0x1fffffff | 0xa0000000
	n := int(volatile.LoadUint32(word(evcbSize))) / (4 * evWords)
	if n > maxEvents {
		n = maxEvents
	}
	return base, n
}

func field(base uintptr, ev, f int) *uint32 {
	return word(base + uintptr(ev*4*evWords+f*4))
}

func dump(name string) {
	base, n := table()
	println("EVENTS", name, "table", base, "events", n, "mask", volatile.LoadUint32(word(0x1f801074))&0xffff)
	for ev := 0; ev < n; ev++ {
		if volatile.LoadUint32(field(base, ev, 1)) == 0 {
			continue
		}
		println("EVENT", name, ev,
			"class", volatile.LoadUint32(field(base, ev, 0)),
			"status", volatile.LoadUint32(field(base, ev, 1)),
			"spec", volatile.LoadUint32(field(base, ev, 2)),
			"mode", volatile.LoadUint32(field(base, ev, 3)),
			"handler", volatile.LoadUint32(field(base, ev, 4)))
	}
}

// found is the table's first five words an event, as the program found it.
var found [maxEvents][5]uint32

func save() {
	base, n := table()
	for ev := 0; ev < n; ev++ {
		for f := 0; f < 5; f++ {
			found[ev][f] = volatile.LoadUint32(field(base, ev, f))
		}
	}
}

func restore() {
	base, n := table()
	yaroze.EnterCriticalSection()
	for ev := 1; ev < n; ev++ {
		for f := 0; f < 5; f++ {
			volatile.StoreUint32(field(base, ev, f), found[ev][f])
		}
	}
	yaroze.ExitCriticalSection()
}

var eightSpecs = [4]uint32{0x0004, 0x8000, 0x0100, 0x2000}

// set closes every event but number 0, and then opens numbers 1 to open: the
// memory card's eight, and after them copies of number 0.
func set(open int) {
	base, n := table()
	yaroze.EnterCriticalSection()
	for ev := 1; ev < n; ev++ {
		volatile.StoreUint32(field(base, ev, 1), 0)
		if ev > open {
			continue
		}
		class, spec := uint32(0xf4000001), eightSpecs[(ev-1)%4]
		switch {
		case ev > 8:
			class, spec = found[0][0], found[0][2]
		case ev > 4:
			class = 0xf0000011
		}
		volatile.StoreUint32(field(base, ev, 0), class)
		volatile.StoreUint32(field(base, ev, 2), spec)
		volatile.StoreUint32(field(base, ev, 3), 0x2000)
		volatile.StoreUint32(field(base, ev, 4), 0)
		volatile.StoreUint32(field(base, ev, 1), 0x2000)
	}
	yaroze.ExitCriticalSection()
}

// settle waits n blanks.
func settle(n int) {
	for i := 0; i < n; i++ {
		yaroze.VSync(0)
	}
}

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

	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	settle(16)
	save()
	dump("found")
	record("found")
	set(0)
	settle(2)
	dump("closed")
	record("closed")
	set(8)
	settle(2)
	dump("eight")
	record("eight")
	set(9)
	settle(2)
	dump("nine")
	record("nine")
	restore()
	settle(2)
	record("again")
	dump("left")

	yaroze.VSyncCallback(0)
	yaroze.EnterCriticalSection()
	call(deq3)
	call(deq0)
	yaroze.ExitCriticalSection()
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), saved)
	println("END")
}
