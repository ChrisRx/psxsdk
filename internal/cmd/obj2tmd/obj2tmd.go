// Package obj2tmd converts Wavefront OBJ models, which most 3D tools export,
// into TMD files the library draws with GsSortObject4. It is the obj2tmd
// tool and yaroze's tmd command.
package obj2tmd

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/format/iso9660"
	"go.chrisrx.dev/psx/format/tim"
	"go.chrisrx.dev/psx/format/tmd"
)

type options struct {
	Output   string
	Scale    float64
	Smooth   bool
	Unlit    bool
	Both     bool
	Textures string
}

// Command returns the command, as use: obj2tmd as a tool of its own, and
// tmd in yaroze.
func Command(use string) *cobra.Command {
	var opts options
	cmd := &cobra.Command{
		Use:   use + " [flags] <model.obj>",
		Short: "convert a Wavefront OBJ model into a TMD file",
		Long: use + ` converts an OBJ model into a TMD beside it, with the extension
.TMD unless -o names it. Each "o" in the file becomes an object of the TMD,
which a program links with GsLinkObject4 by its number, from 0.

The axes are turned from the OBJ's, Y up and the camera looking down -Z, to
the PlayStation's, Y down and Z away from the camera, and the faces' corners
reversed, so that the sides a modelling program shows are the sides the
console draws. Faces of more than four corners are split into triangles.
Coordinates are multiplied by --scale and rounded: a TMD's are 16-bit
integers, and a model a few hundred units across suits the usual
projection.

Faces are lit by the library, flat with a normal worked out from each face,
or with --smooth gouraud with the file's vertex normals. --unlit draws the
colors, or the textures, as they are. A material's Kd is the color of an
untextured face. A material with map_Kd is textured from a TIM made with
png2tim: the map's name with the extension .TIM beside the MTL file, or in
the directory --textures names, such as the program's data, under the map's
name or the disc's form of it (capitals, underscores, eight characters), or
the map itself when it is a TIM. The texture's place in video memory comes
from the TIM, so a program must load the same TIM.

The Net Yaroze library does not draw double-sided polygons, as Sony's FAQ
says; --both makes two faces of each, back to back, instead.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(&opts, args)
		},
		SilenceUsage: true,
	}
	f := cmd.Flags()
	f.StringVarP(&opts.Output, "output", "o", "", "where to write the TMD")
	f.Float64VarP(&opts.Scale, "scale", "s", 100, "multiply coordinates by this")
	f.BoolVar(&opts.Smooth, "smooth", false, "gouraud shading with the vertex normals")
	f.BoolVar(&opts.Unlit, "unlit", false, "draw colors and textures as they are, unlit")
	f.BoolVar(&opts.Both, "both", false, "make every face two-sided, with a second face back to back")
	f.StringVar(&opts.Textures, "textures", "", "a directory to look for the maps' TIMs in, after the MTL file's")
	return cmd
}

func run(opts *options, args []string) error {
	o, err := readOBJ(args[0])
	if err != nil {
		return err
	}
	textures := map[string]*tim.TIM{}
	var t tmd.TMD
	for _, obj := range o.objects {
		ob, err := convert(opts, o, obj, textures)
		if err != nil {
			return fmt.Errorf("%s: object %q: %w", args[0], obj.name, err)
		}
		t.Objects = append(t.Objects, ob)
	}
	out := opts.Output
	if out == "" {
		out = strings.TrimSuffix(args[0], filepath.Ext(args[0])) + ".TMD"
	}
	b := t.Bytes()
	if err := os.WriteFile(out, b, 0o644); err != nil {
		return err
	}
	fmt.Printf("created %q: %d bytes\n", out, len(b))
	for i, ob := range t.Objects {
		fmt.Printf("  object %d %q: %d vertices, %d normals, %d polygons\n", i, o.objects[i].name, len(ob.Vertices), len(ob.Normals), len(ob.Primitives))
	}
	return nil
}

type vec [3]float64

func (a vec) sub(b vec) vec { return vec{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func (a vec) cross(b vec) vec {
	return vec{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}
func (a vec) unit() vec {
	l := math.Sqrt(a[0]*a[0] + a[1]*a[1] + a[2]*a[2])
	if l == 0 {
		return vec{0, -1, 0}
	}
	return vec{a[0] / l, a[1] / l, a[2] / l}
}

// psx turns a direction or position from the OBJ's axes to the console's:
// half a turn about X, which keeps it a rotation.
func psx(v vec) vec { return vec{v[0], -v[1], -v[2]} }

// converter builds one TMD object, numbering the vertices and normals it
// uses as it meets them.
type converter struct {
	opts     *options
	o        *objFile
	textures map[string]*tim.TIM
	ob       tmd.Object
	verts    map[int]uint16
	norms    map[tmd.Vector]uint16
	smooth   map[int]vec // vertex normals worked out from the faces, when the file has none
}

func convert(opts *options, o *objFile, obj *objObject, textures map[string]*tim.TIM) (tmd.Object, error) {
	c := &converter{opts: opts, o: o, textures: textures, verts: map[int]uint16{}, norms: map[tmd.Vector]uint16{}}
	if opts.Smooth {
		c.smooth = map[int]vec{}
		for _, f := range obj.faces {
			n := c.faceNormal(f.corners)
			for _, k := range f.corners {
				s := c.smooth[k.v]
				c.smooth[k.v] = vec{s[0] + n[0], s[1] + n[1], s[2] + n[2]}
			}
		}
	}
	for _, f := range obj.faces {
		// Split anything bigger than a quad into a fan of triangles.
		var polys [][]corner
		if len(f.corners) <= 4 {
			polys = [][]corner{f.corners}
		} else {
			for i := 1; i+1 < len(f.corners); i++ {
				polys = append(polys, []corner{f.corners[0], f.corners[i], f.corners[i+1]})
			}
		}
		for _, p := range polys {
			if err := c.add(p, f.material, false); err != nil {
				return c.ob, err
			}
			if opts.Both {
				back := make([]corner, len(p))
				for i := range p {
					back[i] = p[len(p)-1-i]
				}
				if err := c.add(back, f.material, true); err != nil {
					return c.ob, err
				}
			}
		}
	}
	return c.ob, nil
}

// faceNormal is the outward normal of a face whose corners go anticlockwise
// seen from outside, as an OBJ's do, in the OBJ's axes.
func (c *converter) faceNormal(cs []corner) vec {
	a, b, d := vec(c.o.positions[cs[0].v]), vec(c.o.positions[cs[1].v]), vec(c.o.positions[cs[2].v])
	return b.sub(a).cross(d.sub(a)).unit()
}

func (c *converter) vertex(i int) (uint16, error) {
	if k, ok := c.verts[i]; ok {
		return k, nil
	}
	p := psx(c.o.positions[i])
	var v [3]int16
	for k := range 3 {
		x := math.Round(p[k] * c.opts.Scale)
		if x < math.MinInt16 || x > math.MaxInt16 {
			return 0, fmt.Errorf("vertex %d is outside 16 bits at scale %g: make --scale smaller", i+1, c.opts.Scale)
		}
		v[k] = int16(x)
	}
	k := uint16(len(c.ob.Vertices))
	c.ob.Vertices = append(c.ob.Vertices, tmd.Vector{X: v[0], Y: v[1], Z: v[2]})
	c.verts[i] = k
	return k, nil
}

func (c *converter) normal(n vec) uint16 {
	n = psx(n.unit())
	v := tmd.Vector{X: int16(math.Round(n[0] * 4096)), Y: int16(math.Round(n[1] * 4096)), Z: int16(math.Round(n[2] * 4096))}
	if k, ok := c.norms[v]; ok {
		return k
	}
	k := uint16(len(c.ob.Normals))
	c.ob.Normals = append(c.ob.Normals, v)
	c.norms[v] = k
	return k
}

func (c *converter) texture(m *material) (*tim.TIM, error) {
	path := filepath.Join(m.dir, filepath.FromSlash(strings.ReplaceAll(m.texture, `\`, "/")))
	if !strings.EqualFold(filepath.Ext(path), ".tim") {
		// Beside the MTL file, then in the textures directory, under the
		// map's own name and under the disc's form of it, as .TIM or
		// .tim: the first that exists, or the first for the error.
		stem := strings.TrimSuffix(path, filepath.Ext(path))
		tries := []string{stem + ".TIM", stem + ".tim"}
		if c.opts.Textures != "" {
			base := filepath.Base(stem)
			disc := strings.TrimSuffix(iso9660.ShortName(base+".TIM"), ".TIM")
			for _, name := range []string{base, disc} {
				in := filepath.Join(c.opts.Textures, name)
				tries = append(tries, in+".TIM", in+".tim")
			}
		}
		path = tries[0]
		for _, try := range tries {
			if _, err := os.Stat(try); err == nil {
				path = try
				break
			}
		}
	}
	if t, ok := c.textures[path]; ok {
		return t, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("texture %s: %w (make it with png2tim)", m.texture, err)
	}
	t, err := tim.Decode(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if !t.Mode.Indexed() && t.Mode != tim.Mode16 {
		return nil, fmt.Errorf("%s: a %s TIM cannot be a texture", path, t.Mode)
	}
	c.textures[path] = t
	return t, nil
}

// add adds a triangle or quad, its corners as the OBJ gives them,
// anticlockwise from outside; back is set for the back face --both adds.
func (c *converter) add(cs []corner, mtl string, back bool) error {
	m := c.o.materials[mtl]
	if m == nil {
		m = &material{color: [3]float64{1, 1, 1}}
	}
	// Clockwise on the screen is the front, so reverse the corners, and
	// put a quad's in strip order: a ring a, b, c, d is the strip a, b,
	// d, c.
	order := []int{0, 2, 1}
	if len(cs) == 4 {
		order = []int{0, 3, 1, 2}
	}
	var p tmd.Polygon
	for _, i := range order {
		v, err := c.vertex(cs[i].v)
		if err != nil {
			return err
		}
		p.Vertices = append(p.Vertices, v)
	}
	if !c.opts.Unlit {
		if c.opts.Smooth {
			for _, i := range order {
				n := c.smooth[cs[i].v]
				if k := cs[i].vn; k >= 0 && !back {
					n = c.o.normals[k]
				}
				if back {
					n = vec{-n[0], -n[1], -n[2]}
				}
				p.Normals = append(p.Normals, c.normal(n))
			}
		} else {
			p.Normals = []uint16{c.normal(c.faceNormal(cs))}
		}
	}
	if m.texture != "" {
		t, err := c.texture(m)
		if err != nil {
			return err
		}
		u0, v0 := t.UV()
		w, h := t.Width(), t.Height()
		tex := &tmd.Texture{TPage: t.TPage(), CBA: t.CBA()}
		for k, i := range order {
			if cs[i].vt < 0 {
				return fmt.Errorf("a textured face has a corner with no texture coordinates")
			}
			uv := c.o.uvs[cs[i].vt]
			u := min(max(int(math.Round(uv[0]*float64(w))), 0), w-1)
			v := min(max(int(math.Round((1-uv[1])*float64(h))), 0), h-1)
			tex.UV[k] = [2]uint8{uint8(u0 + u), uint8(v0 + v)}
		}
		p.Texture = tex
		if c.opts.Unlit {
			p.Colors = []tmd.RGB{{R: 128, G: 128, B: 128}}
		}
	} else {
		col := func(f float64) uint8 { return uint8(min(max(math.Round(f*255), 0), 255)) }
		p.Colors = []tmd.RGB{{R: col(m.color[0]), G: col(m.color[1]), B: col(m.color[2])}}
	}
	prim, err := p.Primitive()
	if err != nil {
		return err
	}
	c.ob.Primitives = append(c.ob.Primitives, prim)
	return nil
}
