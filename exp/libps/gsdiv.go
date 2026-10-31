//go:build yaroze

package libps

import "unsafe"

// GsSortObject4's subdivision, for a model whose attribute asks for it:
// each polygon is projected and lit once, as a whole, and then split
// into four through the midpoints of its edges, and those into four
// again, as many times as the attribute says, each piece projected on
// its own and sorted at the whole polygon's depth. The pieces keep the
// texture mapped straight across a large polygon, which the GPU, which
// knows nothing of depth, would otherwise bend.
//
// The original worked in the scratchpad: the polygon's corners in work
// blocks, and a stack of nodes, one a level, each holding the midpoints
// it makes and pointers to the corners it is splitting, which it filled
// by hand in assembly. Here the same blocks and nodes are Go values, and
// one function a shape takes the place of the original's one a kind.
//
// What the original did not do, this does not either: a polygon too
// close, or wholly off one side of the screen, is left out by its
// corners alone, with no test of its pieces; the pieces are not tested
// against the viewer's side; and a quad lit with fog is sorted by the
// depth of its first three corners.

//export libps_getGeomOffset
func getGeomOffset(out *[2]int32)

// A divVertex is a corner or a midpoint: its position, its texture
// coordinate, its color with the GPU code in the top byte, its screen
// point and its depth.
type divVertex struct {
	pos   [3]int16
	u, v  uint8
	color uint32
	sxy   uint32
	sz    uint32
}

// A divNode is one level of the subdivision: the midpoints it makes,
// three for a triangle and five for a quad, and the corners it splits.
type divNode struct {
	mid [5]divVertex
	v   [4]*divVertex
}

// A divider subdivides the primitives of one GsSortObject4 call.
type divider struct {
	sort     objSort // the sort, copied: a pointer to it would put it on the heap
	ndiv     int32
	textured bool
	gouraud  bool
	halfW    int32 // half the screen, for the tests against its edges
	halfH    int32
	halfDist int32 // half the projection distance, for the test against the viewer
	ofx, ofy int32 // the GTE's screen offset
	color    uint32
	code     uint8
	clut     uint32 // the palette and the page, each in the low half
	tpage    uint32
	entry    *uint32 // the ordering table entry every piece goes to
	verts    [4]divVertex
	nodes    [6]divNode
}

// gsDivider is the one divider, reused: its nodes point into it, which
// would put a divider made on the stack on the heap for every call, and
// this target's collector frees nothing.
var gsDivider divider

// setupDivider sets the divider up for the sort s with the model's
// attribute's level, once a model. The corners and nodes are left as the
// last call left them: a corner's texture coordinate is read only when
// textured and written first, its color only when gouraud, and its
// screen point after its projection, so clearing their 800 bytes for
// every model bought nothing.
func (s *objSort) setupDivider() {
	var ofs [2]int32
	getGeomOffset(&ofs)
	d := &gsDivider
	d.sort, d.ndiv = *s, gsNDIV
	d.halfW, d.halfH = int32(screenW>>1), int32(screenH>>1)
	d.halfDist = getGeomScreen() >> 1
	d.ofx, d.ofy = ofs[0], ofs[1]
}

// newDivider is the divider set up for the model, for a run of
// primitives of the kind textured and gouraud say.
func (s *objSort) newDivider(textured, gouraud bool) *divider {
	d := &gsDivider
	d.textured, d.gouraud = textured, gouraud
	return d
}

// setEntry picks the ordering table entry for a primitive at otz.
func (d *divider) setEntry(otz int32) {
	z := (uint32(uint16(otz)) - uint32(d.sort.offset)) >> (d.sort.shift & 31)
	d.entry = (*uint32)(unsafe.Add(d.sort.org, z*4))
}

// setColor sets the flat color, with the code over its top byte.
func (d *divider) setColor(c uint32) { d.color = c&0xffffff | uint32(d.code)<<24 }

// link puts the packet at p, of n words after its tag, on the entry, and
// returns where the next packet goes.
func (d *divider) link(p *uint32, n uint32) *uint32 {
	old := *d.entry
	*d.entry = uint32(uintptr(unsafe.Pointer(p))) & 0xffffff
	*p = old | n<<24
	return advance(p, uintptr(n+1)*4)
}

// offscreen says whether all the corners given are too close to the
// viewer or off the same side of the screen.
func (d *divider) offscreen(vs []*divVertex) bool {
	near, right, left, bottom, top := true, true, true, true, true
	for _, v := range vs {
		sx, sy := int32(int16(v.sxy)), int32(int16(v.sxy>>16))
		near = near && v.sz < uint32(d.halfDist)
		right = right && sx > d.ofx+d.halfW
		left = left && sx < d.ofx-d.halfW
		bottom = bottom && sy > d.ofy+d.halfH
		top = top && sy < d.ofy-d.halfH
	}
	return near || right || left || bottom || top
}

// midpoint makes m the middle of a and b: positions halved towards
// minus infinity, texture coordinates and colors halved as bytes. The
// kind comes as arguments, since read from the divider it would be read
// again after every store, the compiler not knowing the two apart.
func midpoint(m, a, b *divVertex, textured, gouraud bool) {
	for i := range m.pos {
		m.pos[i] = int16((int32(a.pos[i]) + int32(b.pos[i])) >> 1)
	}
	if textured {
		m.u = uint8((uint32(a.u) + uint32(b.u)) >> 1)
		m.v = uint8((uint32(a.v) + uint32(b.v)) >> 1)
	}
	if gouraud {
		m.color = midColor(a.color, b.color)
	}
}

// midColor halves two colors byte by byte, the top byte left 0.
func midColor(a, b uint32) uint32 {
	return (a&0xff+b&0xff)>>1 | ((a>>8&0xff + b>>8&0xff) >> 1 << 8) | ((a>>16&0xff + b>>16&0xff) >> 1 << 16)
}

// quarterColor is the average of four colors byte by byte.
func quarterColor(a, b, c, e uint32) uint32 {
	return (a&0xff+b&0xff+c&0xff+e&0xff)>>2 | ((a>>8&0xff + b>>8&0xff + c>>8&0xff + e>>8&0xff) >> 2 << 8) |
		((a>>16&0xff + b>>16&0xff + c>>16&0xff + e>>16&0xff) >> 2 << 16)
}

// project3 projects three midpoints together, loaded straight from
// their positions: the texture coordinate after a position goes into
// the high half of the second word, which the GTE ignores.
func (d *divider) project3(a, b, c *divVertex) {
	gteSetV3(unsafe.Pointer(&a.pos), unsafe.Pointer(&b.pos), unsafe.Pointer(&c.pos))
	gteRTPT()
}

// tri splits the triangle in node, at depth, into four, and sorts the
// pieces from packet, returning where the next packet goes.
func (d *divider) tri(packet *uint32, depth int32, node *divNode) *uint32 {
	if d.offscreen(node.v[:3]) {
		return packet
	}
	textured, gouraud := d.textured, d.gouraud
	m01, m12, m20 := &node.mid[0], &node.mid[1], &node.mid[2]
	midpoint(m01, node.v[0], node.v[1], textured, gouraud)
	midpoint(m12, node.v[1], node.v[2], textured, gouraud)
	midpoint(m20, node.v[2], node.v[0], textured, gouraud)
	d.project3(m01, m12, m20)
	depth++
	if depth == d.ndiv {
		gteSXYTo(&m01.sxy, &m12.sxy, &m20.sxy)
		packet = d.leafTri(packet, node.v[1], m12, m01)
		packet = d.leafTri(packet, m01, m12, m20)
		packet = d.leafTri(packet, node.v[0], m01, m20)
		packet = d.leafTri(packet, node.v[2], m20, m12)
		return packet
	}
	gteSZTo(&m01.sz, &m12.sz, &m20.sz)
	gteSXYTo(&m01.sxy, &m12.sxy, &m20.sxy)
	child := &d.nodes[depth]
	child.v = [4]*divVertex{node.v[0], m01, m20}
	packet = d.tri(packet, depth, child)
	child.v = [4]*divVertex{node.v[1], m12, m01}
	packet = d.tri(packet, depth, child)
	child.v = [4]*divVertex{node.v[2], m20, m12}
	packet = d.tri(packet, depth, child)
	child.v = [4]*divVertex{m01, m12, m20}
	return d.tri(packet, depth, child)
}

// leafTri sorts one piece of a triangle.
func (d *divider) leafTri(packet *uint32, a, b, c *divVertex) *uint32 {
	switch {
	case d.gouraud && d.textured:
		w := unsafe.Slice(packet, 10)
		a.color = a.color&0xffffff | uint32(d.code)<<24
		w[1], w[4], w[7] = a.color, b.color, c.color
		w[2], w[5], w[8] = a.sxy, b.sxy, c.sxy
		w[3], w[6], w[9] = uv(a)|d.clut<<16, uv(b)|d.tpage<<16, uv(c)
		return d.link(packet, 9)
	case d.gouraud:
		w := unsafe.Slice(packet, 7)
		a.color = a.color&0xffffff | uint32(d.code)<<24
		w[1], w[3], w[5] = a.color, b.color, c.color
		w[2], w[4], w[6] = a.sxy, b.sxy, c.sxy
		return d.link(packet, 6)
	case d.textured:
		w := unsafe.Slice(packet, 8)
		w[1] = d.color
		w[2], w[4], w[6] = a.sxy, b.sxy, c.sxy
		w[3], w[5], w[7] = uv(a)|d.clut<<16, uv(b)|d.tpage<<16, uv(c)
		return d.link(packet, 7)
	}
	w := unsafe.Slice(packet, 5)
	w[1] = d.color
	w[2], w[3], w[4] = a.sxy, b.sxy, c.sxy
	return d.link(packet, 4)
}

// uv is a vertex's texture coordinate as the GPU takes it.
func uv(v *divVertex) uint32 { return uint32(v.u) | uint32(v.v)<<8 }

// quad splits the quad in node, at depth, into four, and sorts the
// pieces from packet, returning where the next packet goes. The corners
// go 0 1 over 2 3; the midpoints are of the edges 01, 02, 13 and 23, and
// the center.
func (d *divider) quad(packet *uint32, depth int32, node *divNode) *uint32 {
	if d.offscreen(node.v[:4]) {
		return packet
	}
	textured, gouraud := d.textured, d.gouraud
	v0, v1, v2, v3 := node.v[0], node.v[1], node.v[2], node.v[3]
	m01, m02, m13, m23, mc := &node.mid[0], &node.mid[1], &node.mid[2], &node.mid[3], &node.mid[4]
	midpoint(m01, v0, v1, textured, gouraud)
	midpoint(m02, v0, v2, textured, gouraud)
	midpoint(m13, v1, v3, textured, gouraud)
	midpoint(m23, v2, v3, textured, gouraud)
	if !textured && !gouraud {
		// The flat quad's center is the middle of two midpoints, which
		// rounds twice; the others take a quarter of the corners' sum.
		midpoint(mc, m01, m23, textured, gouraud)
		d.project3(m01, m02, m02)
		gteSXYTo2(&m01.sxy, &m02.sxy)
		gteSZTo2(&m01.sz, &m02.sz)
		d.project3(m13, m23, mc)
	} else {
		for i := range mc.pos {
			mc.pos[i] = int16((int32(v0.pos[i]) + int32(v1.pos[i]) + int32(v2.pos[i]) + int32(v3.pos[i])) >> 2)
		}
		if textured {
			mc.u = uint8((uint32(v0.u) + uint32(v1.u) + uint32(v2.u) + uint32(v3.u)) >> 2)
			mc.v = uint8((uint32(v0.v) + uint32(v1.v) + uint32(v2.v) + uint32(v3.v)) >> 2)
		}
		if gouraud {
			mc.color = quarterColor(v0.color, v1.color, v2.color, v3.color)
		}
		d.project3(m01, m02, mc)
		gteSXYTo(&m01.sxy, &m02.sxy, &mc.sxy)
		gteSZTo(&m01.sz, &m02.sz, &mc.sz)
		d.project3(m13, m23, mc)
	}
	depth++
	if depth == d.ndiv {
		gteSXYTo2(&m13.sxy, &m23.sxy)
		if !textured && !gouraud {
			gteSXY2At(&mc.sxy)
		}
		packet = d.leafQuad(packet, v0, m01, m02, mc)
		packet = d.leafQuad(packet, v1, m13, m01, mc)
		packet = d.leafQuad(packet, v2, m02, m23, mc)
		packet = d.leafQuad(packet, v3, m23, m13, mc)
		return packet
	}
	gteSXYTo2(&m13.sxy, &m23.sxy)
	gteSZTo2(&m13.sz, &m23.sz)
	if !textured && !gouraud {
		gteSXY2At(&mc.sxy)
		gteSZ3At(&mc.sz)
	}
	child := &d.nodes[depth]
	child.v = [4]*divVertex{v0, m01, m02, mc}
	packet = d.quad(packet, depth, child)
	child.v = [4]*divVertex{v1, m13, m01, mc}
	packet = d.quad(packet, depth, child)
	child.v = [4]*divVertex{v2, m02, m23, mc}
	packet = d.quad(packet, depth, child)
	child.v = [4]*divVertex{v3, m23, m13, mc}
	return d.quad(packet, depth, child)
}

// leafQuad sorts one piece of a quad.
func (d *divider) leafQuad(packet *uint32, a, b, c, e *divVertex) *uint32 {
	switch {
	case d.gouraud && d.textured:
		w := unsafe.Slice(packet, 13)
		a.color = a.color&0xffffff | uint32(d.code)<<24
		w[1], w[4], w[7], w[10] = a.color, b.color, c.color, e.color
		w[2], w[5], w[8], w[11] = a.sxy, b.sxy, c.sxy, e.sxy
		w[3], w[6], w[9], w[12] = uv(a)|d.clut<<16, uv(b)|d.tpage<<16, uv(c), uv(e)
		return d.link(packet, 12)
	case d.gouraud:
		w := unsafe.Slice(packet, 9)
		a.color = a.color&0xffffff | uint32(d.code)<<24
		w[1], w[3], w[5], w[7] = a.color, b.color, c.color, e.color
		w[2], w[4], w[6], w[8] = a.sxy, b.sxy, c.sxy, e.sxy
		return d.link(packet, 8)
	case d.textured:
		w := unsafe.Slice(packet, 10)
		w[1] = d.color
		w[2], w[4], w[6], w[8] = a.sxy, b.sxy, c.sxy, e.sxy
		w[3], w[5], w[7], w[9] = uv(a)|d.clut<<16, uv(b)|d.tpage<<16, uv(c), uv(e)
		return d.link(packet, 9)
	}
	w := unsafe.Slice(packet, 6)
	w[1] = d.color
	w[2], w[3], w[4], w[5] = a.sxy, b.sxy, c.sxy, e.sxy
	return d.link(packet, 5)
}

// The corners of a primitive, from the model's tables.

// corner copies vertex i of the model into the divider's corner n: the
// record's two words whole, its padding over the texture coordinate,
// which texture sets after.
func (d *divider) corner(n int, i uint32) {
	p := d.sort.vertex(i)
	*(*[2]uint32)(unsafe.Pointer(&d.verts[n])) = *(*[2]uint32)(p)
}

// texture gives corner n the texture coordinate in the word w, u in
// the low byte and v above it, as the corner keeps them.
func (d *divider) texture(n int, w uint32) {
	*(*uint16)(unsafe.Pointer(&d.verts[n].u)) = uint16(w)
}

// projectTri projects the three corners and says whether the triangle
// faces the viewer, or with facing false projects it regardless. Its
// depths are kept to be averaged.
func (d *divider) projectTri(facing bool) bool {
	v := &d.verts
	gteSetV3(unsafe.Pointer(&v[0].pos), unsafe.Pointer(&v[1].pos), unsafe.Pointer(&v[2].pos))
	gteRTPT()
	if facing && gteNCLIP() <= 0 {
		return false
	}
	gteSXYTo(&v[0].sxy, &v[1].sxy, &v[2].sxy)
	return true
}

// projectTriFog projects the three corners one at a time, keeping each
// one's depth cue, and says whether the triangle faces the viewer.
func (d *divider) projectTriFog(cue *[3]int32) bool {
	v := &d.verts
	for i := 0; i < 3; i++ {
		gteSetV0(unsafe.Pointer(&v[i].pos))
		gteRTPS()
		cue[i] = gteIR0()
	}
	if gteNCLIP() <= 0 {
		return false
	}
	return true
}

// keepDepths3 keeps the three corners' depths after their projection.
func (d *divider) keepDepths3() {
	v := &d.verts
	gteSZTo(&v[0].sz, &v[1].sz, &v[2].sz)
}

// projectQuad projects the four corners and says whether the quad faces
// the viewer, by its first three.
func (d *divider) projectQuad() bool {
	v := &d.verts
	gteSetV3(unsafe.Pointer(&v[0].pos), unsafe.Pointer(&v[1].pos), unsafe.Pointer(&v[2].pos))
	gteRTPT()
	if gteNCLIP() <= 0 {
		return false
	}
	gteSXYTo(&v[0].sxy, &v[1].sxy, &v[2].sxy)
	gteSetV0(unsafe.Pointer(&v[3].pos))
	gteRTPS()
	gteSXY2At(&v[3].sxy)
	return true
}

// keepDepths4 keeps the four corners' depths after their projection.
func (d *divider) keepDepths4() {
	v := &d.verts
	gteSZTo4(&v[0].sz, &v[1].sz, &v[2].sz, &v[3].sz)
}

// lightOne lights normal n under c with the kind of the sort, into the
// color of the flat primitive, or takes c as it is.
func (d *divider) lightOne(c uint32, n uint32) {
	switch d.sort.kind {
	case 0:
		gteSetRGBC(c)
		gteSetV0(d.sort.normal(n))
		gteNCCS()
		c = gteRGB2()
	case 1:
		gteSetRGBC(c)
		gteSetV0(d.sort.normal(n))
		gteNCDS()
		c = gteRGB2()
	}
	d.setColor(c)
}

// lightThree lights three normals under c into the first three corners'
// colors, or gives them c as it is. For a flat primitive the one normal
// is given three times, and the first corner's color is the flat one.
func (d *divider) lightThree(c uint32, n0, n1, n2 uint32) {
	v := &d.verts
	switch d.sort.kind {
	case 0:
		gteSetRGBC(c)
		gteSetV3(d.sort.normal(n0), d.sort.normal(n1), d.sort.normal(n2))
		gteNCCT()
		gteRGBTo(&v[0].color, &v[1].color, &v[2].color)
	case 1:
		gteSetRGBC(c)
		gteSetV3(d.sort.normal(n0), d.sort.normal(n1), d.sort.normal(n2))
		gteNCDT()
		gteRGBTo(&v[0].color, &v[1].color, &v[2].color)
	default:
		v[0].color, v[1].color, v[2].color = c, c, c
	}
}

// lightThreeFog lights three normals under c with each corner's own
// depth cue, as the fog kind of a gouraud primitive does.
func (d *divider) lightThreeFog(c uint32, n0, n1, n2 uint32, cue *[3]int32) {
	v := &d.verts
	for i, n := range [3]uint32{n0, n1, n2} {
		gteSetV0(d.sort.normal(n))
		gteSetRGBC(c)
		gteSetIR0(cue[i])
		gteNCDS()
		gteRGB2At(&v[i].color)
	}
	gteSXYTo(&v[0].sxy, &v[1].sxy, &v[2].sxy)
}

// lightFourth lights the fourth corner's normal under c, or gives it c.
func (d *divider) lightFourth(c uint32, n uint32) {
	v := &d.verts
	switch d.sort.kind {
	case 0:
		gteSetRGBC(c)
		gteSetV0(d.sort.normal(n))
		gteNCCS()
		gteRGB2At(&v[3].color)
	case 1:
		gteSetRGBC(c)
		gteSetV0(d.sort.normal(n))
		gteNCDS()
		gteRGB2At(&v[3].color)
	default:
		v[3].color = c
	}
}

// start sorts the primitive whose corners are set from packet: its
// entry from its depth, and the first node over the corners.
func (d *divider) startTri(packet *uint32, otz int32) *uint32 {
	d.setEntry(otz)
	d.nodes[0].v = [4]*divVertex{&d.verts[0], &d.verts[1], &d.verts[2]}
	return d.tri(packet, 0, &d.nodes[0])
}

func (d *divider) startQuad(packet *uint32, otz int32) *uint32 {
	d.setEntry(otz)
	d.nodes[0].v = [4]*divVertex{&d.verts[0], &d.verts[1], &d.verts[2], &d.verts[3]}
	return d.quad(packet, 0, &d.nodes[0])
}

// The handlers, one a primitive code, as the undivided ones in gsobj.go.
// Each sets the corners, projects and lights, and starts the split.

// f3div: color; normal and vertex 0; vertices 1 and 2.
//
// libps 0x8002E660, 0x8002E988 and 0x8002EC90
//
//go:noinline
func (s *objSort) f3div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(false, false)
	for ; n > 0; n-- {
		w2, w3 := word(p, 2), word(p, 3)
		d.corner(0, w2>>16)
		d.corner(1, w3&0xffff)
		d.corner(2, w3>>16)
		if d.projectTri(true) {
			d.code = uint8(word(p, 0)>>24) | uint8(s.code1)
			if s.kind == 2 {
				d.setColor(word(p, 1))
			} else {
				d.lightThree(word(p, 1), w2&0xffff, w2&0xffff, w2&0xffff)
				d.setColor(d.verts[0].color)
			}
			otz := gteAVSZ3()
			d.keepDepths3()
			packet = d.startTri(packet, otz)
		}
		p = advance(p, 16)
	}
	return packet
}

// nf3div: color; vertices 0 and 1; vertex 2.
//
// libps 0x8002EF0C
//
//go:noinline
func (s *objSort) nf3div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(false, false)
	for ; n > 0; n-- {
		w2 := word(p, 2)
		d.corner(0, w2&0xffff)
		d.corner(1, w2>>16)
		d.corner(2, word(p, 3)&0xffff)
		if d.projectTri(true) {
			d.code = uint8(word(p, 0)>>24) | uint8(s.code1)
			d.setColor(word(p, 1))
			otz := gteAVSZ3()
			d.keepDepths3()
			packet = d.startTri(packet, otz)
		}
		p = advance(p, 16)
	}
	return packet
}

// textured sets the palette, the page and the corners' coordinates from
// the three or four coordinate words after the header.
func (d *divider) textured3(p *uint32) {
	d.clut, d.tpage = word(p, 1)>>16, word(p, 2)>>16
	d.texture(0, word(p, 1))
	d.texture(1, word(p, 2))
	d.texture(2, word(p, 3))
}

func (d *divider) textured4(p *uint32) {
	d.textured3(p)
	d.texture(3, word(p, 4))
}

// ft3div: three texture coordinates; normal and vertex 0; vertices 1
// and 2. A double-sided one, bit 1 of the header's flag, is not tested
// for facing the viewer.
//
// libps 0x8002F0FC, 0x8002F62C and 0x8002F934
//
//go:noinline
func (s *objSort) ft3div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(true, false)
	for ; n > 0; n-- {
		w4, w5 := word(p, 4), word(p, 5)
		d.corner(0, w4>>16)
		d.corner(1, w5&0xffff)
		d.corner(2, w5>>16)
		if d.projectTri(word(p, 0)>>16&2 == 0) {
			d.code = uint8(word(p, 0)>>24) | uint8(s.code1)
			if s.kind == 2 {
				d.setColor(0x24808080)
			} else {
				d.lightThree(0x24808080, w4&0xffff, w4&0xffff, w4&0xffff)
				d.setColor(d.verts[0].color)
			}
			otz := gteAVSZ3()
			d.keepDepths3()
			d.textured3(p)
			packet = d.startTri(packet, otz)
		}
		p = advance(p, 24)
	}
	return packet
}

// nft3div: three texture coordinates; color; vertices 0 and 1; vertex 2.
//
// libps 0x8002FBBC
//
//go:noinline
func (s *objSort) nft3div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(true, false)
	for ; n > 0; n-- {
		w5 := word(p, 5)
		d.corner(0, w5&0xffff)
		d.corner(1, w5>>16)
		d.corner(2, word(p, 6)&0xffff)
		if d.projectTri(word(p, 0)>>16&2 == 0) {
			d.code = uint8(word(p, 0)>>24)&0xfe | uint8(s.code1)
			d.setColor(word(p, 4))
			otz := gteAVSZ3()
			d.keepDepths3()
			d.textured3(p)
			packet = d.startTri(packet, otz)
		}
		p = advance(p, 28)
	}
	return packet
}

// g3div: color; three normals and vertices.
//
// libps 0x8002FE40, 0x80030084 and 0x800303C8
//
//go:noinline
func (s *objSort) g3div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(false, true)
	for ; n > 0; n-- {
		w2, w3, w4 := word(p, 2), word(p, 3), word(p, 4)
		d.corner(0, w2>>16)
		d.corner(1, w3>>16)
		d.corner(2, w4>>16)
		packet = d.gouraudTri(p, packet, word(p, 1), w2&0xffff, w3&0xffff, w4&0xffff)
		p = advance(p, 20)
	}
	return packet
}

// gouraudTri projects and lights a gouraud triangle's corners, set
// already, under c with the normals given, and starts its split, or
// leaves the packet alone for one facing away.
func (d *divider) gouraudTri(p, packet *uint32, c uint32, n0, n1, n2 uint32) *uint32 {
	d.code = uint8(word(p, 0)>>24) | uint8(d.sort.code1)
	if d.textured && d.sort.kind == 2 {
		c = 0x34808080
	}
	var otz int32
	switch d.sort.kind {
	case 1:
		var cue [3]int32
		if !d.projectTriFog(&cue) {
			return packet
		}
		d.lightThreeFog(c, n0, n1, n2, &cue)
		otz = gteAVSZ3()
	default:
		if !d.projectTri(true) {
			return packet
		}
		d.lightThree(c, n0, n1, n2)
		otz = gteAVSZ3()
	}
	d.keepDepths3()
	if d.textured {
		d.textured3(p)
	}
	return d.startTri(packet, otz)
}

// ng3div: three colors; vertices 0 and 1; vertex 2.
//
// libps 0x800305C4
//
//go:noinline
func (s *objSort) ng3div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(false, true)
	for ; n > 0; n-- {
		w4 := word(p, 4)
		d.corner(0, w4&0xffff)
		d.corner(1, w4>>16)
		d.corner(2, word(p, 5)&0xffff)
		if d.projectTri(true) {
			d.code = uint8(word(p, 0)>>24) | uint8(s.code1)
			d.verts[0].color, d.verts[1].color, d.verts[2].color = word(p, 1), word(p, 2), word(p, 3)
			otz := gteAVSZ3()
			d.keepDepths3()
			packet = d.startTri(packet, otz)
		}
		p = advance(p, 20)
	}
	return packet
}

// gt3div: three texture coordinates; three normals and vertices.
//
// libps 0x800307C0, 0x80030A3C and 0x80030CB8
//
//go:noinline
func (s *objSort) gt3div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(true, true)
	for ; n > 0; n-- {
		w4, w5, w6 := word(p, 4), word(p, 5), word(p, 6)
		d.corner(0, w4>>16)
		d.corner(1, w5>>16)
		d.corner(2, w6>>16)
		packet = d.gouraudTri(p, packet, 0x34808080, w4&0xffff, w5&0xffff, w6&0xffff)
		p = advance(p, 28)
	}
	return packet
}

// ngt3div: three texture coordinates; three colors; vertices 0 and 1;
// vertex 2.
//
// libps 0x80030EEC
//
//go:noinline
func (s *objSort) ngt3div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(true, true)
	for ; n > 0; n-- {
		w7 := word(p, 7)
		d.corner(0, w7&0xffff)
		d.corner(1, w7>>16)
		d.corner(2, word(p, 8)&0xffff)
		if d.projectTri(true) {
			d.code = uint8(word(p, 0)>>24)&0xfe | uint8(s.code1)
			d.verts[0].color, d.verts[1].color, d.verts[2].color = word(p, 4), word(p, 5), word(p, 6)
			otz := gteAVSZ3()
			d.keepDepths3()
			d.textured3(p)
			packet = d.startTri(packet, otz)
		}
		p = advance(p, 36)
	}
	return packet
}

// f4div: color; normal and vertex 0; vertices 1 and 2; vertex 3.
//
// libps 0x80031128, 0x80031464 and 0x800316D4
//
//go:noinline
func (s *objSort) f4div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(false, false)
	for ; n > 0; n-- {
		w2, w3 := word(p, 2), word(p, 3)
		d.corner(0, w2>>16)
		d.corner(1, w3&0xffff)
		d.corner(2, w3>>16)
		d.corner(3, word(p, 4)&0xffff)
		if d.projectQuad() {
			otz := gteAVSZ4()
			d.keepDepths4()
			d.code = uint8(word(p, 0)>>24) | uint8(s.code1)
			if s.kind == 2 {
				d.setColor(word(p, 1))
			} else {
				d.lightOne(word(p, 1), w2&0xffff)
			}
			packet = d.startQuad(packet, otz)
		}
		p = advance(p, 20)
	}
	return packet
}

// nf4div: color; vertices 0 and 1; vertices 2 and 3.
//
// libps 0x80031910
//
//go:noinline
func (s *objSort) nf4div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(false, false)
	for ; n > 0; n-- {
		w2, w3 := word(p, 2), word(p, 3)
		d.corner(0, w2&0xffff)
		d.corner(1, w2>>16)
		d.corner(2, w3&0xffff)
		d.corner(3, w3>>16)
		if d.projectQuad() {
			otz := gteAVSZ4()
			d.keepDepths4()
			d.code = uint8(word(p, 0)>>24) | uint8(s.code1)
			d.setColor(word(p, 1))
			packet = d.startQuad(packet, otz)
		}
		p = advance(p, 16)
	}
	return packet
}

// ft4div: four texture coordinates; normal and vertex 0; vertices 1 and
// 2; vertex 3.
//
// libps 0x80031B48, 0x80031DE8 and 0x8003208C
//
//go:noinline
func (s *objSort) ft4div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(true, false)
	for ; n > 0; n-- {
		w5, w6 := word(p, 5), word(p, 6)
		d.corner(0, w5>>16)
		d.corner(1, w6&0xffff)
		d.corner(2, w6>>16)
		d.corner(3, word(p, 7)&0xffff)
		if d.projectQuad() {
			otz := gteAVSZ4()
			d.keepDepths4()
			d.code = uint8(word(p, 0)>>24) | uint8(s.code1)
			if s.kind == 2 {
				d.setColor(0x2c808080)
			} else {
				d.lightOne(0x2c808080, w5&0xffff)
			}
			d.textured4(p)
			packet = d.startQuad(packet, otz)
		}
		p = advance(p, 32)
	}
	return packet
}

// nft4div: four texture coordinates; color; vertices 0 and 1; vertices
// 2 and 3.
//
// libps 0x80032310
//
//go:noinline
func (s *objSort) nft4div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(true, false)
	for ; n > 0; n-- {
		w6, w7 := word(p, 6), word(p, 7)
		d.corner(0, w6&0xffff)
		d.corner(1, w6>>16)
		d.corner(2, w7&0xffff)
		d.corner(3, w7>>16)
		if d.projectQuad() {
			otz := gteAVSZ4()
			d.keepDepths4()
			d.code = uint8(word(p, 0)>>24)&0xfe | uint8(s.code1)
			d.setColor(word(p, 5))
			d.textured4(p)
			packet = d.startQuad(packet, otz)
		}
		p = advance(p, 32)
	}
	return packet
}

// gouraudQuad projects and lights a gouraud quad's corners, set
// already, under c with the normals given, and starts its split, or
// leaves the packet alone for one facing away. With fog the corners are
// projected one at a time for their own depth cues, and the quad is
// sorted by the first three's average depth, as the original had it.
func (d *divider) gouraudQuad(p, packet *uint32, c uint32, n0, n1, n2, n3 uint32) *uint32 {
	d.code = uint8(word(p, 0)>>24)&0xfe | uint8(d.sort.code1)
	if d.textured && d.sort.kind == 2 {
		c = 0x3c808080
	}
	var otz int32
	switch d.sort.kind {
	case 1:
		var cue [3]int32
		if !d.projectTriFog(&cue) {
			return packet
		}
		d.lightThreeFog(c, n0, n1, n2, &cue)
		otz = gteAVSZ3()
		gteSetV0(unsafe.Pointer(&d.verts[3].pos))
		gteRTPS()
		d.verts[3].sxy = gteSXY2()
		d.keepDepths4()
		d.lightFourth(c, n3)
	default:
		if !d.projectQuad() {
			return packet
		}
		otz = gteAVSZ4()
		d.keepDepths4()
		d.lightThree(c, n0, n1, n2)
		d.lightFourth(c, n3)
	}
	if d.textured {
		d.textured4(p)
	}
	return d.startQuad(packet, otz)
}

// g4div: color; four normals and vertices.
//
// libps 0x80032590, 0x80032838 and 0x80032B28
//
//go:noinline
func (s *objSort) g4div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(false, true)
	for ; n > 0; n-- {
		w2, w3, w4, w5 := word(p, 2), word(p, 3), word(p, 4), word(p, 5)
		d.corner(0, w2>>16)
		d.corner(1, w3>>16)
		d.corner(2, w4>>16)
		d.corner(3, w5>>16)
		packet = d.gouraudQuad(p, packet, word(p, 1), w2&0xffff, w3&0xffff, w4&0xffff, w5&0xffff)
		p = advance(p, 24)
	}
	return packet
}

// ng4div: four colors; vertices 0 and 1; vertices 2 and 3.
//
// libps 0x80032D84
//
//go:noinline
func (s *objSort) ng4div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(false, true)
	for ; n > 0; n-- {
		w5, w6 := word(p, 5), word(p, 6)
		d.corner(0, w5&0xffff)
		d.corner(1, w5>>16)
		d.corner(2, w6&0xffff)
		d.corner(3, w6>>16)
		if d.projectQuad() {
			otz := gteAVSZ4()
			d.keepDepths4()
			d.code = uint8(word(p, 0)>>24) | uint8(s.code1)
			d.verts[0].color, d.verts[1].color, d.verts[2].color, d.verts[3].color = word(p, 1), word(p, 2), word(p, 3), word(p, 4)
			packet = d.startQuad(packet, otz)
		}
		p = advance(p, 28)
	}
	return packet
}

// gt4div: four texture coordinates; four normals and vertices.
//
// libps 0x80032FDC, 0x800332C8 and 0x800335CC
//
//go:noinline
func (s *objSort) gt4div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(true, true)
	for ; n > 0; n-- {
		w5, w6, w7, w8 := word(p, 5), word(p, 6), word(p, 7), word(p, 8)
		d.corner(0, w5>>16)
		d.corner(1, w6>>16)
		d.corner(2, w7>>16)
		d.corner(3, w8>>16)
		packet = d.gouraudQuad(p, packet, 0x3c808080, w5&0xffff, w6&0xffff, w7&0xffff, w8&0xffff)
		p = advance(p, 36)
	}
	return packet
}

// ngt4div: four texture coordinates; four colors; vertices 0 and 1;
// vertices 2 and 3.
//
// libps 0x80033858
//
//go:noinline
func (s *objSort) ngt4div(p, packet *uint32, n uint32) *uint32 {
	d := s.newDivider(true, true)
	for ; n > 0; n-- {
		w9, w10 := word(p, 9), word(p, 10)
		d.corner(0, w9&0xffff)
		d.corner(1, w9>>16)
		d.corner(2, w10&0xffff)
		d.corner(3, w10>>16)
		if d.projectQuad() {
			otz := gteAVSZ4()
			d.keepDepths4()
			d.code = uint8(word(p, 0)>>24)&0xfe | uint8(s.code1)
			d.verts[0].color, d.verts[1].color, d.verts[2].color, d.verts[3].color = word(p, 5), word(p, 6), word(p, 7), word(p, 8)
			d.textured4(p)
			packet = d.startQuad(packet, otz)
		}
		p = advance(p, 44)
	}
	return packet
}
