// A textured sheet morphing into a cylinder, a torus and a sphere, vertex by
// vertex, as it tumbles.
//
// A port of Sony's Net Yaroze demo "morph" (S. Ashley, 1997): a textured
// sheet that turns into a cylinder, a torus, a sphere and back into a
// sheet, at 640 by 480. The sheet is a TMD built in memory at startup, a
// grid of textured quads over a 256 by 256 picture with lighting off and
// both faces drawn, and every frame each of its vertices moves one unit
// towards the matching vertex of the shape it is becoming, so that the
// shape changes over a few hundred frames while the whole thing tumbles.
// The library's font prints the count of horizontal syncs a frame takes.
//
// Differences from the original:
//
//   - The sheet has a coordinate system of its own. The original points it
//     at WORLD, the library's nil coordinate system, and writes its
//     transformation through that pointer, which in C silently overwrites
//     the kernel's memory at address 0.
//   - The quads' headers carry the lengths the TMD format gives a textured
//     quad without lighting, seven words in and nine out. The original
//     writes six and seven.
//   - The flat sheet's vertices are computed at startup. The other three
//     shapes are the tables the original's mesh generator wrote.
//
// Nothing allocates after startup.
//
// # Controls
//
// Start and Select together quit.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	packetMax = 2500 * 24 // GPU packet space per buffer
	lengthOT  = 12

	// The room the sheet's TMD is built in.
	sheetWords = 64 * 1024 / 4

	// The grid, in quads across and down, and the length of its vertex
	// tables.
	gridw    = 20
	gridh    = 16
	meshSize = 3 * (gridw + 1) * (gridh + 1)
)

// F_4T_NL is the primitive mode of a flat textured quad without lighting,
// and the flag that says so, with the lengths of its body and of the GPU
// packet it becomes.
const (
	f4TNL     = 0x2d
	noLight   = 1
	f4TNLIlen = 7
	f4TNLOlen = 9
)

// TMD_F_4T_NL is the body of a flat textured quad without lighting: the
// texture coordinates of its corners with the palette and texture page,
// one color, and the four vertices.
type TMD_F_4T_NL struct {
	U0, V0 uint8
	CBA    uint16
	U1, V1 uint8
	TSB    uint16
	U2, V2 uint8
	Pad0   uint16
	U3, V3 uint8
	Pad1   uint16

	R0, G0, B0, Pad2 uint8
	Vert0, Vert1     uint16
	Vert2, Vert3     uint16
}

// The picture, read into the program's own memory in a buffer of whole
// sectors, and the memory the sheet's TMD is built in.
var (
	pictureData [(131092 + 2047) / 2048 * 512]uint32
	sheetData   [sheetWords]uint32

	pictureFile = disc.Register("ASSETS/SB.TIM", "assets/SB.TIM").Into(pictureData[:])
	files       = []*disc.File{&pictureFile}

	pictureTIM = yaroze.TIM(pictureData[:])
	pictureTMD = yaroze.TMD(sheetData[:])
)

var (
	ot              [2]yaroze.GsOT
	zTable          [2][1 << lengthOT]yaroze.GsOT_TAG
	gpuOutputPacket [2][packetMax / 4]uint32
	buffIdx         int32

	mainView       yaroze.GsVIEW2
	mainViewMatrix yaroze.MATRIX

	padStatus yaroze.Buttons

	pictureOBJ   yaroze.GsDOBJ2
	pictureCoord yaroze.GsCOORDINATE2

	// flat is the sheet the picture starts as and returns to.
	flat [meshSize]int16

	frameNumber int32

	lineBuf [32]byte
	line    yaroze.Buffer
)

func main() {
	line.Init(lineBuf[:])

	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, yaroze.GsOFSGPU, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.GsInit3D()

	mainViewMatrix = yaroze.IDMATRIX

	for i := range 2 {
		ot[i].Length = lengthOT
		ot[i].Org = &zTable[i][0]
	}

	yaroze.GsSetProjection(1000)

	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(-280, -200, 128, 16, 0, 32)

	padStatus = 0
	yaroze.PadInit()

	if missing := demo.Load(files, nil); missing != "" {
		fail("could not read ", missing)
		return
	}

	pictureTIM.Load()

	makeFlat()
	pvert := makeBigTMDTexQuad(pictureTMD, pictureTIM)
	initialiseTMD(pictureTMD, 0, &pictureOBJ)

	var angle int32 // amount of rotation
	var state int32 // where in the cycle of shapes

	for {
		buffIdx = yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&gpuOutputPacket[buffIdx][0]))
		yaroze.GsClearOt(0, 0, &ot[buffIdx])

		rotate := yaroze.SVECTOR{Vx: 0, Vy: int16(angle), Vz: int16(angle / 2)}
		translate := yaroze.VECTOR{Vx: 0, Vy: 0, Vz: 2000}
		yaroze.RotMatrix(&rotate, &mainViewMatrix)
		yaroze.TransMatrix(&mainViewMatrix, &translate)

		mainView.View = mainViewMatrix
		mainView.Super = yaroze.WORLD
		yaroze.GsSetView2(&mainView)

		switch {
		case state < 500:
			changeTo(pvert, &cylinder)
		case state < 1000:
			changeTo(pvert, &torus)
		case state < 1500:
			changeTo(pvert, &sphere)
		case state < 2000:
			changeTo(pvert, &flat)
		case state > 2500:
			state = 0
		}

		drawObject(&pictureOBJ, &yaroze.IDMATRIX, yaroze.WORLD, &ot[buffIdx])

		yaroze.DrawSync(0)
		hsync := yaroze.VSync(1)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 0, &ot[buffIdx])
		yaroze.GsDrawOt(&ot[buffIdx])

		line.Reset()
		line.WriteString("hsync ").WriteInt(int(hsync)).WriteString("\n").Print()
		yaroze.FntFlush(-1)

		state++
		angle += 32
		frameNumber++

		padStatus = yaroze.PadRead()
		if padStatus&yaroze.PADselect != 0 && padStatus&yaroze.PADstart != 0 {
			break
		}
	}
	yaroze.ResetGraph(1)
}

// makeFlat computes the flat sheet: grid points 32 units apart, centered.
func makeFlat() {
	p := 0
	for j := 0; j <= gridh; j++ {
		for i := 0; i <= gridw; i++ {
			flat[p] = int16((i - gridw/2) * 32)
			flat[p+1] = int16((j - gridh/2) * 32)
			flat[p+2] = 0
			p += 3
		}
	}
}

// initialiseTMD ties a handle to an object of the TMD, giving it a
// coordinate system of its own.
func initialiseTMD(tmd yaroze.TMD, objNum int32, obj *yaroze.GsDOBJ2) {
	tmd.Map()
	tmd.Link(obj, objNum)
	pictureCoord.Init(yaroze.WORLD)
	obj.Coord2 = &pictureCoord
}

// drawObject puts an object into the ordering table transformed by tmat
// within the coordinate system super.
func drawObject(obj *yaroze.GsDOBJ2, tmat *yaroze.MATRIX, super *yaroze.GsCOORDINATE2, ot *yaroze.GsOT) {
	obj.Coord2.Coord = *tmat
	obj.Coord2.Super = super
	obj.Coord2.Flg = 0

	var coordMatrix yaroze.MATRIX
	yaroze.GsGetLs(obj.Coord2, &coordMatrix)
	yaroze.GsSetLsMatrix(&coordMatrix)
	yaroze.GsSortObject4(obj, ot, 14-int32(ot.Length), yaroze.Scratch())
}

// put writes v at the cursor and moves it on, returning where v went.
func put[T any](cursor *uintptr, v T) *T {
	p := (*T)(yaroze.Pointer(*cursor))
	*p = v
	*cursor += unsafe.Sizeof(v)
	return p
}

// makeBigTMDTexQuad builds the sheet in tmd: a TMD of one object, a grid
// of textured quads without lighting wearing the TIM, which must be in
// video memory already, each quad drawn from both sides by a second
// primitive with its corners the other way round. It starts as the flat
// sheet, and returns where the vertex table went. The header says its
// offsets are addresses already, so GsMapModelingData leaves it alone.
func makeBigTMDTexQuad(tmd yaroze.TMD, tim yaroze.TIM) uintptr {
	addr := uintptr(unsafe.Pointer(&tmd[0]))
	image := tim.Image()

	pmode := image.Pmode & 3
	cba := uint16(image.Cy)<<6 + uint16(image.Cx)>>4
	tsb := uint16(image.Px)>>6 + uint16(image.Py)>>10 + uint16(pmode)<<7

	// Texels to a pixel of video memory, by the color depth.
	var pixScale int32
	switch pmode {
	case 0:
		pixScale = 4
	case 1:
		pixScale = 2
	default:
		pixScale = 1
	}
	texw := int32(image.Pw) * pixScale
	texh := int32(image.Ph)

	cursor := addr
	put(&cursor, demo.TMD_HDR{ID: 0x41, Flag: 1, NObjs: 1})
	objTable := put(&cursor, demo.TMD_OBJ{})

	primTable := cursor
	for j := int32(0); j < gridh; j++ {
		for i := int32(0); i < gridw; i++ {
			u0, v0 := uint8(i*texw/gridw), uint8(j*texh/gridh)
			u1, v1 := uint8((i+1)*texw/gridw-1), uint8(j*texh/gridh)
			u2, v2 := uint8(i*texw/gridw), uint8((j+1)*texh/gridh-1)
			u3, v3 := uint8((i+1)*texw/gridw-1), uint8((j+1)*texh/gridh-1)

			vcnt := uint16(j*(gridw+1) + i)

			put(&cursor, demo.PRIM_HDR{Olen: f4TNLOlen, Ilen: f4TNLIlen, Flag: noLight, Mode: f4TNL})
			put(&cursor, TMD_F_4T_NL{
				CBA: cba, TSB: tsb,
				U0: u0, V0: v0, U1: u1, V1: v1, U2: u2, V2: v2, U3: u3, V3: v3,
				R0: 127, G0: 127, B0: 127,
				Vert0: vcnt, Vert1: vcnt + 1, Vert2: vcnt + (gridw + 1), Vert3: vcnt + (gridw + 1) + 1,
			})

			// The second side.
			put(&cursor, demo.PRIM_HDR{Olen: f4TNLOlen, Ilen: f4TNLIlen, Flag: noLight, Mode: f4TNL})
			put(&cursor, TMD_F_4T_NL{
				CBA: cba, TSB: tsb,
				U0: u1, V0: v1, U1: u0, V1: v0, U2: u3, V2: v3, U3: u2, V3: v2,
				R0: 127, G0: 127, B0: 127,
				Vert0: vcnt + 1, Vert1: vcnt, Vert2: vcnt + (gridw + 1) + 1, Vert3: vcnt + (gridw + 1),
			})
		}
	}

	vertTable := cursor
	p := 0
	for j := 0; j <= gridh; j++ {
		for i := 0; i <= gridw; i++ {
			put(&cursor, demo.TMD_VERT{Vx: flat[p], Vy: flat[p+1], Vz: flat[p+2]})
			p += 3
		}
	}
	normTable := cursor

	*objTable = demo.TMD_OBJ{
		VertTop: uint32(vertTable), NVert: (gridw + 1) * (gridh + 1),
		NormTop: uint32(normTable), NNorm: 0,
		PrimTop: uint32(primTable), NPrim: 2 * gridw * gridh,
		Scale: 0,
	}
	return vertTable
}

// changeTo moves each vertex of the table at pvert one unit along each
// axis towards the matching vertex of shape.
func changeTo(pvert uintptr, shape *[meshSize]int16) {
	p := 0
	for j := 0; j <= gridh; j++ {
		for i := 0; i <= gridw; i++ {
			v := (*demo.TMD_VERT)(yaroze.Pointer(pvert))
			v.Vx = towards(v.Vx, shape[p])
			v.Vy = towards(v.Vy, shape[p+1])
			v.Vz = towards(v.Vz, shape[p+2])
			p += 3
			pvert += unsafe.Sizeof(demo.TMD_VERT{})
		}
	}
}

// towards moves x one unit closer to target.
func towards(x, target int16) int16 {
	switch {
	case x < target:
		return x + 1
	case x > target:
		return x - 1
	}
	return x
}

// fail shows what went wrong until the program is quit.
func fail(what, detail string) {
	for yaroze.PadRead()&(yaroze.PADstart|yaroze.PADselect) != yaroze.PADstart|yaroze.PADselect {
		buff := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&gpuOutputPacket[buff][0]))
		yaroze.GsClearOt(0, 0, &ot[buff])
		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 0, &ot[buff])
		yaroze.GsDrawOt(&ot[buff])
		line.Reset()
		line.WriteString(what).WriteString(detail).Print()
		yaroze.FntFlush(-1)
	}
	yaroze.ResetGraph(3)
}
