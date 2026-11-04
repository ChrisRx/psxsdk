//go:build yaroze

package libps

import "unsafe"

// Interrupts, as libetc's intr.c (v1.73) handles them. The kernel takes
// every exception first; the library asks it, with
// setCustomExitFromException, to end each one by restoring a jmp_buf of
// the library's, whose return address is a trampoline that calls
// interrupt on a stack of the library's own and then returns from the
// exception. interrupt acknowledges each pending interrupt that has a
// callback and calls it. The callbacks are what the rest of the library
// hangs off: the vertical blank for VSync, DMA completion for the GPU
// queue.

// A jmpBuf is the kernel's: the return address, the stack pointer, the
// frame pointer, s0 to s7 and gp.
type jmpBuf [12]uint32

// The dispatcher's state.
//
// libps 0x8004D938, 0x41A words
var (
	intrReady   uint16 // set once ResetCallback has installed the handler
	inInterrupt uint16 // set while interrupt runs
	// intrCallbacks are the handlers by interrupt number: 0 the vertical
	// blank, 3 DMA, 4 to 6 the root counters.
	intrCallbacks [11]func()
	intrEnabled   uint16 // which interrupts have a callback
	savedMask     uint16 // the mask StopCallback saved
	savedDPCR     uint32
	intrJmp       jmpBuf
	intrStack     [1015]uint32
	intrTimeouts  int32 // libps 0x8004E9D0: interrupts left pending in a row
)

// The library's interrupt entry, in targets/playstation-yaroze-bios.S:
// what the jmp_buf returns to. Its address is taken through an extern
// variable, which is the symbol itself.
//
//go:extern libps_intr_entry
var intrEntry [0]byte

// ResetCallback installs the library's handler, once: every interrupt
// masked and acknowledged, the DMA priorities set, the callbacks cleared,
// the jmp_buf pointed at the trampoline and the library's stack, the
// vertical blank counting and the DMA callbacks dispatching, and the
// kernel's CD-ROM handlers removed.
//
// libps 0x8003CDE4, 220 bytes
func ResetCallback() {
	if intrReady != 0 {
		return
	}
	iMask.Set(0)
	iStat.Set(iMask.Get())
	dpcr.Set(0x33333333)
	for i := range intrCallbacks {
		intrCallbacks[i] = nil
	}
	intrEnabled = 0
	intrJmp = jmpBuf{}
	intrJmp[0] = uint32(uintptr(unsafe.Pointer(&intrEntry)))
	// The stack pointer has to be 8-byte aligned, as the compiler's code
	// assumes: an array's end is not, in general.
	end := uintptr(unsafe.Pointer(&intrStack[0])) + uintptr(len(intrStack))*4
	intrJmp[1] = uint32(end&^15 - 16)
	setCustomExitFromException(&intrJmp)
	intrReady = 1
	startVSync()
	startDMA()
	cdRemove()
	ExitCriticalSection()
}

// funcWords is a function value as its two words, for comparing two:
// Go does not compare functions, and the library keeps callbacks by
// identity.
//
//go:inline
func funcWords(f func()) [2]uintptr {
	return *(*[2]uintptr)(unsafe.Pointer(&f))
}

// interrupt is the handler: it runs on the library's stack with the
// interrupted program's registers held by the kernel, acknowledges each
// pending interrupt that has a callback and calls it, as long as any is
// pending, and reports an interrupt nothing acknowledges when it has been
// pending for 2049 handlers in a row.
//
// libps 0x8003CEC0, 488 bytes
//
//export libps_interrupt
func interrupt() {
	if intrReady == 0 {
		kprintf("unexpected interrupt(%04x)\n", uint32(iStat.Get()))
		return
	}
	inInterrupt = 1
	for {
		pending := iMask.Get() & intrEnabled & iStat.Get()
		if pending == 0 {
			break
		}
		for i := 0; i < len(intrCallbacks) && pending != 0; i++ {
			if pending&1 != 0 {
				iStat.Set(^uint16(1 << i))
				if cb := intrCallbacks[i]; cb != nil {
					cb()
				}
			}
			pending >>= 1
		}
	}
	if iStat.Get()&iMask.Get() != 0 {
		intrTimeouts++
		if intrTimeouts > 2048 {
			kprintf("intr timeout(%04x:%04x)\n", uint32(iStat.Get()), uint32(iMask.Get()))
			intrTimeouts = 0
			iStat.Set(0)
		}
	} else {
		intrTimeouts = 0
	}
	inInterrupt = 0
}

// InterruptCallback sets the callback for interrupt irq and returns the
// one before. The interrupt is let through while it has a callback. For
// the vertical blank and the root counters, the kernel's own handlers are
// told whether to acknowledge the interrupt themselves.
//
// libps 0x8003D0A8, 340 bytes
func InterruptCallback(irq int32, fn func()) func() {
	old := intrCallbacks[irq]
	if sameFunc(fn, old) || intrReady == 0 {
		return old
	}
	mask := iMask.Get()
	iMask.Set(0)
	bit := uint16(1) << irq
	if fn != nil {
		intrCallbacks[irq] = fn
		intrEnabled |= bit
		mask |= bit
	} else {
		intrCallbacks[irq] = nil
		mask &^= bit
		intrEnabled &^= bit
	}
	clear := int32(0)
	if fn == nil {
		clear = 1
	}
	switch irq {
	case 0:
		changeClearPAD(clear)
		changeClearRCnt(3, clear)
	case 4:
		changeClearRCnt(0, clear)
	case 5:
		changeClearRCnt(1, clear)
	case 6:
		changeClearRCnt(2, clear)
	}
	iMask.Set(mask)
	return old
}

// sameFunc reports whether two function values are the same function.
func sameFunc(a, b func()) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return funcWords(a) == funcWords(b)
}

// SetIntrMask sets the interrupt mask and returns the one before.
//
// libps 0x8003CDC8, 28 bytes
func SetIntrMask(mask uint16) uint16 {
	old := iMask.Get()
	iMask.Set(mask)
	return old
}

// StopCallback takes the library's handler out: the mask and DMA
// priorities are saved and cleared and the kernel ends exceptions its own
// way again, until RestartCallback.
//
// libps 0x8003D1FC, 172 bytes
func StopCallback() {
	if intrReady == 0 {
		return
	}
	EnterCriticalSection()
	savedMask = iMask.Get()
	savedDPCR = dpcr.Get()
	iMask.Set(0)
	iStat.Set(iMask.Get())
	dpcr.Set(dpcr.Get() & 0x77777777)
	setDefaultExitFromException()
	intrReady = 0
}

// RestartCallback puts the handler back after StopCallback.
//
// libps 0x8003D2A8, 136 bytes
func RestartCallback() {
	if intrReady != 0 {
		return
	}
	setCustomExitFromException(&intrJmp)
	intrReady = 1
	iMask.Set(savedMask)
	dpcr.Set(savedDPCR)
	ExitCriticalSection()
}

// The vertical blank: a count of them since the handler was installed,
// and up to eight callbacks run on each, the first of which is the
// program's VSyncCallback, a C function by its address.
//
// libps 0x8004E9D4 and 0x8004E9F4
var (
	vsyncUser      uintptr
	vsyncCallbacks [8]func()
	vsyncCount     int32
)

// callC calls the C function at fn, with no arguments: a program's
// callback, given as an address. In targets/playstation-yaroze-bios.S.
//
//export libps_callC
func callC(fn uintptr)

// startVSync counts vertical blanks: root counter 1 is set to count
// horizontal blanks for VSync's report, and the vertical blank interrupt
// gets the counting callback.
//
// libps 0x8003D35C, 88 bytes
func startVSync() {
	vsyncUser = 0
	for i := range vsyncCallbacks {
		vsyncCallbacks[i] = nil
	}
	timer1Mode.Set(0x107)
	vsyncCount = 0
	InterruptCallback(0, onVBlank)
}

// onVBlank counts the vertical blank and runs the callbacks.
//
// libps 0x8003D3B4, 120 bytes
func onVBlank() {
	vsyncCount++
	if vsyncUser != 0 {
		callC(vsyncUser)
	}
	// Indexed: ranging over the array would copy its eight function
	// values first, some 400 instructions on every vertical blank.
	for i := range vsyncCallbacks {
		if cb := vsyncCallbacks[i]; cb != nil {
			cb()
		}
	}
}

// setVSyncCallback sets vertical blank callback i.
//
// libps 0x8003D42C, 44 bytes
func setVSyncCallback(i int32, fn func()) {
	if !sameFunc(vsyncCallbacks[i], fn) {
		vsyncCallbacks[i] = fn
	}
}

// VSyncCallback has the C function at fn called on every vertical blank,
// or none with 0: the first of the eight callbacks. The binding passes
// the address.
//
// libps 0x8003CCDC, 52 bytes
//
//export VSyncCallback
func VSyncCallback(fn uintptr) {
	vsyncUser = fn
}

// DMA completion: a callback per channel, run from the DMA interrupt.
//
// libps 0x8004EA00
var dmaCallbacks [8]func()

// startDMA clears the DMA callbacks and the DMA interrupt register and
// gives the DMA interrupt its dispatcher.
//
// libps 0x8003D484, 80 bytes
func startDMA() {
	for i := range dmaCallbacks {
		dmaCallbacks[i] = nil
	}
	dicr.Set(0)
	InterruptCallback(3, onDMA)
}

// onDMA runs the callback of each channel that has finished, acknowledging
// it, as long as any has, and reports a bus error.
//
// libps 0x8003D4D4, 388 bytes
func onDMA() {
	for {
		flags := (dicr.Get() >> 24) & 0x7f
		if flags == 0 {
			break
		}
		for i := 0; i < 7 && flags != 0; i++ {
			if flags&1 != 0 {
				dicr.Set((dicr.Get() & 0xffffff) | 1<<(24+i))
				if cb := dmaCallbacks[i]; cb != nil {
					cb()
				}
			}
			flags >>= 1
		}
	}
	if d := dicr.Get(); d&0xff000000 == 0x80000000 || d&0x8000 != 0 {
		kprintf("DMA bus error: code=%08x\n", d)
		for i := 0; i < 7; i++ {
			kprintf("MADR[%d]=%08x\n", uint32(i), dmaMADR(i).Get())
		}
	}
}

// DMACallback sets the callback run when channel ch finishes, enabling
// the channel's interrupt, and returns the one before; nil disables it.
//
// libps 0x8003D658, 168 bytes
func DMACallback(ch int32, fn func()) func() {
	old := dmaCallbacks[ch]
	if sameFunc(fn, old) {
		return old
	}
	if fn != nil {
		dmaCallbacks[ch] = fn
		dicr.Set((dicr.Get() & 0xffffff) | 1<<(16+ch) | 0x800000)
	} else {
		dmaCallbacks[ch] = nil
		dicr.Set(((dicr.Get() & 0xffffff) | 0x800000) &^ (1 << (16 + ch)))
	}
	return old
}
