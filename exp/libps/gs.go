//go:build yaroze

package libps

import "unsafe"

// The 2D library's core, as libgs had it: the screen, its two buffers
// and which is shown, the drawing and display environments, and the
// ordering tables.

// The library's variables under their C names, which the yaroze package
// reaches with go:extern.
//
//go:extern PSDOFSX
var PSDOFSX [2]int16 // libps 0x80062D04: each buffer's x in video memory

//go:extern PSDOFSY
var PSDOFSY [2]int16 // libps 0x80062D08

//go:extern POSITION
var POSITION struct{ Offx, Offy int16 } // libps 0x80063D3C: the screen offset

//go:extern GsOUT_PACKET_P
var GsOUT_PACKET_P *PACKET // libps 0x80063D40: where the next packet goes

//go:extern GsLIGHTWSMATRIX
var GsLIGHTWSMATRIX MATRIX // libps 0x80063E30

//go:extern GsDRAWENV
var GsDRAWENV DRAWENV // libps 0x80063E54

//go:extern GsDISPENV
var GsDISPENV DISPENV // libps 0x80063EB0

// DispEnv returns the display environment GsSwapDispBuff puts up each
// frame, for another package, whose reference to the variable itself
// the compiler would give the Go name of rather than the C one.
func DispEnv() *DISPENV { return &GsDISPENV }

// Origin returns the screen offset, where the origin is on the picture,
// for another package, as DispEnv does.
func Origin() (x, y int16) { return POSITION.Offx, POSITION.Offy }

//go:extern GsIDMATRIX
var GsIDMATRIX MATRIX // libps 0x80063F0C

//go:extern GsLSMATRIX
var GsLSMATRIX MATRIX // libps 0x80063F2C

//go:extern GsWSMATRIX
var GsWSMATRIX MATRIX // libps 0x80063F4C

//go:extern GsIDMATRIX2
var GsIDMATRIX2 MATRIX // libps 0x80063F74: the identity with the aspect ratio in it

//go:extern CLIP2
var CLIP2 RECT // libps 0x80063F9C: the drawing area, before the buffer's offset

//go:extern PSDCNT
var PSDCNT uint32 // libps 0x800661E0: how many swaps so far, never 0 after the first

//go:extern PSDIDX
var PSDIDX int16 // libps 0x800661E4: the buffer shown, 0 or 1

var (
	gsOfsX, gsOfsY int16    // libps 0x80063ECC and 0x80063ED0: the GTE's offset in effect
	gsColorMatrix  MATRIX   // libps 0x80063ED4: the light colors, as GsSetFlatLight builds them
	screenW        uint32   // libps 0x80063F04
	screenH        uint32   // libps 0x80063F08
	gsDispArea     [4]int16 // libps 0x80063D34: x0, x1, y0, y1 when the GPU does not offset
	gsOfsGPU       int16    // libps 0x800661EC: GsOFSGPU, the GPU applies the offset
	// clearPackets are the fills GsSortClear sorts, one per buffer.
	clearPackets [2]fillPacket // libps 0x8004FD4C
)

// A fillPacket is a GPU fill of a rectangle: its tag, the color with the
// command 2, the position and the size.
type fillPacket struct {
	Tag  uint32
	R    uint8
	G    uint8
	B    uint8
	Code uint8
	X    int16
	Y    int16
	W    int16
	H    int16
}

// GsInitGraph sets the screen up: x by y, interlaced with bit 0 of
// intmode and offset by the GPU with bit 2, dithered with dith, and in
// 24-bit color with varmmode.
//
// libps 0x80020F3C, 120 bytes
//
//export GsInitGraph
func GsInitGraph(x, y, intmode, dith, varmmode uint16) {
	gsInitEnvs(x, y, intmode, dith, varmmode)
	gsInitGeom()
	PSDIDX = 0
	gsInitScreen(x, y)
	GsSetDrawBuffClip()
	GsSetDrawBuffOffset()
}

// gsInitEnvs resets the GPU and puts the first drawing and display
// environments.
//
// libps 0x80020FB4, 344 bytes
func gsInitEnvs(x, y, intmode, dith, varmmode uint16) {
	ResetGraph(0)
	GsDRAWENV.Ofs = [2]int16{}
	GsDRAWENV.Tw = RECT{}
	GsDRAWENV.Tpage = 0
	GsDRAWENV.Dtd = uint8(dith)
	GsDRAWENV.Dfe = 0
	GsDRAWENV.Isbg = 0
	PutDrawEnv(&GsDRAWENV)
	GsDISPENV.Disp = RECT{0, 0, int16(x), int16(y)}
	GsDISPENV.Screen = RECT{}
	if GetVideoMode() == 1 {
		GsDISPENV.Screen.Y = 24
		GsDISPENV.Pad0 = 1
	}
	GsDISPENV.Isinter = uint8(intmode & 1)
	gsOfsGPU = int16(intmode & 4)
	GsDISPENV.Isrgb24 = uint8(varmmode)
	PutDispEnv(&GsDISPENV)
}

// gsInitGeom sets the GTE up with no offset and black fog.
//
// libps 0x80022930, 76 bytes
func gsInitGeom() {
	initGeom()
	setFarColor(0, 0, 0)
	setGeomOffset(0, 0)
	gsOfsX, gsOfsY = 0, 0
}

// gsInitScreen records the screen's size and sets the matrices, the
// offsets, the clip and the clear packets from it.
//
// libps 0x80021184, 636 bytes
func gsInitScreen(w, h uint16) {
	screenW, screenH = uint32(w), uint32(h)
	aspect := (int32(screenH) << 14) / int32(screenW)
	GsIDMATRIX = MATRIX{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}}
	GsIDMATRIX2 = GsIDMATRIX
	GsLIGHTWSMATRIX = GsIDMATRIX
	GsLIGHTWSMATRIX.M[0][0], GsLIGHTWSMATRIX.M[1][1], GsLIGHTWSMATRIX.M[2][2] = 0, 0, 0
	gsColorMatrix = GsLIGHTWSMATRIX
	gsDispArea = [4]int16{}
	POSITION.Offx, POSITION.Offy = 0, 0
	CLIP2.Y = 0
	GsIDMATRIX2.M[1][1] = int16(aspect / 3)
	CLIP2.X = 0
	CLIP2.W, CLIP2.H = int16(w), int16(h)
	for i := range clearPackets {
		clearPackets[i].Tag = 3 << 24
		clearPackets[i].Code = 2
	}
	PSDCNT = 1
}

// GsDefDispBuff says where in video memory the two buffers are: the
// first at x0, y0 and the second at x1, y1.
//
// libps 0x80021DE0, 160 bytes
//
//export GsDefDispBuff
func GsDefDispBuff(x0, y0, x1, y1 uint16) {
	PSDOFSX = [2]int16{int16(x0), int16(x1)}
	PSDOFSY = [2]int16{int16(y0), int16(y1)}
	if gsOfsGPU != 0 {
		gsDispArea = [4]int16{}
	} else {
		gsDispArea = [4]int16{int16(x0), int16(x1), int16(y0), int16(y1)}
	}
	GsSetDrawBuffClip()
	GsSetDrawBuffOffset()
}

// GsGetActiveBuff is which buffer is shown, 0 or 1.
//
// libps 0x8002153C, 16 bytes
//
//export GsGetActiveBuff
func GsGetActiveBuff() int32 { return int32(PSDIDX) }

// GsSwapDispBuff shows the buffer that was drawn and makes the other the
// one to draw.
//
// libps 0x80021920, 176 bytes
//
//export GsSwapDispBuff
func GsSwapDispBuff() {
	GsDISPENV.Disp.X = PSDOFSX[PSDIDX]
	GsDISPENV.Disp.Y = PSDOFSY[PSDIDX]
	PutDispEnv(&GsDISPENV)
	SetDispMask(1)
	if PSDCNT == 0 {
		PSDCNT = 1
	} else {
		PSDCNT++
	}
	if PSDIDX == 0 {
		PSDIDX = 1
	} else {
		PSDIDX = 0
	}
	GsSetDrawBuffClip()
	GsSetDrawBuffOffset()
	pace(paceGsSwapDisp)
}

// GsSetWorkBase says where the packets of the sorts that follow go.
//
// libps 0x80022A60, 16 bytes
//
//export GsSetWorkBase
func GsSetWorkBase(p *PACKET) {
	GsOUT_PACKET_P = p
	pace(paceGsSetWorkBase)
}

// GsClearOt clears an ordering table, recording offset and point in it,
// and points its Tag at its last entry, the first drawn.
//
// libps 0x8002264C, 88 bytes
//
//export GsClearOt
func GsClearOt(offset, point uint16, ot *GsOT) {
	ot.Offset = uint32(offset)
	ot.Point = uint32(point)
	ot.Tag = (*GsOT_TAG)(unsafe.Add(unsafe.Pointer(ot.Org), 4<<ot.Length-4))
	ClearOTagR((*uint32)(unsafe.Pointer(ot.Org)), 1<<ot.Length)
	pace(paceGsClearOt)
}

// GsDrawOt draws an ordering table.
//
// libps 0x80022628, 36 bytes
//
//export GsDrawOt
func GsDrawOt(ot *GsOT) {
	DrawOTag((*uint32)(unsafe.Pointer(ot.Tag)))
	pace(paceGsDrawOt)
}

// GsSortClear sorts a clear of the buffer being drawn to the color
// behind everything in the table.
//
// libps 0x80021400, 316 bytes
//
//export GsSortClear
func GsSortClear(r, g, b uint8, ot *GsOT) {
	pace(paceGsSortClear)
	p := &clearPackets[PSDIDX]
	p.R, p.G, p.B = r, g, b
	p.X = PSDOFSX[PSDIDX]
	p.Y = PSDOFSY[PSDIDX]
	p.H = int16(screenH)
	if GsDISPENV.Isrgb24 != 0 {
		p.W = int16((int32(screenW) * 3) / 2)
	} else {
		p.W = int16(screenW)
	}
	addPrim((*uint32)(unsafe.Pointer(ot.Tag)), &p.Tag)
}

// GsSetDrawBuffClip sets the drawing area to the clip at the buffer
// being drawn.
//
// libps 0x80021660, 144 bytes
//
//export GsSetDrawBuffClip
func GsSetDrawBuffClip() {
	GsDRAWENV.Clip.W = CLIP2.W
	GsDRAWENV.Clip.H = CLIP2.H
	GsDRAWENV.Clip.X = CLIP2.X + PSDOFSX[PSDIDX]
	GsDRAWENV.Clip.Y = CLIP2.Y + PSDOFSY[PSDIDX]
	PutDrawEnv(&GsDRAWENV)
}

// GsSetDrawBuffOffset sets the drawing offset to the screen offset at the
// buffer being drawn: in the GPU when it applies offsets, in the GTE
// otherwise, for the buffer the other index names.
//
// libps 0x8002154C, 276 bytes
//
//export GsSetDrawBuffOffset
func GsSetDrawBuffOffset() {
	if gsOfsGPU != 0 {
		GsDRAWENV.Ofs[0] = POSITION.Offx + PSDOFSX[PSDIDX]
		GsDRAWENV.Ofs[1] = POSITION.Offy + PSDOFSY[PSDIDX]
		gsOfsX, gsOfsY = 0, 0
		PutDrawEnv(&GsDRAWENV)
		return
	}
	other := 1 - PSDIDX
	x := POSITION.Offx + PSDOFSX[other]
	y := POSITION.Offy + PSDOFSY[other]
	setGeomOffset(int32(x), int32(y))
	gsOfsX, gsOfsY = x, y
}
