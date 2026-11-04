//go:build yaroze

package cleanlib

import (
	"device"
	"unsafe"
)

// Automatic subdivision: a polygon of an object whose attribute asks for
// it (bits 9 to 11, GsDIV1 to GsDIV5, and the field's values 6 and 7) is
// drawn as 4, 16 and so on smaller polygons, cut in the model's own
// coordinates, so that a large texture distorts less.
//
// The polygon is lit, culled by NCLIP and given its ordering table entry
// whole, as without subdivision, but the GTE's errors do not drop it.
// Each cut halves its edges in the model, each new point is transformed
// on its own, and every piece goes into the polygon's entry. Before a
// polygon or piece is cut it is dropped, with all it would become, when
// its corners all lie off the same side of the screen rectangle, or are
// all nearer than half the projection distance; the last level's pieces,
// the leaves, are linked as they come, unchecked. The screen rectangle is
// the display's width and height about the GTE's screen offset, so it is
// the same in the GsOFSGTE mode, where the offset is in the GTE's output.
//
// A lit gouraud polygon's corners are lit once and their colors averaged
// like an unlit one's (dungeon's fogged walls show it). A gouraud piece's
// color words for its corners 1 to 3 are written with a zero top byte.
// Sony's library writes a zero there the first time, but once a level's
// point has been a piece's first corner (which happens from GsDIV2 on,
// to the level above the last) its word keeps the code byte, and a later
// GsDIV1 sort of an unlit gouraud polygon shows it in those words (the
// div probe's second pass through its cases). The GPU ignores the byte;
// it is not reproduced.
//
// Spec: 04-3d-examples.md, "Subdivision (GsDIV)" and the subsections
// after it; probes div, divdeep and ofsgte.

// attrDivShift is where the subdivision level sits in an attribute.
const attrDivShift = 9

// divMax is the most levels an attribute can ask for.
const divMax = 7

// divCorner is a corner of a polygon being cut: its vertex in the model,
// its point and depth on the screen as the GTE gives them, and its texture
// coordinates (u, v<<8) and color (r, g<<8, b<<16). The GTE stores the
// point and depth into it directly (storeProjected), so their offsets
// are fixed: 12 and 16.
type divCorner struct {
	x, y, z int32
	sxy     uint32
	sz      uint32
	uv      uint32
	col     uint32
}

// The corners being cut: the polygon's own, and for each level the five
// points its cut makes, the middles of the edges and a quad's center.
// The cutting goes depth first, so a level's points stay put while the
// levels under it are cut. The last level's points go straight from the
// registers into its leaves and are not kept.
var (
	divRoot [4]divCorner
	divPts  [divMax][5]divCorner
)

// avg2 returns the bytes of the colors a and b averaged, each rounded
// down, and avg4 those of four.
//
//go:inline
func avg2(a, b uint32) uint32 {
	return a&b + (a^b)>>1&0x7f7f7f7f
}

//go:inline
func avg4(a, b, c, d uint32) uint32 {
	const m = 0x00ff00ff
	lo := a&m + b&m + c&m + d&m
	hi := a>>8&m + b>>8&m + c>>8&m + d>>8&m
	return lo>>2&m | (hi>>2&m)<<8
}

// avgUV2 and avgUV4 are the same for texture coordinates, two bytes,
// whose masks are immediates: the sum of four fits beside the u bytes'
// sum, which is taken off it to leave the v bytes' sum.
//
//go:inline
func avgUV2(a, b uint32) uint32 {
	return a&b + (a^b)>>1&0x7f7f
}

//go:inline
func avgUV4(a, b, c, d uint32) uint32 {
	u := a&0xff + b&0xff + c&0xff + d&0xff
	v := (a + b + c + d - u) >> 8
	return u>>2 | v>>2<<8
}

// vxy packs a vertex's x and y as the GTE takes them.
//
//go:inline
func vxy(x, y int32) uint32 { return uint32(uint16(x)) | uint32(y)<<16 }

// storeProjected stores the three points RTPT left, SXY0 to SXY2 and SZ1
// to SZ3, into the corners a, b and c; storeProjected2 stores the first
// two.
//
//go:inline
func storeProjected(a, b, c *divCorner) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$12, 12({a})
	swc2 $$17, 16({a})
	swc2 $$13, 12({b})
	swc2 $$18, 16({b})
	swc2 $$14, 12({c})
	swc2 $$19, 16({c})
	.set pop`, map[string]interface{}{"a": uintptr(unsafe.Pointer(a)), "b": uintptr(unsafe.Pointer(b)), "c": uintptr(unsafe.Pointer(c))})
}

//go:inline
func storeProjected2(a, b *divCorner) {
	device.AsmFull(`.set push
	.set noat
	swc2 $$12, 12({a})
	swc2 $$17, 16({a})
	swc2 $$13, 12({b})
	swc2 $$18, 16({b})
	.set pop`, map[string]interface{}{"a": uintptr(unsafe.Pointer(a)), "b": uintptr(unsafe.Pointer(b))})
}

// storeRootSZ stores the depths of the polygon's own corners, which the
// sort's RTPT and (for a quad) RTPS left in SZ1 to SZ3 or SZ0 to SZ3,
// into divRoot.
//
//go:inline
func storeRootSZ(n uintptr) {
	p := uintptr(unsafe.Pointer(&divRoot[0]))
	if n == 4 {
		device.AsmFull(`.set push
	.set noat
	swc2 $$16, 16({p})
	swc2 $$17, 44({p})
	swc2 $$18, 72({p})
	swc2 $$19, 100({p})
	.set pop`, map[string]interface{}{"p": p})
		return
	}
	device.AsmFull(`.set push
	.set noat
	swc2 $$17, 16({p})
	swc2 $$18, 44({p})
	swc2 $$19, 72({p})
	.set pop`, map[string]interface{}{"p": p})
}

// divider is the state of the polygon being cut: where the pieces go, the
// ordering table entry they are linked into, the packet words every piece
// shares, and the limits a piece is dropped at. It is one variable, not a
// local, so that no cut allocates.
type divider struct {
	out  uintptr
	head uint32    // the entry's first packet, the piece linked last
	col  uint32    // the first color word, or for a gouraud kind its code
	uvhi [4]uint32 // the high halves of the texture words: CBA and TSB

	// The screen rectangle, as a screen point word compares: x shifted
	// up to the top half against xlo and xhi, the word itself against
	// ylo and yhi, which is one past the bottom.
	xlo, xhi, ylo, yhi int32

	// near is the depth a corner is too near at, half the projection.
	near uint32
}

var cut divider

// subdivide cuts the polygon whose packet, made as without subdivision,
// is at out, whose vertices are at verts and which sorts into the
// ordering table entry tag, levels times, and links the pieces into tag
// from out on. It returns where the next packet goes.
func subdivide[K polyKind](out uintptr, verts *[4]uintptr, tag *uint32, levels int) uintptr {
	var k K
	n := k.corners()
	stride, _ := layout[K]()
	w, h := int32(gs.width)/2, int32(gs.height)/2
	cut = divider{
		out:  out,
		head: *tag & 0xffffff,
		col:  word(out, 1),
		xlo:  (gteOFX - w) << 16,
		xhi:  (gteOFX + w) << 16,
		ylo:  (gteOFY - h) << 16,
		yhi:  (gteOFY + h + 1) << 16,
		near: uint32(gteH >> 1),
	}
	if k.gouraud() {
		cut.col &= 0xff000000
	}
	storeRootSZ(n)
	for i := range n {
		p := &divRoot[i]
		v := verts[i]
		xy := word(v, 0)
		p.x, p.y, p.z = int32(int16(xy)), int32(int16(xy>>16)), int32(int16(word(v, 1)))
		p.sxy = word(out, 2+i*stride)
		if k.gouraud() {
			p.col = word(out, 1+i*stride) & 0xffffff
		}
		if k.textured() {
			uv := word(out, 3+i*stride)
			p.uv, cut.uvhi[i] = uv&0xffff, uv&^0xffff
		}
	}
	if levels > divMax {
		levels = divMax
	}
	if n == 4 {
		divQuad[K](&divRoot[0], &divRoot[1], &divRoot[2], &divRoot[3], levels)
	} else {
		divTri[K](&divRoot[0], &divRoot[1], &divRoot[2], levels)
	}
	*tag = *tag&0xff000000 | cut.head
	return cut.out
}

// drop reports whether the piece with the corners a, b, c and e is
// dropped before it is cut: its screen points all off the same side of
// the screen rectangle, a point on its edge being on it, or its depths
// all under half the projection. The first corner alone settles the
// usual case, a piece on the screen and far enough; offScreen, kept
// apart so that this inlines, takes the rest.
//
//go:inline
func drop(a, b, c, e *divCorner) bool {
	d := &cut
	sa := a.sxy
	xa, ya := int32(sa<<16), int32(sa)
	if (xa < d.xlo || xa > d.xhi || ya < d.ylo || ya >= d.yhi) && offScreen(a, b, c, e) {
		return true
	}
	near := d.near
	return a.sz < near && b.sz < near && c.sz < near && e.sz < near
}

// offScreen reports whether the corners' screen points all lie off the
// same side of the screen rectangle.
//
//go:noinline
func offScreen(a, b, c, e *divCorner) bool {
	d := &cut
	sa, sb, sc, se := a.sxy, b.sxy, c.sxy, e.sxy
	xa, xb, xc, xe := int32(sa<<16), int32(sb<<16), int32(sc<<16), int32(se<<16)
	ya, yb, yc, ye := int32(sa), int32(sb), int32(sc), int32(se)
	if lo := d.xlo; xa < lo && xb < lo && xc < lo && xe < lo {
		return true
	}
	if hi := d.xhi; xa > hi && xb > hi && xc > hi && xe > hi {
		return true
	}
	if lo := d.ylo; ya < lo && yb < lo && yc < lo && ye < lo {
		return true
	}
	hi := d.yhi
	return ya >= hi && yb >= hi && yc >= hi && ye >= hi
}

// divTri cuts the triangle a, b, c levels more times. Its pieces are each
// corner with the middles of its edges to the next corner and the
// previous one, then the middle one of the three middles; they are cut
// in that order, but at the last level the leaves are linked second,
// fourth, first and third. The last level's points are not kept: its
// leaves are written from the registers (lastTri).
func divTri[K polyKind](a, b, c *divCorner, levels int) {
	if drop(a, b, c, c) {
		return
	}
	var k K
	ax, ay, az := a.x, a.y, a.z
	bx, by, bz := b.x, b.y, b.z
	cx, cy, cz := c.x, c.y, c.z
	abx, aby, abz := (ax+bx)>>1, (ay+by)>>1, (az+bz)>>1
	bcx, bcy, bcz := (bx+cx)>>1, (by+cy)>>1, (bz+cz)>>1
	cax, cay, caz := (cx+ax)>>1, (cy+ay)>>1, (cz+az)>>1
	rtpt(vxy(abx, aby), uint32(abz), vxy(bcx, bcy), uint32(bcz), vxy(cax, cay), uint32(caz))
	// The GTE works while the colors and texture coordinates are made.
	var abuv, bcuv, cauv, abcol, bccol, cacol uint32
	if k.textured() {
		auv, buv, cuv := a.uv, b.uv, c.uv
		abuv, bcuv, cauv = avgUV2(auv, buv), avgUV2(buv, cuv), avgUV2(cuv, auv)
	}
	if k.gouraud() {
		acol, bcol, ccol := a.col, b.col, c.col
		abcol, bccol, cacol = avg2(acol, bcol), avg2(bcol, ccol), avg2(ccol, acol)
	}
	if levels == 1 {
		lastTri[K](a, b, c, abuv, bcuv, cauv, abcol, bccol, cacol)
		return
	}
	m := &divPts[levels-1]
	ab, bc, ca := &m[0], &m[1], &m[2]
	ab.x, ab.y, ab.z = abx, aby, abz
	bc.x, bc.y, bc.z = bcx, bcy, bcz
	ca.x, ca.y, ca.z = cax, cay, caz
	if k.textured() {
		ab.uv, bc.uv, ca.uv = abuv, bcuv, cauv
	}
	if k.gouraud() {
		ab.col, bc.col, ca.col = abcol, bccol, cacol
	}
	storeProjected(ab, bc, ca)
	divTri[K](a, ab, ca, levels-1)
	divTri[K](b, bc, ab, levels-1)
	divTri[K](c, ca, bc, levels-1)
	divTri[K](ab, bc, ca, levels-1)
}

// lastTri writes the four leaves of the triangle a, b, c whose middles
// RTPT has just projected and whose middles' colors and texture
// coordinates are given.
//
//go:inline
func lastTri[K polyKind](a, b, c *divCorner, abuv, bcuv, cauv, abcol, bccol, cacol uint32) {
	l := newLeafer()
	as, bs, cs := a.sxy, b.sxy, c.sxy
	auv, buv, cuv := a.uv, b.uv, c.uv
	acol, bcol, ccol := a.col, b.col, c.col
	abs, bcs, cas := sxy0(), sxy1(), sxy2()
	leafTri[K](&l, bs, bcs, abs, buv, bcuv, abuv, bcol, bccol, abcol)
	leafTri[K](&l, abs, bcs, cas, abuv, bcuv, cauv, abcol, bccol, cacol)
	leafTri[K](&l, as, abs, cas, auv, abuv, cauv, acol, abcol, cacol)
	leafTri[K](&l, cs, cas, bcs, cuv, cauv, bcuv, ccol, cacol, bccol)
	cut.out, cut.head = l.out, l.head
}

// divQuad cuts the quad a, b, c, e (corners in packet order, so round a, b,
// e, c) levels more times. Its pieces are each corner with the middles
// of its edges to the next corner and the previous one, and the quad's
// center, made and linked corner by corner. As for a triangle, the last
// level's points are not kept (lastQuad).
func divQuad[K polyKind](a, b, c, e *divCorner, levels int) {
	if drop(a, b, c, e) {
		return
	}
	var k K
	ax, ay, az := a.x, a.y, a.z
	bx, by, bz := b.x, b.y, b.z
	cx, cy, cz := c.x, c.y, c.z
	ex, ey, ez := e.x, e.y, e.z
	abx, aby, abz := (ax+bx)>>1, (ay+by)>>1, (az+bz)>>1
	bex, bey, bez := (bx+ex)>>1, (by+ey)>>1, (bz+ez)>>1
	ecx, ecy, ecz := (ex+cx)>>1, (ey+cy)>>1, (ez+cz)>>1
	rtpt(vxy(abx, aby), uint32(abz), vxy(bex, bey), uint32(bez), vxy(ecx, ecy), uint32(ecz))
	// The GTE works while the rest is made.
	cax, cay, caz := (cx+ax)>>1, (cy+ay)>>1, (cz+az)>>1
	mx, my, mz := (ax+bx+cx+ex)>>2, (ay+by+cy+ey)>>2, (az+bz+cz+ez)>>2
	if levels == 1 {
		lastQuad[K](a, b, c, e, vxy(cax, cay), uint32(caz), vxy(mx, my), uint32(mz))
		return
	}
	m := &divPts[levels-1]
	ab, be, ec, ca, mid := &m[0], &m[1], &m[2], &m[3], &m[4]
	ab.x, ab.y, ab.z = abx, aby, abz
	be.x, be.y, be.z = bex, bey, bez
	ec.x, ec.y, ec.z = ecx, ecy, ecz
	ca.x, ca.y, ca.z = cax, cay, caz
	mid.x, mid.y, mid.z = mx, my, mz
	storeProjected(ab, be, ec)
	rtpt(vxy(cax, cay), uint32(caz), vxy(mx, my), uint32(mz), vxy(mx, my), uint32(mz))
	if k.textured() {
		auv, buv, cuv, euv := a.uv, b.uv, c.uv, e.uv
		ab.uv = avgUV2(auv, buv)
		be.uv = avgUV2(buv, euv)
		ec.uv = avgUV2(euv, cuv)
		ca.uv = avgUV2(cuv, auv)
		mid.uv = avgUV4(auv, buv, cuv, euv)
	}
	if k.gouraud() {
		acol, bcol, ccol, ecol := a.col, b.col, c.col, e.col
		ab.col = avg2(acol, bcol)
		be.col = avg2(bcol, ecol)
		ec.col = avg2(ecol, ccol)
		ca.col = avg2(ccol, acol)
		mid.col = avg4(acol, bcol, ccol, ecol)
	}
	storeProjected2(ca, mid)
	divQuad[K](a, ab, ca, mid, levels-1)
	divQuad[K](b, be, ab, mid, levels-1)
	divQuad[K](c, ca, ec, mid, levels-1)
	divQuad[K](e, ec, be, mid, levels-1)
}

// lastQuad writes the four leaves of the quad a, b, c, e whose first three
// middles RTPT has just projected; the fourth and the center, given as
// the GTE takes them, it projects itself.
//
//go:inline
func lastQuad[K polyKind](a, b, c, e *divCorner, caxy, caz, mxy, mz uint32) {
	var k K
	var abuv, beuv, ecuv, cauv, muv uint32
	var abcol, becol, eccol, cacol, mcol uint32
	auv, buv, cuv, euv := a.uv, b.uv, c.uv, e.uv
	acol, bcol, ccol, ecol := a.col, b.col, c.col, e.col
	if k.textured() {
		abuv, beuv = avgUV2(auv, buv), avgUV2(buv, euv)
		ecuv, cauv = avgUV2(euv, cuv), avgUV2(cuv, auv)
		muv = avgUV4(auv, buv, cuv, euv)
	}
	abs, bes, ecs := sxy0(), sxy1(), sxy2()
	rtpt(caxy, caz, mxy, mz, mxy, mz)
	if k.gouraud() {
		abcol, becol = avg2(acol, bcol), avg2(bcol, ecol)
		eccol, cacol = avg2(ecol, ccol), avg2(ccol, acol)
		mcol = avg4(acol, bcol, ccol, ecol)
	}
	l := newLeafer()
	as, bs, cs, es := a.sxy, b.sxy, c.sxy, e.sxy
	cas, ms := sxy0(), sxy1()
	leafQuad[K](&l, as, abs, cas, ms, auv, abuv, cauv, muv, acol, abcol, cacol, mcol)
	leafQuad[K](&l, bs, bes, abs, ms, buv, beuv, abuv, muv, bcol, becol, abcol, mcol)
	leafQuad[K](&l, cs, cas, ecs, ms, cuv, cauv, ecuv, muv, ccol, cacol, eccol, mcol)
	leafQuad[K](&l, es, ecs, bes, ms, euv, ecuv, beuv, muv, ecol, eccol, becol, mcol)
	cut.out, cut.head = l.out, l.head
}

// leafer is what the last level's pieces are written with: the divider's
// words, taken into locals so that four leaves in a row share them.
type leafer struct {
	out  uintptr
	head uint32
	col  uint32
	uvhi [4]uint32
}

//go:inline
func newLeafer() leafer {
	return leafer{out: cut.out, head: cut.head, col: cut.col, uvhi: cut.uvhi}
}

// leafTri and leafQuad write the packet of the piece whose corners have
// the screen points s, texture coordinates u and colors c given, in
// corner order, and link it in front of the entry's packets. A kind
// without texture or gouraud shading leaves its u or c unread.
//
//go:inline
func leafTri[K polyKind](l *leafer, s0, s1, s2, u0, u1, u2, c0, c1, c2 uint32) {
	var k K
	s, plen := layout[K]()
	out := l.out
	set(out, 2, s0)
	set(out, 2+s, s1)
	set(out, 2+2*s, s2)
	if k.gouraud() {
		// The other corners' colors have no code byte, as the
		// reference's lit ones show.
		set(out, 1, l.col|c0)
		set(out, 1+s, c1)
		set(out, 1+2*s, c2)
	} else {
		set(out, 1, l.col)
	}
	if k.textured() {
		set(out, 3, l.uvhi[0]|u0)
		set(out, 3+s, l.uvhi[1]|u1)
		set(out, 3+2*s, l.uvhi[2]|u2)
	}
	set(out, 0, uint32(plen)<<24|l.head)
	l.head = uint32(out) & 0xffffff
	l.out = out + 4*(plen+1)
}

//go:inline
func leafQuad[K polyKind](l *leafer, s0, s1, s2, s3, u0, u1, u2, u3, c0, c1, c2, c3 uint32) {
	var k K
	s, plen := layout[K]()
	out := l.out
	set(out, 2, s0)
	set(out, 2+s, s1)
	set(out, 2+2*s, s2)
	set(out, 2+3*s, s3)
	if k.gouraud() {
		set(out, 1, l.col|c0)
		set(out, 1+s, c1)
		set(out, 1+2*s, c2)
		set(out, 1+3*s, c3)
	} else {
		set(out, 1, l.col)
	}
	if k.textured() {
		set(out, 3, l.uvhi[0]|u0)
		set(out, 3+s, l.uvhi[1]|u1)
		set(out, 3+2*s, l.uvhi[2]|u2)
		set(out, 3+3*s, l.uvhi[3]|u3)
	}
	set(out, 0, uint32(plen)<<24|l.head)
	l.head = uint32(out) & 0xffffff
	l.out = out + 4*(plen+1)
}

// tagAt returns the ordering table entry e of the table at org.
//
//go:inline
func tagAt(org uintptr, e int32) *uint32 {
	return (*uint32)(unsafe.Pointer(org + 4*uintptr(e)))
}
