package main

import (
	"go.chrisrx.dev/psx/exp/ps"
	"go.chrisrx.dev/psx/exp/ps/scene"
	"go.chrisrx.dev/psx/exp/ps/stage"
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
	view   ps.Camera
	mat    ps.Matrix
	dirMat ps.Matrix
	off    ps.Vector
	pos    ps.Vector
}

var cam = camera{
	dir:    camStart,
	target: camStart * 4096 / camSteps,
	angle:  camStart * 4096 / camSteps,
	ahead:  stage.Vec{X: 0, Z: 4096},
	right:  stage.Vec{X: 4096, Z: 0},
	off:    ps.Vector{Y: -camUp, Z: -camBack},
}

// update takes L1 and R1, when input is wanted, as a step round each,
// which can queue up, turns the camera towards where it is to be,
// and works out which way is ahead and right from there: the rotation
// about Y that puts the camera behind the hero has the sine and cosine
// of the angle in its first row, and ahead is (sin, cos), right (cos,
// -sin).
func (c *camera) update(input bool) {
	const each = 4096 / camSteps
	if input && pad.Pressed(ps.L1) {
		c.dir = (c.dir + 1) % camSteps
		c.turn += each
	}
	if input && pad.Pressed(ps.R1) {
		c.dir = (c.dir + camSteps - 1) % camSteps
		c.turn -= each
	}
	c.target = c.dir * each
	c.dirMat.Rotate(ps.Rotation{Y: c.target})
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
	c.off = ps.Vector{Y: -up, Z: -back}
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
	c.mat.Rotate(ps.Rotation{Y: c.angle})
	c.pos = c.mat.Apply(c.off)
	c.view.Target = ps.Vector{
		X: at.X,
		Y: -20 - height,
		Z: at.Z,
	}
	c.view.Eye = ps.Vector{
		X: at.X + c.pos.X,
		Y: c.pos.Y - height,
		Z: at.Z + c.pos.Z,
	}
	c.view.Distance = projection
	c.view.Set()
}
