package yaroze

import (
	"unsafe"

	"go.chrisrx.dev/psx/geom"
)

// 3D: models from TMD files, placed as objects in the world, seen by a
// camera under lights, and drawn on a frame like anything else.

// A Model is a TMD file in memory, mapped by LoadTMD: the models an
// Object can be linked to, numbered from 0.
type Model []uint32

// LoadTMD takes the TMD file in words, as a disc file is read, and maps
// it in place, which the file records, so a file mapped twice is left as
// it is.
func LoadTMD(words []uint32) Model {
	mapModel(words)
	return Model(words)
}

// Objects is how many models the file holds.
func (m Model) Objects() int { return int(m[2]) }

// A Vertex is a model's vertex or normal, 16 bits a component, as the
// GTE takes it.
type Vertex struct {
	X, Y, Z int16
	_       int16
}

// Vertices returns the nth model's vertices, in the file's own memory:
// a program that moves them moves the model. The object table holds
// seven words a model: the vertices' address and count, the normals',
// the primitives', and the scale.
func (m Model) Vertices(n int) []Vertex { return modelVectors(m, 3+7*n) }

// Normals returns the nth model's normals, as Vertices does.
func (m Model) Normals(n int) []Vertex { return modelVectors(m, 5+7*n) }

// Vertices returns the words as vertices, for a file of them, such as
// the differences a model is blended by: two words a vertex.
func Vertices(words []uint32) []Vertex {
	if len(words) < 2 {
		return nil
	}
	return unsafe.Slice((*Vertex)(unsafe.Pointer(&words[0])), len(words)/2)
}

// Blend adds each difference to its vertex, scaled by p, One for the
// whole of it, through the GTE: the vertex animation of a model blended
// towards another shape, as a set of differences describes it. The
// shorter slice says how many.
func Blend(vertices, diffs []Vertex, p geom.Fixed) {
	n := min(len(vertices), len(diffs))
	if n > 0 {
		blend(&vertices[0], &diffs[0], n, p)
	}
}

// An Object is a model drawn somewhere: where it is in its parent's
// coordinates, how it is turned and how big, which Draw makes its
// matrix from each frame. Init ties an object to its parent, so an
// object must not be copied afterwards, which go vet reports as it does a
// copied lock. It is made once, like everything a frame draws: a
// package-level variable, or a variable of main, which the compiler
// places as it would one.
//
//	var cube yaroze.Object
//
//	cube.Init(nil)
//	cube.Link(model, 0)
//	for frame := range screen.Frames() {
//		cube.Rotation.Y += 16
//		cube.Draw(frame)
//	}
type Object struct {
	_ noCopy

	// Position is where the object is in its parent's coordinates, the
	// world's for one with no parent.
	Position geom.Vector

	// Rotation is how it is turned.
	Rotation geom.Rotation

	// Scale is its size along each axis, with 4096 as 1.0. The zero
	// value leaves it unscaled.
	Scale geom.Vector

	obj   objectImpl
	coord coordImpl
}

// Init makes the object's coordinate system the identity inside its
// parent's, or the world's for nil, and the system it is drawn in.
func (o *Object) Init(parent *Object) {
	var super *coordImpl
	if parent != nil {
		super = &parent.coord
	}
	initCoord(&o.coord, super)
	o.obj.coord = &o.coord
}

// Link ties the object to the nth model in m.
func (o *Object) Link(m Model, n int) { linkObject(&o.obj, m, n) }

// Draw puts the object on the frame: its matrix made from Position,
// Rotation and Scale, the lights turned into its coordinates, and its
// model sorted by depth among what else the frame draws. It is
// Transform, SetLightMatrix with the matrix to the world, SetScreenMatrix
// with the one to the screen, and Sort; a program that lights or places
// a model another way calls those itself.
func (o *Object) Draw(f *Frame) {
	var world, screen geom.Matrix
	o.Transform(&world, &screen)
	SetLightMatrix(&world)
	SetScreenMatrix(&screen)
	o.Sort(f)
}

// Transform makes the object's matrix from Position, Rotation and
// Scale, and works out its matrices to the world and to the screen,
// through its parents, into world and screen.
func (o *Object) Transform(world, screen *geom.Matrix) { objectMatrices(o, world, screen) }

// Sort sorts the object's model into the frame with the lights and the
// screen matrix as they are set.
func (o *Object) Sort(f *Frame) { sortObject(o, f) }

// SetLightMatrix turns the lights by m for the models drawn after it,
// which Draw does with the model's matrix to the world.
func SetLightMatrix(m *geom.Matrix) { setLightMatrix(m) }

// SetScreenMatrix sets the matrix the models drawn after it are
// projected by, which Draw does with the model's matrix to the screen.
func SetScreenMatrix(m *geom.Matrix) { setScreenMatrix(m) }

// The library's own layouts, which an Object holds: a coordinate system
// and a drawn object.
type (
	coordImpl struct {
		flg          uint32
		coord, workm geom.Matrix
		param        uintptr
		super, sub   *coordImpl
	}
	objectImpl struct {
		attr  uint32
		coord *coordImpl
		tmd   *uint32
		id    uint32
	}
)

// A Camera is where the world is seen from: the eye at Eye looking at
// Target, rolled by Roll about the line of sight, with the screen
// Distance away from the eye, which is the projection. Set makes it the
// camera the frames are drawn with.
type Camera struct {
	Eye, Target geom.Vector
	Roll        int
	Distance    int
}

// Set makes the camera the one the frames are drawn with.
func (c *Camera) Set() { setCamera(c) }

// A Light is a directional light: light coming the way Direction points,
// of a color. The library has three, numbered 0 to 2, which SetLight
// sets.
type Light struct {
	Direction geom.Vector
	Color     Color
}

// SetLight sets light n, 0 to 2. A direction of no length is refused,
// and the light stays as it was.
func SetLight(n int, l Light) { setLight(n, l) }

// SetAmbient sets the light from everywhere, each component a geom.Fixed of
// One for full.
func SetAmbient(r, g, b geom.Fixed) { setAmbient(r, g, b) }

// A Fog fades lit faces into a color with their distance: nothing of it
// nearer than Near, all of it at Far, and a Far of 0 for a fog that
// thickens for ever, in the units the camera's Distance is in. The
// library's documentation suggests a Near of a fifth of Far. The fade is
// of the face's lit color, which the GPU multiplies a texture by, so a
// textured face fades towards its texture times the fog's color: white
// fog brightens it and black fog darkens it, and a colored fog takes
// out of it whatever the texture lacks.
type Fog struct {
	Near, Far int
	Color     Color
}

// SetFog turns the fog on for the camera last set, or off for the zero
// Fog. Models are then lit with it until it is turned off.
func SetFog(f Fog) {
	if f == (Fog{}) {
		setLightMode(0)
		return
	}
	dqa, dqb := fogParams(f.Near, f.Far, projection)
	setFog(dqa, dqb, f.Color)
	setLightMode(1)
}

// projection is the camera's Distance last set, which the fog's
// coefficients depend on.
var projection = 320

// fogParams is the GTE's depth cue for a fog from near to far with the
// screen distance away from the eye: the cue is b - a/z for a depth z,
// 0 at near and 1 at far, so b is far/(far-near) and a is near times
// that, or 1 and near for a far of 0; the GTE takes b in 8.24 and a as
// its scale in 8.8 over the projection's own division by the depth.
func fogParams(near, far, distance int) (dqa int16, dqb int32) {
	n, f, d := int64(near), int64(far), int64(distance)
	if far > near {
		return int16(-n * f * 256 / ((f - n) * d)), int32(f << 24 / (f - n))
	}
	return int16(-n * 256 / d), 1 << 24
}
