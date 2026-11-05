//go:build yaroze

package cleanlib

import (
	"device"
	"unsafe"
)

// The GTE, the geometry coprocessor the 3D functions do their arithmetic
// on, reached as yaroze/gte_yaroze.go reaches it: its data registers by
// name in inline assembly, its commands as words. A control register move
// needs a fixed register in its encoding, so the value goes through $gp,
// which the compiled code does not use, and gets its own value back at once:
// a C program that called the library keeps its small data's base there. Each
// command is preceded by two nops, as the coprocessor needs after a register
// load.

// SetRotMatrix and SetTransMatrix load the GTE's rotation matrix and
// translation from m; ReadFlag returns its FLAG register. The target
// defines them in assembly (playstation-yaroze-gte.S).
//
//export SetRotMatrix
func SetRotMatrix(m *MATRIX)

//export SetTransMatrix
func SetTransMatrix(m *MATRIX)

// setRotation is SetRotMatrix in line, for a caller that should not need a
// frame for the call. The words go through $gp, whose load needs its delay
// slot before the ctc2.
//
//go:inline
func setRotation(m *MATRIX) {
	device.AsmFull(`.set push
	.set noat
	lw $$28, 0({m})
	nop
	.word 0x48dc0000
	lw $$28, 4({m})
	nop
	.word 0x48dc0800
	lw $$28, 8({m})
	nop
	.word 0x48dc1000
	lw $$28, 12({m})
	nop
	.word 0x48dc1800
	lh $$28, 16({m})
	nop
	.word 0x48dc2000
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": readGP(), "m": uintptr(unsafe.Pointer(m))})
}

// holdRotation stores the GTE's rotation matrix in m's words, as setRotation
// loads them.
//
//go:inline
func holdRotation(m *MATRIX) {
	device.AsmFull(`.set push
	.set noat
	.word 0x485c0000
	nop
	sw $$28, 0({m})
	.word 0x485c0800
	nop
	sw $$28, 4({m})
	.word 0x485c1000
	nop
	sw $$28, 8({m})
	.word 0x485c1800
	nop
	sw $$28, 12({m})
	.word 0x485c2000
	nop
	sw $$28, 16({m})
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": readGP(), "m": uintptr(unsafe.Pointer(m))})
}

// setTranslation is SetTransMatrix in line, as setRotation is SetRotMatrix.
//
//go:inline
func setTranslation(m *MATRIX) {
	device.AsmFull(`.set push
	.set noat
	lw $$28, 20({m})
	nop
	.word 0x48dc2800
	lw $$28, 24({m})
	nop
	.word 0x48dc3000
	lw $$28, 28({m})
	nop
	.word 0x48dc3800
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": readGP(), "m": uintptr(unsafe.Pointer(m))})
}

// readGP returns gp, as the library's caller set it.
//
//go:inline
func readGP() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	move {}, $$28
	.set pop`, nil))
}

// setGP puts back what readGP returned.
//
//go:inline
func setGP(gp uint32) {
	device.AsmFull(`.set push
	.set noat
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": gp})
}

// gteOn is set once gteEnable has run.
var gteOn bool

// gteEnable turns the GTE on in the CPU's status register, bit 30.
func gteEnable() {
	gteOn = true
	sr := device.AsmFull(`.set push
	.set noat
	mfc0 {}, $$12
	nop
	.set pop`, nil)
	device.AsmFull(`.set push
	.set noat
	mtc0 {v}, $$12
	nop
	nop
	.set pop`, map[string]interface{}{"v": uint32(sr) | 1<<30})
}

// The control register setters, each a ctc2 from $gp to its register:
// OFX and OFY, the screen offset in 16.16 fixed point; H, the projection
// distance; ZSF3 and ZSF4, the scales AVSZ3 and AVSZ4 average by. The
// offset and the projection are kept beside the GTE for the sorts, which
// place the screen rectangle around the offset and the near drop at half
// the projection (div.go), and in the GsOFSGTE mode add the offset to
// the 2D primitives (sort.go).
var (
	gteOFX, gteOFY int32 // the whole pixels of OFX and OFY
	gteH           int32
)

func setOFX(v int32) {
	gteOFX = v >> 16
	updatePlace()
	device.AsmFull(`.set push
	.set noat
	move $$28, {v}
	.word 0x48dcc000
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": readGP(), "v": v})
}

func setOFY(v int32) {
	gteOFY = v >> 16
	updatePlace()
	device.AsmFull(`.set push
	.set noat
	move $$28, {v}
	.word 0x48dcc800
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": readGP(), "v": v})
}

func setH(v int32) {
	gteH = v
	device.AsmFull(`.set push
	.set noat
	move $$28, {v}
	.word 0x48dcd000
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": readGP(), "v": v})
}

func setZSF3(v int32) {
	device.AsmFull(`.set push
	.set noat
	move $$28, {v}
	.word 0x48dce800
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": readGP(), "v": v})
}

func setZSF4(v int32) {
	device.AsmFull(`.set push
	.set noat
	move $$28, {v}
	.word 0x48dcf000
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": readGP(), "v": v})
}

// rotate multiplies the vector x, y, z by the GTE's rotation matrix with
// MVMVA, without the translation, each element shifted right by 12 and
// saturated to 16 bits, and returns the result, IR1 to IR3.
//
//go:inline
func rotate(x, y, z int16) (int16, int16, int16) {
	mvmva(x, y, z)
	return ir1(), ir2(), ir3()
}

// ir1, ir2 and ir3 return what the last multiply left in IR1 to IR3.
//
//go:inline
func ir1() int16 {
	return int16(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$9
	nop
	.set pop`, nil))
}

//go:inline
func ir2() int16 {
	return int16(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$10
	nop
	.set pop`, nil))
}

//go:inline
func ir3() int16 {
	return int16(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$11
	nop
	.set pop`, nil))
}

// mvmva runs that multiply and leaves its results in the registers: MAC1 to
// MAC3 hold each sum of products shifted right by 12, unsaturated.
//
//go:inline
func mvmva(x, y, z int16) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$0
	mtc2 {b}, $$1
	nop
	nop
	.word 0x4a486012
	.set pop`, map[string]interface{}{"a": uint32(uint16(x)) | uint32(uint16(y))<<16, "b": int32(z)})
}

// mvmvaWhole is mvmva without the shift: MAC1 to MAC3 hold each sum of products,
// wrapping at 32 bits.
//
//go:inline
func mvmvaWhole(x, y, z int16) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$0
	mtc2 {b}, $$1
	nop
	nop
	.word 0x4a406012
	.set pop`, map[string]interface{}{"a": uint32(uint16(x)) | uint32(uint16(y))<<16, "b": int32(z)})
}

// mvmvaAt is mvmva on the vector at v, in memory.
//
//go:inline
func mvmvaAt(v uintptr) {
	device.AsmFull(`.set push
	.set noat
	lwc2 $$0, 0({v})
	lwc2 $$1, 4({v})
	nop
	nop
	.word 0x4a486012
	.set pop`, map[string]interface{}{"v": v})
}

// mac1, mac2 and mac3 return what the last multiply left in MAC1 to MAC3, not
// saturated as rotate's results are.
//
//go:inline
func mac1() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$25
	nop
	.set pop`, nil))
}

//go:inline
func mac2() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$26
	nop
	.set pop`, nil))
}

//go:inline
func mac3() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$27
	nop
	.set pop`, nil))
}

// leadingZeros returns how many bits lead the 32-bit v up to its top bit by the
// GTE's LZCS and LZCR: for a negative v, how many ones.
//
//go:inline
func leadingZeros(v int32) int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mtc2 {v}, $$30
	nop
	nop
	mfc2 {}, $$31
	nop
	.set pop`, map[string]interface{}{"v": v}))
}

// rtpt transforms three vertices to the screen with RTPT, each given as
// the two words the GTE takes, x and y then z, leaving their points in
// SXY0 to SXY2 and depths in SZ1 to SZ3.
//
//go:inline
func rtpt(a, b, c, d, e, f uint32) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$0
	mtc2 {b}, $$1
	mtc2 {c}, $$2
	mtc2 {d}, $$3
	mtc2 {e}, $$4
	mtc2 {f}, $$5
	nop
	nop
	.word 0x4a280030
	.set pop`, map[string]interface{}{"a": a, "b": b, "c": c, "d": d, "e": e, "f": f})
}

// hold and holdWord make the compiler work a value out by here, and not sink it
// past a wait for the GTE, for which it would then have nothing to do.
//
//go:inline
func hold(v uintptr) {
	device.AsmFull(`# {v}`, map[string]interface{}{"v": v})
}

//go:inline
func holdWord(v uint32) {
	device.AsmFull(`# {v}`, map[string]interface{}{"v": v})
}

// rtptAt is rtpt on three vertices in memory, each at its address: loading them
// straight into the GTE leaves out the moves through registers.
//
//go:inline
func rtptAt(a, b, c uintptr) {
	device.AsmFull(`.set push
	.set noat
	lwc2 $$0, 0({a})
	lwc2 $$1, 4({a})
	lwc2 $$2, 0({b})
	lwc2 $$3, 4({b})
	lwc2 $$4, 0({c})
	lwc2 $$5, 4({c})
	nop
	nop
	.word 0x4a280030
	.set pop`, map[string]interface{}{"a": a, "b": b, "c": c})
}

// rtps transforms one more vertex with RTPS, pushing the screen point and
// depth queues.
//
//go:inline
func rtps(a, b uint32) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$0
	mtc2 {b}, $$1
	nop
	nop
	.word 0x4a180001
	.set pop`, map[string]interface{}{"a": a, "b": b})
}

// rtpsAt is rtps on a vertex in memory, at its address.
//
//go:inline
func rtpsAt(a uintptr) {
	device.AsmFull(`.set push
	.set noat
	lwc2 $$0, 0({a})
	lwc2 $$1, 4({a})
	nop
	nop
	.word 0x4a180001
	.set pop`, map[string]interface{}{"a": a})
}

// nclip is NCLIP on the three screen points: positive when they turn
// clockwise on the screen. It takes what the last perspective transformation
// left, so needs no nops for a register load, as avsz3 and avsz4 need none.
//
//go:inline
func nclip() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	.word 0x4b400006
	mfc2 {}, $$24
	nop
	.set pop`, nil))
}

// avsz3 and avsz4 average the last three or four depths, scaled by ZSF3
// or ZSF4, into OTZ and return it.
//
//go:inline
func avsz3() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	.word 0x4b58002d
	mfc2 {}, $$7
	nop
	.set pop`, nil))
}

//go:inline
func avsz4() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	.word 0x4b68002e
	mfc2 {}, $$7
	nop
	.set pop`, nil))
}

// storeSXY stores the screen points SXY0 to SXY2 into the polygon packet
// at p whose corners take s words each, at the first three corners' point
// words: 8, 8+4s and 8+8s bytes in.
//
//go:inline
func storeSXY(p uintptr, s uintptr) {
	switch s {
	case 1:
		device.AsmFull(`.set push
	.set noat
	swc2 $$12, 8({p})
	swc2 $$13, 12({p})
	swc2 $$14, 16({p})
	.set pop`, map[string]interface{}{"p": p})
	case 2:
		device.AsmFull(`.set push
	.set noat
	swc2 $$12, 8({p})
	swc2 $$13, 16({p})
	swc2 $$14, 24({p})
	.set pop`, map[string]interface{}{"p": p})
	default:
		device.AsmFull(`.set push
	.set noat
	swc2 $$12, 8({p})
	swc2 $$13, 20({p})
	swc2 $$14, 32({p})
	.set pop`, map[string]interface{}{"p": p})
	}
}

// storeSXY4 stores the newest screen point, SXY2, at the fourth corner's
// point word of the quad packet at p whose corners take s words each,
// 8+12s bytes in.
//
//go:inline
func storeSXY4(p uintptr, s uintptr) {
	switch s {
	case 1:
		device.AsmFull(`.set push
	.set noat
	swc2 $$14, 20({p})
	.set pop`, map[string]interface{}{"p": p})
	case 2:
		device.AsmFull(`.set push
	.set noat
	swc2 $$14, 32({p})
	.set pop`, map[string]interface{}{"p": p})
	default:
		device.AsmFull(`.set push
	.set noat
	swc2 $$14, 44({p})
	.set pop`, map[string]interface{}{"p": p})
	}
}

// setRGBC loads the color the lighting commands start from, RGBC, whose
// top byte the colors they push carry.
//
//go:inline
func setRGBC(c uint32) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {c}, $$6
	.set pop`, map[string]interface{}{"c": c})
}

// loadV0 and loadV3 load one vector, or three, from their addresses into V0, or
// V0 to V2.
//
//go:inline
func loadV0(a uintptr) {
	device.AsmFull(`.set push
	.set noat
	lwc2 $$0, 0({a})
	lwc2 $$1, 4({a})
	.set pop`, map[string]interface{}{"a": a})
}

//go:inline
func loadV3(a, b, c uintptr) {
	device.AsmFull(`.set push
	.set noat
	lwc2 $$0, 0({a})
	lwc2 $$1, 4({a})
	lwc2 $$2, 0({b})
	lwc2 $$3, 4({b})
	lwc2 $$4, 0({c})
	lwc2 $$5, 4({c})
	.set pop`, map[string]interface{}{"a": a, "b": b, "c": c})
}

// The lighting commands, on the normals in V0 (the single forms) or V0 to
// V2 (the triple forms), pushing a color for each into RGB0 to RGB2: NCCS
// and NCCT light RGBC, NCDS and NCDT light it and fade it into the far
// color by the depth cue IR0, which the last perspective transformation
// left.

//go:inline
func nccs() {
	device.AsmFull(`.set push
	.set noat
	nop
	nop
	.word 0x4b08041b
	.set pop`, nil)
}

//go:inline
func ncct() {
	device.AsmFull(`.set push
	.set noat
	nop
	nop
	.word 0x4b18043f
	.set pop`, nil)
}

//go:inline
func ncds() {
	device.AsmFull(`.set push
	.set noat
	nop
	nop
	.word 0x4ae80413
	.set pop`, nil)
}

//go:inline
func ncdt() {
	device.AsmFull(`.set push
	.set noat
	nop
	nop
	.word 0x4af80416
	.set pop`, nil)
}

// rgb0, rgb1 and rgb2 return the color FIFO's entries, oldest first.
//
//go:inline
func rgb0() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$20
	nop
	.set pop`, nil))
}

//go:inline
func rgb1() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$21
	nop
	.set pop`, nil))
}

//go:inline
func rgb2() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$22
	nop
	.set pop`, nil))
}

// flag returns the GTE's FLAG register, the errors of the last commands,
// through $gp.
//
//go:inline
func flag() uint32 {
	gp := readGP()
	f := uint32(device.AsmFull(`.set push
	.set noat
	.word 0x485cf800
	nop
	move {}, $$28
	.set pop`, nil))
	setGP(gp)
	return f
}

// sxy0, sxy1 and sxy2 return the screen points in SXY0 to SXY2, each y in
// the high half and x in the low.
//
//go:inline
func sxy0() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$12
	nop
	.set pop`, nil))
}

//go:inline
func sxy1() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$13
	nop
	.set pop`, nil))
}

//go:inline
func sxy2() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$14
	nop
	.set pop`, nil))
}
