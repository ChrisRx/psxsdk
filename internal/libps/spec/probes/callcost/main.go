//go:build yaroze

// Command callcost times the calls showcase-gasgar spends its frame in, on a
// console or the emulator, under either library: GsSortObject4 on models of
// 200 polygons of several kinds, GsSortSprite rotated and not, and the matrix
// calls around them. Each is the least of five runs, in eighths of the system
// clock from timer 2 in mode 200h, an empty call's time taken off. It checks
// whether the clean library's calls cost on a console what they cost in the
// emulator, where their times were matched to Sony's.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timerValue = 0x1f801120
	timerMode  = 0x1f801124
)

type (
	TMD_HDR struct{ ID, Flag, NObjs uint32 }
	TMD_OBJ struct {
		VertTop, NVert, NormTop, NNorm, PrimTop, NPrim uint32
		Scale                                          int32
	}
	TMD_VERT struct{ Vx, Vy, Vz, Pad int16 }
)

type kind struct {
	name       string
	mode, flag uint8
	words      []uint32
}

const (
	uv0 = 0x7801_0402
	uv1 = 0x0123_0844
	uv2 = 0x0000_4010
	uv3 = 0x0000_4c50
)

var kinds = []kind{
	{"lf3", 0x20, 0, []uint32{0x20_a08060, 0 << 16, 2<<16 | 1}},
	{"lf4", 0x28, 0, []uint32{0x28_a08060, 0 << 16, 2<<16 | 1, 3}},
	{"lg4", 0x38, 0, []uint32{0x38_a08060, 0, 1 << 16, 2 << 16, 3 << 16}},
	{"lft4", 0x2c, 0, []uint32{uv0, uv1, uv2, uv3, 0 << 16, 2<<16 | 1, 3}},
	{"f4", 0x29, 1, []uint32{0x29_a08060, 1<<16 | 0, 3<<16 | 2}},
	{"g4", 0x39, 1, []uint32{0x39_102030, 0x405060, 0x708090, 0xa0b0c0, 1<<16 | 0, 3<<16 | 2}},
	{"gt4", 0x3d, 1, []uint32{uv0, uv1, uv2, uv3, 0x3d_102030, 0x405060, 0x708090, 0xa0b0c0, 1<<16 | 0, 3<<16 | 2}},
}

const nPoly = 200

var upright = [4][3]int16{{-96, -64, 0}, {112, -48, 8}, {-80, 72, -8}, {128, 80, 16}}

var (
	ot      yaroze.GsOT
	tags    [1 << 8]yaroze.GsOT_TAG
	packets [1 << 16]uint32
	model   [nPoly*12 + 64]uint32
	obj     yaroze.GsDOBJ2
	coord   yaroze.GsCOORDINATE2
	view    yaroze.GsVIEW2
	light   = yaroze.GsF_LIGHT{Vx: 0, Vy: 0, Vz: 1, R: 0xff, G: 0xc0, B: 0x80}
	sprite  yaroze.GsSPRITE
	lw, ls  yaroze.MATRIX
	m       yaroze.MATRIX
	rot     = yaroze.SVECTOR{Vx: 300, Vy: 500, Vz: 700}
	move    = yaroze.VECTOR{Vx: 100, Vy: 200, Vz: 300}
	base    uint32
)

func put[T any](cursor *uintptr, v T) *T {
	p := (*T)(yaroze.Pointer(*cursor))
	*p = v
	*cursor += unsafe.Sizeof(v)
	return p
}

func build(k *kind) {
	cursor := uintptr(unsafe.Pointer(&model[0]))
	put(&cursor, TMD_HDR{ID: 0x41, Flag: 1, NObjs: 1})
	objTable := put(&cursor, TMD_OBJ{})
	prim := cursor
	for range nPoly {
		put(&cursor, uint32(k.mode)<<24|uint32(k.flag)<<16|0x0509)
		for _, w := range k.words {
			put(&cursor, w)
		}
	}
	verts := cursor
	for _, v := range upright {
		put(&cursor, TMD_VERT{Vx: v[0], Vy: v[1], Vz: v[2]})
	}
	norms := cursor
	put(&cursor, TMD_VERT{Vz: -4096})
	put(&cursor, TMD_VERT{Vx: 2896, Vz: -2896})
	put(&cursor, TMD_VERT{Vy: 2896, Vz: -2896})
	put(&cursor, TMD_VERT{Vx: -2896, Vz: -2896})
	*objTable = TMD_OBJ{
		VertTop: uint32(verts), NVert: 4,
		NormTop: uint32(norms), NNorm: 4,
		PrimTop: uint32(prim), NPrim: nPoly,
	}
}

//go:noinline
func empty() {}

func ticks() uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(timerValue))) & 0xffff }

func cost(prep, call func()) uint32 {
	best := uint32(0xffffffff)
	for range 5 {
		prep()
		volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), 0x200)
		t0 := ticks()
		call()
		t1 := ticks()
		if d := (t1 - t0) & 0xffff; d < best {
			best = d
		}
	}
	return best - base
}

func resetPackets() {
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
	yaroze.GsClearOt(0, 0, &ot)
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.GsInit3D()
	yaroze.GsSetProjection(300)
	yaroze.GsSetFlatLight(0, &light)
	yaroze.GsSetAmbient(1024, 1024, 1024)
	ot.Length = 8
	ot.Org = &tags[0]
	base = 0
	base = cost(func() {}, empty)
	println("empty", base)

	view.View = yaroze.IDMATRIX
	view.View.T = [3]int32{0, 0, 1000}
	view.Super = yaroze.WORLD
	yaroze.GsSetView2(&view)
	yaroze.GsInitCoordinate2(yaroze.WORLD, &coord)
	for i := range kinds {
		k := &kinds[i]
		build(k)
		yaroze.TMD(model[:]).Link(&obj, 0)
		obj.Coord2 = &coord
		obj.Attribute = 0
		yaroze.GsGetLw(&coord, &lw)
		yaroze.GsSetLightMatrix(&lw)
		yaroze.GsGetLs(&coord, &ls)
		yaroze.GsSetLsMatrix(&ls)
		c := cost(resetPackets, func() { yaroze.GsSortObject4(&obj, &ot, 8, yaroze.Scratch()) })
		println("COST GsSortObject4", k.name, nPoly, c)
	}

	sprite = yaroze.GsSPRITE{Attribute: 1 << 24, X: 160, Y: 120, W: 32, H: 32, Tpage: 8, R: 128, G: 128, B: 128, Mx: 16, My: 16, Scalex: 4096, Scaley: 4096, Rotate: 45 * 4096}
	println("COST GsSortSprite rotated", cost(resetPackets, func() { yaroze.GsSortSprite(&sprite, &ot, 0) }))
	sprite.Attribute |= 1 << 27
	println("COST GsSortSprite plain", cost(resetPackets, func() { yaroze.GsSortSprite(&sprite, &ot, 0) }))
	none := func() {}
	println("COST RotMatrix", cost(none, func() { yaroze.RotMatrix(&rot, &m) }))
	println("COST TransMatrix", cost(none, func() { yaroze.TransMatrix(&m, &move) }))
	println("COST GsGetLws", cost(none, func() { yaroze.GsGetLws(&coord, &lw, &ls) }))
	println("COST GsSetLightMatrix", cost(none, func() { yaroze.GsSetLightMatrix(&lw) }))
	println("COST GsSetLsMatrix", cost(none, func() { yaroze.GsSetLsMatrix(&ls) }))
	println("COST GsSortClear", cost(resetPackets, func() { yaroze.GsSortClear(0, 0, 0, &ot) }))
	println("END")
}
