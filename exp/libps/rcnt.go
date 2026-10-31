//go:build yaroze

package libps

import "go.chrisrx.dev/psx/internal/volatile"

// The root counters: three 16-bit timers at 0x1f801100, 16 bytes apart,
// each a count, a mode and a target. StartRCnt lets a counter's
// interrupt through, for a callback set with InterruptCallback; the
// fourth is the vertical blank.

// rcntCount is counter t's count.
func rcntCount(t uint32) *volatile.Register16 { return reg16(0x1f801100 + uintptr(t)*16) }

// rcntIntr is each counter's bit in the interrupt mask.
//
// libps 0x8004EA38
var rcntIntr = [4]uint16{0x10, 0x20, 0x40, 0x01}

// GetRCnt is counter t's count, or 0 for a counter that is not one.
//
// libps 0x800430E8, 56 bytes
//
//export GetRCnt
func GetRCnt(t uint32) int32 {
	if t &= 0xffff; t < 3 {
		return int32(rcntCount(t).Get())
	}
	return 0
}

// ResetRCnt sets counter t's count to 0, returning 1, or 0 for a counter
// that is not one.
//
// libps 0x80043188, 56 bytes
//
//export ResetRCnt
func ResetRCnt(t uint32) int32 {
	if t &= 0xffff; t < 3 {
		rcntCount(t).Set(0)
		return 1
	}
	return 0
}

// StartRCnt lets counter t's interrupt through, returning 1, or 0 for the
// vertical blank, which is let through too.
//
// libps 0x80043120, 52 bytes
//
//export StartRCnt
func StartRCnt(t uint32) int32 {
	t &= 0xffff
	if t < 4 {
		iMask.Set(iMask.Get() | rcntIntr[t])
	}
	return b2i(t < 3)
}

// SetRCnt sets counter t's target and mode: 0x10 to count the system
// clock divided by 8 instead of the clock, or for the first two counters
// the horizontal blank and dot clock, 0x1 to count up to the target
// instead of 0xffff, and 0x1000 to interrupt at the target. It returns
// 1, or 0 for a counter that is not one.
//
// libps 0x80043048, 160 bytes
func SetRCnt(t uint32, target uint16, mode uint32) int32 {
	if t &= 0xffff; t >= 3 {
		return 0
	}
	m := uint16(0x48)
	reg16(0x1f801104 + uintptr(t)*16).Set(0)
	reg16(0x1f801108 + uintptr(t)*16).Set(target)
	if t < 2 {
		if mode&0x10 != 0 {
			m = 0x49
		}
		if mode&1 == 0 {
			m |= 0x100
		}
	} else if mode&1 == 0 {
		m = 0x248
	}
	if mode&0x1000 != 0 {
		m |= 0x10
	}
	reg16(0x1f801104 + uintptr(t)*16).Set(m)
	return 1
}
