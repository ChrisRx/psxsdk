//go:build yaroze

package libps

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The hardware the library drives, as registers read and written every
// time. The library keeps these addresses in tables of its own (libps
// 0x800483B4 for the GPU and DMA, 0x8004E9C4 for the interrupt
// controller, 0x8004D928 for VSync's); here they are constants.

// at makes a pointer to a fixed address. uintptr(nil)+addr folds to the
// address itself, where unsafe.Add(nil, addr) would be a pointer derived
// from nil, which LLVM takes for memory it may fold loads from.
func at(addr uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(unsafe.Pointer(nil)) + addr)
}

func reg32(addr uintptr) *volatile.Register32 { return (*volatile.Register32)(at(addr)) }
func reg16(addr uintptr) *volatile.Register16 { return (*volatile.Register16)(at(addr)) }
func reg8(addr uintptr) *volatile.Register8   { return (*volatile.Register8)(at(addr)) }

// spuReg is a register of the sound processor, by its offset from
// 0x1f801c00.
func spuReg(off uintptr) *volatile.Register16 { return reg16(0x1f801c00 + off) }

var (
	// The interrupt controller: which interrupts are pending and which
	// are let through, both halfwords.
	iStat = reg16(0x1f801070)
	iMask = reg16(0x1f801074)

	// DMA: the channel priorities and the interrupt register.
	dpcr = reg32(0x1f8010f0)
	dicr = reg32(0x1f8010f4)

	// The GPU: commands and data in through GP0 and read back through it,
	// control through GP1 and the status read back through it.
	gp0 = reg32(0x1f801810)
	gp1 = reg32(0x1f801814)

	// DMA channel 2, the GPU's, and channel 6, the ordering table clear.
	dma2MADR = reg32(0x1f8010a0)
	dma2BCR  = reg32(0x1f8010a4)
	dma2CHCR = reg32(0x1f8010a8)
	dma6MADR = reg32(0x1f8010e0)
	dma6BCR  = reg32(0x1f8010e4)
	dma6CHCR = reg32(0x1f8010e8)

	// Root counter 1 counts horizontal blanks, which VSync reports.
	timer1Count = reg32(0x1f801110)
	timer1Mode  = reg32(0x1f801114)

	// The CD-ROM: the index at 0x1f801800 selects what the other three
	// bytes are. Its bus timing is at 0x1f801018 and the common one at
	// 0x1f801020, and its data comes over DMA channel 3.
	cdIndex  = reg8(0x1f801800)
	cdReg1   = reg8(0x1f801801)
	cdReg2   = reg8(0x1f801802)
	cdReg3   = reg8(0x1f801803)
	cdDelay  = reg32(0x1f801018)
	comDelay = reg32(0x1f801020)
	dma3MADR = reg32(0x1f8010b0)
	dma3BCR  = reg32(0x1f8010b4)
	dma3CHCR = reg32(0x1f8010b8)
)

// dmaMADR is channel n's address register, for the bus error report.
func dmaMADR(n int) *volatile.Register32 { return reg32(0x1f801080 + uintptr(n)*16) }

// GPU status bits.
const (
	gpuReadyForCommands = 0x04000000 // GPUSTAT: ready to receive a command word
	gpuReadyForDMA      = 0x10000000
	gpuReadyToSend      = 0x08000000 // GPUSTAT: ready to send video memory to the CPU
	gpuInterlaced       = 0x00080000 // vertical resolution 480, drawn in fields
	gpuOddField         = 0x80000000 // the field being shown
	dmaBusy             = 0x01000000 // CHCR: a transfer is running
)
