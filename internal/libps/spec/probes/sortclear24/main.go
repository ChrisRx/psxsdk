//go:build yaroze

// Command sortclear24 is a probe for spec 48: the size GsSortClear gives its
// fill in 24-bit color, for widths whose 1.5 times passes 16 bits (signed
// past 21,845, unsigned past 43,690), to tell which arithmetic Sony's library
// takes the width times 3 over 2 in. For each width it prints the fill's
// size word (height in the top half, width in the bottom), and, for a check,
// the same at 16-bit color.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var (
	tags [1 << 4]yaroze.GsOT_TAG
	ot   = yaroze.GsOT{Length: 4, Org: &tags[0]}
)

// size returns the fill packet's last word, from the entry Tag points to.
func size() uint32 {
	tag := *(*uint32)(unsafe.Pointer(ot.Tag))
	return *(*uint32)(yaroze.Pointer(uintptr(0x80000000|tag&0xffffff) + 12))
}

func hex(w uint32) string {
	const digits = "0123456789abcdef"
	var b [8]byte
	for i := 7; i >= 0; i-- {
		b[i] = digits[w&15]
		w >>= 4
	}
	return string(b[:])
}

func main() {
	yaroze.ResetGraph(0)
	for _, w := range []uint16{320, 640, 11000, 21845, 21846, 30000, 32767, 32768, 43690, 43691, 50000, 65535} {
		for _, vram := range []uint16{1, 0} {
			yaroze.GsInitGraph(w, 240, yaroze.GsOFSGPU, 0, vram)
			yaroze.GsDefDispBuff(0, 0, 0, 240)
			yaroze.GsClearOt(0, 0, &ot)
			yaroze.GsSortClear(1, 2, 3, &ot)
			println("width", w, "vram", vram, "size", hex(size()))
		}
	}
	println("END")
}
