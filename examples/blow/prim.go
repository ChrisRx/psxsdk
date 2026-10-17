package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

// Reading and writing the parts of a TMD primitive, whatever its kind:
// the demo's tmdutil.c, which spells out every kind of primitive as a
// struct and switches on the mode byte to pick one. The layouts follow
// from the mode and flag bits, so here they are worked out instead: a
// polygon's body is its texture coordinates if it is textured, then its
// colors if it has any, then its normals and vertices, in the arrangement
// the bits choose, and its length is in the header.

// Bits of a polygon's mode byte, and of its flag byte.
const (
	polygon = 0x20 // a polygon rather than a line or a sprite, in the top bits
	iip     = 0x10 // gouraud shaded
	quad    = 0x08 // four sided
	tme     = 0x04 // textured
	tge     = 0x01 // no lighting

	grd = 0x04 // one color a vertex rather than one for the polygon
)

// Modes of the other kinds of primitive.
const (
	straightLine  = 0x40
	gradationLine = 0x50
	sprite3D      = 0x64
	sprite3D1     = 0x6c
	sprite3D8     = 0x74
	sprite3D16    = 0x7c
)

// A layout says where a primitive's parts are in its body: the vertex and
// normal indices as halfword offsets, the colors as byte offsets of their
// red component, and the texture's CBA and TSB as halfword offsets.
type layout struct {
	nvert, nnorm, ncol int
	vert, norm         [4]uint8
	col                [4]uint8
	textured           bool
	cba, tsb           uint8
}

// describe works out the layout of the primitive under h, and reports
// whether its kind is known.
func describe(h *demo.PRIM_HDR) (l layout, ok bool) {
	mode := h.Mode
	switch {
	case mode&0xe0 == polygon:
		n := 3
		if mode&quad != 0 {
			n = 4
		}
		lit := mode&tge == 0
		gouraud := mode&iip != 0
		textured := mode&tme != 0
		manyColors := h.Flag&grd != 0
		if !lit {
			manyColors = gouraud
		}

		hw := 0 // the halfword the next part starts at
		if textured {
			l.textured = true
			l.cba, l.tsb = 1, 3
			hw = 2 * n
		}
		if !(textured && lit) {
			l.ncol = 1
			if manyColors {
				l.ncol = n
			}
			for i := range l.ncol {
				l.col[i] = uint8(2 * hw)
				hw += 2
			}
		}
		l.nvert = n
		switch {
		case lit && gouraud:
			l.nnorm = n
			for i := range n {
				l.norm[i] = uint8(hw)
				l.vert[i] = uint8(hw + 1)
				hw += 2
			}
		case lit:
			l.nnorm = 1
			l.norm[0] = uint8(hw)
			for i := range n {
				l.vert[i] = uint8(hw + 1 + i)
			}
		default:
			for i := range n {
				l.vert[i] = uint8(hw + i)
			}
		}
		return l, true
	case mode == straightLine:
		l.ncol, l.col[0] = 1, 0
		l.nvert, l.vert = 2, [4]uint8{2, 3}
		return l, true
	case mode == gradationLine:
		l.ncol, l.col[0] = 1, 0
		l.nvert, l.vert = 2, [4]uint8{4, 5}
		return l, true
	case mode == sprite3D, mode == sprite3D1, mode == sprite3D8, mode == sprite3D16:
		l.nvert, l.vert[0] = 1, 0
		l.textured, l.tsb, l.cba = true, 1, 3
		return l, true
	}
	return l, false
}

// next returns the primitive after h, from the body length in its header.
func next(h *demo.PRIM_HDR) *demo.PRIM_HDR {
	return (*demo.PRIM_HDR)(unsafe.Add(unsafe.Pointer(h), 4+4*uintptr(h.Ilen)))
}

func body(h *demo.PRIM_HDR) unsafe.Pointer { return unsafe.Add(unsafe.Pointer(h), 4) }

func halfword(h *demo.PRIM_HDR, off uint8) *uint16 {
	return (*uint16)(unsafe.Add(body(h), 2*uintptr(off)))
}

func byteAt(h *demo.PRIM_HDR, off uint8) *uint8 {
	return (*uint8)(unsafe.Add(body(h), uintptr(off)))
}

// vertices returns the primitive's vertex indices and how many there are,
// or none if its kind is unknown.
func vertices(h *demo.PRIM_HDR, vertex *[4]uint16) int {
	l, ok := describe(h)
	if !ok {
		return 0
	}
	for i := range l.nvert {
		vertex[i] = *halfword(h, l.vert[i])
	}
	return l.nvert
}

// setVertices writes the primitive's vertex indices.
func setVertices(h *demo.PRIM_HDR, vertex *[4]uint16) int {
	l, ok := describe(h)
	if !ok {
		return 0
	}
	for i := range l.nvert {
		*halfword(h, l.vert[i]) = vertex[i]
	}
	return l.nvert
}

// normals returns the primitive's normal indices and how many there are.
func normals(h *demo.PRIM_HDR, normal *[4]uint16) int {
	l, ok := describe(h)
	if !ok {
		return 0
	}
	for i := range l.nnorm {
		normal[i] = *halfword(h, l.norm[i])
	}
	return l.nnorm
}

// setNormals writes the primitive's normal indices.
func setNormals(h *demo.PRIM_HDR, normal *[4]uint16) int {
	l, ok := describe(h)
	if !ok {
		return 0
	}
	for i := range l.nnorm {
		*halfword(h, l.norm[i]) = normal[i]
	}
	return l.nnorm
}

// colors returns the primitive's colors, which are values rather than
// indices, and how many there are.
func colors(h *demo.PRIM_HDR, col *[4]yaroze.CVECTOR) int {
	l, ok := describe(h)
	if !ok {
		return 0
	}
	for i := range l.ncol {
		p := byteAt(h, l.col[i])
		col[i].R = *p
		col[i].G = *(*uint8)(unsafe.Add(unsafe.Pointer(p), 1))
		col[i].B = *(*uint8)(unsafe.Add(unsafe.Pointer(p), 2))
	}
	return l.ncol
}

// setColors writes the primitive's colors.
func setColors(h *demo.PRIM_HDR, col *[4]yaroze.CVECTOR) int {
	l, ok := describe(h)
	if !ok {
		return 0
	}
	for i := range l.ncol {
		p := byteAt(h, l.col[i])
		*p = col[i].R
		*(*uint8)(unsafe.Add(unsafe.Pointer(p), 1)) = col[i].G
		*(*uint8)(unsafe.Add(unsafe.Pointer(p), 2)) = col[i].B
	}
	return l.ncol
}

// textureInfo returns a textured primitive's CBA and TSB, and whether it
// has them.
func textureInfo(h *demo.PRIM_HDR) (cba, tsb uint16, ok bool) {
	l, ok := describe(h)
	if !ok || !l.textured {
		return 0, 0, false
	}
	return *halfword(h, l.cba), *halfword(h, l.tsb), true
}

// copyPrim copies a primitive, header and body, to dst, and returns the
// primitives after each.
func copyPrim(src, dst *demo.PRIM_HDR) (srcNext, dstNext *demo.PRIM_HDR) {
	words := 1 + uintptr(src.Ilen)
	s := (*[64]uint32)(unsafe.Pointer(src))
	d := (*[64]uint32)(unsafe.Pointer(dst))
	copy(d[:words], s[:words])
	return next(src), next(dst)
}
