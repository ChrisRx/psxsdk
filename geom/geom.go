// Package geom is the geometry the console works in: points and rectangles
// on the screen, vectors, rotations and matrices in the world, angles, and
// the fixed point numbers the GTE, its geometry coprocessor, takes.
package geom

// A Point is a position on the screen.
type Point struct {
	X, Y int16
}

// A Rect is a rectangle on the screen or in video memory.
type Rect struct {
	X, Y int
	W, H int
}

// Overlaps says whether the two rectangles share any pixel.
func (r Rect) Overlaps(o Rect) bool {
	return r.X < o.X+o.W && o.X < r.X+r.W && r.Y < o.Y+o.H && o.Y < r.Y+r.H
}

// Clamp returns x and y moved as little as they need to be for a w by h
// box at them to lie inside the rectangle.
func (r Rect) Clamp(x, y, w, h int) (int, int) {
	return min(max(x, r.X), r.X+r.W-w), min(max(y, r.Y), r.Y+r.H-h)
}

// A Fixed is a number with twelve bits of fraction: One is 1.0.
type Fixed int16

// One is 1.0 as a Fixed.
const One Fixed = 4096

// An Angle is a turn in units of a 4096th of a degree.
type Angle int32

// Degrees returns the angle of d degrees.
func Degrees(d int) Angle { return Angle(d) << 12 }

// A Vector is a position in the world, in a model's own units.
type Vector struct {
	X, Y, Z int
}

// A Rotation is a turn about each axis, with 4096 a whole turn, applied
// about X, then Y, then Z.
type Rotation struct {
	X, Y, Z int
}

// A Matrix is a rotation and a translation as the GTE takes them: the
// rotation M in 4.12, with the rotation about Y that turns a camera
// behind a figure having the sine and cosine of the angle at M[0][2]
// and M[0][0], and the translation T. An object's Transform fills one,
// and Rotate sets its rotation.
type Matrix struct {
	M [3][3]int16
	_ [2]byte
	T [3]int32
}

// Rotate sets the matrix's rotation to r, each a turn about an axis in
// 4096ths, and clears nothing else.
func (m *Matrix) Rotate(r Rotation) { rotateMatrix(m, r) }

// Apply rotates v by the matrix's rotation. It leaves out the
// translation, as the library's ApplyMatrixLV does.
func (m *Matrix) Apply(v Vector) Vector { return applyMatrix(m, v) }
