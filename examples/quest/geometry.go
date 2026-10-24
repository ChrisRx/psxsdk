package main

import (
	"go.chrisrx.dev/psx/stage"
	"go.chrisrx.dev/psx/yaroze"
)

// The ground is package stage's X-Z plane, and most of what moves on it
// is a stage.Vec, a point or a step on it; the units are whatever the
// use says, world units mostly, sixteenths of one for the hero's
// position, and 4096ths for the camera's directions. Up is -Y, as the
// console has it.

// xy packs a screen point as a GPU packet holds one.
//
//go:inline
func xy(x, y int16) uint32 { return uint32(uint16(x)) | uint32(uint16(y))<<16 }

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
	sprite yaroze.GsSPRITE
	feet   yaroze.SVECTOR
	sxy    uint32
}

const (
	frameW  = 32
	frameH  = 48
	feetRow = 38
)

// people is the sheet the people in a room are drawn from.
var people figure

// cut takes the sheet from an image in video memory.
func (f *figure) cut(im yaroze.GsIMAGE) {
	f.sprite = im.Sprite()
	f.sprite.W, f.sprite.H = frameW, frameH
	f.sprite.Mx, f.sprite.My = frameW/2, feetRow
}

// draw projects the figure's feet, at p on the ground and y above it, to
// the screen through the camera's world-to-screen matrix, puts the
// sprite's feet there, picks the column that faces the figure's way as
// seen from the camera and the row of the walk, and sorts it where
// stage puts a figure standing at the feet's depth, in front of the
// floor it stands on and the wall it may be against. The depth is the
// GTE's average of the point three times over, so that it is on the
// same scale as the depths the polygons are sorted at. It returns the
// feet's screen point and depth, or ok false when the point is off the
// screen.
func (f *figure) draw(ot *yaroze.GsOT, p stage.Vec, y, facing, frame int) (sx, sy, otz int, ok bool) {
	f.feet = yaroze.SVECTOR{Vx: int16(p.X), Vy: int16(-y), Vz: int16(p.Z)}
	yaroze.GsSetLsMatrix(yaroze.GsWSMATRIX())
	if yaroze.RotTransPers(&f.feet, &f.sxy) < 0 {
		return 0, 0, 0, false
	}
	yaroze.RotTransPers(&f.feet, &f.sxy)
	yaroze.RotTransPers(&f.feet, &f.sxy)
	otz = int(yaroze.AverageZ3())
	sx, sy = int(int16(f.sxy)), int(int16(f.sxy>>16))
	if ot != nil {
		f.sprite.X, f.sprite.Y = int16(sx), int16(sy)
		f.sprite.U = uint8(frameW * ((facing - cam.eighths()) & 7))
		f.sprite.V = uint8(frameH * frame)
		f.sprite.Sort(ot, stage.Standing(otz))
	}
	return sx, sy, otz, true
}
