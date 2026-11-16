//go:build !yaroze

package yaroze

import (
	"testing"

	"go.chrisrx.dev/psx/geom"
	"go.chrisrx.dev/psx/gpu"
	"go.chrisrx.dev/x/assert"
)

// TestAPIStubPanics checks that calling the library on a host fails at
// once, with a message that names the call and says where the real one
// is, rather than returning a zero or dereferencing nothing.
func TestAPIStubPanics(t *testing.T) {
	var screen Screen
	var sprite Sprite
	for name, call := range map[string]func(){
		"Screen.Open":  func() { screen.Open() },
		"Sprite.Draw":  func() { sprite.Draw(nil, 0) },
		"LoadTIM":      func() { LoadTIM(nil) },
		"OpenBank":     func() { OpenBank([]uint32{1}, []uint32{1}) },
		"Random":       func() { Random() },
		"Text.Print":   func() { Text{}.Print("x") },
		"Screen.Wait":  func() { screen.Wait(1) },
		"Screen.Close": func() { screen.Close() },
	} {
		assert.Panic(t, stub(name), call, name)
	}
}

// TestZeroSound checks that a bank or sequence that never opened does
// nothing, which is what lets a program carry on without its music.
func TestZeroSound(t *testing.T) {
	var bank Bank
	var song Sequence
	song.Play(1)
	song.Loop()
	song.SetVolume(1, 1)
	song.Pause()
	song.Resume()
	song.Stop()
	song.Close()
	bank.Close()
	assert.Equal(t, false, song.Playing())
	_, err := bank.Sequence([]uint32{1})
	assert.Equal(t, ErrBank, err)
	_, err = OpenBank(nil, nil)
	assert.Equal(t, ErrEmpty, err)
}

func TestDecimal(t *testing.T) {
	for n, want := range map[int]string{0: "0", 7: "7", 10: "10", 12345: "12345", -1: "-1", -32768: "-32768", -1 << 31: "-2147483648"} {
		var buf [20]byte
		assert.Equal(t, want, string(decimal(&buf, n)))
	}
}

// TestImageSprite checks what a sprite takes from an image: its size,
// its texture with its depth, its own colors and a pivot at its middle.
func TestImageSprite(t *testing.T) {
	im := Image{
		depth: 1,
		w:     44,
		h:     24,
		texture: Texture{
			Page: 1<<7 | 0x15,
			CX:   320,
			CY:   480,
		},
	}
	s := im.Sprite()
	s.pack()
	assert.Equal(t, uint32(1<<24), s.attribute, "8 bits a pixel")
	assert.Equal(t, uint16(44), s.W)
	assert.Equal(t, uint16(24), s.H)
	assert.Equal(t, im.texture, s.Texture)
	gray := Color{
		R: 128,
		G: 128,
		B: 128,
	}
	assert.Equal(t, gray, s.Color)
	assert.Equal(t, geom.Point{X: 22, Y: 12}, s.Pivot)
	assert.Equal(t, geom.One, s.ScaleX)
	assert.Equal(t, 44, im.Width())
}

// TestAttribute checks the attribute word Draw packs for the library:
// the blend, the lighting and the color depth, the same for a sprite
// turned or not, and nothing for a hidden one, which is never sorted.
func TestAttribute(t *testing.T) {
	sp := Sprite{
		Texture: Texture{Page: 2 << 7},
		ScaleX:  geom.One,
		ScaleY:  geom.One,
		Blend:   BlendSubtract,
		Unlit:   true,
	}
	const want = 1<<30 | 2<<28 | 2<<24 | 1<<6
	assert.Panic(t, stub("Sprite.Draw"), func() { sp.Draw(nil, 0) })
	assert.Equal(t, uint32(want), sp.attribute)
	sp.attribute, sp.Rotate = 0, geom.Degrees(10)
	assert.Panic(t, stub("Sprite.Draw"), func() { sp.Draw(nil, 0) })
	assert.Equal(t, uint32(want), sp.attribute, "turned")
	sp.attribute = 0
	assert.Panic(t, stub("Sprite.DrawFast"), func() { sp.DrawFast(nil, 0) })
	assert.Equal(t, uint32(want), sp.attribute, "fast")
	sp.Hidden = true
	sp.Draw(nil, 0)
	sp.DrawFast(nil, 0)

	box := Box{Blend: BlendAverage}
	assert.Panic(t, stub("Box.Draw"), func() { box.Draw(nil, 0) })
	assert.Equal(t, uint32(1<<30), box.attribute)
	box = Box{Blend: BlendQuarter}
	assert.Panic(t, stub("Box.Draw"), func() { box.Draw(nil, 0) })
	assert.Equal(t, uint32(1<<30|3<<28), box.attribute)
	line := Line{Hidden: true}
	line.Draw(nil, 0)
}

// TestImageCell checks a cell cut from an image: its corner from the
// image's, its page, and its palette as the library numbers palettes.
func TestImageCell(t *testing.T) {
	im := Image{depth: 1, w: 128, h: 32, texture: Texture{Page: 0x15, U: 0, V: 0, CX: 320, CY: 509}}
	c := im.Cell(16, 16)
	assert.Equal(t, Cell{U: 16, V: 16, Clut: 509<<6 | 320>>4, Page: 0x15}, c)
}

// TestPacketMode checks the drawing mode word a texture gives for a
// blend, and that the stub's frame has no packet space.
func TestPacketMode(t *testing.T) {
	tex := Texture{Page: 0x15 | 2<<5}
	assert.Equal(t, uint32(0xe1000020|0x15), tex.DrawMode(BlendAdd), "the add blend on the page")
	assert.Equal(t, uint32(0xe1000060|0x15), tex.DrawMode(BlendQuarter))
	assert.Equal(t, tex.DrawMode(BlendAverage), tex.DrawMode(BlendNone), "no blend leaves the first")
	var frame Frame
	assert.Panic(t, stub("Frame.Packet"), func() { frame.Packet[gpu.Tile](0) })
}

// TestVerticesOfWords checks a file's words read as vertices, two words
// each, and that an image knows its file's length.
func TestVerticesOfWords(t *testing.T) {
	words := []uint32{0x00020001, 0x00000003, 0xfffffffe, 0x0000fffd, 0x55}
	v := Vertices(words)
	assert.Equal(t, 2, len(v), "two vertices and a word over")
	assert.Equal(t, Vertex{X: 1, Y: 2, Z: 3}, v[0])
	assert.Equal(t, Vertex{X: -2, Y: -1, Z: -3}, v[1])
	assert.Equal(t, 0, len(Vertices(nil)))
	assert.Equal(t, 1234, Image{words: 1234}.Len())
	var o Object
	var m geom.Matrix
	assert.Panic(t, stub("Object.Transform"), func() { o.Transform(&m, &m) })
	assert.Panic(t, stub("SetReverb"), func() { SetReverb(ReverbHall, 1, 1) })
	assert.Panic(t, stub("CopyRect"), func() { CopyRect(geom.Rect{}, 0, 0) })
}

// TestFogParams checks the depth cue against the ones two examples set
// by hand: clone's, from 1225 to 6125 with the screen 280 away, and
// dino's, from 25000 and never complete with the screen 1000 away.
func TestFogParams(t *testing.T) {
	dqa, dqb := fogParams(1225, 6125, 280)
	assert.Equal(t, int16(-1400), dqa)
	assert.Equal(t, int32(20971520), dqb)
	dqa, dqb = fogParams(25000, 0, 1000)
	assert.Equal(t, int16(-6400), dqa)
	assert.Equal(t, int32(16777216), dqb)
	assert.Panic(t, stub("SetFog"), func() { SetFog(Fog{Far: 100}) })
	assert.Panic(t, stub("Model.Vertices"), func() { Model{0, 0, 1}.Vertices(0) })
}

// TestBackgroundStub checks that a background's drawing is for the
// console alone, and that Init fails there rather than after it.
func TestBackgroundStub(t *testing.T) {
	var screen Screen
	var bg Background
	m := TileMap{Width: 1, Height: 1, Cells: []Cell{{}}, Layout: []uint16{0}}
	assert.Panic(t, stub("Background.Init"), func() { bg.Init(&screen, &m) })
	assert.Panic(t, stub("Line.Draw"), func() { (&Line{}).Draw(nil, 0) })
}
