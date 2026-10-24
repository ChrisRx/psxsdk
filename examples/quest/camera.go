package main

import (
	"go.chrisrx.dev/psx/scene"
	"go.chrisrx.dev/psx/stage"
	"go.chrisrx.dev/psx/yaroze"
)

// The camera sits behind and above the hero, at a fixed slant, and turns
// about them an eighth of a turn at a time on L1 and R1, taking eight
// frames over each turn: square to the room's walls at the even steps
// and along its diagonals at the odd, where it starts.
const (
	camBack    = 440
	camUp      = 380
	turnStep   = 64
	camSteps   = 8
	camStart   = 1
	projection = 320
)

type camera struct {
	// dir is the step the camera has turned to, 0 to camSteps-1, target
	// the angle that is, angle its angle now, 4096 to a turn, and turn
	// how far it has still to turn to reach target.
	dir    int
	target int
	angle  int
	turn   int

	// From target: which way the world is from the camera, ahead of it,
	// up the screen, and to its right, across the screen, in 4096ths.
	ahead stage.Vec
	right stage.Vec

	// The library's view, and working storage for the matrices, kept
	// here so that handing their addresses to the library does not
	// allocate.
	view   yaroze.GsRVIEW2
	rot    yaroze.SVECTOR
	mat    yaroze.MATRIX
	dirRot yaroze.SVECTOR
	dirMat yaroze.MATRIX
	off    yaroze.VECTOR
	pos    yaroze.VECTOR
}

var cam = camera{
	dir:    camStart,
	target: camStart * 4096 / camSteps,
	angle:  camStart * 4096 / camSteps,
	ahead:  stage.Vec{X: 0, Z: 4096},
	right:  stage.Vec{X: 4096, Z: 0},
	off:    yaroze.VECTOR{Vy: -camUp, Vz: -camBack},
}

// update takes L1 and R1, when input is wanted, as a step round each,
// which can queue up, turns the camera towards where it is to be,
// and works out which way is ahead and right from there: the rotation
// about Y that puts the camera behind the hero has the sine and cosine
// of the angle in its first row, and ahead is (sin, cos), right (cos,
// -sin).
func (c *camera) update(input bool) {
	const each = 4096 / camSteps
	if input && pad.Pressed(yaroze.PADL1) {
		c.dir = (c.dir + 1) % camSteps
		c.turn += each
	}
	if input && pad.Pressed(yaroze.PADR1) {
		c.dir = (c.dir + camSteps - 1) % camSteps
		c.turn -= each
	}
	c.target = c.dir * each
	c.dirRot.Vy = int16(c.target)
	c.dirMat.Rotate(&c.dirRot)
	s, co := int(c.dirMat.M[0][2]), int(c.dirMat.M[0][0])
	c.ahead = stage.Vec{X: s, Z: co}
	c.right = stage.Vec{X: co, Z: -s}
	step := min(max(c.turn, -turnStep), turnStep)
	c.angle = (c.angle + step) & 4095
	c.turn -= step
}

// slant puts the camera up above the hero and back behind them, the
// room's own slant or the usual one.
func (c *camera) slant(r *scene.Room) {
	up, back := r.CamUp, r.CamBack
	if up == 0 {
		up, back = camUp, camBack
	}
	c.off = yaroze.VECTOR{Vy: int32(-up), Vz: int32(-back)}
}

// eighths is the camera's target as eighths of a turn, which is what a
// figure's facing is counted in.
//
//go:inline
func (c *camera) eighths() int { return c.target >> 9 }

// place puts the camera behind and above the hero, turned about them by
// its angle, looking at a point a little above their feet so that the
// figure is in the middle of the screen.
func (c *camera) place(at stage.Vec, height int) {
	c.rot.Vy = int16(c.angle)
	c.mat.Rotate(&c.rot)
	yaroze.ApplyMatrixLV(&c.mat, &c.off, &c.pos)
	c.view.Vrx, c.view.Vry, c.view.Vrz = int32(at.X), int32(-20-height), int32(at.Z)
	c.view.Vpx, c.view.Vpy, c.view.Vpz = int32(at.X)+c.pos.Vx, c.pos.Vy-int32(height), int32(at.Z)+c.pos.Vz
	c.view.Set()
}
