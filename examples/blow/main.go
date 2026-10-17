// The blow demo, in Go.
//
// A port of Sony's Net Yaroze demo "blow" (S. Ashley, 1997): a model
// viewer that blows its model up. A space shuttle, in five pieces, is read
// off the disc, and one piece at a time is shown at 512 by 240, turned,
// moved and scaled, under three directional lights, ambient light and fog,
// every parameter of which a menu of the library's font lets you change.
// The last page of the menu is the explosion: the piece's polygons fly
// apart along their normals, flashing white and burning through yellow and
// red to black while the screen flashes, and then it is put back together
// to go again. To do that the model is copied at startup with a separate
// set of vertices for every polygon, since a TMD shares its vertices
// between the polygons that meet at them, and the copy is what is drawn.
//
// R1 and R2 step through the menu's pages and Select and Start through
// the pieces. On each page the d-pad, L1 and L2 adjust three values, and
// the symbol buttons the rest: the first page turns the piece, and Triangle
// and Cross bring the viewpoint nearer and further; the second moves the
// viewpoint; the third scales the piece; the next three aim and color the
// lights, Square and Circle choosing the component and Triangle and Cross
// changing it; then the ambient light; then the fog, its distance and
// color, and whether it is on; and last the explosion, its speed and
// whether it is going. Start and Select together quit.
//
// Differences from the original:
//
//   - The viewpoint moves in and out by fiftieths, where the original
//     multiplies and divides by 1.02 in floating point.
//   - The model is read off the disc to the address the original's batch
//     script sent it to, and the copy is checked to fit below it.
//   - The model's extent, which the original works out at startup and
//     prints to the serial console, is left out, and so is the missing
//     data's report, which the original would hang on.
//
// Nothing allocates after startup.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	packetMax = 10000 * 24 // GPU packet space per buffer

	maxNumberOfObjects = 10

	otLength = 14

	// copyWords is the room the copy of the model is built in, which is
	// larger than the model since every primitive gets vertices of its
	// own: the 192KB the demo's batch script left between the two.
	copyWords = 192 * 1024 / 4
)

// The model, read into the program's own memory in a buffer of whole
// sectors, and the memory its copy is built in.
var (
	shuttleData [(47988 + 2047) / 2048 * 512]uint32
	copyData    [copyWords]uint32

	shuttleFile = disc.Register("DATA/SHUTTLE1.TMD", "data/SHUTTLE1.TMD").Into(shuttleData[:])
	files       = []*disc.File{&shuttleFile}

	shuttle = yaroze.TMD(shuttleData[:])

	// The functions that walk the two and copy between them work in
	// addresses, as the demo does.
	tmdAddress = uintptr(unsafe.Pointer(&shuttleData[0]))
	copyTmd    = uintptr(unsafe.Pointer(&copyData[0]))
)

var (
	worldOrderingTable [2]yaroze.GsOT
	zSortTable         [2][1 << otLength]yaroze.GsOT_TAG
	gpuOutputPacket    [2][packetMax / 4]uint32
	padStatus          uint32

	object      [maxNumberOfObjects]yaroze.GsDOBJ2
	theMainView yaroze.GsRVIEW2

	objectCoord yaroze.GsCOORDINATE2
	rotateObj   yaroze.SVECTOR
	translate   yaroze.VECTOR
	viewFrom    = yaroze.VECTOR{Vx: 4000, Vy: 4000, Vz: 4000}
	dispObj     int32
	nObj        int32
	scale       = yaroze.VECTOR{Vx: yaroze.ONE, Vy: yaroze.ONE, Vz: yaroze.ONE}
	lightSource [3]yaroze.GsF_LIGHT

	ambientRed, ambientGreen, ambientBlue int16 = yaroze.ONE / 2, yaroze.ONE / 2, yaroze.ONE / 2

	fogging      = yaroze.GsFOGPARAM{Dqa: -1024, Dqb: 20971520}
	foggingOnOff int32

	// The explosion, and the flash of the screen that starts it.
	bang      bool
	bangCount int32

	frameNumber int32

	textBuf [256]byte
	text    yaroze.Buffer
)

func main() {
	text.Init(textBuf[:])
	initialiseAll()

	if missing := demo.Load(files, nil); missing != "" {
		fail("could not read ", missing)
		return
	}

	shuttle.Map()
	if end := separateVertices(tmdAddress, copyTmd); end > copyTmd+copyWords*4 {
		fail("the copy of the model does not fit", "")
		return
	}

	model := yaroze.TMD(copyData[:])
	nObj = int32(model.Objects())
	for i := range nObj {
		model.Link(&object[i], i)
		yaroze.GsInitCoordinate2(yaroze.WORLD, &objectCoord)
		object[i].Coord2 = &objectCoord
	}

	yaroze.GsSetLightMode(foggingOnOff)

	theMainView.Vrx, theMainView.Vry, theMainView.Vrz = 0, 0, 0
	theMainView.Rz = 0
	theMainView.Super = yaroze.WORLD

	yaroze.GsSetProjection(1000)

	for {
		theMainView.Vpx = viewFrom.Vx
		theMainView.Vpy = viewFrom.Vy
		theMainView.Vpz = viewFrom.Vz
		yaroze.GsSetRefView2(&theMainView)

		outputBufferIndex := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&gpuOutputPacket[outputBufferIndex][0]))
		yaroze.GsClearOt(0, 0, &worldOrderingTable[outputBufferIndex])

		yaroze.GsSetFlatLight(0, &lightSource[0])
		yaroze.GsSetFlatLight(1, &lightSource[1])
		yaroze.GsSetFlatLight(2, &lightSource[2])
		yaroze.GsSetFogParam(&fogging)
		yaroze.GsSetAmbient(int32(ambientRed), int32(ambientGreen), int32(ambientBlue))

		// The piece's transformation: scaled, turned, then moved.
		temp1Matrix := yaroze.IDMATRIX
		yaroze.ScaleMatrix(&temp1Matrix, &scale)
		var temp2Matrix, transformMatrix yaroze.MATRIX
		yaroze.RotMatrix(&rotateObj, &temp2Matrix)
		yaroze.MulMatrix0(&temp2Matrix, &temp1Matrix, &transformMatrix)
		yaroze.TransMatrix(&transformMatrix, &translate)
		object[dispObj].Coord2.Coord = transformMatrix
		object[dispObj].Coord2.Flg = 0

		var coordMatrix yaroze.MATRIX
		yaroze.GsGetLs(object[dispObj].Coord2, &coordMatrix)
		yaroze.GsSetLightMatrix(&yaroze.IDMATRIX)
		yaroze.GsSetLsMatrix(&coordMatrix)
		yaroze.GsSortObject4(&object[dispObj], &worldOrderingTable[outputBufferIndex], 14-otLength, yaroze.Scratch())

		yaroze.DrawSync(0)
		t := yaroze.VSync(1)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()

		// The screen flashes as an explosion starts.
		var bgCol uint8
		switch {
		case bangCount < 15 && bang:
			bgCol = uint8(bangCount * 10)
		case bangCount < 50 && bang:
			bgCol = uint8((50 - bangCount) * 4)
		}
		yaroze.GsSortClear(bgCol, bgCol, bgCol, &worldOrderingTable[outputBufferIndex])
		yaroze.GsDrawOt(&worldOrderingTable[outputBufferIndex])

		padStatus = yaroze.PadRead()

		text.Reset()
		text.WriteString("vsync ").WriteInt(int(t)).WriteString("\nObject ").WriteInt(int(dispObj + 1)).
			WriteString(" of ").WriteInt(int(nObj)).WriteString("\n")
		controlsMainMenu()
		text.Print()
		yaroze.FntFlush(-1)
		frameNumber++

		if padStatus&yaroze.PADselect != 0 && padStatus&yaroze.PADstart != 0 {
			break
		}
	}
	yaroze.ResetGraph(3)
}

// delay waits d frames, by the library's count of them.
func delay(d int32) {
	v := yaroze.VSync(-1)
	for yaroze.VSync(-1)-v < d {
	}
}

func initialiseAll() {
	yaroze.ResetGraph(0)
	padStatus = 0

	// 512 by 240, not interlaced, offsets by the GPU.
	yaroze.GsInitGraph(512, 240, yaroze.GsOFSGPU|yaroze.GsNONINTER, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 256)
	yaroze.GsInit3D()

	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(32, 32, 256, 200, 0, 512)

	for i := range 2 {
		worldOrderingTable[i].Length = otLength
		worldOrderingTable[i].Org = &zSortTable[i][0]
	}

	yaroze.PadInit()
}

// fail shows what went wrong until the program is quit.
func fail(what, detail string) {
	for yaroze.PadRead()&(yaroze.PADstart|yaroze.PADselect) != yaroze.PADstart|yaroze.PADselect {
		buff := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&gpuOutputPacket[buff][0]))
		yaroze.GsClearOt(0, 0, &worldOrderingTable[buff])
		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 0, &worldOrderingTable[buff])
		yaroze.GsDrawOt(&worldOrderingTable[buff])
		text.Reset()
		text.WriteString(what).WriteString(detail).Print()
		yaroze.FntFlush(-1)
	}
	yaroze.ResetGraph(3)
}

var menuOption int8

// controlsMainMenu steps through the menu's pages with R1 and R2 and the
// pieces with Select and Start, prints the page, and hands the pad to it.
// Leaving the explosion's page puts the piece back together.
func controlsMainMenu() {
	if padStatus&yaroze.PADR1 != 0 {
		menuOption--
		if menuOption < 0 {
			menuOption = 8
		}
		delay(10)
	}
	if padStatus&yaroze.PADR2 != 0 {
		menuOption++
		if menuOption > 8 {
			menuOption = 0
		}
		delay(10)
	}
	if padStatus&yaroze.PADselect != 0 {
		dispObj--
		if dispObj < 0 {
			dispObj = nObj - 1
		}
		delay(10)
	}
	if padStatus&yaroze.PADstart != 0 {
		dispObj++
		if dispObj >= nObj {
			dispObj = 0
		}
		delay(10)
	}

	text.WriteString("Menu ").WriteInt(int(menuOption)).WriteString(" ")

	if menuOption != 8 && bang {
		bang = false
		bangCount = 0
		resetColors(tmdAddress, copyTmd, dispObj)
		resetVertices(tmdAddress, copyTmd, dispObj)
	}

	switch menuOption {
	case 0:
		text.WriteString("Move Object\n")
		controlsWRTObject()
	case 1:
		text.WriteString("Move in World\n")
		controlsWRTWorld()
	case 2:
		text.WriteString("Scale Object\n")
		controlsScaling()
	case 3:
		text.WriteString("Light Source 1\n")
		controlsLighting(0)
	case 4:
		text.WriteString("Light Source 2\n")
		controlsLighting(1)
	case 5:
		text.WriteString("Light Source 3\n")
		controlsLighting(2)
	case 6:
		text.WriteString("Ambient Light\n")
		controlsAmbient()
	case 7:
		text.WriteString("Fogging\n")
		controlsFogging()
	case 8:
		text.WriteString("Explosion\n")
		controlsExplosion()
	}
}

// zoom brings the viewpoint nearer with Triangle and further with Cross,
// by a fiftieth each frame.
func zoom() {
	if padStatus&yaroze.PADRup != 0 {
		viewFrom.Vx += viewFrom.Vx / 50
		viewFrom.Vy += viewFrom.Vy / 50
		viewFrom.Vz += viewFrom.Vz / 50
	}
	if padStatus&yaroze.PADRdown != 0 {
		viewFrom.Vx = viewFrom.Vx * 50 / 51
		viewFrom.Vy = viewFrom.Vy * 50 / 51
		viewFrom.Vz = viewFrom.Vz * 50 / 51
	}
}

// adjust3 changes three values by step: the first with left and right,
// the second with up and down, and the third with L2 and L1.
func adjust3(x, y, z *int32, step int32) {
	if padStatus&yaroze.PADLright != 0 {
		*x += step
	}
	if padStatus&yaroze.PADLleft != 0 {
		*x -= step
	}
	if padStatus&yaroze.PADLup != 0 {
		*y += step
	}
	if padStatus&yaroze.PADLdown != 0 {
		*y -= step
	}
	if padStatus&yaroze.PADL2 != 0 {
		*z += step
	}
	if padStatus&yaroze.PADL1 != 0 {
		*z -= step
	}
}

func controlsWRTObject() {
	text.WriteString("Rotation ").WriteInt(int(rotateObj.Vx)).WriteString(" ").WriteInt(int(rotateObj.Vy)).
		WriteString(" ").WriteInt(int(rotateObj.Vz)).WriteString("\n")
	x, y, z := int32(rotateObj.Vx), int32(rotateObj.Vy), int32(rotateObj.Vz)
	adjust3(&x, &y, &z, 8)
	rotateObj.Vx, rotateObj.Vy, rotateObj.Vz = int16(x), int16(y), int16(z)
	zoom()
}

func controlsWRTWorld() {
	text.WriteString("View Point ").WriteInt(int(viewFrom.Vx)).WriteString(" ").WriteInt(int(viewFrom.Vy)).
		WriteString(" ").WriteInt(int(viewFrom.Vz)).WriteString("\n")
	adjust3(&viewFrom.Vx, &viewFrom.Vy, &viewFrom.Vz, 64)
	zoom()
}

func controlsScaling() {
	text.WriteString("Scaling ").WriteInt(int(scale.Vx)).WriteString(" ").WriteInt(int(scale.Vy)).
		WriteString(" ").WriteInt(int(scale.Vz)).WriteString("\n")
	adjust3(&scale.Vx, &scale.Vy, &scale.Vz, 4)
}

// printColourDetails prints a color with one component highlighted in
// green, or none.
func printColourDetails(r, g, b int32, highlight int8) {
	text.WriteString("Colour ")
	switch highlight {
	case 0:
		text.WriteString("~c0f0R ").WriteInt(int(r)).WriteString(" ~cfffG ").WriteInt(int(g)).WriteString(" B ").WriteInt(int(b)).WriteString("\n")
	case 1:
		text.WriteString("R ").WriteInt(int(r)).WriteString(" ~c0f0G ").WriteInt(int(g)).WriteString(" ~cfffB ").WriteInt(int(b)).WriteString("\n")
	case 2:
		text.WriteString("R ").WriteInt(int(r)).WriteString(" G ").WriteInt(int(g)).WriteString(" ~c0f0B ").WriteInt(int(b)).WriteString("\n~cfff")
	default:
		text.WriteString("R ").WriteInt(int(r)).WriteString(" G ").WriteInt(int(g)).WriteString(" B ").WriteInt(int(b)).WriteString("\n")
	}
}

// chooseRGB moves the highlighted component with Square and Circle.
func chooseRGB(rgb *int8) {
	if padStatus&yaroze.PADRright != 0 {
		*rgb++
		if *rgb > 2 {
			*rgb = 0
		}
		delay(10)
	}
	if padStatus&yaroze.PADRleft != 0 {
		*rgb--
		if *rgb < 0 {
			*rgb = 2
		}
		delay(10)
	}
}

// upDown returns how Triangle and Cross change a value: by step up, step
// down, or nothing.
func upDown(step int32) int32 {
	var d int32
	if padStatus&yaroze.PADRup != 0 {
		d += step
	}
	if padStatus&yaroze.PADRdown != 0 {
		d -= step
	}
	return d
}

var lightRGB int8

func controlsLighting(light int) {
	l := &lightSource[light]
	text.WriteString("Direction X ").WriteInt(int(l.Vx)).WriteString("\n          Y ").WriteInt(int(l.Vy)).
		WriteString("\n          Z ").WriteInt(int(l.Vz)).WriteString("\n")
	printColourDetails(int32(l.R), int32(l.G), int32(l.B), lightRGB)

	adjust3(&l.Vx, &l.Vy, &l.Vz, 8)
	chooseRGB(&lightRGB)
	switch lightRGB {
	case 0:
		l.R += uint8(upDown(1))
	case 1:
		l.G += uint8(upDown(1))
	case 2:
		l.B += uint8(upDown(1))
	}
}

var ambientRGB int8

func controlsAmbient() {
	printColourDetails(int32(ambientRed), int32(ambientGreen), int32(ambientBlue), ambientRGB)
	chooseRGB(&ambientRGB)
	switch ambientRGB {
	case 0:
		ambientRed += int16(upDown(8))
	case 1:
		ambientGreen += int16(upDown(8))
	case 2:
		ambientBlue += int16(upDown(8))
	}
}

var fogOption int8

func controlsFogging() {
	onOff := "Fogging Off\n"
	if foggingOnOff != 0 {
		onOff = "Fogging On\n"
	}
	switch fogOption {
	case 0:
		text.WriteString("~c0f0").WriteString(onOff).WriteString("~cfff")
	default:
		text.WriteString(onOff)
	}
	switch fogOption {
	case 1:
		text.WriteString("~c0f0dqa ").WriteInt(int(fogging.Dqa)).WriteString(" ~cfffdqb ").WriteInt(int(fogging.Dqb)).WriteString("\n")
	case 2:
		text.WriteString("dqa ").WriteInt(int(fogging.Dqa)).WriteString(" ~c0f0dqb ").WriteInt(int(fogging.Dqb)).WriteString("\n~cfff")
	default:
		text.WriteString("dqa ").WriteInt(int(fogging.Dqa)).WriteString(" dqb ").WriteInt(int(fogging.Dqb)).WriteString("\n")
	}
	highlight := int8(3)
	if fogOption >= 3 {
		highlight = fogOption - 3
	}
	printColourDetails(int32(fogging.Rfc), int32(fogging.Gfc), int32(fogging.Bfc), highlight)

	if padStatus&yaroze.PADRright != 0 {
		fogOption++
		if fogOption > 5 {
			fogOption = 0
		}
		delay(10)
	}
	if padStatus&yaroze.PADRleft != 0 {
		fogOption--
		if fogOption < 0 {
			fogOption = 5
		}
		delay(10)
	}

	switch fogOption {
	case 0:
		if padStatus&(yaroze.PADRup|yaroze.PADRdown) != 0 {
			foggingOnOff ^= 1
			yaroze.GsSetLightMode(foggingOnOff)
			delay(10)
		}
	case 1:
		fogging.Dqa += int16(upDown(64))
	case 2:
		fogging.Dqb += upDown(4096)
	case 3:
		fogging.Rfc += uint8(upDown(1))
	case 4:
		fogging.Gfc += uint8(upDown(1))
	case 5:
		fogging.Bfc += uint8(upDown(1))
	}
}

var (
	explosionSpeed  int8 = 7
	explosionOption int8
)

// controlsExplosion runs the explosion while it is on, over 314 steps and
// round again, and lets Square and Circle choose between its speed and
// whether it is on, which Triangle and Cross change.
func controlsExplosion() {
	switch {
	case bangCount < 314 && bang:
		blowVertices(&object[dispObj], uint8(explosionSpeed), bangCount)
		bangCount++
	case bang:
		bangCount = 0
		resetColors(tmdAddress, copyTmd, dispObj)
		resetVertices(tmdAddress, copyTmd, dispObj)
	}

	onOff := "Bang Off\n"
	if bang {
		onOff = "Bang On\n"
	}
	switch explosionOption {
	case 0:
		text.WriteString("~c0f0Speed ").WriteInt(int(explosionSpeed)).WriteString("\n~cfff").WriteString(onOff)
	case 1:
		text.WriteString("Speed ").WriteInt(int(explosionSpeed)).WriteString("\n~c0f0").WriteString(onOff[:len(onOff)-1]).WriteString("~cfff\n")
	}

	if padStatus&(yaroze.PADRright|yaroze.PADRleft) != 0 {
		explosionOption ^= 1
		delay(10)
	}

	switch explosionOption {
	case 0:
		if padStatus&yaroze.PADRup != 0 {
			explosionSpeed++
			if explosionSpeed > 13 {
				explosionSpeed = 1
			}
			delay(10)
		}
		if padStatus&yaroze.PADRdown != 0 {
			explosionSpeed--
			if explosionSpeed < 1 {
				explosionSpeed = 13
			}
			delay(10)
		}
	case 1:
		if padStatus&(yaroze.PADRup|yaroze.PADRdown) != 0 {
			bang = !bang
			bangCount = 0
			resetColors(tmdAddress, copyTmd, dispObj)
			resetVertices(tmdAddress, copyTmd, dispObj)
			delay(10)
		}
	}
}
