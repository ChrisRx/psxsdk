//go:build yaroze

// rethook is retphase with the clean monitor's two hooks put in by the probe,
// as monhook puts them in, for spec 46: on a console the clean library runs
// under the clean monitor, whose exception vector and B-function stub every
// interrupt goes through, and Sony's under the boot disc's monitor, which has
// neither. retphase's A at 240 lines is where VSync(0) returns, the
// interrupt's end, so the two libraries' A's under their own monitors differ
// by the hooks' cost as well as by the libraries'. This probe measures A (and
// B) plain and hooked in one program: on Sony's library under the boot disc's
// monitor "240p hooked" is Sony's return under one layer of the hooks, to set
// against the clean library's "240p plain" under the clean monitor, the same
// ELF on both; hooked less plain on either is what the hooks cost a return.
//
// It waits 16 blanks first, so that a library uploaded just before it is past
// its first blanks' state, then runs retphase's 32 pairs of fields four
// times: at 240 lines plain and hooked, and at 480 lines plain and hooked,
// with the hooks taken out after each hooked run. It prints retphase's FIELD
// and PHASE lines, named "240p plain" and so on, and END.
//
// A, B and the B loop are retphase's, and the hooks are monhook's: copies of
// the monitor's interrupt paths at the monitor's page offsets, every way out
// going on to the kernel.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timer2Mod = 0x1f801124
	fields    = 32
	minGap    = 2000
)

func timer() uint32 {
	return volatile.LoadUint32((*uint32)(yaroze.Pointer(0x1f801120))) & 0xffff
}

// pollA polls timer 2 until the interrupt, and returns the cycles from from
// to the last read before it.
func pollA(from uint32) uint32 {
	prev, total := from, uint32(0)
	for n := 0; n < 4000000; n++ {
		now := timer()
		d := (now - prev) & 0xffff
		if d > minGap {
			return total
		}
		total += d
		prev = now
	}
	return 0
}

// The B loop is machine code the probe writes at 100h in a 4 KB page, cache
// lines 10h to 14h, away from both libraries' VSync and from the probe's own
// code: written in Go, the loop sat on the clean image's VSync lines and
// missed on every poll. It calls VSync at its entry, 8003CA68h, the
// address both images give it.
const vsyncEntry = 0x8003ca68

var (
	codeBuf [2048]uint32
	loopB   uintptr
	outB    [4]uint32
)

func iType(op, rs, rt uint32, imm int32) uint32 {
	return op<<26 | rs<<21 | rt<<16 | uint32(imm)&0xffff
}
func rType(rs, rt, rd, fn uint32) uint32 { return rs<<21 | rt<<16 | rd<<11 | fn }

const (
	zero = 0
	v0   = 2
	a0   = 4
	a1   = 5
	t0   = 8
	t1   = 9
	t2   = 10
	t3   = 11
	s0   = 16
	s1   = 17
	s2   = 18
	s3   = 19
	s4   = 20
	s5   = 21
	t4   = 12
	t9   = 25
	k0   = 26
	k1   = 27
	sp   = 29
	ra   = 31
)

func lui(rt, imm uint32) uint32           { return iType(0x0f, 0, rt, int32(imm)) }
func ori(rt, rs, imm uint32) uint32       { return iType(0x0d, rs, rt, int32(imm)) }
func addiu(rt, rs uint32, i int32) uint32 { return iType(0x09, rs, rt, i) }
func andi(rt, rs, imm uint32) uint32      { return iType(0x0c, rs, rt, int32(imm)) }
func sltiu(rt, rs uint32, i int32) uint32 { return iType(0x0b, rs, rt, i) }
func lw(rt, rs uint32, off int32) uint32  { return iType(0x23, rs, rt, off) }
func sw(rt, rs uint32, off int32) uint32  { return iType(0x2b, rs, rt, off) }
func addu(rd, rs, rt uint32) uint32       { return rType(rs, rt, rd, 0x21) }
func subu(rd, rs, rt uint32) uint32       { return rType(rs, rt, rd, 0x23) }
func sltu(rd, rs, rt uint32) uint32       { return rType(rs, rt, rd, 0x2b) }
func jr(rs uint32) uint32                 { return rs<<21 | 0x08 }
func jalr(rd, rs uint32) uint32           { return rs<<21 | rd<<11 | 0x09 }
func beq(rs, rt uint32, off int) uint32   { return iType(0x04, rs, rt, int32(off)) }
func bne(rs, rt uint32, off int) uint32   { return iType(0x05, rs, rt, int32(off)) }

const nop = 0

// buildB writes the B loop: entered with a0 the timer at VSync(0)'s return
// and a1 &outB, it stores the cycles to the read of timer 2 before the
// interrupt's poll, the polls, and how it ended: 0 the interrupt came
// between polls, 1 in a call, 2 the count changed with no gap, plus 4 when
// the count in the last call had the interrupt.
func buildB() uintptr {
	base := uintptr(unsafe.Pointer(&codeBuf[0]))
	page := (base + 4095) &^ 4095
	i := int(page-base)/4 + 0x100/4
	start := i
	var code []uint32
	put := func(w ...uint32) { code = append(code, w...) }
	callVSync := func() {
		put(lui(t9, vsyncEntry>>16), ori(t9, t9, vsyncEntry&0xffff), jalr(ra, t9), addiu(a0, zero, -1))
	}
	put(
		addiu(sp, sp, -32),
		sw(ra, sp, 28), sw(s0, sp, 24), sw(s1, sp, 20), sw(s2, sp, 16), sw(s3, sp, 12), sw(s4, sp, 8), sw(s5, sp, 4),
		addu(s2, a0, zero),
		addu(s5, a1, zero),
		lui(s0, 0x1f80),
	)
	callVSync()
	put(
		addu(s1, v0, zero),
		addu(s3, zero, zero),
		addu(s4, zero, zero),
	)
	loop := len(code)
	put(
		lw(t0, s0, 0x1120),
		nop,
		andi(t0, t0, 0xffff),
		subu(t1, t0, s2),
		andi(t1, t1, 0xffff),
		sltiu(t2, t1, 2000),
	)
	fixBetween := len(code)
	put(0, addiu(t3, zero, 0)) // beq t2, zero, done (between)
	put(
		addu(s3, s3, t1),
		addiu(s4, s4, 1),
		lui(t2, 2),
		sltu(t2, s4, t2),
	)
	fixLimit := len(code)
	put(0, addiu(t3, zero, 0)) // beq t2, zero, done
	put(addu(s2, t0, zero))
	callVSync()
	put(
		lw(t0, s0, 0x1120),
		nop,
		andi(t0, t0, 0xffff),
		subu(t1, t0, s2),
		andi(t1, t1, 0xffff),
		sltiu(t2, t1, 2000),
	)
	fixInCall := len(code)
	put(0, addiu(t3, zero, 1)) // beq t2, zero, done (in call)
	fixCounted := len(code)
	put(0, addiu(t3, zero, 2)) // bne v0, s1, done (counted, no gap)
	put(addu(s3, s3, t1))
	back := len(code)
	put(0, addu(s2, t0, zero)) // b loop
	done := len(code)
	put(
		beq(v0, s1, 2),
		nop,
		addiu(t3, t3, 4),
		sw(s3, s5, 0),
		sw(s4, s5, 4),
		sw(t3, s5, 8),
		lw(ra, sp, 28), lw(s0, sp, 24), lw(s1, sp, 20), lw(s2, sp, 16), lw(s3, sp, 12), lw(s4, sp, 8), lw(s5, sp, 4),
		jr(ra),
		addiu(sp, sp, 32),
	)
	code[fixBetween] = beq(t2, zero, done-(fixBetween+1))
	code[fixLimit] = beq(t2, zero, done-(fixLimit+1))
	code[fixInCall] = beq(t2, zero, done-(fixInCall+1))
	code[fixCounted] = bne(v0, s1, done-(fixCounted+1))
	code[back] = beq(zero, zero, loop-(back+1))
	copy(codeBuf[i:], code)
	return uintptr(unsafe.Pointer(&codeBuf[start]))
}

// pollB runs the B loop from the timer at the return.
func pollB(from uint32) (total, polls uint32, inCall, counted bool) {
	var f func(uint32, *uint32)
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, loopB
	f(from, &outB[0])
	how := volatile.LoadUint32(&outB[2])
	return volatile.LoadUint32(&outB[0]), volatile.LoadUint32(&outB[1]), how&3 == 1, how&4 != 0
}

type stat struct{ lo, hi, sum, n uint32 }

func (s *stat) add(v uint32) {
	if s.n == 0 || v < s.lo {
		s.lo = v
	}
	if v > s.hi {
		s.hi = v
	}
	s.sum += v
	s.n++
}

func (s *stat) show(name, what string) {
	if s.n > 0 {
		println("PHASE", name, what, s.lo, s.sum/s.n, s.hi)
	}
}

var (
	resA            [fields]uint32
	resB, resPolls  [fields]uint32
	resIn, resCount [fields]bool
)

func record(name string) {
	// Start straight after an interrupt. Nothing is printed until the
	// fields are done: printing takes long enough to move them.
	pollA(timer())
	for f := 0; f < 2*fields; f++ {
		yaroze.VSync(0)
		r := timer()
		if f%2 == 0 {
			resA[f/2] = pollA(r)
			continue
		}
		resB[f/2], resPolls[f/2], resIn[f/2], resCount[f/2] = pollB(r)
	}
	var a, b, polls stat
	for i := 0; i < fields; i++ {
		where := "between"
		if resIn[i] {
			where = "in call"
		}
		c := 0
		if resCount[i] {
			c = 1
		}
		println("FIELD", name, i, "A", resA[i], "B", resB[i], "polls", resPolls[i], where, "counted", c)
		a.add(resA[i])
		b.add(resB[i])
		polls.add(resPolls[i])
	}
	a.show(name, "A")
	b.show(name, "B")
	polls.show(name, "polls")
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
		stubCode[n+1] = beq(t1, t2, 12-(n+2))
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
	loopB = buildB()
	hookPage = (uintptr(unsafe.Pointer(&hookBuf[0])) + 4095) &^ 4095
	yaroze.FlushCache()
	saved := volatile.LoadUint32((*uint32)(yaroze.Pointer(timer2Mod)))
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), 0) // system clock
	yaroze.GsInitGraph(320, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	for i := 0; i < 16; i++ {
		yaroze.VSync(0)
	}
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
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), saved)
	println("END")
}
