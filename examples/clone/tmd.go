package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

// The maze, the bad guys and everything else are textured quads, which the
// demo builds as TMDs in memory rather than loading any.

// ABE is the primitive mode bit for semi-transparency.
const abe = 0x02

// put writes v at the cursor and moves it on, returning where v went.
func put[T any](cursor *uintptr, v T) *T {
	p := (*T)(yaroze.Pointer(*cursor))
	*p = v
	*cursor += unsafe.Sizeof(v)
	return p
}

// quad is the texture coordinates of a quad's four corners, in the order
// the demo gives them:
//
//	0 +----+ 1
//	  |    |
//	2 +----+ 3
type quad [4][2]uint16

// texture works out a TIM's CBA and TSB, and moves the coordinates in q
// from the TIM's own to its texture page's. trans, if positive, is the
// semi-transparency mode.
func texture(tim yaroze.TIM, q *quad, trans int8) (cba, tsb uint16) {
	image := tim.Image()

	pmode := uint16(image.Pmode & 3)
	cba = uint16(image.Cy)<<6 + uint16(image.Cx)>>4
	tsb = uint16(image.Px)>>6 + uint16(image.Py)>>10 + pmode<<7
	if trans > 0 {
		tsb += uint16(trans) << 5
	}

	// The offset from the start of the texture page, in texels.
	pixScale := uint16(1)
	switch pmode {
	case 0:
		pixScale = 4
	case 1:
		pixScale = 2
	}
	for i := range q {
		q[i][0] += uint16(image.Px&63) * pixScale
		q[i][1] += uint16(image.Py) & 511
	}
	return cba, tsb
}

// primHeader is a quad's header: mode G_4T, semi-transparent unless trans
// is negative.
func primHeader(trans int8) demo.PRIM_HDR {
	mode := uint8(demo.G_4T)
	if trans >= 0 {
		mode |= abe
	}
	return demo.PRIM_HDR{Mode: mode, Flag: 0, Ilen: 8, Olen: 12}
}

// mid is halfway from a to b, in C's int arithmetic.
func mid(a, b uint16) uint8 { return uint8(int32(a) + (int32(b)-int32(a))/2) }

func midVert(a, b demo.TMD_VERT) demo.TMD_VERT {
	m := func(a, b int16) int16 { return int16(int32(a) + (int32(b)-int32(a))/2) }
	return demo.TMD_VERT{Vx: m(a.Vx, b.Vx), Vy: m(a.Vy, b.Vy), Vz: m(a.Vz, b.Vz)}
}

// g4t is a gouraud textured quad, with every corner's normal 0.
func g4t(cba, tsb uint16, uv [4][2]uint8, v [4]uint16) demo.TMD_G_4T {
	return demo.TMD_G_4T{
		Cba: cba, Tsb: tsb,
		U0: uv[0][0], V0: uv[0][1],
		U1: uv[1][0], V1: uv[1][1],
		U2: uv[2][0], V2: uv[2][1],
		U3: uv[3][0], V3: uv[3][1],
		Vert0: v[0], Vert1: v[1], Vert2: v[2], Vert3: v[3],
	}
}

// makeSplitTMD_G_4T builds a TMD in tmd of one quad with corners v and
// texture coordinates q, divided into four quads, which the demo does
// because it clips less near the eye than the library's own subdivision.
func makeSplitTMD_G_4T(tmd yaroze.TMD, tim yaroze.TIM, q quad, norm demo.TMD_NORM, v [4]demo.TMD_VERT, trans int8) {
	addr := uintptr(unsafe.Pointer(&tmd[0]))
	cba, tsb := texture(tim, &q, trans)
	u := func(i int) uint8 { return uint8(q[i][0]) }
	w := func(i int) uint8 { return uint8(q[i][1]) }
	// Halfway from corner a to corner b.
	m := func(a, b int) [2]uint8 { return [2]uint8{mid(q[a][0], q[b][0]), mid(q[a][1], q[b][1])} }

	cursor := addr
	put(&cursor, demo.TMD_HDR{ID: 0x41, Flag: 1, NObjs: 1})
	obj := put(&cursor, demo.TMD_OBJ{})

	prims := cursor
	for _, p := range [4]struct {
		uv   [4][2]uint8
		vert [4]uint16
	}{
		{[4][2]uint8{{u(0), w(0)}, m(0, 1), m(0, 2), m(0, 3)}, [4]uint16{0, 1, 3, 4}},
		{[4][2]uint8{m(0, 1), {u(1), w(1)}, m(0, 3), m(1, 3)}, [4]uint16{1, 2, 4, 5}},
		{[4][2]uint8{m(0, 2), m(0, 3), {u(2), w(2)}, m(2, 3)}, [4]uint16{3, 4, 6, 7}},
		{[4][2]uint8{m(0, 3), m(1, 3), m(2, 3), {u(3), w(3)}}, [4]uint16{4, 5, 7, 8}},
	} {
		put(&cursor, primHeader(trans))
		put(&cursor, g4t(cba, tsb, p.uv, p.vert))
	}

	verts := cursor
	for _, vert := range [9]demo.TMD_VERT{
		v[0], midVert(v[0], v[1]), v[1],
		midVert(v[0], v[2]), midVert(v[0], v[3]), midVert(v[1], v[3]),
		v[2], midVert(v[2], v[3]), v[3],
	} {
		put(&cursor, vert)
	}

	norms := cursor
	put(&cursor, norm)

	*obj = demo.TMD_OBJ{
		VertTop: uint32(verts), NVert: 9,
		NormTop: uint32(norms), NNorm: 1,
		PrimTop: uint32(prims), NPrim: 4,
	}
}

// makeTMD_G_4T builds a TMD in tmd of one quad, undivided.
func makeTMD_G_4T(tmd yaroze.TMD, tim yaroze.TIM, q quad, norm demo.TMD_NORM, v [4]demo.TMD_VERT, trans int8) {
	addr := uintptr(unsafe.Pointer(&tmd[0]))
	cba, tsb := texture(tim, &q, trans)

	cursor := addr
	put(&cursor, demo.TMD_HDR{ID: 0x41, Flag: 1, NObjs: 1})
	obj := put(&cursor, demo.TMD_OBJ{})

	prims := cursor
	put(&cursor, primHeader(trans))
	var uv [4][2]uint8
	for i := range q {
		uv[i] = [2]uint8{uint8(q[i][0]), uint8(q[i][1])}
	}
	put(&cursor, g4t(cba, tsb, uv, [4]uint16{0, 1, 2, 3}))

	verts := cursor
	for _, vert := range v {
		put(&cursor, vert)
	}

	norms := cursor
	put(&cursor, norm)

	*obj = demo.TMD_OBJ{
		VertTop: uint32(verts), NVert: 4,
		NormTop: uint32(norms), NNorm: 1,
		PrimTop: uint32(prims), NPrim: 1,
	}
}
