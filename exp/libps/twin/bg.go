//go:build yaroze

package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

// The fixed background's packet lists, built and filled under each
// library and printed with their links made relative, and the root
// counters in place.

//export GsInitFixBg16_orig
func GsInitFixBg16_orig(bg *yaroze.GsBG, work *uint32)

//export GsSortFixBg16_orig
func GsSortFixBg16_orig(bg *yaroze.GsBG, work *uint32, otp *yaroze.GsOT, pri uint16)

//export GetRCnt_orig
func GetRCnt_orig(t uint32) int32

//export ResetRCnt_orig
func ResetRCnt_orig(t uint32) int32

//export StartRCnt_orig
func StartRCnt_orig(t uint32) int32

// A map of four by three cells, one of them empty, scrolled off a cell
// boundary both ways, with a depth and a blend set in the attribute.
var (
	bgCells = [4]yaroze.GsCELL{
		{U: 0, V: 0, Cba: 0x1234, Tpage: 0x05},
		{U: 16, V: 0, Cba: 0x1234, Tpage: 0x3f},
		{U: 32, V: 16, Cba: 0x4321, Tpage: 0x0a},
		{U: 48, V: 48, Cba: 0x0001, Tpage: 0x1f},
	}
	bgIndex = [12]uint16{0, 1, 2, 3, 3, 2, 1, 0, 1, 0xffff, 3, 2}
	bgMap   = yaroze.GsMAP{Cellw: 16, Cellh: 16, Ncellw: 4, Ncellh: 3, Base: &bgCells[0], Index: &bgIndex[0]}
	bg      = yaroze.GsBG{Attribute: 0x30000000 | 1<<24, W: 320, H: 240, Scrollx: -5, Scrolly: 37, R: 128, G: 128, B: 128, Map: &bgMap}
	bgWork  [3400]uint32
	bgTags  [8]yaroze.GsOT_TAG
	bgOT    = yaroze.GsOT{Length: 3, Org: &bgTags[0]}
)

// bgScript builds the lists and sorts the background, then prints them.
func bgScript(mode int32) {
	if mode == 0 {
		GsClearOt_orig(0, 0, &bgOT)
		GsInitFixBg16_orig(&bg, &bgWork[0])
		GsSortFixBg16_orig(&bg, &bgWork[0], &bgOT, 2)
	} else {
		yaroze.GsClearOt(0, 0, &bgOT)
		yaroze.GsInitFixBg16(&bg, &bgWork[0])
		yaroze.GsSortFixBg16(&bg, &bgWork[0], &bgOT, 2)
	}
	base := uintptr(unsafe.Pointer(&bgWork[0]))
	print("bg lists ", uintptr(bgWork[0])-base, " ", uintptr(bgWork[1])-base, "\n")
	for list := 0; list < 2; list++ {
		p := uintptr(bgWork[list])
		sum := uint32(0)
		// The draw offset packet, the cell packets for a 320 by 240
		// screen, and the draw offset packet that ends the list.
		for i := 0; i < 1+21*16+1; i++ {
			tag := *(*uint32)(unsafe.Pointer(p))
			link := uintptr(tag&0xffffff | 0x80000000)
			n := int(tag >> 24)
			rel := uint32(0xffffff)
			if link >= base && link < base+uintptr(len(bgWork))*4 {
				rel = uint32(link - base)
			}
			words := 1
			if i > 0 && i < 1+21*16 {
				words = 4
			}
			sum = sum*31 + rel + uint32(n)<<24
			if i < 3 || i >= 21*16-1 {
				print("bg ", list, " ", i, " ", rel, "/", n)
			}
			for w := 1; w <= words; w++ {
				v := *(*uint32)(unsafe.Pointer(p + uintptr(w)*4))
				sum = sum*31 + v
				if i < 3 || i >= 21*16-1 {
					print(" ", hex32(v))
				}
			}
			if i < 3 || i >= 21*16-1 {
				print("\n")
			}
			p += uintptr(words)*4 + 4
		}
		print("bg sum ", list, " ", sum, "\n")
	}
	printOT("bg ot", &bgOT)
}

// intrMask is the interrupt mask register, which StartRCnt sets bits of.
var intrMask = (*uint32)(unsafe.Pointer(uintptr(0x1f801074)))

// checkRCnt compares the root counter calls: what each returns, and
// which bits StartRCnt sets, from the same mask each time.
func checkRCnt() {
	for t := uint32(0); t < 4; t++ {
		name := itoa(int(t))
		checkInt("ResetRCnt "+name, ResetRCnt_orig(t), yaroze.ResetRCnt(t))
		mask := volatile.LoadUint32(intrMask)
		a := StartRCnt_orig(t)
		ma := volatile.LoadUint32(intrMask)
		volatile.StoreUint32(intrMask, mask)
		b := yaroze.StartRCnt(t)
		mb := volatile.LoadUint32(intrMask)
		volatile.StoreUint32(intrMask, mask)
		checkInt("StartRCnt "+name, a, b)
		checkInt("StartRCnt mask "+name, int32(ma), int32(mb))
	}
	checkInt("GetRCnt 3", GetRCnt_orig(3), yaroze.GetRCnt(3))
	// A counter just reset reads a little under either library.
	ResetRCnt_orig(2)
	a, b := GetRCnt_orig(2), yaroze.GetRCnt(2)
	if a < 200 && b < 200 {
		print("ok GetRCnt 2\n")
	} else {
		print("FAIL GetRCnt 2: ", a, " ", b, "\n")
	}
}
