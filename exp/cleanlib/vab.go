//go:build yaroze

package cleanlib

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// Sound banks, VABs: the header (VH) stays in the program's memory, where
// the library reads the programs and tones from when it plays them, and
// the samples (VB) go to the SPU's sound memory.

const (
	vabMagic    = 0x56414270 // "pBAV" as read little-endian
	vabMax      = 16
	vabHeader   = 32
	vabProgSize = 16
	vabToneSize = 32
	vabPrograms = 128
	vabTones    = 16 // tones a program has room for
)

// vabProg is a program's attributes, as the VH holds them. slot and addr
// are what SsVabTransfer writes into the entry: which block of tone rows
// is the program's, and two of the sample start addresses, in 8-byte
// units, that the entries hold in sequence (see half).
type vabProg struct {
	tones, vol, prior, mode, pan, _ uint8
	attr                            int16
	slot                            uint32
	addr                            [2]uint16
}

// vabTone is a tone's attributes, as the VH holds them.
type vabTone struct {
	prior, mode, vol, pan, center, shift, min, max uint8
	vibW, vibT, porW, porT, pbMin, pbMax, _, _     uint8
	adsr1, adsr2                                   uint16
	prog, vag                                      int16
	_                                              [4]int16
}

// A bank keeps no copy of its programs and tones: every key on reads
// them from the header, where SsVabTransfer left them, slot words and
// start addresses included. A program whose header has changed since
// plays with what is there now.
type vab struct {
	open  bool
	mvol  uint8
	hdr   unsafe.Pointer // the header
	progs *[vabPrograms]vabProg
	tones unsafe.Pointer // the first program's tones

	// base and size are the bank's sound memory.
	base, size uint32

	// halves is how many of the header's VAG address halves a key on reads:
	// two for each of the programs the header counted at SsVabTransfer.
	halves int
}

var vabs [vabMax]vab

// tone returns tone t of program p: row slot*16 + t, the slot being the
// low byte of the word in the program's entry (terra's effects, keyed on
// from a header overwritten by another bank's body, pick their rows by
// that byte alone).
func (b *vab) tone(p, t int) *vabTone {
	i := uintptr(uint8(b.progs[p].slot))*vabTones + uintptr(t)
	return (*vabTone)(unsafe.Add(b.tones, i*vabToneSize))
}

// programs returns the header's program count, as it is now.
func (b *vab) programs() int {
	return int(*(*uint16)(unsafe.Add(b.hdr, 18)))
}

// vagNoise is the VAG index that selects the SPU's noise generator: the low
// byte of a tone's index for a sequence's note on, the whole index for
// SsUtKeyOn.
const vagNoise = 0xff

// vagHalves is the table of VAG address halves a key on reads, by the rules
// of half.
const vagHalves = 256

// half returns the VAG address half h, in 8-byte units of sound memory: the
// halves at bytes 12 to 15 of the program entries, taken in sequence, hold
// the VAGs' addresses from the first entry's first half. Only the first
// b.halves are read from the header, and the rest of the table holds FFFFh
// whatever the header does (spec 05, "The VAG index").
func (b *vab) half(h int) uint16 {
	if h >= b.halves && h < vagHalves {
		return 0xffff
	}
	if h >= vagHalves {
		return 0
	}
	return *(*uint16)(unsafe.Add(unsafe.Pointer(b.progs), (h>>1)*vabProgSize+12+(h&1)*2))
}

// seqAddr is the start address of a sequence's note on tone: the low byte of
// the VAG index, byte 0 reading half 1, the second VAG's address, and byte b
// half b - 1.
func (b *vab) seqAddr(vag int16) uint16 {
	i := int(uint8(vag))
	if i == 0 {
		return b.half(1)
	}
	return b.half(i - 1)
}

// utAddr is the start address of SsUtKeyOn's tone: the VAG index v whole and
// signed reads half v - 1. An index below zero reads the memory before the
// header (terra's overwritten header gives one, and finds its start address
// there); one past the table gives 0.
func (b *vab) utAddr(v int) uint16 {
	return b.half(v - 1)
}

// sndMem is the sound memory banks are given: a list of free extents in
// address order, taken first fit.
var sndMem struct {
	free [vabMax + 2]extent
	n    int
}

type extent struct{ addr, size uint32 }

func memInit() {
	sndMem.free[0] = extent{spuUserBase, spuRAMSize - 0x10 - spuUserBase}
	sndMem.n = 1
}

// memAlloc takes size bytes of sound memory, at the lowest address that
// has room, and reports whether it could.
func memAlloc(size uint32) (uint32, bool) {
	for i := 0; i < sndMem.n; i++ {
		e := &sndMem.free[i]
		if e.size < size {
			continue
		}
		addr := e.addr
		e.addr += size
		e.size -= size
		if e.size == 0 {
			copy(sndMem.free[i:], sndMem.free[i+1:sndMem.n])
			sndMem.n--
		}
		return addr, true
	}
	return 0, false
}

// memFree gives size bytes at addr back, merging them with their
// neighbors.
func memFree(addr, size uint32) {
	i := 0
	for i < sndMem.n && sndMem.free[i].addr < addr {
		i++
	}
	copy(sndMem.free[i+1:sndMem.n+1], sndMem.free[i:sndMem.n])
	sndMem.free[i] = extent{addr, size}
	sndMem.n++
	if i+1 < sndMem.n && sndMem.free[i].addr+sndMem.free[i].size == sndMem.free[i+1].addr {
		sndMem.free[i].size += sndMem.free[i+1].size
		copy(sndMem.free[i+1:], sndMem.free[i+2:sndMem.n])
		sndMem.n--
	}
	if i > 0 && sndMem.free[i-1].addr+sndMem.free[i-1].size == sndMem.free[i].addr {
		sndMem.free[i-1].size += sndMem.free[i].size
		copy(sndMem.free[i:], sndMem.free[i+1:sndMem.n])
		sndMem.n--
	}
}

// SsVabTransfer opens the bank whose header is at vh and sends its
// samples, at vb, to sound memory. It returns the bank's id, or -1. The
// result is a whole word here and in the other sound calls, though the C
// headers say short: a Go int16 comes back with the high half of v0 clear,
// where Sony's -1 is all 32 bits (spec 23). The header has to be on a word
// boundary, as Sony's does: at an offset of 2 its call never returns.
//
//export SsVabTransfer
func SsVabTransfer(vh, vb *uint8, vabid, mode int16) int32 {
	soundStart()
	h := unsafe.Pointer(vh)
	if *(*uint32)(h) != vabMagic {
		return -1
	}
	id := -1
	if vabid < 0 {
		for i := range vabs {
			if !vabs[i].open {
				id = i
				break
			}
		}
	} else if int(vabid) < vabMax && !vabs[vabid].open {
		id = int(vabid)
	}
	if id < 0 {
		return -1
	}
	b := &vabs[id]
	ps := int(*(*uint16)(unsafe.Add(h, 18)))
	b.mvol = *(*uint8)(unsafe.Add(h, 24))
	b.hdr = h
	b.halves = 2 * ps
	b.progs = (*[vabPrograms]vabProg)(unsafe.Add(h, vabHeader))
	b.tones = unsafe.Add(h, vabHeader+vabPrograms*vabProgSize)
	sizes := (*[256]uint16)(unsafe.Add(b.tones, ps*vabTones*vabToneSize))
	var total uint32
	for i := 1; i < 256; i++ {
		total += uint32(sizes[i]) << 3
	}
	base, ok := memAlloc(total)
	if !ok {
		return -1
	}
	b.base, b.size = base, total
	// The header's program entries get, in bytes 8 to 11, each program's
	// slot: its index among the programs with tones, one block of 16
	// rows each in order, and so the count of them for every program
	// after the last. Bytes 12 to 15, taken as halves in sequence, get
	// the VAGs' addresses (divided by 8) and then the body's end. Key
	// ons read both from here.
	k := 0
	for p := range b.progs {
		b.progs[p].slot = uint32(k)
		if b.progs[p].tones > 0 {
			k++
		}
	}
	vs := int(*(*uint16)(unsafe.Add(h, 22)))
	at := base
	for i := 1; i < 256; i++ {
		if i <= vs+1 {
			b.progs[(i-1)/2].addr[(i-1)%2] = uint16(at >> 3)
		}
		at += uint32(sizes[i]) << 3
	}
	b.open = true
	state := EnterCriticalSection()
	spuWrite(base, unsafe.Pointer(vb), total)
	if state != 0 {
		ExitCriticalSection()
	}
	spinCycles(ssVabTransferPad)
	return int32(id)
}

// ssVabTransferPad is what SsVabTransfer's mean of 25,200 cycles has over the
// work here (spec 21).
const ssVabTransferPad = 13800

// The SPU's DMA channel, 4.
var (
	dmaSPUAddr    = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010c0)))
	dmaSPUBlock   = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010c4)))
	dmaSPUControl = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010c8)))
)

const (
	dmaSPUEnable = 1 << 19
	dmaToSPU     = 0x01000201 // start, in blocks, from memory
)

// spuWrite copies n bytes from p to sound memory at addr by DMA, in
// blocks of 64 bytes: the last block is whole, so up to 62 bytes past the
// end go too.
func spuWrite(addr uint32, p unsafe.Pointer, n uint32) {
	spuSet(spuXferAt, uint16(addr>>3))
	c := spuGet(spuCtrl) &^ 0x30
	spuSet(spuCtrl, c|0x20)
	dmaPriority.Set(dmaPriority.Get() | dmaSPUEnable)
	dmaSPUAddr.Set(physical(p))
	dmaSPUBlock.Set((n+63)/64<<16 | 16)
	dmaSPUControl.Set(dmaToSPU)
	for dmaSPUControl.Get()&(1<<24) != 0 {
	}
	spuSet(spuCtrl, c)
}

// SsVabClose closes bank id and frees its sound memory.
//
//export SsVabClose
func SsVabClose(id int16) {
	soundStart()
	if id < 0 || id >= vabMax || !vabs[id].open {
		return
	}
	b := &vabs[id]
	b.open = false
	memFree(b.base, b.size)
}
