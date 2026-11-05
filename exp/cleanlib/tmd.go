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
// table is at table, and marks that object's primitives for GsSortObject4
// (spec 21, "GsLinkObject4 changes the model"): they go in runs of the same
// mode and flag bytes, and the low half of each run's first header is
// overwritten with the run's length. The headers of the rest stay as they
// are. A mode with no size gets its message at each visit, and as it has no
// size the walk stays on it to the end.
//
//export GsLinkObject4
func GsLinkObject4(table uint32, obj *GsDOBJ2, n int32) {
	o := (*tmdObject)(unsafe.Pointer(uintptr(table) + uintptr(n)*unsafe.Sizeof(tmdObject{})))
	obj.Tmd = o
	o = at(o)
	prim := uintptr(o.primTop)
	first := prim
	run, kind := uint16(0), uint32(0)
	for left := o.nPrim; left > 0; left-- {
		k := word(prim, 0) >> 16
		if k != kind && run != 0 {
			*(*uint16)(unsafe.Pointer(first)) = run
			first, run = prim, 0
		}
		kind = k
		run++
		size := primSize(k>>8, k&0xff)
		if size == 0 {
			// The walk stays here: this is the rest of the run.
			*(*uint16)(unsafe.Pointer(first)) = run + uint16(left-1)
			printUnassigned(k>>8, left)
			return
		}
		prim += size
		space(linkPrimSpace)
		if k>>8&^(modeSemi|modeGouraud) == 0x20 {
			space(linkFlatSpace)
		}
	}
	if run != 0 {
		*(*uint16)(unsafe.Pointer(first)) = run
	}
}

// printUnassigned prints the message for a mode with no size, n times. It is apart
// from GsLinkObject4 so that that needs no frame for a call it never makes.
//
//go:noinline
func printUnassigned(mode, n uint32) {
	for range n {
		libps_biosA(biosPrintf, uintptr(unsafe.Pointer(unsafe.StringData(primUnassigned))), uintptr(mode), 0, 0)
	}
}

// primUnassigned is what GsLinkObject4 has the kernel print for a mode with no
// size.
const primUnassigned = "GPU CODE %02xH not assigned.\n\x00"

// What GsLinkObject4 spends on a primitive beyond its walk, in 2-cycle stores: 56
// cycles a primitive as on Sony's, and for the two modes that also have to look at the
// flag for their size, 20h and 30h, 62.
const (
	linkPrimSpace = 1
	linkFlatSpace = 2
)

// primSize returns the bytes a primitive of the mode and flags takes in a
// model, or 0 for a mode that is not a polygon GsLinkObject4 knows. The
// translucent bit makes no difference, and only a lit flat or gouraud triangle
// grows for its flag bit 2, a color for each corner. The header's own lengths
// do not count: GsLinkObject4 has overwritten them.
//
//go:inline
func primSize(mode, flags uint32) uintptr {
	i := mode&^modeSemi - 0x20
	if i >= uint32(len(primSizes)) {
		return 0
	}
	size := uintptr(primSizes[i])
	if flags&flagGradation != 0 && i&^modeGouraud == 0 {
		size += 8
	}
	return size
}

// primSizes is the bytes of a primitive by its mode less 20h, with the
// translucent bit clear.
var primSizes = [...]uint8{
	0x20 - 0x20: 16,
	0x21 - 0x20: 16,
	0x24 - 0x20: 24,
	0x25 - 0x20: 28,
	0x28 - 0x20: 20,
	0x29 - 0x20: 16,
	0x2c - 0x20: 32,
	0x2d - 0x20: 32,
	0x30 - 0x20: 20,
	0x31 - 0x20: 24,
	0x34 - 0x20: 28,
	0x35 - 0x20: 36,
	0x38 - 0x20: 24,
	0x39 - 0x20: 28,
	0x3c - 0x20: 36,
	0x3d - 0x20: 44,
}

// A primitive's header word holds its mode, its flags, and in a TMD's file
// the lengths of its body and of the GPU packet it becomes, in words, whose
// place GsLinkObject4 takes over for a run's length. The mode's top three
// bits name what it is, and for a polygon the others say whether it is gouraud
// shaded, a quad, textured, semi-transparent, and whether its colors are as given
// and not lit. It is the mode that says so: the flag byte's own bit for that is
// not looked at.
const (
	primLine    = 2 // the mode's top three bits for a line
	primPolygon = 1 // and for a polygon

	modeGouraud  = 1 << 4
	modeQuad     = 1 << 3
	modeTextured = 1 << 2
	modeSemi     = 1 << 1
	modeUnlit    = 1 << 0

	flagDouble    = 1 << 1 // both sides drawn, only of a textured flat triangle being cut
	flagGradation = 1 << 2 // a lit flat or gouraud triangle with a color for each corner
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
// ordering table, and where the next packet goes. It is one variable, not a
// local, as a program's stack may be small.
var sorter struct {
	div   int // the levels of subdivision, 0 for none
	verts uintptr
	norms uintptr
	light int
	semi  uint32 // the bit set in each packet's command word
	org   uintptr
	shift uint32 // how far an average depth goes down, the hardware's five bits
	out   uintptr
}

// GsSortObject4 transforms the primitives of obj to the screen with the
// matrix in the GTE, leaves out each that is behind the viewpoint or faces
// away, and sorts a GPU packet for each of the rest into ot at its average
// depth shifted right by shift. The flag byte's double-sided bit is one the
// Yaroze library does not support (the member FAQ's question 14), but for a
// textured flat triangle that is cut smaller. scratch is not written: what the
// reference keeps there, this keeps in its own variables.
// The depth is not clamped: an average of 0 goes into entry 0, and one
// past the table writes the word that many entries past Org, as the 2D
// sorts do (spec 04, "Depth 0 and an entry past the table").
//
// It draws polygons, lit and unlit: flat and gouraud, triangles and
// quads, with texture or without, cut smaller when the attribute asks
// (div.go). Lit triangles with a color for each corner are never cut
// (gradTriangles), and lines and sprites are skipped. A polygon is dropped
// when its perspective transformations erred (GTE FLAG bit 31: a point out of
// range, or a vertex too near for the division), as the reference drops it,
// unless it is subdivided.
//
// A lit polygon's colors come from the GTE: NCCS for a flat one's normal,
// NCCT (and NCCS for a quad's fourth) for a gouraud one's, the fog forms
// NCDS and NCDT in fog mode, on RGBC, the polygon's color, or 808080h with
// the mode as its code for a textured one. Lighting off copies RGBC, and takes a
// textured gouraud polygon that is not cut to a flat packet (offPolygons).
//
// The attribute's bit 30 makes every polygon translucent, setting 2h in its
// packet's command code, which a polygon the model marks translucent has
// already.
//
// A polygon's packet holds, for each corner, its color (only the first
// corner's on a flat polygon), its screen point, and its texture
// coordinates, which carry the palette and texture page as in the TMD.
// Primitives come in the runs GsLinkObject4 marked: the low half of a run's
// first header is its length, and that header's mode and flags stand for all
// of the run, whose own headers are not looked at. Each kind has a loop of
// its own for a run, with its layout known when it is compiled. A length of
// 0, in a model that was never linked, ends the sort.
//
//export GsSortObject4
func GsSortObject4(obj *GsDOBJ2, ot *GsOT, shift int32, scratch *uint32) {
	if obj.Attribute&attrHidden != 0 {
		return
	}
	o := obj.Tmd
	s := &sorter
	s.verts = uintptr(o.vertTop)
	s.norms = uintptr(o.normTop)
	s.light = lightMode(obj.Attribute)
	s.semi = 0
	if obj.Attribute&attrSemi != 0 {
		s.semi = 2 << 24
	}
	s.div = int(obj.Attribute >> attrDivShift & 7)
	s.org = uintptr(unsafe.Pointer(ot.Org))
	s.shift = uint32(shift) & 31
	s.out = uintptr(gsOUT_PACKET_P)
	prim := uintptr(o.primTop)
	left := o.nPrim
	for left > 0 {
		hdr := word(prim, 0)
		mode, flag := hdr>>24, hdr>>16&0xff
		n := min(hdr&0xffff, left)
		if n == 0 {
			break
		}
		left -= n
		kind := mode & (modeGouraud | modeQuad | modeTextured)
		switch {
		case mode>>5 != primPolygon:
			prim += uintptr(n) * primSize(mode, flag)
		case mode&modeUnlit != 0:
			switch kind {
			case 0:
				prim = polygons[polyF3](prim, n, hdr)
			case modeTextured:
				prim = polygons[polyFT3](prim, n, hdr)
			case modeQuad:
				prim = polygons[polyF4](prim, n, hdr)
			case modeQuad | modeTextured:
				prim = polygons[polyFT4](prim, n, hdr)
			case modeGouraud:
				prim = polygons[polyG3](prim, n, hdr)
			case modeGouraud | modeTextured:
				prim = polygons[polyGT3](prim, n, hdr)
			case modeGouraud | modeQuad:
				prim = polygons[polyG4](prim, n, hdr)
			default:
				prim = polygons[polyGT4](prim, n, hdr)
			}
		case flag&flagGradation != 0 && (kind == 0 || kind == modeGouraud):
			prim = gradTriangles(prim, n, hdr)
		case s.light == lightOff:
			switch kind {
			case 0:
				prim = offPolygons[polyF3, polyF3](prim, n, hdr)
			case modeTextured:
				prim = offPolygons[polyFT3, polyFT3](prim, n, hdr)
			case modeQuad:
				prim = offPolygons[polyF4, polyF4](prim, n, hdr)
			case modeQuad | modeTextured:
				prim = offPolygons[polyFT4, polyFT4](prim, n, hdr)
			case modeGouraud:
				prim = offPolygons[polyG3, polyG3](prim, n, hdr)
			case modeGouraud | modeQuad:
				prim = offPolygons[polyG4, polyG4](prim, n, hdr)
			case modeGouraud | modeTextured:
				if s.div != 0 {
					prim = offPolygons[polyGT3, polyGT3](prim, n, hdr)
				} else {
					prim = offPolygons[polyGT3, polyFT3](prim, n, hdr)
				}
			default:
				if s.div != 0 {
					prim = offPolygons[polyGT4, polyGT4](prim, n, hdr)
				} else {
					prim = offPolygons[polyGT4, polyFT4](prim, n, hdr)
				}
			}
		default:
			switch kind {
			case 0:
				prim = litPolygons[polyF3](prim, n, hdr)
			case modeTextured:
				prim = litPolygons[polyFT3](prim, n, hdr)
			case modeQuad:
				prim = litPolygons[polyF4](prim, n, hdr)
			case modeQuad | modeTextured:
				prim = litPolygons[polyFT4](prim, n, hdr)
			case modeGouraud:
				prim = litPolygons[polyG3](prim, n, hdr)
			case modeGouraud | modeTextured:
				prim = litPolygons[polyGT3](prim, n, hdr)
			case modeGouraud | modeQuad:
				prim = litPolygons[polyG4](prim, n, hdr)
			default:
				prim = litPolygons[polyGT4](prim, n, hdr)
			}
		}
	}
	gsOUT_PACKET_P = unsafe.Pointer(s.out)
}

// polygons sorts the run of count unlit polygons of kind K from prim, with hdr
// the first one's header, and returns the address after the run.
//
// A textured polygon's body is its texture coordinates, a word for each
// corner, then its colors; an untextured one's starts with the colors.
// The vertex indices follow, a halfword each. A gouraud polygon has a
// color for each corner, a flat one a single color.
func polygons[K polyKind](prim uintptr, count uint32, hdr uint32) uintptr {
	var k K
	s := &sorter
	n, gouraud, textured := k.corners(), k.gouraud(), k.textured()
	semi := s.semi
	code := hdr>>24<<24 | semi
	if textured {
		code &^= modeUnlit << 24
	}
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
	size := 4 * (1 + col + ncol + 2)
	verts, out := s.verts, s.out
	org, shift, div := s.org, s.shift, s.div
	bothSides := doubleSided[K](hdr, div)
	for ; count > 0; count, prim = count-1, prim+size {
		body := prim + 4
		// The GTE starts on the first three corners while the colors
		// and texture coordinates are copied into the packet.
		ia, ib, ic := half(body, vert), half(body, vert+1), half(body, vert+2)
		a, b, c := verts+8*uintptr(ia), verts+8*uintptr(ib), verts+8*uintptr(ic)
		rtptAt(a, b, c)
		first := word(body, col) | semi
		if textured {
			// The library draws a textured polygon's texture
			// modulated by its color.
			first = code | first&0xffffff
			set(out, 3, word(body, tex))
			set(out, 3+stride, word(body, tex+1))
			set(out, 3+2*stride, word(body, tex+2))
			if n == 4 {
				set(out, 3+3*stride, word(body, tex+3))
			}
		}
		set(out, 1, first)
		if gouraud {
			set(out, 1+stride, word(body, col+1))
			set(out, 1+2*stride, word(body, col+2))
			if n == 4 {
				set(out, 1+3*stride, word(body, col+3))
			}
		}
		var d uintptr
		if n == 4 {
			d = verts + 8*uintptr(half(body, vert+3))
		}
		// Reading FLAG waits for the GTE, so it comes after the copies.
		errs := flag()
		if nclip() <= 0 && !bothSides {
			continue
		}
		storeSXY(out, stride)
		var z int32
		if n == 4 {
			rtpsAt(d)
			errs |= flag()
			storeSXY4(out, stride)
			z = avsz4()
		} else {
			z = avsz3()
		}
		e := z >> shift
		if div != 0 {
			divVerts = [4]uintptr{a, b, c, verts + 8*uintptr(half(body, vert+n-1))}
			out = subdivide[K](out, tagAt(org, e), div, code)
			continue
		}
		if errs&gteError != 0 {
			continue
		}
		linkPacket(org, e, out, plen)
		out += 4 * (plen + 1)
	}
	s.out = out
	return prim
}

// doubleSided reports whether a run of kind K with the header hdr, cut div levels,
// draws faces turned away: only textured flat triangles can.
//
//go:inline
func doubleSided[K polyKind](hdr uint32, div int) bool {
	var k K
	return k.textured() && !k.gouraud() && k.corners() == 3 && div != 0 && hdr>>16&flagDouble != 0
}

// linkPacket makes the packet at out, of plen words after its link, first in the
// ordering table entry e of the table at org: it leads to what the entry led to and
// the entry to it, each keeping its top byte.
//
//go:inline
func linkPacket(org uintptr, e int32, out uintptr, plen uintptr) {
	t := org + 4*uintptr(e)
	old := word(t, 0)
	set(out, 0, uint32(plen)<<24|old&0xffffff)
	set(t, 0, old&0xff000000|uint32(out)&0xffffff)
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
	attrModeShift = 3      // the object's own mode, in two bits, when attrOwnMode is set
	attrOwnMode   = 1 << 5 // the object's own mode decides, not GsSetLightMode
	attrLightOff  = 1 << 6 // colors as the model gives them, unlit
)

// lightMode returns how an object with the attribute attr is lit: off, or by
// its own mode or the one GsSetLightMode set, where 1 is fog and any other,
// 3 too, normal.
func lightMode(attr uint32) int {
	if attr&attrLightOff != 0 {
		return lightOff
	}
	mode := uint32(gsLightMode)
	if attr&attrOwnMode != 0 {
		mode = attr >> attrModeShift & 3
	}
	if mode == 1 {
		return lightFog
	}
	return lightNormal
}

// litBody returns the bytes a lit polygon of kind K takes in a model, and where in
// its body, in halfwords, its normals and vertices start, and how many halfwords
// from a vertex to the next.
//
// The body is the texture coordinates, a word for each corner, if textured; the
// color, if not; and then the normals and vertices: for a gouraud polygon a word
// for each corner, its normal's index in the low half and its vertex's in the high;
// for a flat one, the normal and the first vertex, then the other vertices a
// halfword each.
//
//go:inline
func litBody[K polyKind]() (size, first, step uintptr) {
	var k K
	n := k.corners()
	nv := uintptr(1)
	if k.textured() {
		nv = n
	}
	if k.gouraud() {
		return 4 * (1 + nv + n), 2 * nv, 2
	}
	return 4 * (1 + nv + n - 1), 2 * nv, 1
}

// litPolygons sorts the run of count lit polygons of kind K from prim, with hdr
// the first one's header, and returns the address after the run. A gouraud kind
// here has a normal for each corner, a flat one a single normal; the packet of
// either has a color for each lit normal, which the GTE works out from the
// polygon's color, or for a textured polygon from 808080h, the texture's own
// brightness.
//
// What needs no GTE goes in while the GTE is at work. The perspective takes it long
// enough for the texture coordinates and the color and normals to be found, and
// the lighting for the packet to be linked, so its colors are read last.
func litPolygons[K polyKind](prim uintptr, count uint32, hdr uint32) uintptr {
	var k K
	s := &sorter
	n, gouraud, textured := k.corners(), k.gouraud(), k.textured()
	semi := s.semi
	code := hdr>>24<<24 | semi
	size, first, step := litBody[K]()
	v0 := first + 1
	stride, plen := layout[K]()
	verts, norms, out := s.verts, s.norms, s.out
	org, shift, div := s.org, s.shift, s.div
	fog := s.light == lightFog
	bothSides := doubleSided[K](hdr, div)
	for ; count > 0; count, prim = count-1, prim+size {
		body := prim + 4
		ia, ib, ic := half(body, v0), half(body, v0+step), half(body, v0+2*step)
		a, b, c := verts+8*uintptr(ia), verts+8*uintptr(ib), verts+8*uintptr(ic)
		rtptAt(a, b, c)
		rgbc := code | 0x808080
		if textured {
			set(out, 3, word(body, 0))
			set(out, 3+stride, word(body, 1))
			set(out, 3+2*stride, word(body, 2))
			if n == 4 {
				set(out, 3+3*stride, word(body, 3))
			}
		} else {
			rgbc = word(body, 0) | semi
			holdWord(rgbc)
		}
		na := norms + 8*uintptr(half(body, first))
		hold(na)
		var nb, nc, d uintptr
		if gouraud {
			nb = norms + 8*uintptr(half(body, first+2))
			nc = norms + 8*uintptr(half(body, first+4))
			hold(nb)
			hold(nc)
		}
		if n == 4 {
			d = verts + 8*uintptr(half(body, v0+3*step))
		}
		// Reading FLAG waits for the GTE, so it comes after these.
		errs := flag()
		if nclip() <= 0 && !bothSides {
			continue
		}
		storeSXY(out, stride)
		var z int32
		if n == 4 {
			rtpsAt(d)
			errs |= flag()
			storeSXY4(out, stride)
			z = avsz4()
		} else {
			z = avsz3()
		}
		e := z >> shift
		if div != 0 {
			startColors[K](rgbc, na, nb, nc, fog)
			storeColors[K](out, norms+8*uintptr(half(body, first+6)), fog)
			divVerts = [4]uintptr{a, b, c, d}
			out = subdivide[K](out, tagAt(org, e), div, code)
			continue
		}
		if errs&gteError != 0 {
			continue
		}
		startColors[K](rgbc, na, nb, nc, fog)
		linkPacket(org, e, out, plen)
		storeColors[K](out, norms+8*uintptr(half(body, first+6)), fog)
		out += 4 * (plen + 1)
	}
	s.out = out
	return prim
}

// startColors starts the GTE lighting a polygon of kind K, whose color is rgbc
// and whose normals are at na, and for a gouraud kind nb and nc.
//
//go:inline
func startColors[K polyKind](rgbc uint32, na, nb, nc uintptr, fog bool) {
	var k K
	setRGBC(rgbc)
	if k.gouraud() {
		loadV3(na, nb, nc)
		if fog {
			ncdt()
		} else {
			ncct()
		}
		return
	}
	loadV0(na)
	if fog {
		ncds()
	} else {
		nccs()
	}
}

// storeColors stores in the packet at out the colors startColors set going, and
// for a gouraud quad lights its fourth corner, whose normal is at nd.
//
//go:inline
func storeColors[K polyKind](out, nd uintptr, fog bool) {
	var k K
	stride, _ := layout[K]()
	if !k.gouraud() {
		set(out, 1, rgb2())
		return
	}
	set(out, 1, rgb0())
	set(out, 1+stride, rgb1())
	set(out, 1+2*stride, rgb2())
	if k.corners() == 4 {
		loadV0(nd)
		if fog {
			ncds()
		} else {
			nccs()
		}
		set(out, 1+3*stride, rgb2())
	}
}

// offPolygons sorts the run of count lit polygons from prim whose body has the
// layout of kind B, when the attribute turns lighting off, with hdr the first one's
// header, and returns the address after the run. Each packet is of kind P, with the
// polygon's color as it is, or for a textured one 808080h, the texture's own
// brightness. A polygon that is not cut smaller and is textured and gouraud gets a
// flat textured unlit packet: a 34h gives a 25h and a 3Ch a 2Dh.
func offPolygons[B, P polyKind](prim uintptr, count uint32, hdr uint32) uintptr {
	var k B
	var pk P
	s := &sorter
	n, textured := k.corners(), k.textured()
	semi := s.semi
	code := hdr>>24<<24 | semi
	if k.gouraud() && !pk.gouraud() {
		code = (hdr>>24&^modeGouraud|modeUnlit)<<24 | semi
	}
	size, first, step := litBody[B]()
	v0 := first + 1
	stride, plen := layout[P]()
	verts, out := s.verts, s.out
	org, shift, div := s.org, s.shift, s.div
	bothSides := doubleSided[B](hdr, div)
	for ; count > 0; count, prim = count-1, prim+size {
		body := prim + 4
		ia, ib, ic := half(body, v0), half(body, v0+step), half(body, v0+2*step)
		a, b, c := verts+8*uintptr(ia), verts+8*uintptr(ib), verts+8*uintptr(ic)
		rtptAt(a, b, c)
		rgbc := code | 0x808080
		if textured {
			set(out, 3, word(body, 0))
			set(out, 3+stride, word(body, 1))
			set(out, 3+2*stride, word(body, 2))
			if n == 4 {
				set(out, 3+3*stride, word(body, 3))
			}
		} else {
			rgbc = word(body, 0) | semi
		}
		set(out, 1, rgbc)
		if pk.gouraud() {
			set(out, 1+stride, rgbc)
			set(out, 1+2*stride, rgbc)
			if n == 4 {
				set(out, 1+3*stride, rgbc)
			}
		}
		var d uintptr
		if n == 4 {
			d = verts + 8*uintptr(half(body, v0+3*step))
		}
		// Reading FLAG waits for the GTE, so it comes after the copies.
		errs := flag()
		if nclip() <= 0 && !bothSides {
			continue
		}
		storeSXY(out, stride)
		var z int32
		if n == 4 {
			rtpsAt(d)
			errs |= flag()
			storeSXY4(out, stride)
			z = avsz4()
		} else {
			z = avsz3()
		}
		e := z >> shift
		if div != 0 {
			divVerts = [4]uintptr{a, b, c, d}
			out = subdivide[P](out, tagAt(org, e), div, code)
			continue
		}
		if errs&gteError != 0 {
			continue
		}
		linkPacket(org, e, out, plen)
		out += 4 * (plen + 1)
	}
	s.out = out
	return prim
}

// gradTriangles sorts the run of count lit triangles from prim that have a color for
// each corner, flag bit 2 on a 20h or a 30h, with hdr the first one's header, and
// returns the address after the run. Whichever they are they
// make gouraud packets, are never cut smaller, and take no translucency from the
// attribute or the mode. Each corner lights its own color, by its own normal on a 30h
// and by the one normal on a 20h. Lighting off, it keeps its color as it is.
//
// The body is the three colors, and then a 30h's normal and vertex, a word for
// each corner, or a 20h's normal and first vertex, then its other two vertices,
// a halfword each. A color's top byte goes into the packet, except that a 20h's
// first says 30h, the packet's code.
func gradTriangles(prim uintptr, count uint32, hdr uint32) uintptr {
	s := &sorter
	// The halfwords of the second and third corners' vertices and normals.
	v1, v2, n1, n2 := uintptr(9), uintptr(11), uintptr(8), uintptr(10)
	size := uintptr(28)
	first, keep := uint32(0), uint32(0xffffffff)
	if hdr>>24&modeGouraud == 0 {
		v1, v2, n1, n2 = 8, 9, 6, 6
		size = 24
		first, keep = 0x30<<24, 0xffffff
	}
	verts, norms, out := s.verts, s.norms, s.out
	org, shift, light := s.org, s.shift, s.light
	for ; count > 0; count, prim = count-1, prim+size {
		body := prim + 4
		a := verts + 8*uintptr(half(body, 7))
		b := verts + 8*uintptr(half(body, v1))
		c := verts + 8*uintptr(half(body, v2))
		rtptAt(a, b, c)
		errs := flag()
		if nclip() <= 0 {
			continue
		}
		storeSXY(out, 2)
		z := avsz3()
		if errs&gteError != 0 {
			continue
		}
		col0 := word(body, 0)&keep | first
		if light == lightOff {
			set(out, 1, col0)
			set(out, 3, word(body, 1))
			set(out, 5, word(body, 2))
		} else {
			set(out, 1, lightCorner(col0, norms+8*uintptr(half(body, 6)), light))
			set(out, 3, lightCorner(word(body, 1), norms+8*uintptr(half(body, n1)), light))
			set(out, 5, lightCorner(word(body, 2), norms+8*uintptr(half(body, n2)), light))
		}
		linkPacket(org, z>>shift, out, 6)
		out += 4 * 7
	}
	s.out = out
	return prim
}

// lightCorner returns the color rgbc as the normal at n lights it.
func lightCorner(rgbc uint32, n uintptr, light int) uint32 {
	setRGBC(rgbc)
	loadV0(n)
	if light == lightFog {
		ncds()
	} else {
		nccs()
	}
	return rgb2()
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
