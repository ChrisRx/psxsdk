//go:build yaroze

// vsyncstamp times, in system clock cycles, what vsyncpoll and irqgaps see
// only coarsely, for spec 45's difference at 480 lines on a console. It puts
// a handler of its own first in the kernel's interrupt chain of priority 0
// (SysEnqIntRP, C(02h)) that, at a vertical blank, reads timer 2 into stamp and adds
// 1 to marks, both in RAM. The stamp is taken a fixed time after the CPU
// takes the interrupt, the kernel's entry; marks goes up at the same
// interrupt as the library's count. Then:
//
//   - POLL: vsyncpoll's loop with marks read around the call: timer 2 (a),
//     marks (w0), VSync(-1), marks (w1), timer 2 (b), a few cycles later each
//     field after VSync(0). For the interrupt's poll it prints the stamp
//     less a, b less a, and which of w0, the count and w1 had it. The
//     largest stamp seen with a read and the smallest seen without are that
//     read's place; the differences between the three places are free of the
//     kernel's entry time, so they compare the libraries and the emulator
//     directly: where VSync(-1) reads the count after its entry and before
//     its return, in cycles.
//   - EXIT: VSync(0) called straight after an interrupt, as vsyncpoll and
//     irqgaps call it, timed from the stamp of the blank it returned at to
//     its return; then the next two interrupts polled without VSync: from
//     each stamp, the first poll after it (the interrupt's end, less the
//     entry time), GPUSTAT bit 31 before and after it, and when bit 31 next
//     changes. The return less the change is VSync's way out.
//   - BODY: copies of the clean library's VSync(-1) as it is, reading marks
//     for its count, with that read moved by k instructions, polled as in
//     POLL. On a console they give how far each instruction moves the read
//     against Sony's POLL figures.
//
// 240 lines first, then 640 by 480 interlaced. The handler is taken out
// before END.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const debug = false

const (
	iStat     = 0x1f801070
	timer2    = 0x1f801120
	timer2Mod = 0x1f801124
	gpuStat   = 0x1f801814
	scratch   = 0x1f800300
	fields    = 48
	exits     = 16
	maxPolls  = 200000
)

// Registers.
const (
	zero = 0
	v0   = 2
	v1   = 3
	a0   = 4
	a1   = 5
	a2   = 6
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
func andi(rt, rs, imm uint32) uint32      { return iType(0x0c, rs, rt, int32(imm)) }
func lw(rt, rs uint32, off int32) uint32  { return iType(0x23, rs, rt, off) }
func sw(rt, rs uint32, off int32) uint32  { return iType(0x2b, rs, rt, off) }
func beq(rs, rt uint32, off int32) uint32 { return iType(0x04, rs, rt, off) }
func jr(rs uint32) uint32                 { return rs<<21 | 0x08 }
func addu(rd, rs, rt uint32) uint32       { return rs<<21 | rt<<16 | rd<<11 | 0x21 }

const nop = 0

// vars holds the stamp, marks and a word the bodies load for their time,
// in RAM, as the library's count is.
var vars [4]uint32

const (
	vStamp = 0
	vMarks = 4
	vPad   = 8
)

// node is the chain element SysEnqIntRP links in: next, second function,
// first function, 0.
var node [4]uint32

var codeBuf [1024]uint32

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

func hi(p uintptr) uint32 { return uint32(p) >> 16 }
func lo(p uintptr) uint32 { return uint32(p) & 0xffff }

// handler reads timer 2 first, then I_STAT; at a vertical blank it keeps the
// time in stamp and adds 1 to marks. It returns 0, so that the kernel calls
// no second function.
func handler(e *emit) uintptr {
	p := e.start()
	vp := uintptr(unsafe.Pointer(&vars))
	e.put(
		lui(t0, 0x1f80),
		lw(t2, t0, 0x1120),
		lw(t1, t0, 0x1070),
		lui(t3, hi(vp)),
		andi(t1, t1, 1),
		beq(t1, zero, 6),
		ori(t3, t3, lo(vp)),
		sw(t2, t3, vStamp),
		lw(t1, t3, vMarks),
		nop,
		addiu(t1, t1, 1),
		sw(t1, t3, vMarks),
		jr(ra),
		addu(v0, zero, zero),
	)
	return p
}

// kernelC calls the kernel's C(fn)(0, &node).
func kernelC(e *emit, fn int32) uintptr {
	p := e.start()
	np := uintptr(unsafe.Pointer(&node))
	e.put(
		lui(a1, hi(np)),
		ori(a1, a1, lo(np)),
		addu(a0, zero, zero),
		addiu(t2, zero, 0xc0),
		jr(t2),
		addiu(t1, zero, fn),
	)
	return p
}

// body copies the clean library's VSync(-1) once started (the two nops of
// the resident image's jump first), with marks for its count, and the read
// of marks moved by k instructions: later past k of the six after it, or
// earlier past -k of the four nops before it. A nop stands for the test of
// the mode.
func body(e *emit, k int) uintptr {
	p := e.start()
	vp := uintptr(unsafe.Pointer(&vars))
	vhi, vlo := uint32((vp+0x8000)>>16), int32(int16(uint16(vp)))
	pad := lw(a2, 1, vlo+vPad)
	before := []uint32{pad, nop, nop, nop, nop}
	after := []uint32{pad, pad, pad, nop, nop, nop}
	count := lw(v0, 1, vlo+vMarks)
	e.put(
		nop, nop, // the image's jump and its delay
		nop,      // VSync's test of the mode
		lui(v0, 0x1f80),
		lui(1, vhi),
		lw(a1, 1, vlo+vPad),
		pad,
		pad,
		lw(v1, v0, 0x1814),
		lw(v1, v0, 0x1110),
		beq(a1, zero, 0), // never taken: vars[2] holds 1
		nop,
	)
	seq := append(append([]uint32{}, before...), after...)
	at := len(before) + k
	seq = append(seq[:at], append([]uint32{count}, seq[at:]...)...)
	e.put(seq...)
	e.put(jr(ra), nop)
	return p
}

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func store(addr uintptr, v uint32) { volatile.StoreUint32((*uint32)(yaroze.Pointer(addr)), v) }

func marks() uint32 { return volatile.LoadUint32(&vars[1]) }

func stamp() uint32 { return volatile.LoadUint32(&vars[0]) }

func call(p uintptr) {
	var f func()
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, p
	f()
}

func callRet(p uintptr) uint32 {
	var f func() uint32
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, p
	return f()
}

// sample is one field's interrupt in a poll: the stamp less a, b less a,
// and bits 0, 1 and 2 for w0, the count and w1 having it: 0 for an
// interrupt taken after the read of w1, before b.
type sample struct {
	rel   int32
	dur   uint32
	flags uint32
}

var samples [fields]sample

// poll runs fields fields of the poll with fn as the call (0 for
// VSync(-1)) and returns the shortest call without an interrupt.
func poll(fn uintptr) uint32 {
	least := uint32(0xffff)
	for f := 0; f < fields; f++ {
		samples[f] = sample{rel: -99999}
		yaroze.VSync(0)
		for i := 0; i < f; i++ {
			store(scratch, uint32(i))
		}
		w := marks()
		var c0 uint32
		if fn == 0 {
			c0 = uint32(yaroze.VSync(-1))
		} else {
			c0 = w
		}
		for n := 0; n < maxPolls; n++ {
			a := load(timer2) & 0xffff
			w0 := marks()
			var c uint32
			if fn == 0 {
				c = uint32(yaroze.VSync(-1))
			} else {
				c = callRet(fn)
			}
			w1 := marks()
			b := load(timer2) & 0xffff
			d := (b - a) & 0xffff
			if w1 == w {
				if d > 400 && marks() != w {
					// Taken after the read of w1, before b.
					samples[f] = sample{rel: int32(int16(uint16(stamp() - a))), dur: d}
					break
				}
				if d < least {
					least = d
				}
				continue
			}
			var fl uint32
			if w0 != w {
				fl |= 1
			}
			if c != c0 {
				fl |= 2
			}
			fl |= 4
			samples[f] = sample{rel: int32(int16(uint16(stamp() - a))), dur: d, flags: fl}
			break
		}
	}
	return least
}

// place prints, for the read flag bit names, the largest stamp less a with
// the read having the interrupt, the smallest without, and how many samples
// break that order.
func place(name string, bit uint32) {
	yes, no := int32(-99999), int32(99999)
	for _, s := range samples {
		if s.rel == -99999 {
			continue
		}
		if s.flags&bit != 0 {
			yes = max(yes, s.rel)
		} else {
			no = min(no, s.rel)
		}
	}
	bad := 0
	for _, s := range samples {
		if s.rel != -99999 && s.flags&bit == 0 && s.rel < yes {
			bad++
		}
	}
	print(" ", name, " ", yes, " ", no, " bad ", bad)
}

func summary(tag, name string, least uint32) {
	print(tag, " ", name, " least ", least)
	place("w0", 1)
	place("count", 2)
	place("w1", 4)
	ilo, ihi := uint32(0xffff), uint32(0)
	for _, s := range samples {
		if s.rel != -99999 && s.flags&1 == 0 {
			ilo, ihi = min(ilo, s.dur-least), max(ihi, s.dur-least)
		}
	}
	print(" interrupt ", ilo, " to ", ihi, "\n")
}

// untilMark polls GPUSTAT, timer 2 and marks until marks changes from w, and
// returns the timer read straight after, with bit 31 before and after.
func untilMark(w uint32) (at, before, after uint32, ok bool) {
	before = load(gpuStat) >> 31
	for n := 0; n < maxPolls; n++ {
		g := load(gpuStat) >> 31
		if marks() != w {
			return load(timer2) & 0xffff, before, g, true
		}
		before = g
	}
	return 0, 0, 0, false
}

// bitChange polls GPUSTAT bit 31 from its value after, for up to 60,000
// cycles from the stamp s, or until the next interrupt, and returns when it
// changed, less s, or 0.
func bitChange(s, after, w uint32) uint32 {
	for n := 0; n < maxPolls; n++ {
		g := load(gpuStat) >> 31
		t := load(timer2) & 0xffff
		since := (t - s) & 0xffff
		if since > 60000 || marks() != w {
			return 0
		}
		if g != after {
			return since
		}
	}
	return 0
}

func exitTimes(name string) {
	for k := 0; k < exits; k++ {
		// Straight after an interrupt, as vsyncpoll calls it.
		w := marks()
		if _, _, _, ok := untilMark(w); !ok {
			println("EXIT", name, k, "no interrupt")
			return
		}
		yaroze.VSync(0)
		r := load(timer2) & 0xffff
		b31 := load(gpuStat) >> 31
		ret := (r - stamp()) & 0xffff
		print("EXIT ", name, " ", k, " return ", ret, " bit31 ", b31)
		for i := 0; i < 2; i++ {
			w = marks()
			e, before, after, ok := untilMark(w)
			if !ok {
				break
			}
			s := stamp()
			ch := bitChange(s, after, w+1)
			print(" | end ", (e-s)&0xffff, " bit31 ", before, after, " change ", ch)
		}
		print("\n")
	}
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	saved := load(timer2Mod)
	store(timer2Mod, 0) // system clock, free running
	vars[2] = 1

	e := &emit{code: codeBuf[:]}
	h := handler(e)
	enq := kernelC(e, 0x02)
	deq := kernelC(e, 0x03)
	ks := []int{-4, -2, 0, 1, 2, 3, 4, 6}
	var bodies [8]uintptr
	for i, k := range ks {
		bodies[i] = body(e, k)
	}
	node[2] = uint32(h)
	yaroze.FlushCache()

	yaroze.EnterCriticalSection()
	call(enq)
	yaroze.ExitCriticalSection()

	yaroze.GsInitGraph(320, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	if debug {
		ex := load(0x80000100)
		println("handler", h, "node", uintptr(unsafe.Pointer(&node)), "next", node[0], "excb", ex)
		for i := uintptr(0); i < 4; i++ {
			el := load(uintptr(ex) + 8*i)
			for j := 0; el != 0 && j < 8; j++ {
				a := uintptr(el | 0x80000000)
				println(" chain", i, el, load(a), load(a+4), load(a+8))
				el = load(a)
			}
		}
	}
	w := marks()
	yaroze.VSync(0)
	yaroze.VSync(0)
	if marks() == w {
		println("no stamps: the handler is not called")
	} else {
		for _, d := range []string{"240p", "480i"} {
			if d == "480i" {
				yaroze.GsInitGraph(640, 480, 1, 0, 0)
				yaroze.GsDefDispBuff(0, 0, 0, 0)
			}
			least := poll(0)
			for f, s := range samples {
				println("POLL", d, f, "rel", s.rel, "dur", s.dur, "flags", s.flags)
			}
			summary("PLACE", d+" VSync", least)
			exitTimes(d)
			for i, k := range ks {
				least := poll(bodies[i])
				summary("BODY", d+" k "+itoa(k), least)
			}
		}
	}

	yaroze.EnterCriticalSection()
	call(deq)
	yaroze.ExitCriticalSection()
	store(timer2Mod, saved)
	println("END")
}

func itoa(k int) string {
	if k < 0 {
		return "-" + itoa(-k)
	}
	if k < 10 {
		return string(rune('0' + k))
	}
	return itoa(k/10) + string(rune('0'+k%10))
}
