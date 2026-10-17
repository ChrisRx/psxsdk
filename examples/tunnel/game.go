package main

import (
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/examples/internal/lpge"
	"go.chrisrx.dev/psx/yaroze"
)

// objectSize is how close, along every axis, two objects must be to hit.
const objectSize = 32

func objectsVeryClose(a, b *lpge.Handler) bool {
	ta, tb := &a.Coord.Coord.T, &b.Coord.Coord.T
	return abs(ta[0]-tb[0]) <= objectSize && abs(ta[1]-tb[1]) <= objectSize && abs(ta[2]-tb[2]) <= objectSize
}

// playerBulletHits reports, and uses up, a player's bullet close enough to
// hit o.
func playerBulletHits(o *lpge.Handler, sound bool) {
	for j := range theBullets {
		b := &theBullets[j]
		if b.Alive && b.Allegiance == player && objectsVeryClose(o, b) {
			o.CurrentHealth--
			b.Alive = false
			if sound {
				turnOnSound(4, 0, 60, 1, 20, 96, 96)
			}
		}
	}
}

func checkCollisions() {
	if !playerInteractive {
		return
	}
	for i := range theBullets {
		b := &theBullets[i]
		if b.Alive && b.Allegiance != player && objectsVeryClose(b, &playersShip) {
			playersShip.CurrentHealth--
			b.Alive = false
			turnOnSound(4, 0, 60, 1, 20, 96, 96)
		}
	}
	for i := range tieFighters {
		f := &tieFighters[i]
		if !f.Alive {
			continue
		}
		if objectsVeryClose(f, &playersShip) {
			playersShip.CurrentHealth--
			f.CurrentHealth--
		}
		playerBulletHits(f, false)
	}
	for i := range singleAttackers {
		if a := &singleAttackers[i]; a.Alive {
			playerBulletHits(a, true)
		}
	}
	for i := range insectShips {
		s := &insectShips[i]
		if !s.Alive {
			continue
		}
		if objectsVeryClose(s, &playersShip) {
			playersShip.CurrentHealth--
			s.CurrentHealth--
		}
		playerBulletHits(s, false)
	}
	for i := range specialObjects {
		s := &specialObjects[i]
		if !s.Alive || s.ActionState == holdStill || !objectsVeryClose(s, &playersShip) {
			continue
		}
		dealWithSpecialObjectAndPlayer(s)
		// Shown still, large, top left, for a second.
		s.ActionState = holdStill
		s.Sprite.Rotate = 0
		s.ScaleX, s.ScaleY = 2*yaroze.ONE, 2*yaroze.ONE
		s.Position = yaroze.VECTOR{Vx: -screenW / 4, Vy: -screenH / 4}
		registerDelayedAction(killAllSpecialObjects, 60)
	}
	if theRobot.Alive {
		if objectsVeryClose(&theRobot, &playersShip) {
			playersShip.CurrentHealth = 0
			theRobot.CurrentHealth--
		}
		playerBulletHits(&theRobot, true)
	}
}

// explodeObject makes a sound and sends six pieces off from an object,
// two along each of its axes.
func explodeObject(o *lpge.Handler) {
	turnOnSound(2+rand()%2, 0, 60+rand()%13, 1, 30, 64, 64)
	for i := range 6 {
		c := firstDead(theCubes[:])
		if c == nil {
			continue
		}
		c.Alive = true
		c.LifeTime = 0
		c.Position = o.Position
		axis := i / 2
		sign := int32(-1)
		if i%2 != 0 {
			sign = 1
		}
		c.MovementMomentumFlag = true
		m := &o.Coord.Coord.M[axis]
		c.Velocity = yaroze.VECTOR{
			Vx: sign * int32(m[0]>>9),
			Vy: sign * int32(m[1]>>9),
			Vz: sign * int32(m[2]>>9),
		}
	}
}

// frameDelay is how long after the player dies the game goes on.
const frameDelay = 90

func handlePlayersDeath() {
	explodeObject(&playersShip)
	playersLives--
	playerInteractive = false
	if playersLives == 0 {
		registerDelayedAction(resetGame, frameDelay)
	} else {
		registerDelayedAction(turnPlayerDisplayOff, 15)
		registerDelayedAction(turnPlayerDisplayOn, frameDelay)
		registerDelayedAction(resetAfterLoseLife, frameDelay)
	}
	playersShip.CurrentHealth = 1
}

func handleEnemysDeath(o *lpge.Handler) {
	playersShip.MeritRating += findPlayersRewardForKillingObject(o)
	explodeObject(o)
	o.Alive = false
}

func findPlayersRewardForKillingObject(o *lpge.Handler) int32 {
	var reward int32
	switch o.Type {
	case tieFighter:
		reward = 10
	case singleAttacker:
		reward = 25
	case insectShip:
		reward = 5
	case robot:
		reward = 100
	}
	return reward * levelNumber
}

func handlePlayerEndOfGame() {
	basicText[0].Reset()
	basicText[0].WriteString("You have died\n")
	basicText[1].Reset()
	basicText[1].WriteString("Your score: ").WriteInt(int(playersShip.MeritRating)).WriteString("\n")
	basicText[2].Reset()
	basicText[2].WriteString("Press start\nto play again\n")
	dummyGuiLoopForTextPrinting(3, demo.PADstart)
	resetEntireProgram()
}

func resetEntireProgram() {
	restartAfterPlayerLosesALife()
	flyingSequence()
	playersLives = initialNumberOfLives
	levelNumber = 1
	frameNumber = 1
}

func restartAfterPlayerLosesALife() {
	s := &playersShip
	s.CurrentHealth = s.InitialHealth
	playerInteractive = true
	s.Position.Vx, s.Position.Vy = 0, 0
	s.Coord.Coord.T = [3]int32{}
	s.Coord.Flg = 0
	for _, set := range [][]lpge.Handler{theBullets[:], theCubes[:], theSquares[:], tetrahedrons[:], tieFighters[:], singleAttackers[:], insectShips[:]} {
		for i := range set {
			set[i].Alive = false
			set[i].CurrentHealth = set[i].InitialHealth
		}
	}
	theRobot.Alive = false
	createTheTunnel()
	registerDelayedAction(makePlayerInteractiveAgain, frameDelay)
}

// handleLevelUpdating makes everything faster and harder as the level goes
// up.
func handleLevelUpdating() {
	up := levelNumber - 1
	enemyWaveFrequency = 200 - up*25
	robotFrequency = 2000 - up*200
	tieFighterMovementSpeed = 5 + up*3
	tieFighterAngularSpeed = angularSpeed + up*6
	numberTieFightersPerWave = keepWithinRange(12+up*3, 12, maxTieFighters)
	singleAttackerMovementSpeed = 5 + up*3
	singleAttackerAngularSpeed = angularSpeed + up*6
	singleAttackerFiringRate = 24 - up*2
	singleAttackerShotSpeed = 7 + up*2
	insectShipMovementSpeed = 18 + up*3
	numberInsectShipsPerWave = keepWithinRange(24+up*4, 24, maxInsectShips)
	tunnelSpeed = 2 + up*2
	playersShip.FiringRate = 12 + up
	viewBaseVelocity = 2 + up
	viewAngularSpeed = yaroze.ONE/128 + up*(yaroze.ONE/512)
	specialObjectLikelihood = keepWithinRange(300-up*25, 120, 300)
}

// Pick-ups.
const (
	extraLife = iota
	fireQuicker
	shotsMoveFaster
	shipMovesFaster
	oscillateLights
	oscillateViewEffect
)

// effectDuration is how long a pick-up's effect lasts.
const effectDuration = 480

var (
	specialObjectLikelihood int32 = 350

	// How likely each kind of pick-up is.
	specialObjectTypeTable = [...]int32{
		extraLife,
		fireQuicker,
		shotsMoveFaster,
		shipMovesFaster,
		oscillateLights, oscillateLights, oscillateLights,
		oscillateViewEffect, oscillateViewEffect, oscillateViewEffect,
	}
)

// handleSpecialObjectClass now and then sends a pick-up, if there is none.
func handleSpecialObjectClass() {
	if frameNumber <= 0 || countNumberOfLivingTypedObjects(specialObject) != 0 || rand()%specialObjectLikelihood != 0 {
		return
	}
	o := firstDead(specialObjects[:])
	if o == nil {
		return
	}
	o.Alive = true
	o.ActionState = -1
	o.Position = yaroze.VECTOR{Vx: rand()%120 - 60, Vy: rand()%80 - 40, Vz: 6 * squareSize}
	o.Sprite.Rotate = rand() % 360 * 4096
	// As the original has it: 1 - (2*rand() % 2), which is always 1.
	o.Velocity = yaroze.VECTOR{Vx: 1 - 2*rand()%2, Vy: 1 - 2*rand()%2, Vz: -o.MovementSpeed}
	o.MovementMomentumFlag = true
	o.WhichWay = rand() % 2
	o.SubType = specialObjectTypeTable[rand()%int32(len(specialObjectTypeTable))]
}

func dealWithSpecialObjectAndPlayer(s *lpge.Handler) {
	switch s.SubType {
	case extraLife:
		playersLives++
	case fireQuicker:
		playersShip.FiringRate -= 3
		registerDelayedAction(revertPlayerFiringRate, effectDuration)
	case shotsMoveFaster:
		playersShip.ShotSpeed += 40
		registerDelayedAction(revertPlayerShotSpeed, effectDuration)
	case shipMovesFaster:
		playersShip.MovementSpeed += 3
		registerDelayedAction(revertPlayerMovementSpeed, effectDuration)
	case oscillateLights:
		lightOscillation = true
		registerDelayedAction(stopLightOscillation, effectDuration)
	case oscillateViewEffect:
		viewOscillation = true
		viewOscillationJustBegun = true
		viewDistortionMode = rand() % 4
		registerDelayedAction(stopViewOscillation, effectDuration)
	}
}

// Delayed actions: what is to happen some frames from now.
const (
	quitProgram = iota
	resetGame
	resetAfterLoseLife
	makePlayerInteractiveAgain
	turnPlayerDisplayOff
	turnPlayerDisplayOn
	revertPlayerFiringRate
	revertPlayerShotSpeed
	revertPlayerMovementSpeed
	stopLightOscillation
	stopViewOscillation
	killAllSpecialObjects
)

const maxDelayedActions = 20

type delayedAction struct {
	waiting      bool
	frameToStart int32
	typ          int32
}

var actionList [maxDelayedActions]delayedAction

func initialiseDelayedActionList() {
	for i := range actionList {
		actionList[i] = delayedAction{frameToStart: -1, typ: -1}
	}
}

func handleDelayedActionList() {
	for i := range actionList {
		a := &actionList[i]
		if a.waiting && frameNumber == a.frameToStart {
			typ := a.typ
			*a = delayedAction{frameToStart: -1, typ: -1}
			performDelayedAction(typ)
		}
	}
}

func performDelayedAction(typ int32) {
	switch typ {
	case quitProgram:
		quit = true
	case resetGame:
		handlePlayerEndOfGame()
	case resetAfterLoseLife:
		restartAfterPlayerLosesALife()
	case makePlayerInteractiveAgain:
		playerInteractive = true
	case turnPlayerDisplayOff:
		playersShip.Handler.Attribute |= yaroze.GsDOFF
	case turnPlayerDisplayOn:
		playersShip.Handler.Attribute &^= yaroze.GsDOFF
	case revertPlayerFiringRate:
		playersShip.FiringRate = 12
	case revertPlayerShotSpeed:
		playersShip.ShotSpeed = 75
	case revertPlayerMovementSpeed:
		playersShip.MovementSpeed = 5
	case stopLightOscillation:
		lightOscillation = false
		sortTheFlatLights()
	case stopViewOscillation:
		viewOscillation = false
		viewOscillationJustBegun = false
		oldViewDistortionMode = viewDistortionMode
		viewDistortionMode = -1
		view = yaroze.GsRVIEW2{Vpx: viewpointX, Vpy: viewpointY, Vpz: viewpointZ, Super: nil}
		yaroze.GsSetRefView2(&view)
	case killAllSpecialObjects:
		for i := range specialObjects {
			s := &specialObjects[i]
			s.Alive = false
			s.ActionState = -1
			s.ScaleX, s.ScaleY = yaroze.ONE, yaroze.ONE
		}
	}
}

func registerDelayedAction(typ, delay int32) {
	for i := range actionList {
		if a := &actionList[i]; !a.waiting {
			*a = delayedAction{waiting: true, typ: typ, frameToStart: frameNumber + delay}
			return
		}
	}
}

// Text the end of the game shows, in the library's font.
var (
	basicTextBuf [3][96]byte
	basicText    [3]yaroze.Buffer
)

func init() {
	for i := range basicText {
		basicText[i].Init(basicTextBuf[i][:])
	}
}

// dummyGuiLoopForTextPrinting shows the first n texts until a button of
// quitKey is pressed, a second in at the earliest.
func dummyGuiLoopForTextPrinting(n int, quitKey uint32) {
	side := yaroze.GsGetActiveBuff()
	for frames := 1; ; frames++ {
		if demo.PadRead()&quitKey != 0 && frames > 60 {
			return
		}
		yaroze.GsSetWorkBase(yaroze.Packets(&packetArea[side][0]))
		yaroze.GsClearOt(0, 0, &wot[side])
		yaroze.VSync(2)
		yaroze.ResetGraph(1)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 4, &wot[side])
		yaroze.GsDrawOt(&wot[side])
		side ^= 1
		for i := range n {
			basicText[i].Print()
		}
		yaroze.FntFlush(-1)
	}
}

// flyingSequence races down the tunnel for three seconds before the game
// starts: the field of view widening and the tunnel speeding up for the
// first, and slowing back down into the fog for the last.
func flyingSequence() {
	const totalFrames = 180
	side := yaroze.GsGetActiveBuff()
	originalSpeed := tunnelSpeed

	lightMode = 0 // no fog
	yaroze.GsSetLightMode(lightMode)
	n := performBogusWorldClipping()

	for frame := int32(1); frame <= totalFrames; frame++ {
		pad := demo.PadRead()
		if pad&demo.PADstart != 0 && pad&demo.PADselect != 0 {
			break
		}
		switch {
		case frame < 60:
			t := frame
			projection = 192 - 140*t/60
			yaroze.GsSetProjection(projection)
			tunnelSpeed = initialTunnelSpeed + t*18/60
			view.Vpz = -220 + t*50/60
		case frame == totalFrames-60:
			lightMode = 1
			yaroze.GsSetLightMode(lightMode)
			fogging.Dqa = -3960
			yaroze.GsSetFogParam(&fogging)
		case frame > totalFrames-60:
			t := frame - (totalFrames - 60)
			projection = 52 + 140*t/60
			yaroze.GsSetProjection(projection)
			tunnelSpeed = initialTunnelSpeed + (60-t)*18/60
			view.Vpz = -170 - t*50/60
			fogging.Dqa = int16(-3960 + t*3000/60)
			yaroze.GsSetFogParam(&fogging)
		}

		yaroze.GsSetWorkBase(yaroze.Packets(&packetArea[side][0]))
		yaroze.GsClearOt(0, 0, &wot[side])
		updateTheTunnel()
		yaroze.GsSetRefView2(&view)
		drawObjects(n, &wot[side])

		yaroze.VSync(0)
		yaroze.ResetGraph(1)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 4, &wot[side])
		yaroze.GsDrawOt(&wot[side])
		side ^= 1
		yaroze.FntFlush(-1)
	}

	lightMode = 1
	yaroze.GsSetLightMode(lightMode)
	tunnelSpeed = originalSpeed
}

// Sound: a VAB whose first program is a tune, started once and left
// playing, and whose others are sound effects, each keyed on a frame after
// it is asked for and off again a set number of frames later.

const maxVoices = 24

const (
	voiceOff = iota
	voiceWaiting
	voicePlaying
)

type voiceChannel struct {
	value                    int16
	status                   int
	frameToStartOn, duration int32
	program, tone, note      int16
	leftVolume, rightVolume  int16
}

var (
	vab0      int16 = -1
	theVoices [maxVoices]voiceChannel
)

func initialiseSound() {
	vab0 = yaroze.SsVabTransfer(demo.BytesAt(vhAddr), demo.BytesAt(vbAddr), -1, 1)
	if vab0 < 0 {
		return
	}
	yaroze.SsSetMVol(127, 127)
	yaroze.SsUtKeyOn(vab0, 0, 0, 60, 0, 64, 64)
	for i := range theVoices {
		theVoices[i] = voiceChannel{frameToStartOn: -1, duration: -1, program: -1, tone: -1, note: 60, leftVolume: 127, rightVolume: 127}
	}
}

func handleSound() {
	if vab0 < 0 {
		return
	}
	for i := range theVoices {
		v := &theVoices[i]
		switch v.status {
		case voiceWaiting:
			if frameNumber == v.frameToStartOn {
				v.value = yaroze.SsUtKeyOn(vab0, v.program, v.tone, v.note, 0, v.leftVolume, v.rightVolume)
				v.status = voicePlaying
			}
		case voicePlaying:
			if frameNumber == v.frameToStartOn+v.duration {
				yaroze.SsUtKeyOff(v.value, vab0, v.program, v.tone, v.note)
				v.status = voiceOff
			}
		}
	}
}

func turnOnSound(program, tone, note, delay, duration, left, right int32) {
	for i := range theVoices {
		v := &theVoices[i]
		if v.status == voiceOff {
			*v = voiceChannel{
				status:         voiceWaiting,
				program:        int16(program),
				tone:           int16(tone),
				note:           int16(note),
				frameToStartOn: frameNumber + delay,
				duration:       duration,
				leftVolume:     int16(left),
				rightVolume:    int16(right),
			}
			return
		}
	}
}

func cleanUpSound() {
	if vab0 >= 0 {
		yaroze.SsVabClose(vab0)
	}
}
