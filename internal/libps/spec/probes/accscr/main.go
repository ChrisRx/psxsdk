//go:build yaroze

// Command cut times one polygon of each kind under GsSortObject4's cuts.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
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
	name string
	mode, flag uint8
	words []uint32
}

const (
	uv0 = 0x7801_0402
	uv1 = 0x0123_0844
	uv2 = 0x0000_4010
	uv3 = 0x0000_4c50
)

var kinds = []kind{
	{"lf3", 0x20, 0, []uint32{0x20_a08060, 0 << 16, 2<<16 | 1}},
	{"lft3", 0x24, 0, []uint32{uv0, uv1, uv2, 0 << 16, 2<<16 | 1}},
	{"lf4", 0x28, 0, []uint32{0x28_a08060, 0 << 16, 2<<16 | 1, 3}},
	{"lft4", 0x2c, 0, []uint32{uv0, uv1, uv2, uv3, 0 << 16, 2<<16 | 1, 3}},
	{"lg3", 0x30, 0, []uint32{0x30_a08060, 0, 1 << 16, 2 << 16}},
	{"lgt3", 0x34, 0, []uint32{uv0, uv1, uv2, 0, 1 << 16, 2 << 16}},
	{"lg4", 0x38, 0, []uint32{0x38_a08060, 0, 1 << 16, 2 << 16, 3 << 16}},
	{"lgt4", 0x3c, 0, []uint32{uv0, uv1, uv2, uv3, 0, 1 << 16, 2 << 16, 3 << 16}},
	{"f3", 0x21, 1, []uint32{0x21_a08060, 1<<16 | 0, 2}},
	{"ft3", 0x25, 1, []uint32{uv0, uv1, uv2, 0x25_806040, 1<<16 | 0, 2}},
	{"f4", 0x29, 1, []uint32{0x29_a08060, 1<<16 | 0, 3<<16 | 2}},
	{"ft4", 0x2d, 1, []uint32{uv0, uv1, uv2, uv3, 0x2d_806040, 1<<16 | 0, 3<<16 | 2}},
	{"g3", 0x31, 1, []uint32{0x31_102030, 0x405060, 0x708090, 1<<16 | 0, 2}},
	{"gt3", 0x35, 1, []uint32{uv0, uv1, uv2, 0x35_102030, 0x405060, 0x708090, 1<<16 | 0, 2}},
	{"g4", 0x39, 1, []uint32{0x39_102030, 0x405060, 0x708090, 0xa0b0c0, 1<<16 | 0, 3<<16 | 2}},
	{"gt4", 0x3d, 1, []uint32{uv0, uv1, uv2, uv3, 0x3d_102030, 0x405060, 0x708090, 0xa0b0c0, 1<<16 | 0, 3<<16 | 2}},
	{"gr20", 0x20, 4, []uint32{0x20_102030, 0x405060, 0x708090, 0 << 16, 2<<16 | 1}},
	{"gr30", 0x30, 4, []uint32{0x30_102030, 0x405060, 0x708090, 0, 1 << 16, 2 << 16}},
	{"sft3", 0x26, 0, []uint32{uv0, uv1, uv2, 0 << 16, 2<<16 | 1}},
	{"dft3", 0x24, 2, []uint32{uv0, uv1, uv2, 0 << 16, 2<<16 | 1}},
}

type shape struct {
	name    string
	x, y, z int32
	verts   [4][3]int16
}

var flat = [4][3]int16{{-40, -30, 0}, {40, -30, 0}, {-40, 30, 0}, {40, 30, 0}}
var upright = [4][3]int16{{-96, -64, 0}, {112, -48, 8}, {-80, 72, -8}, {128, 80, 16}}
var backf = [4][3]int16{{40, -30, 0}, {-40, -30, 0}, {40, 30, 0}, {-40, 30, 0}}

var shapes = []shape{
	{"on", 0, 0, 1000, upright},
	{"flat", 0, 0, 300, flat},
	{"part", 170, 0, 300, flat},
	{"off", 400, 0, 300, flat},
	{"offy", 0, 400, 300, flat},
	{"offn", -400, -300, 300, flat},
	{"near", 0, 0, 100, flat},
	{"back", 0, 0, 300, backf},
	{"bad", 0, 0, 200, [4][3]int16{{-40, -30, -300}, {40, -30, 0}, {-40, 30, 0}, {40, 30, 0}}},
	{"floor", 0, 0, 0, [4][3]int16{{-512, 100, 512}, {512, 100, 512}, {-512, 100, -512}, {512, 100, -512}}},
}

var (
	buf [4096]uint32
	scr *uint32
	ot      yaroze.GsOT
	tags    [1 << 4]yaroze.GsOT_TAG
	packets [1 << 18]uint32
	model   [256]uint32
	obj     yaroze.GsDOBJ2
	coord   yaroze.GsCOORDINATE2
	view    yaroze.GsVIEW2
	light   = yaroze.GsF_LIGHT{Vx: 0, Vy: 0, Vz: 1, R: 0xff, G: 0xc0, B: 0x80}
	light2  = yaroze.GsF_LIGHT{Vx: 2000, Vy: 0, Vz: 1000, R: 0x40, G: 0xc0, B: 0x80}
)

func put[T any](cursor *uintptr, v T) *T {
	p := (*T)(yaroze.Pointer(*cursor))
	*p = v
	*cursor += unsafe.Sizeof(v)
	return p
}

func build(k *kind, s *shape) {
	cursor := uintptr(unsafe.Pointer(&model[0]))
	put(&cursor, TMD_HDR{ID: 0x41, Flag: 1, NObjs: 1})
	objTable := put(&cursor, TMD_OBJ{})
	prim := cursor
	put(&cursor, uint32(k.mode)<<24|uint32(k.flag)<<16|0x0509)
	for _, w := range k.words {
		put(&cursor, w)
	}
	verts := cursor
	for _, v := range s.verts {
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
		PrimTop: uint32(prim), NPrim: 1,
	}
}

//go:noinline
func hash(from, to uintptr, h uint32) uint32 {
	for p := from; p < to; p += 4 {
		w := *(*uint32)(yaroze.Pointer(p))
		if w>>24 != 0 && w>>24 <= 12 && w&0xffffff >= 0x10000 && w&0xffffff < 0x90000 || w&0xffffff >= 0x10000 && w&0xffffff < 0x90000 && p >= uintptr(unsafe.Pointer(&tags[0])) && p < uintptr(unsafe.Pointer(&tags[0]))+16*4 {
			w &^= 0xffffff
		}
		h = (h ^ w) * 16777619
	}
	return h
}

func hex(v uint32) {
	const d = "0123456789abcdef"
	for i := 28; i >= 0; i -= 4 {
		print(d[v>>uint(i)&15:][:1])
	}
}

// masked is the hash with the top bytes of a color word left out: of everything
// but the link and first color.
//
//go:noinline
func one(k *kind, s *shape, attr uint32, mode int, dump bool) {
	build(k, s)
	view.View = yaroze.IDMATRIX
	view.View.T = [3]int32{s.x, s.y, s.z}
	view.Super = yaroze.WORLD
	yaroze.GsSetView2(&view)
	yaroze.TMD(model[:]).Link(&obj, 0)
	obj.Coord2 = &coord
	obj.Attribute = attr
	yaroze.GsSetLightMode(int32(mode))
	var lw, ls yaroze.MATRIX
	yaroze.GsGetLw(&coord, &lw)
	yaroze.GsSetLightMatrix(&lw)
	yaroze.GsGetLs(&coord, &ls)
	yaroze.GsSetLsMatrix(&ls)
	var h uint32
	for r := 0; r < 3; r++ {
		base := uintptr(unsafe.Pointer(&packets[0]))
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
		yaroze.GsClearOt(0, 0, &ot)
		yaroze.GsSortObject4(&obj, &ot, 10, scr)
		end := uintptr(unsafe.Pointer(yaroze.GsGetWorkBase()))
		if r == 0 {
			h = hash(base, end, 2166136261)
			h = hash(uintptr(unsafe.Pointer(&tags[0])), uintptr(unsafe.Pointer(&tags[0]))+16*4, h)
			print(" n", (end-base)/4, " h")
			hex(h)
			println()
		}
		if dump && (r == 2 || r == 0) {
			print("D ")
			for p := base; p < end; p += 4 {
				hex(*(*uint32)(yaroze.Pointer(p)))
				print(" ")
			}
			println()
		}
	}
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	yaroze.GsSetProjection(300)
	yaroze.GsSetFlatLight(0, &light)
	yaroze.GsSetFlatLight(1, &light2)
	yaroze.GsSetAmbient(1024, 1024, 1024)
	yaroze.GsSetLightMode(0)
	coord.Init(yaroze.WORLD)
	ot.Length = 4
	ot.Org = &tags[0]
	for li := range 3 {
	attr := uint32(0)
	mode := int32(0)
	if li == 1 { mode = 1 }
	if li == 2 { attr = 0x40 }
	yaroze.GsSetLightMode(mode)
	for ki := range kinds {
		if li > 0 && kinds[ki].mode&1 != 0 { continue }
		for _, si := range []int{0, 3, 6, 7} {
		for lv := uint32(1); lv <= 2; lv++ {
			k := &kinds[ki]
			sh := &shapes[si]
			for i := range buf {
				buf[i] = 0xdeadbe00 + uint32(i)
			}
			scr = (*uint32)(unsafe.Pointer(&buf[0]))
			print("C ", li, " ", k.name, " ", sh.name, " ", lv)
			one(k, sh, attr|lv<<9, int(mode), false)
			n := 0
			for i := range buf {
				if buf[i] != 0xdeadbe00+uint32(i) {
					n++
					print(i, ":")
					hex(buf[i])
					print(" ")
				}
			}
			println(" changed ", n)
		}
		}
	}
	}
	println("END")
	for {
		yaroze.VSync(0)
	}
}
