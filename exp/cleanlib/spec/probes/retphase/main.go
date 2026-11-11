//go:build yaroze

// retphase measures, in system clock cycles and without a reference inside
// the interrupt, where a program is when the vertical blank after a VSync(0)
// comes, for spec 45's difference at 480 lines. Each field, called straight
// after an interrupt as vsyncpoll calls it, VSync(0) returns, the program
// reads timer 2 and then:
//
//   - A: polls timer 2 alone in a tight loop until a read comes more than
//     2,000 cycles after the one before (the interrupt): A is the cycles from
//     the return to the last read before it. The blank comes a fixed time
//     after GPUSTAT bit 31's change, which VSync(0) waits for at 480 lines,
//     so A against A is VSync(0)'s return against the other library's.
//   - B: polls as vsyncpoll does, timer 2, VSync(-1), timer 2, until the
//     count changes (machine code the probe writes, see buildB): B is the cycles from the return to the read of timer 2
//     before the interrupt's poll, with the polls made and whether the
//     interrupt came in the call. B less A, over the polls, is what a poll
//     costs; B against B is where the poll is when the blank comes.
//
// Fields alternate A and B, 32 of each, at 240 lines and then at 480. It
// prints, after each display's fields, a line a pair, then the shortest,
// mean and longest of A and B and the polls, then END, and returns.
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
	t9   = 25
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

func main() {
	println("start")
	yaroze.ResetGraph(0)
	loopB = buildB()
	yaroze.FlushCache()
	saved := volatile.LoadUint32((*uint32)(yaroze.Pointer(timer2Mod)))
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), 0) // system clock
	yaroze.GsInitGraph(320, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	yaroze.VSync(0)
	record("240p")
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.VSync(0)
	record("480i")
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), saved)
	println("END")
}
