package yaroze

// An Object is a model drawn somewhere: a GsDOBJ2 with the coordinate
// system it is drawn in, which every drawn model needs and which the
// library keeps as a pointer from the one to the other, and where the
// model is in that system, which Draw makes the coordinate system's matrix
// each frame. Init ties the two together, so the object must not be copied
// afterwards; it is a package-level variable, like everything a frame
// draws.
//
//	var cube yaroze.Object
//	cube.Init(yaroze.WORLD)
//	cube.Link(model, 0)
//	...
//	cube.Rotation.Vy += 16
//	cube.Draw(ot)
type Object struct {
	GsDOBJ2
	Coord GsCOORDINATE2

	// Position is where the object is in its parent's coordinates.
	Position VECTOR

	// Rotation is how it is turned about each axis, with 4096 a whole
	// turn, applied in the order RotMatrix applies them.
	Rotation SVECTOR

	// Scale is its size along each axis, with 4096 as 1.0. The zero value
	// leaves it unscaled.
	Scale VECTOR
}

// Init sets the object's coordinate system to the identity, with super as
// its parent, or WORLD, and makes it the system the object is drawn in.
func (o *Object) Init(super *GsCOORDINATE2) {
	o.Coord.Init(super)
	o.Coord2 = &o.Coord
}

// Link ties the object to the nth model in tmd, which must have been
// mapped.
//
//go:inline
func (o *Object) Link(tmd TMD, n int32) { tmd.Link(&o.GsDOBJ2, n) }

// Draw makes the coordinate system's matrix from Position, Rotation and
// Scale, and sorts the object lit, as GsDOBJ2's Draw does. A program that
// builds the matrix itself sets it with Coord.SetMatrix and calls
// o.GsDOBJ2.Draw instead, which leaves it alone.
func (o *Object) Draw(ot *GsOT) {
	m := &o.Coord.Coord
	RotMatrix(&o.Rotation, m)
	if o.Scale != (VECTOR{}) {
		ScaleMatrix(m, &o.Scale)
	}
	m.T = [3]int32{o.Position.Vx, o.Position.Vy, o.Position.Vz}
	o.Coord.Flg = 0
	o.GsDOBJ2.Draw(ot)
}
