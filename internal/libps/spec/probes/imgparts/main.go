//go:build yaroze

// imgparts is a console probe for spec 50: it splits a free image call's
// time into parts, from outside, and times DrawSync(1) polled behind each
// kind of image call. It runs in the emulator too, for the cost model's
// figures beside the console's.
//
// A call masks every interrupt (I_MASK 0) for its middle, from before its
// look at the channel to after the transfer has started. The probe makes
// root counter 2, on the system clock, interrupt at a count T after it is
// started just before the call, for every T from 1 to past the call's end,
// one call for each T. Machine code the probe puts first in the kernel's
// interrupt chain (SysEnqIntRP, as irqsplit does) reads the counter, and
// D2_CHCR, D2_MADR, D2_BCR and GPUSTAT, when the interrupt is taken, and
// acknowledges it. With the interrupt due before the call masks, the stamp is T and
// what the kernel takes to get to the chain. Due while I_MASK is 0, it is
// taken when the call puts I_MASK back, whatever T was. So the first T with
// the late stamp is where the call masks, the stamp is where it unmasks,
// and the call's end is read after it: three parts, each in cycles. It reads
// no address of the library's code: the stamps are the counter's.
//
// PARTS lines, a case each. "cold" is the call's cycles after a VSync(0)
// and a DrawSync(0), the least of eight, as loadsync times it; "warm" the
// least and most of eight after a DrawSync(0) alone. "lat" is the least and
// most of the stamp less T before the call masks. "mask" is the first T
// taken late, "stamp" the least and most of the counter when those were
// taken and "samples" how many there were, "last" the last T taken then
// (the next is taken on time again: the call has unmasked), and
// "chcr", "madr", "bcr" and "stat" what the handler read at that stamp.
// "after" is the least and most of the stamp less T from there to the
// call's end. A call that masks nothing has mask -1.
//
// COLDPARTS lines are the same for a few of the cases with every shot made
// as loadsync makes its calls, after a VSync(0) and a DrawSync(0), which is
// a shot a frame: T goes by 4 there.
//
// EDGES lines: for T past the unmasking, each T where D2_MADR is first read
// with a new value, and the value's low 16 bits: the blocks' progress while
// the call's end runs, and after it.
//
// POLL lines: after each kind of call on 256 by 256, 24 DrawSync(1) in a
// row, timed as one (cycles), then 16 more timed one by one, what the first
// 24 gave (their sum), and the polls after those to a 0. "POLL IDLE" is
// the same with nothing going on.
//
// It prints END and returns.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	iMaskPort   = 0x1f801074
	timerValue  = 0x1f801120
	timerMode   = 0x1f801124
	timerTarget = 0x1f801128

	modeClock = 0x000 // the system clock, free running
	modeIRQ   = 0x010 // and an interrupt at the target, once
)

func load(a uintptr) uint32     { return volatile.LoadUint32((*uint32)(yaroze.Pointer(a))) }
func store(a uintptr, v uint32) { volatile.StoreUint32((*uint32)(yaroze.Pointer(a)), v) }
func now() uint32               { return load(timerValue) & 0xffff }

// Registers and instructions for the handler.
const (
	zero = 0
	v0   = 2
	a0   = 4
	a1   = 5
	t0   = 8
	t1   = 9
	t2   = 10
	t3   = 11
	t4   = 12
	ra   = 31
)

func iType(op, rs, rt uint32, imm int32) uint32 {
	return op<<26 | rs<<21 | rt<<16 | uint32(imm)&0xffff
}
func lui(rt, imm uint32) uint32           { return iType(0x0f, 0, rt, int32(imm)) }
func ori(rt, rs, imm uint32) uint32       { return iType(0x0d, rs, rt, int32(imm)) }
func andi(rt, rs, imm uint32) uint32      { return iType(0x0c, rs, rt, int32(imm)) }
func addiu(rt, rs uint32, i int32) uint32 { return iType(0x09, rs, rt, i) }
func lw(rt, rs uint32, off int32) uint32  { return iType(0x23, rs, rt, off) }
func sw(rt, rs uint32, off int32) uint32  { return iType(0x2b, rs, rt, off) }
func beq(rs, rt uint32, off int32) uint32 { return iType(0x04, rs, rt, off) }
func jr(rs uint32) uint32                 { return rs<<21 | 0x08 }
func addu(rd, rs, rt uint32) uint32       { return rs<<21 | rt<<16 | rd<<11 | 0x21 }

const nop = 0

// seen is what the handler leaves: the counter, D2_CHCR, D2_MADR, D2_BCR,
// GPUSTAT, and how many interrupts it has taken.
var seen [6]uint32

// node is the chain element: next, second function, first function, 0.
var node [4]uint32

var codeBuf [64]uint32

func aligned() []uint32 {
	for i := range codeBuf {
		if uintptr(unsafe.Pointer(&codeBuf[i]))%16 == 0 {
			return codeBuf[i:]
		}
	}
	return codeBuf[:]
}

// handler writes the chain's code: with root counter 2's interrupt raised,
// it stamps and acknowledges it. It returns 0 either way, for the kernel
// to go on down the chain.
func handler(code []uint32) (entry uintptr, n int) {
	a := uint32(uintptr(unsafe.Pointer(&seen)))
	w := []uint32{
		lui(t0, 0x1f80),
		lw(t2, t0, 0x1120),
		lw(t1, t0, 0x1070),
		lui(t3, a>>16),
		andi(t1, t1, 0x40),
		beq(t1, zero, 19),
		ori(t3, t3, a&0xffff),
		sw(t2, t3, 0),
		lw(t2, t0, 0x10a8),
		lw(t4, t0, 0x10a0),
		sw(t2, t3, 4),
		sw(t4, t3, 8),
		lw(t2, t0, 0x10a4),
		lw(t4, t0, 0x1814),
		sw(t2, t3, 12),
		sw(t4, t3, 16),
		lw(t2, t3, 20),
		addiu(t4, zero, -65), // every bit but 6
		addiu(t2, t2, 1),
		sw(t2, t3, 20),
		sw(t4, t0, 0x1070),
		nop,
		nop,
		nop,
		nop,
		jr(ra),
		addu(v0, zero, zero),
	}
	copy(code, w)
	return uintptr(unsafe.Pointer(&code[0])), len(w)
}

// kernelC writes code that calls the kernel's C(fn)(0, &node).
func kernelC(code []uint32, fn int32) uintptr {
	a := uint32(uintptr(unsafe.Pointer(&node)))
	copy(code, []uint32{
		lui(a1, a>>16),
		ori(a1, a1, a&0xffff),
		addiu(a0, zero, 0),
		addiu(t2, zero, 0xc0),
		jr(t2),
		addiu(t1, zero, fn),
	})
	return uintptr(unsafe.Pointer(&code[0]))
}

func call(p uintptr) {
	var f func()
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, p
	f()
}

var (
	pixels [256 * 256]uint16
	back   [256 * 256]uint16
)

type probe struct {
	name string
	rect yaroze.RECT
	call func(r *yaroze.RECT)
}

func loadImage(r *yaroze.RECT)  { yaroze.LoadImage(r, (*uint32)(unsafe.Pointer(&pixels[0]))) }
func storeImage(r *yaroze.RECT) { yaroze.StoreImage(r, (*uint32)(unsafe.Pointer(&back[0]))) }
func moveImage(r *yaroze.RECT)  { yaroze.MoveImage(r, 384, 256) }
func clearImage(r *yaroze.RECT) { yaroze.ClearImage(r, 1, 2, 3) }

// An image 32 wide and n high is n blocks of 16 words.
var probes = [...]probe{
	{"LOAD 0x4", yaroze.RECT{X: 640, Y: 0, W: 0, H: 4}, loadImage},
	{"LOAD 16x1", yaroze.RECT{X: 640, Y: 0, W: 16, H: 1}, loadImage},
	{"LOAD 20x2", yaroze.RECT{X: 640, Y: 0, W: 20, H: 2}, loadImage},
	{"LOAD 32x1", yaroze.RECT{X: 640, Y: 0, W: 32, H: 1}, loadImage},
	{"LOAD 32x2", yaroze.RECT{X: 640, Y: 0, W: 32, H: 2}, loadImage},
	{"LOAD 32x4", yaroze.RECT{X: 640, Y: 0, W: 32, H: 4}, loadImage},
	{"LOAD 16x16", yaroze.RECT{X: 640, Y: 0, W: 16, H: 16}, loadImage},
	{"LOAD 32x12", yaroze.RECT{X: 640, Y: 0, W: 32, H: 12}, loadImage},
	{"LOAD 24x24", yaroze.RECT{X: 640, Y: 0, W: 24, H: 24}, loadImage},
	{"LOAD 32x24", yaroze.RECT{X: 640, Y: 0, W: 32, H: 24}, loadImage},
	{"LOAD 32x32", yaroze.RECT{X: 640, Y: 0, W: 32, H: 32}, loadImage},
	{"LOAD 32x40", yaroze.RECT{X: 640, Y: 0, W: 32, H: 40}, loadImage},
	{"LOAD 48x48", yaroze.RECT{X: 640, Y: 0, W: 48, H: 48}, loadImage},
	{"LOAD 32x128", yaroze.RECT{X: 640, Y: 0, W: 32, H: 128}, loadImage},
	{"STORE 0x4", yaroze.RECT{X: 640, Y: 0, W: 0, H: 4}, storeImage},
	{"STORE 16x1", yaroze.RECT{X: 640, Y: 0, W: 16, H: 1}, storeImage},
	{"STORE 32x1", yaroze.RECT{X: 640, Y: 0, W: 32, H: 1}, storeImage},
	{"STORE 16x16", yaroze.RECT{X: 640, Y: 0, W: 16, H: 16}, storeImage},
	{"STORE 32x32", yaroze.RECT{X: 640, Y: 0, W: 32, H: 32}, storeImage},
	{"STORE 32x128", yaroze.RECT{X: 640, Y: 0, W: 32, H: 128}, storeImage},
	{"MOVE 16x16", yaroze.RECT{X: 640, Y: 0, W: 16, H: 16}, moveImage},
	{"MOVE 64x64", yaroze.RECT{X: 640, Y: 0, W: 64, H: 64}, moveImage},
	{"CLEAR 16x16", yaroze.RECT{X: 641, Y: 0, W: 16, H: 16}, clearImage},
	{"FILL 64x16", yaroze.RECT{X: 640, Y: 0, W: 64, H: 16}, clearImage},
}

// plain times the call with no interrupt due, in cycles.
//
//go:noinline
func plain(p *probe, blank bool) uint32 {
	if blank {
		yaroze.VSync(0)
	}
	yaroze.DrawSync(0)
	r := p.rect
	store(timerMode, modeClock)
	p.call(&r)
	return now()
}

const maxT = 4096

// sweep is whether the calls are split in parts: without it the probe
// prints each call's time and the POLL lines alone, which is quick in the
// emulator.
const sweep = true

// raw prints every stamp, for looking into the probe itself.
const raw = false

var (
	stamps [maxT]uint16
	madrs  [maxT]uint16
)

// shot makes the call with the interrupt due at count t. Warm, it comes
// after the same call made with none due, so that every shot finds the
// cache as the last left it; cold, after a VSync(0) and a DrawSync(0), as
// loadsync's calls do. It reports whether exactly one interrupt of ours
// came, and, warm, no vertical blank since the shot before ended: a blank's
// handler leaves the kernel's code out of the cache, and the stamp then
// comes some 110 cycles late.
//
//go:noinline
func shot(p *probe, t uint32, cold bool) bool {
	if cold {
		yaroze.VSync(0)
	}
	blanks := yaroze.VSync(-1)
	fresh := cold || blanks == lastBlanks
	yaroze.DrawSync(0)
	r := p.rect
	if !cold {
		p.call(&r)
		yaroze.DrawSync(0)
		r = p.rect
	}
	count := volatile.LoadUint32(&seen[5])
	store(timerTarget, t)
	store(timerMode, modeIRQ)
	p.call(&r)
	for i := 0; i < 20000 && volatile.LoadUint32(&seen[5]) == count; i++ {
	}
	store(timerMode, modeClock)
	lastBlanks = yaroze.VSync(-1)
	return fresh && volatile.LoadUint32(&seen[5]) == count+1 && lastBlanks == blanks
}

// hist counts the samples by stamp, and maskSpread is how far apart the
// stamps of the masked part may lie.
const (
	histLen    = 8192
	maskSpread = 12
)

var hist [histLen]uint16

// lastBlanks is the count of vertical blanks when the last shot ended.
var lastBlanks int32

// parts sweeps T over the call p, every step cycles, warm or cold, and
// prints its PARTS and EDGES lines.
func parts(p *probe, cold bool, step uint32) {
	least, warm, warmMost := uint32(0xffff), uint32(0xffff), uint32(0)
	for range 8 {
		least = min(least, plain(p, true))
	}
	plain(p, false)
	for range 8 {
		w := plain(p, false)
		warm, warmMost = min(warm, w), max(warmMost, w)
	}
	end := warm
	if cold {
		end = least
	}
	if !sweep {
		println("PARTS", p.name, "cold", least, "warm", warm, warmMost)
		return
	}
	// Sample k is at T of 1 + k*step.
	at := func(k uint32) uint32 { return 1 + k*step }
	n := min((end+700)/step, maxT-1)
	missed := 0
	var dev struct{ chcr, madr, bcr, stat uint32 }
	for k := uint32(0); k < n; k++ {
		ok := false
		for try := 0; try < 4 && !ok; try++ {
			ok = shot(p, at(k), cold)
		}
		if !ok {
			missed++
		}
		if k == 7 && missed == 8 {
			// No interrupt comes at all: not worth the sweep.
			println("PARTS", p.name, "cold", least, "warm", warm, warmMost, "no interrupt")
			return
		}
		stamps[k] = uint16(volatile.LoadUint32(&seen[0]))
		madrs[k] = uint16(volatile.LoadUint32(&seen[2]))
	}
	if raw {
		for k := uint32(0); k < n; k++ {
			print(" ", stamps[k])
			if k%32 == 31 {
				println()
			}
		}
		println()
	}
	// The stamp most samples share is the masked part's: every interrupt
	// due while I_MASK is 0 is taken at the one moment. On a console the
	// stamps of one moment spread over a few cycles, so the count is of
	// stamps within maskSpread of each other, and the part runs from the
	// first sample with such a stamp to the last.
	for i := range hist {
		hist[i] = 0
	}
	for k := uint32(0); k < n; k++ {
		hist[min(uint32(stamps[k]), histLen-1)]++
	}
	best, most, sum := uint32(0), uint32(0), uint32(0)
	for v := uint32(0); v < histLen; v++ {
		sum += uint32(hist[v])
		if v >= maskSpread {
			sum -= uint32(hist[v-maskSpread])
		}
		if sum > most {
			best, most = v, sum
		}
	}
	in := func(k uint32) bool { v := uint32(stamps[k]); return v <= best && v+maskSpread > best }
	first, last := n, uint32(0)
	mask, stamp, lo, hi := int32(-1), uint32(0), uint32(0xffff), uint32(0)
	if most*step >= 100 {
		for k := uint32(0); k < n; k++ {
			if in(k) {
				if first == n {
					first = k
				}
				last = k
				lo, hi = min(lo, uint32(stamps[k])), max(hi, uint32(stamps[k]))
			}
		}
		mask, stamp = int32(at(first)), lo
		// The handler's other reads, at a T in the middle of the part.
		shot(p, at((first+last)/2), cold)
		dev.chcr, dev.madr, dev.bcr, dev.stat = volatile.LoadUint32(&seen[1]), volatile.LoadUint32(&seen[2]), volatile.LoadUint32(&seen[3]), volatile.LoadUint32(&seen[4])
	}
	spread := func(from, to uint32) (lo, hi uint32) {
		lo = 0xffff
		for k := from; k < to && k < n; k++ {
			d := uint32(stamps[k]-uint16(at(k))) & 0xffff
			lo, hi = min(lo, d), max(hi, d)
		}
		return lo, hi
	}
	latLo, latHi := spread(0, first)
	aftLo, aftHi := spread(last+1, end/step)
	name := "PARTS"
	if cold {
		name = "COLDPARTS"
	}
	println(name, p.name, "cold", least, "warm", warm, warmMost, "lat", latLo, latHi,
		"mask", mask, "stamp", stamp, hi, "samples", most, "last", at(last),
		"chcr", dev.chcr>>24, "madr", dev.madr&0xffff, "bcr", dev.bcr>>16, "stat", dev.stat>>24,
		"after", aftLo, aftHi, "missed", missed)
	if mask < 0 || cold {
		return
	}
	print("EDGES ", p.name)
	edges := 0
	for k := last + 1; k < n && edges < 40; k++ {
		if madrs[k] != madrs[k-1] {
			print(" ", at(k), ":", madrs[k])
			edges++
		}
	}
	println()
}

// colds are the cases swept cold as well, every 4 cycles: a shot a frame.
var colds = [...]string{"LOAD 0x4", "LOAD 16x16", "LOAD 48x48", "STORE 16x16", "MOVE 16x16", "CLEAR 16x16", "FILL 64x16"}

var big = yaroze.RECT{X: 704, Y: 0, W: 256, H: 256}

const (
	block  = 24
	singly = 16
)

func polls(name string, f func(r *yaroze.RECT)) {
	var each [singly]uint32
	yaroze.VSync(0)
	yaroze.DrawSync(0)
	r := big
	if f != nil {
		f(&r)
	}
	sum := int32(0)
	store(timerMode, modeClock)
	for range block {
		sum += yaroze.DrawSync(1)
	}
	whole := now()
	for i := range each {
		store(timerMode, modeClock)
		yaroze.DrawSync(1)
		each[i] = now()
	}
	rest := 0
	for yaroze.DrawSync(1) != 0 {
		rest++
	}
	print("POLL ", name, " ", block, " polls ", whole, " gave ", sum, " rest ", rest, " each")
	for _, t := range each {
		print(" ", t)
	}
	println()
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	yaroze.VSync(0)

	code := aligned()
	h, n := handler(code)
	enq, deq := kernelC(code[(n+3)&^3:], 2), kernelC(code[(n+3)&^3+8:], 3)
	node[2] = uint32(h)
	yaroze.FlushCache()
	savedMode, savedMask := load(timerMode), load(iMaskPort)
	store(timerMode, modeClock)
	yaroze.EnterCriticalSection()
	call(enq)
	store(iMaskPort, savedMask|0x40)
	yaroze.ExitCriticalSection()

	for round := range 2 {
		println("ROUND", round)
		for _, k := range []struct {
			name string
			f    func(r *yaroze.RECT)
		}{{"IDLE", nil}, {"LOAD", loadImage}, {"STORE", storeImage}, {"MOVE", moveImage}, {"CLEAR", clearRect}, {"FILL", clearImage}} {
			polls(k.name, k.f)
		}
	}
	for i := range probes {
		parts(&probes[i], false, 1)
	}
	for i := range probes {
		if !sweep {
			break
		}
		for _, c := range colds {
			if probes[i].name == c {
				parts(&probes[i], true, 4)
			}
		}
	}

	yaroze.DrawSync(0)
	yaroze.EnterCriticalSection()
	store(iMaskPort, load(iMaskPort)&^0x40|savedMask&0x40)
	call(deq)
	yaroze.ExitCriticalSection()
	store(timerMode, savedMode)
	println("END")
}

// clearRect is ClearImage off a multiple of 64: a rectangle, not the fill.
func clearRect(r *yaroze.RECT) {
	r.X++
	yaroze.ClearImage(r, 1, 2, 3)
}
