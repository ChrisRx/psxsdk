package main

import (
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/examples/internal/lpge"
	"go.chrisrx.dev/psx/yaroze"
)

// Object types.
const (
	player = iota
	bullet
	enemy
	cube
	square
)

const (
	maxBullets = 50
	maxShips   = 6
	maxCubes   = 20
	maxSquares = 400
)

// Enemy strategies.
const (
	waiting = iota
	stationaryTrack
	seeking
	inactive
)

var (
	playersShip lpge.Handler
	theBullets  [maxBullets]lpge.Handler
	theShips    [maxShips]lpge.Handler
	theCubes    [maxCubes]lpge.Handler // the pieces of explosions
	theSquares  [maxSquares]lpge.Handler
)

func initialiseObjects() {
	lpge.InitialiseObjectClass()

	s := &playersShip
	s.Init()
	s.BringToLife(player, shipModelAddr, 0, lpge.None)
	lpge.Register(s)
	positionSomewhere(s)
	s.InitialHealth, s.CurrentHealth = 1000, 1000
	s.Allegiance = player
	s.MovementSpeed = 22
	s.RotationSpeed = yaroze.ONE / 64
	s.FiringRate = 12
	s.FramesSinceLastFire = 0
	s.ShotSpeed = 75

	for i := range theBullets {
		o := &theBullets[i]
		o.Init()
		o.BringToLife(bullet, 0, 0, lpge.None)
		o.DisplayFlag = lpge.Sprite
		o.ImageInfo = &fireTexture
		lpge.Register(o)
	}
	for i := range theCubes {
		o := &theCubes[i]
		o.Init()
		o.BringToLife(cube, 0, 0, lpge.None)
		o.DisplayFlag = lpge.Sprite
		o.ImageInfo = &multipleTexture
		o.ScaleX, o.ScaleY = 2*yaroze.ONE, 2*yaroze.ONE
		lpge.Register(o)
	}
	for i := range theShips {
		o := &theShips[i]
		o.Init()
		o.BringToLife(enemy, shipModelAddr, 0, lpge.None)
		o.InitialHealth, o.CurrentHealth = 10, 10
		o.FiringRate = 12
		o.FramesSinceLastFire = 0
		o.ShotSpeed = 75
		o.StrategyFlag = inactive
		initialiseShipAccordingToStrategy(o)
		positionSomewhere(o)
		o.Allegiance = enemy
		lpge.Register(o)
	}
	for i := range theSquares {
		o := &theSquares[i]
		o.Init()
		o.BringToLife(square, squareModelAddr, 0, lpge.None)
		x := int32(i / worldX)
		z := int32(i % worldZ)
		o.Position = yaroze.VECTOR{Vx: x * gridSize, Vy: 0, Vz: z * gridSize}
		o.Rotate.Vx = yaroze.ONE / 2
		setCoordinates(&o.Rotate, &o.Position, &o.Coord)
		lpge.Register(o)
	}

	lpge.LinkAllObjectsToModelsOrSprites()
	lpge.LinkAllObjectsToTheirCoordinateSystems()

	for i := range theBullets {
		theBullets[i].Alive = false
	}
	for i := range theCubes {
		o := &theCubes[i]
		o.Alive = false
		// Only a corner of the texture, starting at a random turn.
		o.Sprite.W /= 4
		o.Sprite.H /= 4
		o.Sprite.U += 32
		o.Sprite.V += 64
		o.Sprite.Rotate = rand() % 360 * 4096
	}
	for i := range theShips {
		theShips[i].Alive = false
	}
}

// initialiseShipAccordingToStrategy gives an enemy the player's speeds, and
// a firing rate by its strategy.
func initialiseShipAccordingToStrategy(o *lpge.Handler) {
	o.MovementSpeed = 22
	o.RotationSpeed = yaroze.ONE / 64
	switch o.StrategyFlag {
	case waiting:
		o.SpecialTimeLimit = 400 + 100*(rand()%4)
		o.FiringRate = 6
	case stationaryTrack:
		o.FiringRate = 5
	case seeking:
		o.FiringRate = 7
	}
}

// positionSomewhere puts an object on a random tile, half a tile up.
func positionSomewhere(o *lpge.Handler) {
	o.Position.Vx = rand() % worldX * gridSize
	o.Position.Vy = -gridSize / 2
	o.Position.Vz = rand() % worldZ * gridSize
}

const shipCreationFrequency = 120

// handleShipClass sends an enemy when there is none, one time in 120 a
// frame: a sitting tracker or a seeker, by a coin toss.
func handleShipClass() {
	if countNumberOfLivingTypedObjects(enemy) != 0 || rand()%shipCreationFrequency != 0 {
		return
	}
	s := &theShips[0]
	s.Alive = true
	s.CurrentHealth = 10
	positionSomewhere(s)
	if rand()%2 == 0 {
		s.StrategyFlag = stationaryTrack
	} else {
		s.StrategyFlag = seeking
	}
}

func countNumberOfLivingTypedObjects(typ int32) int {
	n := 0
	for _, o := range &lpge.Objects {
		if o != nil && o.Alive && o.Type == typ {
			n++
		}
	}
	return n
}

func handleAllObjects() {
	checkCollisions()
	handleShipClass()

	for _, o := range &lpge.Objects {
		if o == nil || !o.Alive {
			continue
		}
		o.LifeTime++
		switch o.Type {
		case player:
			moveShipWorldRelative(o)
			o.Rotate.Vx += int16(o.Twist.Vx)
			o.Rotate.Vy += int16(o.Twist.Vy)
			o.Rotate.Vz += int16(o.Twist.Vz)
			o.SortSize()
			o.Position.Vx = keepWithinRange(o.Position.Vx, 0, worldMaxX)
			o.Position.Vz = keepWithinRange(o.Position.Vz, 0, worldMaxZ)
			loseMomentum(o)
			if o.CurrentHealth < 1 {
				handlePlayersDeath()
				return
			}
		case bullet:
			handleBullet(o)
		case cube:
			handleCube(o)
		case enemy:
			handleEnemyBehaviour(o)
			updateObjectCoordinates(o)
			o.SortSize()
			o.Position.Vx = keepWithinRange(o.Position.Vx, 0, worldMaxX)
			o.Position.Vz = keepWithinRange(o.Position.Vz, 0, worldMaxZ)
			loseMomentum(o)
			if o.CurrentHealth < 1 {
				handleEnemysDeath(o)
			}
		case square:
			dealWithSquareAttribute(o)
		}
	}
}

// loseMomentum stops an object, and its turning, unless it keeps them.
func loseMomentum(o *lpge.Handler) {
	if !o.MovementMomentumFlag {
		o.Velocity = yaroze.VECTOR{}
	}
	if !o.RotationMomentumFlag {
		o.Twist = yaroze.VECTOR{}
	}
}

// offThePlane reports whether an object's tile is beyond the plane, or
// higher above it than the world is tall.
func offThePlane(o *lpge.Handler) bool {
	gridX := o.Position.Vx / gridSize
	gridY := o.Position.Vy / gridSize
	gridZ := o.Position.Vz / gridSize
	return gridX < 0 || gridX >= worldX || gridY < 0 || gridY >= worldY || gridZ < 0 || gridZ >= worldZ
}

// moveSprite is what bullets and explosions share: a step by the velocity,
// relative to the world, and a turn by the twist.
func moveSprite(o *lpge.Handler) {
	setCoordinates(&o.Rotate, &o.Position, &o.Coord)
	o.SortSize()
	o.Rotate.Vx += int16(o.Twist.Vx)
	o.Rotate.Vy += int16(o.Twist.Vy)
	o.Rotate.Vz += int16(o.Twist.Vz)
	o.Position.Vx += o.Velocity.Vx
	o.Position.Vy += o.Velocity.Vy
	o.Position.Vz += o.Velocity.Vz
	loseMomentum(o)
}

func handleBullet(o *lpge.Handler) {
	moveSprite(o)
	if o.LifeTime > 50 || offThePlane(o) {
		o.Alive = false
		o.LifeTime = 0
		o.Sprite.Attribute &^= yaroze.GsDOFF
	}
}

// handleCube moves a piece of an explosion: it spins, dims and shrinks
// until it goes.
func handleCube(o *lpge.Handler) {
	moveSprite(o)
	o.Sprite.Rotate += 10 * 4096
	if o.LifeTime > 25 {
		o.Alive = false
		o.LifeTime = 0
		o.Sprite.Attribute &^= yaroze.GsDOFF
	}
	if o.LifeTime%5 == 0 {
		sortCubeSpriteDimming(o)
	}
	if offThePlane(o) {
		o.Alive = false
		o.LifeTime = 0
		o.Sprite.Attribute &^= yaroze.GsDOFF
	}
}

// sortCubeSpriteDimming fades a piece of an explosion from full brightness
// to none over its 25 frames, and shrinks it back to its own size.
func sortCubeSpriteDimming(o *lpge.Handler) {
	c := uint8(128 - 5*o.LifeTime)
	o.Sprite.R, o.Sprite.G, o.Sprite.B = c, c, c
	o.ScaleX = 2*yaroze.ONE - o.LifeTime*160
	o.ScaleY = 2*yaroze.ONE - o.LifeTime*160
}

// dealWithSquareAttribute subdivides the tiles near the viewpoint.
func dealWithSquareAttribute(o *lpge.Handler) {
	veryCloseDistance = findSubdivisionDistanceFromViewMode()
	if abs(o.Position.Vx-viewPointX) < veryCloseDistance*gridSize && abs(o.Position.Vz-viewPointZ) < veryCloseDistance*gridSize {
		o.Handler.Attribute |= yaroze.GsDIV1
	} else {
		// The original clears the display-off bit here, not the
		// subdivision, so a tile once subdivided stays so.
		o.Handler.Attribute &^= yaroze.GsDOFF
	}
}

// updateObjectCoordinates is the tutorial's UpdateObjectCoordinates: a move
// by the velocity along the object's own axes and a turn by the twist about
// them, kept in its matrix.
func updateObjectCoordinates(o *lpge.Handler) {
	var realMovement yaroze.VECTOR
	yaroze.ApplyMatrixLV(&o.Matrix, &o.Velocity, &realMovement)
	o.Position.Vx += realMovement.Vx
	o.Position.Vy += realMovement.Vy
	o.Position.Vz += realMovement.Vz

	xVector := yaroze.SVECTOR{Vx: int16(o.Twist.Vx)}
	yVector := yaroze.SVECTOR{Vy: int16(o.Twist.Vy)}
	zVector := yaroze.SVECTOR{Vz: int16(o.Twist.Vz)}
	var xMatrix, yMatrix, zMatrix yaroze.MATRIX
	yaroze.RotMatrix(&xVector, &xMatrix)
	yaroze.RotMatrix(&yVector, &yMatrix)
	yaroze.RotMatrix(&zVector, &zMatrix)
	yaroze.MulMatrix0(&o.Matrix, &xMatrix, &o.Matrix)
	yaroze.MulMatrix0(&o.Matrix, &yMatrix, &o.Matrix)
	yaroze.MulMatrix0(&o.Matrix, &zMatrix, &o.Matrix)

	o.Coord.Coord = o.Matrix
	o.Coord.Coord.T = [3]int32{o.Position.Vx, o.Position.Vy, o.Position.Vz}
	o.Coord.Flg = 0
}

// setCoordinates is the tutorial's UpdateObjectCoordinates2: rotation and
// position, both relative to the world.
func setCoordinates(rotation *yaroze.SVECTOR, position *yaroze.VECTOR, c *yaroze.GsCOORDINATE2) {
	yaroze.RotMatrix(rotation, &c.Coord)
	c.Coord.T = [3]int32{position.Vx, position.Vy, position.Vz}
	c.Flg = 0
}

// moveShipWorldRelative is the tutorial's UpdateObjectCoordinates3: a turn
// relative to the world, and a move by the velocity along the object's own
// axes.
func moveShipWorldRelative(o *lpge.Handler) {
	var tempMatrix yaroze.MATRIX
	yaroze.RotMatrix(&o.Rotate, &tempMatrix)
	var realMovement yaroze.VECTOR
	yaroze.ApplyMatrixLV(&tempMatrix, &o.Velocity, &realMovement)
	o.Coord.Coord = tempMatrix
	o.Position.Vx += realMovement.Vx
	o.Position.Vy += realMovement.Vy
	o.Position.Vz += realMovement.Vz
	o.Coord.Coord.T = [3]int32{o.Position.Vx, o.Position.Vy, o.Position.Vz}
	o.Coord.Flg = 0
}

// firesAShot sends a free bullet from an object the way its model faces,
// which is backwards along its own z axis.
func firesAShot(o *lpge.Handler) {
	b := firstDead(theBullets[:])
	if b == nil {
		return
	}
	b.Alive = true
	b.LifeTime = 0
	b.Allegiance = o.Allegiance
	m := &o.Coord.Coord.M
	b.Velocity = yaroze.VECTOR{
		Vx: o.ShotSpeed * int32(m[2][0]) / 4096,
		Vy: 0,
		Vz: -(o.ShotSpeed * int32(m[2][2]) / 4096),
	}
	b.MovementMomentumFlag = true
	b.Position = o.Position
}

func playerFiresAShot() { firesAShot(&playersShip) }

// firstDead returns the first object in a set that is not alive, or nil.
func firstDead(objs []lpge.Handler) *lpge.Handler {
	for i := range objs {
		if !objs[i].Alive {
			return &objs[i]
		}
	}
	return nil
}

func handleEnemyBehaviour(o *lpge.Handler) {
	o.FramesSinceLastFire++
	switch o.StrategyFlag {
	case stationaryTrack:
		trackThePlayer(o, false)
	case seeking:
		trackThePlayer(o, true)
	}
}

const (
	distanceNearEnough = 300
	xzRatio            = 10
)

// trackThePlayer turns an enemy towards the player's ship and fires when
// it is close; one that pursues moves towards it as well.
func trackThePlayer(o *lpge.Handler, pursue bool) {
	// The way to the player, in the world, and then in the enemy's own
	// coordinate system, whose matrix holds the world's axes in its terms.
	w := yaroze.VECTOR{
		Vx: playersShip.Position.Vx - o.Position.Vx,
		Vy: playersShip.Position.Vy - o.Position.Vy,
		Vz: playersShip.Position.Vz - o.Position.Vz,
	}
	m := &o.Coord.Coord.M
	dx := -(w.Vx*int32(m[0][0]) + w.Vy*int32(m[1][0]) + w.Vz*int32(m[2][0])) / yaroze.ONE
	dz := -((w.Vx*int32(m[0][2]) + w.Vy*int32(m[1][2]) + w.Vz*int32(m[2][2])) / yaroze.ONE) // the model points the wrong way

	fireIfClose := func() {
		if dz < distanceNearEnough {
			if o.FramesSinceLastFire > o.FiringRate {
				firesAShot(o)
				o.FramesSinceLastFire = 0
			}
		} else if pursue {
			o.Velocity.Vz -= o.MovementSpeed
		}
	}
	switch {
	case dx == 0:
		if dz > 0 { // ahead
			fireIfClose()
		} else { // behind: turn right
			o.Twist.Vy += o.RotationSpeed
		}
	case abs(dx)*xzRatio <= dz && dz > 0: // almost directly ahead
		fireIfClose()
	case dx > 0: // on the right
		o.Twist.Vy += o.RotationSpeed
	default: // on the left
		o.Twist.Vy -= o.RotationSpeed
	}
}

// objectSize is how close, along every axis, two objects must be to hit.
const objectSize = 32

func objectsVeryClose(a, b *lpge.Handler) bool {
	return abs(a.Position.Vx-b.Position.Vx) <= objectSize &&
		abs(a.Position.Vy-b.Position.Vy) <= objectSize &&
		abs(a.Position.Vz-b.Position.Vz) <= objectSize
}

// checkCollisions costs the player's ship health for an enemy or an enemy's
// bullet touching it, and an enemy health for the player's bullets.
func checkCollisions() {
	for i := range theShips {
		s := &theShips[i]
		if s.Alive && objectsVeryClose(s, &playersShip) {
			playersShip.CurrentHealth--
			s.CurrentHealth--
		}
	}
	for i := range theBullets {
		b := &theBullets[i]
		if b.Alive && b.Allegiance == enemy && objectsVeryClose(b, &playersShip) {
			playersShip.CurrentHealth--
		}
	}
	for i := range theShips {
		s := &theShips[i]
		if !s.Alive {
			continue
		}
		for j := range theBullets {
			b := &theBullets[j]
			if b.Alive && b.Allegiance == player && objectsVeryClose(b, s) {
				s.CurrentHealth--
			}
		}
	}
}

// explodeObject sends six pieces off from an object, two along each of its
// axes.
func explodeObject(o *lpge.Handler) {
	o.Alive = false
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

func handleEnemysDeath(o *lpge.Handler) {
	// A crude reward: more for the harder strategies.
	playersShip.MeritRating += o.InitialHealth * (o.StrategyFlag + 1)
	explodeObject(o)
}

// handlePlayersDeath blows the ship up, shows the score until Start is
// pressed, and ends the program.
func handlePlayersDeath() {
	explodeObject(&playersShip)
	basicText[0].Reset()
	basicText[0].WriteString("You have died\n")
	basicText[1].Reset()
	basicText[1].WriteString("Your score: ").WriteInt(int(playersShip.MeritRating)).WriteString("\n")
	basicText[2].Reset()
	basicText[2].WriteString("Press start\nto play again\n")
	dummyGuiLoopForTextPrinting(3, demo.PADstart)
	quitFrameNumber = frameNumber
}

// Text shown on its own, in the library's font.
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
		yaroze.GsSetWorkBase(&packetArea[side][0])
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
