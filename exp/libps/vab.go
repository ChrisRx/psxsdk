//go:build yaroze

package libps

import "unsafe"

// Sound banks: a VAB is a header (the VH file) of up to 128 programs of
// 16 tones each, and a body (the VB file) of the samples the tones play,
// which goes into the sound processor's memory. The library opens up to
// 16 at a time, by id.

// The header: 32 bytes, then the program table, then 16 tone entries of
// 32 bytes for each program that has tones, then the sizes of the
// samples as halfwords.
const (
	vabVersion  = 4  // int32: 5 and up has sample sizes in units of 8
	vabPrograms = 18 // uint16: how many programs have tones
	vabVAGs     = 22 // uint8: how many samples
	vabMVol     = 24 // uint8: the bank's volume
	vabHeader   = 32

	// A program entry: its tone count, volume and pan; the index of the
	// program among those with tones at 8, and from 12 the samples'
	// addresses as halfwords, two to an entry.
	progTones = 0
	progVol   = 1
	progPrio  = 2
	progMode  = 3
	progPan   = 4
	progIndex = 8
	progAddrs = 12
	progSize  = 16

	// A tone entry.
	tonePrio   = 0
	toneMode   = 1
	toneVol    = 2
	tonePan    = 3
	toneCenter = 4
	toneShift  = 5
	toneMin    = 6
	toneMax    = 7
	toneVibW   = 8
	toneVibT   = 9
	tonePorW   = 10
	tonePorT   = 11
	tonePBMin  = 12
	tonePBMax  = 13
	toneADSR1  = 16
	toneADSR2  = 18
	toneProg   = 20
	toneVAG    = 22
	toneSize   = 32
)

// The banks' state: free, ready, or with only the header opened.
const (
	vabFree  = 0
	vabReady = 1
	vabHead  = 2
)

// The open banks, by id.
//
// libps 0x80063FC4 (state), 0x80066230 (count), 0x80063DAC (header),
// 0x80063D68 (programs), 0x80063DF0 (tones), 0x80066278 (past the sample
// sizes), 0x80066234 (address), 0x800661F0 (body size), 0x80063EC8
// (programs a header can hold)
var (
	vabState    [16]uint8
	vabCount    int16
	vabVH       [16]unsafe.Pointer
	vabProgs    [16]unsafe.Pointer
	vabTones    [16]unsafe.Pointer
	vabSizesEnd [16]unsafe.Pointer
	vabAddr     [16]uint32
	vabBody     [16]uint32
	vabMaxProg  int16 = 0x80
)

// Reads and writes of the bank's bytes in memory.
func peek8(p unsafe.Pointer, off uintptr) uint8      { return *(*uint8)(unsafe.Add(p, off)) }
func peek16(p unsafe.Pointer, off uintptr) uint16    { return *(*uint16)(unsafe.Add(p, off)) }
func peek32(p unsafe.Pointer, off uintptr) uint32    { return *(*uint32)(unsafe.Add(p, off)) }
func poke8(p unsafe.Pointer, off uintptr, v uint8)   { *(*uint8)(unsafe.Add(p, off)) = v }
func poke16(p unsafe.Pointer, off uintptr, v uint16) { *(*uint16)(unsafe.Add(p, off)) = v }
func poke32(p unsafe.Pointer, off uintptr, v uint32) { *(*uint32)(unsafe.Add(p, off)) = v }

// vabProg is program prog's entry in the header the sequencer has set.
func vabProg(prog int32) unsafe.Pointer { return unsafe.Add(sndProgs, uintptr(prog)*progSize) }

// vabTone is tone t of the program with index idx in the header the
// sequencer has set.
func vabTone(idx, t int32) unsafe.Pointer {
	return unsafe.Add(sndTones, uintptr(idx*16+t)*toneSize)
}

// vabOpenHead opens the header vh as bank id, or as the first free bank
// for an id of -1: the programs with tones are numbered, the samples'
// sizes added up, and memory for them taken from the processor, or used
// at addr when mode is not 0. It returns the id, or -1 when a transfer
// is on, the header is not a bank's, has more programs than the library
// holds, or the samples do not fit.
//
// libps 0x8001576C, 1032 bytes
func vabOpenHead(vh unsafe.Pointer, id int16, mode int32, addr uint32) int16 {
	if spuInTransfer() {
		return -1
	}
	spuSetInTransfer(1)
	slot := int32(16)
	if id < 16 {
		if id == -1 {
			for i := int32(0); i < 16; i++ {
				if vabState[i] == vabFree {
					vabState[i] = vabReady
					vabCount++
					slot = i
					break
				}
			}
		} else if vabState[id] == vabFree {
			vabState[id] = vabReady
			vabCount++
			slot = int32(id)
		}
	}
	if slot >= 16 {
		spuSetInTransfer(0)
		return -1
	}
	vabVH[slot] = vh
	progs := unsafe.Add(vh, vabHeader)
	if peek32(vh, 0)>>8 != 0x564142 {
		vabState[slot] = vabFree
		return vabOpenFailed(slot)
	}
	vabMaxProg = 0x40
	if peek8(vh, 0) == 'p' && int32(peek32(vh, vabVersion)) >= 5 {
		vabMaxProg = 0x80
	}
	ps := int32(peek16(vh, vabPrograms))
	if int32(vabMaxProg) < ps {
		vabState[slot] = vabFree
		return vabOpenFailed(slot)
	}
	vabProgs[slot] = progs
	tones := unsafe.Add(progs, uintptr(vabMaxProg)*progSize)
	// Every program gets the count so far as its index, one with tones
	// moving the count on: the original's store was in a branch's delay
	// slot, so a program without tones got the index of the next one
	// with them.
	n := int32(0)
	for i := int32(0); i < int32(vabMaxProg); i++ {
		p := unsafe.Add(progs, uintptr(i)*progSize)
		poke32(p, progIndex, uint32(n))
		if peek8(p, progTones) != 0 {
			n++
		}
	}
	vabTones[slot] = tones
	vs := int32(peek8(vh, vabVAGs))
	sizes := unsafe.Add(tones, uintptr(ps)*512)
	var size [256]uint32
	total := uint32(0)
	shift := uint(2)
	if int32(peek32(vh, vabVersion)) >= 5 {
		shift = 3
	}
	for i := int32(0); i <= vs; i++ {
		size[i] = uint32(peek16(sizes, uintptr(i)*2)) << shift
		total += size[i]
	}
	vabSizesEnd[slot] = unsafe.Add(sizes, uintptr(vs+1)*2)
	if mode == 0 {
		a := SpuMalloc(total)
		if a == -1 {
			vabState[slot] = vabFree
			return vabOpenFailed(slot)
		}
		addr = uint32(a)
	}
	if addr+total > spuRAMSize {
		vabState[slot] = vabFree
		return vabOpenFailed(slot)
	}
	vabAddr[slot] = addr
	sum := uint32(0)
	for i := int32(0); i <= vs; i++ {
		sum += size[i]
		p := unsafe.Add(progs, uintptr(i/2)*progSize)
		if i&1 == 0 {
			poke16(p, progAddrs, uint16((addr+sum)>>3))
		} else {
			poke16(p, progAddrs+2, uint16((addr+sum)>>3))
		}
	}
	vabBody[slot] = sum
	vabState[slot] = vabHead
	return int16(slot)
}

// vabOpenFailed gives up an open that went wrong after its slot was
// taken.
func vabOpenFailed(slot int32) int16 {
	spuSetInTransfer(0)
	vabCount--
	return -1
}

// vabTransBody sends bank id's body vb to the processor's memory, at the
// address its header was given. It returns the id, or -1 for a bank
// whose header is not open.
//
// libps 0x80015B74, 192 bytes
func vabTransBody(vb unsafe.Pointer, id int16) int16 {
	if uint16(id) < 17 && vabState[id] == vabHead {
		SpuSetTransferMode(0)
		SpuSetTransferStartAddr(vabAddr[id])
		SpuWrite(vb, vabBody[id])
		vabState[id] = vabReady
		return id
	}
	spuSetInTransfer(0)
	return -1
}

// SsVabTransfer opens the bank whose header is at vh and body at vb as
// bank id, or as the first free bank for -1, and sends the body to the
// processor, waiting for it when wait is set. It returns the id, -1
// when the header could not be opened, and -2 when the body could not
// be sent.
//
// libps 0x80015D94, 128 bytes
//
//export SsVabTransfer
func SsVabTransfer(vh *uint8, vb *uint8, id int16, wait int16) int16 {
	sndStart()
	id = vabOpenHead(unsafe.Pointer(vh), id, 0, 0)
	if id < 0 {
		return -1
	}
	id = vabTransBody(unsafe.Pointer(vb), id)
	if id < 0 {
		return -2
	}
	if wait != 0 {
		SpuIsTransferCompleted(1)
	}
	return id
}

// SsVabClose closes bank id, giving its memory back.
//
// libps 0x800155F4, 132 bytes
//
//export SsVabClose
func SsVabClose(id int16) {
	sndStart()
	if uint16(id) < 16 && vabState[id] == vabReady {
		SpuFree(vabAddr[id])
		vabState[id] = vabFree
		vabCount--
	}
}

// The header, programs and tones the sequencer is working in, and the
// program.
//
// libps 0x80063EFC, 0x80063EF4, 0x80063F00
var (
	sndVH    unsafe.Pointer
	sndProgs unsafe.Pointer
	sndTones unsafe.Pointer
)

// sndSetVab points the sequencer at program prog of bank vab, returning
// -1 for a bank that is not ready or a program past the header.
//
// libps 0x80015F6C, 196 bytes
func sndSetVab(vab int16, prog int16) int32 {
	if uint16(vab) >= 16 || vabState[vab] != vabReady || prog >= vabMaxProg {
		return -1
	}
	sndCur.vab = uint8(vab)
	sndCur.prog = uint8(prog)
	sndTones = vabTones[vab]
	sndVH = vabVH[vab]
	sndProgs = vabProgs[vab]
	sndCur.progIdx = peek8(vabProg(int32(prog)), progIndex)
	return 0
}

// A sndProgAttr is a program's attributes as the library reads them.
type sndProgAttr struct {
	tones uint8
	vol   uint8
	prio  uint8
	mode  uint8
	pan   uint8
	attr  uint16
}

// sndGetProgAttr reads program prog of bank vab.
//
// libps 0x80015E64, 264 bytes
func sndGetProgAttr(vab int16, prog int16, a *sndProgAttr) int32 {
	if vabState[vab&0xf] != vabReady {
		return -1
	}
	sndSetVab(vab, prog)
	p := vabProg(int32(prog))
	a.tones = peek8(p, progTones)
	a.vol = peek8(p, progVol)
	a.prio = peek8(p, progPrio)
	a.mode = peek8(p, progMode)
	a.pan = peek8(p, progPan)
	a.attr = peek16(p, 6)
	return 0
}

// A sndToneAttr is a tone's attributes as the library reads and writes
// them: the first 14 bytes of the entry and its four halfwords.
type sndToneAttr struct {
	b     [14]uint8
	adsr1 uint16
	adsr2 uint16
	prog  uint16
	vag   uint16
}

// sndGetToneAttr reads tone t of program prog of bank vab.
//
// libps 0x80016030, 572 bytes
func sndGetToneAttr(vab int16, prog int16, t int16, a *sndToneAttr) int32 {
	if vabState[vab&0xf] != vabReady {
		return -1
	}
	sndSetVab(vab, prog)
	p := vabTone(int32(sndCur.progIdx), int32(t))
	for i := uintptr(0); i < 14; i++ {
		a.b[i] = peek8(p, i)
	}
	a.adsr1 = peek16(p, toneADSR1)
	a.adsr2 = peek16(p, toneADSR2)
	a.prog = peek16(p, toneProg)
	a.vag = peek16(p, toneVAG)
	return 0
}

// sndSetToneAttr writes tone t of program prog of bank vab.
//
// libps 0x80016478, 456 bytes
func sndSetToneAttr(vab int16, prog int16, t int16, a *sndToneAttr) int32 {
	if vabState[vab&0xf] != vabReady {
		return -1
	}
	sndSetVab(vab, prog)
	p := vabTone(int32(sndCur.progIdx), int32(t))
	for i := uintptr(0); i < 14; i++ {
		poke8(p, i, a.b[i])
	}
	poke16(p, toneADSR1, a.adsr1)
	poke16(p, toneADSR2, a.adsr2)
	poke16(p, toneProg, a.prog)
	poke16(p, toneVAG, a.vag)
	return 0
}

// sndSetProgVol sets program prog's volume and returns it, or -1.
//
// libps 0x80019C74, 116 bytes
func sndSetProgVol(vab int16, prog int16, vol uint8) int32 {
	if sndSetVab(vab, prog) != 0 {
		return -1
	}
	poke8(vabProg(int32(prog)), progVol, vol)
	return int32(peek8(vabProg(int32(prog)), progVol))
}

// The reverb as the sequencer sets it: the attributes it sends to the
// processor, with the mode the program chose.
//
// libps 0x80062D1C
var sndReverb = SpuReverbAttr{DepthL: 0x3fff, DepthR: 0x3fff}

// SsUtSetReverbType chooses the reverb mode, one of the ten, and returns
// it, or -1 for one that is not. A negative mode clears the work area.
//
// libps 0x8001626C, 164 bytes
//
//export SsUtSetReverbType
func SsUtSetReverbType(mode int16) int16 {
	sndStart()
	clear := false
	if mode < 0 {
		clear = true
		mode = -mode
	}
	if uint16(mode) >= 10 {
		return -1
	}
	sndReverb.Mask = 1
	sndReverb.Mode = int32(mode)
	if clear {
		sndReverb.Mode = int32(mode | 0x100)
	}
	if mode == 0 {
		SpuSetReverb(0)
	}
	SpuSetReverbModeParam(&sndReverb)
	return mode
}

// SsUtGetReverbType is the mode chosen.
//
// libps 0x80016310, 16 bytes
//
//export SsUtGetReverbType
func SsUtGetReverbType() int16 { return int16(sndReverb.Mode) }

// SsUtReverbOn turns the reverb on.
//
// libps 0x80016320, 32 bytes
//
//export SsUtReverbOn
func SsUtReverbOn() { sndStart(); SpuSetReverb(1) }

// SsUtReverbOff turns the reverb off.
//
// libps 0x80016340, 32 bytes
//
//export SsUtReverbOff
func SsUtReverbOff() { sndStart(); SpuSetReverb(0) }

// SsUtSetReverbFeedback sets the feedback, for the echo modes.
//
// libps 0x80016360, 64 bytes
//
//export SsUtSetReverbFeedback
func SsUtSetReverbFeedback(v int16) {
	sndStart()
	sndReverb.Mask = 0x10
	sndReverb.Feedback = int32(v)
	SpuSetReverbModeParam(&sndReverb)
}

// SsUtSetReverbDepth sets how much reverb there is, each side, from 0 to
// 127.
//
// libps 0x800163A0, 152 bytes
//
//export SsUtSetReverbDepth
func SsUtSetReverbDepth(l int16, r int16) {
	sndStart()
	sndReverb.Mask = 6
	sndReverb.DepthL = int16((int32(l)<<15 - int32(l)) / 127)
	sndReverb.DepthR = int16((int32(r)<<15 - int32(r)) / 127)
	SpuSetReverbModeParam(&sndReverb)
}

// SsUtSetReverbDelay sets the delay, for the echo modes.
//
// libps 0x80016438, 64 bytes
//
//export SsUtSetReverbDelay
func SsUtSetReverbDelay(v int16) {
	sndStart()
	sndReverb.Mask = 8
	sndReverb.Delay = int32(v)
	SpuSetReverbModeParam(&sndReverb)
}
