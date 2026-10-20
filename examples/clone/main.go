// A first-person maze shooter, three levels deep, in fog, with a heartbeat
// that quickens as the monsters close in.
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
// Differences from the original:
//
//   - The sound bank, maps and textures are read off the disc into the
//     program's own memory, and the models it builds are built there too.
//     On a Net Yaroze the loader's batch script sent the files from the PC
//     to fixed addresses, and the models were built at others.
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
//
// # Controls
//
// The d-pad walks and turns, L1 and R1 sidestep, Cross fires, and Circle
// shows the map of where you have been. Hold Start and Select together to
// quit.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/disc"
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

// The data, read into the program's own memory, in buffers of whole
// sectors: the sound bank, the three maps and the textures.
var (
	vhData       [(8736 + 2047) / 2048 * 512]uint32
	vbData       [(186432 + 2047) / 2048 * 512]uint32
	map1Data     [(370 + 2047) / 2048 * 512]uint32
	map2Data     [(488 + 2047) / 2048 * 512]uint32
	map3Data     [(457 + 2047) / 2048 * 512]uint32
	statusData   [(3136 + 2047) / 2048 * 512]uint32
	roofData     [(2112 + 2047) / 2048 * 512]uint32
	wallData     [(2112 + 2047) / 2048 * 512]uint32
	floorData    [(2112 + 2047) / 2048 * 512]uint32
	man1Data     [(16928 + 2047) / 2048 * 512]uint32
	man2Data     [(16928 + 2047) / 2048 * 512]uint32
	bulletData   [(192 + 2047) / 2048 * 512]uint32
	bloodData    [(192 + 2047) / 2048 * 512]uint32
	deadmanData  [(16928 + 2047) / 2048 * 512]uint32
	ricochetData [(192 + 2047) / 2048 * 512]uint32
	healthData   [(2112 + 2047) / 2048 * 512]uint32
	bullet2Data  [(192 + 2047) / 2048 * 512]uint32
	eroofData    [(2112 + 2047) / 2048 * 512]uint32
	ewallData    [(2112 + 2047) / 2048 * 512]uint32
	efloorData   [(2112 + 2047) / 2048 * 512]uint32
	fwallData    [(2112 + 2047) / 2048 * 512]uint32
	faceData     [(532 + 2047) / 2048 * 512]uint32

	vh           = disc.Register("ASSETS/SFX.VH", "assets/SFX.VH").Into(vhData[:])
	vb           = disc.Register("ASSETS/SFX.VB", "assets/SFX.VB").Into(vbData[:])
	map1File     = disc.Register("ASSETS/MAP1.DAT", "assets/MAP1.DAT").Into(map1Data[:])
	map2File     = disc.Register("ASSETS/MAP2.DAT", "assets/MAP2.DAT").Into(map2Data[:])
	map3File     = disc.Register("ASSETS/MAP3.DAT", "assets/MAP3.DAT").Into(map3Data[:])
	statusFile   = disc.Register("ASSETS/STATUS.TIM", "assets/STATUS.TIM").Into(statusData[:])
	roofFile     = disc.Register("ASSETS/STONE07.TIM", "assets/STONE07.TIM").Into(roofData[:])
	wallFile     = disc.Register("ASSETS/STONE01.TIM", "assets/STONE01.TIM").Into(wallData[:])
	floorFile    = disc.Register("ASSETS/WALL03.TIM", "assets/WALL03.TIM").Into(floorData[:])
	man1File     = disc.Register("ASSETS/ALBIOD_1.TIM", "assets/ALBIOD_1.TIM").Into(man1Data[:])
	man2File     = disc.Register("ASSETS/ALBIOD_2.TIM", "assets/ALBIOD_2.TIM").Into(man2Data[:])
	bulletFile   = disc.Register("ASSETS/BULLET.TIM", "assets/BULLET.TIM").Into(bulletData[:])
	bloodFile    = disc.Register("ASSETS/BLOOD.TIM", "assets/BLOOD.TIM").Into(bloodData[:])
	deadmanFile  = disc.Register("ASSETS/ALBIOD_3.TIM", "assets/ALBIOD_3.TIM").Into(deadmanData[:])
	ricochetFile = disc.Register("ASSETS/RICOCHET.TIM", "assets/RICOCHET.TIM").Into(ricochetData[:])
	healthFile   = disc.Register("ASSETS/FIRSTAID.TIM", "assets/FIRSTAID.TIM").Into(healthData[:])
	bullet2File  = disc.Register("ASSETS/BULLET2.TIM", "assets/BULLET2.TIM").Into(bullet2Data[:])
	eroofFile    = disc.Register("ASSETS/ESTONE07.TIM", "assets/ESTONE07.TIM").Into(eroofData[:])
	ewallFile    = disc.Register("ASSETS/ESTONE01.TIM", "assets/ESTONE01.TIM").Into(ewallData[:])
	efloorFile   = disc.Register("ASSETS/EWAVE01.TIM", "assets/EWAVE01.TIM").Into(efloorData[:])
	fwallFile    = disc.Register("ASSETS/FSTONE01.TIM", "assets/FSTONE01.TIM").Into(fwallData[:])
	faceFile     = disc.Register("ASSETS/FACE.TIM", "assets/FACE.TIM").Into(faceData[:])
	files        = []*disc.File{&vh, &vb, &map1File, &map2File, &map3File, &statusFile, &roofFile, &wallFile, &floorFile, &man1File, &man2File, &bulletFile, &bloodFile, &deadmanFile, &ricochetFile, &healthFile, &bullet2File, &eroofFile, &ewallFile, &efloorFile, &fwallFile, &faceFile}

	statusTIM   = yaroze.TIM(statusData[:])
	roofTIM     = yaroze.TIM(roofData[:])
	wallTIM     = yaroze.TIM(wallData[:])
	floorTIM    = yaroze.TIM(floorData[:])
	man1TIM     = yaroze.TIM(man1Data[:])
	man2TIM     = yaroze.TIM(man2Data[:])
	bulletTIM   = yaroze.TIM(bulletData[:])
	bloodTIM    = yaroze.TIM(bloodData[:])
	deadmanTIM  = yaroze.TIM(deadmanData[:])
	ricochetTIM = yaroze.TIM(ricochetData[:])
	healthTIM   = yaroze.TIM(healthData[:])
	bullet2TIM  = yaroze.TIM(bullet2Data[:])
	eroofTIM    = yaroze.TIM(eroofData[:])
	ewallTIM    = yaroze.TIM(ewallData[:])
	efloorTIM   = yaroze.TIM(efloorData[:])
	fwallTIM    = yaroze.TIM(fwallData[:])
	faceTIM     = yaroze.TIM(faceData[:])
)

// The models the demo builds, each in 4KB of its own, as the demo spaced
// them.
const tmdWords = 4096 / 4

var (
	builtData [15 * tmdWords]uint32

	floorTMD    = yaroze.TMD(builtData[0*tmdWords : 1*tmdWords])
	roofTMD     = yaroze.TMD(builtData[1*tmdWords : 2*tmdWords])
	wallTMD     = yaroze.TMD(builtData[2*tmdWords : 3*tmdWords])
	monster1TMD = yaroze.TMD(builtData[3*tmdWords : 4*tmdWords])
	monster2TMD = yaroze.TMD(builtData[4*tmdWords : 5*tmdWords])
	bulletTMD   = yaroze.TMD(builtData[5*tmdWords : 6*tmdWords])
	bloodTMD    = yaroze.TMD(builtData[6*tmdWords : 7*tmdWords])
	deadTMD     = yaroze.TMD(builtData[7*tmdWords : 8*tmdWords])
	ricochetTMD = yaroze.TMD(builtData[8*tmdWords : 9*tmdWords])
	healthTMD   = yaroze.TMD(builtData[9*tmdWords : 10*tmdWords])
	bullet2TMD  = yaroze.TMD(builtData[10*tmdWords : 11*tmdWords])
	eroofTMD    = yaroze.TMD(builtData[11*tmdWords : 12*tmdWords])
	ewallTMD    = yaroze.TMD(builtData[12*tmdWords : 13*tmdWords])
	efloorTMD   = yaroze.TMD(builtData[13*tmdWords : 14*tmdWords])
	fwallTMD    = yaroze.TMD(builtData[14*tmdWords : 15*tmdWords])
)

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
	gpuPacket [2][packetMax / 4]uint32
)

var padStatus yaroze.Buttons

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
	yaroze.PadInit()

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
		for yaroze.PadRead()&(yaroze.PADstart|yaroze.PADselect) != yaroze.PADstart|yaroze.PADselect {
			message("missing data")
		}
		yaroze.ResetGraph(1)
		return
	}

	setupTexturesAndTMDs()
	getExitCLUTRect()

	vab = yaroze.SsVabTransfer(vh.Bytes(), vb.Bytes(), -1, 1)
	if vab < 0 {
		for yaroze.PadRead()&(yaroze.PADstart|yaroze.PADselect) != yaroze.PADstart|yaroze.PADselect {
			message("no sound bank")
		}
		yaroze.ResetGraph(1)
		return
	}
	yaroze.SsUtKeyOn(vab, 10, 0, 60, 0, 100, 100)         // background noise
	heartVoice = yaroze.SsUtKeyOn(vab, 0, 0, 60, 0, 0, 0) // heartbeat

	goodguy.level = 1
	initMap(map1Data[:])
	startLevelMusic()

	// No flat light, to clear any left from before; ambient light and fog
	// alone.
	var light yaroze.GsF_LIGHT
	yaroze.GsSetFlatLight(0, &light)
	yaroze.GsSetFlatLight(1, &light)
	yaroze.GsSetFlatLight(2, &light)
	yaroze.GsSetLightMatrix(&yaroze.IDMATRIX)

	yaroze.GsSetProjection(280)
	yaroze.GsSetLightMode(1) // with fog
	fog := yaroze.GsFOGPARAM{Dqa: -1400, Dqb: 20971520}
	yaroze.GsSetFogParam(&fog)
	yaroze.GsSetAmbient(yaroze.ONE/2, yaroze.ONE/2, yaroze.ONE/2)

	scale := yaroze.SVECTOR{Vx: yaroze.ONE, Vy: 3 * yaroze.ONE / 4, Vz: yaroze.ONE}

	for {
		// The view: from where you stand, turned the way you face.
		transform := yaroze.IDMATRIX
		yaroze.TransMatrix(&transform, &goodguy.position)
		present.Flg = 0
		present.Coord = transform
		present.Super = yaroze.WORLD

		transform = yaroze.IDMATRIX
		rotate := yaroze.SVECTOR{Vy: int16(viewAngle)}
		yaroze.RotMatrix(&rotate, &transform)

		mainView.View = transform
		mainView.Super = &present
		yaroze.GsSetView2(&mainView)

		buffIdx = yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&gpuPacket[buffIdx][0]))
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
			initMap(map1Data[:])
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

		padStatus = yaroze.PadRead()

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

		if padStatus&yaroze.PADselect != 0 && padStatus&yaroze.PADstart != 0 {
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
	yaroze.GsSetWorkBase(yaroze.Packets(&gpuPacket[buff][0]))
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
func initialiseTMD(tmd yaroze.TMD, objNum int32, obj *yaroze.GsDOBJ2) {
	tmd.Link(obj, objNum)
	obj.Coord2 = &objectCoord
}

// initialiseSprite loads a TIM and sets up a sprite to show it, as the
// demo does, quirks included: it takes the texture's u from the low five
// bits of its x.
func initialiseSprite(tim yaroze.TIM, sprite *yaroze.GsSPRITE) {
	image := tim.Load()
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
	for _, tim := range [...]yaroze.TIM{
		roofTIM, wallTIM, floorTIM, man1TIM, man2TIM, deadmanTIM,
		bulletTIM, bloodTIM, ricochetTIM, healthTIM, bullet2TIM,
		eroofTIM, ewallTIM, efloorTIM, fwallTIM,
	} {
		tim.Load()
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

func clutRect(tim yaroze.TIM) yaroze.RECT {
	image := tim.Image()
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
