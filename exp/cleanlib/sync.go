//go:build yaroze

package cleanlib

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// Video modes, for SetVideoMode.
const (
	modeNTSC = 0
	modePAL  = 1
)

var video struct {
	mode int32
}

// SetVideoMode sets the video mode GsInitGraph sets the display to,
// 0 for NTSC, the default, or 1 for PAL, and returns the mode before. It sets
// the rate sequences open at to the new mode's, whatever SsSetTickMode set
// (probe spec/probes/tickrate).
//
//export SetVideoMode
func SetVideoMode(mode int32) int32 {
	old := video.mode
	video.mode = mode
	tickRate = videoRate()
	return old
}

// Root counter 1, which counts horizontal blanks for VSync.
var (
	lineCount = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f801110)))
	lineMode  = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f801114)))
)

// startLineCounter sets root counter 1 counting horizontal blanks, from 0.
// It runs free: VSync measures by the difference of two readings, and a
// program may read and reset it as RCntCNT1.
func startLineCounter() {
	lineMode.Set(0x0100)
	lineCount.Set(0)
}

var vsync struct {
	// blanks counts the vertical blanks since the library started, which
	// the interrupt handler advances.
	blanks uint32

	// last is blanks when VSync last returned, and lines the line
	// counter then.
	last  uint32
	lines uint32

	// callback is the function VSyncCallback installed, called at each
	// vertical blank after the count, or 0.
	callback uint32
}

// VSyncCallback installs the C function at f to be called from the
// interrupt handler at each vertical blank, with interrupts off, or with
// f 0 removes it (probe spec/probes/dispsys: one call a blank, none in a
// critical section).
//
//export VSyncCallback
func VSyncCallback(f uintptr) {
	libStart()
	// The kernel loads gp from the exit buffer on its way to the handler,
	// and a C function finds its small data through it.
	intr.exit.gp = readGP()
	volatile.StoreUint32(&vsync.callback, uint32(f))
}

// VSync waits for the vertical blank. With mode 0 it waits for the next
// one; with a mode n above 1, until n have passed since the last waiting
// VSync returned, and for the next one if they already have, so it always
// waits for one (probe spec/probes/vsynclate); with mode 1 it does not wait.
// Those return the horizontal lines from the last waiting VSync's return to
// this call, before any wait: the time the program took, as offscr's readout
// shows (45 lines, not a frame's 263). Only a mode that waits moves that start,
// so two VSync(1) in a row read about the same (probe spec/probes/vsyncone). A
// negative mode returns the vertical blanks since the library started, without
// waiting.
//
//export VSync
func VSync(mode int32) int32 {
	libStart()
	now := volatile.LoadUint32(&vsync.blanks)
	if mode < 0 {
		return int32(now)
	}
	// The difference wraps at 16 bits, as the counter does, so a program
	// that resets the counter in between sees a large count.
	lines := (lineCount.Get() - vsync.lines) & 0xffff
	switch {
	case mode == 1:
		return int32(lines)
	case mode == 0:
		waitBlanks(now + 1)
	default:
		target := vsync.last + uint32(mode)
		if int32(target-now) <= 0 {
			target = now + 1
		}
		waitBlanks(target)
	}
	vsync.last = volatile.LoadUint32(&vsync.blanks)
	vsync.lines = lineCount.Get() & 0xffff
	return int32(lines)
}

// waitBlanks waits until the count of vertical blanks reaches n.
func waitBlanks(n uint32) {
	for int32(volatile.LoadUint32(&vsync.blanks)-n) < 0 {
	}
}

// GetVideoMode returns the mode SetVideoMode last set, NTSC until then.
//
//export GetVideoMode
func GetVideoMode() int32 { return video.mode }

// The root counters are the kernel's. A counter's spec is RCntCNT0 to
// RCntCNT3, F2000000h to F2000003h; the kernel takes the counter's number.

// GetRCnt returns the value of the root counter spec.
//
//export GetRCnt
func GetRCnt(spec uint32) int32 {
	return libps_biosB(biosGetRCnt, uintptr(spec&3), 0, 0, 0)
}

// StartRCnt enables the root counter spec's interrupt, returning 1.
//
//export StartRCnt
func StartRCnt(spec uint32) int32 {
	return libps_biosB(biosStartRCnt, uintptr(spec&3), 0, 0, 0)
}

// ResetRCnt sets the root counter spec back to 0, returning 1.
//
//export ResetRCnt
func ResetRCnt(spec uint32) int32 {
	return libps_biosB(biosResetRCnt, uintptr(spec&3), 0, 0, 0)
}
