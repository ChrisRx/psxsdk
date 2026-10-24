package main

import (
	"go.chrisrx.dev/psx/stage"
	"go.chrisrx.dev/psx/yaroze"
)

// Nora, the hero: a figure drawn flat over the room, walked by the
// direction buttons relative to the camera, kept to the floor and off
// cliffs, taken through doorways, and speaking to what they face.
const (
	// margin is half the hero's footprint: how near their middle may
	// come to a wall.
	margin = 14

	// Positions are in sixteenths of a unit; the hero walks 3 units a
	// frame, and about that along a diagonal, 3 over root 2 on each axis.
	// A step may climb or drop maxStep sixteenths for each speed of
	// movement: a slope of a level and a quarter a stage.Tile, which the slope a
	// stage.Tile wide between cells two levels apart is under, and three apart
	// is over.
	speed   = 3 << 4
	diag    = 34
	maxStep = 20

	// stepTime is the frames a step of the walk lasts.
	stepTime = 8

	// reach is how far in front of Nora interact looks, in units: a
	// person or a chest within it, the way Nora faces, is spoken to.
	reach = stage.Tile + margin
)

// A hero's first fields are what the tests read: the room, the position,
// the height and the facing, a word each.
type hero struct {
	// room is the room they are in, pos where they stand on its floor,
	// in sixteenths of a unit from the room's middle, height the ground's
	// height under them in units, facing the way they face, in eighths
	// of a turn from +Z clockwise seen from above (0 is away from the
	// camera before it has turned, 2 is to its right), walkTick how long
	// they have been walking, for the walk's frames, and onDoor whether
	// they stand on a doorway, so that one is taken once.
	room     int
	pos      stage.Vec
	height   int
	facing   int
	walkTick int
	onDoor   bool

	// Where the feet are on the screen this frame, and how deep, from
	// project, for the room's pieces to compare with as they are drawn.
	sx, sy int
	otz    int

	figure
}

var nora hero

// screenFacing is the way the direction buttons point as eighths of a turn
// on the screen from up clockwise, indexed by (dx+1)*3 + dy+1 as Pad.Dir
// gives them; -1 when none is held.
var screenFacing = [9]int{7, 6, 5, 0, -1, 4, 1, 2, 3}

// dirs are the eight ways to face as steps along each axis.
var dirs = [8]stage.Vec{{X: 0, Z: 1}, {X: 1, Z: 1}, {X: 1, Z: 0}, {X: 1, Z: -1}, {X: 0, Z: -1}, {X: -1, Z: -1}, {X: -1, Z: 0}, {X: -1, Z: 1}}

// here is the room the hero is in.
func (h *hero) here() *room { return &rooms[h.room] }

// at is where the hero stands, in world units.
func (h *hero) at() stage.Vec { return h.pos.Units() }

// walk moves the hero the way the direction buttons point, relative to
// the camera's angle, turns them that way, and keeps them on the floor
// and off cliffs: each axis is tried on its own, so a step into a wall
// slides along it. A doorway stepped onto takes the hero through it.
func (h *hero) walk() {
	r := h.here()
	dx, dy := pad.Dir()
	f := screenFacing[(dx+1)*3+dy+1]
	if f < 0 {
		h.walkTick = 0
		return
	}
	h.facing = (f + cam.eighths()) & 7
	h.walkTick++
	v := speed
	if dx != 0 && dy != 0 {
		v = diag
	}
	mx := ((dx*cam.right.X-dy*cam.ahead.X)*v + 2048) >> 12
	mz := ((dx*cam.right.Z-dy*cam.ahead.Z)*v + 2048) >> 12
	if mx != 0 && h.fits(stage.Vec{X: h.pos.X + mx, Z: h.pos.Z}, abs(mx)) {
		h.pos.X += mx
	}
	if mz != 0 && h.fits(stage.Vec{X: h.pos.X, Z: h.pos.Z + mz}, abs(mz)) {
		h.pos.Z += mz
	}
	h.height = set.HeightAt(h.pos) >> 4
	if c := r.At(h.at()); stage.KindOf(c) == stage.Door {
		if !h.onDoor {
			h.enter(c)
		}
		h.onDoor = true
	} else {
		h.onDoor = false
	}
}

// fits reports whether the hero can step to a position, in sixteenths,
// moving v sixteenths to get there: the four corners of their footprint
// are all on floor, the ground there is within the step's climb, and
// nobody is standing there, nor a chest.
func (h *hero) fits(to stage.Vec, v int) bool {
	r := h.here()
	if abs(set.HeightAt(to)-set.HeightAt(h.pos))*speed > maxStep*v {
		return false
	}
	p := to.Units()
	for k := range r.npcs {
		if p.Within(r.Middle(r.npcs[k].i, r.npcs[k].j), 2*margin, 2*margin) {
			return false
		}
	}
	for k := range r.chests {
		if p.Within(r.Middle(r.chests[k].i, r.chests[k].j), stage.ChestHalfW+margin, stage.ChestHalfD+margin) {
			return false
		}
	}
	return r.Walkable(stage.Vec{X: p.X - margin, Z: p.Z - margin}) && r.Walkable(stage.Vec{X: p.X + margin, Z: p.Z - margin}) &&
		r.Walkable(stage.Vec{X: p.X - margin, Z: p.Z + margin}) && r.Walkable(stage.Vec{X: p.X + margin, Z: p.Z + margin})
}

// enter takes the hero through doorway c to the room its link names,
// standing on the floor just inside that room's doorway with the same
// digit and facing into the room, and blanks the screen for a few
// frames. A doorway without a link, or whose room is not among rooms,
// leads nowhere.
func (h *hero) enter(c byte) {
	path := world.Link(c)
	n := -1
	for k := range rooms {
		if rooms[k].file.Name == path {
			n = k
		}
	}
	if n < 0 {
		return
	}
	r := &rooms[n]
	h.room = n
	r.load()
	screen.Background = r.background()
	cam.slant(world.Room())
	for j := range r.Rows() {
		for i := range r.Cols() {
			if r.Cell(i, j) != c {
				continue
			}
			for _, s := range &stage.Sides {
				in := stage.KindOf(r.Cell(i+s.DI, j+s.DJ))
				if !in.Walkable() || in == stage.Door {
					continue
				}
				h.pos = r.Middle(i+s.DI, j+s.DJ).Sixteenths()
				h.height = set.HeightAt(h.pos) >> 4
				h.facing = s.Facing
				h.onDoor = false
				blank = blankTime
				return
			}
		}
	}
}

// put stands the hero on a cell of the room, facing away from the
// camera.
func (h *hero) put(i, j int) {
	r := h.here()
	h.pos = r.Middle(i, j).Sixteenths()
	h.height = set.HeightAt(h.pos) >> 4
	h.onDoor = false
}

// visit puts the hero on a cell of the room whose file is at path, or of
// the room they are in when path is empty, going through a doorway's
// black frames to another room: the playground's Play from here.
func (h *hero) visit(path string, i, j int) {
	if path != "" && path != h.here().file.Name {
		n := -1
		for k := range rooms {
			if rooms[k].file.Name == path {
				n = k
			}
		}
		if n < 0 {
			return
		}
		h.room = n
		r := &rooms[n]
		r.load()
		screen.Background = r.background()
		cam.slant(world.Room())
		blank = blankTime
	}
	r := h.here()
	h.put(max(0, min(r.Cols()-1, i)), max(0, min(r.Rows()-1, j)))
}

// project works out where the hero's feet are on the screen, and how
// deep, before the room is drawn.
func (h *hero) project() {
	h.otz = 0
	if sx, sy, otz, ok := h.figure.draw(nil, h.at(), h.height, 0, 0); ok {
		h.sx, h.sy, h.otz = sx, sy, otz
	}
}

// draw draws the hero where they stand, facing their way, in the frame
// of the walk they are at.
func (h *hero) draw(ot *yaroze.GsOT) {
	frame := 0
	if h.walkTick > 0 {
		frame = (h.walkTick/stepTime + 1) & 3
	}
	h.figure.draw(ot, h.at(), h.height, h.facing, frame)
}

// interact looks at what is in front of the hero and speaks to it: a
// person within reach of them, the way they face, says their line, and a
// chest there is opened and gives up what is in it, once. The way the
// hero faces is taken loosely, as a half-turn either side, so that
// standing a little off to the side of something still counts.
func (h *hero) interact() {
	r := h.here()
	at, dir := h.at(), dirs[h.facing]
	ahead := func(p stage.Vec) bool {
		return p.Within(at, reach, reach) && p.Sub(at).Dot(dir) > 0
	}
	for k := range r.npcs {
		n := &r.npcs[k]
		if ahead(r.Middle(n.i, n.j)) {
			talk.say(n.say)
			return
		}
	}
	for k := range r.chests {
		c := &r.chests[k]
		if !ahead(r.Middle(c.i, c.j)) {
			continue
		}
		if c.opened {
			talk.say("The chest is empty.")
		} else {
			c.opened = true
			set.Open(c.i, c.j)
			talk.say(c.text)
		}
		return
	}
}

// drawNPCs draws the room's people, each on their cell, facing their way,
// or the hero when the hero is within three tiles of them.
func drawNPCs(ot *yaroze.GsOT) {
	r := nora.here()
	for k := range r.npcs {
		n := &r.npcs[k]
		p := r.Middle(n.i, n.j)
		d := nora.at().Sub(p)
		f := n.facing
		if abs(d.X) < 3*stage.Tile && abs(d.Z) < 3*stage.Tile {
			f = toward(d)
		}
		people.draw(ot, p, set.HeightAt(p.Sixteenths())>>4, f, 0)
	}
}

// toward is the way, in eighths of a turn from +Z clockwise, that points
// most nearly along d.
func toward(d stage.Vec) int {
	ax, az := abs(d.X), abs(d.Z)
	switch {
	case az*5 > ax*12:
		if d.Z > 0 {
			return 0
		}
		return 4
	case ax*5 > az*12:
		if d.X > 0 {
			return 2
		}
		return 6
	case d.X > 0 && d.Z > 0:
		return 1
	case d.X > 0:
		return 3
	case d.Z < 0:
		return 5
	}
	return 7
}
