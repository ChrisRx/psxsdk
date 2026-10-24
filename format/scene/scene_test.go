package scene

import (
	"encoding/json"
	"testing"

	"go.chrisrx.dev/x/assert"
)

func sample() *Scene {
	return &Scene{
		Camera:     Camera{From: [3]int32{0, -200, -1000}, At: [3]int32{0, 0, 0}, Twist: 5},
		Projection: 320,
		Ambient:    [3]int32{1024, 1024, 1024},
		Lights:     []Light{{Direction: [3]int32{1, 1, 1}, Color: [3]uint8{0xc0, 0xc0, 0xc0}}},
		Textures:   []string{"DATA/CUBE.TIM"},
		Models:     []string{"DATA/CUBE.TMD", "DATA/SHIP.TMD"},
		Objects: []Object{
			{Name: "cube", Model: "DATA/CUBE.TMD", Position: [3]int32{0, 0, 0}, Rotation: [3]int16{0, 512, 0}, Scale: [3]int32{4096, 4096, 4096}},
			{Name: "ship", Model: "DATA/SHIP.TMD", Index: 1, Position: [3]int32{-300, 28, 400}, Rotation: [3]int16{-100, 0, 2048}, Fields: [2]int32{7, -3}},
		},
		Sprites: []Sprite{
			{Name: "badge", Texture: "DATA/CUBE.TIM", X: 10, Y: 20, W: 64, H: 64, TPage: 0xa, CX: 0, CY: 480, R: 128, G: 128, B: 128, ScaleX: 4096, ScaleY: 4096, Rotate: 90 * 4096},
		},
	}
}

func withBackground(s *Scene) *Scene {
	s.Textures = append(s.Textures, "DATA/BG.TIM")
	s.Background = &Background{
		Texture: "DATA/BG.TIM", Cell: 16, Width: 3, Height: 3,
		Map:        []uint16{7, 7, 7, 7, EmptyCell, 7, 2, 2, 2},
		Scroll:     [2]int16{4, -8},
		Window:     [4]int16{-160, -120, 320, 240},
		Brightness: [3]uint8{128, 100, 90},
	}
	return s
}

// withLayers gives the sample a far layer and a near one with flags, as a
// 2D game's are.
func withLayers(s *Scene) *Scene {
	s.Textures = append(s.Textures, "DATA/FAR.TIM", "DATA/TILES.TIM")
	s.Layers = []Layer{
		{
			Name: "far", Texture: "DATA/FAR.TIM", Cell: 16, Width: 4, Height: 2,
			Map:        []uint16{0, 1, 2, 3, 4, EmptyCell, 4, 5},
			Scroll:     [2]int16{0, 8},
			Brightness: [3]uint8{128, 128, 128},
			Speed:      50,
		},
		{
			Name: "ground", Texture: "DATA/TILES.TIM", Cell: 16, Width: 3, Height: 3,
			Map:        []uint16{EmptyCell, 5, EmptyCell, 3, 3, 3, 0, 1, 0},
			Window:     [4]int16{-160, -120, 320, 240},
			Brightness: [3]uint8{128, 100, 90},
			Speed:      100,
			Flags:      []int{Solid, Solid, 0, OneWay, Hurts, Pickup | 1<<6},
		},
	}
	return s
}

func TestRoundTrip(t *testing.T) {
	s := sample()
	b, err := s.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, len(b), s.Size())
	assert.Equal(t, len(b), HeaderSize+3*PathSize+2*ObjectSize+SpriteSize)
	back, err := Decode(b)
	assert.NoError(t, err)
	assert.Equal(t, s, back)

	s = withLayers(sample())
	b, err = s.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, len(b), s.Size())
	far := LayerSize + 16 + LayerNameSize
	ground := LayerSize + 20 + LayerNameSize + 8
	assert.Equal(t, len(b), HeaderSize+5*PathSize+2*ObjectSize+SpriteSize+far+ground)
	back, err = Decode(b)
	assert.NoError(t, err)
	assert.Equal(t, s, back)
}

// An older scene.json's background is written as the first layer, named
// "background", moving with the scene's scroll.
func TestBackgroundAsLayer(t *testing.T) {
	s := withBackground(sample())
	b, err := s.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, len(b), s.Size())
	back, err := Decode(b)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(back.Layers))
	assert.Equal(t, (*Background)(nil), back.Background)
	l := back.Layers[0]
	bg := s.Background
	assert.Equal(t, Layer{
		Name: "background", Texture: bg.Texture, Cell: bg.Cell, Width: bg.Width, Height: bg.Height, Map: bg.Map,
		Scroll: bg.Scroll, Window: bg.Window, Brightness: bg.Brightness, Speed: 100,
	}, l)
}

// A file cooked before layers had one, with no name or flags after its map
// and its speed's word zero: it reads as a layer named "background" that
// moves with the scroll.
func TestDecodeBeforeLayers(t *testing.T) {
	s := withBackground(sample())
	b, err := s.Bytes()
	assert.NoError(t, err)
	at := HeaderSize + 4*PathSize + 2*ObjectSize + SpriteSize
	old := append([]byte{}, b[:at+LayerSize+mapBytes(9)]...)
	old[at+6], old[at+7] = 0, 0 // the speed was a pad
	old[at+27] = 0              // no name or flags follow
	back, err := Decode(old)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(back.Layers))
	assert.Equal(t, "background", back.Layers[0].Name)
	assert.Equal(t, 100, back.Layers[0].Speed)
	assert.Equal(t, s.Background.Map, back.Layers[0].Map)
	assert.Equal(t, 0, len(back.Layers[0].Flags))
}

// withRoom gives the sample a room, as the quest example's are.
func withRoom(s *Scene) *Scene {
	s.Room = &Room{
		Floor: 1, Wall: 2, Tint: [3]uint8{128, 100, 90},
		Cells:   []string{"#1##", "#..#", "#2##"},
		Heights: []string{"0000", "0120", "0000"},
		Rough:   true, RockTall: 128, RockLean: 36, CamUp: 560, CamBack: 300,
		Sky:        [2][3]uint8{{30, 70, 190}, {170, 210, 245}},
		Background: [3]uint8{1, 1, 1},
		Links:      []Link{{Door: 1, Scene: "ASSETS/HALL.DAT"}, {Door: 2, Scene: "ASSETS/CAVE.DAT"}},
		Markers:    []Marker{{Name: "chest", Cell: [2]int{1, 1}, Fields: [2]int32{4, -1}}, {Name: "host", Cell: [2]int{2, 1}}},
	}
	return s
}

func TestRoundTripRoom(t *testing.T) {
	s := withRoom(withLayers(sample()))
	b, err := s.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, len(b), s.Size())
	layers := LayerSize + 16 + LayerNameSize + LayerSize + 20 + LayerNameSize + 8
	assert.Equal(t, len(b), HeaderSize+5*PathSize+2*ObjectSize+SpriteSize+layers+RoomSize+12+12+2*LinkSize+2*MarkerSize)
	back, err := Decode(b)
	assert.NoError(t, err)
	assert.Equal(t, s, back)

	// A room alone, level, is the least a scene can be.
	s = &Scene{Room: &Room{Cells: []string{"..."}}}
	b, err = s.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, HeaderSize+RoomSize+4, len(b))
	back, err = Decode(b)
	assert.NoError(t, err)
	assert.Equal(t, s.Room, back.Room)
}

func TestCheckRoom(t *testing.T) {
	for _, bad := range []*Room{
		{Cells: []string{}},
		{Cells: []string{"...", ".."}},
		{Cells: []string{"..."}, Heights: []string{"00"}},
		{Cells: []string{"..."}, Heights: []string{"0a0"}},
		{Cells: []string{"..."}, Links: []Link{{Door: 10, Scene: "X"}}},
		{Cells: []string{"..."}, Links: []Link{{Door: 1}}},
		{Cells: []string{"..."}, Markers: []Marker{{Name: "off", Cell: [2]int{3, 0}}}},
		{Cells: []string{"..."}, Markers: []Marker{{Name: "a name far too long"}}},
	} {
		_, err := (&Scene{Room: bad}).Bytes()
		assert.Equal(t, true, err != nil, "%v", bad)
	}
}

func TestCheckLayers(t *testing.T) {
	for name, spoil := range map[string]func(s *Scene){
		"short map":       func(s *Scene) { s.Layers[1].Map = s.Layers[1].Map[:8] },
		"8-pixel cells":   func(s *Scene) { s.Layers[0].Cell = 8 },
		"unknown texture": func(s *Scene) { s.Layers[0].Texture = "DATA/NONE.TIM" },
		"long name":       func(s *Scene) { s.Layers[0].Name = "a name far too long" },
		"same names":      func(s *Scene) { s.Layers[1].Name = "far" },
		"four layers": func(s *Scene) {
			s.Layers = append(s.Layers, s.Layers[0], s.Layers[1])
			s.Layers[2].Name, s.Layers[3].Name = "c", "d"
		},
		"too many cells": func(s *Scene) { s.Layers[0].Width, s.Layers[0].Height = 65, 64 },
		"cell past 255":  func(s *Scene) { s.Layers[0].Map[0] = 256 },
		"flag past 255":  func(s *Scene) { s.Layers[1].Flags[0] = 256 },
		"257 flags":      func(s *Scene) { s.Layers[1].Flags = make([]int, 257) },
		"backward speed": func(s *Scene) { s.Layers[0].Speed = -1 },
	} {
		s := withLayers(sample())
		spoil(s)
		_, err := s.Bytes()
		assert.Equal(t, true, err != nil, "%s", name)
	}
	// A long strip of 4096 cells is as good as a square of them.
	s := withLayers(sample())
	s.Layers[0].Width, s.Layers[0].Height = 256, 16
	s.Layers[0].Map = make([]uint16, 256*16)
	_, err := s.Bytes()
	assert.NoError(t, err)
}

func TestJSON(t *testing.T) {
	s := withLayers(sample())
	j, err := json.Marshal(s)
	assert.NoError(t, err)
	var back Scene
	assert.NoError(t, json.Unmarshal(j, &back))
	assert.Equal(t, s, &back)
}

func TestCheck(t *testing.T) {
	s := sample()
	s.Objects[0].Model = "DATA/NONE.TMD"
	_, err := s.Bytes()
	assert.Equal(t, true, err != nil)

	s = sample()
	s.Sprites[0].Name = "a name that is far too long"
	_, err = s.Bytes()
	assert.Equal(t, true, err != nil)

	s = sample()
	s.Lights = append(s.Lights, Light{}, Light{}, Light{})
	_, err = s.Bytes()
	assert.Equal(t, true, err != nil)
}

func TestDecodeShort(t *testing.T) {
	s := sample()
	b, _ := s.Bytes()
	_, err := Decode(b[:len(b)-1])
	assert.Equal(t, true, err != nil)
	_, err = Decode(b[:8])
	assert.Equal(t, true, err != nil)
	b[0] = 0
	_, err = Decode(b)
	assert.Equal(t, true, err != nil)
}
