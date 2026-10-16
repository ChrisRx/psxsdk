// The flying tutorial, in Go.
//
// A port of L. Evans's Net Yaroze tutorial "flying" (Sony, 1996), the
// eighteenth in the "3-D object manipulation" series: a ship flying over a
// plane of tiles under a field of stars, with enemy ships that turn to face
// it and shoot. One enemy at a time appears somewhere on the plane, either
// sitting and tracking or giving chase, and goes down after ten hits in a
// burst of spinning sprites, for a score. A scanner in the corner shows
// where it is, pulsing. The view follows the ship from any of nine places,
// from close behind to high above to either side. The library's font prints
// the ship's position and the score, and the star field slides as the ship
// turns.
//
// Up and down move the ship forwards and back, Square and Circle turn it,
// L2 fires, R1 and R2 step through the views and L1 goes back to the first.
// Holding Select moves the viewpoint instead, by the directional buttons,
// Triangle and Cross, and R1 and R2 change the projection; Select and L2
// step slowly. Start pauses, and Start and Select together quit. When the
// ship runs out of health, the score is shown until Start is pressed, and
// the program ends.
//
// Differences from the original:
//
//   - The data is read off the disc to the addresses the original's batch
//     script sent it to. The cube and sphere models the script also sends,
//     which the program never uses, are left out.
//   - After the score is shown, the program returns to the BIOS. The
//     original goes on to a reset that starts with a failing assertion,
//     which prints a message to the serial console and exits.
//   - The bouncing spheres, which the original has switched off, and the
//     level changes, which nothing in it reaches, are left out, and so are
//     its assertions and what it prints to the serial console.
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
	otLength         = 9

	squareModelAddr = 0x80094000
	shipModelAddr   = 0x80096000
	mainTIMAddr     = 0x800a0000
	starsTIMAddr    = 0x800b0000
	circleTIMAddr   = 0x800b9000
	dotsTIMAddr     = 0x800ba000
	fireTIMAddr     = 0x800bb000
	multipleTIMAddr = 0x800d0000

	// The plane: tiles gridSize across, worldX by worldZ of them, and how
	// far the ship may go.
	gridSize  = 128
	worldX    = 20
	worldY    = 10
	worldZ    = 20
	worldMaxX = (worldX - 1) * gridSize
	worldMaxZ = (worldZ - 1) * gridSize

	// The sprites that are drawn behind everything.
	furthestBack = 1<<otLength - 1
)

var files = []demo.File{
	{File: disc.Register("DATA/TEN.TMD", "data/TEN.TMD"), Addr: squareModelAddr},
	{File: disc.Register("DATA/THESHIP.TMD", "data/THESHIP.TMD"), Addr: shipModelAddr},
	{File: disc.Register("DATA/WAVE09.TIM", "data/WAVE09.TIM"), Addr: mainTIMAddr},
	{File: disc.Register("DATA/SPBACK.TIM", "data/SPBACK.TIM"), Addr: starsTIMAddr},
	{File: disc.Register("DATA/RADAR4.TIM", "data/RADAR4.TIM"), Addr: circleTIMAddr},
	{File: disc.Register("DATA/DOTS.TIM", "data/DOTS.TIM"), Addr: dotsTIMAddr},
	{File: disc.Register("DATA/FIRE3.TIM", "data/FIRE3.TIM"), Addr: fireTIMAddr},
	{File: disc.Register("DATA/MULTI2.TIM", "data/MULTI2.TIM"), Addr: multipleTIMAddr},
}

var (
	wot        [2]yaroze.GsOT
	wtags      [2][1 << otLength]yaroze.GsOT_TAG
	packetArea [2][packetMax]yaroze.PACKET

	view       yaroze.GsRVIEW2
	projection int32

	// Where the viewpoint is, in the world, for clipping.
	viewPointX, viewPointZ int32

	// How far from the ship things are drawn, and subdivided, in tiles.
	simpleClipDistance, veryCloseDistance int32

	viewMode                  int32
	framesSinceLastViewSwitch int32
	viewChangePauseTime       int32
	viewpointXDistance        int32
	viewpointYDistance        int32
	viewpointZDistance        int32

	lights    [3]yaroze.GsF_LIGHT
	lightMode int32

	mainTexture, starsTexture, circleTexture, dotsTexture, fireTexture, multipleTexture yaroze.GsIMAGE

	// The star field, two sprites wide, and the scanner's oval and blips.
	background1, background2 yaroze.GsSPRITE
	ovalSprite               yaroze.GsSPRITE
	blipSprites              [maxShips]yaroze.GsSPRITE

	frameNumber     int32
	quitFrameNumber int32 = -1

	levelNumber       int32 = 1
	numberEnemiesLeft int32 = maxShips

	// displayed is what the world clipping leaves to draw.
	displayed [lpge.MaxObjects]*lpge.Handler

	textBuf [160]byte
	text    yaroze.Buffer
)

// The views: how far behind, above and beside the ship the viewpoint is,
// and the projection.
const (
	view1 = 1 + iota
	view2
	view3
	view4
	view5
	view6
	view7
	view8
	view9
	maxViewMode = view9
)

func main() {
	text.Init(textBuf[:])
	if !initialiseAll() {
		return
	}

	side := yaroze.GsGetActiveBuff()
	for quitFrameNumber != frameNumber {
		text.Reset()
		text.WriteString("frame: ").WriteInt(int(frameNumber)).
			WriteString("\nplayer health: ").WriteInt(int(playersShip.CurrentHealth)).
			WriteString("\nplayer score: ").WriteInt(int(playersShip.MeritRating)).
			WriteString("\nlevel: ").WriteInt(int(levelNumber)).
			WriteString("\nenemies left: ").WriteInt(int(numberEnemiesLeft)).WriteString("\n").Print()
		if frameNumber%20 == 0 {
			numberEnemiesLeft = countNumberOfEnemiesLeft()
		}
		p := &playersShip.Position
		text.Reset()
		text.WriteString("ship pos: \n").WriteInt(int(p.Vx)).WriteString(" ").WriteInt(int(p.Vy)).WriteString(" ").WriteInt(int(p.Vz)).
			WriteString("\nship grid: ").WriteInt(int(p.Vx / gridSize)).WriteString(" ").WriteInt(int(p.Vy / gridSize)).WriteString(" ").WriteInt(int(p.Vz / gridSize)).
			WriteString("\n").Print()
		e := &theShips[0].Position
		text.Reset()
		text.WriteString("enemy 0 at\n").WriteInt(int(e.Vx)).WriteString(" ").WriteInt(int(e.Vy)).WriteString(" ").WriteInt(int(e.Vz)).WriteString("\n").Print()

		frameNumber++
		dealWithControllerPad()

		yaroze.GsSetWorkBase(&packetArea[side][0])
		yaroze.GsClearOt(0, 0, &wot[side])

		updateBackground()
		handleAllObjects()
		if quitFrameNumber == frameNumber {
			break
		}

		yaroze.GsSetRefView2(&view)

		n := performWorldClipping()
		text.Reset()
		text.WriteString("num to display: ").WriteInt(n).WriteString("\n").Print()

		yaroze.GsSortSprite(&background1, &wot[side], furthestBack)
		yaroze.GsSortSprite(&background2, &wot[side], furthestBack)
		yaroze.GsSortFastSprite(&ovalSprite, &wot[side], 3)
		sortBlipSprites(&wot[side])

		for _, o := range displayed[:n] {
			if o.DisplayFlag == lpge.TMD {
				var ls yaroze.MATRIX
				yaroze.GsGetLs(&o.Coord, &ls)
				yaroze.GsSetLightMatrix(&ls)
				yaroze.GsSetLsMatrix(&ls)
				yaroze.GsSortObject4(&o.Handler, &wot[side], 3, demo.Scratch())
			} else {
				z := sortSpriteObjectPosition(o)
				yaroze.GsSortSprite(&o.Sprite, &wot[side], z)
			}
		}

		yaroze.VSync(0)
		yaroze.ResetGraph(1)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 4, &wot[side])
		yaroze.GsDrawOt(&wot[side])
		side ^= 1
		yaroze.FntFlush(-1)
	}
	yaroze.ResetGraph(3)
}

// sortSpriteObjectPosition places an object drawn as a sprite where its
// position projects to on screen, scaled for its distance, and returns its
// depth in the ordering table.
func sortSpriteObjectPosition(o *lpge.Handler) uint16 {
	ws := yaroze.GsWSMATRIX()
	yaroze.PushMatrix()
	var screen yaroze.VECTOR
	yaroze.ApplyMatrixLV(ws, &o.Position, &screen)
	screen.Vx += ws.T[0]
	screen.Vy += ws.T[1]
	screen.Vz += ws.T[2]
	if screen.Vz == 0 {
		// The original returns here without popping the matrix.
		return 0
	}
	visualX := int32(o.Sprite.W) * projection / screen.Vz
	visualY := int32(o.Sprite.H) * projection / screen.Vz
	o.Sprite.X = int16(screen.Vx*projection/screen.Vz - visualX/2)
	o.Sprite.Y = int16(screen.Vy*projection/screen.Vz - visualY/2)
	o.Sprite.Scalex = int16(o.ScaleX * projection / screen.Vz)
	o.Sprite.Scaley = int16(o.ScaleY * projection / screen.Vz)
	yaroze.PopMatrix()

	if screen.Vz < projection/2 {
		o.Sprite.Attribute |= yaroze.GsDOFF
	} else {
		// The original clears the bit on the model handler instead.
		o.Handler.Attribute &^= yaroze.GsDOFF
	}
	return uint16(screen.Vz >> 6)
}

// performWorldClipping puts the living objects within a square of the
// viewpoint, which sits behind the ship, into displayed, and returns how
// many there are.
func performWorldClipping() int {
	viewXOffset := viewpointZDistance * lpge.Rsin(int32(playersShip.Rotate.Vy)) >> 12
	viewZOffset := viewpointZDistance * lpge.Rcos(int32(playersShip.Rotate.Vy)) >> 12
	viewPointX = playersShip.Position.Vx + viewXOffset
	viewPointZ = playersShip.Position.Vz + viewZOffset

	simpleClipDistance = findClipDistanceFromViewMode()
	n := 0
	for _, o := range &lpge.Objects {
		if o == nil || !o.Alive {
			continue
		}
		if abs(o.Position.Vx-viewPointX) < simpleClipDistance*gridSize && abs(o.Position.Vz-viewPointZ) < simpleClipDistance*gridSize {
			displayed[n] = o
			n++
		}
	}
	return n
}

func findClipDistanceFromViewMode() int32 {
	switch viewMode {
	case view1:
		return 5
	case view6:
		return 10
	case view7:
		return 15
	case view8, view9:
		return 3
	}
	return 4
}

func findSubdivisionDistanceFromViewMode() int32 {
	switch viewMode {
	case view7:
		return 5
	case view8, view9:
		return 3
	}
	return 2
}

// updateBackground slides the two star field sprites as the ship turns, the
// one that has gone off one side going round to the other.
func updateBackground() {
	angle := int32(playersShip.Rotate.Vy)
	twist := playersShip.Twist.Vy
	if twist == 0 {
		return
	}
	switch angle {
	case 0, 1024, 2048, 3072: // dead on
		if twist < 0 { // left, anticlockwise
			if background1.X == -screenW/2 {
				background2.X = background1.X - screenW
			} else if background2.X == -screenW/2 {
				background1.X = background2.X - screenW
			}
		} else { // right, clockwise
			if background1.X == -screenW/2 {
				background2.X = background1.X + screenW
			} else if background2.X == -screenW/2 {
				background1.X = background2.X + screenW
			}
		}
	}
	background1.X -= int16(twist * screenW / 1024)
	background2.X -= int16(twist * screenW / 1024)
}

// The scanner: an oval in the top left corner, with a blip for each enemy
// within scannerRange, where it is relative to the ship.
const (
	ovalHalfWidth  = 22
	ovalHalfHeight = 17
	ovalCentreX    = 25
	ovalCentreY    = 25
	scannerRange   = 5 * gridSize
)

func sortBlipSprites(ot *yaroze.GsOT) {
	// The blips pulse red, over thirty frames.
	cyclePoint := frameNumber % 30

	// The world's axes in the ship's coordinate system.
	worldZaxis := yaroze.VECTOR{Vx: lpge.Rsin(int32(playersShip.Rotate.Vy)), Vz: -lpge.Rcos(int32(playersShip.Rotate.Vy))}
	worldXaxis := yaroze.VECTOR{Vx: worldZaxis.Vz, Vz: -worldZaxis.Vx}

	for i := range theShips {
		s := &theShips[i]
		if !s.Alive {
			continue
		}
		wx := s.Position.Vx - playersShip.Position.Vx
		wz := s.Position.Vz - playersShip.Position.Vz
		ox := (wx*worldXaxis.Vx + wz*worldZaxis.Vx) / yaroze.ONE
		oz := (wx*worldXaxis.Vz + wz*worldZaxis.Vz) / yaroze.ONE
		if ox*ox+oz*oz < scannerRange*scannerRange {
			xInCircle := ox * ovalHalfWidth / scannerRange
			zInCircle := oz * ovalHalfHeight / scannerRange
			b := &blipSprites[i]
			b.X = int16(-screenW/2 + ovalCentreX + 8 + xInCircle - 2)
			b.Y = int16(-screenH/2 + ovalCentreY + 19 - zInCircle - 2)
			b.R = uint8(16 + 5*cyclePoint)
			yaroze.GsSortFastSprite(b, ot, 1)
		}
	}
}

func countNumberOfEnemiesLeft() int32 {
	var n int32
	for i := range theShips {
		if theShips[i].Alive {
			n++
		}
	}
	return n
}

func dealWithControllerPad() {
	pad := demo.PadRead()

	if pad&demo.PADstart != 0 && pad&demo.PADselect != 0 {
		quitFrameNumber = frameNumber + 1
	}

	// Select and L2: step slowly.
	if pad&demo.PADselect != 0 && pad&demo.PADL2 != 0 {
		count := yaroze.VSync(-1)
		for count+25 > yaroze.VSync(-1) {
		}
		return
	}

	// Select: move the viewpoint about instead.
	if pad&demo.PADselect != 0 {
		if pad&demo.PADLleft != 0 {
			view.Vpz -= 5
		}
		if pad&demo.PADLright != 0 {
			view.Vpz += 5
		}
		if pad&demo.PADLup != 0 {
			view.Vpy -= 5
		}
		if pad&demo.PADLdown != 0 {
			view.Vpy += 5
		}
		if pad&demo.PADRup != 0 {
			view.Vpx -= 5
		}
		if pad&demo.PADRdown != 0 {
			view.Vpx += 5
		}
		if pad&demo.PADR1 != 0 {
			projection -= 5
			yaroze.GsSetProjection(projection)
		}
		if pad&demo.PADR2 != 0 {
			projection += 5
			yaroze.GsSetProjection(projection)
		}
		return
	}

	s := &playersShip
	if pad&demo.PADLup != 0 {
		s.Velocity.Vz -= s.MovementSpeed
	}
	if pad&demo.PADLdown != 0 {
		s.Velocity.Vz += s.MovementSpeed
	}

	if pad&demo.PADL2 != 0 && s.FramesSinceLastFire > s.FiringRate {
		playerFiresAShot()
		s.FramesSinceLastFire = 0
	}
	s.FramesSinceLastFire++

	if pad&demo.PADRleft != 0 {
		s.Twist.Vy -= s.RotationSpeed
	}
	if pad&demo.PADRright != 0 {
		s.Twist.Vy += s.RotationSpeed
	}
	if s.Rotate.Vy < 0 {
		s.Rotate.Vy += 4096
	} else if s.Rotate.Vy > 4095 {
		s.Rotate.Vy -= 4096
	}

	// R1 and R2 step through the views, L1 goes back to the first.
	if pad&demo.PADR1 != 0 && framesSinceLastViewSwitch > viewChangePauseTime {
		if viewMode == maxViewMode {
			viewMode = view1
		} else {
			viewMode++
		}
		framesSinceLastViewSwitch = 0
		updateTheView()
	}
	if pad&demo.PADR2 != 0 && framesSinceLastViewSwitch > viewChangePauseTime {
		if viewMode == view1 {
			viewMode = maxViewMode
		} else {
			viewMode--
		}
		framesSinceLastViewSwitch = 0
		updateTheView()
	}
	framesSinceLastViewSwitch++
	if pad&demo.PADL1 != 0 {
		viewMode = view1
		updateTheView()
		framesSinceLastViewSwitch = 0
	}

	for pad&demo.PADstart != 0 { // pause
		pad = demo.PadRead()
	}
}

func initialiseAll() bool {
	demo.PadInit()

	yaroze.GsInitGraph(screenW, screenH, yaroze.GsINTER|yaroze.GsOFSGPU, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, screenH)
	yaroze.GsInit3D()
	for i := range wot {
		wot[i].Length = otLength
		wot[i].Org = &wtags[i][0]
		yaroze.GsClearOt(0, 0, &wot[i])
	}
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(0, 0, 256, 200, 0, 512)

	if missing := demo.Load(files, nil); missing != "" {
		basicText[0].Reset()
		basicText[0].WriteString("could not read\n").WriteString(missing)
		dummyGuiLoopForTextPrinting(1, demo.PADstart|demo.PADselect)
		yaroze.ResetGraph(3)
		return false
	}

	initialiseView()
	initialiseLighting()

	lpge.ProperInitialiseTexture(mainTIMAddr, &mainTexture)
	lpge.ProperInitialiseTexture(starsTIMAddr, &starsTexture)
	lpge.ProperInitialiseTexture(circleTIMAddr, &circleTexture)
	lpge.ProperInitialiseTexture(dotsTIMAddr, &dotsTexture)
	lpge.ProperInitialiseTexture(fireTIMAddr, &fireTexture)
	lpge.ProperInitialiseTexture(multipleTIMAddr, &multipleTexture)

	initialiseBackgroundSprite(&background1)
	initialiseBackgroundSprite(&background2)
	initialiseOvalSprite()
	initialiseBlipSprites()

	initialiseObjects()
	return true
}

// initialiseBackgroundSprite sets a star field sprite up over the whole
// screen, from the 4-bit stars texture.
func initialiseBackgroundSprite(s *yaroze.GsSPRITE) {
	lpge.InitGsSprite(s)
	s.Tpage = yaroze.GetTPage(0, 0, 640, 256)
	s.X, s.Y = -screenW/2, -screenH/2
	s.W, s.H = 256, 256
	s.Scalex = yaroze.ONE * screenW / 256
	s.Cx, s.Cy = 0, 481
}

func initialiseOvalSprite() {
	s := &ovalSprite
	lpge.InitGsSprite(s)
	s.Tpage = yaroze.GetTPage(1, 0, 960, 0) // 8-bit
	s.Attribute |= 1 << 24
	s.X, s.Y = -screenW/2+8, -screenH/2+19
	s.W, s.H = 48, 48
	s.Cx, s.Cy = 0, 482
}

// initialiseBlipSprites sets the blips up: a red dot each from the dots
// texture.
func initialiseBlipSprites() {
	for i := range blipSprites {
		s := &blipSprites[i]
		lpge.InitGsSprite(s)
		s.Tpage = yaroze.GetTPage(0, 0, 960, 0)
		s.W, s.H = 2, 2
		s.U, s.V = 11, 52
		s.Cx, s.Cy = 0, 483
	}
}

func initialiseView() {
	projection = 192
	yaroze.GsSetProjection(projection)
	viewpointZDistance = 220
	viewpointYDistance = -40
	viewpointXDistance = 0

	veryCloseDistance = 2
	simpleClipDistance = 5

	viewMode = view1
	framesSinceLastViewSwitch = 0
	viewChangePauseTime = 15

	view = yaroze.GsRVIEW2{
		Vpx: viewpointXDistance, Vpy: viewpointYDistance, Vpz: viewpointZDistance,
		Super: &playersShip.Coord,
	}
	yaroze.GsSetRefView2(&view)
}

// updateTheView moves the viewpoint to where the view mode has it,
// relative to the ship.
func updateTheView() {
	switch viewMode {
	case view1:
		viewpointZDistance, viewpointYDistance, viewpointXDistance, projection = 220, -40, 0, 192
	case view2:
		viewpointZDistance, viewpointYDistance, viewpointXDistance, projection = 220, 0, 0, 192
	case view3:
		viewpointZDistance, viewpointYDistance, viewpointXDistance, projection = 220, 20, 0, 192
	case view4:
		viewpointZDistance, viewpointYDistance, viewpointXDistance, projection = 155, 0, 0, 192
	case view5:
		viewpointZDistance, viewpointYDistance, viewpointXDistance, projection = 155, -340, 0, 192
	case view6:
		viewpointZDistance, viewpointYDistance, viewpointXDistance, projection = 155, -1600, 0, 192
	case view7:
		viewpointZDistance, viewpointYDistance, viewpointXDistance, projection = 80, -25, 0, 32
	case view8:
		viewpointZDistance, viewpointYDistance, viewpointXDistance, projection = 0, 0, 200, 192
	case view9:
		viewpointZDistance, viewpointYDistance, viewpointXDistance, projection = 0, 0, -200, 192
	}
	view.Vpx, view.Vpy, view.Vpz = viewpointXDistance, viewpointYDistance, viewpointZDistance
	yaroze.GsSetProjection(projection)
}

func initialiseLighting() {
	lights[0] = yaroze.GsF_LIGHT{Vx: 1, Vy: 1, Vz: 1, R: 0x80, G: 0x80, B: 0x80}
	yaroze.GsSetFlatLight(0, &lights[0])
	lights[1] = yaroze.GsF_LIGHT{Vx: -1, Vy: -1, Vz: 1, R: 0x80, G: 0x80, B: 0x80}
	yaroze.GsSetFlatLight(1, &lights[1])
	yaroze.GsSetAmbient(yaroze.ONE/4, yaroze.ONE/4, yaroze.ONE/4)
	lightMode = 0 // no fog
	yaroze.GsSetLightMode(lightMode)
}

// rand is the C library's rand, as an int32.
func rand() int32 { return yaroze.Rand() }

func keepWithinRange(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func abs(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
