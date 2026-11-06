// One of everything the exp/ps API draws, on a picture that stands still:
// the library's own font in all its glyphs, boxes, lines, sprites plain,
// scaled, turned and blended, a tile map, the packet types, a lit and
// fogged model, Kanji text, and the overlay.
//
// It is the API's own regression picture, checked by TestGalleryScreenshotPS
// in tests/examples_test.go against a picture of its own: the other programs
// under exp/examples are checked against the originals' pictures.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/exp/ps"
)

var (
	cubeTIM = disc.Register("ASSETS/CUBE.TIM", "../../../examples/square/assets/CUBE.TIM")
	cubeTMD = disc.Register("ASSETS/CUBE.TMD", "../../../examples/square/assets/CUBE.TMD")
	ballTIM = disc.Register("ASSETS/BALL.TIM", "../../../examples/timers/assets/BALL.TIM")

	screen = ps.Screen{
		Background:   ps.Color{R: 24, G: 28, B: 48},
		Depth:        8,
		Packets:      32 << 10,
		Center:       true,
		OverlayDepth: 2,
	}
	bounds ps.Rect
	text   ps.Text
	kanji  ps.KanjiText

	image   ps.Image
	sprites [5]ps.Sprite

	boxes = [3]ps.Box{
		{W: 40, H: 24, Color: ps.Color{R: 220, G: 80, B: 60}},
		{W: 40, H: 24, Mode: ps.Translucent, Color: ps.Color{R: 60, G: 220, B: 80}},
		{W: 40, H: 24, Mode: ps.Translucent | ps.Add, Color: ps.Color{R: 60, G: 80, B: 220}},
	}
	lines = [3]ps.Line{
		{Color: ps.Color{R: 255, G: 255, B: 255}},
		{Color: ps.Color{R: 255, G: 220, B: 80}},
		{Color: ps.Color{R: 80, G: 220, B: 255}},
	}
	gradient = ps.GradientLine{From: ps.Color{R: 255, G: 0, B: 0}, To: ps.Color{R: 0, G: 0, B: 255}}

	// The backdrop: a map of the ball and the empty cell beside it in the
	// ball's texture, which the background tiles across the whole screen.
	cells  [2]ps.Cell
	layout = [6]uint16{0, 1, 1, 1, 0, 1}
	tiles  = ps.TileMap{Width: 3, Height: 2, Cells: cells[:], Layout: layout[:]}
	tilebg = ps.Background{W: 320, H: 240}

	cube ps.Object

	panel = ps.Box{W: 60, H: 14, Color: ps.Color{R: 250, G: 250, B: 250}}
)

// The Shift-JIS for こんにちは, and the ASCII the font takes as well.
const greeting = "\x82\xb1\x82\xf1\x82\xc9\x82\xbf\x82\xcd PSX"

func main() {
	screen.Open()
	bounds = screen.Bounds()
	b := bounds
	text = screen.Font(b.X+8, b.Y+4, 304, 40, 400)
	kanji = screen.KanjiFont(b.X+8, b.Y+196, 200, 20, 768, 256, 768, 504, 16)

	if f := disc.LoadAll(300); f != nil {
		for range screen.Frames() {
			text.Print("could not read ").Print(f.Name)
		}
	}
	setup()

	for frame := range screen.Frames() {
		text.Print(" !\"#$%&'()*+,-./0123456789:;<=>?\n")
		text.Print("@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_\n")
		text.Print("lower case ~c900red ~c090green ~c009blue~c999\n")
		kanji.Print(greeting)

		tilebg.Draw(frame, 250)
		for i := range sprites {
			sprites[i].Draw(frame, 200)
		}
		sprites[4].DrawFast(frame, 200)
		for i := range boxes {
			boxes[i].Draw(frame, 100)
		}
		for i := range lines {
			lines[i].Draw(frame, 50)
		}
		gradient.Draw(frame, 50)
		packets(frame)

		cube.Rotation = ps.Rotation{X: 400, Y: 600, Z: 200}
		cube.Draw(frame)

		panel.Draw(frame.Overlay(), 0)
	}
}

// setup places everything once the assets are read.
func setup() {
	b := bounds
	image = ps.LoadTIM(cubeTIM.Data())
	plain := image.Sprite()
	for i := range sprites {
		sprites[i] = plain
	}
	sprites[0].MoveTo(b.X+8, b.Y+44)
	sprites[1].MoveTo(b.X+76, b.Y+44)
	sprites[1].ScaleX, sprites[1].ScaleY = ps.One/2, ps.One/2
	sprites[2].MoveTo(b.X+144, b.Y+76)
	sprites[2].Pivot = ps.Point{X: 32, Y: 32}
	sprites[2].Rotate = ps.Degrees(45)
	sprites[3].MoveTo(b.X+180, b.Y+44)
	sprites[3].Color = ps.Color{R: 255, G: 64, B: 64}
	sprites[3].Mode |= ps.Translucent
	sprites[4].MoveTo(b.X+248, b.Y+44)
	sprites[4].Mode |= ps.Upright

	for i := range boxes {
		boxes[i].MoveTo(b.X+8+i*20, b.Y+96+i*8)
	}
	for i := range lines {
		l := &lines[i]
		l.SetEnds(b.X+90, b.Y+100+i*10, b.X+150, b.Y+120-i*10)
	}
	gradient.SetEnds(b.X+90, b.Y+136, b.X+150, b.Y+136)

	// The ball's texture goes beside the cube's, since both files place
	// theirs at the same spot; the map's cells are the ball and the empty
	// 16 by 16 beside it.
	ball := ps.LoadTIMAt(ballTIM.Data(), 704, 0)
	cells[0] = ball.Cell(0, 0)
	cells[1] = ball.Cell(16, 0)
	tilebg.MoveTo(b.X, b.Y)
	tilebg.Init(&screen, &tiles)

	model := ps.LoadTMD(cubeTMD.Data())
	cube.Init(nil)
	cube.Link(model, 0)
	cube.Position = ps.Vector{X: -190, Y: 125}
	camera := ps.Camera{Eye: ps.Vector{Z: -1000}, Distance: 320}
	camera.Set()
	ps.SetLight(0, ps.Light{Direction: ps.Vector{X: 1, Y: 1, Z: 1}, Color: ps.Color{R: 0xc0, G: 0xc0, B: 0xc0}})
	ps.SetAmbient(ps.One/4, ps.One/4, ps.One/4)
	ps.SetFog(ps.Fog{Near: 600, Far: 1600, Color: ps.Color{R: 200, G: 200, B: 255}})

	panel.MoveTo(b.X+250, b.Y+220)
}

// packets draws one of each packet type through the frame's allocator.
func packets(frame *ps.Frame) {
	b := bounds
	x, y := b.X+170, b.Y+150
	t := image.Texture()
	clut := uint16(t.CY)<<6 | uint16(t.CX)>>4

	f3 := frame.Packet[ps.PolyF3](60)
	f3.Color = ps.Color{R: 240, G: 200, B: 40}
	f3.XY0, f3.XY1, f3.XY2 = ps.At(x, y+30), ps.At(x+15, y), ps.At(x+30, y+30)

	g4 := frame.Packet[ps.PolyG4](60)
	g4.Color0, g4.Color1, g4.Color2, g4.Color3 = ps.Color{R: 255}, ps.Color{G: 255}, ps.Color{B: 255}, ps.Color{R: 255, G: 255, B: 255}
	g4.XY0, g4.XY1 = ps.At(x+36, y), ps.At(x+66, y)
	g4.XY2, g4.XY3 = ps.At(x+36, y+30), ps.At(x+66, y+30)

	ft4 := frame.Packet[ps.PolyFT4](60)
	ft4.Color = ps.Color{R: 128, G: 128, B: 128}
	ft4.Clut, ft4.Page = clut, t.Page
	ft4.XY0, ft4.UV0 = ps.At(x+72, y), ps.UV{U: t.U, V: t.V}
	ft4.XY1, ft4.UV1 = ps.At(x+112, y+6), ps.UV{U: t.U + 63, V: t.V}
	ft4.XY2, ft4.UV2 = ps.At(x+72, y+30), ps.UV{U: t.U, V: t.V + 63}
	ft4.XY3, ft4.UV3 = ps.At(x+112, y+36), ps.UV{U: t.U + 63, V: t.V + 63}

	g2 := frame.Packet[ps.LineG2](60)
	g2.Color0, g2.Color1 = ps.Color{R: 255, G: 255}, ps.Color{G: 255, B: 255}
	g2.XY0, g2.XY1 = ps.At(x, y+36), ps.At(x+66, y+44)

	tile := frame.Packet[ps.Tile](60)
	tile.Color = ps.Color{R: 90, G: 200, B: 160}
	tile.XY, tile.W, tile.H = ps.At(x, y+50), 30, 12

	// A sprite packet takes its page from the drawing mode before it,
	// which is linked after it at the same depth so that it is drawn
	// first: at one depth, the packet linked last is drawn first.
	sprt := frame.Packet[ps.Sprt](60)
	sprt.Color = ps.Color{R: 128, G: 128, B: 128}
	sprt.XY, sprt.UV, sprt.Clut = ps.At(x+36, y+50), ps.UV{U: t.U, V: t.V}, clut
	sprt.W, sprt.H = 32, 16
	mode := frame.Packet[ps.DrawMode](60)
	mode.Page = t.Mode(0)
}
