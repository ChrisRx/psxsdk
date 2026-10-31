//go:build yaroze

package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// The 3D side. In place: the matrix products and the transforms of
// libgte, the lights as the GTE then colors normals, and the viewpoint
// and coordinate matrices of libgs, each worked out by both libraries
// on the same inputs. On the hardware: a model with a primitive of every
// kind sorted and drawn by one library under each lighting, with the
// packets it made summed, and the lines, the fast sprite, a table sorted
// into another, and the image fills, moves and reads.

//export MulMatrix0_orig
func MulMatrix0_orig(m0, m1, m2 *yaroze.MATRIX) *yaroze.MATRIX

//export MulMatrix_orig
func MulMatrix_orig(m0, m1 *yaroze.MATRIX) *yaroze.MATRIX

//export CompMatrix_orig
func CompMatrix_orig(m0, m1, m2 *yaroze.MATRIX) *yaroze.MATRIX

//export ApplyMatrix_orig
func ApplyMatrix_orig(m *yaroze.MATRIX, v *yaroze.SVECTOR, out *yaroze.VECTOR) *yaroze.VECTOR

//export ApplyMatrixSV_orig
func ApplyMatrixSV_orig(m *yaroze.MATRIX, v *yaroze.SVECTOR, out *yaroze.SVECTOR) *yaroze.SVECTOR

//export ApplyMatrixLV_orig
func ApplyMatrixLV_orig(m *yaroze.MATRIX, v *yaroze.VECTOR, out *yaroze.VECTOR) *yaroze.VECTOR

//export TransposeMatrix_orig
func TransposeMatrix_orig(m, out *yaroze.MATRIX) *yaroze.MATRIX

//export RotMatrixX_orig
func RotMatrixX_orig(r int32, m *yaroze.MATRIX) *yaroze.MATRIX

//export RotMatrixY_orig
func RotMatrixY_orig(r int32, m *yaroze.MATRIX) *yaroze.MATRIX

//export RotMatrixZ_orig
func RotMatrixZ_orig(r int32, m *yaroze.MATRIX) *yaroze.MATRIX

//export ScaleMatrixL_orig
func ScaleMatrixL_orig(m *yaroze.MATRIX, v *yaroze.VECTOR) *yaroze.MATRIX

//export MulMatrix
func MulMatrix(m0, m1 *yaroze.MATRIX) *yaroze.MATRIX

//export GsSetFlatLight_orig
func GsSetFlatLight_orig(id int32, lt *yaroze.GsF_LIGHT) int32

//export GsSetAmbient_orig
func GsSetAmbient_orig(r, g, b int32)

//export GsSetLightMatrix_orig
func GsSetLightMatrix_orig(m *yaroze.MATRIX)

//export GsSetLsMatrix_orig
func GsSetLsMatrix_orig(m *yaroze.MATRIX)

//export GsSetRefView2_orig
func GsSetRefView2_orig(pv *yaroze.GsRVIEW2) int32

//export GsSetView2_orig
func GsSetView2_orig(pv *yaroze.GsVIEW2) int32

//export GsInitCoordinate2_orig
func GsInitCoordinate2_orig(super, coord *yaroze.GsCOORDINATE2)

//export GsGetLw_orig
func GsGetLw_orig(m *yaroze.GsCOORDINATE2, out *yaroze.MATRIX)

//export GsGetLs_orig
func GsGetLs_orig(m *yaroze.GsCOORDINATE2, out *yaroze.MATRIX)

//export GsGetLws_orig
func GsGetLws_orig(m *yaroze.GsCOORDINATE2, outw, outs *yaroze.MATRIX)

//export GsScaleScreen_orig
func GsScaleScreen_orig(scale *yaroze.SVECTOR)

//export GsInit3D_orig
func GsInit3D_orig()

//export GsSetProjection_orig
func GsSetProjection_orig(h int32)

//export GsSetLightMode_orig
func GsSetLightMode_orig(mode int32)

//export GsSetFogParam_orig
func GsSetFogParam_orig(fog *yaroze.GsFOGPARAM)

//export GsMapModelingData_orig
func GsMapModelingData_orig(p *uint32)

//export GsLinkObject4_orig
func GsLinkObject4_orig(tmd *uint32, objp *yaroze.GsDOBJ2, n int32)

//export GsSortObject4_orig
func GsSortObject4_orig(objp *yaroze.GsDOBJ2, ot *yaroze.GsOT, shift int32, scratch *uint32)

//export GsSortLine_orig
func GsSortLine_orig(lp *yaroze.GsLINE, ot *yaroze.GsOT, pri uint16)

//export GsSortGLine_orig
func GsSortGLine_orig(lp *yaroze.GsGLINE, ot *yaroze.GsOT, pri uint16)

//export GsSortFastSprite_orig
func GsSortFastSprite_orig(sp *yaroze.GsSPRITE, ot *yaroze.GsOT, pri uint16)

//export GsSortOt_orig
func GsSortOt_orig(ot0, ot1 *yaroze.GsOT) *yaroze.GsOT

//export ClearImage_orig
func ClearImage_orig(rect *yaroze.RECT, r, g, b uint8) int32

//export MoveImage_orig
func MoveImage_orig(rect *yaroze.RECT, x, y int32) int32

//export StoreImage_orig
func StoreImage_orig(rect *yaroze.RECT, p *uint32) int32

// The light matrix, which both libraries keep in the one place under
// this program.
//
//go:extern GsLIGHTWSMATRIX
var lightWS yaroze.MATRIX

//go:extern GsWSMATRIX
var worldScreen yaroze.MATRIX

// A lib3d is one library's 3D calls, so that the hardware script is
// written once.
type lib3d struct {
	init3D          func()
	setProjection   func(int32)
	initCoordinate2 func(super, coord *yaroze.GsCOORDINATE2)
	setRefView2     func(*yaroze.GsRVIEW2) int32
	setFlatLight    func(int32, *yaroze.GsF_LIGHT) int32
	setAmbient      func(r, g, b int32)
	setLightMode    func(int32)
	setFogParam     func(*yaroze.GsFOGPARAM)
	mapModelingData func(*uint32)
	linkObject4     func(*uint32, *yaroze.GsDOBJ2, int32)
	getLw, getLs    func(*yaroze.GsCOORDINATE2, *yaroze.MATRIX)
	getLws          func(*yaroze.GsCOORDINATE2, *yaroze.MATRIX, *yaroze.MATRIX)
	scaleScreen     func(*yaroze.SVECTOR)
	setView2        func(*yaroze.GsVIEW2) int32
	setLightMatrix  func(*yaroze.MATRIX)
	setLsMatrix     func(*yaroze.MATRIX)
	sortObject4     func(*yaroze.GsDOBJ2, *yaroze.GsOT, int32, *uint32)
	sortLine        func(*yaroze.GsLINE, *yaroze.GsOT, uint16)
	sortGLine       func(*yaroze.GsGLINE, *yaroze.GsOT, uint16)
	sortFastSprite  func(*yaroze.GsSPRITE, *yaroze.GsOT, uint16)
	sortOt          func(ot0, ot1 *yaroze.GsOT) *yaroze.GsOT
	clearImage      func(*yaroze.RECT, uint8, uint8, uint8) int32
	moveImage       func(*yaroze.RECT, int32, int32) int32
	storeImage      func(*yaroze.RECT, *uint32) int32
	setWorkBase     func(*yaroze.PACKET)
	clearOt         func(uint16, uint16, *yaroze.GsOT)
	sortClear       func(uint8, uint8, uint8, *yaroze.GsOT)
	drawOt          func(*yaroze.GsOT)
	drawSync        func(int32) int32
	vsync           func(int32) int32
	swapDispBuff    func()
	getActiveBuff   func() int32
}

func libFor(mode int32) lib3d {
	if mode == 0 {
		return lib3d{
			init3D:          func() { GsInit3D_orig() },
			setProjection:   func(h int32) { GsSetProjection_orig(h) },
			initCoordinate2: func(super, coord *yaroze.GsCOORDINATE2) { GsInitCoordinate2_orig(super, coord) },
			setRefView2:     func(pv *yaroze.GsRVIEW2) int32 { return GsSetRefView2_orig(pv) },
			setFlatLight:    func(id int32, lt *yaroze.GsF_LIGHT) int32 { return GsSetFlatLight_orig(id, lt) },
			setAmbient:      func(r, g, b int32) { GsSetAmbient_orig(r, g, b) },
			setLightMode:    func(m int32) { GsSetLightMode_orig(m) },
			setFogParam:     func(f *yaroze.GsFOGPARAM) { GsSetFogParam_orig(f) },
			mapModelingData: func(p *uint32) { GsMapModelingData_orig(p) },
			linkObject4:     func(t *uint32, o *yaroze.GsDOBJ2, n int32) { GsLinkObject4_orig(t, o, n) },
			getLw:           func(c *yaroze.GsCOORDINATE2, m *yaroze.MATRIX) { GsGetLw_orig(c, m) },
			getLs:           func(c *yaroze.GsCOORDINATE2, m *yaroze.MATRIX) { GsGetLs_orig(c, m) },
			getLws:          func(c *yaroze.GsCOORDINATE2, w, m *yaroze.MATRIX) { GsGetLws_orig(c, w, m) },
			scaleScreen:     func(v *yaroze.SVECTOR) { GsScaleScreen_orig(v) },
			setView2:        func(v *yaroze.GsVIEW2) int32 { return GsSetView2_orig(v) },
			setLightMatrix:  func(m *yaroze.MATRIX) { GsSetLightMatrix_orig(m) },
			setLsMatrix:     func(m *yaroze.MATRIX) { GsSetLsMatrix_orig(m) },
			sortObject4:     func(o *yaroze.GsDOBJ2, ot *yaroze.GsOT, sh int32, sc *uint32) { GsSortObject4_orig(o, ot, sh, sc) },
			sortLine:        func(l *yaroze.GsLINE, ot *yaroze.GsOT, pri uint16) { GsSortLine_orig(l, ot, pri) },
			sortGLine:       func(l *yaroze.GsGLINE, ot *yaroze.GsOT, pri uint16) { GsSortGLine_orig(l, ot, pri) },
			sortFastSprite:  func(sp *yaroze.GsSPRITE, ot *yaroze.GsOT, pri uint16) { GsSortFastSprite_orig(sp, ot, pri) },
			sortOt:          func(a, b *yaroze.GsOT) *yaroze.GsOT { return GsSortOt_orig(a, b) },
			clearImage:      func(r *yaroze.RECT, cr, cg, cb uint8) int32 { return ClearImage_orig(r, cr, cg, cb) },
			moveImage:       func(r *yaroze.RECT, x, y int32) int32 { return MoveImage_orig(r, x, y) },
			storeImage:      func(r *yaroze.RECT, p *uint32) int32 { return StoreImage_orig(r, p) },
			setWorkBase:     func(p *yaroze.PACKET) { GsSetWorkBase_orig(p) },
			clearOt:         func(o, pt uint16, ot *yaroze.GsOT) { GsClearOt_orig(o, pt, ot) },
			sortClear:       func(cr, cg, cb uint8, ot *yaroze.GsOT) { GsSortClear_orig(cr, cg, cb, ot) },
			drawOt:          func(ot *yaroze.GsOT) { GsDrawOt_orig(ot) },
			drawSync:        func(m int32) int32 { return DrawSync_orig(m) },
			vsync:           func(m int32) int32 { return VSync_orig(m) },
			swapDispBuff:    func() { GsSwapDispBuff_orig() },
			getActiveBuff:   func() int32 { return GsGetActiveBuff_orig() },
		}
	}
	return lib3d{
		init3D:          func() { yaroze.GsInit3D() },
		setProjection:   func(h int32) { yaroze.GsSetProjection(h) },
		initCoordinate2: func(super, coord *yaroze.GsCOORDINATE2) { yaroze.GsInitCoordinate2(super, coord) },
		setRefView2:     func(pv *yaroze.GsRVIEW2) int32 { return yaroze.GsSetRefView2(pv) },
		setFlatLight:    func(id int32, lt *yaroze.GsF_LIGHT) int32 { return yaroze.GsSetFlatLight(id, lt) },
		setAmbient:      func(r, g, b int32) { yaroze.GsSetAmbient(r, g, b) },
		setLightMode:    func(m int32) { yaroze.GsSetLightMode(m) },
		setFogParam:     func(f *yaroze.GsFOGPARAM) { yaroze.GsSetFogParam(f) },
		mapModelingData: func(p *uint32) { yaroze.GsMapModelingData(p) },
		linkObject4:     func(t *uint32, o *yaroze.GsDOBJ2, n int32) { yaroze.GsLinkObject4(t, o, n) },
		getLw:           func(c *yaroze.GsCOORDINATE2, m *yaroze.MATRIX) { yaroze.GsGetLw(c, m) },
		getLs:           func(c *yaroze.GsCOORDINATE2, m *yaroze.MATRIX) { yaroze.GsGetLs(c, m) },
		getLws:          func(c *yaroze.GsCOORDINATE2, w, m *yaroze.MATRIX) { yaroze.GsGetLws(c, w, m) },
		scaleScreen:     func(v *yaroze.SVECTOR) { yaroze.GsScaleScreen(v) },
		setView2:        func(v *yaroze.GsVIEW2) int32 { return yaroze.GsSetView2(v) },
		setLightMatrix:  func(m *yaroze.MATRIX) { yaroze.GsSetLightMatrix(m) },
		setLsMatrix:     func(m *yaroze.MATRIX) { yaroze.GsSetLsMatrix(m) },
		sortObject4:     func(o *yaroze.GsDOBJ2, ot *yaroze.GsOT, sh int32, sc *uint32) { yaroze.GsSortObject4(o, ot, sh, sc) },
		sortLine:        func(l *yaroze.GsLINE, ot *yaroze.GsOT, pri uint16) { yaroze.GsSortLine(l, ot, pri) },
		sortGLine:       func(l *yaroze.GsGLINE, ot *yaroze.GsOT, pri uint16) { yaroze.GsSortGLine(l, ot, pri) },
		sortFastSprite:  func(sp *yaroze.GsSPRITE, ot *yaroze.GsOT, pri uint16) { yaroze.GsSortFastSprite(sp, ot, pri) },
		sortOt:          func(a, b *yaroze.GsOT) *yaroze.GsOT { return yaroze.GsSortOt(a, b) },
		clearImage:      func(r *yaroze.RECT, cr, cg, cb uint8) int32 { return yaroze.ClearImage(r, cr, cg, cb) },
		moveImage:       func(r *yaroze.RECT, x, y int32) int32 { return yaroze.MoveImage(r, x, y) },
		storeImage:      func(r *yaroze.RECT, p *uint32) int32 { return yaroze.StoreImage(r, p) },
		setWorkBase:     func(p *yaroze.PACKET) { yaroze.GsSetWorkBase(p) },
		clearOt:         func(o, pt uint16, ot *yaroze.GsOT) { yaroze.GsClearOt(o, pt, ot) },
		sortClear:       func(cr, cg, cb uint8, ot *yaroze.GsOT) { yaroze.GsSortClear(cr, cg, cb, ot) },
		drawOt:          func(ot *yaroze.GsOT) { yaroze.GsDrawOt(ot) },
		drawSync:        func(m int32) int32 { return yaroze.DrawSync(m) },
		vsync:           func(m int32) int32 { return yaroze.VSync(m) },
		swapDispBuff:    func() { yaroze.GsSwapDispBuff() },
		getActiveBuff:   func() int32 { return yaroze.GsGetActiveBuff() },
	}
}

// The in-place cases.

var (
	mA = yaroze.MATRIX{M: [3][3]int16{{4096, 0, 0}, {0, 3547, -2048}, {0, 2048, 3547}}, T: [3]int32{100, -200, 300}}
	mB = yaroze.MATRIX{M: [3][3]int16{{2896, -2896, 0}, {2896, 2896, 0}, {0, 0, 4096}}, T: [3]int32{-5000, 70000, 123456}}
	mC = yaroze.MATRIX{M: [3][3]int16{{-1000, 2000, -3000}, {4000, -5000, 6000}, {-7000, 8000, 9000}}, T: [3]int32{-1, 1, 0}}
)

func gteCases() {
	pairs := [][2]yaroze.MATRIX{{mA, mB}, {mB, mC}, {mC, mA}, {mA, mA}}
	for i, pr := range pairs {
		m0, m1 := pr[0], pr[1]
		var ra, rb yaroze.MATRIX
		MulMatrix0_orig(&m0, &m1, &ra)
		yaroze.MulMatrix0(&m0, &m1, &rb)
		check("MulMatrix0 "+itoa(i), bytesOf32(&ra), bytesOf32(&rb))
		ra, rb = m0, m0
		MulMatrix_orig(&ra, &m1)
		MulMatrix(&rb, &m1)
		check("MulMatrix "+itoa(i), bytesOf32(&ra), bytesOf32(&rb))
		CompMatrix_orig(&m0, &m1, &ra)
		yaroze.CompMatrix(&m0, &m1, &rb)
		check("CompMatrix "+itoa(i), bytesOf32(&ra), bytesOf32(&rb))
		TransposeMatrix_orig(&m0, &ra)
		yaroze.TransposeMatrix(&m0, &rb)
		check("TransposeMatrix "+itoa(i), bytesOf32(&ra), bytesOf32(&rb))
		sv := yaroze.SVECTOR{Vx: int16(1000 * (i + 1)), Vy: -2000, Vz: 3000}
		var va, vb yaroze.VECTOR
		ApplyMatrix_orig(&m0, &sv, &va)
		yaroze.ApplyMatrix(&m0, &sv, &vb)
		check("ApplyMatrix "+itoa(i), bytesOfVec(&va), bytesOfVec(&vb))
		var sa, sb yaroze.SVECTOR
		ApplyMatrixSV_orig(&m0, &sv, &sa)
		yaroze.ApplyMatrixSV(&m0, &sv, &sb)
		check("ApplyMatrixSV "+itoa(i), bytesOf16(&sa), bytesOf16(&sb))
		lv := yaroze.VECTOR{Vx: 123456 * int32(i+1), Vy: -7654321, Vz: 40000}
		ApplyMatrixLV_orig(&m0, &lv, &va)
		yaroze.ApplyMatrixLV(&m0, &lv, &vb)
		check("ApplyMatrixLV "+itoa(i), bytesOfVec(&va), bytesOfVec(&vb))
		for j, r := range []int32{0, 512, -1024, 3000, 5000, -70000} {
			ra, rb = m0, m0
			RotMatrixX_orig(r, &ra)
			yaroze.RotMatrixX(r, &rb)
			check("RotMatrixX "+itoa(i)+" "+itoa(j), bytesOf32(&ra), bytesOf32(&rb))
			ra, rb = m0, m0
			RotMatrixY_orig(r, &ra)
			yaroze.RotMatrixY(r, &rb)
			check("RotMatrixY "+itoa(i)+" "+itoa(j), bytesOf32(&ra), bytesOf32(&rb))
			ra, rb = m0, m0
			RotMatrixZ_orig(r, &ra)
			yaroze.RotMatrixZ(r, &rb)
			check("RotMatrixZ "+itoa(i)+" "+itoa(j), bytesOf32(&ra), bytesOf32(&rb))
		}
		ra, rb = m0, m0
		s := yaroze.VECTOR{Vx: 2048, Vy: -4096, Vz: 8192}
		ScaleMatrixL_orig(&ra, &s)
		yaroze.ScaleMatrixL(&rb, &s)
		check("ScaleMatrixL "+itoa(i), bytesOf32(&ra), bytesOf32(&rb))
	}
}

func bytesOfVec(v *yaroze.VECTOR) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(v)), 12)
}

func bytesOf16(v *yaroze.SVECTOR) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(v)), 6)
}

var (
	lights = [3]yaroze.GsF_LIGHT{
		{Vx: 20, Vy: -100, Vz: -100, R: 0xd0, G: 0xd0, B: 0xd0},
		{Vx: -300, Vy: 0, Vz: 400, R: 255, G: 0, B: 0},
		{Vx: 0, Vy: 1, Vz: 0, R: 0, G: 100, B: 200},
	}
	normals = [...]yaroze.SVECTOR{
		{Vx: 0, Vy: -4096, Vz: 0}, {Vx: 2896, Vy: -2896, Vz: 0}, {Vx: 0, Vy: 0, Vz: -4096},
		{Vx: -2367, Vy: -2367, Vz: 2367}, {Vx: 4096, Vy: 0, Vz: 0}, {Vx: 0, Vy: 0, Vz: 4096},
	}
)

// lightColors sets the lights and the ambient under one library, turns
// the lights by a matrix, and returns what the GTE then makes of each
// normal, with the light matrix as the library left it.
func lightColors(L lib3d, lw *yaroze.MATRIX) ([len(normals)]uint32, yaroze.MATRIX, int32) {
	lightWS = yaroze.MATRIX{}
	ret := int32(0)
	for i := range lights {
		ret = ret*4 + L.setFlatLight(int32(i), &lights[i])
	}
	none := yaroze.GsF_LIGHT{}
	ret = ret*4 + L.setFlatLight(1, &none) + 2
	// A direction too short to normalize, as the square example gives.
	short := yaroze.GsF_LIGHT{Vx: 1, Vy: 1, Vz: 1, R: 0xc0, G: 0xc0, B: 0xc0}
	ret = ret*4 + L.setFlatLight(2, &short)
	set := lightWS
	L.setAmbient(1000, 2000, 3000)
	L.setLightMatrix(lw)
	var cs [len(normals)]uint32
	for i := range normals {
		yaroze.SetRGBC(0x20c08040)
		cs[i] = yaroze.NormalColorCol(&normals[i])
	}
	return cs, set, ret
}

func lightCases() {
	lw := yaroze.IDMATRIX
	r := yaroze.SVECTOR{Vx: 300, Vy: -500, Vz: 1000}
	yaroze.RotMatrix(&r, &lw)
	ca, la, ra := lightColors(libFor(0), &lw)
	cb, lb, rb := lightColors(libFor(1), &lw)
	checkInt("GsSetFlatLight returns", ra, rb)
	check("GsSetFlatLight matrix", bytesOf32(&la), bytesOf32(&lb))
	for i := range ca {
		checkInt("light color "+itoa(i), int32(ca[i]), int32(cb[i]))
	}
}

var (
	coordTop, coordMid, coordLow yaroze.GsCOORDINATE2
)

// chain makes three coordinate systems one inside the next under one
// library, each turned and moved, with their flags clear.
func chain(L lib3d) {
	L.initCoordinate2(yaroze.WORLD, &coordTop)
	L.initCoordinate2(&coordTop, &coordMid)
	L.initCoordinate2(&coordMid, &coordLow)
	r := yaroze.SVECTOR{Vx: 200, Vy: 400, Vz: -100}
	yaroze.RotMatrix(&r, &coordTop.Coord)
	coordTop.Coord.T = [3]int32{1000, -2000, 50000}
	r = yaroze.SVECTOR{Vx: -300, Vy: 0, Vz: 1024}
	yaroze.RotMatrix(&r, &coordMid.Coord)
	coordMid.Coord.T = [3]int32{-70, 800, -9000}
	r = yaroze.SVECTOR{Vx: 0, Vy: 2048, Vz: 0}
	yaroze.RotMatrix(&r, &coordLow.Coord)
	coordLow.Coord.T = [3]int32{12345, 0, -1}
	coordTop.Flg, coordMid.Flg, coordLow.Flg = 0, 0, 0
}

func viewCases() {
	views := []yaroze.GsRVIEW2{
		{Vpx: 0, Vpy: 0, Vpz: 1000},
		{Vpx: 500, Vpy: -300, Vpz: 800, Vrx: 10, Vry: 20, Vrz: -30, Rz: 30 << 12},
		{Vpx: 100000, Vpy: 50000, Vpz: -200000, Rz: -45 << 12},
		{Vpx: -3000, Vpy: 4000, Vpz: 0, Vrx: -3000, Vry: 4000, Vrz: 0},
		{Vpx: 0, Vpy: -1000, Vpz: -1, Vrx: 0, Vry: 1000, Vrz: 0, Rz: 400 << 12},
	}
	for i, v := range views {
		va, vb := v, v
		ra := GsSetRefView2_orig(&va)
		wa := worldScreen
		rb := yaroze.GsSetRefView2(&vb)
		wb := worldScreen
		checkInt("GsSetRefView2 "+itoa(i)+" returns", ra, rb)
		check("GsSetRefView2 "+itoa(i), bytesOf32(&wa), bytesOf32(&wb))
	}
	// The viewpoint in a coordinate system, and the systems' matrices.
	a, b := viewSums(libFor(0)), viewSums(libFor(1))
	for i := range a {
		checkInt("view "+itoa(i), int32(a[i]), int32(b[i]))
	}
}

// viewSums works the view and the coordinate matrices out under one
// library, each summed to a word: the viewpoint inside the chain, the
// chain's matrices to the world and to the screen, the cache after the
// middle system changes, the scaled screen and a view given as a matrix.
func viewSums(L lib3d) (s [8]uint32) {
	chain(L)
	v := yaroze.GsRVIEW2{Vpx: 500, Vpy: -300, Vpz: 800, Vrx: 10, Vry: 20, Vrz: -30, Rz: 30 << 12, Super: &coordLow}
	L.setRefView2(&v)
	s[0] = sum32(&worldScreen)
	var m, m2 yaroze.MATRIX
	chain(L)
	L.getLw(&coordLow, &m)
	s[1] = sum32(&m)
	chain(L)
	L.getLs(&coordLow, &m)
	s[2] = sum32(&m)
	chain(L)
	L.getLws(&coordLow, &m, &m2)
	s[3], s[4] = sum32(&m), sum32(&m2)
	coordMid.Coord.T[0] += 100
	coordMid.Flg = 0
	L.getLw(&coordLow, &m)
	s[5] = sum32(&m)
	sc := yaroze.SVECTOR{Vx: 2048, Vy: 2048, Vz: 4096}
	L.scaleScreen(&sc)
	s[6] = sum32(&worldScreen)
	view2 := yaroze.GsVIEW2{View: mB, Super: &coordMid}
	coordMid.Flg = 0
	L.setView2(&view2)
	s[7] = sum32(&worldScreen)
	return s
}

// sum32 sums a matrix's words, for one line of the report.
func sum32(m *yaroze.MATRIX) uint32 {
	s := uint32(0)
	for _, w := range unsafe.Slice((*uint32)(unsafe.Pointer(m)), 8) {
		s = s*31 + w
	}
	return s
}

func objCases() {
	gteCases()
	lightCases()
	viewCases()
}

// The hardware script: a model of every primitive kind.

// The model: a header, one object, vertices, normals and primitives, with
// the offsets GsMapModelingData turns into addresses.
var (
	model     [512]uint32
	modelObj  yaroze.GsDOBJ2
	objCoord  yaroze.GsCOORDINATE2
	objTags   [2][256]yaroze.GsOT_TAG
	objOT     [2]yaroze.GsOT
	objArea   [2][12288]uint32
	storeBuf  [128]uint32
	smallTags [4]yaroze.GsOT_TAG
)

// buildModel writes the model: 18 primitives, each its own triangle or
// quad facing the viewer in a grid across the x, y plane, with the
// 16-pixel pattern at 640, 0 as the texture of the textured ones.
func buildModel() {
	tpage := uint32(yaroze.GetTPage(2, 0, 640, 0))
	var verts []uint32
	var prims []uint32
	nverts := uint32(0)
	// vertex adds a vertex and returns its index.
	vertex := func(x, y, z int16) uint32 {
		verts = append(verts, uint32(uint16(x))|uint32(uint16(y))<<16, uint32(uint16(z)))
		nverts++
		return nverts - 1
	}
	h := func(v int16) uint32 { return uint32(uint16(v)) }
	normalsTab := []uint32{
		h(0) | h(0)<<16, h(-4096),
		h(2367) | h(-2367)<<16, h(-2367),
		h(-2367) | h(2367)<<16, h(-2367),
		h(0) | h(2896)<<16, h(-2896),
	}
	hdr := func(olen, ilen, flag, mode uint32) uint32 { return olen | ilen<<8 | flag<<16 | mode<<24 }
	color := func(r, g, b, code uint32) uint32 { return r | g<<8 | b<<16 | code<<24 }
	uv := func(u, v, top uint32) uint32 { return u | v<<8 | top<<16 }
	// tri and quad add the vertices of a primitive at a grid cell,
	// wound to face the viewer, and return their indexes.
	cell := 0
	place := func() (x, y int16) {
		x = int16(-180 + 60*(cell%7))
		y = int16(-90 + 60*(cell/7))
		cell++
		return x, y
	}
	tri := func() (a, b, c uint32) {
		x, y := place()
		return vertex(x, y, 0), vertex(x, y+40, 0), vertex(x+40, y, 0)
	}
	quad := func() (a, b, c, d uint32) {
		x, y := place()
		return vertex(x, y, 0), vertex(x+40, y, 0), vertex(x, y+40, 0), vertex(x+40, y+40, 0)
	}
	// F3, two of them, a run.
	a, b, c := tri()
	prims = append(prims, hdr(4, 3, 0, 0x20), color(200, 40, 40, 0x20), 0|a<<16, b|c<<16)
	a, b, c = tri()
	prims = append(prims, hdr(4, 3, 0, 0x20), color(40, 200, 40, 0x20), 1|a<<16, b|c<<16)
	// NF3.
	a, b, c = tri()
	prims = append(prims, hdr(4, 3, 1, 0x21), color(40, 40, 200, 0x21), a|b<<16, c)
	// G3.
	a, b, c = tri()
	prims = append(prims, hdr(6, 4, 0, 0x30), color(200, 200, 200, 0x30), 0|a<<16, 1|b<<16, 2|c<<16)
	// NG3.
	a, b, c = tri()
	prims = append(prims, hdr(6, 5, 1, 0x31), color(255, 0, 0, 0x31), color(0, 255, 0, 0), color(0, 0, 255, 0), a|b<<16, c)
	// FT3.
	a, b, c = tri()
	prims = append(prims, hdr(7, 5, 0, 0x24), uv(0, 0, 0), uv(0, 15, tpage), uv(15, 0, 0), 0|a<<16, b|c<<16)
	// NFT3.
	a, b, c = tri()
	prims = append(prims, hdr(7, 6, 1, 0x25), uv(0, 0, 0), uv(0, 15, tpage), uv(15, 0, 0), color(128, 255, 128, 0x25), a|b<<16, c)
	// GT3.
	a, b, c = tri()
	prims = append(prims, hdr(9, 6, 0, 0x34), uv(0, 0, 0), uv(0, 15, tpage), uv(15, 0, 0), 0|a<<16, 1|b<<16, 2|c<<16)
	// NGT3.
	a, b, c = tri()
	prims = append(prims, hdr(9, 8, 1, 0x35), uv(0, 0, 0), uv(0, 15, tpage), uv(15, 0, 0), color(255, 128, 128, 0x35), color(128, 255, 128, 0), color(128, 128, 255, 0), a|b<<16, c)
	// F4.
	a, b, c, d := quad()
	prims = append(prims, hdr(5, 4, 0, 0x28), color(200, 100, 50, 0x28), 0|a<<16, b|c<<16, d)
	// NF4.
	a, b, c, d = quad()
	prims = append(prims, hdr(5, 3, 1, 0x29), color(50, 100, 200, 0x29), a|b<<16, c|d<<16)
	// G4.
	a, b, c, d = quad()
	prims = append(prims, hdr(8, 5, 0, 0x38), color(220, 220, 220, 0x38), 0|a<<16, 1|b<<16, 2|c<<16, 3|d<<16)
	// NG4.
	a, b, c, d = quad()
	prims = append(prims, hdr(8, 6, 1, 0x39), color(255, 0, 0, 0x39), color(0, 255, 0, 0), color(0, 0, 255, 0), color(255, 255, 0, 0), a|b<<16, c|d<<16)
	// FT4.
	a, b, c, d = quad()
	prims = append(prims, hdr(9, 7, 0, 0x2c), uv(0, 0, 0), uv(15, 0, tpage), uv(0, 15, 0), uv(15, 15, 0), 0|a<<16, b|c<<16, d)
	// NFT4.
	a, b, c, d = quad()
	prims = append(prims, hdr(9, 7, 1, 0x2d), uv(0, 0, 0), uv(15, 0, tpage), uv(0, 15, 0), uv(15, 15, 0), color(200, 200, 100, 0x2d), a|b<<16, c|d<<16)
	// GT4.
	a, b, c, d = quad()
	prims = append(prims, hdr(12, 8, 0, 0x3c), uv(0, 0, 0), uv(15, 0, tpage), uv(0, 15, 0), uv(15, 15, 0), 0|a<<16, 1|b<<16, 2|c<<16, 3|d<<16)
	// NGT4.
	a, b, c, d = quad()
	prims = append(prims, hdr(12, 10, 1, 0x3d), uv(0, 0, 0), uv(15, 0, tpage), uv(0, 15, 0), uv(15, 15, 0), color(255, 0, 0, 0x3d), color(0, 255, 0, 0), color(0, 0, 255, 0), color(255, 255, 255, 0), a|b<<16, c|d<<16)
	// A quad wound the other way, which both leave out.
	a, b, c, d = quad()
	prims = append(prims, hdr(5, 3, 1, 0x29), color(50, 100, 200, 0x29), b|a<<16, d|c<<16)
	// A textured triangle wound the other way but marked double-sided,
	// which only the subdivision honors.
	a, b, c = tri()
	prims = append(prims, hdr(7, 5, 2, 0x24), uv(0, 0, 0), uv(0, 15, tpage), uv(15, 0, 0), 0|b<<16, a|c<<16)
	// F3 and G3 with gradation, last: the original's handlers for them
	// write over the subdivision's work area, where the division count
	// lives, so a model that mixed them with subdivision would crash the
	// original on whatever came after.
	a, b, c = tri()
	prims = append(prims, hdr(6, 5, 4, 0x20), color(255, 0, 0, 0x20), color(0, 255, 0, 0), color(0, 0, 255, 0), 0|a<<16, b|c<<16)
	a, b, c = tri()
	prims = append(prims, hdr(6, 6, 4, 0x30), color(255, 128, 0, 0x30), color(0, 255, 128, 0), color(128, 0, 255, 0), 0|a<<16, 1|b<<16, 3|c<<16)

	// The file: id, flags, one object, its table, then the data, with
	// offsets from the table.
	model[0] = 0x41
	model[1] = 0
	model[2] = 1
	table := 3
	vertOff := uint32(7 * 4)
	normOff := vertOff + uint32(len(verts))*4
	primOff := normOff + uint32(len(normalsTab))*4
	model[table+0] = vertOff
	model[table+1] = nverts
	model[table+2] = normOff
	model[table+3] = uint32(len(normalsTab)) / 2
	model[table+4] = primOff
	model[table+5] = 20
	model[table+6] = 0
	copy(model[table+7:], verts)
	copy(model[table+7+len(verts):], normalsTab)
	copy(model[table+7+len(verts)+len(normalsTab):], prims)
}

// sortModel sorts the model under L with the attribute, and prints the
// packets it made.
func sortModel(L lib3d, name string, attr uint32, buf int32) {
	modelObj.Attribute = attr
	L.setWorkBase(yaroze.Packets(&objArea[buf][0]))
	L.clearOt(0, 0, &objOT[buf])
	var m yaroze.MATRIX
	L.getLw(modelObj.Coord2, &m)
	L.setLightMatrix(&m)
	L.getLs(modelObj.Coord2, &m)
	L.setLsMatrix(&m)
	L.sortObject4(&modelObj, &objOT[buf], 14-8, yaroze.Scratch())
	end := uintptr(unsafe.Pointer(outPacket)) - uintptr(unsafe.Pointer(&objArea[buf][0]))
	print("obj ", name, " ", end, " ", sumPackets(objArea[buf][:end/4]), "\n")
	if name == "lit" {
		dumpPackets(objArea[buf][:end/4])
	}
}

// objScript draws the model four times, under the lighting modes, with
// the lines and the sprite over the last frame, then sorts a small table
// into it, fills, moves and reads back video memory.
func objScript(mode int32) {
	L := libFor(mode)
	buildModel()
	for i := range objOT {
		objOT[i].Length = 8
		objOT[i].Org = &objTags[i][0]
	}
	L.init3D()
	L.setProjection(500)
	L.initCoordinate2(yaroze.WORLD, &objCoord)
	r := yaroze.SVECTOR{Vx: 300, Vy: -200, Vz: 100}
	yaroze.RotMatrix(&r, &objCoord.Coord)
	objCoord.Flg = 0
	view := yaroze.GsRVIEW2{Vpx: 100, Vpy: -50, Vpz: -700, Vrx: 0, Vry: 0, Vrz: 0, Rz: 10 << 12, Super: yaroze.WORLD}
	L.setRefView2(&view)
	for i := range lights {
		L.setFlatLight(int32(i), &lights[i])
	}
	L.setAmbient(1024, 1024, 1024)
	L.setLightMode(0)
	fog := yaroze.GsFOGPARAM{Dqa: -960, Dqb: 5120 * 5120, Rfc: 20, Gfc: 40, Bfc: 80}
	L.setFogParam(&fog)
	L.mapModelingData(&model[1])
	L.linkObject4(&model[3], &modelObj, 0)
	modelObj.Coord2 = &objCoord

	frames := []struct {
		name string
		attr uint32
		mode int32
	}{
		{"lit", 0, 0},
		{"fog", 0, 1},
		{"fog own", 1<<5 | 1<<3, 0},
		{"off alon", 1<<6 | 1<<30, 1},
		{"lit alon", 1 << 30, 0},
		{"div1", 1 << 9, 0},
		{"div2 fog", 2 << 9, 1},
		{"div1 off alon", 1<<9 | 1<<6 | 1<<30, 0},
		{"div2 own fog", 2<<9 | 1<<5 | 1<<3, 0},
	}
	for _, f := range frames {
		buf := L.getActiveBuff()
		L.setLightMode(f.mode)
		sortModel(L, f.name, f.attr, buf)
		L.drawSync(0)
		L.vsync(0)
		L.swapDispBuff()
		L.sortClear(30, 30, 60, &objOT[buf])
		L.drawOt(&objOT[buf])
	}

	// The lines and the sprite into the last frame's table, which is
	// drawn again, with a small table sorted into it.
	buf := L.getActiveBuff()
	objArea[buf] = [12288]uint32{}
	L.setWorkBase(yaroze.Packets(&objArea[buf][0]))
	L.clearOt(0, 0, &objOT[buf])
	line := yaroze.GsLINE{Attribute: 0, X0: -150, Y0: -100, X1: 150, Y1: 100, R: 255, G: 255, B: 0}
	L.sortLine(&line, &objOT[buf], 2)
	line = yaroze.GsLINE{Attribute: 1<<30 | 2<<28, X0: -150, Y0: 100, X1: 150, Y1: -100, R: 0, G: 255, B: 255}
	L.sortLine(&line, &objOT[buf], 2)
	gline := yaroze.GsGLINE{Attribute: 0, X0: -100, Y0: 0, X1: 100, Y1: 0, R0: 255, G0: 0, B0: 0, R1: 0, G1: 0, B1: 255}
	L.sortGLine(&gline, &objOT[buf], 1)
	sp := yaroze.GsSPRITE{Attribute: 2 << 24, X: -40, Y: -40, W: 16, H: 16, Tpage: uint16(yaroze.GetTPage(2, 0, 640, 0)), U: 0, V: 0, R: 128, G: 128, B: 128}
	L.sortFastSprite(&sp, &objOT[buf], 0)
	small := yaroze.GsOT{Length: 2, Org: &smallTags[0]}
	L.clearOt(0, 0, &small)
	small.Point = 3
	box := yaroze.GsBOXF{X: 120, Y: -110, W: 20, H: 30, R: 255, G: 128, B: 0}
	if mode == 0 {
		GsSortBoxFill_orig(&box, &small, 2)
	} else {
		yaroze.GsSortBoxFill(&box, &small, 2)
	}
	L.sortOt(&small, &objOT[buf])
	end := uintptr(unsafe.Pointer(outPacket)) - uintptr(unsafe.Pointer(&objArea[buf][0]))
	print("lines ", end, " ", sumPackets(objArea[buf][:end/4]), "\n")
	L.drawSync(0)
	L.vsync(0)
	L.swapDispBuff()
	L.sortClear(30, 30, 60, &objOT[buf])
	L.drawOt(&objOT[buf])
	L.drawSync(0)

	// Video memory: a fill on 64-pixel columns, one off them, a move of
	// the pattern, and the moved copy read back.
	rect := yaroze.RECT{X: 640, Y: 32, W: 64, H: 8}
	print("clear ", L.clearImage(&rect, 200, 100, 50), "\n")
	rect = yaroze.RECT{X: 650, Y: 48, W: 8, H: 8}
	print("clear2 ", L.clearImage(&rect, 50, 100, 200), "\n")
	rect = yaroze.RECT{X: 640, Y: 0, W: 16, H: 16}
	print("move ", L.moveImage(&rect, 672, 0), "\n")
	rect = yaroze.RECT{X: 672, Y: 0, W: 16, H: 0}
	print("move0 ", L.moveImage(&rect, 700, 0), "\n")
	rect = yaroze.RECT{X: 672, Y: 0, W: 16, H: 16}
	print("store ", L.storeImage(&rect, &storeBuf[0]), "\n")
	L.drawSync(0)
	print("stored ", sum32words(storeBuf[:]), "\n")
}

func sum32words(w []uint32) uint32 {
	s := uint32(0)
	for _, x := range w {
		s = s*31 + x
	}
	return s
}

// sumPackets sums the packets in an area, each by its tag's length and
// the words after it, leaving the links out, which differ between the
// two buffers.
func sumPackets(w []uint32) uint32 {
	s := uint32(0)
	for i := 0; i < len(w); {
		n := int(w[i] >> 24)
		s = s*31 + uint32(n)
		for _, x := range w[i+1 : min(i+1+n, len(w))] {
			s = s*31 + x
		}
		i += n + 1
	}
	return s
}

// dumpPackets prints the packets in an area, a line each, without the
// links.
func dumpPackets(w []uint32) {
	for i := 0; i < len(w); {
		n := int(w[i] >> 24)
		print("packet ", n)
		for _, x := range w[i+1 : min(i+1+n, len(w))] {
			print(" ", hex(uint8(x>>24)), hex(uint8(x>>16)), hex(uint8(x>>8)), hex(uint8(x)))
		}
		print("\n")
		i += n + 1
	}
}
