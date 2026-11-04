//go:build yaroze

package libps

import "unsafe"

// libgs's 3D side: the viewpoint, coordinate systems and the matrices
// between them, the lights, models in TMD files, and the lines and the
// fast sprite that sort through the GTE's offset.

// The attribute bits of the object being sorted, as GsSortObject4 takes
// them apart, and the light mode GsSetLightMode sets. They are the
// library's named variables, GsLMODE and the rest, which a program can
// reach by name as it can the others in gs.go.

//go:extern GsLMODE
var gsLMODE int32 // libps 0x80062D10: the object's own light mode

//go:extern GsLIGNR
var gsLIGNR int32 // libps 0x80062D14: the object asks for its own mode over the library's

//go:extern GsLIOFF
var gsLIOFF int32 // libps 0x80062D0C: the object is not lit

//go:extern GsNDIV
var gsNDIV int32 // libps 0x80066274: how many times its polygons are halved

//go:extern GsTON
var gsTON uint32 // libps 0x800658C8: the object is translucent

//go:extern GsLIGHT_MODE
var gsLightMode int32 // libps 0x80065954: 1 for fog

// gsWSSaved is the world-to-screen matrix as GsSetRefView2 or GsSetView2
// made it, before GsScaleScreen scales it.
//
// libps 0x80063D48
var gsWSSaved MATRIX

// GsInitCoordinate2 makes coord the identity inside super, which is nil
// or 1 for the world.
//
// libps 0x800219E8, 168 bytes
//
//export GsInitCoordinate2
func GsInitCoordinate2(super, coord *GsCOORDINATE2) {
	coord.Coord = GsIDMATRIX
	coord.Super = super
	coord.Flg = 0
	if super != nil && uintptr(unsafe.Pointer(super)) != 1 {
		super.Sub = coord
	}
}

// GsInit3D puts the origin at the middle of the screen and resets the
// light mode.
//
// libps 0x80021E80, 120 bytes
//
//export GsInit3D
func GsInit3D() {
	POSITION.Offx = int16(int32(screenW) / 2)
	POSITION.Offy = int16(int32(screenH) / 2)
	GsSetDrawBuffOffset()
	gsLightMode = 0
}

// GsSetProjection sets the projection distance: how far the screen is
// from the viewpoint.
//
// libps 0x80021F70, 32 bytes
//
//export GsSetProjection
func GsSetProjection(h int32) { setGeomScreen(h) }

// GsSetOrign moves the origin, the screen point the GTE's offset is
// taken from, without telling the GTE: GsSetDrawBuffOffset does.
//
// libps 0x800219D0, 24 bytes
//
//export GsSetOrign
func GsSetOrign(x, y int16) { POSITION.Offx, POSITION.Offy = x, y }

// GsSetOffset moves the drawing offset by x, y: the GPU's, when the
// screen was set up with GsOFSGPU, otherwise the GTE's, into the buffer
// being drawn. The origin is left alone.
//
// libps 0x800216F0, 248 bytes
//
//export GsSetOffset
func GsSetOffset(x, y int32) {
	if gsOfsGPU != 0 {
		GsDRAWENV.Ofs[0] = int16(x) + PSDOFSX[PSDIDX]
		GsDRAWENV.Ofs[1] = int16(y) + PSDOFSY[PSDIDX]
		gsOfsX, gsOfsY = 0, 0
		PutDrawEnv(&GsDRAWENV)
		return
	}
	other := 1 - PSDIDX
	gsOfsX = int16(x) + PSDOFSX[other]
	gsOfsY = int16(y) + PSDOFSY[other]
	setGeomOffset(int32(gsOfsX), int32(gsOfsY))
}

// GsSetClip clips drawing to clip, in the buffer shown.
//
// libps 0x800217E8, 128 bytes
//
//export GsSetClip
func GsSetClip(clip *RECT) {
	GsDRAWENV.Clip = RECT{clip.X + PSDOFSX[PSDIDX], clip.Y + PSDOFSY[PSDIDX], clip.W, clip.H}
	PutDrawEnv(&GsDRAWENV)
}

// GsSetClip2D sets the clip the next swap puts in place.
//
// libps 0x80021868, 56 bytes
//
//export GsSetClip2D
func GsSetClip2D(clip *RECT) { CLIP2 = *clip }

// GsGetWorkBase is where the next packet goes.
//
// libps 0x80022A70, 16 bytes
//
//export GsGetWorkBase
func GsGetWorkBase() *PACKET { return GsOUT_PACKET_P }

// gsRotateZ turns m about the z axis by an angle in degrees times 4096.
//
// libps 0x80022754, 184 bytes
func gsRotateZ(m *MATRIX, angle int32) {
	a := angle / 360
	c := rcos(a)
	s := rsin(a)
	if angle == 0 {
		return
	}
	r := MATRIX{M: [3][3]int16{{int16(c), int16(-s), 0}, {int16(s), int16(c), 0}, {0, 0, 4096}}}
	MulMatrix(m, &r)
}

// gsScaleView copies the viewpoint and the reference point out of pv,
// scaled down so that the largest fits in 16 bits.
//
// libps 0x80022F18, 240 bytes
func gsScaleView(pv *GsRVIEW2, v *[6]int32) {
	in := (*[6]int32)(unsafe.Pointer(pv))
	biggest := int32(0)
	for _, x := range in {
		if x < 0 {
			x = -x
		}
		if biggest < x {
			biggest = x
		}
	}
	bits := 0
	for ; biggest > 0; biggest >>= 1 {
		bits++
	}
	if bits < 16 {
		*v = *in
		return
	}
	for i, x := range in {
		v[i] = x >> uint(bits-15)
	}
}

// gsAxisMatrix makes m a turn about one axis, x, y or z, with the sine
// and cosine given.
//
// libps 0x800230EC, 196 bytes
func gsAxisMatrix(m *MATRIX, s, c int16, axis byte) {
	*m = GsIDMATRIX
	switch axis {
	case 'x', 'X':
		m.M[1][1], m.M[2][2] = c, c
		m.M[1][2], m.M[2][1] = -s, s
	case 'y', 'Y':
		m.M[0][0], m.M[2][2] = c, c
		m.M[0][2], m.M[2][0] = s, -s
	case 'z', 'Z':
		m.M[0][0], m.M[1][1] = c, c
		m.M[0][1], m.M[1][0] = -s, s
	}
}

// gsCompose makes out the product of out and m: out's rotation times
// m's, and m's translation turned by out's rotation and added to out's.
//
// libps 0x80021CA8, 132 bytes
func gsCompose(out, m *MATRIX) {
	var t [3]int32
	applyMatrixLV(out, &m.T, &t)
	MulMatrix(out, m)
	out.T = [3]int32{t[0] + out.T[0], t[1] + out.T[1], t[2] + out.T[2]}
}

// gsPrepend makes m the product of ws and m, into m.
//
// libps 0x80021C24, 132 bytes
func gsPrepend(ws, m *MATRIX) {
	var t [3]int32
	applyMatrixLV(ws, &m.T, &t)
	mulMatrix2(ws, m)
	m.T = [3]int32{t[0] + ws.T[0], t[1] + ws.T[1], t[2] + ws.T[2]}
}

// gsViewInSuper takes the world-to-screen matrix, made for a viewpoint
// given in the coordinate system super, into the world: the inverse of
// super's matrix in front of it.
func gsViewInSuper(super *GsCOORDINATE2) {
	var lw, t MATRIX
	GsGetLw(super, &lw)
	TransposeMatrix(&lw, &t)
	var v [3]int32
	applyMatrixLV(&t, &lw.T, &v)
	t.T = [3]int32{-v[0], -v[1], -v[2]}
	gsPrepend(&GsWSMATRIX, &t)
	GsWSMATRIX = t
}

// GsSetRefView2 makes the world-to-screen matrix for a viewpoint looking
// at a reference point: a roll about z, a tilt about x and a turn about
// y, then the viewpoint moved to the origin. It returns 1 when the two
// points are the same, and 0 otherwise.
//
// libps 0x80022B44, 980 bytes
//
//export GsSetRefView2
func GsSetRefView2(pv *GsRVIEW2) int32 {
	GsWSMATRIX = GsIDMATRIX2
	gsRotateZ(&GsWSMATRIX, -pv.Rz)
	var v [6]int32
	gsScaleView(pv, &v)
	dx, dy, dz := v[3]-v[0], v[4]-v[1], v[5]-v[2]
	d := SquareRoot0(dx*dx + dy*dy + dz*dz)
	if d == 0 {
		return 1
	}
	sx := -(((v[1] - v[4]) << 12) / d)
	dxz := SquareRoot0(dx*dx + dz*dz)
	cx := (dxz << 12) / d
	var m MATRIX
	gsAxisMatrix(&m, int16(sx), int16(cx), 'x')
	MulMatrix(&GsWSMATRIX, &m)
	if dxz != 0 {
		sy := -(((v[3] - v[0]) << 12) / dxz)
		cy := ((v[5] - v[2]) << 12) / dxz
		gsAxisMatrix(&m, int16(sy), int16(cy), 'y')
		MulMatrix(&GsWSMATRIX, &m)
	}
	vp := [3]int32{-pv.Vpx, -pv.Vpy, -pv.Vpz}
	applyMatrixLV(&GsWSMATRIX, &vp, &GsWSMATRIX.T)
	if pv.Super != nil {
		gsViewInSuper(pv.Super)
	}
	gsWSSaved = GsWSMATRIX
	return 0
}

// GsSetView2 takes pv's matrix as the world-to-screen matrix, from its
// coordinate system into the world. It returns 0.
//
// libps 0x800231B0, 356 bytes
//
//export GsSetView2
func GsSetView2(pv *GsVIEW2) int32 {
	GsWSMATRIX = pv.View
	if pv.Super != nil {
		gsViewInSuper(pv.Super)
	}
	gsWSSaved = GsWSMATRIX
	return 0
}

// gsCoordStack is the chain of coordinate systems GsGetLw walks up, from
// the one asked about to the world.
//
// libps 0x800658D4, 400 bytes
var gsCoordStack [100]*GsCOORDINATE2

// gsWorkMatrix makes out the matrix from coord to the world, as GsGetLw
// does, keeping each system's work matrix up to date on the way: a
// system whose flag is this frame's swap count is taken as is, one whose
// flag is 0 has changed, and the rest hold what an earlier frame made.
// The chain is walked up to the world or to the first system made this
// frame, and the work matrices composed back down from there; a chain
// with nothing changed below a stale world gives the lowest work matrix
// unchanged.
//
// libps 0x80023314, 708 bytes
func gsWorkMatrix(coord *GsCOORDINATE2, out *MATRIX) {
	if coord == nil {
		// A C program may ask for WORLD, which is NULL, and the original
		// then read, and wrote, a system at address 0: the kernel's first
		// words, which it took as they were. Go would stop on the nil, so
		// it is given the same memory through its cached mirror.
		coord = (*GsCOORDINATE2)(unsafe.Pointer(uintptr(0x80000000)))
	}
	depth := 0
	changed := 100
	for c := coord; ; c = c.Super {
		gsCoordStack[depth] = c
		if c.Super == nil {
			if c.Flg == PSDCNT || c.Flg == 0 {
				c.Workm = c.Coord
				*out = c.Workm
				c.Flg = PSDCNT
				break
			}
			if changed == 100 {
				*out = gsCoordStack[0].Workm
				depth = 0
				break
			}
			depth = changed + 1
			*out = gsCoordStack[depth].Workm
			break
		}
		if c.Flg == PSDCNT {
			*out = c.Workm
			break
		}
		if c.Flg == 0 {
			changed = depth
		}
		depth++
	}
	for ; depth > 0; depth-- {
		c := gsCoordStack[depth-1]
		gsCompose(out, &c.Coord)
		c.Workm = *out
		c.Flg = PSDCNT
	}
}

// GsGetLw works out the matrix from the coordinate system m to the
// world into out.
//
// libps 0x80023314, 708 bytes
//
//export GsGetLw
func GsGetLw(m *GsCOORDINATE2, out *MATRIX) { gsWorkMatrix(m, out) }

// GsGetLs works out the matrix from the coordinate system m to the
// screen into out: to the world, then the world-to-screen matrix.
//
// libps 0x800235D8, 724 bytes
//
//export GsGetLs
func GsGetLs(m *GsCOORDINATE2, out *MATRIX) {
	gsWorkMatrix(m, out)
	gsPrepend(&GsWSMATRIX, out)
}

// GsGetLws works out both matrices from the coordinate system m: to the
// world into outw, and to the screen into outs.
//
// libps 0x800238AC, 800 bytes
//
//export GsGetLws
func GsGetLws(m *GsCOORDINATE2, outw, outs *MATRIX) {
	gsWorkMatrix(m, outw)
	*outs = *outw
	gsPrepend(&GsWSMATRIX, outs)
}

// GsSetLsMatrix loads m into the GTE as the matrix to draw with.
//
// libps 0x80021A90, 48 bytes
//
//export GsSetLsMatrix
func GsSetLsMatrix(m *MATRIX) {
	setRotMatrix(m)
	setTransMatrix(m)
}

// GsSetLightMatrix loads the light directions into the GTE for a model
// drawn with the local-to-world matrix m: the directions turned by it,
// with the GTE's own matrices kept.
//
// libps 0x80021AC0, 156 bytes
//
//export GsSetLightMatrix
func GsSetLightMatrix(m *MATRIX) {
	l := GsLIGHTWSMATRIX
	PushMatrix()
	MulMatrix(&l, m)
	PopMatrix()
	setLightMatrix(&l)
}

// GsSetFlatLight sets light id, 0 to 2: its direction, normalized, as a
// row of the light matrix, and its color as a column of the color
// matrix, which goes to the GTE. It returns -1 for a direction of no
// length, and 0 otherwise.
//
// libps 0x80021F90, 1244 bytes
//
//export GsSetFlatLight
func GsSetFlatLight(id int32, lt *GsF_LIGHT) int32 {
	l := GsLIGHTWSMATRIX
	c := gsColorMatrix
	d := SquareRoot0(lt.Vx*lt.Vx + lt.Vy*lt.Vy + lt.Vz*lt.Vz)
	if d == 0 {
		return -1
	}
	if id >= 0 && id <= 2 {
		l.M[id] = [3]int16{int16((-lt.Vx << 12) / d), int16((-lt.Vy << 12) / d), int16((-lt.Vz << 12) / d)}
		c.M[0][id] = int16(int32(lt.R) << 12 / 255)
		c.M[1][id] = int16(int32(lt.G) << 12 / 255)
		c.M[2][id] = int16(int32(lt.B) << 12 / 255)
	}
	GsLIGHTWSMATRIX = l
	gsColorMatrix = c
	setColorMatrix(&c)
	return 0
}

// GsSetAmbient sets the light from everywhere, each component up to
// 4095, of which the GTE keeps the top eight bits.
//
// libps 0x80022600, 40 bytes
//
//export GsSetAmbient
func GsSetAmbient(r, g, b int32) { setBackColor(r>>4, g>>4, b>>4) }

// GsSetLightMode sets how models are lit: 0 without fog, 1 with it, and
// 2 and 3 as 1.
//
// libps 0x80022524, 140 bytes
//
//export GsSetLightMode
func GsSetLightMode(mode int32) {
	if mode < 0 || mode > 3 {
		kprintf("not supported light mode %d\n", mode)
		return
	}
	gsLightMode = mode
}

// GsSetFogParam sets the fog: its depth cue and its color.
//
// libps 0x800225B0, 80 bytes
//
//export GsSetFogParam
func GsSetFogParam(fog *GsFOGPARAM) {
	setDQA(int32(fog.Dqa))
	setDQB(fog.Dqb)
	setFarColor(int32(fog.Rfc), int32(fog.Gfc), int32(fog.Bfc))
}

// GsScaleScreen scales the world-to-screen matrix by scale, in 4.12, from
// what the viewpoint made it: the rotation's columns, and the
// translation, each component rounded towards zero.
//
// libps 0x80023BCC, 296 bytes
//
//export GsScaleScreen
func GsScaleScreen(scale *SVECTOR) {
	v := VECTOR{Vx: int32(scale.Vx), Vy: int32(scale.Vy), Vz: int32(scale.Vz)}
	GsWSMATRIX = gsWSSaved
	ScaleMatrix(&GsWSMATRIX, &v)
	s := [3]int32{v.Vx, v.Vy, v.Vz}
	for i := range GsWSMATRIX.T {
		p := s[i] * GsWSMATRIX.T[i]
		if p < 0 {
			p += 4095
		}
		GsWSMATRIX.T[i] = p >> 12
	}
}

// GsMapModelingData turns the offsets in a TMD, from its flags word on,
// into addresses: each model's vertices, normals and primitives, which
// are offsets from the object table. A file already mapped, its flags
// word's low bit set, is left alone.
//
// libps 0x80021EF8, 120 bytes
//
//export GsMapModelingData
func GsMapModelingData(p *uint32) {
	flags := unsafe.Slice(p, 2)
	if flags[0]&1 != 0 {
		return
	}
	flags[0] |= 1
	base := uint32(uintptr(unsafe.Add(unsafe.Pointer(p), 8)))
	n := int32(flags[1])
	for i := int32(0); i < n; i++ {
		obj := unsafe.Slice((*uint32)(unsafe.Add(unsafe.Pointer(p), 8+i*28)), 6)
		obj[0] += base
		obj[4] += base
		obj[2] += base
	}
}

// GsLinkObject4 ties objp to model n of the TMD whose object table is at
// tmd, after GsMapModelingData, and counts the model's primitives into
// runs of one kind, each run's length written over the first word of
// its first primitive, where GsSortObject4 reads it.
//
// libps 0x80023CF4, 356 bytes
//
//export GsLinkObject4
func GsLinkObject4(tmd uint32, objp *GsDOBJ2, n int32) {
	obj := (*uint32)(at(uintptr(tmd) + uintptr(n)*28))
	objp.Tmd = obj
	table := unsafe.Slice(obj, 6)
	p := (*uint32)(at(uintptr(table[4])))
	run := p
	count := uint16(0)
	var lastMode, lastFlag uint32
	for i := uint32(0); i < table[5]; i++ {
		hdr := *p
		mode, flag := hdr>>24, hdr>>16
		if lastMode != 0 && (mode != lastMode || flag&0xff != lastFlag&0xff) {
			*(*uint16)(unsafe.Pointer(run)) = count
			count = 0
			run = p
		}
		lastMode, lastFlag = mode, flag
		size := tmdPrimSize(mode&0xfd, flag&4 != 0)
		if size == 0 {
			kprintf("GPU CODE %02xH not assigned.\n", mode)
		}
		p = (*uint32)(unsafe.Add(unsafe.Pointer(p), size))
		count++
	}
	*(*uint16)(unsafe.Pointer(run)) = count
}

// tmdPrimSize is the size in bytes of a primitive of the given code, with
// the gradation flag for the flat and gouraud triangles with a color a
// vertex, or 0 for a code the library does not draw.
//
// libps 0x80045A68, the table GsLinkObject4 jumps through
func tmdPrimSize(code uint32, grd bool) uintptr {
	switch code {
	case 0x20:
		if grd {
			return 24
		}
		return 16
	case 0x21, 0x29:
		return 16
	case 0x24, 0x31, 0x38:
		return 24
	case 0x25, 0x34, 0x39:
		return 28
	case 0x28:
		return 20
	case 0x2c, 0x2d:
		return 32
	case 0x30:
		if grd {
			return 28
		}
		return 20
	case 0x35, 0x3c:
		return 36
	case 0x3d:
		return 44
	}
	return 0
}

// GsSortLine sorts a line into ot at pri.
//
// libps 0x800203B4, 216 bytes
//
//export GsSortLine
func GsSortLine(lp *GsLINE, ot *GsOT, pri uint16) {
	attr := lp.Attribute
	if int32(attr) < 0 {
		return
	}
	p := (*uint32)(unsafe.Pointer(GsOUT_PACKET_P))
	w := unsafe.Slice(p, 5)
	w[1] = 0xe1000200 | uint32(int32(attr)>>23)&0x60
	w[2] = uint32(lp.R) | uint32(lp.G)<<8 | uint32(lp.B)<<16 | (0x40|uint32(int32(attr)>>29)&2)<<24
	w[3] = uint32(uint16(lp.X0+gsOfsX)) | uint32(uint16(lp.Y0+gsOfsY))<<16
	w[4] = uint32(uint16(lp.X1+gsOfsX)) | uint32(uint16(lp.Y1+gsOfsY))<<16
	GsOUT_PACKET_P = (*PACKET)(unsafe.Pointer(sortPacket(p, ot, pri, 4)))
}

// GsSortGLine sorts a line shaded between two colors into ot at pri. The
// byte after the second color is left as it was, as the original left
// it.
//
// libps 0x80020520, 264 bytes
//
//export GsSortGLine
func GsSortGLine(lp *GsGLINE, ot *GsOT, pri uint16) {
	attr := lp.Attribute
	if int32(attr) < 0 {
		return
	}
	p := (*uint32)(unsafe.Pointer(GsOUT_PACKET_P))
	w := unsafe.Slice(p, 6)
	w[1] = 0xe1000200 | uint32(int32(attr)>>23)&0x60
	w[2] = uint32(lp.R0) | uint32(lp.G0)<<8 | uint32(lp.B0)<<16 | (0x50|uint32(int32(attr)>>29)&2)<<24
	w[3] = uint32(uint16(lp.X0+gsOfsX)) | uint32(uint16(lp.Y0+gsOfsY))<<16
	w[4] = w[4]&0xff000000 | uint32(lp.R1) | uint32(lp.G1)<<8 | uint32(lp.B1)<<16
	w[5] = uint32(uint16(lp.X1+gsOfsX)) | uint32(uint16(lp.Y1+gsOfsY))<<16
	GsOUT_PACKET_P = (*PACKET)(unsafe.Pointer(sortPacket(p, ot, pri, 5)))
}

// GsSortFastSprite sorts a sprite into ot at pri as a GPU sprite, with no
// rotation or scale and no check on the priority. A sprite of no width or
// height is left out.
//
// libps 0x80020DC0, 380 bytes
//
//export GsSortFastSprite
func GsSortFastSprite(sp *GsSPRITE, ot *GsOT, pri uint16) {
	attr := sp.Attribute
	if int32(attr) < 0 || sp.W == 0 || sp.H == 0 {
		return
	}
	p := (*uint32)(unsafe.Pointer(GsOUT_PACKET_P))
	w := unsafe.Slice(p, 6)
	w[1] = drawModeFor(attr, sp.Tpage)
	w[2] = 0x64000000 | attr>>5&0x02000000 | attr<<18&0x01000000 | uint32(sp.B)<<16 | uint32(sp.G)<<8 | uint32(sp.R)
	w[3] = uint32(uint16(sp.X+gsOfsX)) | uint32(uint16(sp.Y+gsOfsY))<<16
	w[4] = uint32(sp.U) | uint32(sp.V)<<8 | uint32(sp.Cy)<<22 | uint32(sp.Cx)<<12&0x3f0000
	w[5] = uint32(sp.W) | uint32(sp.H)<<16
	entry := (*uint32)(unsafe.Add(unsafe.Pointer(ot.Org), (int32(pri)-int32(ot.Offset))*4))
	w[0] = *entry + 0x05000000
	*entry = uint32(uintptr(unsafe.Pointer(p))) & 0xffffff
	GsOUT_PACKET_P = (*PACKET)(unsafe.Add(unsafe.Pointer(p), 24))
}

// GsSortOt sorts the whole of ot0 into ot1 at ot0's point, and returns
// ot1. ot0's chain is followed from its first entry to the last packet
// linked after it, which is taken as its end, so a packet sorted at
// ot0's first entry is left out, as the original left it.
//
// libps 0x800226A4, 176 bytes
//
//export GsSortOt
func GsSortOt(ot0, ot1 *GsOT) *GsOT {
	last := (*uint32)(unsafe.Pointer(ot0.Org))
	if *last&0xffffff != 0xffffff {
		next := last
		for {
			last = next
			next = (*uint32)(at(uintptr(*next & 0xffffff)))
			if *next&0xffffff == 0xffffff {
				break
			}
		}
	}
	entry := (*uint32)(unsafe.Add(unsafe.Pointer(ot1.Org), (int32(ot0.Point)-int32(ot1.Offset))*4))
	*last = *last&0xff000000 | *entry&0xffffff
	*entry = *entry&0xff000000 | uint32(uintptr(unsafe.Pointer(ot0.Tag)))&0xffffff
	return ot1
}
