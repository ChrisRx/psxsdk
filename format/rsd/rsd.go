// Package rsd reads RSD models, the text format Sony's 3D tools wrote, and
// links them into TMD files as Sony's rsdlink did.
//
// An RSD model is four files, all text, with '#' starting a comment:
//
//	RSD  @RSD940102, then PLY=, MAT= and GRP= naming the other three, and
//	     NTEX= and TEX[n]= naming the TIM files its textures are in
//	PLY  @PLY940102, the counts of vertices, normals and polygons, then
//	     each vertex and each normal as three floats, and each polygon as
//	     a hex flag (0 a triangle, 1 a quad, 2 a line, 3 a sprite) and
//	     eight numbers: four vertex and four normal indices, or for a
//	     sprite its vertex, width and height
//	MAT  @MAT940801 (or @MAT940102), the count of materials, then each
//	     material: the polygons it is for ("5", "0-3", "2,4,6"), a hex
//	     flag (bit 0 not lit, bit 1 double-sided, bit 2 semitransparent,
//	     bits 3-5 its rate), F or G for flat or gouraud shading, and a
//	     type with its values: C r g b, G and a color per vertex, T tex
//	     and a U and V per vertex, D as T then a color, H as T then a
//	     color per vertex
//	GRP  @GRP940102, names for groups of polygons, which the TMD does not
//	     keep
//
// A material's U and V are relative to the top left of its texture's
// image, not of the texture page, and linking adds the image's place in
// the page, read from the TIM.
package rsd

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.chrisrx.dev/psx/format/tim"
	"go.chrisrx.dev/psx/format/tmd"
)

// A Polygon is a PLY polygon with its MAT material.
type Polygon struct {
	Kind     int // 0 triangle, 1 quad, 2 line, 3 sprite
	Vertices [4]int
	Normals  [4]int

	Flag    int  // the material's flag
	Gouraud bool // G shading
	Type    byte // C, G, T, D or H
	Texture int  // TEX number, for T, D and H
	UV      [4][2]int
	Colors  [4][3]int
}

// A Model is an RSD model read from its files.
type Model struct {
	Vertices [][3]float64
	Normals  [][3]float64
	Polygons []Polygon
	Textures []*tim.TIM // TEX[n], read for where they sit in video memory
}

// Read reads the RSD file at path and the files it names. Names are looked
// up in the RSD's directory without regard to case, as DOS wrote them, and
// a texture also in ../TIM, where rsdlink looked.
func Read(path string) (*Model, error) {
	dir := filepath.Dir(path)
	fields, err := keyValues(path)
	if err != nil {
		return nil, err
	}
	m := &Model{}
	ply, err := find(dir, fields["PLY"])
	if err != nil {
		return nil, fmt.Errorf("%s: PLY: %w", path, err)
	}
	if err := m.readPLY(ply); err != nil {
		return nil, err
	}
	if fields["MAT"] != "" {
		mat, err := find(dir, fields["MAT"])
		if err != nil {
			return nil, fmt.Errorf("%s: MAT: %w", path, err)
		}
		if err := m.readMAT(mat); err != nil {
			return nil, err
		}
	}
	ntex, _ := strconv.Atoi(fields["NTEX"])
	for i := range ntex {
		name := fields[fmt.Sprintf("TEX[%d]", i)]
		p, err := find(dir, name)
		if err != nil {
			p, err = find(filepath.Join(dir, "..", "TIM"), name)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: TEX[%d]: %w", path, i, err)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		t, err := tim.Decode(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		m.Textures = append(m.Textures, t)
	}
	return m, nil
}

// find returns the file of that name in dir, whatever its case.
func find(dir, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("no file named")
	}
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range ents {
		if strings.EqualFold(e.Name(), name) {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("%s not found in %s", name, dir)
}

// lines returns a file's lines without comments or blank ones.
func lines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		s := sc.Text()
		// A '#' starts a comment, and DOS ended a file with ^Z.
		if i := strings.IndexAny(s, "#\x1a"); i >= 0 {
			s = s[:i]
		}
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out, sc.Err()
}

func keyValues(path string) (map[string]string, error) {
	ls, err := lines(path)
	if err != nil {
		return nil, err
	}
	if len(ls) == 0 || !strings.HasPrefix(ls[0], "@RSD") {
		return nil, fmt.Errorf("%s: not an RSD file", path)
	}
	m := map[string]string{}
	for _, l := range ls[1:] {
		k, v, ok := strings.Cut(l, "=")
		if ok {
			m[strings.ToUpper(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
	}
	return m, nil
}

func (m *Model) readPLY(path string) error {
	ls, err := lines(path)
	if err != nil {
		return err
	}
	if len(ls) < 2 || !strings.HasPrefix(ls[0], "@PLY") {
		return fmt.Errorf("%s: not a PLY file", path)
	}
	var nv, nn, np int
	if _, err := fmt.Sscan(ls[1], &nv, &nn, &np); err != nil {
		return fmt.Errorf("%s: counts: %w", path, err)
	}
	if len(ls) < 2+nv+nn+np {
		return fmt.Errorf("%s: %d lines for %d vertices, %d normals and %d polygons", path, len(ls)-2, nv, nn, np)
	}
	vec := func(l string) ([3]float64, error) {
		var v [3]float64
		_, err := fmt.Sscan(l, &v[0], &v[1], &v[2])
		return v, err
	}
	i := 2
	for range nv {
		v, err := vec(ls[i])
		if err != nil {
			return fmt.Errorf("%s: vertex %q: %w", path, ls[i], err)
		}
		m.Vertices = append(m.Vertices, v)
		i++
	}
	for range nn {
		v, err := vec(ls[i])
		if err != nil {
			return fmt.Errorf("%s: normal %q: %w", path, ls[i], err)
		}
		m.Normals = append(m.Normals, v)
		i++
	}
	for range np {
		f := strings.Fields(ls[i])
		if len(f) < 9 {
			return fmt.Errorf("%s: polygon %q", path, ls[i])
		}
		flag, err := strconv.ParseInt(f[0], 16, 32)
		if err != nil {
			return fmt.Errorf("%s: polygon flag %q", path, f[0])
		}
		p := Polygon{Kind: int(flag & 3), Type: 'C', Colors: [4][3]int{{128, 128, 128}}}
		for k := range 4 {
			p.Vertices[k], _ = strconv.Atoi(f[1+k])
			p.Normals[k], _ = strconv.Atoi(f[5+k])
		}
		m.Polygons = append(m.Polygons, p)
		i++
	}
	return nil
}

// polygonNumbers expands "5", "0-3" or "2,4,6".
func polygonNumbers(s string) ([]int, error) {
	var out []int
	for _, part := range strings.Split(s, ",") {
		a, b, isRange := strings.Cut(part, "-")
		lo, err := strconv.Atoi(strings.TrimSpace(a))
		if err != nil {
			return nil, fmt.Errorf("polygon number %q", s)
		}
		hi := lo
		if isRange {
			if hi, err = strconv.Atoi(strings.TrimSpace(b)); err != nil {
				return nil, fmt.Errorf("polygon range %q", s)
			}
		}
		for n := lo; n <= hi; n++ {
			out = append(out, n)
		}
	}
	return out, nil
}

func (m *Model) readMAT(path string) error {
	ls, err := lines(path)
	if err != nil {
		return err
	}
	if len(ls) < 2 || !strings.HasPrefix(ls[0], "@MAT") {
		return fmt.Errorf("%s: not a MAT file", path)
	}
	// As many materials as the count says: rsdlink ignored any more.
	count, err := strconv.Atoi(strings.Fields(ls[1])[0])
	if err != nil {
		return fmt.Errorf("%s: count %q", path, ls[1])
	}
	mats := ls[2:]
	if count < len(mats) {
		mats = mats[:count]
	}
	for _, l := range mats {
		f := strings.Fields(l)
		if len(f) < 4 {
			return fmt.Errorf("%s: material %q", path, l)
		}
		polys, err := polygonNumbers(f[0])
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		flag, err := strconv.ParseInt(f[1], 16, 32)
		if err != nil {
			return fmt.Errorf("%s: material flag %q", path, f[1])
		}
		nums := make([]int, 0, len(f)-4)
		for _, s := range f[4:] {
			n, err := strconv.Atoi(s)
			if err != nil {
				return fmt.Errorf("%s: material %q: %q is not a number", path, l, s)
			}
			nums = append(nums, n)
		}
		need := map[byte]int{'C': 3, 'G': 12, 'T': 9, 'D': 12, 'H': 21}
		typ := strings.ToUpper(f[3])[0]
		if n, ok := need[typ]; !ok {
			return fmt.Errorf("%s: material type %q", path, f[3])
		} else if len(nums) < n {
			return fmt.Errorf("%s: material %q has %d numbers, not %d", path, l, len(nums), n)
		}
		for _, pi := range polys {
			if pi < 0 || pi >= len(m.Polygons) {
				return fmt.Errorf("%s: material for polygon %d of %d", path, pi, len(m.Polygons))
			}
			p := &m.Polygons[pi]
			p.Flag = int(flag)
			p.Gouraud = strings.EqualFold(f[2], "G")
			p.Type = typ
			switch typ {
			case 'C':
				p.Colors[0] = [3]int{nums[0], nums[1], nums[2]}
			case 'G':
				for k := range 4 {
					p.Colors[k] = [3]int{nums[3*k], nums[3*k+1], nums[3*k+2]}
				}
			case 'T', 'D', 'H':
				p.Texture = nums[0]
				for k := range 4 {
					p.UV[k] = [2]int{nums[1+2*k], nums[2+2*k]}
				}
				switch typ {
				case 'D':
					p.Colors[0] = [3]int{nums[9], nums[10], nums[11]}
				case 'H':
					for k := range 4 {
						p.Colors[k] = [3]int{nums[9+3*k], nums[10+3*k], nums[11+3*k]}
					}
				}
			}
		}
	}
	return nil
}

// Placement is how a model is scaled and moved as it is linked, rsdlink's
// -s and -t.
type Placement struct {
	Scale     float64 // 0 is 1
	Translate [3]float64
}

// Object links the model into a TMD object: its vertices scaled by the
// power of two nearest the placement's scale, as rsdlink rounded it, then
// moved, and rounded to integers; its normals in 4096ths; and a primitive
// for each polygon.
func (m *Model) Object(pl Placement) (tmd.Object, error) {
	exp := 0
	if pl.Scale > 0 {
		exp = int(math.Round(math.Log2(pl.Scale)))
	}
	scale := math.Ldexp(1, exp)
	o := tmd.Object{Scale: int32(exp)}
	for _, v := range m.Vertices {
		var c [3]int16
		for k := range 3 {
			x := math.Round(v[k]*scale + pl.Translate[k])
			if x < math.MinInt16 || x > math.MaxInt16 {
				return o, fmt.Errorf("rsd: a vertex at %v is outside 16 bits at scale %g", v, scale)
			}
			c[k] = int16(x)
		}
		o.Vertices = append(o.Vertices, tmd.Vector{X: c[0], Y: c[1], Z: c[2]})
	}
	for _, n := range m.Normals {
		o.Normals = append(o.Normals, tmd.Vector{
			X: int16(math.Round(n[0] * 4096)), Y: int16(math.Round(n[1] * 4096)), Z: int16(math.Round(n[2] * 4096)),
		})
	}
	for i, p := range m.Polygons {
		prim, err := m.primitive(p)
		if err != nil {
			return o, fmt.Errorf("rsd: polygon %d: %w", i, err)
		}
		o.Primitives = append(o.Primitives, prim)
	}
	return o, nil
}

func (m *Model) primitive(p Polygon) (tmd.Primitive, error) {
	n := 3
	switch p.Kind {
	case 1:
		n = 4
	case 2:
		return tmd.Line{
			Vertices: [2]uint16{uint16(p.Vertices[0]), uint16(p.Vertices[1])},
			Colors:   rgbs(p, map[bool]int{false: 1, true: 2}[p.Type == 'G']),
			Semi:     p.Flag&4 != 0,
		}.Primitive()
	case 3:
		if p.Texture >= len(m.Textures) {
			return tmd.Primitive{}, fmt.Errorf("texture %d of %d", p.Texture, len(m.Textures))
		}
		t := m.Textures[p.Texture]
		u, v := t.UV()
		return tmd.Sprite{
			Vertex: uint16(p.Vertices[0]), TPage: tsb(t, p), CBA: t.CBA(),
			U: uint8(u + p.UV[0][0]), V: uint8(v + p.UV[0][1]),
			W: uint16(p.Vertices[1]), H: uint16(p.Vertices[2]), Semi: p.Flag&4 != 0,
		}.Primitive()
	}
	poly := tmd.Polygon{Semi: p.Flag&4 != 0, DoubleSided: p.Flag&2 != 0}
	for k := range n {
		poly.Vertices = append(poly.Vertices, uint16(p.Vertices[k]))
	}
	lit := p.Flag&1 == 0
	if lit {
		poly.Normals = []uint16{uint16(p.Normals[0])}
		if p.Gouraud {
			poly.Normals = poly.Normals[:0]
			for k := range n {
				poly.Normals = append(poly.Normals, uint16(p.Normals[k]))
			}
		}
	}
	switch p.Type {
	case 'C':
		poly.Colors = rgbs(p, 1)
	case 'G':
		poly.Colors = rgbs(p, n)
	case 'T', 'D', 'H':
		if p.Texture >= len(m.Textures) {
			return tmd.Primitive{}, fmt.Errorf("texture %d of %d", p.Texture, len(m.Textures))
		}
		t := m.Textures[p.Texture]
		u, v := t.UV()
		tex := &tmd.Texture{TPage: tsb(t, p), CBA: t.CBA()}
		for k := range n {
			tex.UV[k] = [2]uint8{uint8(u + p.UV[k][0]), uint8(v + p.UV[k][1])}
		}
		poly.Texture = tex
		switch {
		case p.Type == 'D':
			poly.Colors = rgbs(p, 1)
		case p.Type == 'H':
			poly.Colors = rgbs(p, n)
		case !lit:
			poly.Colors = []tmd.RGB{{R: 128, G: 128, B: 128}}
		}
		if lit {
			poly.Colors = nil
		}
	}
	return poly.Primitive()
}

func rgbs(p Polygon, n int) []tmd.RGB {
	out := make([]tmd.RGB, n)
	for k := range n {
		c := p.Colors[k]
		out[k] = tmd.RGB{R: uint8(c[0]), G: uint8(c[1]), B: uint8(c[2])}
	}
	return out
}

// tsb is the texture page a polygon names, with its blending rate when it
// is semitransparent.
func tsb(t *tim.TIM, p Polygon) uint16 {
	page := t.TPage()
	if p.Flag&4 != 0 {
		page |= uint16(p.Flag>>3&3) << 5
	}
	return page
}
