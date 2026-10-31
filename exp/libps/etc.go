//go:build yaroze

package libps

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// vblanks is the vertical blank count, read every time it is asked for:
// the interrupt handler writes it, so a loop that waits on it must not be
// allowed to read it once and keep the value.
func vblanks() int32 {
	return int32(volatile.LoadUint32((*uint32)(unsafe.Pointer(&vsyncCount))))
}

// VSync's state: the vertical blank count it last returned at, and the
// horizontal blank count it measures from.
//
// libps 0x8004D934 and 0x8004D930
var (
	vsyncLast  int32
	hblankBase uint32
)

// VSync waits for the vertical blank. Mode 0 waits for the next one, a
// mode n above 1 for the nth after the last wait, and both return how
// many horizontal blanks the last wait took, which is a frame's length in
// lines. Mode -1 returns the count of vertical blanks since the library
// started, and mode 1 the horizontal blanks so far, without waiting. On an
// interlaced display the wait goes on to the other field. A wait that
// takes too long is given up with a message.
//
// libps 0x8003CA68, 328 bytes
//
// It is kept out of line, so that a profile can tell the wait from the
// frame's work.
//
//go:noinline
//export VSync
func VSync(mode int32) int32 {
	stat := gp1.Get()
	hblanks := int32((timer1Count.Get() - hblankBase) & 0xffff)
	switch {
	case mode < 0:
		return vblanks()
	case mode == 1:
		return hblanks
	}
	var target, patience int32
	if mode > 0 {
		target = vsyncLast - 1 + mode
		patience = mode - 1
	} else {
		target = vsyncLast
	}
	waitVBlank(target, patience)
	stat = gp1.Get()
	waitVBlank(vblanks()+1, 1)
	if stat&gpuInterlaced != 0 && (stat^gp1.Get())&gpuOddField == 0 {
		for (stat^gp1.Get())&gpuOddField == 0 {
		}
	}
	vsyncLast = vblanks()
	hblankBase = timer1Count.Get()
	return hblanks
}

// waitVBlank waits until the vertical blank count passes target, giving
// up when no vertical blank has come for about four seconds of
// horizontal blanks: the kernel's pad handler and root counter 3 are
// then told to acknowledge the vertical blank themselves again, since the
// library's handler seems not to be.
//
// The original gave up after patience times 32768 polls of its loop,
// which on the console was a frame or more for each count asked for. A
// Go loop polls several times faster and would give up within a frame,
// so the wait watches root counter 1 instead, which counts horizontal
// blanks, and gives up only when the count stops coming; patience says
// whether to wait at all.
//
// libps 0x8003CBB0, 156 bytes
func waitVBlank(target, patience int32) {
	if vblanks() >= target {
		return
	}
	if patience <= 0 {
		timeoutVBlank()
		return
	}
	seen := vblanks()
	start := timer1Count.Get()
	for {
		n := vblanks()
		if n >= target {
			return
		}
		if n != seen {
			seen, start = n, timer1Count.Get()
		}
		if (timer1Count.Get()-start)&0xffff > 60000 {
			timeoutVBlank()
			return
		}
	}
}

// timeoutVBlank is what a wait that ran out does.
func timeoutVBlank() {
	kputs("VSync: timeout\n")
	changeClearPAD(0)
	changeClearRCnt(3, 0)
}
