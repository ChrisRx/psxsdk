package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// An objFile is what obj2tmd reads of a Wavefront OBJ file: positions,
// texture coordinates and normals, and faces grouped into objects.
type objFile struct {
	positions [][3]float64
	uvs       [][2]float64
	normals   [][3]float64
	objects   []*objObject
	materials map[string]*material
}

type objObject struct {
	name  string
	faces []face
}

// A face is a polygon's corners, each indices into the file's lists, -1
// where the corner has none, and the material it was given.
type face struct {
	corners  []corner
	material string
}

type corner struct{ v, vt, vn int }

// A material is what a face takes from an MTL file: its diffuse colour and
// texture.
type material struct {
	color   [3]float64
	texture string // the map_Kd path, relative to the MTL file
	dir     string
}

func readOBJ(path string) (*objFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	o := &objFile{materials: map[string]*material{}}
	cur := &objObject{name: "default"}
	o.objects = append(o.objects, cur)
	mtl := ""
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		fields := strings.Fields(strings.SplitN(sc.Text(), "#", 2)[0])
		if len(fields) == 0 {
			continue
		}
		fail := func(format string, args ...any) error {
			return fmt.Errorf("%s:%d: %s", path, line, fmt.Sprintf(format, args...))
		}
		floats := func(n int) ([]float64, error) {
			if len(fields) < n+1 {
				return nil, fail("%s wants %d numbers", fields[0], n)
			}
			out := make([]float64, n)
			for i := range out {
				v, err := strconv.ParseFloat(fields[1+i], 64)
				if err != nil {
					return nil, fail("%q is not a number", fields[1+i])
				}
				out[i] = v
			}
			return out, nil
		}
		switch fields[0] {
		case "v":
			v, err := floats(3)
			if err != nil {
				return nil, err
			}
			o.positions = append(o.positions, [3]float64{v[0], v[1], v[2]})
		case "vt":
			v, err := floats(2)
			if err != nil {
				return nil, err
			}
			o.uvs = append(o.uvs, [2]float64{v[0], v[1]})
		case "vn":
			v, err := floats(3)
			if err != nil {
				return nil, err
			}
			o.normals = append(o.normals, [3]float64{v[0], v[1], v[2]})
		case "f":
			if len(fields) < 4 {
				return nil, fail("a face of %d corners", len(fields)-1)
			}
			fc := face{material: mtl}
			for _, c := range fields[1:] {
				cn, err := o.corner(c)
				if err != nil {
					return nil, fail("%v", err)
				}
				fc.corners = append(fc.corners, cn)
			}
			cur.faces = append(cur.faces, fc)
		case "o":
			name := strings.Join(fields[1:], " ")
			if len(cur.faces) == 0 {
				cur.name = name
			} else {
				cur = &objObject{name: name}
				o.objects = append(o.objects, cur)
			}
		case "usemtl":
			if len(fields) > 1 {
				mtl = fields[1]
			}
		case "mtllib":
			for _, name := range fields[1:] {
				if err := o.readMTL(filepath.Join(filepath.Dir(path), name)); err != nil {
					return nil, err
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(o.objects[0].faces) == 0 && len(o.objects) > 1 {
		o.objects = o.objects[1:]
	}
	return o, nil
}

// corner reads "v", "v/vt", "v//vn" or "v/vt/vn", with indices from 1, or
// from the end when negative.
func (o *objFile) corner(s string) (corner, error) {
	parts := strings.Split(s, "/")
	c := corner{-1, -1, -1}
	idx := func(p string, n int) (int, error) {
		if p == "" {
			return -1, nil
		}
		i, err := strconv.Atoi(p)
		if err != nil {
			return 0, fmt.Errorf("%q is not an index", p)
		}
		if i < 0 {
			i += n + 1
		}
		if i < 1 || i > n {
			return 0, fmt.Errorf("index %s is outside the %d defined", p, n)
		}
		return i - 1, nil
	}
	var err error
	if c.v, err = idx(parts[0], len(o.positions)); err != nil || c.v < 0 {
		return c, fmt.Errorf("corner %q: %v", s, err)
	}
	if len(parts) > 1 {
		if c.vt, err = idx(parts[1], len(o.uvs)); err != nil {
			return c, fmt.Errorf("corner %q: %v", s, err)
		}
	}
	if len(parts) > 2 {
		if c.vn, err = idx(parts[2], len(o.normals)); err != nil {
			return c, fmt.Errorf("corner %q: %v", s, err)
		}
	}
	return c, nil
}

func (o *objFile) readMTL(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return o.parseMTL(f, filepath.Dir(path))
}

func (o *objFile) parseMTL(r io.Reader, dir string) error {
	var cur *material
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(strings.SplitN(sc.Text(), "#", 2)[0])
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "newmtl":
			cur = &material{color: [3]float64{1, 1, 1}, dir: dir}
			if len(fields) > 1 {
				o.materials[fields[1]] = cur
			}
		case "Kd":
			if cur != nil && len(fields) >= 4 {
				for i := range 3 {
					cur.color[i], _ = strconv.ParseFloat(fields[1+i], 64)
				}
			}
		case "map_Kd":
			if cur != nil && len(fields) >= 2 {
				// Options such as -s come first; the path is last.
				cur.texture = fields[len(fields)-1]
			}
		}
	}
	return sc.Err()
}
