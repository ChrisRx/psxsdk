package main

import (
	"go.chrisrx.dev/psx/examples/internal/lpge"
	"go.chrisrx.dev/psx/yaroze"
)

// The tutorial's own object.c, older than the tunnel's: objects scale
// along each axis, and have no sprites.

const (
	none     = 1001 // unset; as a super coordinate system, the object's own
	theWorld = 1002

	maxObjects = 128
)

type object struct {
	id    int32
	alive bool
	typ   int32
	which int32

	handler      yaroze.GsDOBJ2
	modelAddress uintptr
	whichModel   int32

	scalingFlag            bool
	scaleX, scaleY, scaleZ int32

	position         yaroze.VECTOR
	velocity         yaroze.VECTOR
	movementMomentum bool

	rotate           yaroze.SVECTOR
	twist            yaroze.VECTOR
	rotationMomentum bool

	coord                   yaroze.GsCOORDINATE2
	superCoordinateObjectID int32

	inOrbit        bool
	centralBodyID  int32
	angle          int32
	angleIncrement int32
	whichWay       int32
	radius         int32
	firstVector    yaroze.VECTOR // these two lie on the plane of the orbit
	secondVector   yaroze.VECTOR

	specialMovement int32
}

// objects is ObjectArray, every object but the player's shuttle, by ID.
var objects [maxObjects]*object

func initialiseObjectClass() {
	for i := range objects {
		objects[i] = nil
	}
}

// findNextLowestObjectID is the ID of the living object below id, going
// round to the top.
func findNextLowestObjectID(id int32) int32 {
	for {
		id--
		if id < 0 {
			id = maxObjects - 1
		}
		if o := objects[id]; o != nil && o.alive {
			return id
		}
	}
}

// findNextHighestObjectID is the ID of the living object above id, going
// round to the bottom.
func findNextHighestObjectID(id int32) int32 {
	for {
		id++
		if id >= maxObjects {
			id = 0
		}
		if o := objects[id]; o != nil && o.alive {
			return id
		}
	}
}

func register(o *object) {
	for i := range objects {
		if objects[i] == nil {
			objects[i] = o
			o.id = int32(i)
			return
		}
	}
}

func linkAllObjectsToModels() {
	for _, o := range &objects {
		if o != nil && o.alive && o.modelAddress != 0 {
			lpge.LinkObjectHandlerToTmdObject(&o.handler, o.whichModel, o.modelAddress)
		}
	}
}

func linkAllObjectsToTheirCoordinateSystems() {
	for _, o := range &objects {
		if o == nil || !o.alive {
			continue
		}
		switch o.superCoordinateObjectID {
		case none:
			o.handler.Coord2 = &o.coord
		case theWorld:
			o.handler.Coord2 = nil
		default:
			o.handler.Coord2 = &objects[o.superCoordinateObjectID].coord
		}
	}
}

func (o *object) bringToLife(typ int32, modelAddress uintptr, whichModel int32, super int32) {
	o.alive = true
	o.typ = typ
	o.modelAddress = modelAddress
	o.whichModel = whichModel
	o.superCoordinateObjectID = super
}

// init is InitSingleObject. It leaves the GsDOBJ2 as it was, attributes
// and all.
func (o *object) init() {
	o.id, o.alive, o.typ, o.which = none, false, none, none
	o.modelAddress, o.whichModel = 0, none
	o.scalingFlag = false
	o.scaleX, o.scaleY, o.scaleZ = 1, 1, 1
	o.position, o.velocity = yaroze.VECTOR{}, yaroze.VECTOR{}
	o.movementMomentum = false
	o.rotate, o.twist = yaroze.SVECTOR{}, yaroze.VECTOR{}
	o.rotationMomentum = false
	yaroze.GsInitCoordinate2(nil, &o.coord)
	o.superCoordinateObjectID = none
	o.inOrbit = false
	o.centralBodyID = none
	o.angle, o.angleIncrement = 0, 0
	o.whichWay = none
	o.radius = 0
	o.firstVector, o.secondVector = yaroze.VECTOR{}, yaroze.VECTOR{}
	o.specialMovement = none
}

// highlight marks the selected object by turning its lighting off.
func (o *object) highlight()   { o.handler.Attribute |= yaroze.GsLOFF }
func (o *object) unhighlight() { o.handler.Attribute &^= yaroze.GsLOFF }

func (o *object) setScaling(x, y, z int32) {
	o.scalingFlag = true
	o.scaleX, o.scaleY, o.scaleZ = x, y, z
}

// sortSize scales the object's coordinate system by its scaling.
func (o *object) sortSize() {
	if !o.scalingFlag {
		return
	}
	v := yaroze.VECTOR{Vx: o.scaleX, Vy: o.scaleY, Vz: o.scaleZ}
	yaroze.ScaleMatrix(&o.coord.Coord, &v)
	o.coord.Flg = 0
}

// Kinds of sphere, and the ships.
const (
	sun = iota
	planet
	moon
	ship
)

const (
	planetOrbitSize = 400
	moonOrbitSize   = 250

	maxPlanets         = 3
	maxMoons           = 9
	maxMoonsPerPlanet  = 3
	maxShips           = 24
	firstSimplePath    = 0 // a special movement: round a square
	halfRectangle      = 1000
	shipSpeed          = 10
	initialMoveSpeed   = 150
	initialRotateSpeed = 6
)

var (
	playersShuttle object
	theSun         object
	thePlanets     [maxPlanets]object
	theMoons       [maxMoons]object
	theShips       [maxShips]object

	movementSpeed, rotationSpeed, firingRate int32

	// globalMatrix is the shuttle's orientation, turned a little at a
	// time by its twist.
	globalMatrix = yaroze.MATRIX{M: [3][3]int16{{yaroze.ONE, 0, 0}, {0, yaroze.ONE, 0}, {0, 0, yaroze.ONE}}}
)

func initialiseObjects() {
	initialiseObjectClass()
	playersShuttle.init()
	theSun.init()
	for i := range thePlanets {
		thePlanets[i].init()
	}
	for i := range theMoons {
		theMoons[i].init()
	}
	for i := range theShips {
		theShips[i].init()
	}
	setUpSolarSystem()
	linkAllObjectsToModels()
	linkAllObjectsToTheirCoordinateSystems()

	// The player's shuttle is not in the object array, and is drawn by
	// itself.
	lpge.LinkObjectHandlerToTmdObject(&playersShuttle.handler, 0, shipModelAddr)
	playersShuttle.handler.Coord2 = &playersShuttle.coord
}

// setUpSolarSystem puts a sun at the middle, three planets round it in
// orbits at random angles, three moons round each planet, four ships
// going round a square through the sun, and a shuttle at the middle.
func setUpSolarSystem() {
	movementSpeed = initialMoveSpeed
	rotationSpeed = initialRotateSpeed
	firingRate = 5

	// Away from the solar system, to see where everything is.
	playersShuttle.position.Vz = -12000

	theSun.bringToLife(sun, sphereModelAddr, 3, none)
	theSun.setScaling(2*yaroze.ONE, 2*yaroze.ONE, 2*yaroze.ONE)
	register(&theSun)

	for i := range int32(maxPlanets) {
		p := &thePlanets[i]
		p.bringToLife(planet, sphereModelAddr, 5, none)
		var a1, a2 yaroze.VECTOR
		randomVector(&a1)
		anyUnitPerpendicularVector(&a1, &a2)
		putObjectIntoOrbit(p, theSun.id, yaroze.ONE/maxPlanets*i, yaroze.ONE/720, (i+1)*planetOrbitSize+300, randomOrientation(), &a1, &a2)
		p.position.Vx += (i + 1) * planetOrbitSize
		const scale = yaroze.ONE * 14 / 10 // 1.4 * ONE, truncated
		p.setScaling(scale, scale, scale)
		register(p)

		for j := range int32(maxMoonsPerPlanet) {
			m := &theMoons[i*maxPlanets+j]
			m.bringToLife(moon, sphereModelAddr, 7+j%3, none)
			randomVector(&a1)
			anyUnitPerpendicularVector(&a1, &a2)
			putObjectIntoOrbit(m, p.id, yaroze.ONE/maxMoons*j, yaroze.ONE/180, (j+1)*moonOrbitSize+100, randomOrientation(), &a1, &a2)
			m.position.Vx += (j + 1) * moonOrbitSize
			register(m)
		}
	}

	for i := range 4 {
		s := &theShips[i]
		s.bringToLife(ship, shipModelAddr, 0, none)
		s.specialMovement = firstSimplePath
		s.movementMomentum = true
		s.setScaling(4*yaroze.ONE, 4*yaroze.ONE, 4*yaroze.ONE)
		switch i {
		case 0:
			s.position.Vy = halfRectangle
			s.velocity.Vz = -shipSpeed
		case 1:
			s.position.Vz = -halfRectangle
			s.velocity.Vy = -shipSpeed
			s.rotate.Vx = 3 * yaroze.ONE / 4
		case 2:
			s.position.Vy = -halfRectangle
			s.velocity.Vz = shipSpeed
			s.rotate.Vx = yaroze.ONE / 2
		case 3:
			s.position.Vz = halfRectangle
			s.velocity.Vy = shipSpeed
			s.rotate.Vx = yaroze.ONE / 4
		}
		register(s)
	}

	theShips[4].bringToLife(ship, shuttleModelAddr, 0, none)
	theShips[4].position = yaroze.VECTOR{}
	register(&theShips[4])
}

func putObjectIntoOrbit(o *object, centralBodyID, initialAngle, angleIncrement, radius, whichWay int32, first, second *yaroze.VECTOR) {
	o.inOrbit = true
	o.centralBodyID = centralBodyID
	o.angle = initialAngle
	o.angleIncrement = angleIncrement
	o.radius = radius
	o.whichWay = whichWay
	o.firstVector = *first
	o.secondVector = *second
}

// Ways round a circle.
const (
	clockwise     = 1002
	anticlockwise = 1003
)

func randomOrientation() int32 {
	if rand()%2 == 0 {
		return clockwise
	}
	return anticlockwise
}

func rand() int32 { return yaroze.Rand() }

// dealWithSpatialMovement moves every object in orbit round its central
// body, and every one with a special movement along it.
func dealWithSpatialMovement() {
	for _, o := range &objects {
		if o == nil || !o.alive {
			continue
		}
		if o.inOrbit {
			handleOrbitalMovement(o)
		} else if o.specialMovement != none {
			handleSpecialMovement(o)
		}
	}
}

// handleOrbitalMovement places an object round its central body: the
// orbit is a circle in the plane of its two vectors, which are ONE long,
// rather than a coordinate system under the body's, so that every body can
// spin without turning those that orbit it.
func handleOrbitalMovement(o *object) {
	switch o.whichWay {
	case clockwise:
		o.angle += o.angleIncrement
	case anticlockwise:
		o.angle -= o.angleIncrement
	}
	px := o.radius * lpge.Rcos(o.angle) >> 12
	py := o.radius * lpge.Rsin(o.angle) >> 12
	c := objects[o.centralBodyID]
	o.position.Vx = c.position.Vx + (px*o.firstVector.Vx+py*o.secondVector.Vx)>>12
	o.position.Vy = c.position.Vy + (px*o.firstVector.Vy+py*o.secondVector.Vy)>>12
	o.position.Vz = c.position.Vz + (px*o.firstVector.Vz+py*o.secondVector.Vz)>>12
}

// handleSpecialMovement turns a ship round each corner of its square: it
// goes round in the y-z plane, turning a quarter at each corner.
func handleSpecialMovement(o *object) {
	if o.specialMovement != firstSimplePath {
		return
	}
	turn := func(vy, vz int32) {
		o.rotate.Vx -= yaroze.ONE / 4
		o.velocity = yaroze.VECTOR{Vy: vy, Vz: vz}
	}
	switch {
	case o.position.Vy == halfRectangle && o.position.Vz == halfRectangle:
		turn(0, -shipSpeed)
	case o.position.Vy == halfRectangle && o.position.Vz == -halfRectangle:
		turn(-shipSpeed, 0)
	case o.position.Vy == -halfRectangle && o.position.Vz == halfRectangle:
		turn(shipSpeed, 0)
	case o.position.Vy == -halfRectangle && o.position.Vz == -halfRectangle:
		turn(0, shipSpeed)
	}
}

// handleAllObjects moves and turns every object, and the shuttle, which
// moves and turns relative to itself.
func handleAllObjects() {
	for _, o := range &objects {
		if o == nil || !o.alive {
			continue
		}
		updateCoordinateSystem(&o.rotate, &o.position, &o.coord)
		o.sortSize()
		o.rotate.Vx += int16(o.twist.Vx)
		o.rotate.Vy += int16(o.twist.Vy)
		o.rotate.Vz += int16(o.twist.Vz)
		o.position.Vx += o.velocity.Vx
		o.position.Vy += o.velocity.Vy
		o.position.Vz += o.velocity.Vz
		if !o.movementMomentum {
			o.velocity = yaroze.VECTOR{}
		}
		if !o.rotationMomentum {
			o.twist = yaroze.VECTOR{}
		}
	}

	s := &playersShuttle
	updateCoordinateSystem4(&s.twist, &s.position, &s.velocity, &s.coord)
	if !s.movementMomentum {
		s.velocity = yaroze.VECTOR{}
	}
	if !s.rotationMomentum {
		s.twist = yaroze.VECTOR{}
	}
}

// updateCoordinateSystem sets a coordinate system from a rotation and a
// position, both relative to the world.
func updateCoordinateSystem(rotation *yaroze.SVECTOR, position *yaroze.VECTOR, c *yaroze.GsCOORDINATE2) {
	yaroze.RotMatrix(rotation, &c.Coord)
	c.Coord.T = [3]int32{position.Vx, position.Vy, position.Vz}
	c.Flg = 0
}

// updateCoordinateSystem4 moves and turns the shuttle relative to itself.
// As the original has it, the velocity is applied before the turn, through
// the orientation it had.
func updateCoordinateSystem4(twist, position, velocity *yaroze.VECTOR, c *yaroze.GsCOORDINATE2) {
	var real yaroze.VECTOR
	yaroze.ApplyMatrixLV(&globalMatrix, velocity, &real)
	position.Vx += real.Vx
	position.Vy += real.Vy
	position.Vz += real.Vz

	var xm, ym, zm, tmp yaroze.MATRIX
	xv := yaroze.SVECTOR{Vx: int16(twist.Vx)}
	yv := yaroze.SVECTOR{Vy: int16(twist.Vy)}
	zv := yaroze.SVECTOR{Vz: int16(twist.Vz)}
	yaroze.RotMatrix(&xv, &xm)
	yaroze.RotMatrix(&yv, &ym)
	yaroze.RotMatrix(&zv, &zm)
	yaroze.MulMatrix0(&globalMatrix, &xm, &tmp)
	yaroze.MulMatrix0(&tmp, &ym, &globalMatrix)
	yaroze.MulMatrix0(&globalMatrix, &zm, &tmp)
	globalMatrix = tmp

	c.Coord = globalMatrix
	c.Coord.T = [3]int32{position.Vx, position.Vy, position.Vz}
	c.Flg = 0
}

// Vector routines, from vector.c. The original works the lengths out in
// double precision through pow; here they are integer square roots, which
// come to the same, in 64 bits where the original's ints can overflow.

// randomVector sets v to a random direction, ONE long.
func randomVector(v *yaroze.VECTOR) {
	v.Vx = rand() % yaroze.ONE
	v.Vy = rand() % yaroze.ONE
	v.Vz = rand() % yaroze.ONE
	scaleVectorToUnit(v)
}

func scaleVectorToUnit(v *yaroze.VECTOR) {
	x, y, z := int64(v.Vx), int64(v.Vy), int64(v.Vz)
	length := int64(isqrt(uint64(x*x + y*y + z*z)))
	if length == 0 {
		return
	}
	v.Vx = int32(x * yaroze.ONE / length)
	v.Vy = int32(y * yaroze.ONE / length)
	v.Vz = int32(z * yaroze.ONE / length)
}

// anyUnitPerpendicularVector sets out to a direction at right angles to
// in, ONE long: two components at random, and the third to suit.
func anyUnitPerpendicularVector(in, out *yaroze.VECTOR) {
	switch rand() % 3 {
	case 0:
		if in.Vz == 0 {
			*out = yaroze.VECTOR{Vx: yaroze.ONE}
			return
		}
		out.Vx = rand() % yaroze.ONE
		out.Vy = rand() % yaroze.ONE
		out.Vz = (-in.Vx*out.Vx + -in.Vy*out.Vy) / in.Vz
	case 1:
		if in.Vy == 0 {
			*out = yaroze.VECTOR{Vy: yaroze.ONE}
			return
		}
		out.Vx = rand() % yaroze.ONE
		out.Vz = rand() % yaroze.ONE
		out.Vy = (-in.Vx*out.Vx + -in.Vz*out.Vz) / in.Vy
	case 2:
		if in.Vx == 0 {
			*out = yaroze.VECTOR{Vz: yaroze.ONE}
			return
		}
		out.Vz = rand() % yaroze.ONE
		out.Vy = rand() % yaroze.ONE
		out.Vx = (-in.Vz*out.Vz + -in.Vy*out.Vy) / in.Vx
	}
	scaleVectorToUnit(out)
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

// findObjectSpeed is, as the original works it out, the object's distance
// from the origin rather than its speed.
func findObjectSpeed(o *object) int32 {
	x, y, z := int64(o.position.Vx), int64(o.position.Vy), int64(o.position.Vz)
	return int32(isqrt(uint64(x*x + y*y + z*z)))
}
