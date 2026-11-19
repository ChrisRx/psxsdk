//go:build yaroze

// Command divnear is a probe for the spec: the near drop under
// subdivision when corners are behind the camera, and lines and 3D sprites
// in a TMD. One primitive at a time, built in memory and sorted with
// GsSortObject4; the TTY then lists every ordering table entry that holds
// packets and the packets in link order from the entry, each word in hex
// with the tag's address masked. GP1 marker words (10800000h plus the case)
// go before and after each sort. Built for the resident library it shows
// Sony's behavior.
//
// The near cases bring one to four corners of the flat quad (80 by 60,
// facing the camera at 200 with h 300) behind the camera, to a depth of
// -100 (the GTE saturates sz to 0), 0 and 1, so that the screen rectangle
// never drops the polygon (the saturated corners project to about twice
// their model coordinates, (80, 60), well inside) and only the depth check
// can. The projection cases set h to 2 (h >> 1 = 1) and 1 (h >> 1 = 0) on
// a larger quad so that a depth of 0 is below the half in one and not in
// the other. The line and sprite cases sort each kind of line (40h, 50h)
// and sprite (64h, 6Ch, 74h, 7Ch) undivided and under GsDIV1 and GsDIV2.
// Sony's sort never returns from a primitive it does not know, so these
// cases come last and the word terminal picks one of them: the probe
// prints the word's address at the start, and a build with --load of a
// 4-byte file at that address (the index, little-endian) runs only that
// case after the others; with the word left at FFFFFFFFh every one runs.
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

// Primitive kinds, as TMD mode.
const (
	ft4   = iota // flat textured quad, lit (2Ch)
	f3           // flat triangle, lit (20h)
	line         // line (40h)
	gline        // gradation line (50h)
	spr          // free-size sprite (64h), 16 by 8
	spr1         // 1 by 1 sprite (6Ch)
	spr8         // 8 by 8 sprite (74h)
	spr16        // 16 by 16 sprite (7Ch)
)

// Shapes, both facing the camera in the plane z = 0.
const (
	flat = iota // 80 by 60, so at 300 one unit is a pixel
	big         // 800 by 600
)

type probe struct {
	kind, shape int
	near        int   // corners 0 to near-1 are moved to vz
	vz          int16 // the moved corners' z
	flag        uint8 // the primitive's flag byte
	div         uint32
	z           int32 // the view's distance
	h           int32 // the projection
}

var cases []probe

func init() {
	add := func(p probe) { cases = append(cases, p) }
	divs := []uint32{0, yaroze.GsDIV1, yaroze.GsDIV2}
	// The emulator's drop at exactly half the projection distance, the
	// flat quad undivided and under GsDIV1 at 149, 150 and 151 with h 300.
	for _, div := range divs[:2] {
		for _, z := range []int32{149, 150, 151} {
			add(probe{kind: ft4, shape: flat, div: div, z: z, h: 300})
		}
	}
	// One to four corners behind the camera (depth -100, sz saturated to
	// 0), at the camera (depth 0) and just before it (depth 1), the rest at
	// 200, h 300.
	for _, vz := range []int16{-300, -200, -199} {
		for _, div := range divs {
			for near := 1; near <= 4; near++ {
				add(probe{kind: ft4, shape: flat, near: near, vz: vz, div: div, z: 200, h: 300})
			}
		}
	}
	// The triangle with one, two and all three corners behind the camera.
	for _, div := range divs {
		for near := 1; near <= 3; near++ {
			add(probe{kind: f3, shape: flat, near: near, vz: -300, div: div, z: 200, h: 300})
		}
	}
	// The half at h 2 (1) and h 1 (0): the big quad at 20 with none, one
	// and all four corners at depth -100.
	for _, h := range []int32{2, 1} {
		for _, div := range divs[1:] {
			for _, near := range []int{0, 1, 4} {
				add(probe{kind: ft4, shape: big, near: near, vz: -120, div: div, z: 20, h: h})
			}
		}
	}
	// Lines and sprites, the flat shape at 300, each kind undivided and
	// under GsDIV1 and GsDIV2, the lines with the light-off flag clear and
	// set: 8 cases per level, the one terminal picks or all of them.
	k := uint32(0)
	for _, div := range divs {
		for _, kind := range []int{line, gline} {
			for _, flag := range []uint8{0, 1} {
				if t := volatile.LoadUint32(&terminal); t == k || t == 0xffffffff {
					add(probe{kind: kind, shape: flat, flag: flag, div: div, z: 300, h: 300})
				}
				k++
			}
		}
		for _, kind := range []int{spr, spr1, spr8, spr16} {
			if t := volatile.LoadUint32(&terminal); t == k || t == 0xffffffff {
				add(probe{kind: kind, shape: flat, flag: 1, div: div, z: 300, h: 300})
			}
			k++
		}
	}
}

// terminal is the index of the line or sprite case to run, or FFFFFFFFh
// for all of them; a build's --load overwrites it.
var terminal uint32 = 0xffffffff

// prim is the index in model of the primitive's header word; its body
// follows, then zeros up to the vertices at verts.
const (
	prim  = 10
	verts = 32
)

var (
	ot      yaroze.GsOT
	tags    [1 << 12]yaroze.GsOT_TAG
	packets [1 << 14]uint32
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
	println("terminal", volatile.LoadUint32(&terminal), "at", &terminal, "cases", len(cases))
	for c, p := range cases {
		yaroze.DrawSync(0)
		for i := range tags {
			tags[i] = 0
		}
		ot.Length = 12
		ot.Org = &tags[0]
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
		yaroze.GsClearOt(0, 0, &ot)

		view.View = yaroze.IDMATRIX
		view.View.T = [3]int32{0, 0, p.z}
		view.Super = yaroze.WORLD
		yaroze.GsSetView2(&view)
		yaroze.GsSetProjection(p.h)

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
		yaroze.GsSortObject4(&obj, &ot, 0, yaroze.Scratch())
		mark(0x8000 | c)
		used := uintptr(unsafe.Pointer(yaroze.GsGetWorkBase())) - uintptr(unsafe.Pointer(&packets[0]))
		println("case", c, "kind", p.kind, "shape", p.shape, "near", p.near, "vz", p.vz, "flag", p.flag, "div", p.div>>9, "z", p.z, "h", p.h, "work", used)
		report()

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

// inPackets reports whether the address a (its low 24 bits) is in the
// packet area.
func inPackets(a uint32) bool {
	lo := uint32(uintptr(unsafe.Pointer(&packets[0]))) & 0xffffff
	return a >= lo && a < lo+uint32(len(packets))*4
}

// report prints each tag array entry that leads to packets with its count,
// from the last entry down (the drawing order), then every word of every
// packet. It builds no strings: the heap is small beside the resident
// library.
func report() {
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
	for i := len(tags) - 1; i >= 0; i-- {
		for a := uint32(tags[i]) & 0xffffff; inPackets(a); {
			w := (*[16]uint32)(yaroze.Pointer(uintptr(0x80000000 | a)))
			size := int(w[0] >> 24)
			print("  ")
			hex(w[0]>>24, 2)
			for k := 1; k <= size && k < 16; k++ {
				print(" ")
				hex(w[k], 8)
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
// shape into model, its offsets already addresses. Lines run from corner 0
// to corner 3; sprites sit at corner 0, texture page 0123h, palette 7801h,
// u 4 and v 5.
func build(p probe) {
	for i := range model {
		model[i] = 0
	}
	cursor := uintptr(unsafe.Pointer(&model[0]))
	put(&cursor, TMD_HDR{ID: 0x41, Flag: 1, NObjs: 1})
	objTable := put(&cursor, TMD_OBJ{})
	cursor = uintptr(unsafe.Pointer(&model[prim]))
	switch p.kind {
	case ft4:
		put(&cursor, PRIM_HDR{Olen: 9, Ilen: 7, Flag: p.flag, Mode: 0x2c})
		put(&cursor, [7]uint32{0x7801_0402, 0x0123_0844, 0x0000_4010, 0x0000_4c50, 0<<16 | 0, 2<<16 | 1, 3})
	case f3:
		put(&cursor, PRIM_HDR{Olen: 4, Ilen: 3, Flag: p.flag, Mode: 0x20})
		put(&cursor, [3]uint32{0x20_a08060, 0<<16 | 0, 2<<16 | 1})
	case line:
		put(&cursor, PRIM_HDR{Olen: 3, Ilen: 2, Flag: p.flag, Mode: 0x40})
		put(&cursor, [2]uint32{0x40_a08060, 3<<16 | 0})
	case gline:
		put(&cursor, PRIM_HDR{Olen: 4, Ilen: 3, Flag: p.flag, Mode: 0x50})
		put(&cursor, [3]uint32{0x50_a08060, 0x00_102030, 3<<16 | 0})
	case spr:
		put(&cursor, PRIM_HDR{Olen: 5, Ilen: 3, Flag: p.flag, Mode: 0x64})
		put(&cursor, [3]uint32{0x0123_0000, 0x7801_0504, 8<<16 | 16})
	case spr1:
		put(&cursor, PRIM_HDR{Olen: 4, Ilen: 2, Flag: p.flag, Mode: 0x6c})
		put(&cursor, [2]uint32{0x0123_0000, 0x7801_0504})
	case spr8:
		put(&cursor, PRIM_HDR{Olen: 4, Ilen: 2, Flag: p.flag, Mode: 0x74})
		put(&cursor, [2]uint32{0x0123_0000, 0x7801_0504})
	case spr16:
		put(&cursor, PRIM_HDR{Olen: 4, Ilen: 2, Flag: p.flag, Mode: 0x7c})
		put(&cursor, [2]uint32{0x0123_0000, 0x7801_0504})
	}
	cursor = uintptr(unsafe.Pointer(&model[verts]))
	vs := [4][3]int16{{-40, -30, 0}, {40, -30, 0}, {-40, 30, 0}, {40, 30, 0}}
	if p.shape == big {
		vs = [4][3]int16{{-400, -300, 0}, {400, -300, 0}, {-400, 300, 0}, {400, 300, 0}}
	}
	for i := 0; i < p.near; i++ {
		vs[i][2] = p.vz
	}
	for _, v := range vs {
		put(&cursor, TMD_VERT{Vx: v[0], Vy: v[1], Vz: v[2]})
	}
	norms := cursor
	put(&cursor, TMD_NORM{Nz: -4096})
	*objTable = TMD_OBJ{
		VertTop: uint32(uintptr(unsafe.Pointer(&model[verts]))), NVert: 4,
		NormTop: uint32(norms), NNorm: 1,
		PrimTop: uint32(uintptr(unsafe.Pointer(&model[prim]))), NPrim: 1,
	}
}
