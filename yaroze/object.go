package yaroze

// An Object is a model drawn somewhere: a GsDOBJ2 with the coordinate
// system it is drawn in, which every drawn model needs and which the
// library keeps as a pointer from the one to the other. Init ties the two
// together, so the object must not be copied afterwards; it is a
// package-level variable, like everything a frame draws.
//
//	var cube yaroze.Object
//	cube.Init(yaroze.WORLD)
//	cube.Link(model, 0)
//	...
//	cube.Coord.Coord = rotation
//	cube.Coord.Flg = 0
//	cube.Coord.Ls(&ls)
//	yaroze.GsSetLsMatrix(&ls)
//	cube.Sort(ot, 14-otLength)
type Object struct {
	GsDOBJ2
	Coord GsCOORDINATE2
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
