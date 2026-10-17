// The flame demo, in Go.
//
// A port of Sony's Net Yaroze demo "flame" (S. Ashley, 1997): a sheet of
// fire, drawn at 640 by 480. A grid of heat values is seeded along its
// bottom edge with random hot spots every frame, blurred with its
// neighbors, and scrolled upwards, so that the heat rises, spreads and
// dies out. The grid is a single TMD built in memory at startup, a flat
// sheet of gouraud quads with lighting switched off, and every frame the
// four corner colors of each quad are rewritten from the heat values
// through a palette running from black through red and yellow to white.
// The GTE and the 3D library then draw it like any other model, so the
// sheet can be turned and moved: the d-pad tilts it, L1 and L2 roll it,
// and R1 and R2 move it nearer and further. The library's font prints the
// count of horizontal syncs a frame takes. Start and Select together quit.
//
// Differences from the original:
//
//   - The heat grid is one row and one column larger than the original's,
//     which reads one past its edges in the blur and so takes whatever is
//     next to the array in memory for the heat beyond them. Here that heat
//     is zero.
//   - The sheet has a coordinate system of its own. The original points it
//     at WORLD, the library's nil coordinate system, and writes its
//     transformation through that pointer, which in C silently overwrites
//     the kernel's memory at address 0.
//
// Nothing allocates after startup.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	packetMax = 2500 * 24 // GPU packet space per buffer
	lengthOT  = 8

	// The room the sheet's TMD is built in.
	sheetWords = 64 * 1024 / 4

	// The heat grid, in quads across and down.
	mapx = 40
	mapy = 34
)

// The memory the sheet's TMD is built in.
var (
	sheetData [sheetWords]uint32
	lightTMD  = yaroze.TMD(sheetData[:])
)

// G_4_NL is the primitive mode of a gouraud quad without lighting, and the
// flag that says so.
const (
	g4NL     = 0x39
	noLight  = 1
	g4NLIlen = 6 // words in the primitive's body
	g4NLOlen = 8 // words in the GPU packet it becomes
)

// TMD_G_4_NL is the body of a gouraud quad without lighting: a color at
// each corner, then the four vertices.
type TMD_G_4_NL struct {
	R0, G0, B0, Mode uint8
	R1, G1, B1, Pad0 uint8
	R2, G2, B2, Pad1 uint8
	R3, G3, B3, Pad2 uint8
	Vert0, Vert1     uint16
	Vert2, Vert3     uint16
}

var (
	// fracmap is the heat at each grid point. It is a row and a column
	// larger than the quads need, so that the blur's reads past the edge
	// find zero.
	fracmap [mapx + 2][mapy + 2]uint8

	ot              [2]yaroze.GsOT
	zTable          [2][1 << lengthOT]yaroze.GsOT_TAG
	gpuOutputPacket [2][packetMax / 4]uint32
	buffIdx         int32

	mainView       yaroze.GsVIEW2
	mainViewMatrix yaroze.MATRIX

	padStatus uint32

	lightOBJ   yaroze.GsDOBJ2
	lightCoord yaroze.GsCOORDINATE2

	// colorMap turns a heat value into a color, in the order the primitive
	// stores them: red in the low byte, then green, then blue.
	colorMap [256]uint32

	frameNumber int32

	lineBuf [32]byte
	line    yaroze.Buffer
)

func main() {
	line.Init(lineBuf[:])
	yaroze.Srand(54634)

	yaroze.ResetGraph(0)
	padStatus = 0
	yaroze.PadInit()

	yaroze.GsInitGraph(640, 480, yaroze.GsOFSGPU, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.GsInit3D()

	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(-280, -200, 128, 16, 0, 32)

	for i := range 2 {
		ot[i].Length = lengthOT
		ot[i].Org = &zTable[i][0]
	}

	yaroze.GsSetProjection(1000)

	makeColorMap()
	makeBigTMDGouraudQuad(lightTMD)
	initialiseTMD(lightTMD, 0, &lightOBJ)
	sheetAddr := uintptr(unsafe.Pointer(&sheetData[0]))

	mainViewMatrix = yaroze.IDMATRIX
	translate := yaroze.VECTOR{Vx: 0, Vy: 0, Vz: 1000}
	yaroze.TransMatrix(&mainViewMatrix, &translate)

	// The first primitive: past the header and the object table.
	prims := sheetAddr + unsafe.Sizeof(demo.TMD_HDR{}) + unsafe.Sizeof(demo.TMD_OBJ{})

	for {
		buffIdx = yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&gpuOutputPacket[buffIdx][0]))
		yaroze.GsClearOt(0, 0, &ot[buffIdx])

		mainView.View = mainViewMatrix
		mainView.Super = yaroze.WORLD
		yaroze.GsSetView2(&mainView)

		fireGrid()
		alterColors(prims)
		drawObject(&lightOBJ, &yaroze.IDMATRIX, yaroze.WORLD, &ot[buffIdx])

		yaroze.DrawSync(0)
		hsync := yaroze.VSync(1)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 0, &ot[buffIdx])
		yaroze.GsDrawOt(&ot[buffIdx])

		padStatus = yaroze.PadRead()
		controls()

		line.Reset()
		line.WriteInt(int(hsync)).WriteString("\n").Print()
		yaroze.FntFlush(-1)
		frameNumber++

		if padStatus&yaroze.PADselect != 0 && padStatus&yaroze.PADstart != 0 {
			break
		}
	}
	yaroze.ResetGraph(1)
}

// makeColorMap fills the palette: black to red over the first sixteen
// values, red to yellow over the next, yellow to white over the next, and
// white from there on.
func makeColorMap() {
	for i := range colorMap {
		switch {
		case i < 16:
			colorMap[i] = uint32(i) << 4
		case i < 32:
			colorMap[i] = 0xff | uint32(i-16)<<12
		case i < 48:
			colorMap[i] = 0xffff | uint32(i-32)<<20
		default:
			colorMap[i] = 0xffffff
		}
	}
}

// initialiseTMD ties a handle to an object of the TMD, giving it a
// coordinate system of its own.
func initialiseTMD(tmd yaroze.TMD, objNum int32, obj *yaroze.GsDOBJ2) {
	tmd.Map()
	tmd.Link(obj, objNum)
	lightCoord.Init(yaroze.WORLD)
	obj.Coord2 = &lightCoord
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

// fireGrid moves the fire on a frame: hot spots along the bottom, a blur
// of every point with its eight neighbors, a scroll upwards, and the
// bottom row cleared for the next frame's hot spots.
func fireGrid() {
	for range 10 {
		fracmap[yaroze.Rand()%(mapx-1)+1][mapy] = uint8(yaroze.Rand())
	}

	for i := 1; i < mapx; i++ {
		r0 := int32(fracmap[i-1][0]) + int32(fracmap[i][0]) + int32(fracmap[i+1][0])
		r1 := int32(fracmap[i-1][1]) + int32(fracmap[i][1]) + int32(fracmap[i+1][1])
		r2 := int32(fracmap[i-1][2]) + int32(fracmap[i][2]) + int32(fracmap[i+1][2])
		for j := 1; j < mapy; j++ {
			fracmap[i][j] = uint8((r0 + r1 + r2) / 9)
			r0 = r1
			r1 = r2
			r2 = int32(fracmap[i-1][j+2]) + int32(fracmap[i][j+2]) + int32(fracmap[i+1][j+2])
		}
	}

	for i := 1; i < mapx; i++ {
		for j := 1; j < mapy; j++ {
			fracmap[i][j] = fracmap[i][j+1]
		}
	}

	for i := 0; i <= mapx; i++ {
		fracmap[i][mapy] = 0
	}
}

// controls turns and moves the view: the d-pad tilts it about x and y, L1
// and L2 roll it about z, and R1 and R2 move it along z.
func controls() {
	rotateBy := func(x, y, z int16) {
		rotate := yaroze.SVECTOR{Vx: x, Vy: y, Vz: z}
		var t1 yaroze.MATRIX
		yaroze.RotMatrix(&rotate, &t1)
		tmp := mainViewMatrix
		yaroze.MulMatrix0(&t1, &tmp, &mainViewMatrix)
	}
	if padStatus&yaroze.PADLleft != 0 {
		rotateBy(0, 64, 0)
	}
	if padStatus&yaroze.PADLright != 0 {
		rotateBy(0, -64, 0)
	}
	if padStatus&yaroze.PADLup != 0 {
		rotateBy(-64, 0, 0)
	}
	if padStatus&yaroze.PADLdown != 0 {
		rotateBy(64, 0, 0)
	}
	if padStatus&yaroze.PADL1 != 0 {
		rotateBy(0, 0, 64)
	}
	if padStatus&yaroze.PADL2 != 0 {
		rotateBy(0, 0, -64)
	}
	if padStatus&yaroze.PADR1 != 0 {
		translate := yaroze.VECTOR{Vx: mainViewMatrix.T[0], Vy: mainViewMatrix.T[1], Vz: mainViewMatrix.T[2] - 64}
		yaroze.TransMatrix(&mainViewMatrix, &translate)
	}
	if padStatus&yaroze.PADR2 != 0 {
		translate := yaroze.VECTOR{Vx: mainViewMatrix.T[0], Vy: mainViewMatrix.T[1], Vz: mainViewMatrix.T[2] + 64}
		yaroze.TransMatrix(&mainViewMatrix, &translate)
	}
}

// put writes v at the cursor and moves it on, returning where v went.
func put[T any](cursor *uintptr, v T) *T {
	p := (*T)(yaroze.Pointer(*cursor))
	*p = v
	*cursor += unsafe.Sizeof(v)
	return p
}

// makeBigTMDGouraudQuad builds the sheet in tmd: a TMD of one object, a
// flat grid of gouraud quads with lighting off, mapx across and mapy-2
// down, 16 units a side, with no normals. The header says its offsets are
// addresses already, so GsMapModelingData leaves it alone.
func makeBigTMDGouraudQuad(tmd yaroze.TMD) {
	cursor := uintptr(unsafe.Pointer(&tmd[0]))
	put(&cursor, demo.TMD_HDR{ID: 0x41, Flag: 1, NObjs: 1})
	objTable := put(&cursor, demo.TMD_OBJ{})

	primTable := cursor
	for j := 0; j < mapy-2; j++ {
		for i := 0; i < mapx; i++ {
			vcnt := uint16(j*(mapx+1) + i)
			put(&cursor, demo.PRIM_HDR{Olen: g4NLOlen, Ilen: g4NLIlen, Flag: noLight, Mode: g4NL})
			put(&cursor, TMD_G_4_NL{
				Vert0: vcnt, Vert1: vcnt + 1,
				Vert2: vcnt + (mapx + 1), Vert3: vcnt + (mapx + 1) + 1,
			})
		}
	}

	vertTable := cursor
	for j := 0; j <= mapy-2; j++ {
		for i := 0; i <= mapx; i++ {
			put(&cursor, demo.TMD_VERT{Vx: int16((i - mapx/2) << 4), Vy: int16((j - mapy/2) << 4), Vz: 0})
		}
	}
	normTable := cursor

	*objTable = demo.TMD_OBJ{
		VertTop: uint32(vertTable), NVert: (mapx + 1) * (mapy - 1),
		NormTop: uint32(normTable), NNorm: 0,
		PrimTop: uint32(primTable), NPrim: mapx * (mapy - 2),
		Scale: 0,
	}
}

// alterColors writes each quad's corner colors from the heat at its four
// grid points. The first color's top byte is the primitive's mode, which
// has to be put back.
func alterColors(addr uintptr) {
	const stride = unsafe.Sizeof(demo.PRIM_HDR{}) + unsafe.Sizeof(TMD_G_4_NL{})
	for j := 0; j < mapy-2; j++ {
		for i := 0; i < mapx; i++ {
			body := (*[g4NLIlen]uint32)(yaroze.Pointer(addr + unsafe.Sizeof(demo.PRIM_HDR{})))
			body[0] = colorMap[fracmap[i][j]] | uint32(g4NL)<<24
			body[1] = colorMap[fracmap[i+1][j]]
			body[2] = colorMap[fracmap[i][j+1]]
			body[3] = colorMap[fracmap[i+1][j+1]]
			addr += stride
		}
	}
}
