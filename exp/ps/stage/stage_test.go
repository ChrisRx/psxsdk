package stage

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

// A terrace: level 2 on the left, level 0 on the right, with a smooth
// step of 1 between on the middle row only, so that the top row has a
// cliff and the middle row a slope.
var terrace = Map{
	Cells: []string{
		"....",
		"....",
		"#..~",
	},
	Heights: []string{
		"2200",
		"2100",
		"2100",
	},
}

func TestKinds(t *testing.T) {
	assert.Equal(t, Floor, KindOf('.'))
	assert.Equal(t, Door, KindOf('7'))
	assert.Equal(t, Wall, KindOf('?'), "a rune not in the legend is a wall")
	assert.Equal(t, true, KindOf('t').Walkable(), "a chest's cell is walked up to the chest")
	assert.Equal(t, true, KindOf('~').Ground())
	assert.Equal(t, false, KindOf('~').Walkable())
	assert.Equal(t, true, KindOf('r').Solid())
	assert.Equal(t, false, KindOf('c').Ground())
}

func TestMap(t *testing.T) {
	m := &terrace
	assert.Equal(t, 4, m.Cols())
	assert.Equal(t, 3, m.Rows())
	assert.Equal(t, byte('#'), m.Cell(-1, 0), "outside the map is wall")
	assert.Equal(t, byte('~'), m.Cell(3, 2))
	assert.Equal(t, 2, m.Level(-5, 0), "the level outside is the nearest cell's")
	// The middle of cell 0, 0 is at the far left: -X, +Z.
	assert.Equal(t, Vec{X: -96, Z: 64}, m.Middle(0, 0))
	i, j := m.Index(m.Middle(3, 2))
	assert.Equal(t, 3, i)
	assert.Equal(t, 2, j)
	assert.Equal(t, byte('~'), m.At(m.Middle(3, 2)))
	assert.Equal(t, byte('#'), m.At(Vec{X: -200, Z: 0}), "off the map's left")
	assert.Equal(t, true, m.Walkable(m.Middle(1, 1)))
	assert.Equal(t, false, m.Walkable(m.Middle(3, 2)), "water")
}

func TestLoad(t *testing.T) {
	var s Setting
	s.Art.Floors = make([]Surface, 1)
	s.Art.Walls = make([]Surface, 1)
	s.Load(&terrace)

	// On the top row cells 1 and 2 are two levels apart: a cliff, each
	// keeping its own level at the corners they share, and cell 1
	// drawing a face down towards +X, side 2 of Sides.
	assert.Equal(t, int16(2*LevelUnits), s.corners[0][1][1], "the cliff's upper edge")
	assert.Equal(t, int16(0), s.corners[0][2][0], "the cliff's lower edge")
	assert.Equal(t, uint8(1<<2), s.dropMask[0][1]&(1<<2))
	assert.Equal(t, uint8(0), s.dropMask[0][2]&(1<<3), "the lower cell draws no face")

	// On the middle row the levels step 2, 1, 0: joined, so the corner
	// between cells 0 and 1 in rows 1 and 2 is the mean of 2, 1, 2, 1.
	assert.Equal(t, int16(3*LevelUnits/2), s.corners[1][0][3])
	assert.Equal(t, uint8(0), s.dropMask[1][1], "a slope has no faces")

	// The height under the middle of a cell on the slope is between its
	// corners'; the position is in sixteenths.
	mid := s.HeightAt(terrace.Middle(1, 1).Sixteenths()) >> 4
	assert.Between(t, LevelUnits/2, 3*LevelUnits/2, mid, "between the corners")
	assert.Equal(t, 0, s.HeightAt(terrace.Middle(3, 0).Sixteenths()), "level ground")

	// Plain ground is drawn the short way; water and the wall are not.
	assert.Equal(t, true, s.cells[0][0].plain)
	assert.Equal(t, false, s.cells[2][3].plain)
	assert.Equal(t, false, s.cells[2][0].plain)

	// A chest is closed on loading and opened by Open.
	assert.Equal(t, false, s.cells[0][0].open)
	s.Open(0, 0)
	assert.Equal(t, true, s.cells[0][0].open)
}

func TestView(t *testing.T) {
	v := View{Ahead: Vec{X: 0, Z: 4096}, Right: Vec{X: 4096, Z: 0}}
	assert.Equal(t, true, v.Sees(Vec{X: 0, Z: viewAhead * Tile}, 0))
	assert.Equal(t, false, v.Sees(Vec{X: 0, Z: (viewAhead + 1) * Tile}, 0))
	assert.Equal(t, true, v.Sees(Vec{X: 0, Z: (viewAhead + 1) * Tile}, 1), "within reach")
	assert.Equal(t, false, v.Sees(Vec{X: (viewAside + 1) * Tile, Z: 0}, 0))
}
