// The lighting calculation: a ball, a cone and a cube under three lights you
// aim and color.
//
// A port of Sony's Net Yaroze demo "oden" (Sony, 1993 to 1996): a
// demonstration of the lighting calculation. A ball, a cone and a cube,
// each scaled by its own matrix, stand under three directional lights at
// 640 by 480 interlaced, and every light can be aimed and recolored while
// the scene turns. The d-pad also swaps each model for the textured
// version the TMD carries beside it.
//
// Differences from the original:
//
//   - The model and texture are read off the disc into the program's own
//     memory. The original's batch script sent them to fixed addresses.
//
// Nothing allocates after startup.
//
// # Controls
//
// The symbol buttons turn the scene, Start brings it nearer and Select
// pushes it away. Holding L1, L2 or R2 gives the symbol buttons and the
// d-pad to the first, second or third light: Triangle and Cross change its
// red, Circle and Square its green, Select and Start its blue, and the d-pad
// aims it. Holding R1 with Triangle and Cross raises and lowers the ambient
// light. The d-pad alone swaps each model for the textured version the TMD
// also carries, Left the ball, Right the cube and Up the cone, and Down puts
// the plain versions back. Start and Select together quit.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	packetMax = 2000 * 64 // GPU packet space per buffer
	objectMax = 10
	otLength  = 8
)

// The data, read into the program's own memory, in buffers of whole
// sectors.
var (
	modelData [(23316 + 2047) / 2048 * 512]uint32
	texData   [(16404 + 2047) / 2048 * 512]uint32

	modelFile = disc.Register("DATA/ODEN.TMD", "data/ODEN.TMD").Into(modelData[:])
	texFile   = disc.Register("DATA/ODEN.TIM", "data/ODEN.TIM").Into(texData[:])
	files     = []*disc.File{&modelFile, &texFile}

	model   = yaroze.TMD(modelData[:])
	texture = yaroze.TIM(texData[:])
)

var (
	padd yaroze.Buttons

	wot        [2]yaroze.GsOT
	zsorttable [2][1 << otLength]yaroze.GsOT_TAG
	object     [objectMax]yaroze.GsDOBJ2
	objnum     uint32

	dWorld, dCube, dCone, dBall yaroze.GsCOORDINATE2
	fog1                        yaroze.GsFOGPARAM
	am                          int32

	view  yaroze.GsRVIEW2
	pslt  [3]yaroze.GsF_LIGHT
	pdisp yaroze.RECT

	outPacket [2][packetMax / 4]uint32

	// pWorld holds the scene's angles, from which its matrix is rebuilt.
	pWorld yaroze.SVECTOR

	// modelScale is the scale each object's light matrix is given, the
	// inverse of the scale in its coordinate system.
	modelScale [objectMax]yaroze.VECTOR

	frameNumber int32

	textBuf [64]byte
	text    yaroze.Buffer
)

func main() {
	text.Init(textBuf[:])
	initAll()

	if missing := demo.Load(files, nil); missing != "" {
		fail("could not read ", missing)
		return
	}
	texture.Load()
	modelingDataRead()

	for {
		if objInteractive() {
			break
		}
		drawAll(objnum)
	}
	yaroze.ResetGraph(3)
}

// drawAll draws the objects that are on: each with a light matrix scaled
// back to the model's own size, so that the lighting does not change with
// the scale.
func drawAll(on uint32) {
	outbufIdx := yaroze.GsGetActiveBuff()
	yaroze.GsSetWorkBase(yaroze.Packets(&outPacket[outbufIdx][0]))
	yaroze.GsClearOt(0, 0, &wot[outbufIdx])

	for i := uint32(0); i < on; i++ {
		op := &object[i]
		var tmpm yaroze.MATRIX
		yaroze.GsGetLw(op.Coord2, &tmpm)

		v := modelScale[i]
		yaroze.ScaleMatrix(&tmpm, &v)
		yaroze.GsSetLightMatrix(&tmpm)

		yaroze.GsGetLs(op.Coord2, &tmpm)
		yaroze.GsSetLsMatrix(&tmpm)

		yaroze.GsSortObject4(op, &wot[outbufIdx], 14-otLength, yaroze.Scratch())
	}

	yaroze.VSync(0)
	padd = yaroze.PadRead()
	yaroze.ResetGraph(1)
	yaroze.GsSwapDispBuff()
	yaroze.GsSortClear(0, 0, 0, &wot[outbufIdx])
	yaroze.GsDrawOt(&wot[outbufIdx])
	frameNumber++
}

// objInteractive applies the pad to the lights or the scene, and reports
// whether Start and Select together asked to quit.
func objInteractive() bool {
	modeF := false

	if padd&yaroze.PADL1 != 0 {
		lightSet(padd, 0)
		modeF = true
	}
	if padd&yaroze.PADL2 != 0 {
		lightSet(padd, 1)
		modeF = true
	}
	if padd&yaroze.PADR2 != 0 {
		lightSet(padd, 2)
		modeF = true
	}
	if padd&yaroze.PADR1 != 0 {
		ambientSet(padd)
		modeF = true
	}
	if modeF {
		return false
	}

	switch {
	case padd&yaroze.PADLleft != 0: // ball
		object[0].Attribute = yaroze.GsDOFF
		object[3].Attribute = 0
	case padd&yaroze.PADLright != 0: // cube
		object[2].Attribute = yaroze.GsDOFF | yaroze.GsDIV2
		object[5].Attribute = yaroze.GsDIV2
	case padd&yaroze.PADLup != 0: // cone
		object[1].Attribute = yaroze.GsDOFF
		object[4].Attribute = 0
	case padd&yaroze.PADLdown != 0:
		object[0].Attribute = 0
		object[3].Attribute = yaroze.GsDOFF
		object[2].Attribute = yaroze.GsDIV2
		object[5].Attribute = yaroze.GsDOFF | yaroze.GsDIV2
		object[1].Attribute = 0
		object[4].Attribute = yaroze.GsDOFF
	default:
		if padd&yaroze.PADRleft != 0 {
			pWorld.Vy -= 5 * yaroze.ONE / 360
		}
		if padd&yaroze.PADRright != 0 {
			pWorld.Vy += 5 * yaroze.ONE / 360
		}
		if padd&yaroze.PADRup != 0 {
			pWorld.Vx += 5 * yaroze.ONE / 360
		}
		if padd&yaroze.PADRdown != 0 {
			pWorld.Vx -= 5 * yaroze.ONE / 360
		}

		if padd&yaroze.PADstart != 0 && dWorld.Coord.T[2] > -2800 {
			dWorld.Coord.T[2] -= 80
		}
		if padd&yaroze.PADselect != 0 {
			dWorld.Coord.T[2] += 80
		}
		if padd&yaroze.PADselect != 0 && padd&yaroze.PADstart != 0 {
			return true
		}
		setCoordinate(&pWorld, &dWorld)
	}
	return false
}

// lightSet changes light n's color by the symbol buttons, Start and Select,
// and its direction by the d-pad.
func lightSet(padd yaroze.Buttons, n int) {
	if padd&yaroze.PADRup != 0 && pslt[n].R < 0xfe {
		pslt[n].R += 2
	}
	if padd&yaroze.PADRdown != 0 && pslt[n].R > 0 {
		pslt[n].R -= 2
	}

	if padd&yaroze.PADRright != 0 && pslt[n].G < 0xfe {
		pslt[n].G += 2
	}
	if padd&yaroze.PADRleft != 0 && pslt[n].G > 0 {
		pslt[n].G -= 2
	}

	if padd&yaroze.PADselect != 0 && pslt[n].B < 0xfe {
		pslt[n].B += 2
	}
	if padd&yaroze.PADstart != 0 && pslt[n].B > 0 {
		pslt[n].B -= 2
	}

	if padd&yaroze.PADLup != 0 && pslt[n].Vy < 300 {
		pslt[n].Vy += 10
	}
	if padd&yaroze.PADLdown != 0 && pslt[n].Vy > -300 {
		pslt[n].Vy -= 10
	}

	if padd&yaroze.PADLleft != 0 && pslt[n].Vx < 300 {
		pslt[n].Vx += 10
	}
	if padd&yaroze.PADLright != 0 && pslt[n].Vx > -300 {
		pslt[n].Vx -= 10
	}

	yaroze.GsSetFlatLight(int32(n), &pslt[n])
}

func ambientSet(padd yaroze.Buttons) {
	if padd&yaroze.PADRup != 0 && am < 2*yaroze.ONE {
		am += 20
	}
	if padd&yaroze.PADRdown != 0 && am > 20 {
		am -= 20
	}
	yaroze.GsSetAmbient(am, am, am)
}

// modelingDataRead links the six objects and places the three shapes,
// each in a coordinate system of its own under the scene's, scaled by a
// matrix whose inverse goes into modelScale for the lighting.
func modelingDataRead() {
	model.Map()
	objnum = uint32(model.Objects())
	if objnum > objectMax {
		objnum = objectMax
	}
	for i := int32(0); i < int32(objnum); i++ {
		model.Link(&object[i], i)
	}

	yaroze.GsInitCoordinate2(yaroze.WORLD, &dWorld)
	dWorld.Coord.T[2] = 1000
	yaroze.GsInitCoordinate2(&dWorld, &dCube)
	yaroze.GsInitCoordinate2(&dWorld, &dCone)
	yaroze.GsInitCoordinate2(&dWorld, &dBall)
	for i := range objnum {
		object[i].Coord2 = &dWorld
	}

	pWorld = yaroze.SVECTOR{}

	var v yaroze.VECTOR

	// The ball.
	object[0].Attribute = 0
	object[0].Coord2 = &dBall
	dBall.Coord.T[0] = -400 * 2
	dBall.Coord.T[1] = -140 * 2
	v.Vx, v.Vy, v.Vz = 4*2*yaroze.ONE/5, 4*2*yaroze.ONE/5, 4*2*yaroze.ONE/5
	yaroze.ScaleMatrix(&dBall.Coord, &v)
	modelScale[0].Vx, modelScale[0].Vy, modelScale[0].Vz = 5*yaroze.ONE/4/2, 5*yaroze.ONE/4/2, 5*yaroze.ONE/4/2

	// The cone.
	object[1].Attribute = 0
	object[1].Coord2 = &dCone
	dCone.Coord.T[0] = 0
	v.Vx, v.Vy, v.Vz = 2*yaroze.ONE*4/3, 2*yaroze.ONE, 2*yaroze.ONE*4/3
	yaroze.ScaleMatrix(&dCone.Coord, &v)
	modelScale[1].Vx, modelScale[1].Vz = 3*yaroze.ONE/4/2, 3*yaroze.ONE/4/2
	modelScale[1].Vy = yaroze.ONE / 2

	// The cube, subdivided four by four.
	object[2].Attribute = yaroze.GsDIV2
	object[2].Coord2 = &dCube
	dCube.Coord.T[0] = 300 * 2
	dCube.Coord.T[2] = -160 * 2
	v.Vx, v.Vy, v.Vz = 2*yaroze.ONE*5/2, 2*yaroze.ONE*5/2, 2*yaroze.ONE*5/2
	yaroze.ScaleMatrix(&dCube.Coord, &v)
	modelScale[2].Vx, modelScale[2].Vy, modelScale[2].Vz = 2*yaroze.ONE/5/2, 2*yaroze.ONE/5/2, 2*yaroze.ONE/5/2

	// The textured versions, off until the d-pad asks for them.
	object[3].Attribute = yaroze.GsDOFF
	object[3].Coord2 = &dBall
	modelScale[3] = modelScale[0]
	object[4].Attribute = yaroze.GsDOFF
	object[4].Coord2 = &dCone
	modelScale[4] = modelScale[1]
	object[5].Attribute = yaroze.GsDOFF | yaroze.GsDIV2
	object[5].Coord2 = &dCube
	modelScale[5] = modelScale[2]
}

func initAll() {
	yaroze.PadInit()
	padd = 0

	yaroze.GsInitGraph(640, 480, yaroze.GsINTER|yaroze.GsOFSGPU, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)

	for i := range wot {
		wot[i].Length = otLength
		wot[i].Org = &zsorttable[i][0]
		wot[i].Tag = &zsorttable[i][0]
	}

	yaroze.GsInit3D()
	viewInit()
	lightInit()

	yaroze.GsSetLightMode(0)

	am = 0
	yaroze.GsSetAmbient(am, am, am)

	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(-304, -232, 320, 16, 0, 64)
}

func viewInit() {
	yaroze.GsSetProjection(1000)
	view.Vpx, view.Vpy, view.Vpz = 0, -75, -5000
	view.Vrx, view.Vry, view.Vrz = 0, -75, 0
	view.Rz = 0
	view.Super = yaroze.WORLD
	yaroze.GsSetRefView2(&view)
}

func lightInit() {
	pslt[0] = yaroze.GsF_LIGHT{Vx: 20, Vy: -100, Vz: 100, R: 0x80, G: 0x80, B: 0x80}
	yaroze.GsSetFlatLight(0, &pslt[0])

	pslt[1] = yaroze.GsF_LIGHT{Vx: 20, Vy: -50, Vz: -100, R: 0x80, G: 0x80, B: 0x80}
	yaroze.GsSetFlatLight(1, &pslt[1])

	pslt[2] = yaroze.GsF_LIGHT{Vx: -20, Vy: 20, Vz: 100, R: 0x80, G: 0x80, B: 0x80}
	yaroze.GsSetFlatLight(2, &pslt[2])

	yaroze.GsSetAmbient(am, am, am)

	// The fog parameters are set as the original sets them, though the
	// light mode it runs in does not use them.
	fog1.Dqa = -10000 * yaroze.ONE / 64 / 1000
	fog1.Dqb = 5 / 4 * yaroze.ONE * yaroze.ONE
	fog1.Rfc, fog1.Gfc, fog1.Bfc = 0, 0, 0
	yaroze.GsSetFogParam(&fog1)
}

// setCoordinate rebuilds a coordinate system's matrix from its position
// and the angles in pos.
func setCoordinate(pos *yaroze.SVECTOR, coor *yaroze.GsCOORDINATE2) {
	tmp1 := yaroze.IDMATRIX
	tmp1.T = coor.Coord.T
	v1 := *pos
	yaroze.RotMatrix(&v1, &tmp1)
	coor.Coord = tmp1
	coor.Flg = 0
}

// fail shows what went wrong until the program is quit.
func fail(what, detail string) {
	for yaroze.PadRead()&(yaroze.PADstart|yaroze.PADselect) != yaroze.PADstart|yaroze.PADselect {
		buff := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&outPacket[buff][0]))
		yaroze.GsClearOt(0, 0, &wot[buff])
		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 0, &wot[buff])
		yaroze.GsDrawOt(&wot[buff])
		text.Reset()
		text.WriteString(what).WriteString(detail).Print()
		yaroze.FntFlush(-1)
	}
	yaroze.ResetGraph(3)
}
