//go:build yaroze

// Command readsync is a probe for the spec: when CdReadSync(1) reports a
// read's sectors as all come, against the drive's responses. It reads an
// eight-sector file three times, then one and two of its sectors, polling CdReadSync(1) without waiting
// for a frame, and writes GP1 10000100h plus the count each time the count
// changes (and 1000010Fh for a negative one), so a trace of the drive's
// commands and responses next to the GPU stream shows each change.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

var (
	data [8 * 512]uint32
	file = disc.Register("ASSETS/DATA.BIN", "assets/DATA.BIN").Into(data[:])
)

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

func main() {
	yaroze.ResetGraph(0)
	if f := disc.LoadAll(300); f != nil {
		println("load failed")
	}
	sectors := [...]int32{8, 8, 8, 1, 2, 1}
	for round, n := range sectors {
		for range 10 + round*7 {
			yaroze.VSync(0)
		}
		mark(0xf0 | uint32(round))
		var pos yaroze.CdlLOC
		yaroze.CdIntToPos(int32(file.Sector()), &pos)
		if yaroze.CdControl(yaroze.CdlSetloc, &pos.Minute, nil) == 0 || yaroze.CdRead(n, &data[0], yaroze.CdlModeSpeed) == 0 {
			println("read did not start")
			continue
		}
		last := int32(-100)
		for {
			n := yaroze.CdReadSync(1, nil)
			if n != last {
				if n < 0 {
					mark(0x0f)
				} else {
					mark(uint32(n))
				}
				last = n
			}
			if n <= 0 {
				break
			}
		}
		// Keep polling a while after, in case the count moves again.
		for range 2000 {
			if n := yaroze.CdReadSync(1, nil); n != last {
				mark(uint32(n) & 0xff)
				last = n
			}
		}
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}
