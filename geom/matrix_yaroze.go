//go:build yaroze

package geom

// The library's functions Matrix calls, declared here as the yaroze
// package's bindings declare them: a bodyless function carrying //export
// imports the C symbol, which the resident library or the clean one
// linked into the program defines. Declaring them here rather than calling
// the yaroze package keeps this package free of it, which imports this
// one.

//export RotMatrix
func rotMatrix(r *svector, m *Matrix) *Matrix

//export ApplyMatrixLV
func applyMatrixLV(m *Matrix, v0, v1 *lvector) *lvector

// svector and lvector are the library's SVECTOR and VECTOR: a rotation and
// a vector as it takes them, 16 and 32 bits a component.
type (
	svector struct{ x, y, z, _ int16 }
	lvector struct{ x, y, z, _ int32 }
)

func rotateMatrix(m *Matrix, r Rotation) {
	v := svector{
		x: int16(r.X),
		y: int16(r.Y),
		z: int16(r.Z),
	}
	rotMatrix(&v, m)
}

func applyMatrix(m *Matrix, v Vector) Vector {
	in := lvector{
		x: int32(v.X),
		y: int32(v.Y),
		z: int32(v.Z),
	}
	var out lvector
	applyMatrixLV(m, &in, &out)
	return Vector{
		X: int(out.x),
		Y: int(out.y),
		Z: int(out.z),
	}
}
