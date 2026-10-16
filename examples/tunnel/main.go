// The tunnel tutorial, in Go.
//
// A port of L. Evans's Net Yaroze game demo "tunnel" (Sony, 1997), the last
// of the tutorial series it was built up over: a ship flying down an
// endless tunnel, shooting waves of enemies. The tunnel is rings of four
// scaled squares that slide towards the ship and go round to the far end
// again; the near ones are subdivided so the perspective holds up. Fog,
// three coloured lights, one of them always circling, and semi-transparent
// sprites for shots, explosions and pick-ups make it the most of the
// machine's lighting any of the examples use. Its sound is a VAB: a tune
// started once and left playing, and sound effects keyed on and off by
// hand. The score is drawn in sprites cut from a font texture.
//
// Enemies come in waves: helixes of fighters circling as they come, some
// switching direction or darting as one, lone attackers that jiggle, evade
// or circle while they fire, swarms of insect ships, and every so often a
// robot. Pick-ups give a life, faster shots or movement, or set the lights
// or the view swinging. The game gets faster every thousand frames.
//
// The directional buttons and L1 and L2 move the ship, R1 fires, Select
// makes moves faster, Select and L2 step slowly, Start pauses, and Start and
// Select together quit.
//
// Differences from the original:
//
//   - The data is read off the disc to the addresses the original's batch
//     script sent it to.
//   - The random numbers are seeded with a constant. The original seeds
//     them from the seconds of the time it was compiled.
//   - Quitting returns to the BIOS. The original first saves the screen to
//     RAM for the serial console to fetch, and calls exit.
//   - The direction a shot is fired in is normalised in integers, exactly;
//     the original uses floating point.
//   - What the original prints to the serial console is left out, and so
//     are its assertions.
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

	squareModelAddr   = 0x80090000
	shipModelAddr     = 0x80091000
	mainTIMAddr       = 0x80092000
	fireTIMAddr       = 0x80093000
	multipleModelAddr = 0x80094000
	multipleTIMAddr   = 0x80099000
	vhAddr            = 0x800a0000
	vbAddr            = 0x800b0000
	mascotsTIMAddr    = 0x80120000
	asciiTIMAddr      = 0x80130000

	// The square model's own size.
	squareSize = 128
)

var files = []demo.File{
	{File: disc.Register("DATA/TEN.TMD", "data/TEN.TMD"), Addr: squareModelAddr},
	{File: disc.Register("DATA/THESHIP.TMD", "data/THESHIP.TMD"), Addr: shipModelAddr},
	{File: disc.Register("DATA/WAVE09.TIM", "data/WAVE09.TIM"), Addr: mainTIMAddr},
	{File: disc.Register("DATA/FIRE7.TIM", "data/FIRE7.TIM"), Addr: fireTIMAddr},
	{File: disc.Register("DATA/ENEMY.TMD", "data/ENEMY.TMD"), Addr: multipleModelAddr},
	{File: disc.Register("DATA/MULTI3.TIM", "data/MULTI3.TIM"), Addr: multipleTIMAddr},
	{File: disc.Register("DATA/MASCOTS2.TIM", "data/MASCOTS2.TIM"), Addr: mascotsTIMAddr},
	{File: disc.Register("DATA/ASCII2.TIM", "data/ASCII2.TIM"), Addr: asciiTIMAddr},
	{File: disc.Register("DATA/LEWIS.VH", "data/LEWIS.VH"), Addr: vhAddr},
	{File: disc.Register("DATA/LEWIS.VB", "data/LEWIS.VB"), Addr: vbAddr},
}

var (
	wot        [2]yaroze.GsOT
	wtags      [2][1 << otLength]yaroze.GsOT_TAG
	packetArea [2][packetMax]yaroze.PACKET

	view       yaroze.GsRVIEW2
	projection int32

	// Where the view sits, relative to the world's origin.
	viewpointX, viewpointY, viewpointZ int32

	lights    [3]yaroze.GsF_LIGHT
	fogging   yaroze.GsFOGPARAM
	lightMode int32

	mainTexture, fireTexture, multipleTexture, mascotsTexture, asciiTexture yaroze.GsIMAGE

	frameNumber int32

	// displayed is what the world clipping leaves to draw.
	displayed [lpge.MaxObjects]*lpge.Handler

	quit bool
)

func main() {
	if !initialiseAll() {
		return
	}
	flyingSequence()

	side := yaroze.GsGetActiveBuff()
	for !quit {
		handleDelayedActionList()
		if quit {
			break
		}
		checkForEndOfLevel()

		registerNumber("frame ", frameNumber, 50, 50)
		registerNumber("health ", playersShip.CurrentHealth, 50, 60)
		registerNumber("lives ", playersLives, 50, 70)
		registerNumber("score ", playersShip.MeritRating, 50, 80)
		registerNumber("level ", levelNumber, 50, 90)

		frameNumber++
		dealWithControllerPad()

		if lightOscillation {
			oscillateLighting()
		}
		if viewOscillation {
			oscillateView()
		}
		minorOscillateLighting()
		handleSound()

		yaroze.GsSetWorkBase(&packetArea[side][0])
		yaroze.GsClearOt(0, 0, &wot[side])

		handleAllObjects()
		updateTheTunnel()
		yaroze.GsSetRefView2(&view)

		drawObjects(performWorldClipping(), &wot[side])
		displayTextStrings(&wot[side])

		yaroze.VSync(0)
		yaroze.ResetGraph(1)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 4, &wot[side])
		yaroze.GsDrawOt(&wot[side])
		side ^= 1
		yaroze.FntFlush(-1)
	}
	yaroze.ResetGraph(3)
	cleanUpSound()
}

// drawObjects sorts the first n displayed objects into an ordering table.
func drawObjects(n int, ot *yaroze.GsOT) {
	for _, o := range displayed[:n] {
		if o.DisplayFlag == lpge.TMD {
			var ls yaroze.MATRIX
			yaroze.GsGetLs(&o.Coord, &ls)
			yaroze.GsSetLightMatrix(&ls)
			yaroze.GsSetLsMatrix(&ls)
			yaroze.GsSortObject4(&o.Handler, ot, 3, demo.Scratch())
		} else {
			z := sortSpriteObjectPosition(o)
			yaroze.GsSortSprite(&o.Sprite, ot, z)
		}
	}
}

// levelTime is how many frames a level lasts: the original's extremely
// crude notion of a level.
const levelTime = 1000

func checkForEndOfLevel() {
	if actual := frameNumber/levelTime + 1; levelNumber < actual {
		levelNumber++
		handleLevelUpdating()
	}
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
		o.Sprite.Attribute &^= yaroze.GsDOFF
	}
	return uint16(screen.Vz >> 6)
}

// performWorldClipping puts the living objects not too far down the tunnel
// into displayed, and returns how many there are.
func performWorldClipping() int {
	const clipDistance = squareSize * 8
	n := 0
	for _, o := range &lpge.Objects {
		if o != nil && o.Alive && o.Coord.Coord.T[2] < clipDistance {
			displayed[n] = o
			n++
		}
	}
	return n
}

// performBogusWorldClipping puts every living object into displayed.
func performBogusWorldClipping() int {
	n := 0
	for _, o := range &lpge.Objects {
		if o != nil && o.Alive {
			displayed[n] = o
			n++
		}
	}
	return n
}

func dealWithControllerPad() {
	pad := demo.PadRead()
	if pad&demo.PADstart != 0 && pad&demo.PADselect != 0 {
		registerDelayedAction(quitProgram, 1)
	}
	speed := int32(1)
	if pad&demo.PADselect != 0 {
		speed = 10
	}
	// Select and L2: step slowly.
	if pad&demo.PADselect != 0 && pad&demo.PADL2 != 0 {
		count := yaroze.VSync(-1)
		for count+25 > yaroze.VSync(-1) {
		}
		return
	}

	if playerInteractive {
		s := &playersShip
		by := s.MovementSpeed * speed
		if pad&demo.PADL1 != 0 {
			s.Position.Vz -= by
		}
		if pad&demo.PADL2 != 0 {
			s.Position.Vz += by
		}
		if pad&demo.PADLup != 0 {
			s.Position.Vy -= by
		}
		if pad&demo.PADLdown != 0 {
			s.Position.Vy += by
		}
		if pad&demo.PADLleft != 0 {
			s.Position.Vx -= by
		}
		if pad&demo.PADLright != 0 {
			s.Position.Vx += by
		}
		s.Position.Vx = keepWithinRange(s.Position.Vx, playerMinX, playerMaxX)
		s.Position.Vy = keepWithinRange(s.Position.Vy, playerMinY, playerMaxY)
		s.Position.Vz = keepWithinRange(s.Position.Vz, playerMinZ, sectionLength)
		if pad&demo.PADR1 != 0 {
			m := &s.Coord.Coord.M
			// Negated in z because of the way the ship model points.
			dir := yaroze.VECTOR{Vx: int32(m[2][0]), Vy: int32(m[2][1]), Vz: -int32(m[2][2])}
			handleObjectContinuousFiring(s, &dir)
		}
	}

	for pad&demo.PADstart != 0 { // pause
		pad = demo.PadRead()
	}
}

func initialiseAll() bool {
	demo.PadInit()

	// The squares' scaling, for smoother walls; set and never used.
	scalingVector = yaroze.VECTOR{Vx: 7659, Vy: 7059, Vz: 7779}

	precompileCircularPositions()
	initialiseDelayedActionList()

	yaroze.GsInitGraph(screenW, screenH, yaroze.GsINTER|yaroze.GsOFSGPU, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, screenH)
	yaroze.GsInit3D()
	for i := range wot {
		wot[i].Length = otLength
		wot[i].Org = &wtags[i][0]
		yaroze.GsClearOt(0, 0, &wot[i])
	}
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(0, 40, 256, 200, 0, 512)

	if missing := demo.Load(files, nil); missing != "" {
		basicText[0].Reset()
		basicText[0].WriteString("could not read\n").WriteString(missing)
		dummyGuiLoopForTextPrinting(1, demo.PADstart|demo.PADselect)
		yaroze.ResetGraph(3)
		return false
	}

	initialiseSound()
	yaroze.Srand(randomSeed)

	initialiseView()
	initialiseLighting()

	lpge.ProperInitialiseTexture(mainTIMAddr, &mainTexture)
	lpge.ProperInitialiseTexture(fireTIMAddr, &fireTexture)
	lpge.ProperInitialiseTexture(multipleTIMAddr, &multipleTexture)
	lpge.ProperInitialiseTexture(mascotsTIMAddr, &mascotsTexture)
	lpge.ProperInitialiseTexture(asciiTIMAddr, &asciiTexture)

	initialiseTextStrings()
	initialiseObjects()
	initialiseTunnel()
	return true
}

// randomSeed stands in for the original's seed, the sum of the two digits
// of the seconds it was compiled at: as if at 12:00:00.
const randomSeed = '0' + '0'

// rand is the C library's rand, as an int32.
func rand() int32 { return yaroze.Rand() }

func initialiseView() {
	projection = 192
	yaroze.GsSetProjection(projection)
	viewpointX, viewpointY, viewpointZ = 0, 0, -220
	view = yaroze.GsRVIEW2{Vpx: viewpointX, Vpy: viewpointY, Vpz: viewpointZ, Super: nil}
	yaroze.GsSetRefView2(&view)
}

func initialiseLighting() {
	sortTheFlatLights()
	yaroze.GsSetAmbient(yaroze.ONE/4, yaroze.ONE/4, yaroze.ONE/4)
	fogging = yaroze.GsFOGPARAM{Dqa: -960, Dqb: 5120 * 5120, Bfc: 4}
	yaroze.GsSetFogParam(&fogging)
	lightMode = 1 // with fog
	yaroze.GsSetLightMode(lightMode)
}

// sortTheFlatLights sets the two purple lights shining across the tunnel.
func sortTheFlatLights() {
	lights[0] = yaroze.GsF_LIGHT{Vx: 1, Vy: 1, Vz: 1, R: 128, G: 42, B: 128}
	yaroze.GsSetFlatLight(0, &lights[0])
	lights[1] = yaroze.GsF_LIGHT{Vx: -1, Vy: -1, Vz: 1, R: 128, G: 42, B: 128}
	yaroze.GsSetFlatLight(1, &lights[1])
}

const lightingCycleTime = 120

// oscillateLighting turns the two lights, blue now, round the tunnel's
// axis, a turn every two seconds.
func oscillateLighting() {
	angle := frameNumber % lightingCycleTime * (4096 / lightingCycleTime)
	lights[0].R, lights[0].G, lights[0].B = 0, 0, 64
	lights[1].R, lights[1].G, lights[1].B = 0, 0, 64
	lights[0].Vx, lights[0].Vy, lights[0].Vz = lpge.Rcos(angle), lpge.Rsin(angle), 0
	lights[1].Vx, lights[1].Vy, lights[1].Vz = -lights[0].Vx, -lights[0].Vy, 0
	for i := range 2 {
		yaroze.GsSetFlatLight(int32(i), &lights[i])
	}
}

const minorOscillationPeriod = 60

// minorOscillateLighting turns the third light round the tunnel's axis, a
// turn a second, all the time.
func minorOscillateLighting() {
	angle := frameNumber % minorOscillationPeriod * (4096 / minorOscillationPeriod)
	lights[2] = yaroze.GsF_LIGHT{Vx: lpge.Rcos(angle), Vy: lpge.Rsin(angle), R: 128, G: 12, B: 128}
	yaroze.GsSetFlatLight(2, &lights[2])
}

// View oscillations, a pick-up's effect.
const (
	leftToRight = iota
	upAndDown
	circlingClockwise
	circlingAnticlockwise
)

var (
	lightOscillation, viewOscillation bool

	viewDistortionMode    int32
	oldViewDistortionMode int32 = leftToRight

	viewHorizontalVelocity, viewVerticalVelocity int32
	viewBaseVelocity                             int32 = 2
	viewAngle                                    int32
	viewAngularSpeed                             int32 = yaroze.ONE / 128
	viewOscillationJustBegun                     bool
)

func oscillateView() {
	if viewOscillationJustBegun {
		switch viewDistortionMode {
		case leftToRight:
			viewHorizontalVelocity = viewBaseVelocity - 2*viewBaseVelocity*(rand()%2)
		case upAndDown:
			viewVerticalVelocity = viewBaseVelocity - 2*viewBaseVelocity*(rand()%2)
		case circlingClockwise, circlingAnticlockwise:
			viewAngle = rand() % yaroze.ONE
		}
		viewOscillationJustBegun = false
	}
	switch viewDistortionMode {
	case leftToRight:
		if v := view.Vpx + viewHorizontalVelocity; v < playerMinX || v > playerMaxX {
			viewHorizontalVelocity = -viewHorizontalVelocity
		}
		view.Vpx += viewHorizontalVelocity
	case upAndDown:
		if v := view.Vpy + viewVerticalVelocity; v < playerMinY || v > playerMaxY {
			viewVerticalVelocity = -viewVerticalVelocity
		}
		view.Vpy += viewVerticalVelocity
	case circlingClockwise:
		viewAngle += viewAngularSpeed
		view.Vpx = radius * lpge.Rcos(viewAngle) >> 12
		view.Vpy = radius * lpge.Rsin(viewAngle) >> 12
	case circlingAnticlockwise:
		viewAngle -= viewAngularSpeed
		view.Vpx = radius * lpge.Rcos(viewAngle) >> 12
		view.Vpy = radius * lpge.Rsin(viewAngle) >> 12
	}
}

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

// The score and the like are drawn in sprites cut from a font texture,
// a letter or a digit each, eight pixels square.

const (
	maxTextStrings  = 20
	maxStringLength = 50
)

type textString struct {
	x, y   int16
	length int
	data   [maxStringLength]byte
}

var (
	letterSprites [26]yaroze.GsSPRITE
	numberSprites [10]yaroze.GsSPRITE
	allStrings    [maxTextStrings]textString

	numberBuf [maxStringLength]byte
	number    yaroze.Buffer
)

func initialiseTextStrings() {
	for i := range letterSprites {
		s := &letterSprites[i]
		lpge.LinkSpriteToImageInfo(s, &asciiTexture)
		s.W, s.H = 8, 8
		if i < 15 {
			s.U += uint8(8 + i*8)
			s.V += 16
		} else {
			s.U += uint8((i - 15) * 8)
			s.V += 24
		}
	}
	for i := range numberSprites {
		s := &numberSprites[i]
		lpge.LinkSpriteToImageInfo(s, &asciiTexture)
		s.W, s.H = 8, 8
		s.U += uint8(i * 8)
		s.V += 8
	}
	for i := range allStrings {
		allStrings[i] = textString{}
	}
	number.Init(numberBuf[:])
}

// registerNumber queues a label and a number, and a newline, to be drawn at
// x, y this frame, as the original's sprintf and RegisterTextStringForDisplay
// do.
func registerNumber(label string, n int32, x, y int16) {
	number.Reset()
	number.WriteString(label).WriteInt(int(n)).WriteString("\n")
	registerTextStringForDisplay(number.Bytes(), x, y)
}

func registerTextStringForDisplay(s []byte, x, y int16) {
	for i := range allStrings {
		t := &allStrings[i]
		if t.length == 0 {
			t.x, t.y = x, y
			t.length = copy(t.data[:], s)
			return
		}
	}
}

// displayTextStrings sorts the queued strings into an ordering table, a
// sprite a character, and empties the queue. A character the font does not
// have still takes its space.
func displayTextStrings(ot *yaroze.GsOT) {
	for i := range allStrings {
		t := &allStrings[i]
		if t.length == 0 {
			continue
		}
		x := t.x
		for _, c := range t.data[:t.length] {
			if s := spriteForCharacter(c); s != nil {
				s.X, s.Y = x, t.y
				yaroze.GsSortSprite(s, ot, 0)
			}
			x += 8
		}
		t.length = 0
	}
}

func spriteForCharacter(c byte) *yaroze.GsSPRITE {
	switch {
	case c >= 'A' && c <= 'Z':
		return &letterSprites[c-'A']
	case c >= 'a' && c <= 'z':
		return &letterSprites[c-'a']
	case c >= '0' && c <= '9':
		return &numberSprites[c-'0']
	}
	return nil
}
