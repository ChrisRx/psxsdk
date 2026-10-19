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

func TestRoundTrip(t *testing.T) {
	s := sample()
	b, err := s.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, len(b), s.Size())
	assert.Equal(t, len(b), HeaderSize+3*PathSize+2*ObjectSize+SpriteSize)
	back, err := Decode(b)
	assert.NoError(t, err)
	assert.Equal(t, s, back)

	s = withBackground(sample())
	b, err = s.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, len(b), s.Size())
	assert.Equal(t, len(b), HeaderSize+4*PathSize+2*ObjectSize+SpriteSize+BackgroundSize+20)
	back, err = Decode(b)
	assert.NoError(t, err)
	assert.Equal(t, s, back)
}

func TestCheckBackground(t *testing.T) {
	s := withBackground(sample())
	s.Background.Map = s.Background.Map[:8]
	_, err := s.Bytes()
	assert.Equal(t, true, err != nil)

	s = withBackground(sample())
	s.Background.Cell = 8
	_, err = s.Bytes()
	assert.Equal(t, true, err != nil)

	s = withBackground(sample())
	s.Background.Texture = "DATA/NONE.TIM"
	_, err = s.Bytes()
	assert.Equal(t, true, err != nil)
}

func TestJSON(t *testing.T) {
	s := sample()
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
