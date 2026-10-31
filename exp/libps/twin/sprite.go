//go:build yaroze

package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// The sprite cases, run after the hardware script in both modes, since
// the rotated sprite goes through the GTE, which whichever library set
// the screen up has set up for both: a sprite sorted by each library into
// its own packet area and table, with the packets and the table's entry
// compared; boxes; the matrices; and a TIM's header read.

//export GsSortSprite_orig
func GsSortSprite_orig(sp *yaroze.GsSPRITE, ot *yaroze.GsOT, pri uint16)

//export GsSortBoxFill_orig
func GsSortBoxFill_orig(bp *yaroze.GsBOXF, ot *yaroze.GsOT, pri uint16)

//export GsGetTimInfo_orig
func GsGetTimInfo_orig(tim *uint32, im *yaroze.GsIMAGE)

//export RotMatrix_orig
func RotMatrix_orig(r *yaroze.SVECTOR, m *yaroze.MATRIX) *yaroze.MATRIX

//export ScaleMatrix_orig
func ScaleMatrix_orig(m *yaroze.MATRIX, v *yaroze.VECTOR) *yaroze.MATRIX

//export TransMatrix_orig
func TransMatrix_orig(m *yaroze.MATRIX, v *yaroze.VECTOR) *yaroze.MATRIX

var (
	areaA, areaB [64]uint32
	tagsA, tagsB [8]yaroze.GsOT_TAG
)

// twoTables gives each library a cleared table and a packet area, and
// returns the tables.
func twoTables() (a, b *yaroze.GsOT) {
	a = &yaroze.GsOT{Length: 3, Org: &tagsA[0]}
	b = &yaroze.GsOT{Length: 3, Org: &tagsB[0]}
	for i := range tagsA {
		tagsA[i], tagsB[i] = 0xffffff, 0xffffff
	}
	areaA, areaB = [64]uint32{}, [64]uint32{}
	return a, b
}

// checkSorted compares the two packet areas, each read relative to its
// own base, and the table entry the packet was linked at.
func checkSorted(name string, a, b *yaroze.GsOT, pri int, words int) {
	wa, wb := areaA[:words], areaB[:words]
	// The tag's link is an address; compare its length byte and the rest.
	wa0, wb0 := wa[0]>>24, wb[0]>>24
	checkInt(name+" tag", int32(wa0), int32(wb0))
	check(name+" packet", bytesOf(wa[1:]), bytesOf(wb[1:]))
	ta := uintptr(tagsA[pri])&0xffffff | 0x80000000
	tb := uintptr(tagsB[pri])&0xffffff | 0x80000000
	checkInt(name+" entry", int32(ta-uintptr(unsafe.Pointer(&areaA[0]))), int32(tb-uintptr(unsafe.Pointer(&areaB[0]))))
	checkInt(name+" next", int32(uintptr(unsafe.Pointer(nextA))-uintptr(unsafe.Pointer(&areaA[0]))), int32(uintptr(unsafe.Pointer(nextB))-uintptr(unsafe.Pointer(&areaB[0]))))
}

var nextA, nextB *yaroze.PACKET

//go:extern GsOUT_PACKET_P
var outPacket *yaroze.PACKET

func bytesOf(w []uint32) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(&w[0])), len(w)*4)
}

// sortBoth sorts sp with each library.
func sortBoth(name string, sp yaroze.GsSPRITE, pri uint16, words int) {
	a, b := twoTables()
	spA, spB := sp, sp
	GsSetWorkBase_orig(yaroze.Packets(&areaA[0]))
	GsSortSprite_orig(&spA, a, pri)
	nextA = outPacket
	yaroze.GsSetWorkBase(yaroze.Packets(&areaB[0]))
	yaroze.GsSortSprite(&spB, b, pri)
	nextB = outPacket
	checkSorted(name, a, b, int(pri), words)
}

func spriteCases() {
	base := yaroze.GsSPRITE{
		Attribute: 2 << 24, X: 100, Y: 80, W: 32, H: 32, Tpage: 5, U: 8, V: 16,
		Cx: 320, Cy: 480, R: 128, G: 128, B: 128, Mx: 16, My: 16, Scalex: 4096, Scaley: 4096,
	}
	sortBoth("sprite plain", base, 3, 6)
	rot := base
	rot.Rotate = 90 << 12
	sortBoth("sprite rotated", rot, 3, 10)
	scaled := base
	scaled.Scalex, scaled.Scaley = 2<<12, 1<<12
	sortBoth("sprite scaled", scaled, 3, 10)
	both := rot
	both.Scalex, both.Scaley = 3<<11, 1<<11
	both.Rotate = -(45 << 12)
	sortBoth("sprite rotated scaled", both, 2, 10)
	off := both
	off.Attribute |= 1 << 27
	sortBoth("sprite rotoff", off, 3, 6)
	flipped := rot
	flipped.Attribute |= 1<<22 | 1<<23 | 1<<6 | 1<<30
	sortBoth("sprite flipped", flipped, 1, 10)
	four := base
	four.Attribute = 0<<24 | 2<<28
	four.Cx, four.Cy = 640, 511
	sortBoth("sprite 4-bit", four, 3, 6)
	eight := base
	eight.Attribute = 1 << 24
	eight.Scalex = 4096
	eight.Attribute |= 1 << 22
	sortBoth("sprite 8-bit flipped", eight, 3, 10)

	for i, box := range []yaroze.GsBOXF{
		{Attribute: 0, X: 10, Y: 10, W: 50, H: 20, R: 200, G: 100, B: 50},
		{Attribute: 1<<30 | 1<<28, X: 10, Y: 10, W: 50, H: 20, R: 200, G: 100, B: 50},
	} {
		a, b := twoTables()
		bA, bB := box, box
		GsSetWorkBase_orig(yaroze.Packets(&areaA[0]))
		GsSortBoxFill_orig(&bA, a, 4)
		nextA = outPacket
		yaroze.GsSetWorkBase(yaroze.Packets(&areaB[0]))
		yaroze.GsSortBoxFill(&bB, b, 4)
		nextB = outPacket
		checkSorted("box "+itoa(i), a, b, 4, 5)
	}

	for i, r := range []yaroze.SVECTOR{{}, {Vx: 1024}, {Vy: 2048}, {Vz: 3000}, {Vx: -500, Vy: 700, Vz: -1200}, {Vx: 4096 + 100, Vy: -4096 - 7}} {
		var ma, mb yaroze.MATRIX
		RotMatrix_orig(&r, &ma)
		yaroze.RotMatrix(&r, &mb)
		check("RotMatrix "+itoa(i), bytesOf32(&ma), bytesOf32(&mb))
		v := yaroze.VECTOR{Vx: 2 << 12, Vy: 1 << 12, Vz: 4096}
		ScaleMatrix_orig(&ma, &v)
		yaroze.ScaleMatrix(&mb, &v)
		check("ScaleMatrix "+itoa(i), bytesOf32(&ma), bytesOf32(&mb))
		t := yaroze.VECTOR{Vx: 7, Vy: -8, Vz: 1000}
		TransMatrix_orig(&ma, &t)
		yaroze.TransMatrix(&mb, &t)
		check("TransMatrix "+itoa(i), bytesOf32(&ma), bytesOf32(&mb))
	}

	// TIMs: a 4-bit one with a palette and a 16-bit one without, their
	// headers as the files have them, after the id word.
	tim4 := []uint32{0x08, 44, 320 | 480<<16, 16 | 1<<16, 1, 2, 3, 4, 5, 6, 7, 8, 44, 640 | 0<<16, 2 | 8<<16}
	tim16 := []uint32{0x02, 12 + 8, 500 | 100<<16, 2 | 1<<16, 0, 0}
	for i, tim := range [][]uint32{tim4, tim16} {
		var ia, ib yaroze.GsIMAGE
		GsGetTimInfo_orig(&tim[0], &ia)
		yaroze.GsGetTimInfo(&tim[0], &ib)
		check("GsGetTimInfo "+itoa(i), unsafe.Slice((*byte)(unsafe.Pointer(&ia)), 28), unsafe.Slice((*byte)(unsafe.Pointer(&ib)), 28))
	}
}

func bytesOf32(m *yaroze.MATRIX) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(m)), 32)
}
