//go:build !yaroze

package yaroze

import (
	"testing"
	"unsafe"

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
		"PadButtons":   func() { PadButtons(0) },
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

func TestOverlaps(t *testing.T) {
	a := Rect{X: 10, Y: 10, W: 20, H: 20}
	assert.Equal(t, true, a.Overlaps(Rect{X: 29, Y: 29, W: 5, H: 5}), "corner touching inside")
	assert.Equal(t, false, a.Overlaps(Rect{X: 30, Y: 10, W: 5, H: 5}), "edge to edge")
	assert.Equal(t, false, a.Overlaps(Rect{X: 0, Y: 0, W: 10, H: 10}), "apart")
	assert.Equal(t, true, a.Overlaps(Rect{X: 0, Y: 0, W: 100, H: 100}), "inside")
}

func TestButtons(t *testing.T) {
	held := Start | Select | Cross
	assert.Equal(t, true, held.Has(Start|Select))
	assert.Equal(t, false, held.Has(Start|Triangle))
	assert.Equal(t, true, held.Any(Triangle|Cross))
	assert.Equal(t, false, held.Any(Up|Down))
}

func TestDecimal(t *testing.T) {
	for n, want := range map[int]string{0: "0", 7: "7", 10: "10", 12345: "12345", -1: "-1", -32768: "-32768", -1 << 31: "-2147483648"} {
		var buf [20]byte
		assert.Equal(t, want, string(decimal(&buf, n)))
	}
}

// TestImageSprite checks what a sprite takes from an image: its size,
// its texture, its depth in the mode, its own colors and a pivot at its
// middle.
func TestImageSprite(t *testing.T) {
	im := Image{depth: 1, w: 44, h: 24, texture: Texture{Page: 0x15, CX: 320, CY: 480}}
	s := im.Sprite()
	assert.Equal(t, Depth8, s.Mode)
	assert.Equal(t, uint16(44), s.W)
	assert.Equal(t, uint16(24), s.H)
	assert.Equal(t, im.texture, s.Texture)
	assert.Equal(t, Color{128, 128, 128}, s.Color)
	assert.Equal(t, Point{22, 12}, s.Pivot)
	assert.Equal(t, One, s.ScaleX)
	assert.Equal(t, 44, im.Width())
	assert.Equal(t, Angle(90<<12), Degrees(90))
}

// TestImageCell checks a cell cut from an image: its corner from the
// image's, its page, and its palette as the library numbers palettes.
func TestImageCell(t *testing.T) {
	im := Image{depth: 1, w: 128, h: 32, texture: Texture{Page: 0x15, U: 0, V: 0, CX: 320, CY: 509}}
	c := im.Cell(16, 16)
	assert.Equal(t, Cell{U: 16, V: 16, Clut: 509<<6 | 320>>4, Page: 0x15}, c)
}

// TestPad checks a press from a hold: a button acts the frame it
// goes down, not while it stays down, and the directions give an axis.
func TestPad(t *testing.T) {
	var c Pad
	c.update(Start | Right)
	assert.Equal(t, true, c.Pressed(Start))
	assert.Equal(t, true, c.Held(Start))
	c.update(Start | Right | Down)
	assert.Equal(t, false, c.Pressed(Start), "still held")
	assert.Equal(t, true, c.Pressed(Down))
	dx, dy := c.Dir()
	assert.Equal(t, [2]int{1, 1}, [2]int{dx, dy})
	c.update(Left | Right)
	assert.Equal(t, true, c.Released(Start))
	dx, dy = c.Dir()
	assert.Equal(t, [2]int{0, 0}, [2]int{dx, dy}, "both ways cancel")
	x, y := (Rect{X: 0, Y: 0, W: 50, H: 140}).Clamp(-5, 200, 40, 40)
	assert.Equal(t, [2]int{0, 100}, [2]int{x, y}, "clamped into the rectangle")
}

// TestPacketTypes checks each packet type's size in words against the
// GPU's, and the code its allocation gives it.
func TestPacketTypes(t *testing.T) {
	for _, c := range []struct {
		p     any
		words uintptr
		code  Code
	}{
		{&PolyF3{}, 5, 0x20}, {&PolyF4{}, 6, 0x28}, {&PolyFT3{}, 8, 0x24}, {&PolyFT4{}, 10, 0x2c},
		{&PolyG3{}, 7, 0x30}, {&PolyG4{}, 9, 0x38}, {&PolyGT3{}, 10, 0x34}, {&PolyGT4{}, 13, 0x3c},
		{&LineF2{}, 4, 0x40}, {&LineG2{}, 5, 0x50}, {&Tile{}, 4, 0x60}, {&Sprt{}, 5, 0x64},
	} {
		setCode(c.p)
		switch p := c.p.(type) {
		case *PolyF3:
			assert.Equal(t, c.words, unsafe.Sizeof(*p)/4)
			assert.Equal(t, c.code, p.Code)
		case *PolyF4:
			assert.Equal(t, c.words, unsafe.Sizeof(*p)/4)
			assert.Equal(t, c.code, p.Code)
		case *PolyFT3:
			assert.Equal(t, c.words, unsafe.Sizeof(*p)/4)
			assert.Equal(t, c.code, p.Code)
		case *PolyFT4:
			assert.Equal(t, c.words, unsafe.Sizeof(*p)/4)
			assert.Equal(t, c.code, p.Code)
		case *PolyG3:
			assert.Equal(t, c.words, unsafe.Sizeof(*p)/4)
			assert.Equal(t, c.code, p.Code)
		case *PolyG4:
			assert.Equal(t, c.words, unsafe.Sizeof(*p)/4)
			assert.Equal(t, c.code, p.Code)
		case *PolyGT3:
			assert.Equal(t, c.words, unsafe.Sizeof(*p)/4)
			assert.Equal(t, c.code, p.Code)
		case *PolyGT4:
			assert.Equal(t, c.words, unsafe.Sizeof(*p)/4)
			assert.Equal(t, c.code, p.Code)
		case *LineF2:
			assert.Equal(t, c.words, unsafe.Sizeof(*p)/4)
			assert.Equal(t, c.code, p.Code)
		case *LineG2:
			assert.Equal(t, c.words, unsafe.Sizeof(*p)/4)
			assert.Equal(t, c.code, p.Code)
		case *Tile:
			assert.Equal(t, c.words, unsafe.Sizeof(*p)/4)
			assert.Equal(t, c.code, p.Code)
		case *Sprt:
			assert.Equal(t, c.words, unsafe.Sizeof(*p)/4)
			assert.Equal(t, c.code, p.Code)
		}
	}
	var mode DrawMode
	setCode(&mode)
	assert.Equal(t, uintptr(2), unsafe.Sizeof(mode)/4)
	assert.Equal(t, uint32(0xe1000000), mode.Page)
	assert.Equal(t, uint32(0xe1000020|0x15), Texture{Page: 0x15}.Mode(1), "the add blend on the page")
	var frame Frame
	assert.Panic(t, stub("Frame.Packet"), func() { frame.Packet[Tile](0) })
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
	var m Matrix
	assert.Panic(t, stub("Object.Transform"), func() { o.Transform(&m, &m) })
	assert.Panic(t, stub("SetReverb"), func() { SetReverb(ReverbHall, 1, 1) })
	assert.Panic(t, stub("CopyRect"), func() { CopyRect(Rect{}, 0, 0) })
}

// TestStick checks the sticks' dead zone and that a controller without
// sticks reports none.
func TestStick(t *testing.T) {
	c := Pad{Connected: true, Kind: Analog, Axes: [4]int{0xff, 0x80, 0x10, 0x85}}
	x, y := c.Stick(0, 24)
	assert.Equal(t, [2]int{-112, 0}, [2]int{x, y}, "the left stick, its small push in the dead zone")
	x, y = c.Stick(1, 24)
	assert.Equal(t, [2]int{127, 0}, [2]int{x, y}, "the right stick hard right")
	c.Kind = Digital
	x, y = c.Stick(0, 0)
	assert.Equal(t, [2]int{0, 0}, [2]int{x, y}, "no sticks on a digital controller")
	assert.Equal(t, 0, deadZone(0x80, 0), "the center is 0")
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
