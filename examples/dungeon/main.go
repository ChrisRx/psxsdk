// A random maze of stone walls walked in the first person, built around the
// viewpoint every frame, in blue fog.
//
// A port of "dungeon" (Mizuki, 1996), a demo posted to the Japanese Net
// Yaroze members' forum, in the form the LPGE team's copy of it took, with
// a ship added: a maze of stone walls, laid out at random in blocks of three
// cells, walked through in the first person. Only the cells around the
// viewpoint are given walls, a ceiling and a floor each frame, as objects
// in their own coordinate systems, and the nearest are subdivided. A line
// and two boxes in the corner show which way the viewpoint faces, and the
// library's font prints where it is. Walls stop the viewpoint, blue fog
// closes in behind them, and two lights follow the view, one from above
// and one from below.
//
// Differences from the original:
//
//   - The data is read off the disc to the addresses the original's batch
//     script sent it to. The cube model the script also sends, which the
//     program never uses, is left out.
//   - The objects a frame draws are bounded by their array's size, 256.
//     The original goes on filling past its end when more of the maze is
//     in view than that, over the coordinate systems next to it.
//   - The checks for the edge of the maze come before the reads they
//     guard, which the original makes a cell past the edge first.
//   - Quitting returns to the BIOS. The original calls exit.
//
// # Controls
//
// The directional buttons turn and walk, L1 and R1 sidestep, L2 and R2 look
// up and down, and Circle makes everything faster. Start switches to
// steering the ship, which sits near the start: the directional buttons and
// L1 and L2 move it along its own axes, the symbol buttons and R1 and R2
// spin it about them, and Select makes those faster. Start and Select
// together quit.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/examples/internal/lpge"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	screenW, screenH = 320, 240
	packetMax        = 2048 * 24
	objMax           = 256
	otLength         = 9
)

// The models and the texture, read into the program's own memory, in
// buffers of whole sectors.
var (
	wall0Data [(112 + 2047) / 2048 * 512]uint32
	wall1Data [(116 + 2047) / 2048 * 512]uint32
	wall2Data [(116 + 2047) / 2048 * 512]uint32
	wall3Data [(116 + 2047) / 2048 * 512]uint32
	tenData   [(112 + 2047) / 2048 * 512]uint32
	yukaData  [(112 + 2047) / 2048 * 512]uint32
	shipData  [(2044 + 2047) / 2048 * 512]uint32
	stoneData [(2112 + 2047) / 2048 * 512]uint32

	wall0File = disc.Register("ASSETS/WALL0.TMD", "assets/WALL0.TMD").Into(wall0Data[:])
	wall1File = disc.Register("ASSETS/WALL1.TMD", "assets/WALL1.TMD").Into(wall1Data[:])
	wall2File = disc.Register("ASSETS/WALL2.TMD", "assets/WALL2.TMD").Into(wall2Data[:])
	wall3File = disc.Register("ASSETS/WALL3.TMD", "assets/WALL3.TMD").Into(wall3Data[:])
	tenFile   = disc.Register("ASSETS/TEN.TMD", "assets/TEN.TMD").Into(tenData[:])
	yukaFile  = disc.Register("ASSETS/YUKA.TMD", "assets/YUKA.TMD").Into(yukaData[:])
	shipFile  = disc.Register("ASSETS/THESHIP.TMD", "assets/THESHIP.TMD").Into(shipData[:])
	stoneFile = disc.Register("ASSETS/STONE.TIM", "assets/STONE.TIM").Into(stoneData[:])
	files     = []*disc.File{&wall0File, &wall1File, &wall2File, &wall3File, &tenFile, &yukaFile, &shipFile, &stoneFile}

	shipTMD  = yaroze.TMD(shipData[:])
	stoneTIM = yaroze.TIM(stoneData[:])
)

// The maze: cells fact across, in a grid worldX by worldZ, each a wall or
// not. The viewpoint stays west short of a wall, at posY above the floor.
const (
	fact   = 128
	worldX = 60
	worldZ = 60
	west   = 112
	posY   = 28
	wall   = 0x01
)

// What the pad steers.
const (
	moveView = iota
	moveShip
)

// The faces a cell may be given, each a model: the walls on the cell's
// four sides, the ceiling and the floor.
const (
	backWall = iota
	frontWall
	rightWall
	leftWall
	ceiling
	floor
)

var (
	sot   [2]yaroze.GsOT // the compass's
	stags [2][16]yaroze.GsOT_TAG
	wot   [2]yaroze.GsOT // the world's
	wtags [2][1 << otLength]yaroze.GsOT_TAG

	models  [objMax]yaroze.GsDOBJ2
	dworld  [objMax]yaroze.GsCOORDINATE2
	nModels int

	// faces holds each face's model, mapped.
	faces [6]yaroze.TMD

	// pvect is the view's angles: vy round the vertical, vx up and down.
	pvect      yaroze.SVECTOR
	view       yaroze.GsRVIEW2
	lights     [3]yaroze.GsF_LIGHT
	fogging    yaroze.GsFOGPARAM
	lightMode  int32
	packetArea [2][packetMax / 4]uint32

	line yaroze.GsLINE
	box  [2]yaroze.GsBOXF

	worldMaps [worldX][worldZ]byte
	world     [worldX][worldZ]yaroze.GsCOORDINATE2

	mainMode    int
	frameNumber int32

	textBuf [192]byte
	text    yaroze.Buffer
)

// The ship, steered about by its own axes.
var ship struct {
	coord                              yaroze.GsCOORDINATE2
	position, velocity, twist          yaroze.VECTOR
	handler                            yaroze.GsDOBJ2
	movementMomentum, rotationMomentum bool
	matrix                             yaroze.MATRIX
	movementSpeed, rotationSpeed       int32
}

func main() {
	text.Init(textBuf[:])
	if !initializeAll() {
		return
	}

	var hsync int32
	side := yaroze.GsGetActiveBuff()
	for {
		text.Reset()
		text.WriteString("X = ").WriteInt(view.Vpx).WriteString("\nZ = ").WriteInt(view.Vpz).
			WriteString("\nV = ").WriteInt(pvect.Vy).WriteString("\n").Print()
		text.Reset()
		text.WriteString("Num models: ").WriteInt(nModels).WriteString("\nHsync = ").WriteInt(hsync).WriteString("\n").Print()
		text.Reset()
		text.WriteString("frame: ").WriteInt(frameNumber).WriteString("\nmode: ")
		switch mainMode {
		case moveView:
			text.WriteString("move view\n")
		case moveShip:
			text.WriteString("move ship\n")
		}
		text.Print()
		text.Reset()
		text.WriteString("ship pos: \n").WriteInt(ship.position.Vx).WriteString(" ").
			WriteInt(ship.position.Vy).WriteString(" ").WriteInt(ship.position.Vz).WriteString("\n").Print()
		frameNumber++

		if dealWithControllerPad() {
			break
		}
		moveLightingSpot()
		nModels = calcWorldData()

		yaroze.GsSetRefView2(&view)
		yaroze.GsSetWorkBase(yaroze.Packets(&packetArea[side][0]))
		yaroze.GsClearOt(0, 0, &wot[side])
		yaroze.GsClearOt(0, 0, &sot[side])
		for i := range nModels {
			op := &models[i]
			var lw, ls yaroze.MATRIX
			yaroze.GsGetLws(op.Coord2, &lw, &ls)
			yaroze.GsSetLightMatrix(&lw)
			yaroze.GsSetLsMatrix(&ls)
			yaroze.GsSortObject4(op, &wot[side], 3, yaroze.Scratch())
		}

		updateTheShip()
		var ls yaroze.MATRIX
		yaroze.GsGetLs(&ship.coord, &ls)
		yaroze.GsSetLightMatrix(&ls)
		yaroze.GsSetLsMatrix(&ls)
		yaroze.GsSortObject4(&ship.handler, &wot[side], 3, yaroze.Scratch())

		drawWorldMaps(&sot[side])
		hsync = yaroze.VSync(2)
		yaroze.ResetGraph(1)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 4, &wot[side])
		yaroze.GsDrawOt(&wot[side])
		yaroze.GsDrawOt(&sot[side])
		side ^= 1
		yaroze.FntFlush(-1)
	}
	yaroze.ResetGraph(3)
}

// drawWorldMaps draws the compass: a line from a box, the way the view
// faces, over a larger, see-through one.
func drawWorldMaps(ot *yaroze.GsOT) {
	line.Attribute = 0
	line.R, line.G, line.B = 128, 128, 128
	line.X0, line.Y0 = screenW/2-48, -screenH/2+48
	line.X1 = line.X0 + int16(rsin(int32(pvect.Vy))/256)
	line.Y1 = line.Y0 - int16(rcos(int32(pvect.Vy))/256)

	box[0].X, box[0].Y, box[0].W, box[0].H = line.X0-16, line.Y0-16, 32, 32
	box[0].X += int16(rsin(int32(pvect.Vy)) / 256)
	box[0].Y -= int16(rcos(int32(pvect.Vy)) / 256)
	box[0].R, box[0].G, box[0].B = 0, 0, 128

	box[1].Attribute = yaroze.GsALON
	box[1].X, box[1].Y, box[1].W, box[1].H = line.X0-32, line.Y0-32, 64, 64
	box[1].R, box[1].G, box[1].B = 128, 128, 128

	yaroze.GsSortLine(&line, ot, 2)
	yaroze.GsSortBoxFill(&box[0], ot, 2)
	yaroze.GsSortBoxFill(&box[1], ot, 2)
}

const speed = 2

// controller is the pad as the frame sees it, which tells the press of
// Start that switches modes from the frames it stays held.
var controller yaroze.Pad

// dealWithControllerPad acts on the pad, and reports whether to quit.
func dealWithControllerPad() bool {
	pad := controller.Update()

	switch mainMode {
	case moveView:
		spd := int32(speed)
		if pad&yaroze.PADRright != 0 {
			spd *= 4
		}

		// Left and right turn.
		if pad&yaroze.PADLleft != 0 {
			pvect.Vy = int16((int32(pvect.Vy) - spd*8) & 4095)
		}
		if pad&yaroze.PADLright != 0 {
			pvect.Vy = int16((int32(pvect.Vy) + spd*8) & 4095)
		}

		// R2 and L2 look up and down.
		if pad&yaroze.PADR2 != 0 && int32(pvect.Vx)+spd*8 < 960 {
			pvect.Vx += int16(spd * 8)
		}
		if pad&yaroze.PADL2 != 0 && int32(pvect.Vx)-spd*8 > -960 {
			pvect.Vx -= int16(spd * 8)
		}

		// Up and down walk; R1 and L1 sidestep, a quarter turn either way.
		var vx, vz int32
		if pad&yaroze.PADLup != 0 {
			vz += rcos(int32(pvect.Vy)) / 64 * spd / 16
			vx += rsin(int32(pvect.Vy)) / 64 * spd / 16
		}
		if pad&yaroze.PADLdown != 0 {
			vz -= rcos(int32(pvect.Vy)) / 64 * spd / 16
			vx -= rsin(int32(pvect.Vy)) / 64 * spd / 16
		}
		if pad&yaroze.PADR1 != 0 {
			vz += rcos((int32(pvect.Vy)+1024)&4095) / 64 * spd / 16
			vx += rsin((int32(pvect.Vy)+1024)&4095) / 64 * spd / 16
		}
		if pad&yaroze.PADL1 != 0 {
			vz += rcos((int32(pvect.Vy)-1024)&4095) / 64 * spd / 16
			vx += rsin((int32(pvect.Vy)-1024)&4095) / 64 * spd / 16
		}

		// Not into walls.
		if vz > 0 && (collision(0, vz+west) || collision(-west, vz+west) || collision(west, vz+west)) {
			vz = 0
		}
		if vz < 0 && (collision(0, vz-west) || collision(-west, vz-west) || collision(west, vz-west)) {
			vz = 0
		}
		if vx > 0 && (collision(vx+west, 0) || collision(vx+west, west) || collision(vx+west, -west)) {
			vx = 0
		}
		if vx < 0 && (collision(vx-west, 0) || collision(vx-west, west) || collision(vx-west, -west)) {
			vx = 0
		}
		view.Vpz += vz
		view.Vpx += vx

		rng := rcos(int32(pvect.Vx))
		view.Vrz = view.Vpz + rcos(int32(pvect.Vy))*rng/yaroze.ONE
		view.Vrx = view.Vpx + rsin(int32(pvect.Vy))*rng/yaroze.ONE
		view.Vry = rsin(int32(pvect.Vx)) + posY

	case moveShip:
		multiplier := int32(1)
		if pad&yaroze.PADselect != 0 {
			multiplier = 10
		}
		by := ship.movementSpeed * multiplier
		if pad&yaroze.PADLright != 0 {
			ship.velocity.Vx -= by
		}
		if pad&yaroze.PADLleft != 0 {
			ship.velocity.Vx += by
		}
		if pad&yaroze.PADLup != 0 {
			ship.velocity.Vy -= by
		}
		if pad&yaroze.PADLdown != 0 {
			ship.velocity.Vy += by
		}
		if pad&yaroze.PADL2 != 0 {
			ship.velocity.Vz -= by
		}
		if pad&yaroze.PADL1 != 0 {
			ship.velocity.Vz += by
		}

		turn := ship.rotationSpeed * multiplier
		if pad&yaroze.PADRright != 0 {
			ship.twist.Vy -= turn
		}
		if pad&yaroze.PADRleft != 0 {
			ship.twist.Vy += turn
		}
		if pad&yaroze.PADRup != 0 {
			ship.twist.Vx -= turn
		}
		if pad&yaroze.PADRdown != 0 {
			ship.twist.Vx += turn
		}
		if pad&yaroze.PADR2 != 0 {
			ship.twist.Vz -= turn
		}
		if pad&yaroze.PADR1 != 0 {
			ship.twist.Vz += turn
		}
	}

	// Start switches what is steered, once per press.
	if controller.Pressed(yaroze.PADstart) {
		mainMode = 1 - mainMode
	}

	if pad&yaroze.PADstart != 0 && pad&yaroze.PADselect != 0 {
		yaroze.DrawSync(0)
		return true
	}
	return false
}

// collision reports whether the cell vx, vz from the viewpoint is a wall,
// or off the maze.
func collision(vx, vz int32) bool {
	wx := view.Vpx + fact/2 + vx
	wz := view.Vpz + fact/2 + vz
	if wx < 0 || wz < 0 {
		return true
	}
	wx, wz = wx/fact, wz/fact
	if wx >= worldX || wz >= worldZ {
		return true
	}
	return worldMaps[wx][wz] != 0
}

// moveLightingSpot points the two lights the way the view faces, one from
// above and one from below.
func moveLightingSpot() {
	lights[0].R, lights[0].G, lights[0].B = 0x70, 0x70, 0x70
	lights[0].Vx = view.Vrx - view.Vpx
	lights[0].Vy = 4096
	lights[0].Vz = view.Vrz - view.Vpz
	yaroze.GsSetFlatLight(0, &lights[0])

	lights[1].R, lights[1].G, lights[1].B = 0x70, 0x70, 0x70
	lights[1].Vx = view.Vrx - view.Vpx
	lights[1].Vy = -4096
	lights[1].Vz = view.Vrz - view.Vpz
	yaroze.GsSetFlatLight(1, &lights[1])
}

// calcWorldData gives the cells around the viewpoint, shifted the way it
// faces, their faces, and returns how many objects that makes. The cells
// nearest the viewpoint are subdivided.
func calcWorldData() int {
	px := (view.Vpx + fact/2) / fact
	py := (view.Vpz + fact/2) / fact

	sx, ex := px-4, px+4
	sy, ey := py-4, py+4
	sx += rsin(int32(pvect.Vy)) / 1000
	sy += rcos(int32(pvect.Vy)) / 1000
	ex += rsin(int32(pvect.Vy)) / 1000
	ey += rcos(int32(pvect.Vy)) / 1000
	sx, sy = max(sx, 0), max(sy, 0)
	ex, ey = min(ex, worldX-1), min(ey, worldZ-1)

	n := 0
	for x := sx; x <= ex; x++ {
		for y := sy; y <= ey; y++ {
			if abs(px-x) < 3 && abs(py-y) < 3 {
				n = calcModelData(n, x, y, yaroze.GsDIV1)
			} else {
				n = calcModelData(n, x, y, 0)
			}
		}
	}
	return n
}

// calcModelData adds the faces of the cell px, py to the objects, from the
// nth on, and returns the count after them: a wall on each side with a
// wall or the maze's edge beyond it, the ceiling and the floor.
func calcModelData(n int, px, py int32, attribute uint32) int {
	if worldMaps[px][py]&wall != 0 {
		return n
	}
	if px == 0 || worldMaps[px-1][py]&wall != 0 {
		n = addFace(n, px, py, leftWall, 0, attribute)
	}
	if px == worldX-1 || worldMaps[px+1][py]&wall != 0 {
		n = addFace(n, px, py, rightWall, 0, attribute)
	}
	if py == 0 || worldMaps[px][py-1]&wall != 0 {
		n = addFace(n, px, py, backWall, 0, attribute)
	}
	if py == worldZ-1 || worldMaps[px][py+1]&wall != 0 {
		n = addFace(n, px, py, frontWall, 0, attribute)
	}
	n = addFace(n, px, py, ceiling, -fact, attribute)
	n = addFace(n, px, py, floor, fact, attribute)
	return n
}

// addFace makes object n a face of the cell px, py, at height y in the
// cell's coordinate system, and returns n+1; or n, with no object, once
// the objects are used up.
func addFace(n int, px, py int32, face int, y int32, attribute uint32) int {
	if n >= objMax {
		return n
	}
	yaroze.GsInitCoordinate2(&world[px][py], &dworld[n])
	dworld[n].Coord.T = [3]int32{0, y, 0}
	faces[face].Link(&models[n], 0)
	models[n].Coord2 = &dworld[n]
	models[n].Attribute = attribute
	return n + 1
}

func initializeAll() bool {
	yaroze.PadInit()

	yaroze.GsInitGraph(screenW, screenH, yaroze.GsINTER|yaroze.GsOFSGPU, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, screenH)
	yaroze.GsInit3D()

	for i := range wot {
		wot[i].Length = otLength
		wot[i].Org = &wtags[i][0]
		sot[i].Length = 4
		sot[i].Org = &stags[i][0]
		yaroze.GsClearOt(0, 0, &wot[i])
		yaroze.GsClearOt(0, 0, &sot[i])
	}
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(0, 0, 256, 200, 0, 512)

	if missing := demo.Load(files, nil); missing != "" {
		fail("could not read\n", missing)
		return false
	}

	initializeView()
	initializeLighting()

	stoneTIM.Load()
	faces[backWall] = modelInit(wall0Data[:])
	faces[frontWall] = modelInit(wall1Data[:])
	faces[rightWall] = modelInit(wall2Data[:])
	faces[leftWall] = modelInit(wall3Data[:])
	faces[ceiling] = modelInit(tenData[:])
	faces[floor] = modelInit(yukaData[:])

	makeWorldMaps()
	clearViewPoint()
	nModels = calcWorldData()

	initializeTheShip()
	mainMode = moveView
	return true
}

// fail shows what went wrong until the program is quit.
func fail(what, detail string) {
	side := yaroze.GsGetActiveBuff()
	for yaroze.PadRead()&(yaroze.PADstart|yaroze.PADselect) != yaroze.PADstart|yaroze.PADselect {
		yaroze.GsSetWorkBase(yaroze.Packets(&packetArea[side][0]))
		yaroze.GsClearOt(0, 0, &wot[side])
		yaroze.VSync(0)
		yaroze.ResetGraph(1)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 4, &wot[side])
		yaroze.GsDrawOt(&wot[side])
		side ^= 1
		text.Reset()
		text.WriteString(what).WriteString(detail).Print()
		yaroze.FntFlush(-1)
	}
	yaroze.ResetGraph(3)
}

func initializeTheShip() {
	yaroze.GsInitCoordinate2(nil, &ship.coord)
	ship.position = yaroze.VECTOR{Vx: 4000, Vy: 0, Vz: 4000} // near the start
	ship.velocity = yaroze.VECTOR{}
	ship.twist = yaroze.VECTOR{}
	ship.movementMomentum = false
	ship.rotationMomentum = false
	ship.movementSpeed = 10
	ship.rotationSpeed = 5
	lpge.LinkObjectHandlerToTmdObject(&ship.handler, 0, shipTMD)
	ship.matrix = yaroze.MATRIX{M: [3][3]int16{
		{yaroze.ONE, 0, 0},
		{0, yaroze.ONE, 0},
		{0, 0, yaroze.ONE},
	}}
	ship.handler.Coord2 = &ship.coord
}

func updateTheShip() {
	updateObjectCoordinates(&ship.twist, &ship.position, &ship.velocity, &ship.coord)
	if !ship.movementMomentum {
		ship.velocity = yaroze.VECTOR{}
	}
	if !ship.rotationMomentum {
		ship.twist = yaroze.VECTOR{}
	}
}

// updateObjectCoordinates moves the ship by its velocity along its own axes
// and turns it by its twist about them, keeping its orientation in its
// matrix, and sets its coordinate system from the two.
func updateObjectCoordinates(twist, position, velocity *yaroze.VECTOR, c *yaroze.GsCOORDINATE2) {
	var realMovement yaroze.VECTOR
	yaroze.ApplyMatrixLV(&ship.matrix, velocity, &realMovement)
	position.Vx += realMovement.Vx
	position.Vy += realMovement.Vy
	position.Vz += realMovement.Vz

	xVector := yaroze.SVECTOR{Vx: int16(twist.Vx)}
	yVector := yaroze.SVECTOR{Vy: int16(twist.Vy)}
	zVector := yaroze.SVECTOR{Vz: int16(twist.Vz)}
	var xMatrix, yMatrix, zMatrix, copy yaroze.MATRIX
	yaroze.RotMatrix(&xVector, &xMatrix)
	yaroze.RotMatrix(&yVector, &yMatrix)
	yaroze.RotMatrix(&zVector, &zMatrix)

	yaroze.MulMatrix0(&ship.matrix, &xMatrix, &copy)
	yaroze.MulMatrix0(&copy, &yMatrix, &ship.matrix)
	yaroze.MulMatrix0(&ship.matrix, &zMatrix, &copy)
	ship.matrix = copy

	c.Coord = ship.matrix
	c.Coord.T = [3]int32{position.Vx, position.Vy, position.Vz}
	c.Flg = 0
}

func initializeView() {
	yaroze.GsSetProjection(192)
	pvect = yaroze.SVECTOR{}
	view = yaroze.GsRVIEW2{
		Vpx: worldX / 2 * fact, Vpy: posY, Vpz: worldZ / 2 * fact,
		Vrx: 0, Vry: posY, Vrz: 0,
		Rz: 0, Super: nil,
	}
	yaroze.GsSetRefView2(&view)
}

func initializeLighting() {
	lights[0] = yaroze.GsF_LIGHT{Vx: 1, Vy: 1, Vz: 1, R: 0x80, G: 0x80, B: 0x80}
	yaroze.GsSetFlatLight(0, &lights[0])
	lights[1] = yaroze.GsF_LIGHT{Vx: -1, Vy: -1, Vz: 1, R: 0x80, G: 0x80, B: 0x80}
	yaroze.GsSetFlatLight(1, &lights[1])
	yaroze.GsSetAmbient(yaroze.ONE/4, yaroze.ONE/4, yaroze.ONE/4)

	lightMode = 1
	yaroze.GsSetLightMode(lightMode)

	fogging = yaroze.GsFOGPARAM{Dqa: -960, Dqb: 5120 * 5120, Rfc: 0, Gfc: 0, Bfc: 4}
	yaroze.GsSetFogParam(&fogging)
}

// modelInit maps a TMD's offsets to addresses and returns it, for linking.
func modelInit(tmd yaroze.TMD) yaroze.TMD {
	tmd.Map()
	return tmd
}

// makeWorldMaps lays the maze out: blocks of three cells by three, a wall
// one time in four, and a coordinate system for every cell.
func makeWorldMaps() {
	for x := int32(0); x < worldX; x += 3 {
		for y := int32(0); y < worldZ; y += 3 {
			var cell byte
			if yaroze.Rand()%4 == 0 {
				cell = wall
			}
			for dx := int32(0); dx < 3; dx++ {
				for dy := int32(0); dy < 3; dy++ {
					worldMaps[x+dx][y+dy] = cell
				}
			}
		}
	}
	for x := int32(0); x < worldX; x++ {
		for y := int32(0); y < worldZ; y++ {
			yaroze.GsInitCoordinate2(nil, &world[x][y])
			world[x][y].Coord.T = [3]int32{x * fact, 0, y * fact}
		}
	}
}

// clearViewPoint knocks down the walls within four cells of the viewpoint.
func clearViewPoint() {
	px := view.Vpx / fact
	py := view.Vpz / fact
	sx, sy := max(px-4, 0), max(py-4, 0)
	ex, ey := min(px+4, worldX-1), min(py+4, worldZ-1)
	for x := sx; x <= ex; x++ {
		for y := sy; y <= ey; y++ {
			worldMaps[x][y] = 0
		}
	}
}

func abs(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
