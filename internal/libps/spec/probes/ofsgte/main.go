//go:build yaroze

// Command ofsgte is a probe for the spec: what the sorts put in their
// packets in the GsOFSGTE mode, where the GTE's screen offset positions
// primitives and the drawing environment's offset stays 0, against the
// GsOFSGPU mode. In each mode, for each of three origins and in each
// drawing buffer, one frame sorts an undivided textured quad through
// GsSortObject4 and a sprite, a fast sprite, a box, a line and a gouraud
// line into one ordering table, and the TTY lists the packets in drawing
// order, each word in hex with the tag's address masked, after the GTE's
// OFX and OFY and the active buffer. Then, in the GsOFSGTE mode, a flat
// quad under GsDIV2 is moved out of each edge of the screen in each
// buffer, as spec/probes/div does in the GsOFSGPU mode, and the TTY gives
// the pieces linked per case, to place the off-screen rectangle. GP1
// marker words (10800000h plus the case) go before each sort and before
// each GsDrawOt, so a GPU recording shows what the draw sends. Built for
// the resident library it shows Sony's behavior.
package main

import (
	"device"
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

var (
	ot      yaroze.GsOT
	tags    [1 << 12]yaroze.GsOT_TAG
	packets [1 << 15]uint32
	model   [64]uint32
	obj     yaroze.GsDOBJ2
	coord   yaroze.GsCOORDINATE2
	view    yaroze.GsVIEW2
	light   = yaroze.GsF_LIGHT{Vx: 0, Vy: 0, Vz: 1, R: 0xff, G: 0xc0, B: 0x80}
	gte     bool
	n       int
)

func mark(v int) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10800000|uint32(v))
}

func ofx() int32 {
	return int32(device.AsmFull(".set push\n.set noat\n.word 0x485cc000\nnop\nmove {}, $$28\n.set pop", nil))
}

func ofy() int32 {
	return int32(device.AsmFull(".set push\n.set noat\n.word 0x485cc800\nnop\nmove {}, $$28\n.set pop", nil))
}

func main() {
	yaroze.ResetGraph(0)
	for _, mode := range []uint16{yaroze.GsOFSGPU, yaroze.GsOFSGTE} {
		gte = mode == yaroze.GsOFSGTE
		yaroze.GsInitGraph(320, 240, mode, 0, 0)
		yaroze.GsDefDispBuff(0, 0, 0, 240)
		yaroze.GsInit3D()
		coord.Init(yaroze.WORLD)
		yaroze.GsSetProjection(300)
		yaroze.GsSetFlatLight(0, &light)
		yaroze.GsSetAmbient(1024, 1024, 1024)
		yaroze.GsSetLightMode(0)
		for _, o := range [][2]int32{{160, 120}, {10, 20}, {0, 0}} {
			yaroze.GsSetOrign(o[0], o[1])
			yaroze.GsSetDrawBuffOffset()
			for range 2 {
				frame("sorts origin "+itoa(int(o[0]))+" "+itoa(int(o[1])), func() {
					sort3D(0, 0, 300, 0)
					sort2D()
				}, true)
			}
		}
	}
	// The off-screen rectangle under subdivision in the GsOFSGTE mode: the
	// flat quad's center moved out of each edge, in each buffer. The div
	// probe's sweeps in the GsOFSGPU mode are the comparison.
	yaroze.GsSetOrign(160, 120)
	yaroze.GsSetDrawBuffOffset()
	sweeps()
	yaroze.GsSetOrign(0, 0)
	yaroze.GsSetDrawBuffOffset()
	for range 2 {
		for d := int32(150); d <= 210; d++ {
			frame("div x "+itoa(int(d)), func() { sort3D(d, 0, 300, yaroze.GsDIV2) }, false)
		}
	}
	yaroze.GsSetOrign(100, 50)
	yaroze.GsSetDrawBuffOffset()
	frame("skip", func() {}, false)
	for d := int32(150); d <= 210; d++ {
		frame("div x "+itoa(int(d)), func() { sort3D(d, 0, 300, yaroze.GsDIV2) }, false)
	}
	for d := int32(110); d <= 170; d++ {
		frame("div y "+itoa(int(d)), func() { sort3D(0, d, 300, yaroze.GsDIV2) }, false)
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}

// sweeps moves the quad out of each edge in each buffer.
func sweeps() {
	for range 2 {
		for d := int32(150); d <= 210; d++ {
			frame("div x "+itoa(int(d)), func() { sort3D(d, 0, 300, yaroze.GsDIV2) }, false)
		}
		for d := int32(150); d <= 210; d += 5 {
			frame("div x -"+itoa(int(d)), func() { sort3D(-d, 0, 300, yaroze.GsDIV2) }, false)
		}
		for d := int32(110); d <= 170; d++ {
			frame("div y "+itoa(int(d)), func() { sort3D(0, d, 300, yaroze.GsDIV2) }, false)
		}
		for d := int32(110); d <= 170; d += 5 {
			frame("div y -"+itoa(int(d)), func() { sort3D(0, -d, 300, yaroze.GsDIV2) }, false)
		}
	}
}

// frame runs one case: the table cleared, f sorting into it, the report,
// then the swap and the draw.
func frame(what string, f func(), dump bool) {
	yaroze.DrawSync(0)
	for i := range tags {
		tags[i] = 0
	}
	ot.Length = 12
	ot.Org = &tags[0]
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
	yaroze.GsClearOt(0, 0, &ot)
	m := "gpu"
	if gte {
		m = "gte"
	}
	print("case ", n, " ", m, " buff ", yaroze.GsGetActiveBuff(), " ", what)
	if gte {
		print(" ofs ", ofx()>>16, " ", ofy()>>16)
	}
	println()
	mark(n)
	f()
	report(dump)
	yaroze.VSync(0)
	yaroze.GsSwapDispBuff()
	yaroze.GsSortClear(uint8(n), 0, 0, &ot)
	mark(0x4000 | n)
	yaroze.GsDrawOt(&ot)
	n++
}

// sort3D sorts the flat quad, 80 by 60 at (x, y) in the view, div set in
// its attribute, with shift 2.
func sort3D(x, y, z int32, div uint32) {
	view.View = yaroze.IDMATRIX
	view.View.T = [3]int32{x, y, z}
	view.Super = yaroze.WORLD
	yaroze.GsSetView2(&view)
	build()
	yaroze.TMD(model[:]).Link(&obj, 0)
	obj.Coord2 = &coord
	obj.Attribute = div
	var lw, ls yaroze.MATRIX
	yaroze.GsGetLw(&coord, &lw)
	yaroze.GsSetLightMatrix(&lw)
	yaroze.GsGetLs(&coord, &ls)
	yaroze.GsSetLsMatrix(&ls)
	yaroze.GsSortObject4(&obj, &ot, 2, yaroze.Scratch())
}

// sort2D sorts one of each 2D primitive, each at its own depth so the
// report lists them in a fixed order.
func sort2D() {
	sp := yaroze.GsSPRITE{X: 10, Y: 20, W: 16, H: 8, Tpage: 0x20, U: 4, V: 5, Cx: 320, Cy: 256,
		R: 128, G: 128, B: 128, Mx: 3, My: 2, Scalex: 4096, Scaley: 4096}
	yaroze.GsSortSprite(&sp, &ot, 10)
	sp.Attribute = yaroze.GsROTOFF
	yaroze.GsSortSprite(&sp, &ot, 9)
	sp.Attribute = 0
	yaroze.GsSortFastSprite(&sp, &ot, 8)
	box := yaroze.GsBOXF{X: 30, Y: 40, W: 5, H: 6, R: 1, G: 2, B: 3}
	yaroze.GsSortBoxFill(&box, &ot, 7)
	line := yaroze.GsLINE{X0: 1, Y0: 2, X1: 100, Y1: 50, R: 4, G: 5, B: 6}
	yaroze.GsSortLine(&line, &ot, 6)
	gline := yaroze.GsGLINE{X0: -7, Y0: -8, X1: 200, Y1: 100, R0: 7, G0: 8, B0: 9, R1: 10, G1: 11, B1: 12}
	yaroze.GsSortGLine(&gline, &ot, 5)
}

// inPackets reports whether the address a (its low 24 bits) is in the
// packet area.
func inPackets(a uint32) bool {
	lo := uint32(uintptr(unsafe.Pointer(&packets[0]))) & 0xffffff
	return a >= lo && a < lo+uint32(len(packets))*4
}

// report prints each entry that leads to packets with its count, from the
// last entry down, then every packet when dump is set.
func report(dump bool) {
	line := "  entries:"
	for i := len(tags) - 1; i >= 0; i-- {
		c := 0
		for a := uint32(tags[i]) & 0xffffff; inPackets(a); c++ {
			a = *(*uint32)(yaroze.Pointer(uintptr(0x80000000 | a))) & 0xffffff
		}
		if c > 0 {
			line += " " + itoa(i) + "x" + itoa(c)
		}
	}
	println(line)
	if !dump {
		return
	}
	for i := len(tags) - 1; i >= 0; i-- {
		for a := uint32(tags[i]) & 0xffffff; inPackets(a); {
			w := (*[16]uint32)(yaroze.Pointer(uintptr(0x80000000 | a)))
			line := "  " + hex(w[0])[:2]
			for k := 1; k <= int(w[0]>>24); k++ {
				line += " " + hex(w[k])
			}
			println(line)
			a = w[0] & 0xffffff
		}
	}
}

const hexDigits = "0123456789abcdef"

func hex(w uint32) string {
	var b [8]byte
	for i := 7; i >= 0; i-- {
		b[i] = hexDigits[w&15]
		w >>= 4
	}
	return string(b[:])
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

func put[T any](cursor *uintptr, v T) *T {
	p := (*T)(yaroze.Pointer(*cursor))
	*p = v
	*cursor += unsafe.Sizeof(v)
	return p
}

// build writes a TMD of one object with one flat textured quad (2Ch), 80
// by 60 in the plane z = 0, into model, its offsets already addresses.
func build() {
	cursor := uintptr(unsafe.Pointer(&model[0]))
	put(&cursor, TMD_HDR{ID: 0x41, Flag: 1, NObjs: 1})
	objTable := put(&cursor, TMD_OBJ{})
	prim := cursor
	put(&cursor, PRIM_HDR{Olen: 9, Ilen: 7, Flag: 0, Mode: 0x2c})
	put(&cursor, [7]uint32{0x7801_0402, 0x0123_0844, 0x0000_4010, 0x0000_4c50, 0<<16 | 0, 2<<16 | 1, 3})
	verts := cursor
	for _, v := range [4][3]int16{{-40, -30, 0}, {40, -30, 0}, {-40, 30, 0}, {40, 30, 0}} {
		put(&cursor, TMD_VERT{Vx: v[0], Vy: v[1], Vz: v[2]})
	}
	norms := cursor
	put(&cursor, TMD_NORM{Nz: -4096})
	*objTable = TMD_OBJ{
		VertTop: uint32(verts), NVert: 4,
		NormTop: uint32(norms), NNorm: 1,
		PrimTop: uint32(prim), NPrim: 1,
	}
}
