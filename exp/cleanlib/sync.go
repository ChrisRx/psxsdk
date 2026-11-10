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

// Root counter 1, which counts horizontal blanks for VSync. The library's start
// writes its mode 0107h, which holds it at 0 until the next vertical blank and then
// lets it run free (probe spec/probes/linestart). VSync measures by the difference
// of two readings, and a program may read and reset it as RCntCNT1.
var (
	lineCount = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f801110)))
	lineMode  = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f801114)))
)

// vsyncLines is the line counter when VSync last returned from a wait.
//
//go:extern libps_vsync_lines
var vsyncLines uint32

var vsync struct {
	// blanks counts the vertical blanks since the library started, which
	// the interrupt handler advances.
	blanks uint32

	// last is blanks when VSync last returned.
	last uint32

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
// waiting. On a display of 480 lines, the modes that wait then wait for the
// field to change, as vsyncField says.
//
// Never inlined: hot reload applies a patch at the address of this symbol, on
// the static target as on the resident image.
//
// VSync(1) is all in the target's assembly, which goes to vsyncWait for the rest,
// or to vsyncField on a display of 480 lines.
//
//export VSync
func VSync(mode int32) int32

//export libps_vsync_wait
//go:noinline
func vsyncWait(mode int32) int32 {
	libStart()
	now := volatile.LoadUint32(&vsync.blanks)
	if mode < 0 {
		return int32(now)
	}
	// The difference wraps at 16 bits, as the counter does, so a program
	// that resets the counter in between sees a large count.
	lines := (lineCount.Get() - vsyncLines) & 0xffff
	switch {
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
	vsyncLines = lineCount.Get() & 0xffff
	return int32(lines)
}

// vsyncField is vsyncWait on a display of 480 lines, where Sony's VSync,
// after the blank it waits for, waits on for the field to change (probes
// spec/probes/vsyncphase and vsyncmodes, spec 42). VSync's assembly comes
// here when GPUSTAT bit 19 shows 480 lines; it does that test before the
// call, in as many instructions as a pass of the wait takes, so that the
// wait on any other display ends when it did before.
//
//export libps_vsync_field
//go:noinline
func vsyncField(mode int32) int32 {
	libStart()
	now := volatile.LoadUint32(&vsync.blanks)
	if mode < 0 {
		return int32(now)
	}
	lines := (lineCount.Get() - vsyncLines) & 0xffff
	target := now + 1
	if mode != 0 {
		target = vsync.last + uint32(mode)
		if int32(target-now) <= 0 {
			target = now + 1
		}
	}
	waitField(target, now)
	vsync.last = volatile.LoadUint32(&vsync.blanks)
	vsyncLines = lineCount.Get() & 0xffff
	return int32(lines)
}

// waitField waits until the count of vertical blanks, seen at the call,
// reaches n, and then until GPUSTAT bit 31, which on an interlaced display
// of 480 lines changes with the field, differs from what it read at the
// call or, when blanks were counted before the last, at the one before the
// last. With bit 31 the same, the wait ends when the field changes after the
// blank, 8 or 23 lines into it; with it already different, at the count.
// That is what Sony's VSync is seen to do with modes 0, 2 and 3 (probe
// spec/probes/vsyncmodes). It ends anyway at the next blank, should bit 31
// not change.
func waitField(n, seen uint32) {
	field := gp1.Get() >> 31
	for {
		now := volatile.LoadUint32(&vsync.blanks)
		if now == seen {
			continue
		}
		if int32(now-n) >= 0 {
			break
		}
		seen = now
		field = gp1.Get() >> 31
	}
	last := volatile.LoadUint32(&vsync.blanks)
	for gp1.Get()>>31 == field && volatile.LoadUint32(&vsync.blanks) == last {
	}
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

// The root counters, by the registers, in the target's assembly. A counter's spec is
// RCntCNT0 to RCntCNT3, F2000000h to F2000003h, of which the low half is the counter's
// number; the third and any past it return 0 and do nothing.

// GetRCnt returns the value of the root counter spec.
//
//export GetRCnt
func GetRCnt(spec uint32) int32

// StartRCnt enables the root counter spec's interrupt, returning 1.
//
//export StartRCnt
func StartRCnt(spec uint32) int32

// ResetRCnt sets the root counter spec back to 0, returning 1.
//
//export ResetRCnt
func ResetRCnt(spec uint32) int32
