package main

import (
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
	tetrahedron
	tieFighter
	singleAttacker
	insectShip
	specialObject
	robot
)

const (
	initialNumberOfLives = 5

	maxBullets         = 50
	maxCubes           = 20
	maxSquares         = 400 // a multiple of 4: a ring of the tunnel each
	maxTetrahedrons    = 64
	maxTieFighters     = 24
	maxSingleAttackers = 24
	maxInsectShips     = 36
	maxSpecialObjects  = 6

	// A special object being shown still, once picked up.
	holdStill = 57

	// Where the player may go, across the tunnel.
	playerMinX = -65
	playerMaxX = 65
	playerMinY = -50
	playerMaxY = 50
	playerMinZ = 0

	// Where fighters may go: ten units in from the tunnel's sides.
	tieFighterMinX = -86
	tieFighterMaxX = 86
	tieFighterMinY = -54
	tieFighterMaxY = 54

	robotMinX = -50
	robotMaxX = 50
	robotMinY = -35
	robotMaxY = 35

	clockwise     = 0
	anticlockwise = 1

	switchPeriod = 45
	angularSpeed = yaroze.ONE / 128
	radius       = 48
	zDistance    = squareSize * 4
	zSeparation  = squareSize

	insectInitialZ    = squareSize * 4
	insectZSeparation = squareSize / 2

	// Enemy classes, for a wave. The robot comes by itself.
	tieFighterClass     = 0
	singleAttackerClass = 1
	insectShipClass     = 2
	numberEnemyClasses  = 3

	initialTunnelSpeed = 8
)

// Fighter wave movements.
const (
	staticHelix = iota
	switchingHelix
	dartingStaticHelix
	dartingSwitchingHelix
	numberWaveTypes
)

// Lone attackers' movements.
const (
	randomJiggler = iota
	xyEvader
	staticCircler
	switchingCircler
	numberAttackerSubtypes
)

var (
	playersShip     lpge.Handler
	theBullets      [maxBullets]lpge.Handler
	theCubes        [maxCubes]lpge.Handler
	theSquares      [maxSquares]lpge.Handler
	tetrahedrons    [maxTetrahedrons]lpge.Handler
	tieFighters     [maxTieFighters]lpge.Handler
	singleAttackers [maxSingleAttackers]lpge.Handler
	insectShips     [maxInsectShips]lpge.Handler
	specialObjects  [maxSpecialObjects]lpge.Handler
	theRobot        lpge.Handler

	playersLives      int32
	levelNumber       int32 = 1
	playerInteractive bool

	scalingVector yaroze.VECTOR

	sectionLength, tunnelWidth, tunnelHeight    int32
	numberTunnelSections, tunnelSpeed           int32
	leastX, mostX, leastY, mostY, leastZ, mostZ int32

	// Parameters that the levels speed up.
	enemyWaveFrequency          int32
	robotFrequency              int32
	tieFighterMovementSpeed     int32
	tieFighterAngularSpeed      int32
	numberTieFightersPerWave    int32
	singleAttackerMovementSpeed int32
	singleAttackerAngularSpeed  int32
	singleAttackerFiringRate    int32
	singleAttackerShotSpeed     int32
	insectShipMovementSpeed     int32
	numberInsectShipsPerWave    int32
	robotCirclingSpeed          int32
	robotMovementSpeed          int32
	robotRotationSpeed          int32
	robotPeriod                 int32
	robotZPeriod                int32
	robotFiringPeriod           int32

	// Fighters darting together.
	dartOn         bool
	dartDirection  int32 = -1
	dartFrameStart int32 = -1
	dartDuration   int32 = -1

	// Positions round a circle, worked out once; nothing uses them.
	circularXs, circularYs [256]int32
	numberPositions        int32
)

func initializeObjects() {
	lpge.InitializeObjectClass()
	initializeEnemyMovementGlobals()
	playersLives = initialNumberOfLives
	levelNumber = 1
	playerInteractive = true

	s := &playersShip
	s.Init()
	s.BringToLife(player, shipModel, 0, lpge.None)
	lpge.Register(s)
	s.Position = yaroze.VECTOR{}
	s.Rotate.Vy = yaroze.ONE / 2
	setCoordinates(&s.Rotate, &s.Position, &s.Coord)
	s.InitialHealth, s.CurrentHealth = 12, 12
	s.Allegiance = player
	s.MovementSpeed = 5
	s.RotationSpeed = yaroze.ONE / 64
	s.CanFire = true
	s.FiringRate = 12
	s.FramesSinceLastFire = 0
	s.ShotSpeed = 75
	s.ShotType = lpge.FireSprite
	s.SetScaling(yaroze.ONE/2, yaroze.ONE/2, yaroze.ONE/2)
	s.SortSize()

	for i := range theBullets {
		o := &theBullets[i]
		o.Init()
		o.BringToLife(bullet, squareModel, 0, lpge.None)
		o.DisplayFlag = lpge.Sprite
		o.ImageInfo = &fireTexture
		lpge.Register(o)
	}
	for i := range theCubes {
		o := &theCubes[i]
		o.Init()
		o.BringToLife(cube, squareModel, 0, lpge.None)
		o.DisplayFlag = lpge.Sprite
		o.ImageInfo = &multipleTexture
		lpge.Register(o)
	}
	for i := range tetrahedrons {
		o := &tetrahedrons[i]
		o.Init()
		o.BringToLife(tetrahedron, multipleModel, 0, lpge.None)
		o.InitialHealth, o.CurrentHealth = 1, 1
		o.Allegiance = enemy
		o.MovementSpeed = 5
		o.SetScaling(yaroze.ONE/4, yaroze.ONE/4, yaroze.ONE/4)
		o.SortSize()
		lpge.Register(o)
	}
	for i := range tieFighters {
		o := &tieFighters[i]
		o.Init()
		o.BringToLife(tieFighter, multipleModel, 6, lpge.None)
		o.InitialHealth, o.CurrentHealth = 1, 1
		o.Allegiance = enemy
		o.MovementSpeed = tieFighterMovementSpeed
		o.SetScaling(yaroze.ONE/2, yaroze.ONE/2, yaroze.ONE/2)
		o.SortSize()
		lpge.Register(o)
	}
	for i := range insectShips {
		o := &insectShips[i]
		o.Init()
		o.BringToLife(insectShip, multipleModel, 3, lpge.None)
		o.InitialHealth, o.CurrentHealth = 1, 1
		o.Allegiance = enemy
		o.MovementSpeed = insectShipMovementSpeed
		o.SetScaling(yaroze.ONE/4, yaroze.ONE/4, yaroze.ONE/4)
		o.SortSize()
		lpge.Register(o)
	}
	for i := range singleAttackers {
		o := &singleAttackers[i]
		o.Init()
		o.BringToLife(singleAttacker, multipleModel, 6, lpge.None)
		o.InitialHealth, o.CurrentHealth = 4, 4
		o.Allegiance = enemy
		o.MovementSpeed = singleAttackerMovementSpeed
		o.CanFire = true
		o.FiringRate = singleAttackerFiringRate
		o.FramesSinceLastFire = 0
		o.ShotSpeed = singleAttackerShotSpeed
		o.ShotType = lpge.FireSprite
		o.SetScaling(yaroze.ONE/2, yaroze.ONE/2, yaroze.ONE/2)
		o.SortSize()
		lpge.Register(o)
	}
	for i := range theSquares {
		o := &theSquares[i]
		o.Init()
		o.BringToLife(square, squareModel, 0, lpge.None)
		lpge.Register(o)
	}
	for i := range specialObjects {
		o := &specialObjects[i]
		o.Init()
		o.BringToLife(specialObject, squareModel, 0, lpge.None)
		o.DisplayFlag = lpge.Sprite
		o.ImageInfo = &mascotsTexture
		o.MovementSpeed = 8
		lpge.Register(o)
	}
	r := &theRobot
	r.Init()
	r.BringToLife(robot, multipleModel, 9, lpge.None)
	r.InitialHealth, r.CurrentHealth = 10, 10
	r.Allegiance = robot
	r.MovementSpeed = 5
	r.RotationSpeed = yaroze.ONE / 32
	r.CanFire = true
	r.FiringRate = 9
	r.FramesSinceLastFire = 0
	r.ShotSpeed = 25
	r.ShotType = lpge.FireSprite
	r.SetScaling(yaroze.ONE/12, yaroze.ONE/12, yaroze.ONE/12)
	r.SortSize()
	lpge.Register(r)

	lpge.LinkAllObjectsToModelsOrSprites()
	lpge.LinkAllObjectsToTheirCoordinateSystems()

	for i := range theBullets {
		theBullets[i].Alive = false
		theBullets[i].Sprite.Attribute |= yaroze.GsALON
	}
	for i := range theCubes {
		o := &theCubes[i]
		o.Alive = false
		// Only a corner of the texture, turning about its middle.
		o.Sprite.W /= 4
		o.Sprite.H /= 4
		o.Sprite.U += 32
		o.Sprite.V += 64
		o.Sprite.Mx = int16(o.Sprite.W / 2)
		o.Sprite.My = int16(o.Sprite.H / 2)
		// Semi-transparent, adding.
		o.Sprite.Attribute |= yaroze.GsALON | 1<<28 | 1<<29
		o.Sprite.Rotate = rand() % 360 * 4096
	}
	for i := range theSquares {
		theSquares[i].Alive = false
	}
	for i := range tetrahedrons {
		tetrahedrons[i].Alive = false
	}
	for i := range tieFighters {
		tieFighters[i].Alive = false
	}
	for i := range singleAttackers {
		singleAttackers[i].Alive = false
	}
	for i := range insectShips {
		insectShips[i].Alive = false
	}
	for i := range specialObjects {
		o := &specialObjects[i]
		o.Alive = false
		// One mascot each from the texture. The original centers them
		// on the cubes' middles, not their own.
		o.Sprite.W, o.Sprite.H = 34, 60
		o.Sprite.U += uint8(i * 34)
		o.Sprite.Mx = int16(theCubes[i].Sprite.W / 2)
		o.Sprite.My = int16(theCubes[i].Sprite.H / 2)
		o.Sprite.Attribute |= yaroze.GsALON
		o.SubType = int32(i)
	}
	theRobot.Alive = false
}

func initializeEnemyMovementGlobals() {
	enemyWaveFrequency = 200
	robotFrequency = 2000
	tieFighterMovementSpeed = 5
	tieFighterAngularSpeed = angularSpeed
	numberTieFightersPerWave = 12
	singleAttackerMovementSpeed = 5
	singleAttackerAngularSpeed = angularSpeed
	singleAttackerFiringRate = 24
	singleAttackerShotSpeed = 7
	insectShipMovementSpeed = 18
	numberInsectShipsPerWave = 24
	robotCirclingSpeed = yaroze.ONE / 128
	robotMovementSpeed = 7
	robotRotationSpeed = yaroze.ONE / 32
	robotPeriod = 80
	robotZPeriod = 80
	robotFiringPeriod = 80
}

func initializeTunnel() {
	sectionLength = squareSize
	tunnelWidth = squareSize * 3 / 2
	tunnelHeight = squareSize
	numberTunnelSections = 20
	tunnelSpeed = initialTunnelSpeed
	leastX, mostX = -tunnelWidth/2, tunnelWidth/2
	leastY, mostY = -tunnelHeight/2, tunnelHeight/2
	leastZ, mostZ = -sectionLength, numberTunnelSections*sectionLength
	createTheTunnel()
}

// createTheTunnel sets out the tunnel's rings, each the two walls, the
// floor and the ceiling, as squares scaled to fit.
func createTheTunnel() {
	// The original means to reset every square here, but resets the first,
	// as many times as there are squares. The rest keep their rotations,
	// which are the ones they are about to be given again.
	theSquares[0].Position = yaroze.VECTOR{}
	theSquares[0].Rotate = yaroze.SVECTOR{}
	id := 0
	for i := range numberTunnelSections {
		z := i * sectionLength
		if id+3 >= maxSquares {
			return
		}
		sides := [4]struct {
			x, y, rz, width int32
		}{
			{tunnelWidth / 2, 0, yaroze.ONE / 4, tunnelWidth},
			{-tunnelWidth / 2, 0, 3 * yaroze.ONE / 4, tunnelWidth},
			{0, tunnelHeight / 2, yaroze.ONE / 2, tunnelHeight},
			{0, -tunnelHeight / 2, 0, tunnelHeight},
		}
		for _, side := range sides {
			s := &theSquares[id]
			s.Alive = true
			s.Position = yaroze.VECTOR{Vx: side.x, Vy: side.y, Vz: z}
			s.Rotate.Vz = int16(side.rz)
			setCoordinates(&s.Rotate, &s.Position, &s.Coord)
			s.SetScaling(yaroze.ONE*side.width/squareSize, yaroze.ONE, yaroze.ONE*sectionLength/squareSize)
			s.SortSize()
			id++
		}
	}
}

// updateTheTunnel moves the tunnel towards the ship, a ring that falls
// well behind going round to the far end, and subdivides the rings level
// with the ship and behind it.
func updateTheTunnel() {
	tunnelLength := numberTunnelSections * sectionLength
	veryFarBack := -sectionLength * 2
	for i := range theSquares {
		s := &theSquares[i]
		if !s.Alive {
			continue
		}
		s.Coord.Coord.T[2] -= tunnelSpeed
		if s.Coord.Coord.T[2] <= veryFarBack {
			s.Coord.Coord.T[2] += tunnelLength
		}
		s.Coord.Flg = 0
	}
	for i := 0; i < maxSquares; i += 4 {
		if !theSquares[i].Alive {
			continue
		}
		near := theSquares[i].Coord.Coord.T[2]-playersShip.Coord.Coord.T[2] <= 0
		for j := i; j < i+4; j++ {
			if near {
				theSquares[j].Handler.Attribute |= yaroze.GsDIV1
			} else {
				theSquares[j].Handler.Attribute &^= yaroze.GsDIV1
			}
		}
	}
}

func handleAllObjects() {
	checkCollisions()
	handleEnemyClass()
	handleSpecialObjectClass()
	for _, o := range &lpge.Objects {
		if o == nil || !o.Alive {
			continue
		}
		o.LifeTime++
		if o.CanFire {
			o.FramesSinceLastFire++
		}
		switch o.Type {
		case player:
			moveWorldRelative(o)
			if o.CurrentHealth < 1 {
				handlePlayersDeath()
				return
			}
		case bullet:
			handleABullet(o)
		case cube:
			handleACube(o)
		case square:
			// updateTheTunnel moves the squares.
		case tetrahedron:
			handleATetrahedron(o)
		case tieFighter:
			handleATieFighter(o)
		case singleAttacker:
			handleSingleAttacker(o)
		case insectShip:
			handleAnInsectShip(o)
		case specialObject:
			handleSpecialObject(o)
		case robot:
			handleRobot(o)
		}
	}
	handleTieFighterWaves()
}

// outOfTunnel reports whether an object has left the tunnel's box.
func outOfTunnel(o *lpge.Handler) bool {
	p := &o.Position
	return p.Vx <= leastX || p.Vx >= mostX || p.Vy <= leastY || p.Vy >= mostY || p.Vz <= leastZ || p.Vz >= mostZ
}

const (
	bulletLifetime      = 60
	cubeLifetime        = 24
	tetrahedronLifetime = 60
)

func handleABullet(o *lpge.Handler) {
	moveWorldRelative(o)
	if o.LifeTime > bulletLifetime || outOfTunnel(o) {
		o.Alive = false
		o.LifeTime = 0
	}
}

// handleACube moves a piece of an explosion: it spins, dims and shrinks
// until it goes.
func handleACube(o *lpge.Handler) {
	moveWorldRelative(o)
	o.Sprite.Rotate += 10 * 4096
	if o.LifeTime > cubeLifetime {
		o.Alive = false
		o.LifeTime = 0
	}
	if o.LifeTime%3 == 0 {
		c := uint8(128 - 4*o.LifeTime)
		o.Sprite.R, o.Sprite.G, o.Sprite.B = c, c, c
		o.ScaleX = yaroze.ONE - o.LifeTime*160
		o.ScaleY = yaroze.ONE - o.LifeTime*160
	}
	if outOfTunnel(o) {
		o.Alive = false
		o.LifeTime = 0
	}
}

func handleATetrahedron(o *lpge.Handler) {
	moveWorldRelative(o)
	if o.LifeTime > tetrahedronLifetime || outOfTunnel(o) {
		o.Alive = false
		o.LifeTime = 0
	}
}

// circle moves an object's angle on round its circle.
func circle(o *lpge.Handler) {
	switch o.WhichWay {
	case clockwise:
		o.Angle += o.AngleIncrement
	case anticlockwise:
		o.Angle -= o.AngleIncrement
	}
}

func handleATieFighter(o *lpge.Handler) {
	circle(o)
	t := &o.Coord.Coord.T
	t[0] = keepWithinRange(o.FirstVector.Vx+o.Radius*lpge.Rcos(o.Angle)>>12, tieFighterMinX, tieFighterMaxX)
	t[1] = keepWithinRange(o.FirstVector.Vy+o.Radius*lpge.Rsin(o.Angle)>>12, tieFighterMinY, tieFighterMaxY)
	t[2] -= o.MovementSpeed
	o.Coord.Flg = 0
	if t[2] < -sectionLength {
		o.Alive = false
	}
	if o.CurrentHealth <= 0 {
		handleEnemysDeath(o)
	}
}

// jiggle moves an object a step one of four ways.
func jiggle(o *lpge.Handler, way int32) {
	switch way {
	case 0:
		o.Position.Vx -= o.MovementSpeed
	case 1:
		o.Position.Vx += o.MovementSpeed
	case 2:
		o.Position.Vy -= o.MovementSpeed
	case 3:
		o.Position.Vy += o.MovementSpeed
	}
}

func handleSingleAttacker(o *lpge.Handler) {
	const waitingDistance = 2 * squareSize
	if o.Position.Vz >= waitingDistance {
		o.Position.Vz -= o.MovementSpeed
	}

	dir := yaroze.VECTOR{
		Vx: playersShip.Position.Vx - o.Position.Vx,
		Vy: playersShip.Position.Vy - o.Position.Vy,
		Vz: playersShip.Position.Vz - o.Position.Vz,
	}
	handleObjectContinuousFiring(o, &dir)

	switch o.SubType {
	case randomJiggler:
		if o.LifeTime%o.SpecialTimeLimit == 0 {
			o.ActionState = rand() % 4
		}
		jiggle(o, o.ActionState)
		o.Position.Vx = keepWithinRange(o.Position.Vx, playerMinX, playerMaxX)
		o.Position.Vy = keepWithinRange(o.Position.Vy, playerMinY, playerMaxY)
	case xyEvader:
		x := playersShip.Coord.Coord.T[0] - o.Coord.Coord.T[0]
		y := playersShip.Coord.Coord.T[1] - o.Coord.Coord.T[1]
		if abs(x) < o.MovementSpeed && abs(y) < o.MovementSpeed {
			jiggle(o, rand()%4)
		} else {
			// Always away.
			if x > 0 {
				o.Position.Vx -= o.MovementSpeed
			} else {
				o.Position.Vx += o.MovementSpeed
			}
			if y > 0 {
				o.Position.Vy -= o.MovementSpeed
			} else {
				o.Position.Vy += o.MovementSpeed
			}
		}
		o.Position.Vx = keepWithinRange(o.Position.Vx, playerMinX, playerMaxX)
		o.Position.Vy = keepWithinRange(o.Position.Vy, playerMinY, playerMaxY)
	case staticCircler, switchingCircler:
		circle(o)
		o.Position.Vx = keepWithinRange(o.FirstVector.Vx+o.Radius*lpge.Rcos(o.Angle)>>12, tieFighterMinX, tieFighterMaxX)
		o.Position.Vy = keepWithinRange(o.FirstVector.Vy+o.Radius*lpge.Rsin(o.Angle)>>12, tieFighterMinY, tieFighterMaxY)
		if o.SubType == switchingCircler && o.LifeTime%o.SpecialTimeLimit == 0 {
			o.WhichWay = 1 - o.WhichWay
			o.SpecialTimeLimit = 30 + rand()%60
		}
	}
	moveWorldRelative(o)
	if o.CurrentHealth <= 0 {
		handleEnemysDeath(o)
	}
}

func handleAnInsectShip(o *lpge.Handler) {
	o.Coord.Coord.T[2] -= o.MovementSpeed
	o.Coord.Flg = 0
	if o.Coord.Coord.T[2] < -sectionLength {
		o.Alive = false
	}
	if o.CurrentHealth < 1 {
		handleEnemysDeath(o)
	}
}

// handleSpecialObject spins a pick-up and bounces it about the tunnel as it
// comes.
func handleSpecialObject(o *lpge.Handler) {
	if o.ActionState == holdStill {
		return
	}
	switch o.WhichWay {
	case clockwise:
		o.Sprite.Rotate += 4096
	case anticlockwise:
		o.Sprite.Rotate -= 4096
	}
	for o.Sprite.Rotate >= 4096*360 {
		o.Sprite.Rotate -= 4096 * 360
	}
	for o.Sprite.Rotate < 0 {
		o.Sprite.Rotate += 4096 * 360
	}
	if x := o.Position.Vx + o.Velocity.Vx; x < playerMinX || x > playerMaxX {
		o.Velocity.Vx = -o.Velocity.Vx
	}
	if y := o.Position.Vy + o.Velocity.Vy; y < playerMinY || y > playerMaxY {
		o.Velocity.Vy = -o.Velocity.Vy
	}
	moveWorldRelative(o)
	if o.Coord.Coord.T[2] < -sectionLength {
		o.Alive = false
	}
}

// handleRobot moves the robot round a wandering circle, back and forth
// along the tunnel, firing hard for half of each period and circling fast
// for the other.
func handleRobot(o *lpge.Handler) {
	if o.LifeTime%o.ThirdPeriod*2/o.ThirdPeriod == 0 {
		dir := yaroze.VECTOR{
			Vx: playersShip.Position.Vx - o.Position.Vx,
			Vy: playersShip.Position.Vy - o.Position.Vy,
			Vz: playersShip.Position.Vz - o.Position.Vz,
		}
		handleObjectContinuousFiring(o, &dir)
		o.AngleIncrement = robotCirclingSpeed
	} else {
		o.AngleIncrement = robotCirclingSpeed * 2
	}

	if o.LifeTime%o.SpecialTimeLimit == 0 {
		o.WhichWay = 1 - o.WhichWay
		switch rand() % 4 {
		case 0:
			o.SecondVector.Vx = 1
		case 1:
			o.SecondVector.Vx = -1
		case 2:
			o.SecondVector.Vy = 1
		case 3:
			o.SecondVector.Vy = -1
		}
	}

	o.Position.Vx = keepWithinRange(o.FirstVector.Vx+o.Radius*lpge.Rcos(o.Angle)>>12, robotMinX, robotMaxX)
	o.Position.Vy = keepWithinRange(o.FirstVector.Vy+o.Radius*lpge.Rsin(o.Angle)>>12, robotMinY, robotMaxY)
	circle(o)

	o.FirstVector.Vx = keepWithinRange(o.FirstVector.Vx+o.SecondVector.Vx, robotMinX, robotMaxX)
	o.FirstVector.Vy = keepWithinRange(o.FirstVector.Vy+o.SecondVector.Vy, robotMinY, robotMaxY)

	switch o.LifeTime % o.SecondPeriod * 4 / o.SecondPeriod {
	case 1:
		o.Position.Vz -= 5
	case 3:
		o.Position.Vz += 5
	}

	o.Coord.Coord.T = [3]int32{o.Position.Vx, o.Position.Vy, o.Position.Vz}
	o.Coord.Flg = 0
	if o.Coord.Coord.T[2] < -sectionLength {
		o.Alive = false
	}
	if o.CurrentHealth < 1 {
		handleEnemysDeath(o)
	}
}

// handleTieFighterWaves moves a wave of fighters together, as its type has
// it: switching all their directions now and then, darting all to one side,
// or both.
func handleTieFighterWaves() {
	var leader *lpge.Handler
	for i := range tieFighters {
		if tieFighters[i].Alive {
			leader = &tieFighters[i]
			break
		}
	}
	if leader == nil {
		return
	}
	mode := leader.SpecialMovement
	if mode == staticHelix {
		return
	}
	if mode == switchingHelix || mode == dartingSwitchingHelix {
		if leader.LifeTime%leader.SpecialTimeLimit == 0 {
			for i := range tieFighters {
				if tieFighters[i].Alive {
					tieFighters[i].WhichWay = 1 - tieFighters[i].WhichWay
				}
			}
			leader.SpecialTimeLimit = switchPeriod + rand()%switchPeriod
		}
		if mode == switchingHelix {
			return
		}
	}
	// Darting.
	if !dartOn && leader.LifeTime%leader.SpecialTimeLimit == 0 {
		dartOn = true
		dartDirection = rand() % 4
		dartFrameStart = frameNumber + 1
		dartDuration = 8 + rand()%8
		return
	}
	if !dartOn {
		return
	}
	if frameNumber == dartFrameStart+dartDuration {
		dartOn = false
		return
	}
	for i := range tieFighters {
		f := &tieFighters[i]
		if !f.Alive {
			continue
		}
		switch dartDirection {
		case 0:
			f.FirstVector.Vx -= 2
		case 1:
			f.FirstVector.Vx += 2
		case 2:
			f.FirstVector.Vy -= 2
		case 3:
			f.FirstVector.Vy += 2
		}
		f.Coord.Flg = 0
	}
}

// setCoordinates is the tutorial's UpdateObjectCoordinates2: rotation and
// position, both relative to the world.
func setCoordinates(rotation *yaroze.SVECTOR, position *yaroze.VECTOR, c *yaroze.GsCOORDINATE2) {
	yaroze.RotMatrix(rotation, &c.Coord)
	c.Coord.T = [3]int32{position.Vx, position.Vy, position.Vz}
	c.Flg = 0
}

// moveWorldRelative is the tutorial's UpdateObjectCoordinates4: a move by
// the object's velocity, which it keeps only if it has momentum, and no
// rotation.
func moveWorldRelative(o *lpge.Handler) {
	o.Position.Vx += o.Velocity.Vx
	o.Position.Vy += o.Velocity.Vy
	o.Position.Vz += o.Velocity.Vz
	if !o.MovementMomentumFlag {
		o.Velocity = yaroze.VECTOR{}
	}
	o.Coord.Coord.T = [3]int32{o.Position.Vx, o.Position.Vy, o.Position.Vz}
	o.Coord.Flg = 0
}

func handleEnemyClass() {
	if frameNumber%robotFrequency == 0 && !theRobot.Alive {
		createRobot()
	}
	if frameNumber%enemyWaveFrequency == 0 && frameNumber > 0 && numberOfEnemiesLeft() == 0 {
		switch rand() % numberEnemyClasses {
		case tieFighterClass:
			createWaveOfTieFighters()
		case singleAttackerClass:
			createSingleAttacker()
		case insectShipClass:
			createWaveOfInsectShips()
		}
	}
}

func numberOfEnemiesLeft() int {
	return countNumberOfLivingTypedObjects(tieFighter) +
		countNumberOfLivingTypedObjects(singleAttacker) +
		countNumberOfLivingTypedObjects(insectShip)
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

// firstDead returns the first object in a set that is not alive, or nil.
func firstDead(objs []lpge.Handler) *lpge.Handler {
	for i := range objs {
		if !objs[i].Alive {
			return &objs[i]
		}
	}
	return nil
}

// createWaveOfTieFighters sends a wave of fighters in a helix: one behind
// another, spaced round a circle.
func createWaveOfTieFighters() {
	waveType := rand() % numberWaveTypes
	whichWay := rand() % 2
	initialAngle := rand() % yaroze.ONE
	angleStep := yaroze.ONE / numberTieFightersPerWave
	startX := rand()%65 - 32
	startY := rand()%18 - 9

	var previous *lpge.Handler
	for i := range numberTieFightersPerWave {
		f := firstDead(tieFighters[:])
		if f == nil {
			return
		}
		f.Revive()
		f.SpecialMovement = waveType
		f.WhichWay = whichWay
		f.SpecialTimeLimit = switchPeriod + rand()%switchPeriod
		if previous == nil {
			f.Coord.Coord.T[2] = zDistance
		} else {
			f.Coord.Coord.T[2] = previous.Coord.Coord.T[2] + zSeparation
		}
		f.Angle = initialAngle + i*angleStep
		f.AngleIncrement = tieFighterAngularSpeed
		f.Radius = radius
		f.FirstVector = yaroze.VECTOR{Vx: startX, Vy: startY}
		f.Coord.Flg = 0
		previous = f
	}
}

func createSingleAttacker() {
	a := firstDead(singleAttackers[:])
	if a == nil {
		return
	}
	a.Revive()
	a.SubType = rand() % numberAttackerSubtypes
	a.LifeTime = 0
	a.Position = yaroze.VECTOR{Vz: 5 * squareSize}
	switch a.SubType {
	case randomJiggler:
		a.ActionState = rand() % 4
		a.SpecialTimeLimit = 10 + rand()%10
	case staticCircler, switchingCircler:
		a.WhichWay = rand() % 2
		a.Radius = radius
		if a.SubType == switchingCircler {
			a.SpecialTimeLimit = 30 + rand()%60
		}
		a.AngleIncrement = singleAttackerAngularSpeed
		a.Angle = rand() % yaroze.ONE
		startX := rand()%65 - 32
		startY := rand()%18 - 9
		a.FirstVector = yaroze.VECTOR{Vx: startX, Vy: startY}
	}
}

func createWaveOfInsectShips() {
	for i := range numberInsectShipsPerWave {
		s := firstDead(insectShips[:])
		if s == nil {
			return
		}
		s.Revive()
		startX := rand()%161 - 80
		startY := rand()%99 - 49
		s.Coord.Coord.T = [3]int32{startX, startY, insectInitialZ + i*insectZSeparation}
		s.Coord.Flg = 0
	}
}

func createRobot() {
	r := &theRobot
	r.Alive = true
	startX := rand()%17 - 8
	startY := rand()%17 - 8
	r.FirstVector.Vx, r.FirstVector.Vy = startX, startY
	r.WhichWay = rand() % 2
	r.Angle = rand() % yaroze.ONE
	r.AngleIncrement = robotCirclingSpeed
	r.Radius = radius
	r.MovementSpeed = robotMovementSpeed
	r.RotationSpeed = robotRotationSpeed
	r.SpecialTimeLimit = robotPeriod
	r.SecondPeriod = robotZPeriod
	r.ThirdPeriod = robotFiringPeriod
	r.Rotate = yaroze.SVECTOR{}
	r.Position = yaroze.VECTOR{Vz: 3 * squareSize / 2}
}

func handleObjectContinuousFiring(o *lpge.Handler, dir *yaroze.VECTOR) {
	if o.FramesSinceLastFire > o.FiringRate {
		objectFiresAShot(o, dir)
		o.FramesSinceLastFire = 0
	}
}

// objectFiresAShot sends a free bullet or tetrahedron from an object
// towards dir.
func objectFiresAShot(o *lpge.Handler, dir *yaroze.VECTOR) {
	d := *dir
	normalizeVector(&d)

	var p *lpge.Handler
	switch o.ShotType {
	case lpge.FireSprite:
		p = firstDead(theBullets[:])
	case lpge.TetrahedronModel:
		p = firstDead(tetrahedrons[:])
	}
	if p == nil {
		return
	}
	p.Alive = true
	p.LifeTime = 0
	p.Allegiance = o.Allegiance
	p.Velocity = yaroze.VECTOR{
		Vx: o.ShotSpeed * d.Vx / yaroze.ONE,
		Vy: o.ShotSpeed * d.Vy / yaroze.ONE,
		Vz: o.ShotSpeed * d.Vz / yaroze.ONE,
	}
	p.MovementMomentumFlag = true
	t := o.Coord.Coord.T
	p.Position = yaroze.VECTOR{Vx: t[0], Vy: t[1], Vz: t[2]}
	p.Coord.Coord.T = t
	p.Coord.Flg = 0
	turnOnSound(1, 0, 60, 1, 10, 48, 48)
}

// normalizeVector scales a vector to a length of ONE, each component
// truncated towards zero as the original's floating point does: the
// magnitude of v·ONE/|v| is the integer square root of v²·ONE²/|v|².
func normalizeVector(v *yaroze.VECTOR) {
	sum := uint64(int64(v.Vx)*int64(v.Vx) + int64(v.Vy)*int64(v.Vy) + int64(v.Vz)*int64(v.Vz))
	if sum == 0 {
		return
	}
	scale := func(c int32) int32 {
		m := int32(isqrt(uint64(int64(c)*int64(c)) << 24 / sum))
		if c < 0 {
			return -m
		}
		return m
	}
	v.Vx, v.Vy, v.Vz = scale(v.Vx), scale(v.Vy), scale(v.Vz)
}

// isqrt is the integer square root, rounded down.
func isqrt(n uint64) uint64 {
	var r uint64
	bit := uint64(1) << 62
	for bit > n {
		bit >>= 2
	}
	for bit != 0 {
		if n >= r+bit {
			n -= r + bit
			r = r>>1 + bit
		} else {
			r >>= 1
		}
		bit >>= 2
	}
	return r
}

func precompileCircularPositions() {
	numberPositions = yaroze.ONE / angularSpeed
	for i := range numberPositions {
		angle := i * angularSpeed
		circularXs[i] = radius * lpge.Rcos(angle) >> 12
		circularYs[i] = radius * lpge.Rsin(angle) >> 12
	}
}
