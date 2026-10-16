// The solar tutorial, in Go.
//
// A port of the last of the "3-D object manipulation" tutorials in Sony's
// Net Yaroze solar2 demo (LPGE, 1996): a solar system to fly a shuttle
// through and take apart. A sun, three planets orbiting it in planes at
// random angles, three moons round each planet, four ships flying round a
// square through the sun, and a shuttle sitting at the middle, all lit by
// three coloured lights.
//
// Everything is on screen to change. Start switches between flying the
// shuttle and the menus. In the menus, up and down on the right-hand
// buttons pick a line, and left and right on them, with the first line
// picked, the menu: the shuttle; the three lights' directions and colours;
// the ambient light; the fog, its colour and whether it is on; the view,
// its projection, its two points and which object it hangs off; and any
// object, selected by turning its lighting off, to move, spin, scale,
// change how it is drawn, or watch from one of five views: from along an
// axis, circling it, flying by it, from it, or a director's cut of three.
// The directional buttons and L1 and L2 change the picked line; Select
// makes them faster. Flying, the directional buttons turn the shuttle and
// L1 and L2 move it. L2 and R2 held step slowly, and held for a moment
// start again. Start and Select together quit.
//
// Differences from the original:
//
//   - The models are read off the disc to the addresses the original's
//     batch script sent them to. The original links itself at 80100000,
//     below the 80140000 a Net Yaroze program usually loads at; the
//     models sit below either.
//   - The vector lengths the original works out in double precision are
//     integer square roots, which come to the same.
//   - What the original prints to the serial console, a line or two every
//     frame, is left out, and so are its assertions.
package main

import (
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	packetMax = 10000 * 24
	otLength  = 14

	// The sphere model holds the same sphere in several colours.
	sphereModelAddr  = 0x800c0000
	shuttleModelAddr = 0x800e0000
	shipModelAddr    = 0x800f0000
)

var files = []demo.File{
	{Name: "\\DATA\\SPHERES.TMD;1", Addr: sphereModelAddr},
	{Name: "\\DATA\\SHUTTLE1.TMD;1", Addr: shuttleModelAddr},
	{Name: "\\DATA\\THESHIP.TMD;1", Addr: shipModelAddr},
}

var (
	ot        [2]yaroze.GsOT
	zTable    [2][1 << otLength]yaroze.GsOT_TAG
	gpuPacket [2][packetMax]yaroze.PACKET

	frameNumber uint32

	// What the menus change, and whether it has changed since it was last
	// handed to the library.
	lights                                [3]yaroze.GsF_LIGHT
	selectedLight                         int32
	lightsChanged                         bool
	ambientRed, ambientGreen, ambientBlue uint16
	ambientChanged                        bool
	fog                                   yaroze.GsFOGPARAM
	fogChanged                            bool
	lightMode                             int32
	lightModeChanged                      bool
	projection                            uint32
	projectionChanged                     bool
	mainView                              yaroze.GsRVIEW2
	viewChanged                           bool
)

func main() {
	line.Init(lineBuf[:])
	if !initialiseAll() {
		return
	}
	for {
		dealWithSpatialMovement()
		if !dealWithController() {
			break
		}
		handleAllObjects()

		if lightsChanged {
			yaroze.GsSetFlatLight(selectedLight, &lights[selectedLight])
			lightsChanged = false
		}
		if ambientChanged {
			yaroze.GsSetAmbient(int32(ambientRed), int32(ambientGreen), int32(ambientBlue))
			ambientChanged = false
		}
		if fogChanged {
			yaroze.GsSetFogParam(&fog)
			fogChanged = false
		}
		if projectionChanged {
			yaroze.GsSetProjection(int32(projection))
			projectionChanged = false
		}
		if viewChanged || specialViewFlag {
			if specialViewFlag {
				dealWithSpecialView()
			}
			yaroze.GsSetRefView2(&mainView)
			viewChanged = false
		}
		if lightModeChanged {
			yaroze.GsSetLightMode(lightMode)
			lightModeChanged = false
		}

		buff := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(&gpuPacket[buff][0])
		yaroze.GsClearOt(0, 0, &ot[buff])
		for _, o := range &objects {
			if o != nil && o.alive {
				sortObject(&o.handler, &o.coord, &ot[buff])
			}
		}
		sortObject(&playersShuttle.handler, &playersShuttle.coord, &ot[buff])

		padStatus = demo.PadRead()

		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 0, &ot[buff])
		yaroze.GsDrawOt(&ot[buff])

		frameNumber++
		framesSinceLastSwitch++
		framesSinceLastSelection++
		framesSinceLastLightChange++
		framesSinceLastLightModeChanged++
		numberFramesSinceViewpointChange++
		numberFramesSinceViewModeChange++

		printMenus()
		yaroze.FntFlush(-1)
	}
	yaroze.ResetGraph(3)
}

func sortObject(h *yaroze.GsDOBJ2, c *yaroze.GsCOORDINATE2, o *yaroze.GsOT) {
	var ls yaroze.MATRIX
	yaroze.GsGetLs(c, &ls)
	yaroze.GsSetLightMatrix(&ls)
	yaroze.GsSetLsMatrix(&ls)
	yaroze.GsSortObject4(h, o, 14-otLength, demo.Scratch())
}

func initialiseAll() bool {
	yaroze.ResetGraph(0)
	padStatus = 0

	// 512 by 240, not interlaced.
	yaroze.GsInitGraph(512, 240, yaroze.GsOFSGPU|yaroze.GsNONINTER, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 256)
	yaroze.GsInit3D()
	for i := range ot {
		ot[i].Length = otLength
		ot[i].Org = &zTable[i][0]
	}
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(32, 32, 256, 200, 0, 512)
	demo.PadInit()

	if missing := demo.Load(files, nil); missing != "" {
		for demo.PadRead()&(demo.PADstart|demo.PADselect) != demo.PADstart|demo.PADselect {
			buff := yaroze.GsGetActiveBuff()
			yaroze.GsSetWorkBase(&gpuPacket[buff][0])
			yaroze.GsClearOt(0, 0, &ot[buff])
			say("could not read\n")
			say(missing)
			yaroze.DrawSync(0)
			yaroze.VSync(0)
			yaroze.GsSwapDispBuff()
			yaroze.GsSortClear(0, 0, 0, &ot[buff])
			yaroze.GsDrawOt(&ot[buff])
			yaroze.FntFlush(-1)
		}
		yaroze.ResetGraph(3)
		return false
	}

	initialiseLighting()
	initialiseObjects()

	superMode = moveMode
	selectionMode = moveShip
	framesSinceLastSwitch = 0
	selectedObject = 0
	framesSinceLastSelection = 0
	resetSelectionData()
	initialiseView()
	viewChanged = true
	return true
}

// initialiseLighting starts each light along an axis in a primary colour:
// red along z, green along y and blue along x. As the original has it,
// the blue one is set up but never handed to the library, until its menu
// changes it.
func initialiseLighting() {
	lights[0] = yaroze.GsF_LIGHT{Vz: 100, R: 128}
	lights[1] = yaroze.GsF_LIGHT{Vy: 100, G: 128}
	lights[2] = yaroze.GsF_LIGHT{Vx: 100, B: 128}
	for i := range 2 {
		yaroze.GsSetFlatLight(int32(i), &lights[i])
	}
	selectedLight = 0
	lightsChanged = false
	framesSinceLastLightChange = 0

	ambientRed, ambientGreen, ambientBlue = yaroze.ONE/20, yaroze.ONE/10, yaroze.ONE/2
	ambientChanged = false
	yaroze.GsSetAmbient(int32(ambientRed), int32(ambientGreen), int32(ambientBlue))

	fog = yaroze.GsFOGPARAM{}
	fogChanged = false

	lightMode = 0 // fog off
	yaroze.GsSetLightMode(lightMode)
	lightModeChanged = false
	framesSinceLastLightModeChanged = 0
}

// initialiseView looks from just behind the shuttle, and sets every
// special view's settings to where they start.
func initialiseView() {
	projection = 2000
	projectionChanged = false
	yaroze.GsSetProjection(int32(projection))

	mainView = yaroze.GsRVIEW2{Vpz: -200, Super: &playersShuttle.coord}
	mainViewObjectID = -1
	numberFramesSinceViewpointChange = 0
	viewMode = generalDistortion
	numberFramesSinceViewModeChange = 0
	viewChanged = true

	specialViewFlag = false
	specialViewType = none

	whichExternalView = minusY // from above
	externalViewDistance = 12000

	whichCircularView = xyPlane
	whichWayRoundCirclingView = clockwise
	circleViewRadius = 5000
	circleViewAngle = 0
	circleViewAngleIncrement = 24

	typeOfFlybyView = none
	pathDistance = 2000
	pathAngle = 0
	flybyCycleTime = 30
	numberFlybyFramesSoFar = 0

	whichLocalView = plusZ
	localViewDistance = 5000

	sequencePosition = 0
	singleViewCycleTime = 30
	directorFrameCounter = 0
	directorLocalView = none
	directorLocalViewDistance = 2000
	directorWhichCircularView = none
	directorWhichWayRoundCirclingView = none
	directorCircleViewRadius = 2000
	directorCircleViewAngleIncrement = 24
	directorPathDistance = 2000
	directorPathAngle = 3072 // underneath
}

func resetAll() {
	initialiseObjects()
	initialiseLighting()
	selectionMode = moveShip
	framesSinceLastSwitch = 0
	resetSelectionData()
	selectedObject = 0
	framesSinceLastSelection = 0
	initialiseView()
	viewChanged = true
}
