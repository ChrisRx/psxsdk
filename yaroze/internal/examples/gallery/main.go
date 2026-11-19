// One of everything the yaroze package's Go API draws, on a picture that
// stands still: the library's own font in all its glyphs, boxes, lines,
// sprites plain, scaled, turned and blended, a tile map, the packet types, a
// lit and fogged model, Kanji text, and the overlay.
//
// It is the API's own regression picture, checked by TestGalleryScreenshotPS
// in tests/examples_test.go against a picture of its own: the other programs
// under yaroze/internal/examples are checked against the originals' pictures.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/geom"
	"go.chrisrx.dev/psx/gpu"
	"go.chrisrx.dev/psx/yaroze"
)

var (
	cubeTIM = disc.Register("ASSETS/CUBE.TIM", "../../../../examples/square/assets/CUBE.TIM")
	cubeTMD = disc.Register("ASSETS/CUBE.TMD", "../../../../examples/square/assets/CUBE.TMD")
	ballTIM = disc.Register("ASSETS/BALL.TIM", "../../../../examples/timers/assets/BALL.TIM")

	screen = yaroze.Screen{
		Background:   yaroze.Color{R: 24, G: 28, B: 48},
		Depth:        8,
		Packets:      32 << 10,
		Origin:       yaroze.ScreenOriginMiddle,
		OverlayDepth: 2,
	}
	bounds geom.Rect
	text   yaroze.Text
	kanji  yaroze.KanjiText

	image   yaroze.Image
	sprites [5]yaroze.Sprite

	boxes = [3]yaroze.Box{
		{
			W:     40,
			H:     24,
			Color: yaroze.Color{R: 220, G: 80, B: 60},
		},
		{
			W:     40,
			H:     24,
			Color: yaroze.Color{R: 60, G: 220, B: 80},
			Blend: yaroze.BlendAverage,
		},
		{
			W:     40,
			H:     24,
			Color: yaroze.Color{R: 60, G: 80, B: 220},
			Blend: yaroze.BlendAdd,
		},
	}
	lines = [3]yaroze.Line{
		{Color: yaroze.Color{R: 255, G: 255, B: 255}},
		{Color: yaroze.Color{R: 255, G: 220, B: 80}},
		{Color: yaroze.Color{R: 80, G: 220, B: 255}},
	}
	gradient = yaroze.GradientLine{
		From: yaroze.Color{R: 255, G: 0, B: 0},
		To:   yaroze.Color{R: 0, G: 0, B: 255},
	}

	// The backdrop: a map of the ball and the empty cell beside it in the
	// ball's texture, which the background tiles across the whole screen.
	cells  [2]yaroze.Cell
	layout = [6]uint16{0, 1, 1, 1, 0, 1}
	tiles  = yaroze.TileMap{
		Width:  3,
		Height: 2,
		Cells:  cells[:],
		Layout: layout[:],
	}
	tilebg = yaroze.Background{
		W: 320,
		H: 240,
	}

	cube yaroze.Object

	panel = yaroze.Box{
		W:     60,
		H:     14,
		Color: yaroze.Color{R: 250, G: 250, B: 250},
	}
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

		cube.Rotation = geom.Rotation{X: 400, Y: 600, Z: 200}
		cube.Draw(frame)

		panel.Draw(frame.Overlay(), 0)
	}
}

// setup places everything once the assets are read.
func setup() {
	b := bounds
	image = yaroze.LoadTIM(cubeTIM.Data())
	plain := image.Sprite()
	for i := range sprites {
		sprites[i] = plain
	}
	sprites[0].MoveTo(b.X+8, b.Y+44)
	sprites[1].MoveTo(b.X+76, b.Y+44)
	sprites[1].ScaleX, sprites[1].ScaleY = geom.One/2, geom.One/2
	sprites[2].MoveTo(b.X+144, b.Y+76)
	sprites[2].Pivot = geom.Point{X: 32, Y: 32}
	sprites[2].Rotate = geom.Degrees(45)
	sprites[3].MoveTo(b.X+180, b.Y+44)
	sprites[3].Color = yaroze.Color{R: 255, G: 64, B: 64}
	sprites[3].Blend = yaroze.BlendAverage
	sprites[4].MoveTo(b.X+248, b.Y+44)

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
	ball := yaroze.LoadTIMAt(ballTIM.Data(), 704, 0)
	cells[0] = ball.Cell(0, 0)
	cells[1] = ball.Cell(16, 0)
	tilebg.MoveTo(b.X, b.Y)
	tilebg.Init(&screen, &tiles)

	model := yaroze.LoadTMD(cubeTMD.Data())
	cube.Init(nil)
	cube.Link(model, 0)
	cube.Position = geom.Vector{X: -190, Y: 125}
	camera := yaroze.Camera{
		Eye:      geom.Vector{Z: -1000},
		Distance: 320,
	}
	camera.Set()
	yaroze.SetLight(0, yaroze.Light{
		Direction: geom.Vector{X: 1, Y: 1, Z: 1},
		Color:     yaroze.Color{R: 0xc0, G: 0xc0, B: 0xc0},
	})
	yaroze.SetAmbient(geom.One/4, geom.One/4, geom.One/4)
	yaroze.SetFog(yaroze.Fog{
		Near:  600,
		Far:   1600,
		Color: yaroze.Color{R: 200, G: 200, B: 255},
	})

	panel.MoveTo(b.X+250, b.Y+220)
}

// packets draws one of each packet type through the frame's allocator.
func packets(frame *yaroze.Frame) {
	b := bounds
	x, y := b.X+170, b.Y+150
	t := image.Texture()
	clut := uint16(t.CY)<<6 | uint16(t.CX)>>4

	f3 := frame.Packet[gpu.FlatTriangle](60)
	f3.Color = yaroze.Color{R: 240, G: 200, B: 40}
	f3.XY0, f3.XY1, f3.XY2 = gpu.At(x, y+30), gpu.At(x+15, y), gpu.At(x+30, y+30)

	g4 := frame.Packet[gpu.ShadedQuad](60)
	g4.Color0, g4.Color1, g4.Color2, g4.Color3 = yaroze.Color{R: 255}, yaroze.Color{G: 255}, yaroze.Color{B: 255}, yaroze.Color{R: 255, G: 255, B: 255}
	g4.XY0, g4.XY1 = gpu.At(x+36, y), gpu.At(x+66, y)
	g4.XY2, g4.XY3 = gpu.At(x+36, y+30), gpu.At(x+66, y+30)

	ft4 := frame.Packet[gpu.TexturedQuad](60)
	ft4.Color = yaroze.Color{R: 128, G: 128, B: 128}
	ft4.Clut, ft4.Page = clut, t.Page
	ft4.XY0, ft4.UV0 = gpu.At(x+72, y), gpu.UV{U: t.U, V: t.V}
	ft4.XY1, ft4.UV1 = gpu.At(x+112, y+6), gpu.UV{U: t.U + 63, V: t.V}
	ft4.XY2, ft4.UV2 = gpu.At(x+72, y+30), gpu.UV{U: t.U, V: t.V + 63}
	ft4.XY3, ft4.UV3 = gpu.At(x+112, y+36), gpu.UV{U: t.U + 63, V: t.V + 63}

	g2 := frame.Packet[gpu.ShadedLine](60)
	g2.Color0, g2.Color1 = yaroze.Color{R: 255, G: 255}, yaroze.Color{G: 255, B: 255}
	g2.XY0, g2.XY1 = gpu.At(x, y+36), gpu.At(x+66, y+44)

	tile := frame.Packet[gpu.Tile](60)
	tile.Color = yaroze.Color{R: 90, G: 200, B: 160}
	tile.XY, tile.W, tile.H = gpu.At(x, y+50), 30, 12

	// A sprite packet takes its page from the drawing mode before it,
	// which is linked after it at the same depth so that it is drawn
	// first: at one depth, the packet linked last is drawn first.
	sprt := frame.Packet[gpu.Sprite](60)
	sprt.Color = yaroze.Color{R: 128, G: 128, B: 128}
	sprt.XY, sprt.UV, sprt.Clut = gpu.At(x+36, y+50), gpu.UV{U: t.U, V: t.V}, clut
	sprt.W, sprt.H = 32, 16
	mode := frame.Packet[gpu.DrawMode](60)
	mode.Page = t.DrawMode(yaroze.BlendNone)
}
