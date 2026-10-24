package main

import (
	"go.chrisrx.dev/psx/stage"
	"go.chrisrx.dev/psx/yaroze"
)

// The sky outdoors: a gradient over the whole screen, and in front of it,
// far enough back to be behind the ground, a backdrop of clouds and hills
// that turns with the camera as anything far away would, and a sun at a
// fixed point of the turn. The backdrop's 256 pixels are a quarter of
// the camera's turn, so a full turn is 1024 and the camera's angle over 4
// is the scroll; the clouds drift one pixel every cloudDrift frames; and
// the sun stands sunAngle of the turn round from where the camera first
// looks.
const (
	backdropW  = 256
	cloudDrift = 6
	sunAngle   = 3700
)

type backdrop struct {
	// The clouds and the hills, strips of the backdrop that wrap across
	// the screen, the sun, and the land under the hills, down to the
	// bottom of the screen, for where the ground stops short of it.
	clouds, hills, sun yaroze.GsSPRITE
	land               yaroze.GsBOXF
}

var sky backdrop

// setup cuts the strips and the sun from their images and sizes the
// land to the screen.
func (s *backdrop) setup(b yaroze.RECT) {
	s.sun = yaroze.TIM(sunTIM.Data()).Load().Sprite()
	s.clouds = yaroze.TIM(skyTIM.Data()).Load().Sprite()
	s.clouds.H = 32
	s.hills = s.clouds
	s.hills.V += 32
	s.land = yaroze.GsBOXF{W: uint16(b.W), H: uint16(b.H - 68), R: 50, G: 110, B: 80}
}

// draw fills the screen with the room's sky, shaded from its stage.Color at
// the top to its stage.Color at the bottom, as one shaded quad in the deepest
// entry of the table, sorted after everything else there so that it is
// drawn before it all; and over it the sun, the clouds and the hills.
// The ground covers them where it reaches the top of the screen. A room
// without sky colors has no sky, and the screen's own background shows.
func (s *backdrop) draw(ot *yaroze.GsOT, b yaroze.RECT) {
	colors := world.Room().Sky
	if colors == ([2]stage.Color{}) {
		return
	}
	const deepest = 1<<stage.Depth - 1
	scroll := cam.angle >> 2
	// The sun's place along the turn, from the middle of the screen, as
	// a signed distance, drawn when it is on the screen.
	sx := int16((sunAngle-cam.angle+2048)&4095-2048) >> 2
	if sx > -200 && sx < 200 {
		s.sun.X, s.sun.Y = sx-16, b.Y+16
		s.sun.SortFast(ot, deepest-1)
	}
	s.strip(ot, &s.clouds, scroll+screen.Frame()/cloudDrift, b, b.Y, deepest-2)
	s.strip(ot, &s.hills, scroll, b, b.Y+36, deepest-3)
	s.land.X, s.land.Y = b.X, b.Y+68
	s.land.Sort(ot, deepest-3)

	top, bottom := colors[0].Word(0x38), colors[1].Word(0)
	p := yaroze.AllocPacket(9)
	p[0] = 8 << 24
	p[1] = top
	p[2] = xy(b.X, b.Y)
	p[3] = top &^ 0xff000000
	p[4] = xy(b.X+b.W, b.Y)
	p[5] = bottom
	p[6] = xy(b.X, b.Y+b.H)
	p[7] = bottom
	p[8] = xy(b.X+b.W, b.Y+b.H)
	ot.Link(deepest, p)
}

// strip draws a strip of the backdrop across the screen at y, scrolled
// left by scroll and wrapped, as many times as it takes to cover the
// screen's width.
func (s *backdrop) strip(ot *yaroze.GsOT, sp *yaroze.GsSPRITE, scroll int, b yaroze.RECT, y int16, pri uint16) {
	x := b.X - int16(scroll&(backdropW-1))
	for ; x < b.X+b.W; x += backdropW {
		sp.X, sp.Y = x, y
		sp.SortFast(ot, pri)
	}
}
