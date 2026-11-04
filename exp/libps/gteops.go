//go:build yaroze

package libps

import (
	"device"
	"unsafe"
)

// The GTE's data registers and commands, as inline assembly in the
// pattern of the yaroze package's gte_yaroze.go: a load puts words in by
// mtc2, a command is written as its word, and a result is read by mfc2
// with a nop after it. The control registers, which take ctc2, are moved
// by the functions in targets/playstation-yaroze-libps-gte.S, since the
// assembler takes no ctc2 and a .word for one needs a fixed register.
//
// A vector is loaded from memory as its two words: x and y, then z. A
// block given a register says .set noat, since the compiler may give it
// $at, the register the assembler keeps for its own expansions.

// gteSetV0 loads the vector at p into V0, straight from memory by lwc2.
//
//go:inline
func gteSetV0(p unsafe.Pointer) {
	device.AsmFull(`.set push
	.set noat
	lwc2 $$0, 0({a})
	lwc2 $$1, 4({a})
	.set pop`, map[string]interface{}{"a": uintptr(p)})
}

// gteSetV0Words loads V0 from its two words.
//
//go:inline
func gteSetV0Words(a, b uint32) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$0
	mtc2 {b}, $$1
	.set pop`, map[string]interface{}{"a": a, "b": b})
}

// gteSetV3 loads the vectors at p0, p1 and p2 into V0, V1 and V2,
// straight from memory by lwc2.
//
//go:inline
func gteSetV3(p0, p1, p2 unsafe.Pointer) {
	device.AsmFull(`.set push
	.set noat
	lwc2 $$0, 0({a})
	lwc2 $$1, 4({a})
	lwc2 $$2, 0({b})
	lwc2 $$3, 4({b})
	lwc2 $$4, 0({c})
	lwc2 $$5, 4({c})
	.set pop`, map[string]interface{}{"a": uintptr(p0), "b": uintptr(p1), "c": uintptr(p2)})
}

// The screen points and colors go from the GTE straight into a packet by
// swc2, at the words of the packet's layout: a vertex every one, two or
// three words, its screen point from word 2 and its color from word 1.

// gteSXYEvery1 stores the last three screen points at words 2, 3 and 4
// of the packet at p.
//
//go:inline
func gteSXYEvery1(p *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$12, 8({p})
	swc2 $$13, 12({p})
	swc2 $$14, 16({p})
	.set pop`, map[string]interface{}{"p": uintptr(unsafe.Pointer(p))})
}

// gteSXYEvery2 stores them at words 2, 4 and 6.
//
//go:inline
func gteSXYEvery2(p *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$12, 8({p})
	swc2 $$13, 16({p})
	swc2 $$14, 24({p})
	.set pop`, map[string]interface{}{"p": uintptr(unsafe.Pointer(p))})
}

// gteSXYEvery3 stores them at words 2, 5 and 8.
//
//go:inline
func gteSXYEvery3(p *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$12, 8({p})
	swc2 $$13, 20({p})
	swc2 $$14, 32({p})
	.set pop`, map[string]interface{}{"p": uintptr(unsafe.Pointer(p))})
}

// gteSXY2At stores the last screen point at p.
//
//go:inline
func gteSXY2At(p *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$14, 0({p})
	.set pop`, map[string]interface{}{"p": uintptr(unsafe.Pointer(p))})
}

// gteRGBEvery2 stores the three colors lit at words 1, 3 and 5 of the
// packet at p.
//
//go:inline
func gteRGBEvery2(p *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$20, 4({p})
	swc2 $$21, 12({p})
	swc2 $$22, 20({p})
	.set pop`, map[string]interface{}{"p": uintptr(unsafe.Pointer(p))})
}

// gteRGBEvery3 stores them at words 1, 4 and 7.
//
//go:inline
func gteRGBEvery3(p *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$20, 4({p})
	swc2 $$21, 16({p})
	swc2 $$22, 28({p})
	.set pop`, map[string]interface{}{"p": uintptr(unsafe.Pointer(p))})
}

// gteRGB2At stores the last color lit at p.
//
//go:inline
func gteRGB2At(p *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$22, 0({p})
	.set pop`, map[string]interface{}{"p": uintptr(unsafe.Pointer(p))})
}

// The subdivision keeps results in its corners and midpoints, each at
// its own address.

// gteSXYTo stores the last three screen points at a, b and c.
//
//go:inline
func gteSXYTo(a, b, c *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$12, 0({a})
	swc2 $$13, 0({b})
	swc2 $$14, 0({c})
	.set pop`, map[string]interface{}{"a": uintptr(unsafe.Pointer(a)), "b": uintptr(unsafe.Pointer(b)), "c": uintptr(unsafe.Pointer(c))})
}

// gteSXYTo2 stores the first two of the last three screen points at a
// and b.
//
//go:inline
func gteSXYTo2(a, b *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$12, 0({a})
	swc2 $$13, 0({b})
	.set pop`, map[string]interface{}{"a": uintptr(unsafe.Pointer(a)), "b": uintptr(unsafe.Pointer(b))})
}

// gteSZTo stores the depths of the last three projections at a, b and c.
//
//go:inline
func gteSZTo(a, b, c *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$17, 0({a})
	swc2 $$18, 0({b})
	swc2 $$19, 0({c})
	.set pop`, map[string]interface{}{"a": uintptr(unsafe.Pointer(a)), "b": uintptr(unsafe.Pointer(b)), "c": uintptr(unsafe.Pointer(c))})
}

// gteSZTo2 stores the depths of the first two of the last three
// projections at a and b.
//
//go:inline
func gteSZTo2(a, b *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$17, 0({a})
	swc2 $$18, 0({b})
	.set pop`, map[string]interface{}{"a": uintptr(unsafe.Pointer(a)), "b": uintptr(unsafe.Pointer(b))})
}

// gteSZTo4 stores the depths of the last four projections, oldest
// first, at a, b, c and e.
//
//go:inline
func gteSZTo4(a, b, c, e *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$16, 0({a})
	swc2 $$17, 0({b})
	swc2 $$18, 0({c})
	swc2 $$19, 0({e})
	.set pop`, map[string]interface{}{"a": uintptr(unsafe.Pointer(a)), "b": uintptr(unsafe.Pointer(b)), "c": uintptr(unsafe.Pointer(c)), "e": uintptr(unsafe.Pointer(e))})
}

// gteSZ3At stores the depth of the last projection at p.
//
//go:inline
func gteSZ3At(p *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$19, 0({p})
	.set pop`, map[string]interface{}{"p": uintptr(unsafe.Pointer(p))})
}

// gteRGBTo stores the three colors lit at a, b and c.
//
//go:inline
func gteRGBTo(a, b, c *uint32) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$20, 0({a})
	swc2 $$21, 0({b})
	swc2 $$22, 0({c})
	.set pop`, map[string]interface{}{"a": uintptr(unsafe.Pointer(a)), "b": uintptr(unsafe.Pointer(b)), "c": uintptr(unsafe.Pointer(c))})
}

// gteSetV3Words loads V0, V1 and V2 from their words.
//
//go:inline
func gteSetV3Words(a, b, c, d, e, f uint32) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$0
	mtc2 {b}, $$1
	mtc2 {c}, $$2
	mtc2 {d}, $$3
	mtc2 {e}, $$4
	mtc2 {f}, $$5
	.set pop`, map[string]interface{}{"a": a, "b": b, "c": c, "d": d, "e": e, "f": f})
}

// gteSetIR loads IR1 to IR3.
//
//go:inline
func gteSetIR(x, y, z int32) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$9
	mtc2 {b}, $$10
	mtc2 {c}, $$11
	.set pop`, map[string]interface{}{"a": x, "b": y, "c": z})
}

// gteSetRGBC sets the color the lighting commands multiply by, with the
// GPU code in its top byte, which they carry into every color they make.
//
//go:inline
func gteSetRGBC(c uint32) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {c}, $$6
	.set pop`, map[string]interface{}{"c": c})
}

// gteRTPT projects V0, V1 and V2 to the screen.
//
//go:inline
func gteRTPT() {
	device.AsmFull(`nop
	nop
	.word 0x4a280030`, nil)
}

// gteRTPS projects V0 to the screen, pushing the screen point and depth
// queues.
//
//go:inline
func gteRTPS() {
	device.AsmFull(`nop
	nop
	.word 0x4a180001`, nil)
}

// gteNCLIP is NCLIP on the last three screen points: positive when they
// turn clockwise, a polygon facing the viewer.
//
//go:inline
func gteNCLIP() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	.word 0x4b400006
	mfc2 {}, $$24
	nop
	.set pop`, nil))
}

// gteAVSZ3 is AVSZ3: the average of the last three depths, scaled, as
// the depth to sort at.
//
//go:inline
func gteAVSZ3() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	.word 0x4b58002d
	mfc2 {}, $$7
	nop
	.set pop`, nil))
}

// gteAVSZ4 is AVSZ4: the average of the last four depths, scaled.
//
//go:inline
func gteAVSZ4() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	.word 0x4b68002e
	mfc2 {}, $$7
	nop
	.set pop`, nil))
}

// gteNCCS lights the normal in V0 under the lights and the color set, into
// RGB2.
//
//go:inline
func gteNCCS() {
	device.AsmFull(`nop
	nop
	.word 0x4b08041b`, nil)
}

// gteNCDS is gteNCCS with the depth cue, the fog, applied by the last
// projection's depth.
//
//go:inline
func gteNCDS() {
	device.AsmFull(`nop
	nop
	.word 0x4ae80413`, nil)
}

// gteNCCT lights the normals in V0, V1 and V2 into RGB0, RGB1 and RGB2.
//
//go:inline
func gteNCCT() {
	device.AsmFull(`nop
	nop
	.word 0x4b18043f`, nil)
}

// gteNCDT is gteNCCT with the fog.
//
//go:inline
func gteNCDT() {
	device.AsmFull(`nop
	nop
	.word 0x4af80416`, nil)
}

// gteMVMVA multiplies V0 by the rotation matrix, shifted back by 12,
// into IR1 to IR3 and MAC1 to MAC3.
//
//go:inline
func gteMVMVA() {
	device.AsmFull(`nop
	nop
	.word 0x4a486012`, nil)
}

// gteMVMVAIR multiplies IR1 to IR3 by the rotation matrix, unshifted,
// into MAC1 to MAC3.
//
//go:inline
func gteMVMVAIR() {
	device.AsmFull(`nop
	nop
	.word 0x4a41e012`, nil)
}

// gteMVMVAIRSF multiplies IR1 to IR3 by the rotation matrix, shifted back
// by 12, into MAC1 to MAC3.
//
//go:inline
func gteMVMVAIRSF() {
	device.AsmFull(`nop
	nop
	.word 0x4a49e012`, nil)
}

// The screen points of the last three projections.

//go:inline
func gteSXY0() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$12
	nop
	.set pop`, nil))
}

//go:inline
func gteSXY1() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$13
	nop
	.set pop`, nil))
}

//go:inline
func gteSXY2() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$14
	nop
	.set pop`, nil))
}

// The colors the lighting commands made, each with its GPU code.

//go:inline
func gteRGB0() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$20
	nop
	.set pop`, nil))
}

//go:inline
func gteRGB1() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$21
	nop
	.set pop`, nil))
}

//go:inline
func gteRGB2() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$22
	nop
	.set pop`, nil))
}

// IR1 to IR3, the 16-bit results.

//go:inline
func gteIR1() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$9
	nop
	.set pop`, nil))
}

//go:inline
func gteIR2() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$10
	nop
	.set pop`, nil))
}

//go:inline
func gteIR3() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$11
	nop
	.set pop`, nil))
}

// MAC1 to MAC3, the 32-bit results.

//go:inline
func gteMAC1() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$25
	nop
	.set pop`, nil))
}

//go:inline
func gteMAC2() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$26
	nop
	.set pop`, nil))
}

//go:inline
func gteMAC3() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$27
	nop
	.set pop`, nil))
}

// gteIR0 is the depth cue factor the last projection left.
//
//go:inline
func gteIR0() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$8
	nop
	.set pop`, nil))
}

// The depths of the last four projections, oldest first.

//go:inline
func gteSZ0() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$16
	nop
	.set pop`, nil))
}

//go:inline
func gteSZ1() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$17
	nop
	.set pop`, nil))
}

//go:inline
func gteSZ2() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$18
	nop
	.set pop`, nil))
}

//go:inline
func gteSZ3() uint32 {
	return uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$19
	nop
	.set pop`, nil))
}

// gteLZCR counts a's leading bits that match its sign, by the GTE's
// LZCS register.
//
//go:inline
func gteLZCR(a int32) int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$30
	nop
	mfc2 {}, $$31
	nop
	.set pop`, map[string]interface{}{"a": a}))
}

// gteSetIR0 loads IR0, the interpolation factor.
//
//go:inline
func gteSetIR0(v int32) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$8
	.set pop`, map[string]interface{}{"a": v})
}

// gteGPF multiplies IR1 to IR3 by IR0, shifted back by 12, into IR1 to
// IR3.
//
//go:inline
func gteGPF() {
	device.AsmFull(`nop
	nop
	.word 0x4b98003d`, nil)
}

// gteFlag is the flag register as the last command left it: negative
// when a result overflowed. The assembler takes no cfc2, so the move is
// written as its word, into a register named here, which must be one
// the compiler never gives a value to, since the block cannot say it
// clobbers one: the global pointer, which Go code has no use for and the
// kernel keeps across an interrupt. The compiler does give values to
// $at, and a value of the caller's held there across an inlined read was
// once overwritten by it. A C program built with Sony's compiler does use
// the global pointer, for its small data, so it is kept in the result's
// register across the move and the two are swapped back after it.
//
//go:inline
func gteFlag() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	addu {}, $$gp, $$zero
	.word 0x485cf800
	nop
	xor $$gp, $$gp, {}
	xor {}, {}, $$gp
	xor $$gp, $$gp, {}
	.set pop`, nil))
}
