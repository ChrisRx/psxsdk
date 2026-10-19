package tmd

import (
	"os"
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestRoundTrip(t *testing.T) {
	b, err := os.ReadFile("../../examples/square/data/CUBE.TMD")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	m, err := Decode(b)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, 1, len(m.Objects))
	assert.Equal(t, 8, len(m.Objects[0].Vertices))
	assert.Equal(t, 6, len(m.Objects[0].Primitives))
	assert.Equal(t, b, m.Bytes())
}

func TestPolygon(t *testing.T) {
	// The square example's cube is six flat textured quads, lit, which
	// Polygon should lay out as the cube's file has them.
	b, err := os.ReadFile("../../examples/square/data/CUBE.TMD")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	m, err := Decode(b)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	faces := [6][4]uint16{{0, 1, 3, 2}, {5, 4, 6, 7}, {4, 0, 7, 3}, {1, 5, 2, 6}, {4, 5, 0, 1}, {3, 2, 7, 6}}
	for i, f := range faces {
		p, err := Polygon{
			Vertices: f[:],
			Normals:  []uint16{uint16(i)},
			Texture:  &Texture{TPage: 0x000a, CBA: 0x7800, UV: [4][2]uint8{{0, 0}, {63, 0}, {0, 63}, {63, 63}}},
		}.Primitive()
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		assert.Equal(t, m.Objects[0].Primitives[i], p)
	}
}

func TestLayouts(t *testing.T) {
	// The sizes in Sony's tables: mode, flag, ilen, olen.
	red := RGB{R: 255}
	tex := &Texture{}
	for _, c := range []struct {
		p                      Polygon
		mode, flag, ilen, olen int
	}{
		{Polygon{Vertices: []uint16{0, 1, 2}, Normals: []uint16{0}, Colors: []RGB{red}}, 0x20, 0, 3, 4},
		{Polygon{Vertices: []uint16{0, 1, 2}, Normals: []uint16{0}, Colors: []RGB{red, red, red}}, 0x20, 4, 5, 6},
		{Polygon{Vertices: []uint16{0, 1, 2}, Normals: []uint16{0, 1, 2}, Colors: []RGB{red}}, 0x30, 0, 4, 6},
		{Polygon{Vertices: []uint16{0, 1, 2}, Normals: []uint16{0, 1, 2}, Colors: []RGB{red, red, red}}, 0x30, 4, 6, 6},
		{Polygon{Vertices: []uint16{0, 1, 2}, Normals: []uint16{0}, Texture: tex}, 0x24, 0, 5, 7},
		{Polygon{Vertices: []uint16{0, 1, 2}, Normals: []uint16{0, 1, 2}, Texture: tex}, 0x34, 0, 6, 9},
		{Polygon{Vertices: []uint16{0, 1, 2}, Colors: []RGB{red}}, 0x21, 1, 3, 4},
		{Polygon{Vertices: []uint16{0, 1, 2}, Colors: []RGB{red, red, red}}, 0x31, 1, 5, 6},
		{Polygon{Vertices: []uint16{0, 1, 2}, Colors: []RGB{red}, Texture: tex}, 0x25, 1, 6, 7},
		{Polygon{Vertices: []uint16{0, 1, 2}, Colors: []RGB{red, red, red}, Texture: tex}, 0x35, 1, 8, 9},
		{Polygon{Vertices: []uint16{0, 1, 2, 3}, Normals: []uint16{0}, Colors: []RGB{red}}, 0x28, 0, 4, 5},
		{Polygon{Vertices: []uint16{0, 1, 2, 3}, Normals: []uint16{0}, Colors: []RGB{red, red, red, red}}, 0x28, 4, 7, 8},
		{Polygon{Vertices: []uint16{0, 1, 2, 3}, Normals: []uint16{0, 1, 2, 3}, Colors: []RGB{red}}, 0x38, 0, 5, 8},
		{Polygon{Vertices: []uint16{0, 1, 2, 3}, Normals: []uint16{0, 1, 2, 3}, Colors: []RGB{red, red, red, red}}, 0x38, 4, 8, 8},
		{Polygon{Vertices: []uint16{0, 1, 2, 3}, Normals: []uint16{0}, Texture: tex}, 0x2c, 0, 7, 9},
		{Polygon{Vertices: []uint16{0, 1, 2, 3}, Normals: []uint16{0, 1, 2, 3}, Texture: tex}, 0x3c, 0, 8, 12},
		{Polygon{Vertices: []uint16{0, 1, 2, 3}, Colors: []RGB{red}}, 0x29, 1, 3, 5},
		{Polygon{Vertices: []uint16{0, 1, 2, 3}, Colors: []RGB{red, red, red, red}}, 0x39, 1, 6, 8},
		{Polygon{Vertices: []uint16{0, 1, 2, 3}, Colors: []RGB{red}, Texture: tex}, 0x2d, 1, 7, 9},
		{Polygon{Vertices: []uint16{0, 1, 2, 3}, Colors: []RGB{red, red, red, red}, Texture: tex}, 0x3d, 1, 10, 12},
	} {
		p, err := c.p.Primitive()
		if !assert.NoError(t, err) {
			continue
		}
		assert.Equal(t, [4]int{c.mode, c.flag, c.ilen, c.olen}, [4]int{int(p.Mode), int(p.Flag), len(p.Packet), int(p.Olen)})
	}
}
