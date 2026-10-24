// Package stage draws a room of cells as Dragon Quest VII drew its
// towns: a map of tiles, each a floor, a wall, rock, water or a piece of
// a cabin, with the ground's height at each cell, seen from a camera
// above that turns about whoever stands in it, as textured quads built
// every frame. A game writes its rooms as Maps, strings of cells with a
// legend of runes, hands the textures over in an Art, and each frame
// tells the Setting where the camera looks from in a View and has it
// Draw.
//
// The ground is the X-Z plane, up is -Y as the console has it, and a
// cell is a tile of Tile units, which is what the textures are made for.
// Cells with heights slope between their corners, up to a level a tile;
// a difference of two levels or more between neighbors is a cliff, and
// the higher cell draws a face of rock down to the lower. Walls are
// drawn only on the sides that face the camera, so that a room is seen
// into from whichever side the camera is on, and what stands between the
// camera and the figure in View is drawn half see-through, as Dragon
// Quest's was.
//
// The ordering table a room is drawn into is 1<<Depth entries deep, with
// Shift turning the GTE's depth into an entry; a game's own figures and
// models go into the same table at the entries Standing gives them, and
// a model sorted by the library at Shift.
package stage

import "go.chrisrx.dev/psx/yaroze"

const (
	// Tile is a cell's size in units, which is the textures' size.
	Tile = 64

	// LevelUnits is a level of height in units: a map's heights are
	// digits, a level each.
	LevelUnits = 20

	// MaxCells is the most cells a map is across or down.
	MaxCells = 32

	// Depth sizes the ordering table, 1<<Depth entries, and Shift turns
	// the GTE's depth, a quarter of the camera Z, into an entry: 8 units
	// of Z each, fine enough that a wall's face and the floor beside it
	// do not share one and get drawn in the order they were sorted,
	// which is the wrong way round half the time.
	Depth = 10
	Shift = 1

	// The floor goes floorBack entries further back than it is, since
	// from a camera above it it is never in front of a wall; a figure
	// comes figureFront entries forward, in front of the floor under
	// them and a wall they stand against. nearest is the depth a face is
	// left out under, too near the camera to draw sensibly.
	floorBack   = 8
	figureFront = 16
	nearest     = 8
)

// Standing is the ordering table entry for a figure or a thing standing
// on the ground at the GTE's depth otz: in front of the floor under it
// and a wall it stands against.
//
//go:inline
func Standing(otz int) uint16 { return uint16(max(otz>>Shift-figureFront, 0)) }

// A Kind is what a cell is, which its rune in a map says.
type Kind uint8

const (
	Floor   Kind = iota // '.', ground to walk on
	Wall                // '#', a wall two tiles high
	Rock                // 'r', rock standing on the ground, a cliff's face
	Cabin               // 'c', a cabin's wall of logs under a roof
	Bed                 // 'b', a bed
	Pillow              // 'p', the end of a bed with the pillow
	Chest               // 't', ground with a treasure chest on it
	Water               // '~', a pool
	Prop                // 'o', ground with a clay pot on it
	Blocked             // 'x', ground nobody walks on, under a creature
	Door                // '0' to '9', a doorway, which leads to the doorway with the same digit in another map
)

// KindOf is the kind of a cell by its rune; a rune not in the legend is
// a wall.
func KindOf(c byte) Kind {
	switch c {
	case '.':
		return Floor
	case 'r':
		return Rock
	case 'c':
		return Cabin
	case 'b':
		return Bed
	case 'p':
		return Pillow
	case 't':
		return Chest
	case '~':
		return Water
	case 'o':
		return Prop
	case 'x':
		return Blocked
	}
	if c >= '0' && c <= '9' {
		return Door
	}
	return Wall
}

// Ground reports whether the cell is drawn as the ground: anything that
// is not a wall, a cabin or a bed.
func (k Kind) Ground() bool {
	return k == Floor || k == Chest || k == Water || k == Prop || k == Blocked || k == Door
}

// Walkable reports whether a figure may walk on the cell: a chest's
// cell is walked on up to the chest itself, which the game keeps them
// off.
func (k Kind) Walkable() bool { return k == Floor || k == Chest || k == Door }

// Solid reports whether the cell is a wall, of stone or of rock.
func (k Kind) Solid() bool { return k == Wall || k == Rock }

func ground(c byte) bool { return KindOf(c).Ground() }
func wall(c byte) bool   { return KindOf(c).Solid() }
func bed(c byte) bool    { return c == 'b' || c == 'p' }
func door(c byte) bool   { return c >= '0' && c <= '9' }

// A Map is a room as a game writes it: rows of cells, a rune each, the
// ground's level at each, and how it is dressed. Row 0 is the far side
// of the room before the camera has turned, +Z, and the first column
// is -X; a map is at most MaxCells across and down. A cabin is a block
// of 'c' any number of rows deep, its roof rising to a ridge along the
// middle, with a doorway in its far or near row that leads to the map
// inside it. Walls stand on level ground only.
type Map struct {
	// Floor and Wall are the ground's texture and the walls', as
	// indexes into the Art's Floors and Walls, and Tint the ground's
	// color, 128 leaving the texture as it is.
	Floor, Wall int
	Tint        Color

	// Cells are the rows, all the same length, and Heights the ground's
	// levels, the same shape, or nil for level ground: the ground is
	// drawn through the corners of the cells, each corner at the mean
	// of the four cells round it, so a plateau of 2 among 0 is a hill
	// with a slope a tile wide on each side.
	Cells   []string
	Heights []string

	// Rough moves every corner of the map's cells a little, by a hash of
	// where it is, so that rock walls and the floor between them have
	// the broken outlines of a cave rather than a grid's.
	Rough bool

	// RockTall is how high the map's rock stands, a tile when zero, and
	// RockLean how far the top of a rock face leans back into the rock,
	// so that the face slopes like a cliff's. Black leaves the rock's
	// tops undrawn but for the rock bordering the ground, so that beyond
	// the faces there is only black, and a cave is its passages alone.
	RockTall, RockLean int
	Black              bool
}

// Cols and Rows are the map's size in cells.
func (m *Map) Cols() int { return len(m.Cells[0]) }
func (m *Map) Rows() int { return len(m.Cells) }

// Cell returns the cell at column i, row j, or a wall outside the map.
func (m *Map) Cell(i, j int) byte {
	if j < 0 || j >= len(m.Cells) || i < 0 || i >= len(m.Cells[j]) {
		return '#'
	}
	return m.Cells[j][i]
}

// Level returns the ground's level at a cell, the nearest cell's
// outside the map, and 0 in a map without heights.
func (m *Map) Level(i, j int) int {
	if m.Heights == nil {
		return 0
	}
	i = min(max(i, 0), m.Cols()-1)
	j = min(max(j, 0), m.Rows()-1)
	return int(m.Heights[j][i] - '0')
}

// rockHeight is how high the map's rock stands.
func (m *Map) rockHeight() int {
	if m.RockTall == 0 {
		return Tile
	}
	return m.RockTall
}

// Index returns the column and row of the cell under a point on the
// ground, in units from the map's middle, which may be outside the map.
func (m *Map) Index(p Vec) (i, j int) {
	u := p.X + m.Cols()*Tile/2
	v := m.Rows()*Tile/2 - p.Z
	return u / Tile, v / Tile
}

// At returns the cell under a point on the ground, in units from the
// map's middle, or a wall outside the map.
func (m *Map) At(p Vec) byte {
	i, j := m.Index(p)
	if p.X+m.Cols()*Tile/2 < 0 || m.Rows()*Tile/2-p.Z < 0 {
		return '#'
	}
	return m.Cell(i, j)
}

// Walkable reports whether a figure may stand at a point on the ground.
func (m *Map) Walkable(p Vec) bool { return KindOf(m.At(p)).Walkable() }

// Middle returns the position of the middle of a cell.
func (m *Map) Middle(i, j int) Vec {
	return Vec{i*Tile - m.Cols()*Tile/2 + Tile/2, m.Rows()*Tile/2 - j*Tile - Tile/2}
}

// A Side is a cell's neighbor, as steps in column and row, with the way
// a figure faces walking that way, in eighths of a turn from +Z
// clockwise: a row down the map is -Z.
type Side struct{ DI, DJ, Facing int }

// Sides are a cell's four neighbors: towards -Z, +Z, +X and -X.
var Sides = [4]Side{{0, 1, 4}, {0, -1, 0}, {1, 0, 2}, {-1, 0, 6}}

// A View is where a map is seen from: the point the drawing is
// centered on, the figure the camera follows, in units; which way the
// camera looks along the ground and across it to the right, in 4096ths;
// and where that figure's feet are on the screen and how deep, as the
// GTE gives them, for drawing what covers them see-through, or Depth 0
// when they are off the screen.
type View struct {
	At           Vec
	Ahead, Right Vec
	X, Y, Depth  int
}

// relative gives how far a point is ahead of the view's center, as the
// camera looks, and how far across from it, to the right.
func (v *View) relative(p Vec) (ahead, aside int) {
	d := p.Sub(v.At)
	return d.Dot(v.Ahead) >> 12, d.Dot(v.Right) >> 12
}

// Sees reports whether a point within reach tiles of the cells drawn is
// near enough to the view's center to be drawn: within viewAhead tiles
// ahead, viewBehind behind and viewAside to either side, plus reach.
func (v *View) Sees(p Vec, reach int) bool {
	ahead, aside := v.relative(p)
	return ahead <= (viewAhead+reach)*Tile && ahead >= -(viewBehind+reach)*Tile && abs(aside) <= (viewAside+reach)*Tile
}

// An Art is the textures a setting is drawn with, loaded into video
// memory by the game: the floors and the walls a map picks from by
// index, the logs of a cabin's walls and its roof, rock, water, the bed
// and the chest, and the sprite the pots are cut from, a PropSize square
// each from its U on, with its base on row PropFeet.
type Art struct {
	Floors, Walls []Surface
	Logs, Roof    Surface
	Rock, Water   Surface
	Bed, Chest    yaroze.Texture
	Props         yaroze.GsSPRITE
}

// A prop on the sheet is PropSize square, with its base on row PropFeet.
const (
	PropSize = 32
	PropFeet = 28
)
