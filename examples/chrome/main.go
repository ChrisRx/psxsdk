// The chrome sample, in Go.
//
// A port of Sony's Net Yaroze demo "chrome" (S. Ashley, 1997): a textured
// torus, turning, whose texture coordinates are worked out again every frame
// from its normals as the torus turns, so the texture reads as a reflection
// of the world rather than paint on the surface. That is chrome, or
// environment, mapping. It puts the GTE, the 3D library and lighting
// through their paces, which the check example does not touch.
//
// Differences from the original:
//
//   - The texture and model are read off the disc. On a Net Yaroze the
//     loader's batch script sent them from the PC to fixed addresses; they
//     are loaded to the same addresses.
//   - The torus has a coordinate system of its own. The original points the
//     object at WORLD, the library's nil coordinate system, and then writes
//     its transformation through that pointer every frame, which in C
//     silently overwrites the kernel's memory at address 0.
//   - A loading message, and a report if the data is missing, rather than a
//     black screen.
//
// Hold Start and Select together to quit.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	packetMax = 2500 * 24 // GPU packet space per buffer
	otLength  = 12        // ordering table size, as a power of two

	// Where the texture and the model are loaded.
	textureTIM = 0x80090000
	torusTMD   = 0x800a0000

	screenW, screenH = 640, 480
)

var files = []demo.File{
	{File: disc.Register("DATA/TEXTURE.TIM", "data/TEXTURE.TIM"), Addr: textureTIM},
	{File: disc.Register("DATA/TORUS.TMD", "data/TORUS.TMD"), Addr: torusTMD},
}

// Ordering tables and GPU packet space, one set per display buffer.
var (
	ot        [2]yaroze.GsOT
	zTable    [2][1 << otLength]yaroze.GsOT_TAG
	gpuPacket [2][packetMax]yaroze.PACKET
)

var (
	torusOBJ   yaroze.GsDOBJ2
	torusCoord yaroze.GsCOORDINATE2
	mainView   yaroze.GsVIEW2
)

// initialiseTMD maps a TMD's offsets to addresses and ties a handle to one
// of its objects, with a coordinate system of its own.
func initialiseTMD(addr uintptr, objNum int32, obj *yaroze.GsDOBJ2, coord *yaroze.GsCOORDINATE2) {
	// Past the ID word, then past the whole header.
	yaroze.GsMapModelingData(demo.At(addr + 4))
	yaroze.GsLinkObject4(uint32(addr+12), obj, objNum)
	yaroze.GsInitCoordinate2(demo.WORLD, coord)
	obj.Coord2 = coord
}

// drawObject places an object with a transformation, relative to the
// coordinate system super, and adds it to an ordering table.
func drawObject(obj *yaroze.GsDOBJ2, transform *yaroze.MATRIX, super *yaroze.GsCOORDINATE2, ot *yaroze.GsOT) {
	obj.Coord2.Coord = *transform
	obj.Coord2.Super = super
	obj.Coord2.Flg = 0 // moved

	var ls yaroze.MATRIX
	yaroze.GsGetLs(obj.Coord2, &ls) // local to screen, with perspective
	yaroze.GsSetLsMatrix(&ls)
	yaroze.GsSortObject4(obj, ot, 14-int32(ot.Length), demo.Scratch())
}

// changeTexture works out every quad's texture coordinates from its normals
// as the transformation turns them: the normal's x and y, from -ONE to ONE,
// become a texture coordinate from 0 to 255 about the middle. The torus is
// all gouraud-textured quads, which is what this assumes.
func changeTexture(addr uintptr, tm *yaroze.MATRIX) {
	o := demo.TMDObject(addr)
	for i := range o.NPrim {
		p := demo.Prim[demo.TMD_G_4T](o, i)
		p.U0, p.V0 = reflect(o, p.Norm0, tm)
		p.U1, p.V1 = reflect(o, p.Norm1, tm)
		p.U2, p.V2 = reflect(o, p.Norm2, tm)
		p.U3, p.V3 = reflect(o, p.Norm3, tm)
	}
}

// reflect is the texture coordinate for one normal, turned by tm.
func reflect(o *demo.TMD_OBJ, i uint16, tm *yaroze.MATRIX) (u, v uint8) {
	n := o.Norm(i)
	in := yaroze.VECTOR{Vx: int32(n.Nx), Vy: int32(n.Ny), Vz: int32(n.Nz)}
	var out yaroze.VECTOR
	yaroze.ApplyMatrixLV(tm, &in, &out)
	return uint8(128 - out.Vx>>5), uint8(128 - out.Vy>>5)
}

var (
	lineBuf [96]byte
	line    yaroze.Buffer
)

// message shows text until Start and Select are held, for when the demo
// cannot run.
func message(text, detail string) {
	for demo.PadRead()&(demo.PADstart|demo.PADselect) != demo.PADstart|demo.PADselect {
		frame()
		line.Reset()
		line.WriteString(text).WriteString(detail)
		line.Print()
		yaroze.FntFlush(-1)
	}
}

// frame draws a blank frame.
func frame() {
	buff := yaroze.GsGetActiveBuff()
	yaroze.GsSetWorkBase(&gpuPacket[buff][0])
	yaroze.GsClearOt(0, 0, &ot[buff])
	yaroze.DrawSync(0)
	yaroze.VSync(0)
	yaroze.GsSwapDispBuff()
	yaroze.GsSortClear(0, 0, 0, &ot[buff])
	yaroze.GsDrawOt(&ot[buff])
}

func main() {
	line.Init(lineBuf[:])
	yaroze.ResetGraph(0)
	demo.PadInit()

	// 640x480, offsets by the GPU, dithered.
	yaroze.GsInitGraph(screenW, screenH, 4, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.GsInit3D()
	for i := range ot {
		ot[i].Length = otLength
		ot[i].Org = &zTable[i][0]
	}
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(32, 32, 576, 400, 0, 256)

	if missing := demo.Load(files, func(string) {
		line.Reset()
		line.WriteString("loading\n")
		line.Print()
		frame()
		yaroze.FntFlush(-1)
	}); missing != "" {
		message("could not read\n", missing)
		yaroze.ResetGraph(1)
		return
	}

	demo.LoadTIM(textureTIM)
	initialiseTMD(torusTMD, 0, &torusOBJ, &torusCoord)

	// One white light from above, as all three, and half ambient.
	light := yaroze.GsF_LIGHT{Vx: 0, Vy: yaroze.ONE, Vz: 0, R: 128, G: 128, B: 128}
	yaroze.GsSetFlatLight(0, &light)
	yaroze.GsSetFlatLight(1, &light)
	yaroze.GsSetFlatLight(2, &light)
	yaroze.GsSetLightMode(0)
	yaroze.GsSetAmbient(yaroze.ONE/2, yaroze.ONE/2, yaroze.ONE/2)
	yaroze.GsSetProjection(1000)

	var angle int32
	for {
		buff := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(&gpuPacket[buff][0])
		yaroze.GsClearOt(0, 0, &ot[buff])

		mainView.View = demo.IDMATRIX
		mainView.Super = demo.WORLD
		yaroze.GsSetView2(&mainView)

		// 1000 in front of the eye, turning about y at half the rate it
		// turns about z.
		transform := demo.IDMATRIX
		translate := yaroze.VECTOR{Vz: 1000}
		yaroze.TransMatrix(&transform, &translate)
		rotate := yaroze.SVECTOR{Vy: int16(angle >> 1), Vz: int16(angle)}
		yaroze.RotMatrix(&rotate, &transform)

		yaroze.GsSetLightMatrix(&transform)
		changeTexture(torusTMD, &transform)
		drawObject(&torusOBJ, &transform, demo.WORLD, &ot[buff])

		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 0, &ot[buff])
		yaroze.GsDrawOt(&ot[buff])

		pad := demo.PadRead()
		angle += 32
		if pad&demo.PADselect != 0 && pad&demo.PADstart != 0 {
			break
		}
	}
	yaroze.ResetGraph(1)
}
