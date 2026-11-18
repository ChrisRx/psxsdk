//go:build yaroze

// Command sortclear is a probe for the spec: GsSortClear around
// GsDefDispBuff and GsSwapDispBuff calls. After each step it prints the
// active buffer, GsDRAWENV's clip and offset, and the ordering table from
// its Tag: each entry of the table as ot[i], and each packet as its words,
// named by the order its address was first seen (pkt A, pkt B) since the
// packets are the library's own and lie elsewhere in each build.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var (
	packets [1024]uint32
	tags    [1 << 4]yaroze.GsOT_TAG
	ot      = yaroze.GsOT{Length: 4, Org: &tags[0]}
	seen    []uintptr
)

func hex(w uint32) string {
	const hexDigits = "0123456789abcdef"
	var b [8]byte
	for i := 7; i >= 0; i-- {
		b[i] = hexDigits[w&15]
		w >>= 4
	}
	return string(b[:])
}

// name gives a packet's address a letter, by first sight.
func name(a uintptr) string {
	for i, s := range seen {
		if s == a {
			return string(rune('A' + i))
		}
	}
	seen = append(seen, a)
	return string(rune('A' + len(seen) - 1))
}

// dump prints the table from its Tag.
func dump() {
	p := (*uint32)(unsafe.Pointer(ot.Tag))
	line := "  tag"
	lo := uintptr(unsafe.Pointer(&tags[0]))
	for n := 0; n < 60 && p != nil; n++ {
		a := uintptr(unsafe.Pointer(p))
		tag := *p
		size := int(tag >> 24)
		if a >= lo && a < lo+uintptr(len(tags))*4 {
			line += " ot[" + itoa(int(a-lo)/4) + "]"
			if size != 0 {
				line += "!" + itoa(size)
			}
		} else {
			line += " pkt " + name(a) + ":"
			for k := 1; k <= size; k++ {
				line += " " + hex(*(*uint32)(unsafe.Add(unsafe.Pointer(p), 4*k)))
			}
			line += ";"
		}
		next := tag & 0xffffff
		if next == 0xffffff {
			break
		}
		p = (*uint32)(unsafe.Pointer(uintptr(0x80000000 | next)))
	}
	println(line)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}

// state prints the active buffer and GsDRAWENV's clip and offset.
func state(what string) {
	e := yaroze.GsDRAWENV()
	println("  "+what, "active", yaroze.GsGetActiveBuff(), "clip", e.Clip.X, e.Clip.Y, e.Clip.W, e.Clip.H, "ofs", e.Ofs[0], e.Ofs[1])
}

func clear() { yaroze.GsClearOt(0, 0, &ot) }

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
	yaroze.DrawSync(0)

	println("case 0: sort clear")
	clear()
	state("init")
	yaroze.GsSortClear(10, 20, 30, &ot)
	dump()

	println("case 1: def disp buff swapped, no new sort")
	yaroze.GsDefDispBuff(0, 240, 0, 0)
	state("def")
	dump()

	println("case 2: sort clear again into the same table")
	yaroze.GsSortClear(40, 50, 60, &ot)
	dump()

	println("case 3: sort, def disp buff elsewhere, sort")
	clear()
	yaroze.GsSortClear(1, 2, 3, &ot)
	dump()
	yaroze.GsDefDispBuff(64, 16, 64, 256)
	state("def")
	dump()
	yaroze.GsSortClear(4, 5, 6, &ot)
	dump()

	println("case 4: swap disp buff, sort")
	clear()
	yaroze.GsSwapDispBuff()
	state("swap")
	yaroze.GsSortClear(7, 8, 9, &ot)
	dump()

	println("case 5: sort, swap disp buff, sort")
	clear()
	yaroze.GsSortClear(11, 12, 13, &ot)
	dump()
	yaroze.GsSwapDispBuff()
	state("swap")
	dump()
	yaroze.GsSortClear(14, 15, 16, &ot)
	dump()

	println("case 6: def disp buff, sort, def disp buff with the same values")
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	state("def")
	clear()
	yaroze.GsSortClear(21, 22, 23, &ot)
	dump()
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	state("def")
	dump()

	println("case 7: sort, def disp buff swapped, draw")
	clear()
	yaroze.GsSortClear(31, 32, 33, &ot)
	yaroze.GsDefDispBuff(0, 240, 0, 0)
	state("def")
	dump()
	yaroze.GsDrawOt(&ot)
	yaroze.DrawSync(0)
	state("drawn")
	dump()

	println("case 8: def disp buff, swap, def disp buff, sort")
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsSwapDispBuff()
	state("swap")
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	state("def")
	clear()
	yaroze.GsSortClear(41, 42, 43, &ot)
	dump()

	println("done")
	for {
		yaroze.VSync(0)
	}
}
