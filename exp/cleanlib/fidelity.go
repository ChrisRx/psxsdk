//go:build yaroze && fidelity

package cleanlib

import (
	"device"
	"math/bits"
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// Everything that exists only to match Sony's time or leftover memory goes
// through the operations in this file, on the resident image and the build
// compared with a reference (spec 30). nofidelity.go has the same operations
// empty, for a program's static build, so that the compiler removes them.
// They spend time and leave memory, and a result never depends on them.

// spinCount is what spin counts on, so that the loop is not optimized
// away.
var spinCount uint32

// spinCycles busy-waits for about c CPU cycles, as the emulator counts
// them, longer if interrupts come in between.
func spinCycles(c uint32) { spin(c / spinTurn) }

// spinTurn is the cycles one turn of spin's loop takes in the emulator: a
// run of 512 turns spans 6,132 cycles in a trace of spec/probes/drawq
// (FntOpen's spin).
const spinTurn = 12

// spin busy-waits for about n of its loops, spinTurn cycles each.
//
//go:noinline
func spin(n uint32) {
	for range n {
		volatile.StoreUint32(&spinCount, volatile.LoadUint32(&spinCount)+1)
	}
}

// spinCall is what a call of spin costs beyond its turns, measured on
// spec/probes/drawq: a spin(1) at a call's end lengthens it by 26 cycles
// and a spin(2) by 38.
const spinCall = 14

// pace busy-waits so that the call takes about c cycles, the call itself
// included: whole turns of spin, so c less spinCall should be a multiple
// of spinTurn. c is a constant at every call and pace is inlined, so the
// turns are computed by the compiler. The library's calls end in one,
// sized so that each takes as long as Sony's (spec 08, "Pacing to Sony's
// times"); an interrupt in between lengthens it as it would Sony's.
//
//go:inline
func pace(c uint32) { spin((c - spinCall) / spinTurn) }

// spacer is the word the short spacers store to: a store to memory costs
// 2 cycles in the emulator, which spaces register accesses more finely
// than spin's turns.
var spacer uint32

// space spends 2 cycles a store on n stores to spacer. n is a constant at
// every call, so that the loop unrolls into plain stores.
func space(n int) {
	for range n {
		volatile.StoreUint32(&spacer, 0)
	}
}

// spaceRun spends about 8 cycles a turn on a loop of stores to spacer, as many
// turns as *n holds: for a long pad in code inlined where it would take too much
// room as space's unrolled stores, and where a call of spin would cost the
// function a frame of its own. The count is read from memory, so that the loop
// is not unrolled.
//
//go:inline
func spaceRun(n *uint32) {
	for i := volatile.LoadUint32(n); i != 0; i-- {
		volatile.StoreUint32(&spacer, 0)
	}
}

// spaceWith is space storing v, a value live on its path: when two paths
// pad with stores of the same value the compiler merges their tails, and
// then a change to one path's count moves the other's time.
func spaceWith(n int, v uint32) {
	for range n {
		volatile.StoreUint32(&spacer, v)
	}
}

// spacer2 is a second word for a pad that ends where another path's pad
// does: the compiler sinks pads that store to the same word into one block
// after the paths join, feeding the stores with whichever value each path
// had, and then their counts move together. A pad that stores another
// value to another word stays where it is.
var spacer2 uint32

// spaceApart is spaceWith storing v to spacer2 (see spacer2).
func spaceApart(n int, v uint32) {
	for range n {
		volatile.StoreUint32(&spacer2, v)
	}
}

// spaceAt spends one store, to a word of the caller's that is at hand: its
// address costs nothing to load, where spacer's costs 2 more.
//
//go:inline
func spaceAt(p *uint32) { volatile.StoreUint32(p, 0) }

// nops spends 2 cycles a nop, in line, for what a store would not fit.
//
//go:inline
func nops(n int) {
	for range n {
		device.Asm("nop")
	}
}

// delay spends n instructions and the call. In assembly, for what a Go call needs to
// spend exactly.
//
//export libps_delay
func delay(n uint32)

// The flush's working memory takes the stack as Sony's does (see kanjiFrame).
const (
	kanjiFrameBytes = 65536

	// kanjiFrameExtra is what the assembly takes off the stack besides the
	// frame: the callee's argument home at the bottom and, above the frame,
	// what brings the image to Sony's depth below the caller's sp.
	kanjiFrameExtra = 16 + 40
)

// What Krom2Tim and Krom2Tim2 leave below the caller's sp, from a pattern laid
// there by a probe (spec 30). Sony's frame is 73,872 bytes. Every call, one that
// fails too, clears 73,728 of them, from 73,784 below the caller's sp to 56 below
// it, at 8 cycles a byte. From the bottom of that, each cell has 32 bytes for the
// 30 ROM bytes of its glyph, 16 cells to a row, for the 16 rows, and then at 65,592
// below sp a successful call has the image, in rows of 16 cells however wide the
// TIM is. The rest of the frame stays as the stack had it, but for what its calls
// leave at the very bottom.
const (
	kromClearLow = 73784
	kromImageAt  = 65592
)

// clearKromStack clears the memory below the stack as Sony's frame does, frame
// being how far the calling function's sp is below its caller's. It stops at sp:
// the few bytes between that and Sony's top are the caller's own.
//
//go:inline
func clearKromStack(frame uint32) { libps_krom_clear(kromClearLow - frame) }

// leaveKromGlyph leaves the 30 ROM bytes of the glyph at addr where Sony's frame
// has them for the cell at x, y, or spends the time of it for an addr of 0.
//
//go:inline
func leaveKromGlyph(frame uint32, x, y int, addr uintptr) {
	libps_krom_glyph(int32(frame)-kromClearLow, x, y, addr)
}

// leaveKromGlyphs leaves the glyphs of the string at s up to the character that
// made Krom2Tim fail.
func leaveKromGlyphs(frame uint32, s *byte) {
	var at kanjiCursor
	for c, p := nextKanji(unsafe.Pointer(s)); c != 0; c, p = nextKanji(p) {
		if c != '\n' {
			addr, kind := glyphFor(c, false)
			if kind != glyphROM && kind != glyphBlank {
				return
			}
			leaveKromGlyph(frame, at.x, at.y, addr)
		}
		at = at.past(c)
	}
}

// leaveKromImage leaves the pixels of the TIM at t where Sony's frame has the
// image, by copying them.
//
//go:inline
func leaveKromImage(frame uint32, t *uint32, shape kromShape) {
	pix := unsafe.Add(unsafe.Pointer(t), 16*4)
	libps_krom_image(int32(frame)-kromImageAt, pix, shape.cols*8, shape.rows*kanjiCell)
}

//export libps_krom_clear
func libps_krom_clear(n uint32)

//export libps_krom_glyph
func libps_krom_glyph(at int32, x, y int, addr uintptr)

//export libps_krom_image
func libps_krom_image(at int32, pix unsafe.Pointer, stride, lines uint32)

// spend busy-waits for c cycles to the cycle, c counting the call.
func spend(c uint32) {
	if c >= spendShort+spinCall {
		n := (c - spinCall) / spinTurn
		spin(n)
		c -= spinCall + n*spinTurn
	}
	if c >= delayCall {
		delay((c - delayCall) / 2)
	}
}

const (
	spendShort = 200
	delayCall  = 14
)

// The arithmetic's time on Sony's, from a fit of hundreds of probed operands (spec 31). A sum starts by its signs and their order, takes 24
// cycles for each place between the two, 60 if the result moves up and 84 for each
// place it moves down.
const (
	addEven    = 36
	addEqual   = 1052
	addOwn     = 610 // what the sum takes of its own to the pad
	addPlace   = 24
	addCarry   = 60
	addDownBit = 81
	mulBase    = 3832
	mulBit     = 4
	mulOwn     = 584
	divBit     = 279
)

var addBases = [8]uint16{
	0<<2 | 0<<1 | 0: 1340, // a and b positive, b the bigger
	0<<2 | 0<<1 | 1: 1340,
	0<<2 | 1<<1 | 0: 1640, // a positive, b negative
	0<<2 | 1<<1 | 1: 1500,
	1<<2 | 0<<1 | 0: 1490,
	1<<2 | 0<<1 | 1: 1640,
	1<<2 | 1<<1 | 0: 1790,
	1<<2 | 1<<1 | 1: 1790,
}

func padAdd(a, b, r uint64) {
	if dIsZero(a) || dIsZero(b) {
		return
	}
	ea, eb := int32(a>>dFracBits&dExpMax), int32(b>>dFracBits&dExpMax)
	big, d := ea, ea-eb
	i := uint32(a>>63<<2 | b>>63<<1)
	if ea >= eb {
		i |= 1
	} else {
		big, d = eb, eb-ea
	}
	if d > 54 {
		return
	}
	if dIsZero(r) {
		spend(addEqual - addOwn)
		return
	}
	t := uint32(addBases[i]) + addPlace*uint32(d)
	if d == 0 {
		t += addEven
	}
	switch m := int32(r>>dFracBits&dExpMax) - big; {
	case m > 0:
		t += addCarry
	case m < 0:
		t += addDownBit * uint32(-m)
	}
	spend(t - addOwn)
}

func padMul(a, b uint64) {
	if dIsZero(a) || dIsZero(b) {
		return
	}
	spend(mulBase + mulBit*uint32(bits.OnesCount64(a&dFracMask)+bits.OnesCount64(b&dFracMask)) - mulOwn)
}

func padDiv(q uint64) {
	if q != 0 {
		spinCycles(programDivPad + divBit*uint32(bits.OnesCount64(q)) - 4*uint32(bits.OnesCount64(q)))
	}
}

func padFix(a uint64) {
	if dIsZero(a) {
		pace(programFixZeroPad)
	} else {
		pace(programFixPad)
	}
}

func padFloat() { pace(programFloatPad) }

const (
	programDivPad     = 456
	programFixPad     = 290
	programFixZeroPad = 14
	programFloatPad   = 302
)

// What sqrt and sin take beyond their arithmetic. Sony's sqrt scales by each power of two
// (20,000 cycles each) and has more of its own besides; these are its means over the probed
// arguments. Nothing goes on 0, on 1, whose Sony's is quicker, or below 0, not measured. Sony's sin
// of 0 goes the long way round, and the rest has more than the arithmetic says, on the sum over
// two programs that spend most of their time on it.
const (
	sqrtBasePad = 72000
	sqrtExpPad  = 20000
	sinZeroPad  = 6080
	sinNearPad  = 7300
	sinFarPad   = 3200
	sinNearTop  = 0x3ff921fb54442d18 // pi/2
)

func padSqrt(b uint64) {
	if b != 0 && b != 1023<<dFracBits && b&dSign == 0 {
		e := int(b>>dFracBits&dExpMax) - 1023
		spinCycles(sqrtBasePad + sqrtExpPad*uint32(max(e, -e)))
	}
}

func padSin(b uint64) {
	switch size := b &^ dSign; {
	case size == 0:
		spinCycles(sinZeroPad)
	case size < sinNearTop:
		spinCycles(sinNearPad)
	default:
		spinCycles(sinFarPad)
	}
}

// What converting between float and double takes on Sony's, beyond the work.
func padExtend() { delay(70) }
func padTrunc()  { delay(64) }

// tickVoiceTime counts a voice's read for tickWordsTime.
//
//go:inline
func tickVoiceTime(sv *sndVoice, e uint16, settled, live *uint32) {
	if e != 0 {
		sv.quiet = 0
		*live++
	} else if sv.quiet < silentReads {
		sv.quiet++
	} else {
		*settled++
	}
}

// tickWordsTime spends what the tick takes before its six words, from the last envelope
// read, over a run of spec/probes/voices: 3,124 cycles on Sony's with every voice's count
// still going, which here is its base, and 18 more for each voice settled at the top,
// which the count itself costs here. Sony's is 8 less for 24 live voices and 42 more for each
// voice it writes, where the clean loops take 29 less for a live voice and 126 more for
// one written.
func tickWordsTime(settled, written, live uint32) {
	spaceWith(1, settled)
	spend(uint32(max(0, tickWordsBase+tickLivePad*int32(live)+tickWrittenPad*int32(written))))
	nops(5)
}

const (
	tickWordsBase  = 827
	tickWrittenPad = -84
	tickLivePad    = 29
)

// kromCharTime evens out what glyphFor takes by the character, for Krom2Tim. Sony's time
// hardly changes with it, and glyphFor's table walk does: these are what each range
// is short of the longest, on pssoft-koro-koro's 283 strings of one character.
func kromCharTime(c uint16) {
	var n uint32
	switch {
	case c >= '0' && c <= '9', c >= 0x824f && c <= 0x8258:
		n = 404
	case c >= 'A' && c <= 'Z', c >= 0x8260 && c <= 0x8279:
		n = 356
	case c >= 'a' && c <= 'z', c >= 0x8281 && c <= 0x829a:
		n = 310
	case c >= 0x829f && c <= 0x82f1:
		n = 162
	case c >= 0x8340 && c <= 0x837e:
		n = 64
	case c >= 0x8380 && c <= 0x83ff:
		n = 0
	default:
		n = 810
	}
	spend(14 + n)
}

// padPow spends what a pow by a power that isn't whole takes beyond exp and log here. This
// is a fit over a probe of 26 x by 5 y, within about 20,000 of Sony's for most and
// 33,000 at worst: what Sony's does in that time isn't known. It follows x's binary
// exponent e, and more for a power of two with e odd.
func padPow(x float64) {
	b := *(*uint64)(unsafe.Pointer(&x))
	if b == 1023<<dFracBits {
		spinCycles(31000)
		return
	}
	e := int32(b>>dFracBits&dExpMax) - 1023
	if e < 0 {
		e = -e
	}
	c := 45000 + 2000*uint32(e)
	if b&dFracMask == 0 && e&1 != 0 {
		c += 15000
	}
	spinCycles(c)
}

// padPowInt is what a pow by a whole power takes around its products, beyond the work here.
func padPowInt() { spinCycles(powIntPad) }

const powIntPad = 4368

// imageBlocksWait waits, for startImage, until the blocks of a queued image
// have gone, as Sony's handler is seen to wait for them (probe
// spec/probes/imgpace, spec 45): it reads D2_CHCR 124 cycles after starting
// them and, with the channel still sending, puts I_MASK back, reads DICR
// three times, then I_STAT and I_MASK, and with the channel's completion
// raised there acknowledges it in I_STAT and in DICR, before it reads D2_CHCR
// again, 528 cycles after the start for a 16 by 16 image. With the channel
// done at the first read, it returns there. Sony's waits that way for the
// last image too, whose completion does not interrupt, until 606 cycles
// after the start, and then returns from its handler; the clean library
// still has the image's empty packet list to send and the handler's second
// pass to make after it, some 800 cycles, so startImage waits for the last
// image's blocks in a tight loop as before, which keeps the chain's end
// some 400 cycles behind Sony's, as it was. Before spec 45 the clean library read
// D2_CHCR in a tight loop and went on as soon as it showed the channel
// done, which with the transfer taking time, as the emulator has it since
// c31746e and a console always does, sent each image some 270 cycles sooner
// than Sony's. Sony's handler also takes a vertical blank raised meanwhile
// here, after its read of I_STAT; imageBlank takes it after the image.
// Inlined, so that drawNext, which it goes into, stays a function that calls
// nothing, whose free path costs what it did. Interrupts must be off. A
// build without the fidelity tag waits in a tight loop (nofidelity.go).
//
//go:inline
func imageBlocksWait() {
	spaceRun(&blocksRuns[0])
	space(blocksSpaceFirst)
	if dmaGPUControl.Get()&dmaBusy == 0 {
		return
	}
	space(blocksSpaceMask)
	iMask.Set(iMask.Get())
	spaceRun(&blocksRuns[1])
	space(blocksSpaceDICR)
	dmaInterrupt.Get()
	space(blocksSpaceDICR2)
	dmaInterrupt.Get()
	space(blocksSpaceDICR3)
	dmaInterrupt.Get()
	spaceRun(&blocksRuns[2])
	space(blocksSpaceStat)
	if iStat.Get()&iMask.Get()&irqDMA != 0 {
		spaceRun(&blocksRuns[3])
		space(blocksSpaceAck)
		iStat.Set(^uint32(irqDMA))
		space(blocksSpaceAckDICR)
		dmaInterrupt.Get()
		spaceRun(&blocksRuns[4])
		space(blocksSpaceAckDICR2)
		d := dmaInterrupt.Get()
		dmaInterrupt.Set(d&0x00ffffff | dicrGPUFlag)
		space(blocksSpaceAckEnd)
	}
	for dmaGPUControl.Get()&dmaBusy != 0 {
	}
}

// The pads imageBlocksWait spends before each of its accesses, to Sony's
// cycles: turns of spaceRun's loop, about 8 cycles each, for the long ones,
// to keep the inlined code small, then 2-cycle stores.
var blocksRuns = [...]uint32{10, 5, 5, 6, 8}

const (
	blocksSpaceFirst    = 9
	blocksSpaceMask     = 5
	blocksSpaceDICR     = 4
	blocksSpaceDICR2    = 8
	blocksSpaceDICR3    = 5
	blocksSpaceStat     = 5
	blocksSpaceAck      = 9
	blocksSpaceAckDICR  = 16
	blocksSpaceAckDICR2 = 8
	blocksSpaceAckEnd   = 8
)

// imageBlank takes, for sendImages, a vertical blank raised while an image
// went out, between two images, as Sony's handler takes it; the kernel's
// handlers do not see it. From pollQueue too, on the program's stack with
// interrupts off: the chain runs there when the program polls DrawSync(1),
// where Sony's runs in its handler (spec 45). The stores after it keep
// Sony's time from one image to the next. A build without the fidelity tag
// leaves the blank to the interrupt after the chain, as before spec 45
// (nofidelity.go).
//
//go:inline
func imageBlank() {
	if iStat.Get()&iMask.Get()&irqVBlank != 0 {
		vblankInterrupt()
		space(blocksSpaceBlank)
	}
}

// blocksSpaceBlank is the 2-cycle stores imageBlank spends after the blank.
const blocksSpaceBlank = 17
