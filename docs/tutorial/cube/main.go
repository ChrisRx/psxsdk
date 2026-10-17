//go:build yaroze

// Cube: a textured cube from the disc, turned with the directional buttons.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	screenW, screenH = 320, 240

	// The ordering table holds 1<<otLength depths; one object needs few.
	otLength  = 4
	packetMax = 4096

	// How far a held button turns the cube each frame, where 4096 is a
	// whole turn.
	step = 24
)

// The files on the disc, and the memory they are read into. The drive
// delivers whole 2048-byte sectors, so each buffer is a whole number of them,
// enough for its file.
var (
	model   = disc.Register("DATA/CUBE.TMD", "data/CUBE.TMD")
	texture = disc.Register("DATA/CUBE.TIM", "data/CUBE.TIM")

	modelData   [2048]uint32
	textureData [4096]uint32
)

var (
	ot         [2]yaroze.GsOT
	otTags     [2][1 << otLength]yaroze.GsOT_TAG
	packetArea [2][packetMax / 4]uint32

	view  yaroze.GsRVIEW2
	light yaroze.GsF_LIGHT
	cube  yaroze.GsDOBJ2
	coord yaroze.GsCOORDINATE2
	image yaroze.GsIMAGE

	// rotation is which way the cube faces, built up turn by turn; turned
	// and ls are the frame's working matrices.
	rotation, turned, ls yaroze.MATRIX

	padBuf0, padBuf1 *uint8
	pathBuf          [64]byte
	textBuf          [64]byte
)

// The directional buttons, as bits of what padRead returns.
const (
	padUp    = 1 << 12
	padRight = 1 << 13
	padDown  = 1 << 14
	padLeft  = 1 << 15
)

func main() {
	yaroze.SetVideoMode(yaroze.MODE_NTSC)
	yaroze.GsInitGraph(screenW, screenH, yaroze.GsNONINTER|yaroze.GsOFSGPU, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, screenH)
	yaroze.GsInit3D()
	for i := range ot {
		ot[i].Length = otLength
		ot[i].Org = &otTags[i][0]
	}
	yaroze.FntLoad(960, 256)
	// With GsOFSGPU the origin is the middle of the screen, for 3D's sake,
	// so the text window is placed from there.
	yaroze.FntOpen(-screenW/2+16, -screenH/2+16, 256, 64, 0, 512)
	yaroze.GetPadBuf(&padBuf0, &padBuf1)

	if !load(&model, &modelData[0]) || !load(&texture, &textureData[0]) {
		yaroze.FntPrint(yaroze.CString(textBuf[:], "could not read the disc"))
		for {
			yaroze.FntFlush(-1)
			yaroze.VSync(0)
		}
	}

	// The texture goes into video memory, at the place the file names and
	// the model's polygons point to, with its palette.
	yaroze.GsGetTimInfo(&textureData[1], &image)
	rect := yaroze.RECT{X: image.Px, Y: image.Py, W: int16(image.Pw), H: int16(image.Ph)}
	yaroze.LoadImage(&rect, image.Pixel)
	if image.Pmode&8 != 0 {
		rect = yaroze.RECT{X: image.Cx, Y: image.Cy, W: int16(image.Cw), H: int16(image.Ch)}
		yaroze.LoadImage(&rect, image.Clut)
	}
	yaroze.DrawSync(0)

	// The model's offsets become addresses now that it has one, and the
	// object is linked to the first model in the file, in its own
	// coordinate system.
	yaroze.GsMapModelingData(&modelData[1])
	yaroze.GsLinkObject4(uint32(uintptr(unsafe.Pointer(&modelData[3]))), &cube, 0)
	yaroze.GsInitCoordinate2(nil, &coord)
	cube.Coord2 = &coord

	// The camera: back from the origin and a little above it, looking at it.
	view = yaroze.GsRVIEW2{Vpx: 0, Vpy: -120, Vpz: -320}
	yaroze.GsSetProjection(240)
	yaroze.GsSetRefView2(&view)

	// One light from over the camera's shoulder, and some ambient light,
	// so that the faces turned away from it are not black.
	light = yaroze.GsF_LIGHT{Vx: 1, Vy: 1, Vz: 1, R: 0xc0, G: 0xc0, B: 0xc0}
	yaroze.GsSetFlatLight(0, &light)
	yaroze.GsSetAmbient(yaroze.ONE/4, yaroze.ONE/4, yaroze.ONE/4)
	yaroze.GsSetLightMode(0)

	// Facing straight ahead, with a little of the top and a side showing.
	rotation = yaroze.MATRIX{M: [3][3]int16{{yaroze.ONE, 0, 0}, {0, yaroze.ONE, 0}, {0, 0, yaroze.ONE}}}
	turn(&yaroze.SVECTOR{Vx: -step * 12, Vy: step * 12})

	hint := yaroze.CString(textBuf[:], "the directions turn the cube")
	for {
		pad := padRead()
		if pad&padUp != 0 {
			turn(&yaroze.SVECTOR{Vx: -step})
		}
		if pad&padDown != 0 {
			turn(&yaroze.SVECTOR{Vx: step})
		}
		if pad&padLeft != 0 {
			turn(&yaroze.SVECTOR{Vy: -step})
		}
		if pad&padRight != 0 {
			turn(&yaroze.SVECTOR{Vy: step})
		}
		coord.Coord = rotation
		coord.Flg = 0

		active := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&packetArea[active][0]))
		yaroze.GsClearOt(0, 0, &ot[active])

		yaroze.GsGetLs(&coord, &ls)
		yaroze.GsSetLightMatrix(&ls)
		yaroze.GsSetLsMatrix(&ls)
		yaroze.GsSortObject4(&cube, &ot[active], 14-otLength, scratch)

		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(60, 120, 120, &ot[active])
		yaroze.GsDrawOt(&ot[active])
		yaroze.FntPrint(hint)
		yaroze.FntFlush(-1)
	}
}

// turn rotates the cube by the angles given about the world's axes, which
// are the screen's: the new turn is applied after everything so far, so up
// always tips the top towards the camera and left always turns the left
// side away, however the cube already sits.
func turn(by *yaroze.SVECTOR) {
	var m yaroze.MATRIX
	yaroze.RotMatrix(by, &m)
	yaroze.MulMatrix0(&m, &rotation, &turned)
	rotation = turned
}

// scratch is the CPU's scratchpad, 1KB of fast memory that GsSortObject4
// uses as its work space.
var scratch = (*uint32)(unsafe.Pointer(uintptr(0x1f800000)))

// load reads a file off the disc into memory and waits for it, giving up
// after a few seconds so that a missing disc shows a message rather than
// hanging.
func load(f *disc.File, into *uint32) bool {
	if !f.Read(into, pathBuf[:]) {
		return false
	}
	for range 300 {
		if yaroze.CdReadSync(1, nil) <= 0 {
			return true
		}
		yaroze.VSync(0)
	}
	return false
}

// padRead returns the buttons held on the first controller, one bit each,
// from the buffers the library refreshes every frame.
func padRead() uint32 {
	if padBuf0 == nil {
		return 0
	}
	at := func(n uintptr) uint32 { return uint32(*(*uint8)(unsafe.Add(unsafe.Pointer(padBuf0), n))) }
	return ^(at(3) | at(2)<<8) & 0xffff
}
