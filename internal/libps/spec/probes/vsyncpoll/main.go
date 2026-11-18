//go:build yaroze

// vsyncpoll finds where in VSync(-1) the library reads its count of
// vertical blanks, for spec 45's late count: whether a vertical blank
// interrupt that comes during the call is in the count it returns. After
// VSync(0) it waits a few more cycles each field, so that the next blank
// comes at a different point of its poll, and then polls in a tight loop:
// timer 2 (the system clock), VSync(-1), timer 2 again. A poll whose call
// took more than minGap cycles had the interrupt in it; the poll ends the
// field when the count went up. Each field prints one line: the delay, the
// call's time and whether the count was in its result ("in call"), or where
// the interrupt came instead (before or after the call). At the end, for
// each display, the shortest call and how many calls held the interrupt and
// how many of those counted it: on a console that fraction of the call, by
// its time, is where the count is read. Then VSync(0) the same way, a few
// cycles later each field after the VSync(0) before, printing the lines each
// returned. The emulator's TestVSyncPoll times both from outside. 240 lines
// first, then 640 by 480 interlaced.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timer2    = 0x1f801120
	timer2Mod = 0x1f801124
	scratch   = 0x1f800300
	minGap    = 400
	fields    = 48
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func store(addr uintptr, v uint32) { volatile.StoreUint32((*uint32)(yaroze.Pointer(addr)), v) }

func record(name string) {
	least := uint32(0xffff)
	inCall, counted := 0, 0
	for f := uint32(0); f < fields; f++ {
		yaroze.VSync(0)
		for i := uint32(0); i < f; i++ {
			store(scratch, i)
		}
		count := yaroze.VSync(-1)
		prev := load(timer2) & 0xffff
		where := "no gap seen"
		took, got := uint32(0), 0
		for polls := 0; polls < 100000; polls++ {
			a := load(timer2) & 0xffff
			c := yaroze.VSync(-1)
			b := load(timer2) & 0xffff
			switch call := (b - a) & 0xffff; {
			case call > minGap:
				where = "in call"
				took = call
				if c != count {
					got = 1
				}
			case (a-prev)&0xffff > minGap:
				where = "between calls"
			case call < least:
				least = call
			}
			if c != count {
				break
			}
			prev = b
		}
		if where == "in call" {
			inCall++
			counted += got
			println("POLL", name, "delay", f, "in call", took, "counted", got)
		} else {
			println("POLL", name, "delay", f, where)
		}
	}
	println("POLL", name, "least call", least, "in call", inCall, "counted", counted)
}

// waits calls VSync(0) a few more cycles after the one before each field,
// for TestVSyncPoll to time its return from outside against the end of the
// interrupt and GPUSTAT bit 31. It prints the lines each call returned.
func waits(name string) {
	var got [fields]int32
	for f := uint32(0); f < fields; f++ {
		yaroze.VSync(0)
		for i := uint32(0); i < f; i++ {
			store(scratch, i)
		}
		got[f] = yaroze.VSync(0)
	}
	for f, r := range got {
		println("WAIT", name, "delay", f, "lines", r)
	}
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	saved := load(timer2Mod)
	store(timer2Mod, 0) // system clock, free running
	yaroze.GsInitGraph(320, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	record("240p")
	waits("240p")
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	record("480i")
	waits("480i")
	store(timer2Mod, saved)
	println("END")
}
