// The clone sample, in Go.
//
// A port of Sony's Net Yaroze demo "clone" (S. Ashley, 1997): a first-person
// maze shooter. Walk a fogged, textured maze, shoot the monsters before they
// shoot you, pick up first aid, find the pulsing exit, and do it again on
// the next of three levels. A heart beats louder as monsters get closer.
//
// It puts much of the machine to work at once: the GTE and the 3D library,
// with fog and subdivision, models built in memory, sprites, lines, filled
// boxes and text, a palette rewritten every frame, and sound effects played
// straight on SPU voices, which the check example's sequenced music does
// not use.
//
// Controls: the d-pad walks and turns, L1 and R1 sidestep, X fires, and
// circle shows the map of where you have been. Hold Start and Select
// together to quit.
//
// Differences from the original:
//
//   - The sound bank, maps and textures are read off the disc. On a Net
//     Yaroze the loader's batch script sent them from the PC to fixed
//     addresses; they are loaded to the same addresses.
//   - Every object has a real coordinate system. The original points them
//     all at WORLD, the library's nil coordinate system, and writes each
//     object's transformation through that pointer, which in C silently
//     overwrites the kernel's memory at address 0. They share one here, as
//     they shared that address there.
//   - Reading the map outside its bounds reads as no walls. The original
//     reads whatever memory is next to the map, which Go does not allow.
//   - The heartbeat's volume starts from zero. The original compares against
//     a variable it never sets.
//   - It shows a loading message, and a report if the data is missing,
//     where the original would hang or exit.
//
// Nothing allocates after startup.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	packetMax = 5000 * 24 // GPU packet space per buffer

	maxMapRows = 20 // the most rows a map can have
	maxMapCols = 20 // and columns
	maxBadGuys = 20 // the most monsters
	maxShots   = 6  // shots of yours in the air at once

	lengthOT1 = 1  // the ordering table for floors and roofs, drawn first
	lengthOT2 = 10 // and for everything else

	border = 200 // how close you can come to a wall
)

// Where the data is loaded, as the demo's batch script loaded it.
const (
	vhAddr = 0x80090000
	vbAddr = 0x80092400

	map1Addr = 0x800c0000
	map2Addr = 0x800c0800
	map3Addr = 0x800c1000

	statusTIM   = 0x800c8000
	roofTIM     = 0x800d0000
	wallTIM     = 0x800d2000
	floorTIM    = 0x800d4000
	man1TIM     = 0x800da000
	man2TIM     = 0x800c2000
	deadmanTIM  = 0x800ca000
	bulletTIM   = 0x800d6000
	bloodTIM    = 0x800d8000
	ricochetTIM = 0x800e0000
	healthTIM   = 0x800e2000
	bullet2TIM  = 0x800e4000
	eroofTIM    = 0x800e6000
	ewallTIM    = 0x800e8000
	efloorTIM   = 0x800ea000
	fwallTIM    = 0x800ec000
	faceTIM     = 0x800ee000

	// The models the demo builds.
	floorTMD    = 0x800f0000
	roofTMD     = 0x800f1000
	wallTMD     = 0x800f2000
	monster1TMD = 0x800f3000
	monster2TMD = 0x800f4000
	bulletTMD   = 0x800f5000
	bloodTMD    = 0x800f6000
	deadTMD     = 0x800f7000
	ricochetTMD = 0x800f8000
	healthTMD   = 0x800f9000
	bullet2TMD  = 0x800fa000
	eroofTMD    = 0x800fb000
	ewallTMD    = 0x800fc000
	efloorTMD   = 0x800fd000
	fwallTMD    = 0x800fe000
)

var files = []demo.File{
	{Name: "\\DATA\\SFX.VH;1", Addr: vhAddr},
	{Name: "\\DATA\\SFX.VB;1", Addr: vbAddr},
	{Name: "\\DATA\\MAP1.DAT;1", Addr: map1Addr},
	{Name: "\\DATA\\MAP2.DAT;1", Addr: map2Addr},
	{Name: "\\DATA\\MAP3.DAT;1", Addr: map3Addr},
	{Name: "\\DATA\\STATUS.TIM;1", Addr: statusTIM},
	{Name: "\\DATA\\STONE07.TIM;1", Addr: roofTIM},
	{Name: "\\DATA\\STONE01.TIM;1", Addr: wallTIM},
	{Name: "\\DATA\\WALL03.TIM;1", Addr: floorTIM},
	{Name: "\\DATA\\ALBIOD_1.TIM;1", Addr: man1TIM},
	{Name: "\\DATA\\ALBIOD_2.TIM;1", Addr: man2TIM},
	{Name: "\\DATA\\BULLET.TIM;1", Addr: bulletTIM},
	{Name: "\\DATA\\BLOOD.TIM;1", Addr: bloodTIM},
	{Name: "\\DATA\\ALBIOD_3.TIM;1", Addr: deadmanTIM},
	{Name: "\\DATA\\RICOCHET.TIM;1", Addr: ricochetTIM},
	{Name: "\\DATA\\FIRSTAID.TIM;1", Addr: healthTIM},
	{Name: "\\DATA\\BULLET2.TIM;1", Addr: bullet2TIM},
	{Name: "\\DATA\\ESTONE07.TIM;1", Addr: eroofTIM},
	{Name: "\\DATA\\ESTONE01.TIM;1", Addr: ewallTIM},
	{Name: "\\DATA\\EWAVE01.TIM;1", Addr: efloorTIM},
	{Name: "\\DATA\\FSTONE01.TIM;1", Addr: fwallTIM},
	{Name: "\\DATA\\FACE.TIM;1", Addr: faceTIM},
}

// Map square flags.
const (
	north  = 1 << 0 // walls
	south  = 1 << 1
	east   = 1 << 2
	west   = 1 << 3
	health = 1 << 4 // a first aid box
	fake   = 1 << 5 // a block that shooting destroys
	exit   = 1 << 6 // the way out
)

type projectile struct {
	active    int16 // moving
	available int16 // ready to fire
	hit       int16 // has hit something
	vsync     int32 // when it hit, to leave the blood or ricochet a moment
	position  yaroze.VECTOR
	increment yaroze.VECTOR
	object    *yaroze.GsDOBJ2
}

type badGuy struct {
	active     int16 // moving around
	activeDist int32 // how near you come before it does
	direction  int16 // 0 north, 1 south, 2 east, 3 west, -1 none
	facing     int16
	health     int16
	hitsize    int16 // how big a target it is
	position   yaroze.VECTOR
	bullet     projectile // one shot at a time
	pvsync     int32      // frames between shots
	object     *yaroze.GsDOBJ2
}

type goodGuy struct {
	position yaroze.VECTOR
	health   int16
	kills    int16
	level    int16
	top      int32 // the bounce of walking: its limits and step
	bottom   int32
	bounce   int32
	bullet   [maxShots]projectile
}

// Ordering tables, the first for floors and roofs so they are drawn first,
// and GPU packet space, one of each per display buffer.
var (
	ot1       [2]yaroze.GsOT
	zTable1   [2][1 << lengthOT1]yaroze.GsOT_TAG
	ot2       [2]yaroze.GsOT
	zTable2   [2][1 << lengthOT2]yaroze.GsOT_TAG
	gpuPacket [2][packetMax]yaroze.PACKET
)

var padStatus uint32

// The model handles, which all share one coordinate system: drawObject
// sets it, and works out the screen matrix from it, as it draws each one.
var (
	floorOBJ, roofOBJ, wallOBJ                         yaroze.GsDOBJ2
	monster1OBJ, monster2OBJ, deadOBJ                  yaroze.GsDOBJ2
	bulletOBJ, bloodOBJ, ricochetOBJ, bullet2OBJ       yaroze.GsDOBJ2
	healthOBJ, efloorOBJ, eroofOBJ, ewallOBJ, fwallOBJ yaroze.GsDOBJ2
	objectCoord                                        yaroze.GsCOORDINATE2
)

var mainView yaroze.GsVIEW2

var (
	world   [maxMapRows][maxMapCols]uint16 // the map
	visited [maxMapRows][maxMapCols]uint8  // where you have seen
	mapRows uint8
	mapCols uint8

	badguy     [maxBadGuys]badGuy
	numBadGuys uint8
	goodguy    goodGuy

	present yaroze.GsCOORDINATE2 // the view from where you stand

	viewAngle int32
	buffIdx   int32

	vab          int16
	heartVoice   int16
	levelVoice   int16
	playVsync    int32
	playingMusic bool

	displayMap bool
)

// The exit's palettes, rewritten every frame to make it pulse.
var (
	exitFloorCLUTRect, exitWallCLUTRect, exitRoofCLUTRect yaroze.RECT
	exitFloorCLUT, exitWallCLUT, exitRoofCLUT             [16]uint16
	exitCLUTInc                                           int8 = 1
	exitCLUTVal                                           int8
)

var status, face yaroze.GsSPRITE

var (
	lineBuf [64]byte
	line    yaroze.Buffer
)

// mapAt reads a map square, or no walls outside the map.
func mapAt(r, c int32) uint16 {
	if r < 0 || r >= maxMapRows || c < 0 || c >= maxMapCols {
		return 0
	}
	return world[r][c]
}

func main() {
	line.Init(lineBuf[:])
	yaroze.ResetGraph(0)
	demo.PadInit()

	// 320 wide, offsets by the GPU, not interlaced, dithered.
	if yaroze.GetVideoMode() == yaroze.MODE_NTSC {
		yaroze.GsInitGraph(320, 240, 4, 1, 0)
	} else {
		yaroze.GsInitGraph(320, 256, 4, 1, 0)
	}
	yaroze.GsDefDispBuff(0, 0, 0, 256)
	yaroze.GsInit3D()

	// The demo opens a second stream for timing figures it never prints,
	// so only this one is opened, which makes it the one FntPrint uses.
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(-30, 91, 128, 8, 0, 32)

	for i := range 2 {
		ot1[i].Length = lengthOT1
		ot1[i].Org = &zTable1[i][0]
		ot2[i].Length = lengthOT2
		ot2[i].Org = &zTable2[i][0]
	}

	if missing := demo.Load(files, func(string) { message("loading") }); missing != "" {
		for demo.PadRead()&(demo.PADstart|demo.PADselect) != demo.PADstart|demo.PADselect {
			message("missing data")
		}
		yaroze.ResetGraph(1)
		return
	}

	setupTexturesAndTMDs()
	getExitCLUTRect()

	vab = yaroze.SsVabTransfer(demo.BytesAt(vhAddr), demo.BytesAt(vbAddr), -1, 1)
	if vab < 0 {
		for demo.PadRead()&(demo.PADstart|demo.PADselect) != demo.PADstart|demo.PADselect {
			message("no sound bank")
		}
		yaroze.ResetGraph(1)
		return
	}
	yaroze.SsUtKeyOn(vab, 10, 0, 60, 0, 100, 100)         // background noise
	heartVoice = yaroze.SsUtKeyOn(vab, 0, 0, 60, 0, 0, 0) // heartbeat

	goodguy.level = 1
	initMap(map1Addr)
	startLevelMusic()

	// No flat light, to clear any left from before; ambient light and fog
	// alone.
	var light yaroze.GsF_LIGHT
	yaroze.GsSetFlatLight(0, &light)
	yaroze.GsSetFlatLight(1, &light)
	yaroze.GsSetFlatLight(2, &light)
	yaroze.GsSetLightMatrix(&demo.IDMATRIX)

	yaroze.GsSetProjection(280)
	yaroze.GsSetLightMode(1) // with fog
	fog := yaroze.GsFOGPARAM{Dqa: -1400, Dqb: 20971520}
	yaroze.GsSetFogParam(&fog)
	yaroze.GsSetAmbient(yaroze.ONE/2, yaroze.ONE/2, yaroze.ONE/2)

	scale := yaroze.SVECTOR{Vx: yaroze.ONE, Vy: 3 * yaroze.ONE / 4, Vz: yaroze.ONE}

	for {
		// The view: from where you stand, turned the way you face.
		transform := demo.IDMATRIX
		yaroze.TransMatrix(&transform, &goodguy.position)
		present.Flg = 0
		present.Coord = transform
		present.Super = demo.WORLD

		transform = demo.IDMATRIX
		rotate := yaroze.SVECTOR{Vy: int16(viewAngle)}
		yaroze.RotMatrix(&rotate, &transform)

		mainView.View = transform
		mainView.Super = &present
		yaroze.GsSetView2(&mainView)

		buffIdx = yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(&gpuPacket[buffIdx][0])
		yaroze.GsClearOt(0, 0, &ot1[buffIdx])
		yaroze.GsClearOt(0, 0, &ot2[buffIdx])

		yaroze.GsScaleScreen(&scale)

		checkForHealthPickUp()
		checkGoodGuyBullets()
		poundingHeart()
		checkBadGuys()
		checkLevel()
		animateBadGuys()
		drawWalls()
		drawHealthBox(int32(goodguy.health))
		yaroze.GsSortFastSprite(&face, &ot2[buffIdx], 0)
		yaroze.GsSortFastSprite(&status, &ot2[buffIdx], 0)

		if goodguy.health <= 0 { // back to the beginning
			goodguy.level = 1
			initMap(map1Addr)
			startLevelMusic()
		}

		if displayMap {
			drawMap(int16(viewAngle), goodguy.position)
		}

		grabExitCLUT()

		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()

		yaroze.GsSortClear(0, 0, 0, &ot1[buffIdx])
		yaroze.GsDrawOt(&ot1[buffIdx]) // floors and roofs
		yaroze.GsDrawOt(&ot2[buffIdx]) // walls, shots, monsters

		padStatus = demo.PadRead()

		// The level, in red, and kills out of monsters.
		line.Reset()
		line.WriteString("~cF00").WriteInt(int(goodguy.level)).
			WriteString("         ").WriteInt(int(goodguy.kills)).
			WriteString("/").WriteInt(int(numBadGuys))
		line.Print()

		alterExitCLUT()

		displayMap = false
		viewControl()

		yaroze.FntFlush(-1)

		if padStatus&demo.PADselect != 0 && padStatus&demo.PADstart != 0 {
			break
		}
	}

	yaroze.SsUtAllKeyOff(0)
	yaroze.SsVabClose(vab)
	yaroze.ResetGraph(1)
}

// startLevelMusic plays a level's opening music, which checkLevel stops
// after five seconds.
func startLevelMusic() {
	levelVoice = yaroze.SsUtKeyOn(vab, 11, 0, 60, 0, 127, 127)
	playVsync = yaroze.VSync(-1)
	playingMusic = true
}

// message draws a frame with text in the status line.
func message(text string) {
	buff := yaroze.GsGetActiveBuff()
	yaroze.GsSetWorkBase(&gpuPacket[buff][0])
	yaroze.GsClearOt(0, 0, &ot1[buff])
	yaroze.DrawSync(0)
	yaroze.VSync(0)
	yaroze.GsSwapDispBuff()
	yaroze.GsSortClear(0, 0, 0, &ot1[buff])
	yaroze.GsDrawOt(&ot1[buff])
	line.Reset()
	line.WriteString(text)
	line.Print()
	yaroze.FntFlush(-1)
}

// initialiseTMD ties a handle to one of a built TMD's objects.
func initialiseTMD(addr uintptr, objNum int32, obj *yaroze.GsDOBJ2) {
	yaroze.GsLinkObject4(uint32(addr+12), obj, objNum)
	obj.Coord2 = &objectCoord
}

// initialiseSprite loads a TIM and sets up a sprite to show it, as the
// demo does, quirks included: it takes the texture's u from the low five
// bits of its x.
func initialiseSprite(timAddr uintptr, sprite *yaroze.GsSPRITE) {
	image := demo.LoadTIM(timAddr)
	pmode := image.Pmode & 3
	sprite.Attribute |= pmode << 24
	sprite.W = image.Pw << (2 - pmode)
	sprite.H = image.Ph
	sprite.U = uint8(image.Px & 31)
	sprite.V = uint8(image.Py & 511)
	sprite.Cx = image.Cx
	sprite.Cy = image.Cy
	sprite.R, sprite.G, sprite.B = 100, 100, 100
	sprite.Tpage = uint16(image.Px>>6) + uint16(image.Py>>10)
}

func setupTexturesAndTMDs() {
	for _, tim := range [...]uintptr{
		roofTIM, wallTIM, floorTIM, man1TIM, man2TIM, deadmanTIM,
		bulletTIM, bloodTIM, ricochetTIM, healthTIM, bullet2TIM,
		eroofTIM, ewallTIM, efloorTIM, fwallTIM,
	} {
		demo.LoadTIM(tim)
	}
	initialiseSprite(statusTIM, &status)
	status.X = int16(-int32(status.W) >> 1)
	if yaroze.GetVideoMode() == yaroze.MODE_NTSC {
		status.Y = 88
	} else {
		status.Y = 96
	}
	initialiseSprite(faceTIM, &face)
	face.X = status.X + 16
	face.Y = status.Y + 4

	vert := func(x, y, z int16) demo.TMD_VERT { return demo.TMD_VERT{Vx: x, Vy: y, Vz: z} }
	norm := func(x, y, z int16) demo.TMD_NORM { return demo.TMD_NORM{Nx: x, Ny: y, Nz: z} }
	square := func(n int16) quad { return quad{{0, 0}, {uint16(n), 0}, {0, uint16(n)}, {uint16(n), uint16(n)}} }

	// Floors.
	nm := norm(0, yaroze.ONE, 0)
	v := [4]demo.TMD_VERT{vert(-512, 0, -512), vert(-512, 0, 512), vert(512, 0, -512), vert(512, 0, 512)}
	makeSplitTMD_G_4T(floorTMD, floorTIM, square(63), nm, v, -1)
	initialiseTMD(floorTMD, 0, &floorOBJ)
	makeSplitTMD_G_4T(efloorTMD, efloorTIM, square(63), nm, v, -1)
	initialiseTMD(efloorTMD, 0, &efloorOBJ)

	// Roofs.
	nm = norm(0, -yaroze.ONE, 0)
	v = [4]demo.TMD_VERT{vert(-512, 0, -512), vert(512, 0, -512), vert(-512, 0, 512), vert(512, 0, 512)}
	makeSplitTMD_G_4T(roofTMD, roofTIM, square(63), nm, v, -1)
	initialiseTMD(roofTMD, 0, &roofOBJ)
	makeSplitTMD_G_4T(eroofTMD, eroofTIM, square(63), nm, v, -1)
	initialiseTMD(eroofTMD, 0, &eroofOBJ)

	// Walls.
	nm = norm(0, 0, -yaroze.ONE)
	v = [4]demo.TMD_VERT{vert(-512, -512, 0), vert(512, -512, 0), vert(-512, 512, 0), vert(512, 512, 0)}
	makeSplitTMD_G_4T(wallTMD, wallTIM, square(63), nm, v, -1)
	initialiseTMD(wallTMD, 0, &wallOBJ)
	makeSplitTMD_G_4T(ewallTMD, ewallTIM, square(63), nm, v, -1)
	initialiseTMD(ewallTMD, 0, &ewallOBJ)
	makeSplitTMD_G_4T(fwallTMD, fwallTIM, square(63), nm, v, -1)
	initialiseTMD(fwallTMD, 0, &fwallOBJ)

	// The monster, the same quad for both frames of its walk.
	nm = norm(0, 0, -yaroze.ONE)
	v = [4]demo.TMD_VERT{vert(-384, -768, 0), vert(384, -768, 0), vert(-384, 0, 0), vert(384, 0, 0)}
	makeTMD_G_4T(monster1TMD, man1TIM, square(127), nm, v, -1)
	initialiseTMD(monster1TMD, 0, &monster1OBJ)
	makeTMD_G_4T(monster2TMD, man2TIM, square(127), nm, v, -1)
	initialiseTMD(monster2TMD, 0, &monster2OBJ)

	// A dead monster, lying down.
	nm = norm(0, yaroze.ONE, 0)
	v = [4]demo.TMD_VERT{vert(-384, -50, -384), vert(-384, -50, 384), vert(384, -50, -384), vert(384, -50, 384)}
	makeTMD_G_4T(deadTMD, deadmanTIM, square(127), nm, v, -1)
	initialiseTMD(deadTMD, 0, &deadOBJ)

	// Shots, blood and ricochets: the same quad, textured differently.
	nm = norm(0, 0, -yaroze.ONE)
	v = [4]demo.TMD_VERT{vert(-48, -432, 0), vert(48, -432, 0), vert(-48, -336, 0), vert(48, -336, 0)}
	makeTMD_G_4T(bulletTMD, bulletTIM, square(15), nm, v, 1)
	initialiseTMD(bulletTMD, 0, &bulletOBJ)
	makeTMD_G_4T(bullet2TMD, bullet2TIM, square(15), nm, v, 1)
	initialiseTMD(bullet2TMD, 0, &bullet2OBJ)
	makeTMD_G_4T(bloodTMD, bloodTIM, square(15), nm, v, -1)
	initialiseTMD(bloodTMD, 0, &bloodOBJ)
	makeTMD_G_4T(ricochetTMD, ricochetTIM, square(15), nm, v, 1)
	initialiseTMD(ricochetTMD, 0, &ricochetOBJ)

	// A first aid box.
	nm = norm(0, 0, -yaroze.ONE)
	v = [4]demo.TMD_VERT{vert(-192, -384, 0), vert(192, -384, 0), vert(-192, 0, 0), vert(192, 0, 0)}
	makeTMD_G_4T(healthTMD, healthTIM, square(63), nm, v, -1)
	initialiseTMD(healthTMD, 0, &healthOBJ)
}

// The exit's palettes: where they are in video memory, read back every
// frame, and brightened or darkened a step to make the exit pulse.

func clutRect(timAddr uintptr) yaroze.RECT {
	var image yaroze.GsIMAGE
	yaroze.GsGetTimInfo(demo.At(timAddr+4), &image)
	return yaroze.RECT{X: image.Cx, Y: image.Cy, W: int16(image.Cw), H: int16(image.Ch)}
}

func getExitCLUTRect() {
	exitFloorCLUTRect = clutRect(efloorTIM)
	exitWallCLUTRect = clutRect(ewallTIM)
	exitRoofCLUTRect = clutRect(eroofTIM)
}

func words(clut *[16]uint16) *uint32 { return (*uint32)(unsafe.Pointer(&clut[0])) }

func grabExitCLUT() {
	yaroze.StoreImage(&exitFloorCLUTRect, words(&exitFloorCLUT))
	yaroze.StoreImage(&exitWallCLUTRect, words(&exitWallCLUT))
	yaroze.StoreImage(&exitRoofCLUTRect, words(&exitRoofCLUT))
}

func alterExitCLUT() {
	if exitCLUTVal > 20 {
		exitCLUTInc = -1
	} else if exitCLUTVal < 0 {
		exitCLUTInc = 1
	}
	exitCLUTVal += exitCLUTInc

	// Only the red component, the low five bits, changes.
	step := func(c *uint16) {
		if exitCLUTInc == 1 && *c&31 < 30 {
			*c++
		} else if exitCLUTInc == -1 && *c&31 > 1 {
			*c--
		}
	}
	for i := range 16 {
		step(&exitFloorCLUT[i])
		step(&exitWallCLUT[i])
		step(&exitRoofCLUT[i])
	}

	yaroze.LoadImage(&exitFloorCLUTRect, words(&exitFloorCLUT))
	yaroze.LoadImage(&exitWallCLUTRect, words(&exitWallCLUT))
	yaroze.LoadImage(&exitRoofCLUTRect, words(&exitRoofCLUT))
}
