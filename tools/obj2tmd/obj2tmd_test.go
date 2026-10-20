package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/format/tmd"
)

// TestCube writes the square example's cube, which the example draws the
// right way out, as an OBJ file the way a modelling program would, and
// converts it back: the faces must come out with the same corners, in the
// same order, with the same normals and texture coordinates, which is to
// say obj2tmd turns the axes and the winding the right way.
func TestCube(t *testing.T) {
	data := "../../examples/square/assets"
	b, err := os.ReadFile(filepath.Join(data, "CUBE.TMD"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	want, err := tmd.Decode(b)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	cube := want.Objects[0]

	dir := t.TempDir()
	tex, err := os.ReadFile(filepath.Join(data, "CUBE.TIM"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "cube.TIM"), tex, 0o644))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "cube.mtl"), []byte("newmtl stone\nKd 1 1 1\nmap_Kd cube.png\n"), 0o644))

	// The OBJ's axes are Y up and Z towards the camera; its faces go
	// anticlockwise from outside, a ring where the TMD's quads are a
	// strip: strip a, b, c, d is the clockwise ring a, b, d, c, which
	// reversed is a, c, d, b.
	var obj strings.Builder
	obj.WriteString("mtllib cube.mtl\no cube\nusemtl stone\n")
	for _, v := range cube.Vertices {
		fmt.Fprintf(&obj, "v %d %d %d\n", v.X, -v.Y, -v.Z)
	}
	uvs := [4][2]float64{{0, 1}, {63.0 / 64, 1}, {0, 1 - 63.0/64}, {63.0 / 64, 1 - 63.0/64}}
	for _, uv := range uvs {
		fmt.Fprintf(&obj, "vt %g %g\n", uv[0], uv[1])
	}
	for _, p := range cube.Primitives {
		// A lit flat textured quad: three UV words, a fourth, then the
		// normal and vertices.
		v := [4]uint32{p.Packet[4] >> 16, p.Packet[5] & 0xffff, p.Packet[5] >> 16, p.Packet[6] & 0xffff}
		ring := [4]int{0, 2, 3, 1}
		obj.WriteString("f")
		for _, k := range ring {
			fmt.Fprintf(&obj, " %d/%d", v[k]+1, k+1)
		}
		obj.WriteString("\n")
	}
	path := filepath.Join(dir, "cube.obj")
	assert.NoError(t, os.WriteFile(path, []byte(obj.String()), 0o644))

	opts.Scale, opts.Output = 1, filepath.Join(dir, "out.TMD")
	defer func() { opts.Scale, opts.Output = 100, "" }()
	if !assert.NoError(t, run(nil, []string{path})) {
		t.FailNow()
	}
	b, err = os.ReadFile(opts.Output)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	got, err := tmd.Decode(b)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// The vertices and normals may be numbered differently, so compare
	// each face by what its indices point at.
	type face struct {
		corners [4]tmd.Vector
		normal  tmd.Vector
		uvs     [4]uint32
	}
	faces := func(o tmd.Object) []face {
		var out []face
		for _, p := range o.Primitives {
			f := face{normal: o.Normals[p.Packet[4]&0xffff]}
			for k, i := range [4]uint32{p.Packet[4] >> 16, p.Packet[5] & 0xffff, p.Packet[5] >> 16, p.Packet[6] & 0xffff} {
				f.corners[k] = o.Vertices[i]
				f.uvs[k] = p.Packet[k]
			}
			out = append(out, f)
		}
		return out
	}
	assert.Equal(t, faces(cube), faces(got.Objects[0]))
	for i := range cube.Primitives {
		assert.Equal(t, cube.Primitives[i].Mode, got.Objects[0].Primitives[i].Mode)
	}
}
