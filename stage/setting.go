package stage

import "go.chrisrx.dev/psx/yaroze"

// The room the hero is in, drawn from its map every frame: a textured
// quad for each floor cell, lit by its slope, for each side of a wall
// cell that borders floor, two tiles high, and for the wall's top, a
// cabin's log walls and tented roof, a bed, a chest. A face is drawn
// only when it faces the camera, so the walls nearest the camera, whose
// faces look away from it, are not drawn and the room is seen into from
// whichever side the camera is on. The walls nearer the camera than the
// hero have no tops, and one that stands between the camera and the hero
// is drawn half see-through, so that the hero shows through it as Dragon
// Quest's did; a cabin is see-through as a whole when any piece of it
// nearer the camera than the hero covers the hero on the screen.
const (
	// A wall is two tiles high, and a cabin's roof rises ridge above its
	// walls.
	wallHeight = 2 * Tile
	ridge      = 40
	bedHeight  = 24

	// A chest: half its width and depth, which a game keeps its figures
	// off, the height of its box, and of its lid on top; the lid stands
	// up when it is open, leaning back chestLean.
	ChestHalfW = 20
	ChestHalfD = 14
	chestBox   = 18
	chestLid   = 8
	chestLean  = 8

	// The cells drawn: those within viewAhead tiles ahead of the view's
	// center as the camera looks, viewBehind behind, and viewAside to
	// either side, which covers the screen with the camera at any
	// quarter and between.
	viewAhead  = 11
	viewBehind = 6
	viewAside  = 8

	maxBlocks = 15
)

const (
	// The water in a pool lies PoolDepth units below the ground round
	// it, and its bed PoolDepth more; a game puts what comes out of a
	// pool at the water's height.
	PoolDepth = 6

	// jitter is how far a corner of a rough room's cells may move, in
	// units, either way along each axis.
	jitter = 14
)

// A Setting is a map made ready to draw, by Load: the ground's height
// at each corner of its cells, each cell's shape as the drawing takes
// it, and which cabin each cell belongs to; and what the drawing keeps
// between frames about the cabins. A game has one, fills its Art
// before the first Load, and Draws it every frame.
type Setting struct {
	// Art is the textures the setting is drawn with.
	Art Art

	m *Map

	// view is where the map was last drawn from, frame how many times
	// it has been drawn, for what moves on its own, and prop the pots'
	// sprite, with feet and fsxy the point a prop is placed by.
	view  View
	frame int
	prop  yaroze.GsSPRITE
	feet  yaroze.SVECTOR
	fsxy  uint32

	// cells holds each cell's shape, ready to draw.
	cells [MaxCells][MaxCells]cellShape

	// corners holds the ground's height in units at each cell's four
	// corners, far left, far right, near left and near right. A corner
	// shared by cells within a level of each other is at the mean of
	// their levels, so that the ground slopes between them; one shared
	// across a cliff, cells two or more levels apart, is at each cell's
	// own level, and the higher cell draws a rock face down to the lower.
	corners [MaxCells][MaxCells][4]int16

	// at holds the world position of each corner of the room's cells,
	// corner ci, cj being the far left corner of cell ci, cj, moved a
	// little in a rough room; and dropMask holds, for each cell, a bit
	// for each side towards a neighbor lower than it, in sides' order,
	// which is where a rock face is drawn down to the neighbor.
	at       [MaxCells + 1][MaxCells + 1]Vec
	dropMask [MaxCells][MaxCells]uint8

	// low holds the lowest ground at each corner among the cells round
	// it, which is where a rock face down a cliff ends, so that the faces
	// of a cell meeting at a corner meet, whatever each neighbor's height
	// there. top holds where each corner is at the top of the rock: on
	// the rock's outline, moved into the rock by the room's lean, away
	// from the ground cells round it, so that the faces slope and the
	// tops of the cells sharing the corner still meet.
	low [MaxCells + 1][MaxCells + 1]int16
	top [MaxCells + 1][MaxCells + 1]Vec

	// edges marks the corners on the rock's outline, with both rock and
	// ground among the cells round them, which the ground's smoothing
	// moves; ring marks those and the other corners of the rock that
	// borders the ground, which the tops' smoothing moves, so that the
	// band of rock seen round a passage is smooth on both its sides.
	edges   [MaxCells + 1][MaxCells + 1]bool
	ring    [MaxCells + 1][MaxCells + 1]bool
	scratch [MaxCells + 1][MaxCells + 1]Vec

	// blocks numbers the room's cabins from 1, by cell: a cabin is the
	// 'c' cells joined side to side. 0 is no cabin. There are at most
	// maxBlocks of them, and blockRows holds each one's first and last
	// row.
	blocks    [MaxCells][MaxCells]uint8
	blockRows [maxBlocks + 1][2]int

	// ghost is which cabins are see-through this frame, hit which have
	// had a piece drawn over the hero this frame, and block the cabin
	// whose pieces are being drawn, or 0.
	ghost [maxBlocks + 1]bool
	hit   [maxBlocks + 1]bool
	block uint8
}

// cur is the setting being drawn, for the polygons to report what they
// cover.
var cur *Setting

// Load makes the map ready to draw: works out its corner heights, each
// the mean of the levels of the four cells round it, where its corners
// lie, the shape of each cell, and numbers its cabins. Every chest is
// closed; Open opens one.
func (s *Setting) Load(r *Map) {
	s.m = r
	s.prop = s.Art.Props
	s.prop.W, s.prop.H = PropSize, PropSize
	s.prop.Mx, s.prop.My = PropSize/2, PropFeet
	for j := range r.Rows() {
		for i := range r.Cols() {
			s.blocks[j][i] = 0
			for k := range 4 {
				s.corners[j][i][k] = int16(s.cornerHeight(i, j, k))
			}
		}
	}
	// The corners' places, and where rock stands, its outline rounded
	// and its tops leaned.
	rocky := false
	for j := range r.Rows() {
		for i := range r.Cols() {
			rocky = rocky || r.Cell(i, j) == 'r'
		}
	}
	for cj := 0; cj <= r.Rows(); cj++ {
		for ci := 0; ci <= r.Cols(); ci++ {
			s.at[cj][ci] = s.place(ci, cj)
			s.edges[cj][ci] = rocky && s.edge(ci, cj)
			s.ring[cj][ci] = rocky && s.inRing(ci, cj)
		}
	}
	if rocky {
		s.smooth(&s.at, &s.edges, 2)
	}
	for cj := 0; cj <= r.Rows(); cj++ {
		for ci := 0; ci <= r.Cols(); ci++ {
			s.top[cj][ci] = s.leaned(ci, cj)
		}
	}
	if rocky {
		s.smooth(&s.top, &s.ring, 2)
	}
	for cj := 0; cj <= r.Rows(); cj++ {
		for ci := 0; ci <= r.Cols(); ci++ {
			// The cells round the corner: corner k of each, the far
			// right corner of the cell before and above, and so on.
			// Outside the room the ground is at nothing, so that a
			// room's edge that stands high has a face down to it.
			lowest := int16(32767)
			for k := range 4 {
				i, j := ci-1+k&1, cj-1+k>>1
				if i >= 0 && j >= 0 && i < r.Cols() && j < r.Rows() {
					lowest = min(lowest, s.corners[j][i][3-k])
				} else {
					lowest = 0
				}
			}
			s.low[cj][ci] = lowest
		}
	}
	for j := range r.Rows() {
		for i := range r.Cols() {
			s.dropMask[j][i] = 0
			c := &s.corners[j][i]
			for k, side := range &Sides {
				ni, nj := i+side.DI, j+side.DJ
				a, b := across[k][0], across[k][1]
				var ha, hb int16
				if ni >= 0 && nj >= 0 && ni < r.Cols() && nj < r.Rows() {
					n := &s.corners[nj][ni]
					ha, hb = n[a[1]], n[b[1]]
				}
				if c[a[0]] > ha || c[b[0]] > hb {
					s.dropMask[j][i] |= 1 << k
				}
			}
		}
	}
	s.shapes()
	n := uint8(0)
	for j := range r.Rows() {
		for i := range r.Cols() {
			if r.Cell(i, j) == 'c' && s.blocks[j][i] == 0 && n < maxBlocks {
				n++
				s.blockRows[n] = [2]int{j, j}
				s.fill(i, j, n)
			}
		}
	}
}

// cornerHeight is the ground's height at corner k of cell i, j: the mean
// of the levels of the cells round the corner that the cell is joined to
// without a cliff between, side by side cells two or more levels apart
// being a cliff. On a smooth hill that is all four; at a terrace's edge
// the cells above and the cells below each keep their own level, and
// drops draws the face between.
func (s *Setting) cornerHeight(i, j, k int) int {
	r := s.m
	if r.Heights == nil {
		return 0
	}
	ci, cj := i+k&1, j+k>>1
	levels := [4]int{r.Level(ci-1, cj-1), r.Level(ci, cj-1), r.Level(ci-1, cj), r.Level(ci, cj)}
	if lo, hi := min(levels[0], levels[1], levels[2], levels[3]), max(levels[0], levels[1], levels[2], levels[3]); hi-lo <= 1 {
		return (levels[0] + levels[1] + levels[2] + levels[3]) * LevelUnits / 4
	}
	// The cell itself is at the corner's far side for its near corners,
	// and so on: the fourth cell for corner 0, the third for 1, the
	// second for 2 and the first for 3.
	var joined [4]bool
	joined[3-k] = true
	for range 3 {
		for _, pair := range [4][2]int{{0, 1}, {2, 3}, {0, 2}, {1, 3}} {
			a, b := pair[0], pair[1]
			if joined[a] != joined[b] && abs(levels[a]-levels[b]) <= 1 {
				joined[a], joined[b] = true, true
			}
		}
	}
	sum, n := 0, 0
	for c := range 4 {
		if joined[c] {
			sum += levels[c]
			n++
		}
	}
	return sum * LevelUnits / n
}

// fill numbers the cabin cell at i, j and, through its sides, the rest
// of its cabin, as n, and widens the cabin's rows to take the cell in.
func (s *Setting) fill(i, j int, n uint8) {
	if s.m.Cell(i, j) != 'c' || s.blocks[j][i] != 0 {
		return
	}
	s.blocks[j][i] = n
	s.blockRows[n][0] = min(s.blockRows[n][0], j)
	s.blockRows[n][1] = max(s.blockRows[n][1], j)
	s.fill(i+1, j, n)
	s.fill(i-1, j, n)
	s.fill(i, j+1, n)
	s.fill(i, j-1, n)
}

// heightAt returns the ground's height under a point, in sixteenths of a
// unit from a position in sixteenths, between the corners of its cell;
// outside the room it is the height at the edge.
func (s *Setting) HeightAt(p Vec) int {
	const cell = Tile * 16
	r := s.m
	u := min(max(p.X+r.Cols()*Tile*8, 0), r.Cols()*cell-1)
	v := min(max(r.Rows()*Tile*8-p.Z, 0), r.Rows()*cell-1)
	i, fx := u/cell, u%cell
	j, fz := v/cell, v%cell
	c := &s.corners[j][i]
	tl, tr, bl, br := int(c[0]), int(c[1]), int(c[2]), int(c[3])
	top := tl*cell + (tr-tl)*fx
	bottom := bl*cell + (br-bl)*fx
	return (top*cell + (bottom-top)*fz) >> 16
}

// place returns the world position of corner ci, cj of the room's cells:
// corner i, j is the far left corner of cell i, j. In a rough room, and
// wherever rock stands, the corner is moved by a hash of where it is,
// the same every frame, so that cells sharing it stay joined and rock has
// the broken outline of a cliff rather than a wall's.
func (s *Setting) place(ci, cj int) Vec {
	r := s.m
	p := Vec{ci*Tile - r.Cols()*Tile/2, r.Rows()*Tile/2 - cj*Tile}
	rocky := r.Cell(ci-1, cj-1) == 'r' || r.Cell(ci, cj-1) == 'r' || r.Cell(ci-1, cj) == 'r' || r.Cell(ci, cj) == 'r'
	if r.Rough || rocky {
		h := uint32(ci)*374761393 + uint32(cj)*668265263
		h = (h ^ h>>13) * 1274126177
		p.X += int(h%(2*jitter+1)) - jitter
		p.Z += int(h>>8%(2*jitter+1)) - jitter
	}
	return p
}

// edge reports whether corner ci, cj is on the outline of the rock:
// among the cells round it both rock and ground.
func (s *Setting) edge(ci, cj int) bool {
	r := s.m
	rock, open := false, false
	for k := range 4 {
		c := r.Cell(ci-1+k&1, cj-1+k>>1)
		rock = rock || c == 'r'
		open = open || ground(c)
	}
	return rock && open
}

// inRing reports whether corner ci, cj belongs to a rock cell that
// borders the ground.
func (s *Setting) inRing(ci, cj int) bool {
	r := s.m
	for k := range 4 {
		i, j := ci-1+k&1, cj-1+k>>1
		if r.Cell(i, j) == 'r' && s.borders(i, j) {
			return true
		}
	}
	return false
}

// smooth rounds the rock's outline in a table of corner positions, the
// ground's or the tops', passes times over: each corner marked in which
// moves halfway towards the mean of the marked corners next to it, which
// turns the steps of a passage that runs diagonally across the cells
// into a slope, and leaves the cells joined, since they share the
// corners.
func (s *Setting) smooth(at *[MaxCells + 1][MaxCells + 1]Vec, which *[MaxCells + 1][MaxCells + 1]bool, passes int) {
	r := s.m
	for range passes {
		// The moved corners are worked out from the corners as they are
		// and put in place after, in the scratch table, which is only
		// read and written at the outline's corners: copying the whole
		// table would be a byte at a time on this target.
		for cj := 0; cj <= r.Rows(); cj++ {
			for ci := 0; ci <= r.Cols(); ci++ {
				if !which[cj][ci] {
					continue
				}
				sum, n := Vec{}, 0
				for _, d := range [4]Vec{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					ni, nj := ci+d.X, cj+d.Z
					if ni >= 0 && nj >= 0 && ni <= r.Cols() && nj <= r.Rows() && which[nj][ni] {
						sum = sum.Add(at[nj][ni])
						n++
					}
				}
				p := at[cj][ci]
				if n > 0 {
					p = Vec{(p.X*n + sum.X) / (2 * n), (p.Z*n + sum.Z) / (2 * n)}
				}
				s.scratch[cj][ci] = p
			}
		}
		for cj := 0; cj <= r.Rows(); cj++ {
			for ci := 0; ci <= r.Cols(); ci++ {
				if which[cj][ci] {
					at[cj][ci] = s.scratch[cj][ci]
				}
			}
		}
	}
}

// leaned returns where corner ci, cj is at the top of the rock: its
// place on the ground moved by the room's lean away from the ground
// cells round it, along the sum of the directions from their middles,
// so that a straight face leans straight back and a corner of the rock
// leans back on both its sides.
func (s *Setting) leaned(ci, cj int) Vec {
	r := s.m
	p := s.at[cj][ci]
	if r.RockLean == 0 {
		return p
	}
	var sum Vec
	rock := false
	for k := range 4 {
		// Cell k round the corner lies towards (-1 or +1, -1 or +1)
		// from it in columns and rows; a row down is -Z.
		di, dj := k&1*2-1, k>>1*2-1
		c := r.Cell(ci+(di-1)/2, cj+(dj-1)/2)
		rock = rock || c == 'r'
		if ground(c) {
			sum.X -= di
			sum.Z += dj
		}
	}
	// Only rock leans: a cabin's corners stay where its walls stand.
	if !rock || sum == (Vec{}) {
		return p
	}
	n := max(abs(sum.X), abs(sum.Z))
	return Vec{p.X + r.RockLean*sum.X/n, p.Z + r.RockLean*sum.Z/n}
}

// A cellShape is a cell as the drawing takes it: its four corners on
// the ground, far left, far right, near left and near right, as a quad's
// corners go, each at the ground's height there, as the GTE takes them,
// up being -Y; the corners at the top of the rock, leaned back from the
// ground, for rock and the doorways through it, and the ground's corners
// again for the rest; what the cell is; and, for the ground, the Color
// word of its quad, lit by its slope, and the skin it is drawn with, the
// floor's or, where it is too steep to walk, the wall's. A plain cell is
// ground with nothing on it, which is most of a room, and is drawn by
// the shortest way.
type cellShape struct {
	v     [4]yaroze.SVECTOR
	top   [4]Vec
	cell  byte
	plain bool
	open  bool
	rgb   uint32
	skin  *skin
}

// Open opens the chest on cell i, j, which is drawn open from then on,
// until the map is loaded again.
func (s *Setting) Open(i, j int) { s.cells[j][i].open = true }

// x, z and h are the cell's corner n on the ground: where it is and how
// high.
//
//go:inline
func (c *cellShape) x(n int) int { return int(c.v[n].Vx) }

//go:inline
func (c *cellShape) z(n int) int { return int(c.v[n].Vz) }

//go:inline
func (c *cellShape) h(n int) int { return -int(c.v[n].Vy) }

// shapes fills in the cells' shapes from the corners' heights and
// places, once the outlines are smoothed and the tops leaned, and lights
// the ground: a rise towards +X or +Z, where the sun is, brighter, a
// fall darker; a doorway darker still, and ground too steep to walk as
// rock. A pool's bed is darker than the ground round it; the water over
// it is drawn by drawGround, which shimmers.
func (s *Setting) shapes() {
	r := s.m
	for j := range r.Rows() {
		for i := range r.Cols() {
			sh := &s.cells[j][i]
			c := r.Cell(i, j)
			sh.cell = c
			sh.plain = c == '.' || c == 'x'
			sh.open = false
			heights := &s.corners[j][i]
			for k := range 4 {
				ci, cj := i+k&1, j+k>>1
				at := s.at[cj][ci]
				sh.v[k] = yaroze.SVECTOR{Vx: int16(at.X), Vy: -heights[k], Vz: int16(at.Z)}
				sh.top[k] = at
				if c == 'r' || door(c) {
					sh.top[k] = s.top[cj][ci]
				}
			}
			tl, tr, bl, br := int(heights[0]), int(heights[1]), int(heights[2]), int(heights[3])
			bright := min(max(128+(tr+br-tl-bl)/2+(tl+tr-bl-br)/2, 64), 200)
			tint, sk := r.Tint, &s.Art.Floors[r.Floor].full
			switch {
			case door(c):
				tint = Color{48, 48, 56}
			case c == '~':
				bright = bright * 2 / 3
			case steep(tl, tr, bl, br):
				tint, sk = Gray(120), &s.Art.Walls[r.Wall].full
			}
			sh.rgb = tint.Shade(bright).Word(quadCode)
			sh.skin = sk
		}
	}
}

// across pairs a cell's corners on its side towards sides[k], in
// wallSide's order, with the neighbor's corners at the same places.
var across = [4][2][2]int{{{2, 0}, {3, 1}}, {{1, 3}, {0, 2}}, {{3, 2}, {1, 0}}, {{0, 1}, {2, 3}}}

// drops draws the rock faces from the cell down to each neighbor whose
// corners on their shared side are lower: the cliffs between terraces,
// the ground under a wall that stands higher than the floor beside it,
// and the side of a room's edge that stands above nothing. The face is
// seen from the neighbor, so its corners run as a wall's side does, and
// it goes down to the lowest ground at each corner, so that faces
// meeting at a corner meet.
func (s *Setting) drops(ot *yaroze.GsOT, i, j int, c *cellShape) {
	mask := s.dropMask[j][i]
	if mask == 0 {
		return
	}
	r := s.m
	for k := range 4 {
		if mask&(1<<k) == 0 {
			continue
		}
		a, b := across[k][0], across[k][1]
		ha, hb := int(s.low[j+a[0]>>1][i+a[0]&1]), int(s.low[j+b[0]>>1][i+b[0]&1])
		poly.v[0], poly.v[1] = c.v[a[0]], c.v[b[0]]
		poly.corner(2, c.x(a[0]), -ha, c.z(a[0]))
		poly.corner(3, c.x(b[0]), -hb, c.z(b[0]))
		poly.quad(ot, &s.Art.Walls[r.Wall], Gray(104), false, 0)
	}
}

// set puts the cell's corners into the polygon, each at its ground's
// height and raised more.
func (c *cellShape) set(raise int) {
	for n := range 4 {
		v := c.v[n]
		v.Vy -= int16(raise)
		poly.v[n] = v
	}
}

// setTop puts the cell's top corners into the polygon, raised over the
// ground's height at each.
func (c *cellShape) setTop(raise int) {
	for n := range 4 {
		poly.corner(n, c.top[n].X, -c.h(n)-raise, c.top[n].Z)
	}
}

// covers marks the cabin being drawn as covering the view's figure when
// the piece just projected, the first n corners of p at depth otz, is
// nearer than the figure and lies over the middle of them on the
// screen, a strip a few pixels either side of the feet's x up to the
// head.
func (s *Setting) covers(p *polygon, n, otz int) {
	v := &s.view
	if s.block == 0 || s.hit[s.block] || otz >= v.Depth {
		return
	}
	minX, minY, maxX, maxY := p.bounds(n)
	if maxX >= v.X-6 && minX <= v.X+6 && maxY >= v.Y-38 && minY <= v.Y {
		s.hit[s.block] = true
	}
}

// Draw draws the map from the view, cell by cell, those near enough to
// its center to be on the screen, into the ordering table, through the
// world-to-screen matrix the library has from the camera the game set.
func (s *Setting) Draw(ot *yaroze.GsOT, v *View) {
	r := s.m
	cur = s
	s.view = *v
	s.frame++
	yaroze.GsSetLsMatrix(yaroze.GsWSMATRIX())
	// A cabin is see-through as a whole when any piece of it nearer the
	// camera than the figure covers the figure on the screen, which the
	// pieces report as they are drawn, for the next frame.
	s.ghost, s.hit = s.hit, [maxBlocks + 1]bool{}
	// How far the first cell's middle is ahead of the view's center, as
	// the camera looks, and across from it, in 4096ths of a unit, and
	// what a cell along a row and a cell down the map add, a row down
	// being -Z; cells beyond the screen are skipped.
	p := r.Middle(0, 0).Sub(v.At)
	d0, lat0 := p.Dot(v.Ahead), p.Dot(v.Right)
	dCol, latCol := v.Ahead.X*Tile, v.Right.X*Tile
	dRow, latRow := -v.Ahead.Z*Tile, -v.Right.Z*Tile
	for j := range r.Rows() {
		d, lat := d0+j*dRow, lat0+j*latRow
		for i := range r.Cols() {
			if d <= viewAhead*Tile<<12 && d >= -viewBehind*Tile<<12 && abs(lat) <= viewAside*Tile<<12 {
				s.drawCell(ot, i, j, d>>12, lat>>12)
			}
			d += dCol
			lat += latCol
		}
	}
}

// drawCell draws cell i, j, d ahead of the hero and lat across: the
// ground, with what stands on it, or a wall or cabin cell. Plain ground
// is the shortest way through: its quad's corners, Color and skin were
// worked out when the room was entered.
func (s *Setting) drawCell(ot *yaroze.GsOT, i, j int, d, lat int) {
	r := s.m
	shape := &s.cells[j][i]
	s.drops(ot, i, j, shape)
	if shape.plain {
		poly.place(ot, &shape.v, shape.skin, shape.rgb, floorBack, false)
		return
	}
	c := shape.cell
	p := r.Middle(i, j)
	x0, x1, z0, z1 := p.X-Tile/2, p.X+Tile/2, p.Z-Tile/2, p.Z+Tile/2
	if ground(c) {
		s.drawGround(ot, i, j, shape)
		switch {
		case c == 't':
			s.drawChest(ot, p, shape.open)
		case c == 'o':
			s.drawProp(ot, p, s.HeightAt(p.Sixteenths())>>4, 0)
		case door(c):
			s.mouth(ot, i, j, shape)
		}
		return
	}
	if bed(c) {
		s.drawBed(ot, i, j, x0, x1, z0, z1, c == 'p')
		return
	}
	near := d < -Tile/2
	ghost := near && lat > -Tile && lat < Tile
	cabin := c == 'c'
	tex, height, top := &s.Art.Walls[r.Wall], wallHeight, Color{72, 72, 80}
	if c == 'r' {
		tex, height, top = &s.Art.Rock, r.rockHeight(), Gray(100)
	}
	if cabin {
		tex = &s.Art.Logs
		s.block = s.blocks[j][i]
		ghost = s.ghost[s.block]
	}
	// The sides onto the floor first; a wall's sides onto a doorway, the
	// jambs, only when one of those was seen, and the top too, so that a
	// wall seen from behind, which is what the walls nearest the camera
	// are, is left out whole rather than left as a jamb or a top. A
	// cabin's or rock's side onto its doorway is the back of the doorway,
	// the mouth of a cave, drawn whenever it is seen.
	seen := false
	for pass := range 2 {
		for k, side := range &Sides {
			n := r.Cell(i+side.DI, j+side.DJ)
			jamb := door(n) && !cabin && c != 'r'
			if !ground(n) || jamb != (pass == 1) || (pass == 1 && !seen) {
				continue
			}
			a, b, sh := wallSide(k)
			if door(n) && c == 'r' {
				// The inside of a cave's mouth, dark, so that the
				// opening reads as a hole.
				sh = 36
			}
			seen = s.face(ot, tex, shape, a, b, height, sh, ghost) || seen
		}
	}
	// A wall's top goes with the wall, when none of its sides faces the
	// camera, and when the wall is nearer the camera than the hero, where
	// it could cover them. Rock keeps its top whether a side was seen or
	// not, since rock is a mass, whose top is what is seen of it beyond
	// the first face, and near the camera too; a top in line with the
	// hero is see-through. A black room keeps only the tops of the rock
	// that borders the ground, the walls of its passages, which from the
	// camera's side are seen by their tops alone, and leaves the rock
	// beyond black. A cabin's roof is drawn whatever its sides do, since
	// the cells in the middle of a cabin have no sides to be seen, and a
	// cabin keeps its roof, which is what makes it a cabin, drawn
	// see-through instead when it is in the way.
	switch {
	case cabin:
		s.roof(ot, i, j, x0, x1, z0, z1, ghost)
	case c != 'r' && (near || !seen):
	case c == 'r' && r.Black && !s.borders(i, j):
	default:
		shape.setTop(height)
		poly.quad(ot, tex, top, ghost, 0)
	}
	s.block = 0
}

// drawGround draws the ground of a cell that is not plain: a pool, a
// doorway, or ground with a chest or a pot on it. A pool is its bed,
// dark, below the ground round it, and the water over it, half
// see-through, its brightness swelling and fading. A doorway under a
// cabin's roof, in its far row or its near one, gets its share of the
// roof.
func (s *Setting) drawGround(ot *yaroze.GsOT, i, j int, shape *cellShape) {
	r := s.m
	c := shape.cell
	if c == '~' {
		shape.set(-2 * PoolDepth)
		poly.place(ot, &poly.v, shape.skin, shape.rgb, floorBack, false)
		shape.set(-PoolDepth)
		wave := 100 + abs((s.frame>>2)&31-16)
		poly.ground(ot, &s.Art.Water, Gray(uint8(wave)), true, floorBack-1)
		return
	}
	poly.place(ot, &shape.v, shape.skin, shape.rgb, floorBack, false)
	if !door(c) {
		return
	}
	x0, x1, z0, z1 := shape.x(0), shape.x(1), shape.z(2), shape.z(0)
	if b := s.blocks[max(j-1, 0)][i]; b != 0 || j+1 < r.Rows() && s.blocks[j+1][i] != 0 {
		if b == 0 {
			b = s.blocks[j+1][i]
		}
		k, n := j-s.blockRows[b][0], s.blockRows[b][1]-s.blockRows[b][0]+1
		s.block = b
		s.roofSlope(ot, rise(k, n), rise(k+1, n), x0, x1, z0, z1, s.ghost[b])
		s.block = 0
	}
}

// borders reports whether any side of cell i, j is onto the ground.
func (s *Setting) borders(i, j int) bool {
	r := s.m
	return ground(r.Cell(i+1, j)) || ground(r.Cell(i-1, j)) || ground(r.Cell(i, j+1)) || ground(r.Cell(i, j-1))
}

// mouthHeight is how tall the opening of a cave's mouth is.
const mouthHeight = Tile * 3 / 4

// mouth draws, over a doorway with rock to either side of it, the rock
// over the opening: the top of the rock across the cell, and on each open
// side a lintel from the top down to mouthHeight, so that the doorway is
// a hole in a rock face, the mouth of a cave.
func (s *Setting) mouth(ot *yaroze.GsOT, i, j int, c *cellShape) {
	r := s.m
	var open [2]int // the sides the opening faces, in sides' order
	switch {
	case r.Cell(i-1, j) == 'r' && r.Cell(i+1, j) == 'r':
		open = [2]int{0, 1}
	case r.Cell(i, j-1) == 'r' && r.Cell(i, j+1) == 'r':
		open = [2]int{2, 3}
	default:
		return
	}
	height := r.rockHeight()
	if !r.Black {
		c.setTop(height)
		poly.quad(ot, &s.Art.Rock, Gray(100), false, 0)
	}
	for _, k := range open {
		a, b, sh := wallSide(k)
		ta, tb := c.top[a], c.top[b]
		ba, bb := c.lerp(a, mouthHeight, height), c.lerp(b, mouthHeight, height)
		poly.corner(0, ta.X, -c.h(a)-height, ta.Z)
		poly.corner(1, tb.X, -c.h(b)-height, tb.Z)
		poly.corner(2, ba.X, -c.h(a)-mouthHeight, ba.Z)
		poly.corner(3, bb.X, -c.h(b)-mouthHeight, bb.Z)
		poly.quad(ot, &s.Art.Rock, Gray(sh), false, 0)
	}
}

// steep reports whether a cell's ground, between its corner heights,
// rises along a side by more than a level and a third over the tile,
// which is more than a figure climbs: a cliff.
func steep(tl, tr, bl, br int) bool {
	rise := max(tl-tr, tr-tl, bl-br, br-bl, tl-bl, bl-tl, tr-br, br-tr)
	return rise*3 > LevelUnits*4
}

// wallSide gives the side of a cell towards its neighbor sides[k] as the
// cell's corners, left and right seen from the neighbor, and its shade:
// the sides along X a little darker than those along Z.
func wallSide(k int) (a, b int, shade uint8) {
	switch k {
	case 0: // towards -Z
		return 2, 3, 120
	case 1: // towards +Z
		return 1, 0, 120
	case 2: // towards +X
		return 3, 1, 100
	}
	return 0, 2, 100 // towards -X
}

// face draws one side of a wall cell, from the ground at its corners up
// height, a quad a tile, between the corners a and b of the cell's
// shape, its left and right seen from in front of it, and reports whether
// any of it was drawn: whether the side faces the camera.
func (s *Setting) face(ot *yaroze.GsOT, tex *Surface, c *cellShape, a, b, height int, shade uint8, ghost bool) bool {
	seen := false
	for top := height; top > 0; top -= Tile {
		// The corners lean back in proportion to their height up the
		// face, from the ground's at the bottom to the top's at the top.
		ta, tb := c.lerp(a, top, height), c.lerp(b, top, height)
		ba, bb := c.lerp(a, top-Tile, height), c.lerp(b, top-Tile, height)
		poly.corner(0, ta.X, -c.h(a)-top, ta.Z)
		poly.corner(1, tb.X, -c.h(b)-top, tb.Z)
		poly.corner(2, ba.X, -c.h(a)-top+Tile, ba.Z)
		poly.corner(3, bb.X, -c.h(b)-top+Tile, bb.Z)
		seen = poly.quad(ot, tex, Gray(shade), ghost, 0) || seen
	}
	return seen
}

// lerp is where corner n of the cell is at height up of the full
// height of its face: on the ground's corner at 0 and the top's at
// height.
func (c *cellShape) lerp(n, up, height int) Vec {
	x, z := c.x(n), c.z(n)
	if up <= 0 {
		return Vec{x, z}
	}
	return Vec{x + (c.top[n].X-x)*up/height, z + (c.top[n].Z-z)*up/height}
}

// rectSide returns a cell's plain rectangle's side towards sides[k], for the
// beds and chests, which stand on level ground.
func rectSide(k int, x0, x1, z0, z1 int) (ax, az, bx, bz int) {
	switch k {
	case 0:
		return x0, z0, x1, z0
	case 1:
		return x1, z1, x0, z1
	case 2:
		return x1, z0, x1, z1
	}
	return x0, z1, x0, z0
}

// drawProp draws a thing from the props sheet standing at p on ground y
// high: a billboard sorted at its feet's depth, as a game's figures
// are. The depth is the GTE's average of the point three times over, so
// that it is on the same scale as the polygons'.
func (s *Setting) drawProp(ot *yaroze.GsOT, p Vec, y, which int) {
	s.feet = yaroze.SVECTOR{Vx: int16(p.X), Vy: int16(-y), Vz: int16(p.Z)}
	if yaroze.RotTransPers(&s.feet, &s.fsxy) < 0 {
		return
	}
	yaroze.RotTransPers(&s.feet, &s.fsxy)
	yaroze.RotTransPers(&s.feet, &s.fsxy)
	otz := int(yaroze.AverageZ3())
	s.prop.X, s.prop.Y = int16(s.fsxy), int16(s.fsxy>>16)
	s.prop.U = s.Art.Props.U + uint8(PropSize*which)
	s.prop.Sort(ot, Standing(otz))
}

// roof draws a cabin cell's share of its roof: the slope over it, from
// the roof's height at the cell's far edge to its height at the near
// edge, and at an end of the block its piece of the gable there,
// see-through when ghost.
func (s *Setting) roof(ot *yaroze.GsOT, i, j int, x0, x1, z0, z1 int, ghost bool) {
	b := s.blocks[j][i]
	k, n := j-s.blockRows[b][0], s.blockRows[b][1]-s.blockRows[b][0]+1
	far, near := rise(k, n), rise(k+1, n)
	s.roofSlope(ot, far, near, x0, x1, z0, z1, ghost)
	if s.m.Cell(i-1, j) != 'c' {
		s.gable(ot, far, near, x0, z0, z1, -1, ghost)
	}
	if s.m.Cell(i+1, j) != 'c' {
		s.gable(ot, far, near, x1, z0, z1, 1, ghost)
	}
}

// rise is how far above the walls the roof of a cabin n rows deep is at
// the edge between its kth and (k+1)th rows: a tent, from nothing at the
// far and near edges up to ridge in the middle.
//
//go:inline
func rise(k, n int) int { return min(k, n-k) * 2 * ridge / n }

// roofSlope draws the roof over one cell, from far above the walls at
// its far edge to near at its near edge; a slope that faces the far side
// faces away from the sun, so it is darker.
func (s *Setting) roofSlope(ot *yaroze.GsOT, far, near, x0, x1, z0, z1 int, ghost bool) {
	poly.corner(0, x0, -wallHeight-far, z1)
	poly.corner(1, x1, -wallHeight-far, z1)
	poly.corner(2, x0, -wallHeight-near, z0)
	poly.corner(3, x1, -wallHeight-near, z0)
	shade := uint8(128)
	if near > far {
		shade = 96
	}
	poly.quad(ot, &s.Art.Roof, Gray(shade), ghost, 0)
}

// gableUV and gableUV4 are the texture corners of the gable piece being
// drawn.
var (
	gableUV  [3][2]uint8
	gableUV4 [4][2]uint8
)

// gable draws a row's piece of the gable at the end of a cabin block, on
// its -X side at x for side -1 or its +X side for 1: logs from the wall's
// top up to the roof, far above the walls at the row's far edge and near
// at its near edge, its corners clockwise seen from outside; where the
// roof meets the wall's top at one edge, a triangle.
func (s *Setting) gable(ot *yaroze.GsOT, far, near, x, z0, z1 int, side int, ghost bool) {
	// Left and right seen from outside: from -X, left is the far edge.
	tl, tr, zl, zr := far, near, z1, z0
	if side > 0 {
		tl, tr, zl, zr = near, far, z0, z1
	}
	vOf := func(t int) uint8 { return uint8(ridge - 1 - t*(ridge-1)/ridge) }
	top := func(n int, t, z int) { poly.corner(n, x, -wallHeight-t, z) }
	logs := &s.Art.Logs.Texture
	switch {
	case tl == 0 && tr == 0:
	case tl == 0:
		top(0, tr, zr)
		top(1, 0, zr)
		top(2, 0, zl)
		gableUV = [3][2]uint8{{63, vOf(tr)}, {63, ridge - 1}, {0, ridge - 1}}
		poly.tri(ot, logs, &gableUV, Gray(100), ghost)
	case tr == 0:
		top(0, tl, zl)
		top(1, 0, zr)
		top(2, 0, zl)
		gableUV = [3][2]uint8{{0, vOf(tl)}, {63, ridge - 1}, {0, ridge - 1}}
		poly.tri(ot, logs, &gableUV, Gray(100), ghost)
	default:
		top(0, tl, zl)
		top(1, tr, zr)
		top(2, 0, zl)
		top(3, 0, zr)
		gableUV4 = [4][2]uint8{{0, vOf(tl)}, {63, vOf(tr)}, {0, ridge - 1}, {63, ridge - 1}}
		poly.quadUV(ot, logs, &gableUV4, Gray(100), ghost, 0)
	}
}

// The bed texture's bands: the quilt, the pillow and the planks.
var (
	quiltUV  = [4][2]uint8{{0, 0}, {63, 0}, {0, 31}, {63, 31}}
	pillowUV = [4][2]uint8{{0, 32}, {63, 32}, {0, 47}, {63, 47}}
	plankUV  = [4][2]uint8{{0, 48}, {63, 48}, {0, 63}, {63, 63}}
)

// drawBed draws a bed's cell: a box bedHeight high with the quilt on
// top, or the pillow at the pillow's end, and planks on the sides that
// are not against the rest of the bed.
func (s *Setting) drawBed(ot *yaroze.GsOT, i, j int, x0, x1, z0, z1 int, pillow bool) {
	poly.corner(0, x0, -bedHeight, z1)
	poly.corner(1, x1, -bedHeight, z1)
	poly.corner(2, x0, -bedHeight, z0)
	poly.corner(3, x1, -bedHeight, z0)
	uv := &quiltUV
	if pillow {
		uv = &pillowUV
	}
	poly.quadUV(ot, &s.Art.Bed, uv, Gray(128), false, 0)
	for k, side := range &Sides {
		if bed(s.m.Cell(i+side.DI, j+side.DJ)) {
			continue
		}
		ax, az, bx, bz := rectSide(k, x0, x1, z0, z1)
		poly.corner(0, ax, -bedHeight, az)
		poly.corner(1, bx, -bedHeight, bz)
		poly.corner(2, ax, 0, az)
		poly.corner(3, bx, 0, bz)
		poly.quadUV(ot, &s.Art.Bed, &plankUV, Gray(110), false, 0)
	}
}

// The chest texture's bands: the lid's top, the front with the lock, a
// side, and the inside.
var (
	lidUV    = [4][2]uint8{{0, 0}, {63, 0}, {0, 15}, {63, 15}}
	frontUV  = [4][2]uint8{{0, 16}, {63, 16}, {0, 39}, {63, 39}}
	sideUV   = [4][2]uint8{{0, 40}, {63, 40}, {0, 55}, {63, 55}}
	insideUV = [4][2]uint8{{0, 56}, {63, 56}, {0, 63}, {63, 63}}
)

// drawChest draws a chest on the floor, centered at p, its front towards
// -Z: a box with the lid on top, closed, or, once opened, open to the
// inside with the lid standing up along its back edge, leaning back, its
// top towards the front and its underside behind.
func (s *Setting) drawChest(ot *yaroze.GsOT, p Vec, opened bool) {
	x0, x1, z0, z1 := p.X-ChestHalfW, p.X+ChestHalfW, p.Z-ChestHalfD, p.Z+ChestHalfD
	h := chestBox + chestLid
	top := &lidUV
	if opened {
		h, top = chestBox, &insideUV
	}
	poly.corner(0, x0, -h, z1)
	poly.corner(1, x1, -h, z1)
	poly.corner(2, x0, -h, z0)
	poly.corner(3, x1, -h, z0)
	poly.quadUV(ot, &s.Art.Chest, top, Gray(128), false, 0)
	// The sides, as a wall's: towards -Z, +Z, +X and -X.
	for k := range 4 {
		ax, az, bx, bz := rectSide(k, x0, x1, z0, z1)
		uv, shade := &sideUV, uint8(90)
		if k < 2 {
			uv, shade = &frontUV, 110
		}
		poly.corner(0, ax, -h, az)
		poly.corner(1, bx, -h, bz)
		poly.corner(2, ax, 0, az)
		poly.corner(3, bx, 0, bz)
		poly.quadUV(ot, &s.Art.Chest, uv, Gray(shade), false, 0)
	}
	if !opened {
		return
	}
	// The lid up: its top seen from the front, and its underside, the
	// same quad the other way round, from behind.
	lidTop, lidBack := -chestBox-2*chestLid-ChestHalfD, z1+chestLean
	poly.corner(0, x0, lidTop, lidBack)
	poly.corner(1, x1, lidTop, lidBack)
	poly.corner(2, x0, -chestBox, z1)
	poly.corner(3, x1, -chestBox, z1)
	poly.quadUV(ot, &s.Art.Chest, &lidUV, Gray(128), false, 0)
	poly.corner(0, x1, lidTop, lidBack)
	poly.corner(1, x0, lidTop, lidBack)
	poly.corner(2, x1, -chestBox, z1)
	poly.corner(3, x0, -chestBox, z1)
	poly.quadUV(ot, &s.Art.Chest, &sideUV, Gray(100), false, 0)
}
