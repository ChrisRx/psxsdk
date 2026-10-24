// Command gen renders the mdec example's movie: spheres circling a mirrored
// one over a chequered floor, ray traced, and encoded for the MDEC.
//
// The five spheres are 72 degrees apart, and each one's color and height
// follow from where it is on its orbit, so after a fifth of a turn the scene
// is as it started. The movie is that fifth of a turn, and loops without a
// seam.
//
//	go run ./examples/mdec/gen -o examples/mdec/disc/DATA/MOVIE.MDC
package main

import (
	"flag"
	"image"
	"image/color"
	"log"
	"math"
	"os"

	"go.chrisrx.dev/psx/format/mdec"
)

const (
	width, height = 320, 240
	frames        = 30
	qscale        = 8 // the most that fits 30 frames in the space the program has
)

type vec [3]float64

func add(a, b vec) vec         { return vec{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }
func sub(a, b vec) vec         { return vec{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func mul(a vec, s float64) vec { return vec{a[0] * s, a[1] * s, a[2] * s} }
func dot(a, b vec) float64     { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func norm(a vec) vec           { return mul(a, 1/math.Sqrt(dot(a, a))) }
func cross(a, b vec) vec {
	return vec{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

type sphere struct {
	center vec
	radius float64
	color  vec
}

var light = norm(vec{-0.5, 1, -0.6})

// hit returns the distance along d from o to the nearest sphere, and which,
// or -1.
func hit(o, d vec, spheres []sphere) (float64, int) {
	t, which := math.Inf(1), -1
	for i, s := range spheres {
		oc := sub(o, s.center)
		b := dot(oc, d)
		h := b*b - dot(oc, oc) + s.radius*s.radius
		if h <= 0 {
			continue
		}
		if tt := -b - math.Sqrt(h); tt > 1e-4 && tt < t {
			t, which = tt, i
		}
	}
	return t, which
}

func trace(o, d vec, spheres []sphere, depth int) vec {
	t, which := hit(o, d, spheres)

	// The floor, at y = 0, fading into the sky with distance.
	if d[1] < 0 {
		if tf := -o[1] / d[1]; tf < t {
			p := add(o, mul(d, tf))
			c := vec{0.9, 0.9, 0.85}
			if (int(math.Floor(p[0]))+int(math.Floor(p[2])))&1 == 0 {
				c = vec{0.15, 0.2, 0.45}
			}
			shade := 0.3 + 0.7*light[1]
			if _, s := hit(p, light, spheres); s >= 0 {
				shade *= 0.4
			}
			fog := math.Exp(-tf * 0.04)
			return add(mul(c, shade*fog), mul(vec{0.5, 0.7, 1}, 1-fog))
		}
	}
	if which < 0 {
		y := math.Max(d[1], 0)
		return add(mul(vec{1, 0.75, 0.5}, 1-y), mul(vec{0.25, 0.45, 0.9}, y))
	}

	s := spheres[which]
	p := add(o, mul(d, t))
	n := norm(sub(p, s.center))
	diffuse := math.Max(dot(n, light), 0)
	specular := math.Pow(math.Max(dot(n, norm(sub(light, d))), 0), 40)
	c := add(mul(s.color, 0.15+0.75*diffuse), vec{specular, specular, specular})
	if depth < 2 {
		r := sub(d, mul(n, 2*dot(d, n)))
		c = add(mul(c, 0.65), mul(trace(p, r, spheres, depth+1), 0.35))
	}
	return c
}

// hue is a fully saturated color, h turns round the color wheel.
func hue(h float64) vec {
	h = (h - math.Floor(h)) * 6
	x := 1 - math.Abs(math.Mod(h, 2)-1)
	return [6]vec{{1, x, 0}, {x, 1, 0}, {0, 1, x}, {0, x, 1}, {x, 0, 1}, {1, 0, x}}[int(h)]
}

func frame(i int) image.Image {
	spheres := []sphere{{vec{0, 1.3, 0}, 1.3, vec{0.85, 0.85, 0.9}}}
	turn := 2 * math.Pi / 5 * float64(i) / frames
	for k := range 5 {
		a := turn + float64(k)*2*math.Pi/5
		c := mul(add(hue(a/(2*math.Pi)), vec{0.25, 0.25, 0.25}), 0.8)
		spheres = append(spheres, sphere{
			vec{2.2 * math.Cos(a), 0.75 + 0.5*math.Abs(math.Sin(a*2.5)), 2.2 * math.Sin(a)}, 0.7, c,
		})
	}

	eye, look := vec{0, 2.6, -7.5}, vec{0, 0.9, 0}
	fw := norm(sub(look, eye))
	right := norm(vec{fw[2], 0, -fw[0]})
	up := cross(right, fw)

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	gamma := func(v float64) uint8 { return uint8(255 * math.Pow(min(max(v, 0), 1), 1/2.2)) }
	for y := range height {
		for x := range width {
			var c vec
			for s := range 4 { // 2x2 samples a pixel
				u := (float64(x) + 0.25 + 0.5*float64(s&1) - width/2) / height
				v := (float64(y) + 0.25 + 0.5*float64(s>>1) - height/2) / height
				d := norm(add(add(fw, mul(right, -u*1.1)), mul(up, v*1.1)))
				c = add(c, trace(eye, d, spheres, 0))
			}
			c = mul(c, 0.25)
			img.SetNRGBA(x, y, color.NRGBA{gamma(c[0]), gamma(c[1]), gamma(c[2]), 0xff})
		}
	}
	return img
}

func main() {
	out := flag.String("o", "MOVIE.MDC", "where to write the movie")
	flag.Parse()

	m := &mdec.Movie{Width: width, Height: height}
	for i := range frames {
		m.Frames = append(m.Frames, mdec.Encode(frame(i), qscale))
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := m.WriteTo(f); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}
