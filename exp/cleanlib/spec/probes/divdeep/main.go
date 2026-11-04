//go:build yaroze

// Command divdeep is a probe for the spec: GsSortObject4 under the
// subdivision levels above GsDIV2 (GsDIV3, GsDIV4, GsDIV5, and the unnamed
// values 6 and 7 of the same field), and the drops under subdivision: a
// polygon whose average depth comes out 0, one whose ordering table entry
// lies past the table, and one whose perspective division overflows (a
// corner nearer than half the projection distance). One primitive at a time, built in memory and
// sorted; the TTY then lists every ordering table entry that holds packets
// (the whole tag array is scanned, so an entry past the table shows too)
// and, for the cases that ask, the packets in link order from the entry,
// each word in hex with the tag's address masked. GP1 marker words
// (10800000h plus the case) go before and after each sort, so a GPU
// recording with cycle counts gives the sort's cost. Built for the
// resident library it shows Sony's behavior.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// The TMD's structures, from the File Formats manual.
type (
	TMD_HDR struct{ ID, Flag, NObjs uint32 }
	TMD_OBJ struct {
		VertTop, NVert, NormTop, NNorm, PrimTop, NPrim uint32
		Scale                                          int32
	}
	PRIM_HDR struct{ Olen, Ilen, Flag, Mode uint8 }
	TMD_VERT struct{ Vx, Vy, Vz, Pad int16 }
	TMD_NORM struct{ Nx, Ny, Nz, Pad int16 }
)

// Primitive kinds, as TMD mode and flag.
const (
	g4  = iota // gouraud quad, unlit (39h, flag 1)
	f3         // flat triangle, lit (20h)
	ft3        // flat textured triangle, lit (24h)
	ft4        // flat textured quad, lit (2Ch)
)

// Shapes. The first three face the camera in the plane z = 0.
const (
	flat   = iota // 80 by 60, so at 300 one unit is a pixel
	tiny          // 2 by 2
	medium        // 10 by 10
	ramp1         // flat with corner 0 brought 100 nearer
	ramp2         // flat with corners 0 and 1 brought 100 nearer
	ramp3         // flat with corners 0, 1 and 2 brought 100 nearer
	floor         // 1024 square, 100 below the eye (the div probe's)
)

// How much of the packets to print.
const (
	counts  = iota // the entries and their packet counts only
	compact        // and, per packet, the code word and the xy words
	full           // and every word of every packet
)

type probe struct {
	kind, shape int
	div         uint32 // the GsDOBJ2 attribute (the GsDIV field)
	otlen       uint32
	shift       int32
	z           int32 // the view's distance
	h           int32 // the projection, 300 when 0
	dump        int
}

var cases []probe

func init() {
	add := func(p probe) { cases = append(cases, p) }
	// Levels on the flat shape, otlen 12 and shift 2: depth 300 gives
	// entry 75.
	lv := func(kind int, div uint32, dump int) {
		add(probe{kind: kind, shape: flat, div: div, otlen: 12, shift: 2, z: 300, dump: dump})
	}
	lv(ft4, yaroze.GsDIV2, full) // the known level, for the checker
	lv(ft4, yaroze.GsDIV3, full)
	lv(g4, yaroze.GsDIV3, full)
	lv(f3, yaroze.GsDIV3, full)
	lv(ft3, yaroze.GsDIV3, full)
	lv(ft4, yaroze.GsDIV4, compact)
	lv(g4, yaroze.GsDIV4, compact)
	lv(f3, yaroze.GsDIV4, compact)
	lv(ft4, yaroze.GsDIV5, compact)
	lv(f3, yaroze.GsDIV5, compact)
	// The cost of each level, undivided to GsDIV5, on ft4 and f3.
	for d := uint32(0); d <= 5; d++ {
		lv(ft4, d<<9, counts)
	}
	for d := uint32(0); d <= 5; d++ {
		lv(f3, d<<9, counts)
	}
	// Depth 0: the tiny shape close to the camera, so the four depths add
	// up to less than 16 (OTZ = sum/16 for a quad, sum/12 for a triangle)
	// without any corner behind the camera. 2 and 3 give 0, 4 gives 1.
	for _, div := range []uint32{0, yaroze.GsDIV1, yaroze.GsDIV2} {
		for _, z := range []int32{2, 3, 4, 8, 100} {
			add(probe{kind: ft4, shape: tiny, div: div, otlen: 12, shift: 0, z: z, dump: compact})
			add(probe{kind: f3, shape: tiny, div: div, otlen: 12, shift: 0, z: z, dump: compact})
		}
	}
	// Entry 0 from a depth above 0: 300 >> 9.
	for _, div := range []uint32{0, yaroze.GsDIV1, yaroze.GsDIV2} {
		add(probe{kind: ft4, shape: flat, div: div, otlen: 12, shift: 9, z: 300, dump: counts})
	}
	// Past the table: depth 1024 into a table of 2 entries, at shift 10
	// (entry 1, the last), 9 (entry 2, the first past), 8 and 0; then a
	// table of 8 entries at shift 8 (entry 4) and 7 (entry 8).
	for _, div := range []uint32{0, yaroze.GsDIV1, yaroze.GsDIV2} {
		for _, sh := range []int32{10, 9, 8, 0} {
			add(probe{kind: ft4, shape: flat, div: div, otlen: 1, shift: sh, z: 1024, dump: counts})
		}
		for _, sh := range []int32{8, 7} {
			add(probe{kind: f3, shape: flat, div: div, otlen: 3, shift: sh, z: 1024, dump: counts})
		}
	}
	// Entry 2 exactly, the first past a table of 2: depth 256 at shift 7.
	for _, div := range []uint32{0, yaroze.GsDIV1, yaroze.GsDIV2} {
		add(probe{kind: ft4, shape: flat, div: div, otlen: 1, shift: 7, z: 1024, dump: counts})
	}
	// The division overflow (h >= 2 sz, GTE FLAG bit 17) without any
	// saturation: the flat shape about 150, then the tiny and medium
	// shapes nearer and farther, then the flat shape with one, two or
	// three corners 100 nearer than the rest at 200, and the floor on
	// past the camera.
	for _, div := range []uint32{0, yaroze.GsDIV1, yaroze.GsDIV2} {
		for _, z := range []int32{149, 150, 151, 200} {
			add(probe{kind: ft4, shape: flat, div: div, otlen: 12, shift: 0, z: z, dump: counts})
		}
		for _, z := range []int32{151, 200, 600} {
			add(probe{kind: ft4, shape: tiny, div: div, otlen: 12, shift: 0, z: z, dump: counts})
		}
		for _, z := range []int32{100, 151} {
			add(probe{kind: ft4, shape: medium, div: div, otlen: 12, shift: 0, z: z, dump: counts})
		}
		for _, sh := range []int{ramp1, ramp2, ramp3} {
			add(probe{kind: ft4, shape: sh, div: div, otlen: 12, shift: 0, z: 200, dump: compact})
		}
		for _, z := range []int32{-200, -300, -400, -500} {
			add(probe{kind: ft4, shape: floor, div: div, otlen: 12, shift: 0, z: z, dump: counts})
		}
	}
	// Depth 0 without the overflow: the projection 4 and the tiny shape at
	// 3 (OTZ 0 for both kinds), 4 (1 for the quad, 0 for the triangle), 5
	// and 8.
	for _, div := range []uint32{0, yaroze.GsDIV1, yaroze.GsDIV2} {
		for _, z := range []int32{3, 4, 5, 8} {
			add(probe{kind: ft4, shape: tiny, div: div, otlen: 12, shift: 0, z: z, h: 4, dump: compact})
			add(probe{kind: f3, shape: tiny, div: div, otlen: 12, shift: 0, z: z, h: 4, dump: compact})
		}
	}
	// Where the near drop under subdivision sits against the projection:
	// the flat shape about h/2 at h 200 and at an odd h, 301; the tiny
	// shape at 1 and 2 with h 4; and triangles with two or all three
	// corners 100 nearer at 200.
	for _, div := range []uint32{0, yaroze.GsDIV1} {
		for _, z := range []int32{99, 100, 101} {
			add(probe{kind: ft4, shape: flat, div: div, otlen: 12, shift: 0, z: z, h: 200, dump: counts})
		}
		for _, z := range []int32{149, 150, 151} {
			add(probe{kind: ft4, shape: flat, div: div, otlen: 12, shift: 0, z: z, h: 301, dump: counts})
		}
		for _, z := range []int32{1, 2} {
			add(probe{kind: ft4, shape: tiny, div: div, otlen: 12, shift: 0, z: z, h: 4, dump: counts})
		}
		add(probe{kind: f3, shape: ramp2, div: div, otlen: 12, shift: 0, z: 200, dump: compact})
		add(probe{kind: f3, shape: ramp3, div: div, otlen: 12, shift: 0, z: 200, dump: compact})
	}
	// The field's values 6 and 7, last in case they go wrong.
	lv(f3, 6<<9, counts)
	lv(f3, 7<<9, counts)
}

var (
	ot      yaroze.GsOT
	tags    [1 << 14]yaroze.GsOT_TAG
	packets [1 << 17]uint32
	model   [256]uint32
	obj     yaroze.GsDOBJ2
	coord   yaroze.GsCOORDINATE2
	view    yaroze.GsVIEW2
	light   = yaroze.GsF_LIGHT{Vx: 0, Vy: 0, Vz: 1, R: 0xff, G: 0xc0, B: 0x80}
)

func mark(n int) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10800000|uint32(n))
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	yaroze.GsSetProjection(300)
	yaroze.GsSetFlatLight(0, &light)
	yaroze.GsSetAmbient(1024, 1024, 1024)
	yaroze.GsSetLightMode(0)
	coord.Init(yaroze.WORLD)
	for c, p := range cases {
		yaroze.DrawSync(0)
		for i := range tags {
			tags[i] = 0
		}
		ot.Length = p.otlen
		ot.Org = &tags[0]
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
		yaroze.GsClearOt(0, 0, &ot)

		view.View = yaroze.IDMATRIX
		view.View.T = [3]int32{0, 0, p.z}
		view.Super = yaroze.WORLD
		yaroze.GsSetView2(&view)
		if p.h != 0 {
			yaroze.GsSetProjection(p.h)
		} else {
			yaroze.GsSetProjection(300)
		}

		build(p)
		yaroze.TMD(model[:]).Link(&obj, 0)
		obj.Coord2 = &coord
		obj.Attribute = p.div
		var lw, ls yaroze.MATRIX
		yaroze.GsGetLw(&coord, &lw)
		yaroze.GsSetLightMatrix(&lw)
		yaroze.GsGetLs(&coord, &ls)
		yaroze.GsSetLsMatrix(&ls)
		mark(c)
		yaroze.GsSortObject4(&obj, &ot, p.shift, yaroze.Scratch())
		mark(0x8000 | c)
		used := uintptr(unsafe.Pointer(yaroze.GsGetWorkBase())) - uintptr(unsafe.Pointer(&packets[0]))
		println("case", c, "kind", p.kind, "shape", p.shape, "div", p.div>>9, "otlen", p.otlen, "shift", p.shift, "z", p.z, "h", p.h, "work", used)
		report(p)

		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(uint8(c), 0, 0, &ot)
		yaroze.GsDrawOt(&ot)
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}

// The words of each kind's packet that hold a vertex.
var xyWords = [4][]int{
	g4:  {2, 4, 6, 8},
	f3:  {2, 3, 4},
	ft3: {2, 4, 6},
	ft4: {2, 4, 6, 8},
}

// inPackets reports whether the address a (its low 24 bits) is in the
// packet area.
func inPackets(a uint32) bool {
	lo := uint32(uintptr(unsafe.Pointer(&packets[0]))) & 0xffffff
	return a >= lo && a < lo+uint32(len(packets))*4
}

// report prints each tag array entry that leads to packets with its count,
// from the last entry down (the drawing order), then the packets as p.dump
// asks. It builds no strings: the heap is small beside the resident
// library and a long dump ran it out.
func report(p probe) {
	print("  entries:")
	for i := len(tags) - 1; i >= 0; i-- {
		n := 0
		for a := uint32(tags[i]) & 0xffffff; inPackets(a); n++ {
			a = *(*uint32)(yaroze.Pointer(uintptr(0x80000000 | a))) & 0xffffff
		}
		if n > 0 {
			print(" ", i, "x", n)
		}
	}
	println()
	if p.dump == counts {
		return
	}
	for i := len(tags) - 1; i >= 0; i-- {
		for a := uint32(tags[i]) & 0xffffff; inPackets(a); {
			w := (*[16]uint32)(yaroze.Pointer(uintptr(0x80000000 | a)))
			size := int(w[0] >> 24)
			print("  ")
			hex(w[0]>>24, 2)
			if p.dump == full {
				for k := 1; k <= size; k++ {
					print(" ")
					hex(w[k], 8)
				}
			} else {
				print(" ")
				hex(w[1], 8)
				for _, k := range xyWords[p.kind] {
					print(" ")
					hex(w[k], 8)
				}
			}
			println()
			a = w[0] & 0xffffff
		}
	}
}

const hexDigits = "0123456789abcdef"

// hex prints the low n hex digits of w.
func hex(w uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		d := w >> (4 * i) & 15
		print(hexDigits[d : d+1])
	}
}

func put[T any](cursor *uintptr, v T) *T {
	p := (*T)(yaroze.Pointer(*cursor))
	*p = v
	*cursor += unsafe.Sizeof(v)
	return p
}

// build writes a TMD of one object with one primitive of p's kind and
// shape into model, its offsets already addresses.
func build(p probe) {
	cursor := uintptr(unsafe.Pointer(&model[0]))
	put(&cursor, TMD_HDR{ID: 0x41, Flag: 1, NObjs: 1})
	objTable := put(&cursor, TMD_OBJ{})
	prim := cursor
	switch p.kind {
	case g4:
		put(&cursor, PRIM_HDR{Olen: 8, Ilen: 6, Flag: 1, Mode: 0x39})
		put(&cursor, [6]uint32{0x39_102030, 0x405060, 0x708090, 0xa0b0c0, 1<<16 | 0, 3<<16 | 2})
	case f3:
		put(&cursor, PRIM_HDR{Olen: 4, Ilen: 3, Flag: 0, Mode: 0x20})
		put(&cursor, [3]uint32{0x20_a08060, 0<<16 | 0, 2<<16 | 1})
	case ft3:
		put(&cursor, PRIM_HDR{Olen: 7, Ilen: 5, Flag: 0, Mode: 0x24})
		put(&cursor, [5]uint32{0x7801_0402, 0x0123_0844, 0x0000_4010, 0<<16 | 0, 2<<16 | 1})
	case ft4:
		put(&cursor, PRIM_HDR{Olen: 9, Ilen: 7, Flag: 0, Mode: 0x2c})
		put(&cursor, [7]uint32{0x7801_0402, 0x0123_0844, 0x0000_4010, 0x0000_4c50, 0<<16 | 0, 2<<16 | 1, 3})
	}
	verts := cursor
	switch p.shape {
	case tiny:
		for _, v := range [4][3]int16{{-1, -1, 0}, {1, -1, 0}, {-1, 1, 0}, {1, 1, 0}} {
			put(&cursor, TMD_VERT{Vx: v[0], Vy: v[1], Vz: v[2]})
		}
	case medium:
		for _, v := range [4][3]int16{{-5, -5, 0}, {5, -5, 0}, {-5, 5, 0}, {5, 5, 0}} {
			put(&cursor, TMD_VERT{Vx: v[0], Vy: v[1], Vz: v[2]})
		}
	case ramp1, ramp2, ramp3:
		vs := [4][3]int16{{-40, -30, 0}, {40, -30, 0}, {-40, 30, 0}, {40, 30, 0}}
		for i := 0; i <= p.shape-ramp1; i++ {
			vs[i][2] = -100
		}
		for _, v := range vs {
			put(&cursor, TMD_VERT{Vx: v[0], Vy: v[1], Vz: v[2]})
		}
	case floor:
		for _, v := range [4][3]int16{{-512, 100, 512}, {512, 100, 512}, {-512, 100, -512}, {512, 100, -512}} {
			put(&cursor, TMD_VERT{Vx: v[0], Vy: v[1], Vz: v[2]})
		}
	default:
		for _, v := range [4][3]int16{{-40, -30, 0}, {40, -30, 0}, {-40, 30, 0}, {40, 30, 0}} {
			put(&cursor, TMD_VERT{Vx: v[0], Vy: v[1], Vz: v[2]})
		}
	}
	norms := cursor
	if p.shape == floor {
		put(&cursor, TMD_NORM{Ny: -4096})
	} else {
		put(&cursor, TMD_NORM{Nz: -4096})
	}
	*objTable = TMD_OBJ{
		VertTop: uint32(verts), NVert: 4,
		NormTop: uint32(norms), NNorm: 1,
		PrimTop: uint32(prim), NPrim: 1,
	}
}
