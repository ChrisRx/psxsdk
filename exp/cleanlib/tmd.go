//go:build yaroze

package cleanlib

import "unsafe"

// Models in the TMD format, and GsSortObject4, which draws them.

// GsDOBJ2 is a model as the program draws it: its attributes, the
// coordinate system it is in, and its object in a TMD.
type GsDOBJ2 struct {
	Attribute uint32
	Coord2    *GsCOORDINATE2
	Tmd       *tmdObject
	Id        uint32
}

// tmdObject is an entry of a TMD's object table: where its vertices,
// normals and primitives are, and how many of each. In the file the
// addresses are offsets from the table, until GsMapModelingData maps
// them.
type tmdObject struct {
	vertTop, nVert uint32
	normTop, nNorm uint32
	primTop, nPrim uint32
	scale          int32
}

// tmdMapped is the bit of a TMD's flags that says its offsets are
// addresses.
const tmdMapped = 1

// GsMapModelingData turns the offsets of the TMD whose flags word is at p,
// the word after the file's ID, into addresses, and marks the file as
// mapped, so that doing it again changes nothing.
//
//export GsMapModelingData
func GsMapModelingData(p *uint32) {
	if *p&tmdMapped != 0 {
		return
	}
	n := *(*uint32)(unsafe.Add(unsafe.Pointer(p), 4))
	table := unsafe.Add(unsafe.Pointer(p), 8)
	base := uint32(uintptr(table))
	objs := unsafe.Slice((*tmdObject)(table), n)
	for i := range objs {
		o := &objs[i]
		o.vertTop += base
		o.normTop += base
		o.primTop += base
	}
	*p |= tmdMapped
}

// GsLinkObject4 points obj at object n of the mapped TMD whose object
// table is at table.
//
//export GsLinkObject4
func GsLinkObject4(table uint32, obj *GsDOBJ2, n int32) {
	obj.Tmd = (*tmdObject)(unsafe.Pointer(uintptr(table) + uintptr(n)*unsafe.Sizeof(tmdObject{})))
}

// A primitive's header word holds its mode, its flags, and the lengths of
// its body and of the GPU packet it becomes, in words. The mode's top
// three bits name what it is, and for a polygon the others say whether it
// is gouraud shaded, a quad, textured, semi-transparent, and whether a
// texture is drawn without brightness.
const (
	primLine    = 2 // the mode's top three bits for a line
	primPolygon = 1 // and for a polygon

	modeGouraud  = 1 << 4
	modeQuad     = 1 << 3
	modeTextured = 1 << 2

	flagNoLight   = 1 << 0 // colors as given, not lit
	flagDouble    = 1 << 1 // drawn from both sides
	flagGradation = 1 << 2 // a lit polygon with a color for each corner
)

// polyKind is a kind of unlit polygon, as a type, so that each kind's
// sorting loop is compiled on its own with its layout as constants: its
// corners, and whether it is gouraud shaded and textured.
type polyKind interface {
	corners() uintptr
	gouraud() bool
	textured() bool
}

// The kinds, by their GPU commands' names: flat (F) or gouraud (G),
// textured (T) or not, with three corners or four.
type (
	polyF3  struct{}
	polyFT3 struct{}
	polyF4  struct{}
	polyFT4 struct{}
	polyG3  struct{}
	polyGT3 struct{}
	polyG4  struct{}
	polyGT4 struct{}
)

func (polyF3) corners() uintptr  { return 3 }
func (polyFT3) corners() uintptr { return 3 }
func (polyF4) corners() uintptr  { return 4 }
func (polyFT4) corners() uintptr { return 4 }
func (polyG3) corners() uintptr  { return 3 }
func (polyGT3) corners() uintptr { return 3 }
func (polyG4) corners() uintptr  { return 4 }
func (polyGT4) corners() uintptr { return 4 }

func (polyF3) gouraud() bool  { return false }
func (polyFT3) gouraud() bool { return false }
func (polyF4) gouraud() bool  { return false }
func (polyFT4) gouraud() bool { return false }
func (polyG3) gouraud() bool  { return true }
func (polyGT3) gouraud() bool { return true }
func (polyG4) gouraud() bool  { return true }
func (polyGT4) gouraud() bool { return true }

func (polyF3) textured() bool  { return false }
func (polyFT3) textured() bool { return true }
func (polyF4) textured() bool  { return false }
func (polyFT4) textured() bool { return true }
func (polyG3) textured() bool  { return false }
func (polyGT3) textured() bool { return true }
func (polyG4) textured() bool  { return false }
func (polyGT4) textured() bool { return true }

// layout returns the words a corner of a packet of kind K takes, its
// point and its color and texture coordinates if it has them, and the
// packet's length in words: a word for each corner's each, and a flat
// kind's one color.
//
//go:inline
func layout[K polyKind]() (stride, plen uintptr) {
	var k K
	stride = 1
	if k.textured() {
		stride++
	}
	if k.gouraud() {
		stride++
	}
	plen = k.corners() * stride
	if !k.gouraud() {
		plen++
	}
	return stride, plen
}

// sorter is what GsSortObject4 sorts with: the object's vertices, the
// ordering table, and where the next packet goes.
type sorter struct {
	div   int // the levels of subdivision, 0 for none
	verts uintptr
	norms uintptr
	light int
	org   uintptr
	shift int32
	out   uintptr
}

// GsSortObject4 transforms the primitives of obj to the screen with the
// matrix in the GTE, leaves out each that is behind the viewpoint or, if
// one-sided, faces away, and sorts a GPU packet for each of the rest into
// ot at its average depth shifted right by shift. scratch is not needed.
// The depth is not clamped: an average of 0 goes into entry 0, and one
// past the table writes the word that many entries past Org, as the 2D
// sorts do (spec 04, "Depth 0 and an entry past the table").
//
// It draws polygons, lit and unlit: flat and gouraud, triangles and
// quads, with texture or without, cut smaller when the attribute asks
// (div.go). Lit polygons with a color for each corner (gradation), lines
// and sprites are skipped. A polygon is dropped when its perspective
// transformations erred (GTE FLAG bit 31: a point out of range, or a
// vertex too near for the division), as the reference drops it, unless
// it is subdivided.
//
// A lit polygon's colors come from the GTE: NCCS for a flat one's normal,
// NCCT (and NCCS for a quad's fourth) for a gouraud one's, the fog forms
// NCDS and NCDT in fog mode, on RGBC, the polygon's color, or 808080h with
// the mode as its code for a textured one. Lighting off copies RGBC.
//
// A polygon's packet holds, for each corner, its color (only the first
// corner's on a flat polygon), its screen point, and its texture
// coordinates, which carry the palette and texture page as in the TMD.
// Primitives come in runs with the same header, and each kind has a loop
// of its own for a run, with its layout known when it is compiled.
//
//export GsSortObject4
func GsSortObject4(obj *GsDOBJ2, ot *GsOT, shift int32, scratch *uint32) {
	if obj.Attribute&attrHidden != 0 {
		return
	}
	o := obj.Tmd
	s := sorter{
		verts: uintptr(o.vertTop),
		norms: uintptr(o.normTop),
		light: lightMode(obj.Attribute),
		div:   int(obj.Attribute >> attrDivShift & 7),
		org:   uintptr(unsafe.Pointer(ot.Org)),
		shift: shift,
		out:   uintptr(gsOUT_PACKET_P),
	}
	prim := uintptr(o.primTop)
	left := o.nPrim
	for left > 0 {
		hdr := word(prim, 0)
		mode, flag := hdr>>24, hdr>>16&0xff
		var end uintptr
		var n uint32
		switch {
		case mode>>5 != primPolygon || flag&flagNoLight == 0 && flag&flagGradation != 0:
			end, n = s.skip(prim, left, hdr)
		case flag&flagNoLight == 0:
			switch mode & (modeGouraud | modeQuad | modeTextured) {
			case 0:
				end, n = litPolygons[polyF3](&s, prim, left, hdr)
			case modeTextured:
				end, n = litPolygons[polyFT3](&s, prim, left, hdr)
			case modeQuad:
				end, n = litPolygons[polyF4](&s, prim, left, hdr)
			case modeQuad | modeTextured:
				end, n = litPolygons[polyFT4](&s, prim, left, hdr)
			case modeGouraud:
				end, n = litPolygons[polyG3](&s, prim, left, hdr)
			case modeGouraud | modeTextured:
				end, n = litPolygons[polyGT3](&s, prim, left, hdr)
			case modeGouraud | modeQuad:
				end, n = litPolygons[polyG4](&s, prim, left, hdr)
			default:
				end, n = litPolygons[polyGT4](&s, prim, left, hdr)
			}
		default:
			switch mode & (modeGouraud | modeQuad | modeTextured) {
			case 0:
				end, n = polygons[polyF3](&s, prim, left, hdr)
			case modeTextured:
				end, n = polygons[polyFT3](&s, prim, left, hdr)
			case modeQuad:
				end, n = polygons[polyF4](&s, prim, left, hdr)
			case modeQuad | modeTextured:
				end, n = polygons[polyFT4](&s, prim, left, hdr)
			case modeGouraud:
				end, n = polygons[polyG3](&s, prim, left, hdr)
			case modeGouraud | modeTextured:
				end, n = polygons[polyGT3](&s, prim, left, hdr)
			case modeGouraud | modeQuad:
				end, n = polygons[polyG4](&s, prim, left, hdr)
			default:
				end, n = polygons[polyGT4](&s, prim, left, hdr)
			}
		}
		prim, left = end, left-n
	}
	gsOUT_PACKET_P = unsafe.Pointer(s.out)
}

// skip passes over the run of primitives with header hdr from prim, at
// most left of them, and returns the address after it and how many it
// passed.
func (s *sorter) skip(prim uintptr, left uint32, hdr uint32) (uintptr, uint32) {
	size := 4 + uintptr(hdr>>6&0x3fc)
	n := uint32(0)
	for n < left && word(prim, 0) == hdr {
		prim += size
		n++
	}
	return prim, n
}

// polygons sorts the run of unlit polygons of kind K with header hdr from
// prim, at most left of them, and returns the address after the run and
// how many it held.
//
// A textured polygon's body is its texture coordinates, a word for each
// corner, then its colors; an untextured one's starts with the colors.
// The vertex indices follow, a halfword each. A gouraud polygon has a
// color for each corner, a flat one a single color.
func polygons[K polyKind](s *sorter, prim uintptr, left uint32, hdr uint32) (uintptr, uint32) {
	var k K
	n, gouraud, textured := k.corners(), k.gouraud(), k.textured()
	size := 4 + uintptr(hdr>>6&0x3fc)
	oneSided := hdr>>16&flagDouble == 0
	code := (hdr >> 24 &^ 1) << 24
	var tex, col uintptr
	ncol := uintptr(1)
	if gouraud {
		ncol = n
	}
	if textured {
		col = n
	}
	vert := 2 * (col + ncol)
	stride, plen := layout[K]()
	verts, out := s.verts, s.out
	org, shift := s.org, s.shift
	count := uint32(0)
	for ; count < left && word(prim, 0) == hdr; prim += size {
		count++
		body := prim + 4
		// The GTE starts on the first three corners while the colors
		// and texture coordinates are copied into the packet.
		a := verts + 8*uintptr(half(body, vert))
		b := verts + 8*uintptr(half(body, vert+1))
		c := verts + 8*uintptr(half(body, vert+2))
		rtpt(word(a, 0), word(a, 1), word(b, 0), word(b, 1), word(c, 0), word(c, 1))
		if gouraud {
			set(out, 1, word(body, col))
			set(out, 1+stride, word(body, col+1))
			set(out, 1+2*stride, word(body, col+2))
			if n == 4 {
				set(out, 1+3*stride, word(body, col+3))
			}
		} else {
			set(out, 1, word(body, col))
		}
		if textured {
			// The library draws a textured polygon's texture
			// modulated by its color.
			set(out, 1, code|word(out, 1)&0xffffff)
			set(out, 3, word(body, tex))
			set(out, 3+stride, word(body, tex+1))
			set(out, 3+2*stride, word(body, tex+2))
			if n == 4 {
				set(out, 3+3*stride, word(body, tex+3))
			}
		}
		var dxy, dz uint32
		if n == 4 {
			d := verts + 8*uintptr(half(body, vert+3))
			dxy, dz = word(d, 0), word(d, 1)
		}
		// Reading FLAG waits for the GTE, so it comes after the copies.
		errs := flag()
		if oneSided && nclip() <= 0 {
			continue
		}
		storeSXY(out, stride)
		var z int32
		if n == 4 {
			rtps(dxy, dz)
			errs |= flag()
			storeSXY4(out, stride)
			z = avsz4()
		} else {
			z = avsz3()
		}
		e := z >> shift
		if errs&gteError != 0 && s.div == 0 {
			continue
		}
		if s.div != 0 {
			vs := [4]uintptr{a, b, c, verts + 8*uintptr(half(body, vert+n-1))}
			out = subdivide[K](out, &vs, tagAt(org, e), s.div)
			continue
		}
		t := (*uint32)(unsafe.Pointer(org + 4*uintptr(e)))
		set(out, 0, uint32(plen)<<24|*t&0xffffff)
		*t = *t&0xff000000 | uint32(out)&0xffffff
		out += 4 * (plen + 1)
	}
	s.out = out
	return prim, count
}

// gteError is the FLAG bits that drop a polygon: any of the
// perspective transformations erred, a point out of range or a depth too
// near for the division.
const gteError = 1 << 31

// Light modes, as lightMode gives them.
const (
	lightNormal = 0
	lightFog    = 1
	lightOff    = 2
)

// GsDOBJ2 attribute bits for lighting.
const (
	attrFog      = 1 << 3 // light with fog, when attrOwnMode is set
	attrOwnMode  = 1 << 5 // attrFog decides the mode, not GsSetLightMode
	attrLightOff = 1 << 6 // colors as the model gives them, unlit
)

// lightMode returns how an object with the attribute attr is lit: off,
// with fog, or normally, by its own fog bit or the mode GsSetLightMode
// set.
func lightMode(attr uint32) int {
	switch {
	case attr&attrLightOff != 0:
		return lightOff
	case attr&attrOwnMode != 0:
		if attr&attrFog != 0 {
			return lightFog
		}
		return lightNormal
	case gsLMODE == 1:
		return lightFog
	}
	return lightNormal
}

// litPolygons sorts the run of lit polygons of kind K with header hdr from
// prim, at most left of them, and returns the address after the run and
// how many it held. A gouraud kind here has a normal for each corner, a
// flat one a single normal; the packet of either has a color for each
// lit normal, which the GTE works out from the polygon's color, or for a
// textured polygon from 808080h, the texture's own brightness.
//
// The body is the texture coordinates, a word for each corner, if
// textured; the color, if not; and then the normals and vertices: for a
// gouraud polygon a word for each corner, its normal's index in the low
// half and its vertex's in the high; for a flat one, the normal and the
// first vertex, then the other vertices a halfword each.
func litPolygons[K polyKind](s *sorter, prim uintptr, left uint32, hdr uint32) (uintptr, uint32) {
	var k K
	n, gouraud, textured := k.corners(), k.gouraud(), k.textured()
	size := 4 + uintptr(hdr>>6&0x3fc)
	oneSided := hdr>>16&flagDouble == 0
	code := hdr >> 24 << 24
	var tex, col, nv uintptr
	if textured {
		nv = n
	} else {
		col, nv = 0, 1
	}
	// The halfwords of the vertex indices and the normal indices.
	var v0, v1, v2, v3, n0, n1, n2, n3 uintptr
	if gouraud {
		n0, v0 = 2*nv, 2*nv+1
		n1, v1 = 2*nv+2, 2*nv+3
		n2, v2 = 2*nv+4, 2*nv+5
		n3, v3 = 2*nv+6, 2*nv+7
	} else {
		n0, v0, v1, v2, v3 = 2*nv, 2*nv+1, 2*nv+2, 2*nv+3, 2*nv+4
	}
	stride, plen := layout[K]()
	verts, norms, out := s.verts, s.norms, s.out
	org, shift, light := s.org, s.shift, s.light
	count := uint32(0)
	for ; count < left && word(prim, 0) == hdr; prim += size {
		count++
		body := prim + 4
		a := verts + 8*uintptr(half(body, v0))
		b := verts + 8*uintptr(half(body, v1))
		c := verts + 8*uintptr(half(body, v2))
		rtpt(word(a, 0), word(a, 1), word(b, 0), word(b, 1), word(c, 0), word(c, 1))
		if textured {
			set(out, 3, word(body, tex))
			set(out, 3+stride, word(body, tex+1))
			set(out, 3+2*stride, word(body, tex+2))
			if n == 4 {
				set(out, 3+3*stride, word(body, tex+3))
			}
		}
		var dxy, dz uint32
		if n == 4 {
			d := verts + 8*uintptr(half(body, v3))
			dxy, dz = word(d, 0), word(d, 1)
		}
		// Reading FLAG waits for the GTE, so it comes after the copies.
		errs := flag()
		if oneSided && nclip() <= 0 {
			continue
		}
		storeSXY(out, stride)
		var z int32
		if n == 4 {
			rtps(dxy, dz)
			errs |= flag()
			storeSXY4(out, stride)
			z = avsz4()
		} else {
			z = avsz3()
		}
		e := z >> shift
		if errs&gteError != 0 && s.div == 0 {
			continue
		}
		// The color the light falls on.
		rgbc := code | 0x808080
		if !textured {
			rgbc = word(body, col)
		}
		if light == lightOff {
			set(out, 1, rgbc)
			if gouraud {
				set(out, 1+stride, rgbc)
				set(out, 1+2*stride, rgbc)
				if n == 4 {
					set(out, 1+3*stride, rgbc)
				}
			}
		} else {
			setRGBC(rgbc)
			na := norms + 8*uintptr(half(body, n0))
			if gouraud {
				nb := norms + 8*uintptr(half(body, n1))
				nc := norms + 8*uintptr(half(body, n2))
				loadV3(word(na, 0), word(na, 1), word(nb, 0), word(nb, 1), word(nc, 0), word(nc, 1))
				if light == lightFog {
					ncdt()
				} else {
					ncct()
				}
				set(out, 1, rgb0())
				set(out, 1+stride, rgb1())
				set(out, 1+2*stride, rgb2())
				if n == 4 {
					nd := norms + 8*uintptr(half(body, n3))
					loadV0(word(nd, 0), word(nd, 1))
					if light == lightFog {
						ncds()
					} else {
						nccs()
					}
					set(out, 1+3*stride, rgb2())
				}
			} else {
				loadV0(word(na, 0), word(na, 1))
				if light == lightFog {
					ncds()
				} else {
					nccs()
				}
				set(out, 1, rgb2())
			}
		}
		if s.div != 0 {
			vs := [4]uintptr{a, b, c, verts + 8*uintptr(half(body, v3))}
			out = subdivide[K](out, &vs, tagAt(org, e), s.div)
			continue
		}
		t := (*uint32)(unsafe.Pointer(org + 4*uintptr(e)))
		set(out, 0, uint32(plen)<<24|*t&0xffffff)
		*t = *t&0xff000000 | uint32(out)&0xffffff
		out += 4 * (plen + 1)
	}
	s.out = out
	return prim, count
}

// word returns the ith word from the address p, and set stores one.
//
//go:inline
func word(p uintptr, i uintptr) uint32 { return *(*uint32)(unsafe.Pointer(p + 4*i)) }

//go:inline
func set(p uintptr, i uintptr, v uint32) { *(*uint32)(unsafe.Pointer(p + 4*i)) = v }

// half returns the ith halfword from the address p.
//
//go:inline
func half(p uintptr, i uintptr) uint16 { return *(*uint16)(unsafe.Pointer(p + 2*i)) }
