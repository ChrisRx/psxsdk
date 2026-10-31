//go:build yaroze

package libps

import "go.chrisrx.dev/psx/internal/volatile"

// The sequencer's setup and its tick. The resident library's startup
// initialized it (SsInit, a table of 16 sequences, tick mode 5 and
// SsStart); this library does the same the first time a program calls
// any of it.

var (
	sndStarted  bool
	sndTickRate int32  = 60 // libps 0x800658CC: ticks a second
	sndTickMode int32  = 60 // libps 0x80046C40: the mode, or a rate for a custom one
	sndCustom   bool        // libps 0x80046C44: the mode is a rate
	sndSavedCB  func()      // libps 0x80046C4C: the vertical blank's callback before the tick's
	sndOnVSync  bool        // libps 0x80046C50: the tick runs from VSyncCallback
	sndHalfRate bool        // libps 0x80046C51: the counter runs at twice the rate and the tick takes every other interrupt
	sndCounter  uint8       // libps 0x80046C52: the root counter's interrupt, or 0 for the vertical blank
	sndToggle   bool        // libps 0x80046C54
	sndInTick   bool        // libps 0x80063EC4
)

// sndStart initializes the sequencer the first time, as the startup
// did, with the drive initialized first as the startup had it: the
// processor's reset turns the drive's audio off, which CdInit turns on.
func sndStart() {
	if sndStarted {
		return
	}
	sndStarted = true
	cdStart()
	spuEventsOpened = false
	SsInit()
	sndSetTableSize()
	SsSetTickMode(5)
	SsStart()
}

// SsInit resets the processor and the sequencer.
//
// libps 0x80013B04, 48 bytes
//
//export SsInit
func SsInit() {
	ResetCallback()
	spuInit()
	sndInit()
}

// The registers every voice and the common block are set to.
//
// libps 0x80046C10 and 0x80046C20
var (
	sndVoiceInitRegs  = [8]uint16{0, 0, 0x1000, 0x3000, 0x00bf, 0, 0, 0}
	sndCommonInitRegs = [16]uint16{0x3fff, 0x3fff}
)

// sndInit sets the voices' and common registers, the voice table and
// the sequencer's state.
//
// libps 0x80013A1C, 232 bytes
func sndInit() {
	for v := uintptr(0); v < 24; v++ {
		for i := uintptr(0); i < 8; i++ {
			spuReg(v*16 + i*2).Set(sndVoiceInitRegs[i])
		}
	}
	for i := uintptr(0); i < 16; i++ {
		spuReg(spuMainVolL + i*2).Set(sndCommonInitRegs[i])
	}
	sndVoiceInit(24)
	seqMarkCB = [seqMax][16]func(seq, track int16, v uint8){}
	sndTickRate = 60
	seqOpenMask = 0
	sndInTick = false
}

// sndSetTableSize resets the scores, as SsSetTableSize did for the
// table the startup gave.
//
// libps 0x80014958, 480 bytes
func sndSetTableSize() {
	for i := range seqScores {
		sc := &seqScores[i]
		sc.flags = 0
		sc.nextSeq = 0xff
		sc.nextTrack = 0
		sc.volTarget, sc.volCount = 0, 0
		sc.fadeTotal, sc.fadeLeft = 0, 0
		sc.volStep = 0
		sc.accelTempo, sc.accelLeft, sc.accelTicks = 0, 0, 0
		sc.accelStep = 0
		sc.volL, sc.volR = 0x7f, 0x7f
		sc.volL0, sc.volR0 = 0x7f, 0x7f
	}
}

// SsSetTickMode sets how often the tick runs: mode 0 at 50 or 60 a
// second by the video mode, 1 at 60, 2 at 240, 3 at 120, 4 at 50, 5 at
// the vertical blank, and with 0x1000 added a rate of its own.
//
// libps 0x800147EC, 364 bytes
//
//export SsSetTickMode
func SsSetTickMode(mode int32) {
	vm := GetVideoMode()
	if mode&0x1000 != 0 {
		sndCustom = true
		sndTickMode = mode & 0xfff
	} else {
		sndCustom = false
		sndTickMode = mode
	}
	m := sndTickMode
	if m >= 6 {
		sndTickRate = m
		return
	}
	switch m {
	case 0:
		sndTickRate = 60
		if vm == 1 {
			sndTickRate = 50
		}
	case 1:
		sndTickRate = 60
		sndTickMode = 60
		if vm == 0 {
			sndTickMode = 5
		}
	case 2:
		sndTickRate = 240
	case 3:
		sndTickRate = 120
	case 4:
		sndTickRate = 50
		sndTickMode = 50
		if vm == 1 {
			sndTickMode = 5
		}
	case 5:
		sndTickRate = 60
		if vm == 1 {
			sndTickRate = 50
		}
	}
}

// SsStart starts the tick: on the vertical blank for mode 5, before the
// callback that was there; on a root counter at the rate otherwise.
//
// libps 0x80013D9C, 32 bytes
//
//export SsStart
func SsStart() { sndStartTick(1) }

// SsStart2 starts the tick on the vertical blank through VSyncCallback.
//
// libps 0x80013DBC, 32 bytes
//
//export SsStart2
func SsStart2() { sndStartTick(0) }

// sndStartTick starts the tick, from the vertical blank's interrupt
// when onIntr is set and from VSyncCallback otherwise.
//
// libps 0x80013B34, 616 bytes
func sndStartTick(onIntr int32) {
	for i := 0; i < 1000; i++ {
		volatile.StoreUint32(&spuSink, uint32(i))
	}
	sndOnVSync = false
	sndCounter = 6
	sndHalfRate = false
	sndSavedCB = nil
	spec := uint32(0xf2000002)
	target := uint16(0)
	switch mode := sndTickMode; {
	case mode == 2:
		target = 0x44e8
	case mode == 0:
		sndCounter = 0xff
		return
	case mode == 3:
		target = 0x89d0
	case mode == 5:
		sndCounter = 0
		if onIntr == 0 {
			sndOnVSync = true
		} else {
			spec = 0xf2000003
			target = 1
		}
	default:
		if sndCustom {
			return
		}
		if mode < 70 {
			target = uint16(0x204cc0 / mode)
			sndHalfRate = true
		} else {
			target = uint16(0x409980 / mode)
		}
	}
	EnterCriticalSection()
	if sndOnVSync {
		// The original gave VSyncCallback the tick; here it joins the
		// callbacks the vertical blank runs after the program's.
		vsyncCallbacks[7] = sndTick
	} else {
		ResetRCnt(spec)
		SetRCnt(spec, target, 0x1000)
		if sndCounter == 0 {
			sndSavedCB = InterruptCallback(0, nil)
			InterruptCallback(0, sndTickChained)
		} else if sndHalfRate {
			InterruptCallback(int32(sndCounter), sndTickHalf)
		} else {
			InterruptCallback(int32(sndCounter), sndTick)
		}
	}
	ExitCriticalSection()
}

// sndTickChained runs the vertical blank's earlier callback, then the
// tick.
//
// libps 0x80013DDC, 64 bytes
func sndTickChained() {
	if sndSavedCB != nil {
		sndSavedCB()
	}
	sndTick()
}

// sndTickHalf runs the tick every other time.
//
// libps 0x80013E1C, 80 bytes
func sndTickHalf() {
	if !sndToggle {
		sndToggle = true
		return
	}
	sndToggle = false
	sndTick()
}

// sndTick is the tick: the voices, then each open sequence's tracks by
// their flags.
//
// libps 0x80013E6C, 628 bytes
func sndTick() {
	if sndInTick {
		return
	}
	sndInTick = true
	sndVoiceTick()
	for s := int32(0); s < seqMax; s++ {
		if seqOpenMask&(1<<uint(s)) == 0 {
			continue
		}
		for t := int32(0); t < trackMax; t++ {
			sc := seqAt(s, t)
			if sc.flags&flagPlay != 0 {
				seqTick(s, t)
				if sc.flags&flagCresc != 0 {
					seqCrescTick(s, t)
				}
				if sc.flags&flagDecresc != 0 {
					seqDecrescTick(s, t)
				}
				if sc.flags&flagAccel != 0 {
					seqAccelTick(s, t)
				}
				if sc.flags&flagRitard != 0 {
					seqAccelTick(s, t)
				}
			}
			if sc.flags&flagStop != 0 {
				sndAllKeysOff(seqST(s, t))
				sc.playing = 0
				sc.flags &^= flagStop
			}
			if sc.flags&flagResume != 0 {
				sc.playing = 1
				sc.flags &^= flagResume
			}
			if sc.flags&flagClose != 0 {
				seqStop(s, t)
				sc.flags = 0
			}
		}
	}
	sndInTick = false
}

// SsSetMVol sets the main volume, each side from 0 to 127.
//
// libps 0x80010EE0, 80 bytes
//
//export SsSetMVol
func SsSetMVol(l int16, r int16) {
	sndStart()
	attr := SpuCommonAttr{Mask: 3, MVolL: l * 129, MVolR: r * 129}
	SpuSetCommonAttr(&attr)
}
