package main

import (
	"go.chrisrx.dev/psx/geom"
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
	clouds, hills, sun yaroze.Sprite
	land               yaroze.Box
}

var sky backdrop

// setup cuts the strips and the sun from their images and sizes the
// land to the screen.
func (s *backdrop) setup(b geom.Rect) {
	s.sun = yaroze.LoadTIM(sunTIM.Data()).Sprite()
	s.clouds = yaroze.LoadTIM(skyTIM.Data()).Sprite()
	s.clouds.H = 32
	s.hills = s.clouds
	s.hills.Texture.V += 32
	s.land = yaroze.Box{Color: yaroze.Color{R: 50, G: 110, B: 80}}
	s.land.SetRect(geom.Rect{
		W: b.W,
		H: b.H - 68,
	})
}

// draw fills the screen with the room's sky, shaded from its color at
// the top to its color at the bottom, as one shaded quad at the deepest
// depth, drawn after everything else there so that it is drawn before
// it all; and over it the sun, the clouds and the hills. The ground
// covers them where it reaches the top of the screen. A room without
// sky colors has no sky, and the screen's own background shows.
func (s *backdrop) draw(frame *yaroze.Frame, b geom.Rect) {
	colors := world.Room().Sky
	if colors == ([2]stage.Color{}) {
		return
	}
	deepest := frame.Depths() - 1
	scroll := cam.angle >> 2
	// The sun's place along the turn, from the middle of the screen, as
	// a signed distance, drawn when it is on the screen.
	sx := ((sunAngle-cam.angle+2048)&4095 - 2048) >> 2
	if sx > -200 && sx < 200 {
		s.sun.MoveTo(sx-16, b.Y+16)
		s.sun.DrawFast(frame, deepest-1)
	}
	s.strip(frame, &s.clouds, scroll+frame.Number()/cloudDrift, b, b.Y, deepest-2)
	s.strip(frame, &s.hills, scroll, b, b.Y+36, deepest-3)
	s.land.MoveTo(b.X, b.Y+68)
	s.land.Draw(frame, deepest-3)

	top, bottom := colors[0].RGB(), colors[1].RGB()
	p := frame.Packet[yaroze.PolyG4](deepest)
	p.Color0, p.Color1, p.Color2, p.Color3 = top, top, bottom, bottom
	p.XY0 = yaroze.At(b.X, b.Y)
	p.XY1 = yaroze.At(b.X+b.W, b.Y)
	p.XY2 = yaroze.At(b.X, b.Y+b.H)
	p.XY3 = yaroze.At(b.X+b.W, b.Y+b.H)
}

// strip draws a strip of the backdrop across the screen at y, scrolled
// left by scroll and wrapped, as many times as it takes to cover the
// screen's width.
func (s *backdrop) strip(frame *yaroze.Frame, sp *yaroze.Sprite, scroll int, b geom.Rect, y, depth int) {
	x := b.X - scroll&(backdropW-1)
	for ; x < b.X+b.W; x += backdropW {
		sp.MoveTo(x, y)
		sp.DrawFast(frame, depth)
	}
}
