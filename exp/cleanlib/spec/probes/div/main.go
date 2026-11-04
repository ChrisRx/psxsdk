//go:build yaroze

// Command div is a probe for the spec: one primitive at a time, built in
// memory and sorted with GsSortObject4 under GsDIV1 or GsDIV2, to see how
// the library cuts it into pieces, in what order it links them, which
// depth each gets, and which pieces of a floor running past the camera it
// keeps. Each frame draws one case; the clear color's red is the case's
// number. Built for the resident library it shows Sony's behavior.
package main

import (
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

// Shapes.
const (
	upright = iota // facing the camera, corners at odd places
	floor          // a floor 1024 square, 100 below the eye
	flat           // 80 by 60, facing the camera at 300, so one unit is a pixel
	back           // flat, facing away
)

type probe struct {
	kind, shape int
	div         uint32
	otlen       int // 1 or 12
	x, y, z     int32
}

var cases []probe

func init() {
	add := func(p probe) { cases = append(cases, p) }
	add(probe{kind: g4, div: yaroze.GsDIV1, otlen: 1, z: 1000})
	add(probe{kind: g4, div: yaroze.GsDIV1, otlen: 12, z: 1000})
	add(probe{kind: g4, div: yaroze.GsDIV2, otlen: 1, z: 1000})
	add(probe{kind: f3, div: yaroze.GsDIV1, otlen: 1, z: 1000})
	add(probe{kind: f3, div: yaroze.GsDIV2, otlen: 1, z: 1000})
	add(probe{kind: f3, div: yaroze.GsDIV2, otlen: 12, z: 1000})
	add(probe{kind: ft3, div: yaroze.GsDIV2, otlen: 1, z: 1000})
	add(probe{kind: ft4, div: yaroze.GsDIV1, otlen: 1, z: 1000})
	add(probe{kind: ft4, div: yaroze.GsDIV2, otlen: 1, z: 1000})
	add(probe{kind: ft4, div: 0, otlen: 1, z: 1000})
	for _, div := range []uint32{0, yaroze.GsDIV1, yaroze.GsDIV2} {
		for z := int32(1200); z >= -200; z -= 100 {
			add(probe{kind: ft4, shape: floor, div: div, otlen: 12, z: z})
		}
	}
	// Each edge of the screen, the quad's center going from 150 to 210
	// pixels out: its far half leaves the screen first, then all of it.
	for d := int32(150); d <= 210; d++ {
		add(probe{kind: ft4, shape: flat, div: yaroze.GsDIV2, otlen: 1, x: d, z: 300})
	}
	for d := int32(150); d <= 210; d++ {
		add(probe{kind: ft4, shape: flat, div: yaroze.GsDIV2, otlen: 1, x: -d, z: 300})
	}
	for d := int32(110); d <= 170; d++ {
		add(probe{kind: ft4, shape: flat, div: yaroze.GsDIV2, otlen: 1, y: d, z: 300})
	}
	for d := int32(110); d <= 170; d++ {
		add(probe{kind: ft4, shape: flat, div: yaroze.GsDIV2, otlen: 1, y: -d, z: 300})
	}
	for d := int32(150); d <= 210; d++ {
		add(probe{kind: ft4, shape: flat, div: yaroze.GsDIV1, otlen: 1, x: d, z: 300})
	}
	for _, div := range []uint32{0, yaroze.GsDIV1, yaroze.GsDIV2} {
		add(probe{kind: ft4, shape: back, div: div, otlen: 1, z: 300})
		add(probe{kind: f3, shape: back, div: div, otlen: 1, z: 300})
	}
}

var (
	ot      [2]yaroze.GsOT
	tags    [2][1 << 12]yaroze.GsOT_TAG
	packets [2][16384]uint32
	model   [256]uint32
	obj     yaroze.GsDOBJ2
	coord   yaroze.GsCOORDINATE2
	view    yaroze.GsVIEW2
	light   = yaroze.GsF_LIGHT{Vx: 0, Vy: 0, Vz: 1, R: 0xff, G: 0xc0, B: 0x80}
)

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
	for frame := 0; ; frame++ {
		p := cases[frame%len(cases)]
		b := yaroze.GsGetActiveBuff()
		ot[b].Length = uint32(p.otlen)
		ot[b].Org = &tags[b][0]
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[b][0]))
		yaroze.GsClearOt(0, 0, &ot[b])

		view.View = yaroze.IDMATRIX
		view.View.T = [3]int32{p.x, p.y, p.z}
		view.Super = yaroze.WORLD
		yaroze.GsSetView2(&view)

		build(p)
		yaroze.TMD(model[:]).Link(&obj, 0)
		obj.Coord2 = &coord
		obj.Attribute = p.div
		var lw, ls yaroze.MATRIX
		yaroze.GsGetLw(&coord, &lw)
		yaroze.GsSetLightMatrix(&lw)
		yaroze.GsGetLs(&coord, &ls)
		yaroze.GsSetLsMatrix(&ls)
		yaroze.GsSortObject4(&obj, &ot[b], 14-int32(p.otlen), yaroze.Scratch())
		slots(frame%len(cases), tags[b][:1<<p.otlen])

		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(uint8(frame%len(cases)), 0, 0, &ot[b])
		yaroze.GsDrawOt(&ot[b])
	}
}

// slots prints, for case c, each ordering table entry that holds packets
// and how many, from the last entry (drawn first) down.
func slots(c int, tags []yaroze.GsOT_TAG) {
	lo := uint32(uintptr(unsafe.Pointer(&tags[0]))) & 0xffffff
	hi := lo + uint32(len(tags))*4
	print("case ", c, ":")
	for i := len(tags) - 1; i >= 0; i-- {
		n := 0
		for a := uint32(tags[i]) & 0xffffff; (a < lo || a >= hi) && a != 0xffffff; n++ {
			a = *(*uint32)(yaroze.Pointer(uintptr(0x80000000 | a))) & 0xffffff
		}
		if n > 0 {
			print(" ", i, "x", n)
		}
	}
	println()
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
	case floor:
		for _, v := range [4][3]int16{{-512, 100, 512}, {512, 100, 512}, {-512, 100, -512}, {512, 100, -512}} {
			put(&cursor, TMD_VERT{Vx: v[0], Vy: v[1], Vz: v[2]})
		}
	case flat:
		for _, v := range [4][3]int16{{-40, -30, 0}, {40, -30, 0}, {-40, 30, 0}, {40, 30, 0}} {
			put(&cursor, TMD_VERT{Vx: v[0], Vy: v[1], Vz: v[2]})
		}
	case back:
		for _, v := range [4][3]int16{{40, -30, 0}, {-40, -30, 0}, {40, 30, 0}, {-40, 30, 0}} {
			put(&cursor, TMD_VERT{Vx: v[0], Vy: v[1], Vz: v[2]})
		}
	default:
		for _, v := range [4][3]int16{{-96, -64, 0}, {112, -48, 8}, {-80, 72, -8}, {128, 80, 16}} {
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
