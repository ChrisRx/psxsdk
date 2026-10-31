//go:build yaroze

package libps

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The sound processor, as libspu drives it for the sequencer: its reset,
// transfers into its memory over DMA channel 4 or through its data port,
// the allocation of that memory, the common volumes, and the reverb.

// The sound processor's registers, by offset from 0x1f801c00: a voice's
// at voice*16, and the common ones from 0x180.
const (
	spuMainVolL   = 0x180
	spuMainVolR   = 0x182
	spuRevVolL    = 0x184
	spuRevVolR    = 0x186
	spuKeyOnLo    = 0x188
	spuKeyOnHi    = 0x18a
	spuKeyOffLo   = 0x18c
	spuKeyOffHi   = 0x18e
	spuFMLo       = 0x190
	spuNoiseLo    = 0x194
	spuNoiseHi    = 0x196
	spuRevOnLo    = 0x198
	spuRevOnHi    = 0x19a
	spuRevWork    = 0x1a2
	spuTransAddr  = 0x1a6
	spuData       = 0x1a8
	spuCnt        = 0x1aa
	spuTransCtl   = 0x1ac
	spuStat       = 0x1ae
	spuCDVolL     = 0x1b0
	spuCDVolR     = 0x1b2
	spuExtVolL    = 0x1b4
	spuExtVolR    = 0x1b6
	spuCurVolL    = 0x1b8
	spuCurVolR    = 0x1ba
	spuRevRegs    = 0x1c0
	spuRAMSize    = 0x80000
	spuWriteLimit = 0x7f000
)

// The DMA channel the sound processor is on, and its bus timing.
var (
	spuDelay = reg32(0x1f801014)
	dma4MADR = reg32(0x1f8010c0)
	dma4BCR  = reg32(0x1f8010c4)
	dma4CHCR = reg32(0x1f8010c8)
)

// The driver's state.
//
// libps 0x80046DEC and 0x8004724C on
var (
	spuTransferMode int32          // 0x80046DEC: 0 for DMA, 1 for the data port
	spuIOMode       int32          // 0x8004726C: whether transfers go through the data port
	spuTransferAt   uint16         // 0x80047250: the transfer address in units of 8 bytes
	spuMemMode      int32      = 2 // 0x80047274
	spuAddrShift    uint32     = 3 // 0x80047278: bytes to units
	spuAlign        uint32     = 8 // 0x8004727C
	spuAlignMask    uint32     = 7 // 0x80047280
	spuTransferred  int32      = 1 // 0x80047284: no transfer is in progress
	spuTransferCB   func()         // 0x80047288: the program's, told when a transfer ends
	spuIRQCB        func()         // 0x80046DE8
	spuEventsOpened bool           // 0x80047248
	spuEvent        uint32         // 0x80046E40: the kernel event a finished transfer delivers
	spuTimeouts     int32          // 0x8004724C
	spuDMARead      int32          // 0x800472A0: the direction of the transfer being set up
	spuDMAAddr      uint32         // 0x800472A4
	spuDMABlocks    uint32         // 0x800472A8
	spuVoiceCenter  [24]uint16     // 0x80046E10: each voice's center note, unused by the sequencer

	// The reverb's state.
	spuRevOn       int32       // 0x80046DF0
	spuRevReserved int32       // 0x80046DF4: a work area reserved ahead, which nothing here does
	spuRevWorkAt   uint32      // 0x80046DF8: the work area's start in units of 8 bytes
	spuRevMode     int32       // 0x80046E00
	spuRevDepthL   int16       // 0x80046E04
	spuRevDepthR   int16       // 0x80046E06
	spuRevDelay    int32       // 0x80046E08
	spuRevFeedback int32       // 0x80046E0C
	spuRevBlock    [34]uint16  // the register block being set, 68 bytes
	spuZeros       [256]uint32 // 0x80046E48: a kilobyte of zeros to clear the work area with
)

// spuWait is the pause the original took between register writes: a
// loop of 240 steps of a multiply, kept from being folded away by a
// volatile store each step.
var spuSink uint32

func spuWait() {
	v := uint32(13)
	for i := 0; i < 240; i++ {
		v *= 3
		volatile.StoreUint32(&spuSink, v)
	}
}

// spuInit resets the processor and the driver: the registers, the voices,
// the DMA callback and the event, and the reverb.
//
// libps 0x8001B490, 200 bytes
func spuInit() {
	ResetCallback()
	spuInitRegs(false)
	for i := range spuVoiceCenter {
		spuVoiceCenter[i] = 0xc000
	}
	spuInitEvents()
	spuRevOn = 0
	spuRevReserved = 0
	spuRevMode = 0
	spuRevDepthL = 0
	spuRevDepthR = 0
	spuRevDelay = 0
	spuRevFeedback = 0
	spuRevWorkAt = spuRevSizes[0]
	spuWriteReg(0xd1, uint16(spuRevSizes[0]), false)
	spuTransferMode = 0
	spuIOMode = 0
}

// spuInitEvents takes the DMA channel's completion and opens the event a
// transfer delivers, once.
//
// libps 0x8001B558, 272 bytes
func spuInitEvents() {
	if spuEventsOpened {
		return
	}
	spuEventsOpened = true
	EnterCriticalSection()
	spuIRQCB = nil
	DMACallback(4, spuDMADone)
	spuEvent = openEvent(0xf0000009, 0x20, 0x2000, 0)
	enableEvent(spuEvent)
	ExitCriticalSection()
}

// spuInitRegs resets the registers: the volumes off, the processor
// disabled until it reports idle, the keys off, and unless hot, every
// voice set to a silent block the voices are started and stopped on.
//
// libps 0x8001B668, 1308 bytes
func spuInitRegs(hot bool) {
	dpcr.Set(dpcr.Get() | 0xb0000)
	spuReg(spuMainVolL).Set(0)
	spuReg(spuMainVolR).Set(0)
	spuReg(spuCnt).Set(0)
	spuIOMode = 0
	spuDMARead = 0
	spuTransferAt = 0
	spuWait()
	spuReg(spuMainVolL).Set(0)
	spuReg(spuMainVolR).Set(0)
	spuTimeouts = 0
	for spuReg(spuStat).Get()&0x7ff != 0 {
		spuTimeouts++
		if spuTimeouts >= 5001 {
			kprintf("SPU:T/O [%s]\n", "wait (reset)")
			break
		}
	}
	spuMemMode = 2
	spuAddrShift = 3
	spuAlign = 8
	spuAlignMask = 7
	spuReg(spuTransCtl).Set(4)
	spuReg(spuRevVolL).Set(0)
	spuReg(spuRevVolR).Set(0)
	spuReg(spuKeyOffLo).Set(0xffff)
	spuReg(spuKeyOffHi).Set(0xffff)
	spuReg(spuRevOnLo).Set(0)
	spuReg(spuRevOnHi).Set(0)
	if !hot {
		for off := uintptr(spuFMLo); off <= spuNoiseHi; off += 2 {
			spuReg(off).Set(0)
		}
		for off := uintptr(spuCDVolL); off <= spuExtVolR; off += 2 {
			spuReg(off).Set(0)
		}
		spuTransferAt = 0x200
		spuWriteIO(unsafe.Pointer(&spuSilence[0]), 16)
		for v := uintptr(0); v < 24; v++ {
			spuReg(v * 16).Set(0)
			spuReg(v*16 + 2).Set(0)
			spuReg(v*16 + 4).Set(0x3fff)
			spuReg(v*16 + 6).Set(0x200)
			spuReg(v*16 + 8).Set(0)
			spuReg(v*16 + 10).Set(0)
		}
		spuReg(spuKeyOnLo).Set(0xffff)
		spuReg(spuKeyOnHi).Set(spuReg(spuKeyOnHi).Get() | 0xff)
		for range 4 {
			spuWait()
		}
		spuReg(spuKeyOffLo).Set(0xffff)
		spuReg(spuKeyOffHi).Set(spuReg(spuKeyOffHi).Get() | 0xff)
		for range 4 {
			spuWait()
		}
	}
	spuTransferred = 1
	spuReg(spuCnt).Set(0xc000)
	spuTransferCB = nil
}

// spuSilence is the block the voices are reset on: sixteen bytes that
// decode to silence and end.
//
// libps 0x80047290
var spuSilence = [16]byte{7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7}

// spuWriteIO writes size bytes from src to the processor's memory at the
// transfer address through its data port, 64 bytes at a time.
//
// libps 0x8001BB84, 820 bytes
func spuWriteIO(src unsafe.Pointer, size int32) {
	spuReg(spuTransAddr).Set(spuTransferAt)
	mode := spuReg(spuStat).Get() & 0x7ff
	spuWait()
	for size > 0 {
		n := size
		if n > 64 {
			n = 64
		}
		for i := int32(0); i < n; i += 2 {
			spuReg(spuData).Set(*(*uint16)(src))
			src = unsafe.Add(src, 2)
		}
		spuReg(spuCnt).Set(spuReg(spuCnt).Get()&0xffcf | 0x10)
		spuWait()
		spuTimeouts = 0
		for spuReg(spuStat).Get()&0x400 != 0 {
			spuTimeouts++
			if spuTimeouts >= 5001 {
				kprintf("SPU:T/O [%s]\n", "wait (wrdy H -> L)")
				break
			}
		}
		spuWait()
		spuWait()
		size -= n
	}
	spuReg(spuCnt).Set(spuReg(spuCnt).Get() & 0xffcf)
	spuTimeouts = 0
	for spuReg(spuStat).Get()&0x7ff != mode {
		spuTimeouts++
		if spuTimeouts >= 5001 {
			kprintf("SPU:T/O [%s]\n", "wait (dmaf clear/W)")
			break
		}
	}
}

// spuDMADone runs when the DMA channel finishes: the transfer mode is
// taken off the processor, and the program's callback is run, or the
// event delivered.
//
// libps 0x8001BEB8, 416 bytes
func spuDMADone() {
	if spuDMARead == 0 {
		spuWait()
		spuWait()
		spuWait()
	}
	spuReg(spuCnt).Set(spuReg(spuCnt).Get() & 0xffcf)
	for i := 0; i < 3840 && spuReg(spuCnt).Get()&0x30 != 0; i++ {
	}
	if spuTransferCB != nil {
		spuTransferCB()
	} else {
		deliverEvent(0xf0000009, 0x20)
	}
}

// spuTransfer is a step of a DMA transfer: 2 sets the address, 1 puts the
// processor into DMA write and 0 into DMA read, each waiting for the
// address to be taken, and 3 starts the channel on size bytes at p. It
// returns -2 when a wait ran out.
//
// libps 0x8001C238, 680 bytes
func spuTransfer(step int32, arg uint32, p unsafe.Pointer, size uint32) int32 {
	switch step {
	case 2:
		spuTransferAt = uint16(arg >> spuAddrShift)
		spuReg(spuTransAddr).Set(spuTransferAt)
		return 0
	case 1:
		spuDMARead = 0
		for i := 0; spuReg(spuTransAddr).Get() != spuTransferAt; i++ {
			if i >= 3840 {
				return -2
			}
		}
		spuReg(spuCnt).Set(spuReg(spuCnt).Get()&0xffcf | 0x20)
		return 0
	case 0:
		spuDMARead = 1
		for i := 0; spuReg(spuTransAddr).Get() != spuTransferAt; i++ {
			if i >= 3840 {
				return -2
			}
		}
		spuReg(spuCnt).Set(spuReg(spuCnt).Get() | 0x30)
		return 0
	case 3:
		want := uint16(0x20)
		if spuDMARead == 1 {
			want = 0x30
		}
		for i := 0; spuReg(spuCnt).Get()&0x30 != want; i++ {
			if i >= 3840 {
				return -2
			}
		}
		if spuDMARead == 1 {
			spuDelay.Set(spuDelay.Get()&0xf0ffffff | 0x22000000)
		} else {
			spuDelay.Set(spuDelay.Get()&0x2200ffff | 0x20000000)
		}
		spuDMAAddr = uint32(uintptr(p))
		spuDMABlocks = size >> 6
		if size&0x3f != 0 {
			spuDMABlocks++
		}
		dma4MADR.Set(spuDMAAddr)
		dma4BCR.Set(spuDMABlocks<<16 | 0x10)
		if spuDMARead == 1 {
			dma4CHCR.Set(0x01000200)
		} else {
			dma4CHCR.Set(0x01000201)
		}
		return 0
	}
	return 0
}

// spuWriteMem writes size bytes from p to the processor's memory at the
// transfer address, over the channel or through the port.
//
// libps 0x8001C4E0, 136 bytes
func spuWriteMem(p unsafe.Pointer, size uint32) uint32 {
	if spuIOMode != 0 {
		spuWriteIO(p, int32(size))
		return size
	}
	spuTransfer(2, uint32(spuTransferAt)<<spuAddrShift, nil, 0)
	spuTransfer(1, 0, nil, 0)
	spuTransfer(3, 0, p, size)
	return size
}

// spuWriteReg writes a register by its number, the value shifted to units
// when shift is set.
//
// libps 0x8001C5D0, 72 bytes
func spuWriteReg(reg uint32, v uint16, shift bool) {
	if shift {
		v >>= spuAddrShift
	}
	spuReg(uintptr(reg) * 2).Set(v)
}

// spuAddress converts a byte address to units, aligning it up first, and
// writes it to the register numbered reg: -1 returns it instead and -2
// returns the aligned byte address.
//
// libps 0x8001C618, 224 bytes
func spuAddress(reg int32, addr uint32) uint32 {
	if spuMemMode != 0 && addr%spuAlign != 0 {
		addr = (addr + spuAlign) &^ spuAlignMask
	}
	units := addr >> spuAddrShift
	switch reg {
	case -2:
		return addr
	case -1:
		return units & 0xffff
	}
	spuReg(uintptr(reg) * 2).Set(uint16(units))
	return addr
}

// SpuWrite writes size bytes at p to the processor's memory at the
// transfer address and returns how many it will write: at most 0x7f000.
//
// libps 0x8001DC58, 96 bytes
func SpuWrite(p unsafe.Pointer, size uint32) uint32 {
	if size > spuWriteLimit {
		size = spuWriteLimit
	}
	spuWriteMem(p, size)
	if spuTransferCB == nil {
		spuTransferred = 0
	}
	return size
}

// SpuSetTransferStartAddr sets where the next transfer goes.
//
// libps 0x8001DCB8, 60 bytes
func SpuSetTransferStartAddr(addr uint32) uint32 {
	spuTransferAt = uint16(spuAddress(-1, addr))
	return addr
}

// SpuSetTransferMode chooses the channel (0) or the port (1).
//
// libps 0x8001DCF4, 52 bytes
func SpuSetTransferMode(mode int32) {
	spuTransferMode = mode
	spuIOMode = b2i(mode == 1)
}

// SpuIsTransferCompleted reports whether the last transfer has finished,
// waiting for it when wait is 1.
//
// libps 0x8001DDB4, 168 bytes
func SpuIsTransferCompleted(wait int32) int32 {
	if spuTransferMode == 1 || spuTransferred == 1 {
		return 1
	}
	r := int32(testEvent(spuEvent))
	if wait != 1 {
		if r == 1 {
			spuTransferred = 1
		}
		return r
	}
	for r == 0 {
		r = int32(testEvent(spuEvent))
	}
	spuTransferred = 1
	return 1
}

// spuSetInTransfer marks a transfer as begun (1) or over.
//
// libps 0x8001DE5C, 44 bytes
func spuSetInTransfer(in int32) { spuTransferred = b2i(in != 1) }

// spuInTransfer reports whether a transfer is on.
//
// libps 0x8001DE88, 16 bytes
func spuInTransfer() bool { return spuTransferred == 0 }

// The processor's memory is given out from a table of blocks, each an
// address with its state in the top bits and a size: 0x80000000 marks a
// free block, 0x40000000 the free block at the end, and 0x2fffffff a
// block taken out of the table.
//
// libps 0x800472AC, 0x800472B0 and 0x800472B4
const (
	spuBlockFree    = 0x80000000
	spuBlockLast    = 0x40000000
	spuBlockRemoved = 0x2fffffff
	spuBlockAddr    = 0x0fffffff
)

type spuBlock struct {
	addr uint32
	size uint32
}

var (
	spuBlocks    *[32]spuBlock // the table the program gave
	spuBlockMax  int32
	spuBlockUsed int32
)

// SpuInitMalloc gives the driver its table of n blocks, with the whole of
// memory past the first 0x1010 bytes free.
//
// libps 0x8001C7B4, 84 bytes
func SpuInitMalloc(n int32, table *[32]spuBlock) int32 {
	if n <= 0 {
		return 0
	}
	table[0] = spuBlock{spuBlockLast | 0x1010, 1<<(16+spuAddrShift) - 0x1010}
	spuBlocks = table
	spuBlockUsed = 0
	spuBlockMax = n
	return n
}

// SpuMalloc takes size bytes of the processor's memory and returns where,
// or -1 when nothing large enough is free.
//
// libps 0x8001C808, 712 bytes
func SpuMalloc(size uint32) int32 {
	reserved := uint32(0)
	if spuRevReserved != 0 {
		reserved = (0x10000 - spuRevWorkAt) << spuAddrShift
	}
	if size&^spuAlignMask != size {
		size += spuAlignMask
	}
	size = size >> spuAddrShift << spuAddrShift
	found := int32(-1)
	if spuBlocks[0].addr&spuBlockLast == 0 {
		spuCompact()
		for i := int32(0); i < spuBlockMax; i++ {
			b := &spuBlocks[i]
			if b.addr&spuBlockLast != 0 {
				found = i
				break
			}
			if b.addr&spuBlockFree != 0 && b.size >= size {
				found = i
				break
			}
		}
	} else {
		found = 0
	}
	if found == -1 {
		return -1
	}
	b := &spuBlocks[found]
	if b.addr&spuBlockLast != 0 {
		if found >= spuBlockMax || b.size-reserved < size {
			return -1
		}
		next := &spuBlocks[found+1]
		next.addr = b.addr&spuBlockAddr + size | spuBlockLast
		next.size = b.size - size
		spuBlockUsed = found + 1
		b.size = size
		b.addr &= spuBlockAddr
		spuCompact()
		return int32(spuBlocks[found].addr)
	}
	if size < b.size && spuBlockUsed < spuBlockMax {
		rest := &spuBlocks[spuBlockUsed]
		rest.addr = (b.addr&spuBlockAddr + size) | spuBlockFree
		rest.size = b.size - size
		spuBlockUsed++
	}
	b.size = size
	b.addr &= spuBlockAddr
	spuCompact()
	return int32(spuBlocks[found].addr)
}

// spuCompact tidies the table: adjacent free blocks are joined, empty
// ones taken out, the rest sorted by address, and the free block at the
// end made from whatever follows the last taken one.
//
// libps 0x8001CAD0, 768 bytes
func spuCompact() {
	t := spuBlocks
	used := spuBlockUsed
	if used >= 0 {
		for i := int32(0); i <= used; i++ {
			if t[i].addr&spuBlockFree == 0 {
				continue
			}
			j := i + 1
			for t[j].addr == spuBlockRemoved {
				j++
			}
			if t[j].addr&spuBlockFree == 0 {
				continue
			}
			if t[j].addr&spuBlockAddr != t[i].addr&spuBlockAddr+t[i].size {
				continue
			}
			t[j].addr = spuBlockRemoved
			t[i].size += t[j].size
			i--
		}
	}
	if used >= 0 {
		for i := int32(0); i <= used; i++ {
			if t[i].size == 0 {
				t[i].addr = spuBlockRemoved
			}
		}
	}
	if used >= 0 {
		for i := int32(0); i <= used; i++ {
			if t[i].addr&spuBlockLast != 0 {
				break
			}
			for j := i + 1; j <= spuBlockUsed; j++ {
				if t[j].addr&spuBlockLast != 0 {
					break
				}
				if t[j].addr&spuBlockAddr < t[i].addr&spuBlockAddr {
					t[i], t[j] = t[j], t[i]
				}
			}
		}
	}
	if used >= 0 {
		for i := int32(0); i <= spuBlockUsed; i++ {
			if t[i].addr&spuBlockLast != 0 {
				break
			}
			if t[i].addr == spuBlockRemoved {
				t[i] = t[spuBlockUsed]
				spuBlockUsed = i
				break
			}
		}
	}
	for i := spuBlockUsed - 1; i >= 0; i-- {
		if t[i].addr&spuBlockFree == 0 {
			break
		}
		t[i].addr = t[i].addr&spuBlockAddr | spuBlockLast
		t[i].size += t[spuBlockUsed].size
		spuBlockUsed = i
	}
}

// SpuFree gives back the block at addr.
//
// libps 0x8001CDD0, 124 bytes
func SpuFree(addr uint32) {
	for i := int32(0); i < spuBlockMax; i++ {
		b := &spuBlocks[i]
		if b.addr&spuBlockLast != 0 {
			break
		}
		if b.addr == addr {
			b.addr = addr | spuBlockFree
			break
		}
	}
	spuCompact()
}

// spuSetVoiceBits sets or clears the voices in mask, a bit each, in the
// pair of registers at lo and hi, and returns the pair as 24 bits.
//
// libps 0x8001CE70, 180 bytes
func spuSetVoiceBits(on int32, mask uint32, lo, hi uintptr) uint32 {
	cur := uint32(spuReg(lo).Get()) | uint32(spuReg(hi).Get()&0xff)<<16
	switch on {
	case 1:
		cur |= mask & 0xffffff
		spuReg(lo).Set(spuReg(lo).Get() | uint16(mask))
		spuReg(hi).Set(spuReg(hi).Get() | uint16(mask>>16&0xff))
	case 0:
		cur &^= mask & 0xffffff
		spuReg(lo).Set(spuReg(lo).Get() &^ uint16(mask))
		spuReg(hi).Set(spuReg(hi).Get() &^ uint16(mask>>16&0xff))
	}
	return cur & 0xffffff
}

// SpuSetNoiseVoice turns the noise generator on or off for the voices in
// mask.
//
// libps 0x8001CE4C, 36 bytes
func SpuSetNoiseVoice(on int32, mask uint32) uint32 {
	return spuSetVoiceBits(on, mask, spuNoiseLo, spuNoiseHi)
}

// spuSetMute mutes (1) or unmutes (0) the processor.
//
// libps 0x8001C73C, 88 bytes
func spuSetMute(on int32) int32 {
	switch on {
	case 0:
		spuReg(spuCnt).Set(spuReg(spuCnt).Get() | 0x4000)
	case 1:
		spuReg(spuCnt).Set(spuReg(spuCnt).Get() & 0xbfff)
	}
	return on
}

// spuMuted reports whether the processor is muted.
//
// libps 0x8001C794, 32 bytes
func spuMuted() bool { return spuReg(spuCnt).Get()&0x4000 == 0 }

// A SpuCommonAttr is the processor's common settings: the main volume
// with its modes, the CD and external input volumes, and whether each
// input is mixed in and sent to the reverb. Its mask says which are set.
type SpuCommonAttr struct {
	Mask      uint32
	MVolL     int16
	MVolR     int16
	MVolModeL int16
	MVolModeR int16
	MVolxL    int16
	MVolxR    int16
	CDVolL    int16
	CDVolR    int16
	CDReverb  int32
	CDMix     int32
	ExtVolL   int16
	ExtVolR   int16
	ExtReverb int32
	ExtMix    int32
}

// spuVolumeWord is a volume register's value for a volume and its mode: a
// plain level, or a sweep with the mode in the top bits.
func spuVolumeWord(vol, mode int16, setMode bool) uint16 {
	sweep := uint16(0)
	if setMode && mode >= 1 && mode < 8 {
		sweep = 0x8000 + uint16(mode-1)<<12
	}
	if sweep == 0 {
		return uint16(vol) & 0x7fff
	}
	level := vol
	if vol >= 128 {
		level = 0x7f
	} else if vol < 0 {
		level = 0
	}
	return uint16(level)&0x7fff | sweep
}

// SpuSetCommonAttr sets the common settings attr's mask names, or all of
// them for a mask of 0.
//
// libps 0x8001DE98, 916 bytes
func SpuSetCommonAttr(attr *SpuCommonAttr) {
	m := attr.Mask
	all := m == 0
	if all || m&1 != 0 {
		spuReg(spuMainVolL).Set(spuVolumeWord(attr.MVolL, attr.MVolModeL, all || m&4 != 0))
	}
	if all || m&2 != 0 {
		spuReg(spuMainVolR).Set(spuVolumeWord(attr.MVolR, attr.MVolModeR, all || m&8 != 0))
	}
	if all || m&0x40 != 0 {
		spuReg(spuCDVolL).Set(uint16(attr.CDVolL))
	}
	if all || m&0x80 != 0 {
		spuReg(spuCDVolR).Set(uint16(attr.CDVolR))
	}
	cnt := spuReg(spuCnt)
	if all || m&0x100 != 0 {
		if attr.CDReverb != 0 {
			cnt.Set(cnt.Get() | 4)
		} else {
			cnt.Set(cnt.Get() & 0xfffb)
		}
	}
	if all || m&0x200 != 0 {
		if attr.CDMix != 0 {
			cnt.Set(cnt.Get() | 1)
		} else {
			cnt.Set(cnt.Get() & 0xfffe)
		}
	}
	if all || m&0x400 != 0 {
		spuReg(spuExtVolL).Set(uint16(attr.ExtVolL))
	}
	if all || m&0x800 != 0 {
		spuReg(spuExtVolR).Set(uint16(attr.ExtVolR))
	}
	if all || m&0x1000 != 0 {
		if attr.ExtReverb != 0 {
			cnt.Set(cnt.Get() | 8)
		} else {
			cnt.Set(cnt.Get() & 0xfff7)
		}
	}
	if all || m&0x2000 != 0 {
		if attr.ExtMix != 0 {
			cnt.Set(cnt.Get() | 2)
		} else {
			cnt.Set(cnt.Get() & 0xfffd)
		}
	}
}

// spuVolumeMode reads a volume register back as a level and a mode: a
// sweep's mode from its top bits with the level below them, and a level
// of 0x4000 or more, a negative one, with 0x8000 taken off.
func spuVolumeMode(v uint16) (int16, int16) {
	mode := int16(0)
	if v&0x8000 != 0 {
		switch v & 0xf000 {
		case 0x8000:
			mode = 1
		case 0x9000:
			mode = 2
		case 0xa000:
			mode = 3
		case 0xb000:
			mode = 4
		case 0xc000:
			mode = 5
		case 0xd000:
			mode = 6
		case 0xe000, 0xf000:
			mode = 7
		}
		v &= 0xfff
	}
	if v >= 0x4000 {
		v -= 0x8000
	}
	return int16(v), mode
}

// SpuGetCommonAttr reads the common settings back.
//
// libps 0x8001E22C, 772 bytes
func SpuGetCommonAttr(attr *SpuCommonAttr) {
	l, lm := spuVolumeMode(spuReg(spuMainVolL).Get())
	r, rm := spuVolumeMode(spuReg(spuMainVolR).Get())
	attr.MVolL, attr.MVolR = l, r
	attr.MVolModeL, attr.MVolModeR = lm, rm
	attr.MVolxL = int16(spuReg(spuCurVolL).Get())
	attr.MVolxR = int16(spuReg(spuCurVolR).Get())
	attr.CDVolL = int16(spuReg(spuCDVolL).Get())
	attr.CDVolR = int16(spuReg(spuCDVolR).Get())
	cnt := spuReg(spuCnt).Get()
	attr.CDReverb = int32(b2i(cnt&4 != 0))
	attr.CDMix = int32(b2i(cnt&1 != 0))
	attr.ExtVolL = int16(spuReg(spuExtVolL).Get())
	attr.ExtVolR = int16(spuReg(spuExtVolR).Get())
	attr.ExtReverb = int32(b2i(cnt&8 != 0))
	attr.ExtMix = int32(b2i(cnt&2 != 0))
}

// The reverb: each of the ten modes has a work area at the top of the
// processor's memory, whose start in units of 8 bytes is below, and a
// block of its 32 registers, with a word of flags first, in spuRevPresets.
//
// libps 0x800472B8 and 0x80047308
var spuRevSizes = [10]uint32{0xfffe, 0xfb28, 0xfc18, 0xf6f8, 0xf204, 0xea44, 0xe128, 0xcff8, 0xcff8, 0xf880}

// spuRevAreaUsed reports whether a block of the table lies at or past the
// work area beginning at units, which the reverb cannot then have.
//
// libps 0x8001D064, 120 bytes
func spuRevAreaUsed(units uint32) bool {
	start := units << spuAddrShift
	for i := int32(0); ; i++ {
		b := &spuBlocks[i]
		if b.addr&spuBlockFree != 0 {
			continue
		}
		if b.addr&spuBlockLast != 0 {
			return false
		}
		addr := b.addr & spuBlockAddr
		if addr >= start || start < addr+b.size {
			return true
		}
	}
}

// SpuSetReverb turns the reverb on or off and returns whether it is on:
// it stays off while something is allocated in its work area.
//
// libps 0x8001CF24, 320 bytes
func SpuSetReverb(on int32) int32 {
	cnt := spuReg(spuCnt)
	switch on {
	case 0:
		spuRevOn = 0
		cnt.Set(cnt.Get() & 0xff7f)
	case 1:
		if spuRevReserved != 1 && spuRevAreaUsed(spuRevWorkAt) {
			spuRevOn = 0
			cnt.Set(cnt.Get() & 0xff7f)
		} else {
			spuRevOn = 1
			cnt.Set(cnt.Get() | 0x80)
		}
	}
	return spuRevOn
}

// A SpuReverbAttr is the reverb's settings: the mode, with 0x100 added to
// have the work area cleared, the depth each side, and for the delay and
// echo modes the delay and feedback. Its mask says which are set.
type SpuReverbAttr struct {
	Mask     uint32
	Mode     int32
	DepthL   int16
	DepthR   int16
	Delay    int32
	Feedback int32
}

// spuLoadPreset copies mode's register block into the block being set.
func spuLoadPreset(mode int32) {
	copy(spuRevBlock[:], spuRevPresets[mode][:])
}

// div127 divides by 127 as the original did, rounding toward zero.
func div127(v int32) int32 {
	return (mulhi(v, -0x7efdfbf7)+v)>>6 - v>>31
}

// SpuSetReverbModeParam sets the reverb's settings attr's mask names, or
// all of them for a mask of 0. A mode's registers come from its preset,
// adjusted by the delay and feedback for the modes that take them; the
// reverb is turned off while the mode changes and the area cleared when
// asked.
//
// libps 0x8001D0DC, 1296 bytes
func SpuSetReverbModeParam(attr *SpuReverbAttr) int32 {
	m := attr.Mask
	all := m == 0
	modeSet, delaySet, feedbackSet, clearArea := false, false, false, false
	if all || m&1 != 0 {
		mode := attr.Mode
		if mode&0x100 != 0 {
			mode &^= 0x100
			clearArea = true
		}
		if mode >= 10 || spuRevAreaUsed(spuRevSizes[mode]) {
			return -1
		}
		modeSet = true
		spuRevMode = mode
		spuRevWorkAt = spuRevSizes[mode]
		spuLoadPreset(mode)
		switch mode {
		case 7:
			spuRevFeedback, spuRevDelay = 0x7f, 0x7f
		case 8:
			spuRevFeedback, spuRevDelay = 0, 0x7f
		default:
			spuRevFeedback, spuRevDelay = 0, 0
		}
	}
	if all || m&8 != 0 {
		if spuRevMode >= 7 && spuRevMode <= 8 {
			if !modeSet {
				spuLoadPreset(spuRevMode)
				spuRevBlockFlags(0x0c011c00)
			}
			delaySet = true
			spuRevDelay = attr.Delay
			a := div127(attr.Delay << 13)
			b := div127(attr.Delay << 12)
			// The block's registers from index 2: the delay sets
			// registers 10 to 12, 16, 26 and 27 from 0, 1, 13, 17, 28
			// and 29.
			blk := &spuRevBlock
			blk[12] = uint16(a - int32(blk[2]))
			blk[13] = uint16(b - int32(blk[3]))
			blk[18] = uint16(b + int32(blk[19]))
			blk[14] = uint16(b + int32(blk[15]))
			blk[28] = uint16(b + int32(blk[30]))
			blk[29] = uint16(b + int32(blk[31]))
		} else {
			spuRevDelay = 0
		}
	}
	if all || m&0x10 != 0 {
		if spuRevMode >= 7 && spuRevMode <= 8 {
			if !modeSet {
				if !delaySet {
					spuLoadPreset(spuRevMode)
					spuRevBlockFlags(0x80)
				} else {
					spuRevBlockFlags(spuRevBlockGetFlags() | 0x80)
				}
			}
			feedbackSet = true
			spuRevFeedback = attr.Feedback
			spuRevBlock[9] = uint16(div127((attr.Feedback * 129) << 8))
		} else {
			spuRevFeedback = 0
		}
	}
	wasOn := false
	if modeSet {
		cnt := spuReg(spuCnt)
		wasOn = cnt.Get()>>7&1 != 0
		if wasOn {
			cnt.Set(cnt.Get() & 0xff7f)
		}
		spuReg(spuRevVolL).Set(0)
		spuReg(spuRevVolR).Set(0)
		spuRevDepthL, spuRevDepthR = 0, 0
	} else {
		if all || m&2 != 0 {
			spuReg(spuRevVolL).Set(uint16(attr.DepthL))
			spuRevDepthL = attr.DepthL
		}
		if all || m&4 != 0 {
			spuReg(spuRevVolR).Set(uint16(attr.DepthR))
			spuRevDepthR = attr.DepthR
		}
	}
	if modeSet || delaySet || feedbackSet {
		spuWriteRevRegs()
	}
	if clearArea {
		SpuClearReverbWorkArea(spuRevMode)
	}
	if modeSet {
		spuWriteReg(0xd1, uint16(spuRevWorkAt), false)
		if wasOn {
			spuReg(spuCnt).Set(spuReg(spuCnt).Get() | 0x80)
		}
	}
	return 0
}

// The block's first word is its flags: which of the 32 registers to
// write, or all for 0.
func spuRevBlockGetFlags() uint32 { return uint32(spuRevBlock[0]) | uint32(spuRevBlock[1])<<16 }
func spuRevBlockFlags(f uint32)   { spuRevBlock[0], spuRevBlock[1] = uint16(f), uint16(f>>16) }

// spuWriteRevRegs writes the block's registers its flags name.
//
// libps 0x8001D5EC, 1232 bytes
func spuWriteRevRegs() {
	flags := spuRevBlockGetFlags()
	for i := uintptr(0); i < 32; i++ {
		if flags == 0 || flags&(1<<i) != 0 {
			spuReg(spuRevRegs + i*2).Set(spuRevBlock[2+i])
		}
	}
}

// SpuClearReverbWorkArea zeros mode's work area, a kilobyte at a time
// over the channel, waiting on the event for each.
//
// libps 0x8001DABC, 412 bytes
func SpuClearReverbWorkArea(mode int32) int32 {
	if mode >= 10 || spuRevAreaUsed(spuRevSizes[mode]) {
		return -1
	}
	var size, addr uint32
	if mode == 0 {
		size = 0x10 << spuAddrShift
		addr = 0xfff0 << spuAddrShift
	} else {
		size = (0x10000 - spuRevSizes[mode]) << spuAddrShift
		addr = spuRevSizes[mode] << spuAddrShift
	}
	ioMode := spuIOMode
	forced := false
	if ioMode == 1 {
		spuIOMode = 0
		forced = true
	}
	cb := spuTransferCB
	spuTransferCB = nil
	for {
		n := size
		more := true
		if size <= 0x400 {
			more = false
		} else {
			n = 0x400
		}
		spuTransfer(2, addr, nil, 0)
		spuTransfer(1, 0, nil, 0)
		spuTransfer(3, 0, unsafe.Pointer(&spuZeros[0]), n)
		waitEvent(spuEvent)
		size -= 0x400
		addr += 0x400
		if !more {
			break
		}
	}
	if forced {
		spuIOMode = ioMode
	}
	spuTransferCB = cb
	return 0
}

// mulhi is the high word of a signed 32 by 32 multiply, as the original's
// divisions by constants used it.
func mulhi(a, b int32) int32 { return int32((int64(a) * int64(b)) >> 32) }

// mulhiu is the same for unsigned words.
func mulhiu(a, b uint32) uint32 { return uint32((uint64(a) * uint64(b)) >> 32) }
