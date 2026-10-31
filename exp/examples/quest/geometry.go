package main

import (
	"go.chrisrx.dev/psx/exp/ps"
	"go.chrisrx.dev/psx/exp/ps/stage"
)

// The ground is package stage's X-Z plane, and most of what moves on it
// is a stage.Vec, a point or a step on it; the units are whatever the
// use says, world units mostly, sixteenths of one for the hero's
// position, and 4096ths for the camera's directions. Up is -Y, as the
// console has it.

//go:inline
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// A figure is a sprite sheet of someone standing on the ground: a column
// of frames for each of the eight ways to face, from up the screen
// clockwise, and a row for each frame of the walk, each frameW by
// frameH with the feet on row feetRow.
type figure struct {
	sprite ps.Sprite
	feet   ps.Vertex
	sxy    ps.XY
}

const (
	frameW  = 32
	frameH  = 48
	feetRow = 38
)

// people is the sheet the people in a room are drawn from.
var people figure

// cut takes the sheet from an image in video memory.
func (f *figure) cut(im ps.Image) {
	f.sprite = im.Sprite()
	f.sprite.W, f.sprite.H = frameW, frameH
	f.sprite.Pivot = ps.Point{X: frameW / 2, Y: feetRow}
}

// draw projects the figure's feet, at p on the ground and y above it, to
// the screen through the camera's world-to-screen matrix, puts the
// sprite's feet there, picks the column that faces the figure's way as
// seen from the camera and the row of the walk, and draws it where
// stage puts a figure standing at the feet's depth, in front of the
// floor it stands on and the wall it may be against. The depth is the
// GTE's average of the point three times over, so that it is on the
// same scale as the depths the polygons are sorted at. It returns the
// feet's screen point and depth, or ok false when the point is off the
// screen.
func (f *figure) draw(frame *ps.Frame, p stage.Vec, y, facing, step int) (sx, sy, otz int, ok bool) {
	f.feet = ps.Vertex{X: int16(p.X), Y: int16(-y), Z: int16(p.Z)}
	ps.UseCamera()
	if !ps.Project(&f.feet, &f.sxy) {
		return 0, 0, 0, false
	}
	ps.Project(&f.feet, &f.sxy)
	ps.Project(&f.feet, &f.sxy)
	otz = ps.AverageDepth3()
	sx, sy = int(f.sxy.X), int(f.sxy.Y)
	if frame != nil {
		f.sprite.MoveTo(sx, sy)
		f.sprite.Texture.U = uint8(frameW * ((facing - cam.eighths()) & 7))
		f.sprite.Texture.V = uint8(frameH * step)
		f.sprite.Draw(frame, stage.Standing(otz))
	}
	return sx, sy, otz, true
}
