// Cube: a textured cube from the disc, turned with the directional buttons.
package main

import (
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
// enough for its file, which the build checks.
var (
	modelData   [2048]uint32
	textureData [4096]uint32

	modelFile   = disc.Register("DATA/CUBE.TMD", "data/CUBE.TMD").Into(modelData[:])
	textureFile = disc.Register("DATA/CUBE.TIM", "data/CUBE.TIM").Into(textureData[:])

	model   = yaroze.TMD(modelData[:])
	texture = yaroze.TIM(textureData[:])
)

var (
	// The frame: two ordering tables and two packet areas, one of each
	// per display buffer, which the screen takes in halves.
	screen     yaroze.Screen
	otTags     [2 << otLength]yaroze.GsOT_TAG
	packetArea [2 * packetMax / 4]uint32

	view  yaroze.GsRVIEW2
	light yaroze.GsF_LIGHT
	cube  yaroze.Object

	// rotation is which way the cube faces, built up turn by turn; turned
	// and ls are the frame's working matrices.
	rotation, turned, ls yaroze.MATRIX

	textBuf [64]byte
)

func main() {
	yaroze.SetVideoMode(yaroze.MODE_NTSC)
	yaroze.GsInitGraph(screenW, screenH, yaroze.GsNONINTER|yaroze.GsOFSGPU, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, screenH)
	yaroze.GsInit3D()
	screen.Init(otTags[:], packetArea[:])
	yaroze.FntLoad(960, 256)
	// With GsOFSGPU the origin is the middle of the screen, for 3D's sake,
	// so the text window is placed from there.
	yaroze.FntOpen(-screenW/2+16, -screenH/2+16, 256, 64, 0, 512)
	yaroze.PadInit()

	// Each read goes on in the background; waiting gives up after a few
	// seconds, so that a missing disc shows a message rather than hanging.
	if !modelFile.Load() || !disc.Wait(300) || !textureFile.Load() || !disc.Wait(300) {
		yaroze.FntPrint(yaroze.CString(textBuf[:], "could not read the disc"))
		for {
			yaroze.FntFlush(-1)
			yaroze.VSync(0)
		}
	}

	// The texture goes into video memory, at the place the file names and
	// the model's polygons point to, with its palette.
	texture.Load()

	// The model's offsets become addresses now that it has one, and the
	// object is linked to the first model in the file, in its own
	// coordinate system.
	model.Map()
	cube.Init(yaroze.WORLD)
	cube.Link(model, 0)

	// The camera: back from the origin and a little above it, looking at it.
	view = yaroze.GsRVIEW2{Vpx: 0, Vpy: -120, Vpz: -320}
	yaroze.GsSetProjection(240)
	view.Set()

	// One light from over the camera's shoulder, and some ambient light,
	// so that the faces turned away from it are not black.
	light = yaroze.GsF_LIGHT{Vx: 1, Vy: 1, Vz: 1, R: 0xc0, G: 0xc0, B: 0xc0}
	light.Set(0)
	yaroze.GsSetAmbient(yaroze.ONE/4, yaroze.ONE/4, yaroze.ONE/4)
	yaroze.GsSetLightMode(0)

	// Facing straight ahead, with a little of the top and a side showing.
	rotation = yaroze.IDMATRIX
	turn(&yaroze.SVECTOR{Vx: -step * 12, Vy: step * 12})

	hint := yaroze.CString(textBuf[:], "the directions turn the cube")
	for {
		pad := yaroze.PadRead()
		if pad&yaroze.PADLup != 0 {
			turn(&yaroze.SVECTOR{Vx: -step})
		}
		if pad&yaroze.PADLdown != 0 {
			turn(&yaroze.SVECTOR{Vx: step})
		}
		if pad&yaroze.PADLleft != 0 {
			turn(&yaroze.SVECTOR{Vy: -step})
		}
		if pad&yaroze.PADLright != 0 {
			turn(&yaroze.SVECTOR{Vy: step})
		}
		cube.Coord.Coord = rotation
		cube.Coord.Flg = 0

		ot := screen.Begin()
		cube.Coord.Ls(&ls)
		yaroze.GsSetLightMatrix(&ls)
		yaroze.GsSetLsMatrix(&ls)
		cube.Sort(ot, 14-otLength)
		screen.End(60, 120, 120)

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
	m.Rotate(by)
	m.Mul(&rotation, &turned)
	rotation = turned
}
