//go:build yaroze

// monhook is irqsplit with the clean monitor's two hooks put in by the probe,
// for spec 46: on a console the clean library runs under the clean monitor
// (serial/monitor), which points the exception vector at 80h and the kernel's
// B-function stub at B0h at code of its own, where Sony's library runs under
// the boot disc's monitor. The vector's code looks at the cause and at the
// serial port before every interrupt reaches the kernel, and the stub's
// compares the function's number with four of its own before every
// B-function, the kernel's ReturnFromException among them. stagemap found
// seven cache lines more in the kernel's entry under the clean library, the
// vector code's; this probe measures what both cost, from outside, on either
// library.
//
// The hooks here are copies of the monitor's interrupt paths, the same
// instructions at the same places in two arrays laid out as the monitor's are
// (the stub's code a page offset D70h, the vector's 100h after it, as the
// monitor's dispatch and vector are on the console by stagemap's lines), but
// with every way out going on to the kernel: they stop nothing and print
// nothing. Each is entered through the four instructions it replaces, which
// it keeps and runs at its end, so it works over the kernel's own stubs and
// over the clean monitor's alike.
//
// It runs irqsplit's 32 blanks five times: at 240 lines plain and hooked, and
// at 480 lines plain, hooked and plain again, with the hooks taken out before
// each plain run and before END. It prints irqsplit's BLANK and SPLIT lines
// for each, named "240p plain", "240p hooked" and so on. A hooked run's parts
// less the plain runs' are what the hooks cost: the entry (gap less whole),
// the kernel's part and the part after the callback, where the kernel returns
// through the stub.
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

// The hooks' code: a 4 KB page, with the stub's at D70h and the vector's at
// E70h.
var hookBuf [3 * 1024]uint32

var (
	hookPage  uintptr
	watching  uint32 = 1
	savedVec  [4]uint32
	savedStub [4]uint32
	hooked    bool
)

const (
	k0 = 26
	k1 = 27
	t4 = 12
)

func beq(rs, rt uint32, off int32) uint32 { return iType(0x04, rs, rt, off) }

func words(p uintptr, n int) []uint32 {
	return unsafe.Slice((*uint32)(unsafe.Pointer(p)), n)
}

// jumpTo is the four instructions of a jump to p through reg.
func jumpTo(reg uint32, p uintptr) [4]uint32 {
	a := uint32(p)
	return [4]uint32{lui(reg, (a+0x8000)>>16), addiu(reg, reg, int32(int16(a))), jr(reg), nop}
}

// install copies the vector's and the stub's instructions to the ends of the
// hooks and points both at the hooks.
func install() {
	if hooked {
		return
	}
	vec, stub := (*[4]uint32)(yaroze.Pointer(0x80)), (*[4]uint32)(yaroze.Pointer(0xb0))
	stubCode, vecCode := words(hookPage+0xd70, 64), words(hookPage+0xe70, 96)

	// The stub's: four numbers compared, none taken anywhere but on, and
	// then the kernel's stub.
	n := 0
	for _, f := range []int32{0x3d, 0x3b, 0x3f, 0x35} {
		stubCode[n] = addiu(t2, zero, f)
		stubCode[n+1] = beq(t1, t2, int32(12-(n+2)))
		stubCode[n+2] = nop
		n += 3
	}
	// The vector's: the cause, then, for an interrupt, the monitor's flag and
	// the serial port's status, and then the kernel's vector, at word 80.
	const (
		interrupt = 58
		kernel    = 80
	)
	for i := range vecCode {
		vecCode[i] = nop
	}
	copy(vecCode, []uint32{
		0x401a6800, // mfc0 k0, cause
		nop,
		iType(0x0c, k0, k0, 0x7c),
		beq(k0, zero, interrupt-4),
		nop,
		addiu(k1, zero, 8<<2),
		beq(k0, k1, kernel-7),
		nop,
		beq(zero, zero, kernel-9),
		nop,
	})
	w := uint32(uintptr(unsafe.Pointer(&watching)))
	copy(vecCode[interrupt:], []uint32{
		lui(k1, (w+0x8000)>>16),
		lw(k1, k1, int32(int16(w))),
		nop,
		beq(k1, zero, kernel-(interrupt+4)),
		nop,
		lui(k1, 0x1f80),
		lw(k0, k1, 0x1054),
		nop,
		iType(0x0c, k0, k0, 2),
		beq(k0, zero, kernel-(interrupt+10)),
		nop,
	})

	yaroze.EnterCriticalSection()
	for i := range 4 {
		savedVec[i], savedStub[i] = vec[i], stub[i]
		vecCode[kernel+i], stubCode[12+i] = vec[i], stub[i]
	}
	jv, js := jumpTo(k0, hookPage+0xe70), jumpTo(t0, hookPage+0xd70)
	for i := range 4 {
		volatile.StoreUint32(&vec[i], jv[i])
	}
	yaroze.FlushCache()
	for i := range 4 {
		volatile.StoreUint32(&stub[i], js[i])
	}
	yaroze.FlushCache()
	yaroze.ExitCriticalSection()
	hooked = true
}

// remove puts the vector and the stub back.
func remove() {
	if !hooked {
		return
	}
	vec, stub := (*[4]uint32)(yaroze.Pointer(0x80)), (*[4]uint32)(yaroze.Pointer(0xb0))
	yaroze.EnterCriticalSection()
	for i := range 4 {
		volatile.StoreUint32(&stub[i], savedStub[i])
	}
	for i := range 4 {
		volatile.StoreUint32(&vec[i], savedVec[i])
	}
	yaroze.ExitCriticalSection()
	yaroze.FlushCache()
	hooked = false
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	saved := volatile.LoadUint32((*uint32)(yaroze.Pointer(timer2Mod)))
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), 0) // system clock
	hookPage = (uintptr(unsafe.Pointer(&hookBuf[0])) + 4095) &^ 4095

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
	record("240p plain")
	install()
	yaroze.VSync(0)
	record("240p hooked")
	remove()
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.VSync(0)
	record("480i plain")
	install()
	yaroze.VSync(0)
	record("480i hooked")
	remove()
	yaroze.VSync(0)
	record("480i plain again")

	yaroze.VSyncCallback(0)
	yaroze.EnterCriticalSection()
	call(deq3)
	call(deq0)
	yaroze.ExitCriticalSection()
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), saved)
	println("END")
}
