//go:build yaroze && irqstamp

package cleanlib

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// Timer 2 stamps inside the vertical blank interrupt, for spec 46: an image
// built with the irqstamp tag (TGOFLAGS=-tags=irqstamp exp/cleanlib/image/make.sh)
// is for the measurement only, never shipped. Each stamp reads timer 2 and
// stores it in the scratchpad's last 16 words, 1F8003C0h on, which the library
// never uses otherwise; spec/probes/irqstamp reads them after each blank. A
// stamp is a load from the counter and a store to the scratchpad, some 6
// cycles, the same in the emulator and on a console.
//
// The stamps, in the order the handler makes them:
//
//	0 libps_interrupt's start
//	1 after its reads of I_STAT and I_MASK
//	2 vblankInterrupt, after the acknowledgment
//	3 after the count, before the callback (the probe's own callback stamp follows)
//	4 soundTick's start, after the callback
//	5 after its pace, before the envelope reads
//	6 after the envelope reads
//	7 after the voice writes
//	8 after tickWordsTime, before the key words
//	9 after the key words and the sequences, before the exit pace
//	10 soundTick's end
//	11 libps_interrupt's end, before the kernel's way out
const irqStampBase = 0x1f8003c0

// irqStamp stores timer 2 in stamp i.
//
//go:inline
func irqStamp(i int) {
	t := volatile.LoadUint32((*uint32)(unsafe.Pointer(uintptr(0x1f801120))))
	volatile.StoreUint32((*uint32)(unsafe.Pointer(uintptr(irqStampBase+4*i))), t)
}
