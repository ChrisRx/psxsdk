//go:build !yaroze

package geom

// Every build that is not for the console has no GTE: Matrix's methods
// panic there, as the yaroze package's calls on the library do.

func rotateMatrix(m *Matrix, r Rotation)     { panic(stub("Matrix.Rotate")) }
func applyMatrix(m *Matrix, v Vector) Vector { panic(stub("Matrix.Apply")) }

// stub is the message a call panics with: what was called, and that it
// runs only on the console.
func stub(name string) string {
	return "geom." + name + ": the GTE is only on the console; build for the playstation-yaroze target"
}
