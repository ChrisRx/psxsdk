//go:build yaroze

// Command gsstack is a probe for spec 19: the stack the graphics and matrix calls use
// on a program whose stack is in the scratchpad, the method of gschain: the stack
// below sp is filled with a pattern, the call runs, and the line gives how far down
// the pattern was changed, measured from the probe's sp, so a line holds the probe's
// own small frame, shown by the "empty" line. GsSortObject4 is also asked for how many
// bytes of its scratch argument it changes.
package main

import (
	"device"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const span = 4096

//go:extern PSDCNT
var psdCNT uint32

// The TMD's structures, from the File Formats manual.
type (
	tmdHeader struct{ ID, Flag, NObjs uint32 }
	tmdObject struct {
		VertTop, NVert, NormTop, NNorm, PrimTop, NPrim uint32
		Scale                                          int32
	}
	primHeader struct{ Olen, Ilen, Flag, Mode uint8 }
	tmdVertex  struct{ Vx, Vy, Vz, Pad int16 }
	tmdNormal  struct{ Nx, Ny, Nz, Pad int16 }
)

var (
	prep  func()
	call  func()
	ot    yaroze.GsOT
	tags  [1 << 12]yaroze.GsOT_TAG
	pkts  [1 << 16]uint32
	model [64]uint32
	work  [512]uint32
	obj   yaroze.GsDOBJ2
	coord yaroze.GsCOORDINATE2
	view  yaroze.GsVIEW2
	rview yaroze.GsRVIEW2
	links [16]yaroze.GsCOORDINATE2
	lw    yaroze.MATRIX
	ls    yaroze.MATRIX
	m0    yaroze.MATRIX
	m1    yaroze.MATRIX
	m2    yaroze.MATRIX
	sv    yaroze.SVECTOR
	sv2   yaroze.SVECTOR
	lv    yaroze.VECTOR
	lv2   yaroze.VECTOR
	sprt  yaroze.GsSPRITE
	box   yaroze.GsBOXF
	gline yaroze.GsLINE
	light = yaroze.GsF_LIGHT{Vx: 100, Vy: 200, Vz: 300, R: 0xff, G: 0xc0, B: 0x80}
)

//go:noinline
func reach() int32 {
	sp := uint32(device.AsmFull(".set push\n.set noat\nmove {}, $$29\n.set pop", nil))
	low := sp - span
	most := int32(0)
	for _, pat := range [...]uint32{0x5aa5c33c, 0xa55a3cc3} {
		if prep != nil {
			prep()
		}
		for a := low; a < sp-8; a += 4 {
			*(*uint32)(unsafe.Pointer(uintptr(a))) = pat
		}
		call()
		a := low
		for a < sp-8 && *(*uint32)(unsafe.Pointer(uintptr(a))) == pat {
			a += 4
		}
		if d := int32(sp - a); d > most {
			most = d
		}
	}
	return most
}

func run(name string, f func()) {
	call = f
	println("stack", name, reach())
	prep = nil
}

// Primitive kinds.
const (
	g4 = iota
	f3
	ft3
	ft4
	g3
)

func put[T any](cursor *uintptr, v T) {
	p := (*T)(yaroze.Pointer(*cursor))
	*p = v
	*cursor += unsafe.Sizeof(v)
}

func build(kind int, floor bool) {
	cursor := uintptr(unsafe.Pointer(&model[0]))
	put(&cursor, tmdHeader{ID: 0x41, Flag: 1, NObjs: 1})
	objTable := (*tmdObject)(yaroze.Pointer(cursor))
	put(&cursor, tmdObject{})
	prim := cursor
	switch kind {
	case g4:
		put(&cursor, primHeader{Olen: 8, Ilen: 6, Flag: 1, Mode: 0x39})
		put(&cursor, [6]uint32{0x39_102030, 0x405060, 0x708090, 0xa0b0c0, 1<<16 | 0, 3<<16 | 2})
	case f3:
		put(&cursor, primHeader{Olen: 4, Ilen: 3, Flag: 0, Mode: 0x20})
		put(&cursor, [3]uint32{0x20_a08060, 0<<16 | 0, 2<<16 | 1})
	case ft3:
		put(&cursor, primHeader{Olen: 7, Ilen: 5, Flag: 0, Mode: 0x24})
		put(&cursor, [5]uint32{0x7801_0402, 0x0123_0844, 0x0000_4010, 0<<16 | 0, 2<<16 | 1})
	case ft4:
		put(&cursor, primHeader{Olen: 9, Ilen: 7, Flag: 0, Mode: 0x2c})
		put(&cursor, [7]uint32{0x7801_0402, 0x0123_0844, 0x0000_4010, 0x0000_4c50, 0<<16 | 0, 2<<16 | 1, 3})
	case g3:
		put(&cursor, primHeader{Olen: 6, Ilen: 5, Flag: 1, Mode: 0x31})
		put(&cursor, [5]uint32{0x31_102030, 0x405060, 0x708090, 1<<16 | 0, 2})
	}
	verts := cursor
	shape := [4][3]int16{{-96, -64, 0}, {112, -48, 8}, {-80, 72, -8}, {128, 80, 16}}
	if floor {
		shape = [4][3]int16{{-512, 100, 512}, {512, 100, 512}, {-512, 100, -512}, {512, 100, -512}}
	}
	for _, v := range shape {
		put(&cursor, tmdVertex{Vx: v[0], Vy: v[1], Vz: v[2]})
	}
	norms := cursor
	put(&cursor, tmdNormal{Nz: -4096})
	*objTable = tmdObject{
		VertTop: uint32(verts), NVert: 4,
		NormTop: uint32(norms), NNorm: 1,
		PrimTop: uint32(prim), NPrim: 1,
	}
}

func fresh() {
	yaroze.DrawSync(0)
	yaroze.GsSetWorkBase(yaroze.Packets(&pkts[0]))
	yaroze.GsClearOt(0, 0, &ot)
}

// sorted makes a case of GsSortObject4.
func sorted(name string, kind int, floor bool, attr uint32, z int32) {
	view.View = yaroze.IDMATRIX
	view.View.T = [3]int32{0, 0, z}
	view.Super = yaroze.WORLD
	yaroze.GsSetView2(&view)
	build(kind, floor)
	yaroze.TMD(model[:]).Link(&obj, 0)
	obj.Coord2 = &coord
	obj.Attribute = attr
	yaroze.GsGetLw(&coord, &lw)
	yaroze.GsSetLightMatrix(&lw)
	yaroze.GsGetLs(&coord, &ls)
	yaroze.GsSetLsMatrix(&ls)
	prep = func() {
		fresh()
		for i := range work {
			work[i] = 0xdeadbeef
		}
	}
	call = func() { yaroze.GsSortObject4(&obj, &ot, 2, &work[0]) }
	d := reach()
	last := -1
	for i := range work {
		if work[i] != 0xdeadbeef {
			last = i
		}
	}
	prep = nil
	println("stack", name, d, "scratch bytes", (last+1)*4)
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	ot.Length = 12
	ot.Org = &tags[0]
	coord.Init(yaroze.WORLD)
	yaroze.GsSetProjection(300)
	yaroze.GsSetFlatLight(0, &light)
	yaroze.GsSetAmbient(1024, 1024, 1024)
	yaroze.GsSetLightMode(0)
	fresh()

	run("empty", func() {})

	// Sorting: every kind, flat and past the camera, at each subdivision.
	kinds := [...]struct {
		name string
		kind int
	}{{"g4", g4}, {"f3", f3}, {"ft3", ft3}, {"ft4", ft4}, {"g3", g3}}
	divs := [...]uint32{0, yaroze.GsDIV1, yaroze.GsDIV2, yaroze.GsDIV3, yaroze.GsDIV4, yaroze.GsDIV5}
	for _, k := range kinds {
		for d, div := range divs {
			sorted("GsSortObject4 "+k.name+" div "+digit(d), k.kind, false, div, 1000)
		}
		sorted("GsSortObject4 "+k.name+" floor div 0", k.kind, true, 0, 0)
		sorted("GsSortObject4 "+k.name+" floor div 2", k.kind, true, yaroze.GsDIV2, 0)
	}
	yaroze.GsSetLightMode(1)
	sorted("GsSortObject4 ft4 fog div 0", ft4, false, 0, 1000)
	sorted("GsSortObject4 ft4 fog div 2", ft4, false, yaroze.GsDIV2, 1000)
	yaroze.GsSetLightMode(0)

	// Whether sorting looks at the object's coordinate system.
	sorted("GsSortObject4 ft4 again", ft4, false, 0, 1000)
	coord.Flg = 0
	coord.Workm.T = [3]int32{-5, -5, -5}
	fresh()
	yaroze.GsSortObject4(&obj, &ot, 2, &work[0])
	println("after sort, Flg", coord.Flg, "Workm", coord.Workm.T[0])
	yaroze.DrawSync(0)

	sprt = yaroze.GsSPRITE{X: 100, Y: 100, W: 64, H: 64, R: 128, G: 128, B: 128, Mx: 32, My: 32, Scalex: 4096, Scaley: 4096}
	prep = fresh
	run("GsSortSprite turned", func() { sprt.Rotate = 45 * 4096; yaroze.GsSortSprite(&sprt, &ot, 1) })
	sprt.Attribute = yaroze.GsROTOFF
	prep = fresh
	run("GsSortSprite ROTOFF", func() { yaroze.GsSortSprite(&sprt, &ot, 1) })
	sprt.Attribute = 0
	prep = fresh
	run("GsSortFastSprite", func() { yaroze.GsSortFastSprite(&sprt, &ot, 1) })
	box = yaroze.GsBOXF{X: 10, Y: 10, W: 50, H: 50, R: 10, G: 20, B: 30}
	prep = fresh
	run("GsSortBoxFill", func() { yaroze.GsSortBoxFill(&box, &ot, 1) })
	gline = yaroze.GsLINE{X0: 1, Y0: 2, X1: 100, Y1: 200, R: 1, G: 2, B: 3}
	prep = fresh
	run("GsSortLine", func() { yaroze.GsSortLine(&gline, &ot, 1) })
	run("GsClearOt", func() { yaroze.GsClearOt(0, 0, &ot) })
	prep = fresh
	run("GsDrawOt", func() { yaroze.GsDrawOt(&ot) })
	yaroze.DrawSync(0)
	prep = func() {
		fresh()
		yaroze.GsSortBoxFill(&box, &ot, 1)
		yaroze.GsDrawOt(&ot)
	}
	run("GsDrawOt behind a draw", func() { yaroze.GsDrawOt(&ot) })
	yaroze.DrawSync(0)
	run("GsSwapDispBuff", func() { yaroze.GsSwapDispBuff() })
	run("GsSetFlatLight", func() { yaroze.GsSetFlatLight(1, &light) })
	run("GsSetAmbient", func() { yaroze.GsSetAmbient(100, 200, 300) })
	run("GsSetLightMode", func() { yaroze.GsSetLightMode(0) })
	m0 = yaroze.IDMATRIX
	m1 = yaroze.IDMATRIX
	run("GsSetLsMatrix", func() { yaroze.GsSetLsMatrix(&m0) })
	run("GsSetLightMatrix", func() { yaroze.GsSetLightMatrix(&m0) })
	sv = yaroze.SVECTOR{Vx: 100, Vy: 200, Vz: 300}
	run("RotMatrix", func() { yaroze.RotMatrix(&sv, &m0) })
	lv = yaroze.VECTOR{Vx: 100, Vy: 200, Vz: 300}
	run("TransMatrix", func() { yaroze.TransMatrix(&m0, &lv) })
	lv = yaroze.VECTOR{Vx: 4096, Vy: 4000, Vz: 5000}
	run("ScaleMatrix", func() { yaroze.ScaleMatrix(&m0, &lv) })
	run("MulMatrix0", func() { yaroze.MulMatrix0(&m0, &m1, &m2) })
	run("ApplyMatrix", func() { yaroze.ApplyMatrix(&m0, &sv, &lv2) })
	run("ApplyMatrixLV", func() { yaroze.ApplyMatrixLV(&m0, &lv, &lv2) })
	run("CompMatrix", func() { yaroze.CompMatrix(&m0, &m1, &m2) })
	run("GsInitCoordinate2", func() { yaroze.GsInitCoordinate2(nil, &coord) })
	coord.Flg = 0
	for _, d := range [...]int{1, 2, 4, 16} {
		var up *yaroze.GsCOORDINATE2
		for i := 0; i < d; i++ {
			yaroze.GsInitCoordinate2(up, &links[i])
			links[i].Coord.T = [3]int32{1, 2, 3}
			up = &links[i]
		}
		end := up
		view.View = yaroze.IDMATRIX
		view.Super = end
		prep = func() {
			for i := 0; i < d; i++ {
				links[i].Flg = 0
			}
		}
		run("GsSetView2 depth "+number(d), func() { yaroze.GsSetView2(&view) })
		rview = yaroze.GsRVIEW2{Vpx: 100, Vpy: 200, Vpz: -1000, Vrx: 10, Vry: 20, Vrz: 30, Super: end}
		prep = func() {
			for i := 0; i < d; i++ {
				links[i].Flg = 0
			}
		}
		run("GsSetRefView2 depth "+number(d), func() { yaroze.GsSetRefView2(&rview) })
	}
	// The chain calls proper, after GsInitGraph: PSDCNT is not 0, so a flag of 0 is
	// worked out. Every flag 0, then C, then x.
	for _, d := range [...]int{1, 2, 3, 4, 8, 16} {
		var up *yaroze.GsCOORDINATE2
		for i := 0; i < d; i++ {
			yaroze.GsInitCoordinate2(up, &links[i])
			links[i].Coord.T = [3]int32{1, 2, 3}
			up = &links[i]
		}
		end := up
		for _, f := range [...]struct {
			name string
			flag func() uint32
		}{
			{"0", func() uint32 { return 0 }},
			{"C", func() uint32 { return psdCNT }},
			{"x", func() uint32 { return psdCNT + 1 }},
		} {
			flags := func() {
				for i := 0; i < d; i++ {
					links[i].Flg = f.flag()
				}
			}
			prep = flags
			run("GsGetLw depth "+number(d)+" flags "+f.name, func() { yaroze.GsGetLw(end, &lw) })
			prep = flags
			run("GsGetLs depth "+number(d)+" flags "+f.name, func() { yaroze.GsGetLs(end, &ls) })
			prep = flags
			run("GsGetLws depth "+number(d)+" flags "+f.name, func() { yaroze.GsGetLws(end, &lw, &ls) })
		}
	}
	view.Super = nil
	run("GsSetView2 world", func() { yaroze.GsSetView2(&view) })
	rview.Super = nil
	run("GsSetRefView2 world", func() { yaroze.GsSetRefView2(&rview) })
	run("VSync(0)", func() { yaroze.VSync(0) })
	run("VSync(1)", func() { yaroze.VSync(1) })
	run("VSync(-1)", func() { yaroze.VSync(-1) })
	run("DrawSync(0)", func() { yaroze.DrawSync(0) })
	run("GetPadBuf", func() {
		var a, b *uint8
		yaroze.GetPadBuf(&a, &b)
	})
	// What interrupts alone take: a spin of over a frame with nothing called.
	run("spin", func() {
		for i := uint32(0); i < 200000; i++ {
			volatile.LoadUint32((*uint32)(yaroze.Pointer(0x1f801814)))
		}
	})
	println("END")
	for {
	}
}

func digit(d int) string { return "0123456789"[d : d+1] }

func number(d int) string {
	if d < 10 {
		return digit(d)
	}
	return digit(d/10) + digit(d%10)
}
