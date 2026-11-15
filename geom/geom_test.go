package geom

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestOverlaps(t *testing.T) {
	a := Rect{X: 10, Y: 10, W: 20, H: 20}
	assert.Equal(t, true, a.Overlaps(Rect{X: 29, Y: 29, W: 5, H: 5}), "corner touching inside")
	assert.Equal(t, false, a.Overlaps(Rect{X: 30, Y: 10, W: 5, H: 5}), "edge to edge")
	assert.Equal(t, false, a.Overlaps(Rect{X: 0, Y: 0, W: 10, H: 10}), "apart")
	assert.Equal(t, true, a.Overlaps(Rect{X: 0, Y: 0, W: 100, H: 100}), "inside")
}

func TestClamp(t *testing.T) {
	x, y := (Rect{X: 0, Y: 0, W: 50, H: 140}).Clamp(-5, 200, 40, 40)
	assert.Equal(t, [2]int{0, 100}, [2]int{x, y}, "clamped into the rectangle")
}

func TestDegrees(t *testing.T) {
	assert.Equal(t, Angle(90<<12), Degrees(90))
}

// TestMatrixStub checks that the GTE's calls fail at once off the console.
func TestMatrixStub(t *testing.T) {
	var m Matrix
	assert.Panic(t, stub("Matrix.Rotate"), func() { m.Rotate(Rotation{}) })
	assert.Panic(t, stub("Matrix.Apply"), func() { m.Apply(Vector{}) })
}
