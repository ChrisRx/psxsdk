//go:build yaroze

package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// The 2D core's script: the screen set up, two frames drawn with a clear
// each, swapped, and the library's state printed, which the test compares
// between the originals and the Go versions.

//export GsInitGraph_orig
func GsInitGraph_orig(x, y, intmode, dith, varmmode uint16)

//export GsDefDispBuff_orig
func GsDefDispBuff_orig(x0, y0, x1, y1 uint16)

//export GsClearOt_orig
func GsClearOt_orig(offset, point uint16, ot *yaroze.GsOT)

//export GsSortClear_orig
func GsSortClear_orig(r, g, b uint8, ot *yaroze.GsOT)

//export GsDrawOt_orig
func GsDrawOt_orig(ot *yaroze.GsOT)

//export GsSwapDispBuff_orig
func GsSwapDispBuff_orig()

//export GsGetActiveBuff_orig
func GsGetActiveBuff_orig() int32

//export GsSetWorkBase_orig
func GsSetWorkBase_orig(p *yaroze.PACKET)

// The originals' state, at its fixed addresses.
//
//go:extern GsDISPENV_orig
var gsDISPENV_orig yaroze.DISPENV

//go:extern GsDRAWENV_orig
var gsDRAWENV_orig yaroze.DRAWENV

//go:extern PSDCNT_orig
var psdcnt_orig uint32

//go:extern PSDIDX_orig
var psdidx_orig int16

//go:extern PSDOFSX_orig
var psdofsx_orig [2]int16

//go:extern PSDOFSY_orig
var psdofsy_orig [2]int16

var (
	otTags  [2][1 << 4]yaroze.GsOT_TAG
	ots     [2]yaroze.GsOT
	packets [2][1024]uint32
	smallOT [1]yaroze.GsOT_TAG
)

// gsScript draws two frames and prints the state.
func gsScript(mode int32) {
	for i := range ots {
		ots[i].Length = 4
		ots[i].Org = &otTags[i][0]
	}
	if mode == 0 {
		GsInitGraph_orig(320, 240, yaroze.GsNONINTER|yaroze.GsOFSGPU, 1, 0)
		GsDefDispBuff_orig(0, 0, 0, 240)
		for frame := range 2 {
			i := GsGetActiveBuff_orig()
			GsSetWorkBase_orig(yaroze.Packets(&packets[i][0]))
			GsClearOt_orig(0, 0, &ots[i])
			GsSortClear_orig(60, 120+uint8(frame)*40, 120, &ots[i])
			GsDrawOt_orig(&ots[i])
			DrawSync_orig(0)
			VSync_orig(0)
			GsSwapDispBuff_orig()
		}
		printState(&gsDISPENV_orig, &gsDRAWENV_orig, psdcnt_orig, psdidx_orig, psdofsx_orig, psdofsy_orig)
		ot := yaroze.GsOT{Length: 0, Org: &smallOT[0]}
		GsClearOt_orig(3, 7, &ot)
		printOT("small", &ot)
		GsClearOt_orig(3, 7, &ots[0])
		printOT("ot", &ots[0])
		one := yaroze.GsOT{Length: 0, Org: &smallOT[0]}
		GsClearOt_orig(0, 0, &one)
		GsSortClear_orig(1, 2, 3, &one)
		printClear("clear1", &one)
		return
	}
	yaroze.GsInitGraph(320, 240, yaroze.GsNONINTER|yaroze.GsOFSGPU, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	for frame := range 2 {
		i := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[i][0]))
		yaroze.GsClearOt(0, 0, &ots[i])
		yaroze.GsSortClear(60, 120+uint8(frame)*40, 120, &ots[i])
		yaroze.GsDrawOt(&ots[i])
		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
	}
	printState(yaroze.GsDISPENV(), yaroze.GsDRAWENV(), psdcnt(), psdidx(), psdofsx(), psdofsy())
	ot := yaroze.GsOT{Length: 0, Org: &smallOT[0]}
	yaroze.GsClearOt(3, 7, &ot)
	printOT("small", &ot)
	yaroze.GsClearOt(3, 7, &ots[0])
	printOT("ot", &ots[0])
	one := yaroze.GsOT{Length: 0, Org: &smallOT[0]}
	yaroze.GsClearOt(0, 0, &one)
	yaroze.GsSortClear(1, 2, 3, &one)
	printClear("clear1", &one)
}

// The Go library's state, under its C names.
//
//go:extern PSDCNT
var psdcntGo uint32

//go:extern PSDIDX
var psdidxGo int16

//go:extern PSDOFSX
var psdofsxGo [2]int16

//go:extern PSDOFSY
var psdofsyGo [2]int16

func psdcnt() uint32    { return psdcntGo }
func psdidx() int16     { return psdidxGo }
func psdofsx() [2]int16 { return psdofsxGo }
func psdofsy() [2]int16 { return psdofsyGo }

// printState prints the environments and the buffer bookkeeping.
func printState(disp *yaroze.DISPENV, draw *yaroze.DRAWENV, cnt uint32, idx int16, ofsx, ofsy [2]int16) {
	print("dispenv")
	printBytes(unsafe.Pointer(disp), 20)
	print("drawenv")
	printBytes(unsafe.Pointer(draw), 28)
	print("drawpacket")
	printBytes(unsafe.Pointer(&draw.Dr_env), 4*10)
	print("buffers ", cnt, " ", idx, " ", ofsx[0], " ", ofsx[1], " ", ofsy[0], " ", ofsy[1], "\n")
	// The clear packets are linked into the tables; each is found from
	// the table's last entry.
	for i := range ots {
		tag := *(*uint32)(unsafe.Pointer(ots[i].Tag))
		p := (*[4]uint32)(unsafe.Pointer(uintptr(tag&0xffffff | 0x80000000)))
		print("clear ", i, " ", hex32(p[0]>>24), " ", hex32(p[1]), " ", hex32(p[2]), " ", hex32(p[3]), "\n")
	}
}

// printOT prints a cleared table's fields and its tags relative to its
// own storage, so the two libraries' tables compare.
func printOT(name string, ot *yaroze.GsOT) {
	base := uintptr(unsafe.Pointer(ot.Org))
	print(name, " ", ot.Offset, " ", ot.Point, " ", uintptr(unsafe.Pointer(ot.Tag))-base)
	n := 1 << ot.Length
	tags := unsafe.Slice(ot.Org, n)
	for _, t := range tags {
		addr := uintptr(t)&0xffffff | 0x80000000
		switch {
		case uint32(t)&0xffffff == 0xffffff:
			print(" end", t>>24)
		case addr < base || addr >= base+uintptr(n)*4:
			// Outside the table: each library's own terminator packet,
			// which is printed as the words it holds.
			p := (*[2]uint32)(unsafe.Pointer(addr))
			print(" term/", t>>24, ":", hex32(p[0]), ":", hex32(p[1]))
		default:
			print(" ", addr-base, "/", t>>24)
		}
	}
	print("\n")
}

func printBytes(p unsafe.Pointer, n int) {
	for _, b := range unsafe.Slice((*byte)(p), n) {
		print(" ", hex(b))
	}
	print("\n")
}

func hex32(v uint32) string {
	var buf [8]byte
	for i := range buf {
		buf[7-i] = "0123456789abcdef"[v&15]
		v >>= 4
	}
	return string(buf[:])
}

// printClear prints the clear packet a one-entry table's entry links to:
// its length and words, and where its own link goes.
func printClear(name string, ot *yaroze.GsOT) {
	t := *ot.Org
	p := (*[4]uint32)(unsafe.Pointer(uintptr(t)&0xffffff | 0x80000000))
	print(name, " ", t>>24, " ", hex32(p[0]>>24), " ", hex32(p[1]), " ", hex32(p[2]), " ", hex32(p[3]))
	link := p[0] & 0xffffff
	base := uintptr(unsafe.Pointer(ot.Org))
	switch addr := uintptr(link) | 0x80000000; {
	case link == 0xffffff:
		print(" end\n")
	case addr < base || addr >= base+4:
		q := (*[2]uint32)(unsafe.Pointer(addr))
		print(" term:", hex32(q[0]), ":", hex32(q[1]), "\n")
	default:
		print(" ", addr-base, "\n")
	}
}
