//go:build yaroze

package libps

import (
	"device"
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The kernel's services, reached through the trampolines in
// internal/toolchain/yaroze/bios.S, and the library's interrupt handling
// built on them.

// libps_syscall runs the syscall instruction with code in a0 and returns
// the kernel's v0.
//
//export libps_syscall
func libps_syscall(code uintptr) int32

// libps_biosA, libps_biosB and libps_biosC call function fn of the
// kernel's A0, B0 or C0 table with the arguments after it.
//
//export libps_biosA
func libps_biosA(fn, a0, a1, a2, a3 uintptr) int32

//export libps_biosB
func libps_biosB(fn, a0, a1, a2, a3 uintptr) int32

//export libps_biosC
func libps_biosC(fn, a0, a1, a2, a3 uintptr) int32

// libps_intr_entry is where the kernel resumes, on the library's own
// stack, when it ends an interrupt: it calls libps_interrupt and then
// returns from the exception.
//
//go:extern libps_intr_entry
var libpsIntrEntry [0]byte

// The kernel functions the library calls.
const (
	biosGPUCommand    = 0x49 // A0: writes a word to GP0
	biosCdRemove      = 0x72 // A0: removes the kernel's CD-ROM handlers
	biosFlushCache    = 0x44 // A0: empties the instruction cache
	biosPrintf        = 0x3f // A0: the kernel's printf
	biosGetRCnt       = 0x03 // B0: reads a root counter
	biosStartRCnt     = 0x04 // B0: enables a root counter's interrupt
	biosResetRCnt     = 0x06 // B0: sets a root counter back to 0
	biosInitPad       = 0x12 // B0: gives the controller driver its buffers
	biosStartPad      = 0x13 // B0: starts it reading every vertical blank
	biosClearPad      = 0x5b // B0: whether it acknowledges the blank
	biosSetCustomExit = 0x19 // B0: where the kernel goes after an interrupt
	biosPutchar       = 0x3d // B0: writes a character to the TTY (not used: putchar goes through write, as Sony's does)
	biosWrite         = 0x35 // B0: writes to an open file
	biosKrom2RawAdd   = 0x51 // B0: a Shift JIS code's glyph in the ROM font
	biosClearRCnt     = 0x0a // C0: whether the kernel acknowledges a counter
)

// The interrupt controller's registers. A bit of I_STAT is set when its
// source raises an interrupt and stays set until written as 0; I_MASK
// says which bits interrupt the CPU.
var (
	iStat = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f801070)))
	iMask = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f801074)))
)

// The interrupt sources the library handles.
const (
	irqVBlank = 1 << 0
	irqCD     = 1 << 2
	irqDMA    = 1 << 3
)

// EnterCriticalSection turns interrupts off through the kernel. It returns
// 1 if they were on, and 0 if they were already off.
//
//export EnterCriticalSection
func EnterCriticalSection() int32 {
	return libps_syscall(1)
}

// intOff turns interrupts off by clearing the current enable bit of the
// CPU's status register, without the kernel's system call, which costs a
// thousand cycles each way, and returns the register as it was for
// intRestore. The library's own short critical sections use it.
func intOff() uint32 {
	sr := uint32(device.AsmFull(`.set push
	.set noat
	mfc0 {}, $$12
	nop
	.set pop`, nil))
	device.AsmFull(`.set push
	.set noat
	mtc0 {v}, $$12
	nop
	nop
	.set pop`, map[string]interface{}{"v": sr &^ 1})
	return sr
}

// intRestore turns interrupts back on if they were on when intOff
// returned sr.
func intRestore(sr uint32) {
	if sr&1 == 0 {
		return
	}
	cur := uint32(device.AsmFull(`.set push
	.set noat
	mfc0 {}, $$12
	nop
	.set pop`, nil))
	device.AsmFull(`.set push
	.set noat
	mtc0 {v}, $$12
	nop
	nop
	.set pop`, map[string]interface{}{"v": cur | 1})
}

// ExitCriticalSection turns interrupts back on through the kernel.
//
//export ExitCriticalSection
func ExitCriticalSection() {
	libps_syscall(2)
}

// putchar writes c to the TTY through the kernel's write(1, &c, 1) and
// returns what the write returned. A "\n" goes as "\r" then "\n" (the kernel's write adds a
// "\r" of its own before a "\n", so a line ends "\r\r\n" on the TTY).
//
// Sony's putchar does the same (iotrace of the seqhdr probe built on
// Sony's library: B0 35h from its putchar for every character the
// program prints, two calls 28 cycles apart for a "\n"), not the kernel's
// putchar (B0 3Dh), which takes 4,722 cycles a call against write's
// 4,182: through it a long line printed ten percent slower and the
// program's next calls fell a frame behind Sony's. Between write's return
// and the next call Sony's library and the program's print loop spend 68
// to 84 cycles (76 the most often, 76 on average); keeping the column takes
// that time here, with no pad. Sony's "\r" and "\n" are 28
// cycles apart; here about 32. A tab goes as spaces, by ttyColumn. Only the low
// byte of c counts.
//
//export putchar
func putchar(c int32) int32

// putcharBelowSpace is putchar for a byte below a space, from its assembly.
//
//export libps_putchar_other
func putcharBelowSpace(c int32) int32 {
	b := byte(c)
	if putcharOther(b) {
		return putcharLast
	}
	volatile.StoreUint8(&ttyChar, b)
	return libps_biosB(biosWrite, 1, uintptr(unsafe.Pointer(&ttyChar)), 1, 0)
}

// putcharLast is what the last write of a tab returned, for putchar to return.
var putcharLast int32

// putcharOther does what comes before the write of a byte that is not plain text,
// or for a tab all of it, when it reports true. It is apart from putchar so that
// the plain text, what a program mostly prints, saves no registers for its loop.
//
//go:noinline
func putcharOther(b byte) bool {
	switch {
	case b == '\n':
		ttyColumn = 0
		volatile.StoreUint8(&ttyChar, '\r')
		libps_biosB(biosWrite, 1, uintptr(unsafe.Pointer(&ttyChar)), 1, 0)
	case b == '\t':
		// As spaces to the next stop of 8, always at least one.
		for {
			volatile.StoreUint8(&ttyChar, ' ')
			putcharLast = libps_biosB(biosWrite, 1, uintptr(unsafe.Pointer(&ttyChar)), 1, 0)
			ttyColumn++
			if ttyColumn&7 == 0 {
				return true
			}
		}
	}
	return false
}

// ttyColumn is the column putchar has reached, for a tab: only its own printing
// characters move it, and only a newline sets it back. Backspace and a carriage
// return leave it, and write and putc go around it (probe spec/probes/ctab).
//
//go:extern libps_tty_column
var ttyColumn uint32

// ttyPrintf prints format to the TTY (file 1) as the reference library's
// printf does: formatting as it goes, one character a call to the
// kernel's write, a "\n" written as "\r\n". The directives are %08x, %x
// (without leading zeros) and %d (signed), which take a and then b.
func ttyPrintf(format string, a, b uint32) {
	args := [2]uint32{a, b}
	k := 0
	for i := 0; i < len(format); i++ {
		c := format[i]
		switch c {
		case '%':
			v := args[k]
			k++
			switch format[i+1] {
			case 'd':
				i++
				ttyDecimal(int32(v))
			case 'x':
				i++
				ttyHexShort(v)
			default:
				i += 3 // 08x
				ttyHex(v)
			}
			// What the reference spends after a conversion, past the
			// next character's own time.
			ttyAfterConvPad()
		case '\n':
			ttyNewlinePad()
			ttyPut('\r')
			ttyPut('\n')
		default:
			ttyPlainPad()
			ttyPut(c)
		}
	}
}

// ttyHex writes v as eight hexadecimal digits, lower case.
func ttyHex(v uint32) {
	ttyFirstDigPad()
	for i := 28; i >= 0; i -= 4 {
		if i != 28 {
			ttyDigitPad()
		}
		ttyPut("0123456789abcdef"[v>>uint(i)&15])
	}
}

// ttyHexShort writes v in hexadecimal, lower case, without leading zeros.
func ttyHexShort(v uint32) {
	n := 28
	for n > 0 && v>>uint(n) == 0 {
		n -= 4
	}
	ttyFirstDigPad()
	for i := n; i >= 0; i -= 4 {
		if i != n {
			ttyDigitPad()
		}
		ttyPut("0123456789abcdef"[v>>uint(i)&15])
	}
}

// ttyDecimal writes v in decimal, with a minus sign if it is negative.
func ttyDecimal(v int32) {
	var d [11]byte
	n := 0
	u := uint32(v)
	if v < 0 {
		u = uint32(-v)
	}
	for {
		d[n] = byte('0' + u%10)
		n++
		u /= 10
		if u == 0 {
			break
		}
	}
	if v < 0 {
		d[n] = '-'
		n++
	}
	ttyFirstDigPad()
	for i := n - 1; i >= 0; i-- {
		if i != n-1 {
			ttyDigitPad()
		}
		ttyPut(d[i])
	}
}

// ttyChar is the character ttyPut hands the kernel.
//
//go:extern libps_tty_char
var ttyChar byte

// ttyPut writes c to the TTY through the kernel's write(1, &c, 1).
//
//go:noinline
func ttyPut(c byte) {
	volatile.StoreUint8(&ttyChar, c)
	libps_biosB(biosWrite, 1, uintptr(unsafe.Pointer(&ttyChar)), 1, 0)
}

// exitBuf is what the kernel resumes with at the end of an interrupt, as
// SetCustomExitFromException takes it: the return address, the stack and
// frame pointers, s0 to s7 and gp, restored before the jump.
type exitBuf struct {
	ra, sp, fp uint32
	s          [8]uint32
	gp         uint32
}

// libStarted is 1 once libStart has run. VSync's assembly reads it.
//
//go:extern libps_lib_started
var libStarted uint32

var intr struct {
	exit exitBuf

	// stack is where the handler runs: a stack of the library's own, so
	// that an interrupt never runs on whatever the program's was. It holds
	// the handler twice over, with the vertical blank's callback, since the
	// handler runs again inside itself after sending images queued behind a
	// drawing (lateInterrupts, spec 41).
	stack [192]uint64
}

// libStart starts the library, once: it takes over the end of every
// interrupt, so that libps_interrupt sees the vertical blank and the end
// of each drawing request, has the kernel leave the vertical blank and the
// controllers' acknowledgement to it, removes the kernel's CD handlers and
// starts the counters VSync reads. The reference does this in its first
// ResetGraph(0), with these kernel calls in this order; the library's
// other entry points that need it call it too, for a program that starts
// without ResetGraph.
func libStart() {
	if libStarted == 0 {
		libBegin()
	}
}

// libBegin is libStart's work, apart so that a call after the start costs no
// stack.
//
//go:noinline
func libBegin() {
	libStarted = 1
	// Sony's sets every DMA channel's priority to 3 and disables them all
	// first; ResetGraph, the sound and GsClearOt enable their own after
	// (probe spec/probes/dmaleft, spec 45). The kernel leaves 00009099h.
	dmaPriority.Set(dmaPriorityStart)
	EnterCriticalSection()
	intr.exit = exitBuf{
		ra: uint32(uintptr(unsafe.Pointer(&libpsIntrEntry))),
		sp: uint32(uintptr(unsafe.Pointer(&intr.stack[len(intr.stack)-1]))),
	}
	libps_biosB(biosSetCustomExit, uintptr(unsafe.Pointer(&intr.exit)), 0, 0, 0)
	// The reference clears a 64-byte table of its own here, a byte at a
	// time, which the clean library does not have (spec 08), sets timer
	// 1 counting lines and masks every interrupt.
	libTablePad()
	lineMode.Set(0x0107)
	iMask.Get()
	iMask.Set(0)
	libps_biosB(biosClearPad, 0, 0, 0, 0)
	// The kernel would otherwise acknowledge the vertical blank itself,
	// before the handler could count it.
	libps_biosC(biosClearRCnt, 3, 0, 0, 0)
	// Then the vertical blank alone, a second table of 64 bytes, no DMA
	// interrupts, and the vertical blank's and the DMA's.
	iMask.Set(irqVBlank)
	libTablePad()
	dmaInterrupt.Set(0)
	iMask.Set(0)
	iMask.Set(irqVBlank | irqDMA)
	libps_biosA(biosCdRemove, 0, 0, 0, 0)
	startQueue()
	iStat.Set(^uint32(irqVBlank))
	iMask.Set(iMask.Get() | irqVBlank)
	ExitCriticalSection()
}

// dmaPriorityStart is DPCR as the library's start leaves it.
const dmaPriorityStart = 0x33333333

// The 2-cycle instructions the vertical blank's path spends before
// acknowledging the blank and before counting it, to Sony's cycles (spec
// 45). They were all stores; on a console a store costs the handler more
// than that, since its code runs cold at every blank and a line missed
// while stores drain waits for them (spec 46), so now the ones before the
// acknowledgment and some before the count are nops, which cost the same
// with the emulator's costs off and a console's cold cycles less.
const (
	vblankAckNops    = 4
	vblankCountSpace = 10
	vblankCountNops  = 8
)

// The 2-cycle instructions libps_interrupt spends at its start and its
// end: the time its prologue and epilogue took when the vertical blank's
// work was compiled into it, before spec 45 put that in vblankInterrupt,
// for imageBlank to share. Both are nops now (spec 46, as above): the
// end's were stores, which the kernel's way out then missed its lines
// behind; the end has two more for the stores' address loads that are gone
// (TestVSyncCount and vsyncpoll's waits hold the handler's cycles with the
// costs off).
const (
	interruptNopsIn  = 5
	interruptNopsOut = 8
)

// vblankInterrupt handles the vertical blank: it acknowledges it, counts
// it, calls the program's callback and runs the sound's tick. Sony's
// handler acknowledges the blank 38 cycles after its read of I_STAT and
// counts it 52 after that, its callback 28 later (probe
// spec/probes/vsynccount, spec 45): the spaces put the clean handler's
// there, and soundTick takes the time back.
//
//go:inline
func vblankInterrupt() {
	nops(vblankAckNops)
	iStat.Set(^uint32(irqVBlank))
	irqStamp(2)
	space(vblankCountSpace)
	nops(vblankCountNops)
	volatile.StoreUint32(&vsyncBlanks, volatile.LoadUint32(&vsyncBlanks)+1)
	irqStamp(3)
	if f := volatile.LoadUint32(&vsync.callback); f != 0 {
		callC(uintptr(f))
	}
	irqStamp(4)
	if snd.started {
		soundTick()
	}
}

// libps_interrupt handles an interrupt, after the kernel's own handlers
// have run. It must not allocate, and returns quickly. Each source is
// acknowledged before it is handled, so that one raised again meanwhile
// is not lost.
//
//export libps_interrupt
func libps_interrupt() {
	irqStamp(0)
	nops(interruptNopsIn)
	stat := iStat.Get() & iMask.Get()
	irqStamp(1)
	if stat&irqVBlank != 0 {
		vblankInterrupt()
	}
	if stat&irqDMA != 0 {
		iStat.Set(^uint32(irqDMA))
	}
	// The DMA channels are checked whatever I_STAT says, in case the
	// kernel acknowledged the interrupt first.
	spuDMADone()
	handlerDrawDone()
	// The drive's interrupt is taken again while it is raised: the
	// handler waits for the drive's first answer to the Pause after a
	// read, which raises it while the handler runs, as Sony's library
	// does.
	for stat&irqCD != 0 {
		iStat.Set(^uint32(irqCD))
		cd.interrupt()
		stat = iStat.Get() & iMask.Get()
	}
	nops(interruptNopsOut)
	irqStamp(11)
}

// rand's state is 0 until srand, in the target's assembly with it. The kernel has a
// rand and a srand, A0h 2Fh and 30h, which the library does not call: a retail kernel's
// sequence starts elsewhere (probe spec/probes/crand, and Terra Incognita's scenery).

// rand returns the next pseudo-random number, from 0 to 32767.
//
//export rand
func rand() int32

// srand seeds rand.
//
//export srand
func srand(seed uint32)

// callC calls the C function at f, which takes no arguments, as a Go
// func value whose context is nil: the function ignores the register the
// context goes in.
func callC(f uintptr) {
	var fn func()
	(*[2]uintptr)(unsafe.Pointer(&fn))[1] = f
	fn()
}

// FlushCache empties the instruction cache through the kernel, after code
// in memory has changed. Sony's library exports it under this name, and the
// yaroze package binds it under the same name.
//
//export FlushCache
func FlushCache() {
	libps_biosA(biosFlushCache, 0, 0, 0, 0)
}

// FlushCashe is FlushCache under the header's spelling.
//
//export FlushCashe
func FlushCashe() {
	FlushCache()
}
