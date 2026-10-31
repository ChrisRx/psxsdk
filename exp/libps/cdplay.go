//go:build yaroze

package libps

import "unsafe"

// CD audio: CdPlay plays a list of the disc's audio tracks, one after
// another from the drive's table of contents, going on to the next as the
// drive reports each one's end; and CdReadExec loads a PSX-EXE off the
// disc.

const (
	cdlGetTN = 0x13
	cdlGetTD = 0x14
)

// CdPlay's state.
//
// libps 0x8004D91C on, 0x80062B4C and 0x80062CDC
var (
	cdPlayIndex  int32       = -1 // 0x8004D91C: which of the list plays, -1 for none
	cdPlayEnded  int32            // 0x8004D920: the drive stopped and the next track is due
	cdPlayLoop   int32            // 0x8004D924: the list starts over at its end
	cdPlayTracks [100]int32       // 0x8004D78C: the list, ended by 0
	cdPlayCount  int32            // 0x8004D788: tracks started
	cdTOC        [100]CdlLOC      // 0x80062B4C: where each track starts, from the lead-out at 0
	cdTOCTracks  int32            // 0x80062CDC: how many the drive reported
)

// CdPlay plays tracks, a list of track numbers ended by a 0, from the
// one at offset in it, with mode 1 once through and mode 2 round and
// round; mode 0 stops, and mode 3 asks what is playing, starting the
// next track when the drive has ended one. It returns the list index of
// the track playing, or -1.
//
// libps 0x8003C388, 496 bytes
//
//export CdPlay
func CdPlay(mode int32, tracks *int32, offset int32) int32 {
	cdStart()
	var result [8]uint8
	switch mode {
	case 3:
		if cdPlayIndex == -1 {
			return -1
		}
		if cdSync(1, &result[0]) == 0 {
			return cdPlayIndex
		}
		status := result[0]
		if status&0xc0 == 0 {
			cdPlayEnded = 1
		}
		if cdPlayEnded != 0 && status&0x10 == 0 {
			cdPlayNext()
		} else {
			CdControlF(cdlNop, nil, nil)
		}
		return cdPlayIndex
	case 0:
		cdPlayEnded = 0
		cdSetReadyCB(nil)
		CdControl(cdlPause, nil, nil)
		cdPlayIndex = -1
		return -1
	}
	cdTOCTracks = cdGetTOC(&cdTOC)
	if cdTOCTracks == 0 {
		kprintf("No TOC found: please use CD-DA disc...\n")
		cdPlayIndex = -1
		return -1
	}
	cdPlayLoop = b2i(mode != 1)
	n := int32(0)
	for ; n < 99; n++ {
		t := *(*int32)(unsafe.Add(unsafe.Pointer(tracks), n*4))
		if t == 0 {
			break
		}
		cdPlayTracks[n] = t
	}
	cdPlayTracks[n] = 0
	cdPlayIndex = offset
	switch {
	case offset < 0:
		cdPlayIndex = 0
	case n < offset:
		cdPlayIndex = n
	}
	var setmode uint8 = 3
	CdControl(cdlSetmode, &setmode, nil)
	cdSetReadyCB(cdPlayReady)
	cdPlayNext()
	return cdPlayIndex
}

// cdPlayReady is told what the drive reports while a track plays: its
// end starts the next track; a stop that is not an error does too.
//
// libps 0x8003C578, 200 bytes
func cdPlayReady(intr uint8, result *[8]uint8) {
	if intr == cdlDataEnd {
		kprintf("cbdataready: CdlDataEnd (track=%d,time=%d)\n", cdPlayIndex, VSync(-1))
		cdPlayIndex++
		cdPlayNext()
		return
	}
	if result[0]&1 != 0 {
		return
	}
	cdPlayEnded = 1
	name := "none"
	if intr < 7 {
		name = cdIntNames[intr]
	}
	kprintf("%s:%02x,%02x\n", name, result[0], result[1])
	if result[0]&0x11 == 0 {
		cdPlayNext()
	}
}

// cdPlayNext plays the track the index names, starting the list over or
// stopping at its end, and keeping a number past the table's within it.
//
// libps 0x8003C640, 296 bytes
func cdPlayNext() {
	cdPlayEnded = 0
	if cdPlayTracks[cdPlayIndex] == 0 {
		if cdPlayLoop != 0 {
			cdPlayIndex = 0
		} else {
			cdPlayIndex = -1
		}
	}
	if cdPlayIndex < 0 {
		return
	}
	cdPlayCount++
	t := cdPlayTracks[cdPlayIndex]
	if cdTOCTracks < t {
		kprintf("%d: track overflow\n", t)
		cdPlayTracks[cdPlayIndex] = cdTOCTracks
	}
	CdControl(cdlPlay, &cdTOC[cdPlayTracks[cdPlayIndex]].Minute, nil)
}

// cdGetTOC reads the disc's table of contents into toc: the lead-out at
// 0, then where each track starts, and returns how many tracks there
// are, or 0 when the drive would not say.
//
// libps 0x8003C78C, 548 bytes
func cdGetTOC(toc *[100]CdlLOC) int32 {
	var result [8]uint8
	param := uint8(1)
	debug := CdSetDebug(0)
	if CdControlB(cdlGetTN, nil, &result[0]) == 0 {
		return cdGetTOCFailed(debug)
	}
	first, last := btoi(result[1]), btoi(result[2])
	if debug >= 2 {
		kprintf("track=%d,%d\n", first, last)
	}
	param = 0
	if CdControlB(cdlGetTD, &param, &result[0]) == 0 {
		return cdGetTOCFailed(debug)
	}
	toc[0] = CdlLOC{Minute: result[1], Second: result[2]}
	n := int32(1)
	for t := first; t <= last; t++ {
		param = itob(t)
		if CdControlB(cdlGetTD, &param, &result[0]) == 0 {
			return cdGetTOCFailed(debug)
		}
		toc[n] = CdlLOC{Minute: result[1], Second: result[2]}
		n++
	}
	if debug >= 2 {
		for i := int32(0); i < n; i++ {
			kprintf("CdGetToc2: %02x:%02x:00\n", toc[i].Minute, toc[i].Second)
		}
	}
	CdSetDebug(debug)
	return n - 1
}

func cdGetTOCFailed(debug int32) int32 {
	if debug != 0 {
		kprintf("CdGetToc2: error\n")
	}
	CdSetDebug(debug)
	return 0
}

// An EXEC describes a loaded PSX-EXE, as the kernel's Exec takes it.
type EXEC struct {
	Pc0, Gp0       uint32
	T_addr, T_size uint32
	D_addr, D_size uint32
	B_addr, B_size uint32
	S_addr, S_size uint32
	Sp, Fp, Gp     uint32
	Ret, Base      uint32
}

// cdExec is what CdReadExec returns.
//
// libps 0x80062B10
var cdExec EXEC

// CdReadExec reads the PSX-EXE named file off the disc: its header's
// first sector, then the rest to the address the header names, and
// returns the header's description of it, with no stack given, or nil
// when either read failed.
//
// libps 0x8003C2B4, 212 bytes
//
//export CdReadExec
func CdReadExec(file *byte) *EXEC {
	var header [2048 / 4]uint32
	if CdReadFile(file, &header[0], 0x800) == 0 || CdReadSync(0, nil) != 0 {
		return nil
	}
	addr := header[6]
	if CdReadFile(nil, (*uint32)(unsafe.Pointer(uintptr(addr))), 0) == 0 || CdReadSync(0, nil) != 0 {
		return nil
	}
	cdExec = *(*EXEC)(unsafe.Pointer(&header[4]))
	cdExec.S_addr, cdExec.S_size = 0, 0
	return &cdExec
}

// The disc's audio and the external input, as the sound library mixes
// them: whether each is heard and goes through the reverb, and its
// volume.

// SsSetSerialAttr turns attr, SS_MIX (0, mixed in) or SS_REV (1, through
// the reverb), on or off for source: SS_CD (0) or SS_SERIAL (1).
//
// libps 0x80010F30, 148 bytes
//
//export SsSetSerialAttr
func SsSetSerialAttr(source uint8, attr uint8, on uint8) {
	sndStart()
	var a SpuCommonAttr
	switch {
	case source == 0 && attr == 0:
		a.Mask = 0x200
		a.CDMix = int32(on)
	case source == 0 && attr == 1:
		a.Mask = 0x100
		a.CDReverb = int32(on)
	case source == 1 && attr == 0:
		a.Mask = 0x2000
		a.ExtMix = int32(on)
	case source == 1 && attr == 1:
		a.Mask = 0x1000
		a.ExtReverb = int32(on)
	}
	SpuSetCommonAttr(&a)
}

// SsSetSerialVol sets source's volume, each side from 0 to 127.
//
// libps 0x80010FC4, 252 bytes
//
//export SsSetSerialVol
func SsSetSerialVol(source uint8, l int16, r int16) {
	sndStart()
	var a SpuCommonAttr
	if l >= 128 {
		l = 0x7f
	}
	if r >= 128 {
		r = 0x7f
	}
	switch source {
	case 0:
		a.Mask = 0xc0
		a.CDVolL, a.CDVolR = l*258, r*258
	case 1:
		a.Mask = 0xc00
		a.ExtVolL, a.ExtVolR = l*258, r*258
	default:
		return
	}
	SpuSetCommonAttr(&a)
}

// SsGetSerialAttr reads whether attr is on for source, as
// SsSetSerialAttr has them.
//
// libps 0x80010D34, 152 bytes
//
//export SsGetSerialAttr
func SsGetSerialAttr(source uint8, attr uint8) uint8 {
	sndStart()
	var a SpuCommonAttr
	SpuGetCommonAttr(&a)
	switch {
	case source == 0 && attr == 0:
		return uint8(a.CDMix)
	case source == 0 && attr == 1:
		return uint8(a.CDReverb)
	case source == 1 && attr == 0:
		return uint8(a.ExtMix)
	case source == 1 && attr == 1:
		return uint8(a.ExtReverb)
	}
	return 0
}

// A SndVolume is a volume each side.
type SndVolume struct {
	Left  uint16
	Right uint16
}

// SsGetSerialVol reads source's volume, each side from 0 to 127.
//
// libps 0x80010DCC, 212 bytes
//
//export SsGetSerialVol
func SsGetSerialVol(source uint8, vol *SndVolume) {
	sndStart()
	var a SpuCommonAttr
	SpuGetCommonAttr(&a)
	switch source {
	case 0:
		vol.Left = uint16(int32(a.CDVolL) / 258)
		vol.Right = uint16(int32(a.CDVolR) / 258)
	case 1:
		vol.Left = uint16(int32(a.ExtVolL) / 258)
		vol.Right = uint16(int32(a.ExtVolR) / 258)
	}
}
