//go:build yaroze

package libps

import "unsafe"

// GsSortObject4: a model's primitives through the GTE into GPU packets
// sorted by depth. A TMD primitive is a header word, its color, texture
// and vertex words after it, each vertex and normal an index into the
// model's tables; the GPU packet for it is a tag, the colors with their
// code, the screen points and the texture coordinates, in the order the
// GPU takes them. The library's original had a handler for every kind of
// primitive under every lighting; here one function a kind takes the
// lighting as a parameter.
//
// A primitive's code is its header's mode byte without the translucency
// bit: 0x20 for a flat triangle, 0x24 a textured one, 0x28 a flat quad,
// 0x2c textured, 0x30 a gouraud triangle, 0x34 textured, 0x38 a gouraud
// quad, 0x3c textured, each with bit 0 set when its colors are given
// rather than lit. A flat or gouraud triangle with the gradation flag,
// bit 2 of the header's flag byte, has a color a vertex.

// An objSort is what every handler needs: the model's tables, the
// ordering table, and the lighting.
type objSort struct {
	verts   unsafe.Pointer // the vertex table
	normals unsafe.Pointer // the normal table
	org     unsafe.Pointer // the ordering table's first entry
	offset  int32          // the depth of that entry
	shift   uint32         // how far depths are shifted to index the table
	kind    int32          // 0 lit, 1 lit with fog, 2 colors as given
	ton     uint32         // the translucency bit in a color word's code
	code1   uint32         // the translucency bit in a code byte
}

func (s *objSort) vertex(i uint32) unsafe.Pointer { return unsafe.Add(s.verts, i*8) }
func (s *objSort) normal(i uint32) unsafe.Pointer { return unsafe.Add(s.normals, i*8) }

// The GTE runs a command while the CPU goes on, and holds the CPU at
// its next GTE instruction until the command is done. So a handler is
// ordered as the original's was: it starts the projection, makes and
// sets the color while that runs, checks the result, starts the
// lighting, links the packet and fills in what needs no result while
// that runs, and stores the results last, straight from the GTE into
// the packet.
//
// Each handler is kept out of line, with its own registers for the loop,
// and works on a copy of the sort: a pointer's fields would be read back
// after every packet store, since the compiler cannot tell the two
// apart, where a copy on the stack is known to be nothing a store
// reaches.

// project3 loads vertices i0, i1 and i2 and starts their projection.
func (s *objSort) project3(i0, i1, i2 uint32) {
	gteSetV3(s.vertex(i0), s.vertex(i1), s.vertex(i2))
	gteRTPT()
}

// facing says whether the last three projections overflowed nothing
// and turn clockwise, a polygon facing the viewer. Their depths stay in
// the GTE to be averaged.
func facing() bool {
	return gteFlag() >= 0 && gteNCLIP() > 0
}

// project1 stores the last three screen points into the packet at p, a
// vertex every `every` words, since the next projection pushes them
// along, and starts projecting vertex i3.
func (s *objSort) project1(i3 uint32, p *uint32, every int) {
	switch every {
	case 1:
		gteSXYEvery1(p)
	case 2:
		gteSXYEvery2(p)
	default:
		gteSXYEvery3(p)
	}
	gteSetV0(s.vertex(i3))
	gteRTPS()
}

// projected says whether the last projection overflowed nothing.
func projected() bool { return gteFlag() >= 0 }

// quad is project1 and projected together, for a quad with nothing to
// do while its fourth vertex is projected.
func (s *objSort) quad(i3 uint32, p *uint32, every int) bool {
	s.project1(i3, p, every)
	return projected()
}

// insert links the packet, of length words after its tag, into the
// ordering table at otz. The shift's low five bits are taken, as srlv
// took them.
func (s *objSort) insert(packet *uint32, otz int32, length uint32) {
	z := (uint32(otz-s.offset) >> (s.shift & 31)) & 0xffff
	entry := (*uint32)(unsafe.Add(s.org, z*4))
	old := *entry
	*packet = length<<24 | old&0xffffff
	*entry = old&0xff000000 | uint32(uintptr(unsafe.Pointer(packet)))&0xffffff
}

// setColor sets the color the lighting starts from, unless the colors
// are taken as given.
func (s *objSort) setColor(c uint32) {
	if s.kind != 2 {
		gteSetRGBC(c)
	}
}

// light1 starts lighting normal n under the color set, or does nothing
// when the colors are taken as given.
func (s *objSort) light1(n uint32) {
	switch s.kind {
	case 0:
		gteSetV0(s.normal(n))
		gteNCCS()
	case 1:
		gteSetV0(s.normal(n))
		gteNCDS()
	}
}

// lit1 stores the color light1 made at p, or c as given.
func (s *objSort) lit1(c uint32, p *uint32) {
	if s.kind == 2 {
		*p = c
	} else {
		gteRGB2At(p)
	}
}

// light3 starts lighting normals n0, n1 and n2 under the color set, or
// does nothing when the colors are taken as given.
func (s *objSort) light3(n0, n1, n2 uint32) {
	switch s.kind {
	case 0:
		gteSetV3(s.normal(n0), s.normal(n1), s.normal(n2))
		gteNCCT()
	case 1:
		gteSetV3(s.normal(n0), s.normal(n1), s.normal(n2))
		gteNCDT()
	}
}

// lit3 stores the three colors light3 made into the packet at p, a
// vertex every two or three words, or c three times as given.
func (s *objSort) lit3(c uint32, p *uint32, every int) {
	switch {
	case s.kind == 2 && every == 2:
		w := unsafe.Slice(p, 6)
		w[1], w[3], w[5] = c, c, c
	case s.kind == 2:
		w := unsafe.Slice(p, 8)
		w[1], w[4], w[7] = c, c, c
	case every == 2:
		gteRGBEvery2(p)
	default:
		gteRGBEvery3(p)
	}
}

// one lights normal n under the color c into the word at p, or gives
// it c as given: a color of a gradation primitive, lit on its own.
func (s *objSort) one(c uint32, n uint32, p *uint32) {
	s.setColor(c)
	s.light1(n)
	s.lit1(c, p)
}

// texColor is the color a lit textured primitive starts from: a mid gray
// under the primitive's mode as its code.
func (s *objSort) texColor(mode uint32) uint32 { return 0x808080 | (mode|s.code1)<<24 }

// givenColor is the color word c of an unlit textured primitive under
// the primitive's mode as its code, without the lighting bit.
func (s *objSort) givenColor(c, mode uint32) uint32 {
	return c&0xffffff | ((mode|s.code1)&^1)<<24
}

// word reads word i of the primitive at p.
func word(p *uint32, i uintptr) uint32 { return *(*uint32)(unsafe.Add(unsafe.Pointer(p), i*4)) }

// advance moves a word pointer on by n bytes.
func advance(p *uint32, n uintptr) *uint32 { return (*uint32)(unsafe.Add(unsafe.Pointer(p), n)) }

// Each handler sorts n primitives from p into packets from packet, and
// returns where the next packet goes. The comment on each gives the
// primitive's words after its header. A packet is written before the
// primitive is known to be drawn where that fills a wait: the next
// primitive's packet goes over it if not.

// f3: color; normal and vertex 0; vertices 1 and 2.
//
// libps 0x80036348, 0x80036490 and 0x80036208
//
//go:noinline
func (sp *objSort) f3(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w2, w3 := word(p, 2), word(p, 3)
		s.project3(w2>>16, w3&0xffff, w3>>16)
		c := word(p, 1) | s.ton
		s.setColor(c)
		if facing() {
			otz := gteAVSZ3()
			s.light1(w2 & 0xffff)
			s.insert(packet, otz, 4)
			gteSXYEvery1(packet)
			s.lit1(c, advance(packet, 4))
			packet = advance(packet, 20)
		}
		p = advance(p, 16)
	}
	return packet
}

// nf3: color; vertices 0 and 1; vertex 2.
//
// libps 0x800360CC
//
//go:noinline
func (sp *objSort) nf3(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w2 := word(p, 2)
		s.project3(w2&0xffff, w2>>16, word(p, 3)&0xffff)
		w := unsafe.Slice(packet, 5)
		w[1] = word(p, 1) | s.ton
		if facing() {
			otz := gteAVSZ3()
			s.insert(packet, otz, 4)
			gteSXYEvery1(packet)
			packet = advance(packet, 20)
		}
		p = advance(p, 16)
	}
	return packet
}

// f3grd: three colors; normal and vertex 0; vertices 1 and 2. The one
// normal lights each color, and the packet is a gouraud triangle; the
// translucency bit is left out, as the original left it.
//
// libps 0x80024C18, 0x80024E60 and 0x800250A8
//
//go:noinline
func (sp *objSort) f3grd(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w4, w5 := word(p, 4), word(p, 5)
		s.project3(w4>>16, w5&0xffff, w5>>16)
		c0 := word(p, 1)&0xffffff | 0x30000000
		if facing() {
			otz := gteAVSZ3()
			s.one(c0, w4&0xffff, advance(packet, 4))
			s.one(word(p, 2), w4&0xffff, advance(packet, 12))
			s.one(word(p, 3), w4&0xffff, advance(packet, 20))
			s.insert(packet, otz, 6)
			gteSXYEvery2(packet)
			packet = advance(packet, 28)
		}
		p = advance(p, 24)
	}
	return packet
}

// g3: color; normal and vertex 0; normal and vertex 1; normal and
// vertex 2.
//
// libps 0x80036E34, 0x80036FC4 and 0x80036CF4
//
//go:noinline
func (sp *objSort) g3(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w2, w3, w4 := word(p, 2), word(p, 3), word(p, 4)
		s.project3(w2>>16, w3>>16, w4>>16)
		c := word(p, 1) | s.ton
		s.setColor(c)
		if facing() {
			otz := gteAVSZ3()
			s.light3(w2&0xffff, w3&0xffff, w4&0xffff)
			s.insert(packet, otz, 6)
			gteSXYEvery2(packet)
			s.lit3(c, packet, 2)
			packet = advance(packet, 28)
		}
		p = advance(p, 20)
	}
	return packet
}

// g3grd: three colors; three normals and vertices, each lit under its
// own color. The translucency bit is left out, as the original left it.
//
// libps 0x8002461C, 0x80024854 and 0x80024A8C
//
//go:noinline
func (sp *objSort) g3grd(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w4, w5, w6 := word(p, 4), word(p, 5), word(p, 6)
		s.project3(w4>>16, w5>>16, w6>>16)
		if facing() {
			otz := gteAVSZ3()
			s.one(word(p, 1), w4&0xffff, advance(packet, 4))
			s.one(word(p, 2), w5&0xffff, advance(packet, 12))
			s.one(word(p, 3), w6&0xffff, advance(packet, 20))
			s.insert(packet, otz, 6)
			gteSXYEvery2(packet)
			packet = advance(packet, 28)
		}
		p = advance(p, 28)
	}
	return packet
}

// ng3: three colors; vertices 0 and 1; vertex 2.
//
// libps 0x80036BAC
//
//go:noinline
func (sp *objSort) ng3(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w4 := word(p, 4)
		s.project3(w4&0xffff, w4>>16, word(p, 5)&0xffff)
		w := unsafe.Slice(packet, 7)
		w[1], w[3], w[5] = word(p, 1)|s.ton, word(p, 2), word(p, 3)
		if facing() {
			otz := gteAVSZ3()
			s.insert(packet, otz, 6)
			gteSXYEvery2(packet)
			packet = advance(packet, 28)
		}
		p = advance(p, 20)
	}
	return packet
}

// ft3: three texture coordinates, the palette and page with the first
// two; normal and vertex 0; vertices 1 and 2.
//
// libps 0x80037AEC, 0x80037C5C and 0x80037988
//
//go:noinline
func (sp *objSort) ft3(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w4, w5 := word(p, 4), word(p, 5)
		s.project3(w4>>16, w5&0xffff, w5>>16)
		c := s.texColor(word(p, 0) >> 24)
		s.setColor(c)
		if facing() {
			otz := gteAVSZ3()
			s.light1(w4 & 0xffff)
			w := unsafe.Slice(packet, 8)
			w[3], w[5], w[7] = word(p, 1), word(p, 2), word(p, 3)
			s.insert(packet, otz, 7)
			gteSXYEvery2(packet)
			s.lit1(c, advance(packet, 4))
			packet = advance(packet, 32)
		}
		p = advance(p, 24)
	}
	return packet
}

// nft3: three texture coordinates; color; vertices 0 and 1; vertex 2.
//
// libps 0x8003781C
//
//go:noinline
func (sp *objSort) nft3(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w5 := word(p, 5)
		s.project3(w5&0xffff, w5>>16, word(p, 6)&0xffff)
		w := unsafe.Slice(packet, 8)
		w[1] = s.givenColor(word(p, 4), word(p, 0)>>24)
		w[3], w[5], w[7] = word(p, 1), word(p, 2), word(p, 3)
		if facing() {
			otz := gteAVSZ3()
			s.insert(packet, otz, 7)
			gteSXYEvery2(packet)
			packet = advance(packet, 32)
		}
		p = advance(p, 28)
	}
	return packet
}

// gt3: three texture coordinates; three normals and vertices. Unlit, the
// packet is a flat textured triangle with its texture as it is.
//
// libps 0x80038740, 0x800388F4 and 0x800385E4
//
//go:noinline
func (sp *objSort) gt3(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w4, w5, w6 := word(p, 4), word(p, 5), word(p, 6)
		s.project3(w4>>16, w5>>16, w6>>16)
		mode := word(p, 0) >> 24
		c := s.texColor(mode)
		s.setColor(c)
		if facing() {
			otz := gteAVSZ3()
			if s.kind == 2 {
				w := unsafe.Slice(packet, 8)
				w[1] = 0x25808080 | (mode&2|s.code1)<<24
				w[3], w[5], w[7] = word(p, 1), word(p, 2), word(p, 3)
				s.insert(packet, otz, 7)
				gteSXYEvery2(packet)
				packet = advance(packet, 32)
			} else {
				s.light3(w4&0xffff, w5&0xffff, w6&0xffff)
				w := unsafe.Slice(packet, 10)
				w[3], w[6], w[9] = word(p, 1), word(p, 2), word(p, 3)
				s.insert(packet, otz, 9)
				gteSXYEvery3(packet)
				s.lit3(c, packet, 3)
				packet = advance(packet, 40)
			}
		}
		p = advance(p, 28)
	}
	return packet
}

// ngt3: three texture coordinates; three colors; vertices 0 and 1;
// vertex 2.
//
// libps 0x8003846C
//
//go:noinline
func (sp *objSort) ngt3(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w7 := word(p, 7)
		s.project3(w7&0xffff, w7>>16, word(p, 8)&0xffff)
		w := unsafe.Slice(packet, 10)
		w[1] = s.givenColor(word(p, 4), word(p, 0)>>24)
		w[4], w[7] = word(p, 5), word(p, 6)
		w[3], w[6], w[9] = word(p, 1), word(p, 2), word(p, 3)
		if facing() {
			otz := gteAVSZ3()
			s.insert(packet, otz, 9)
			gteSXYEvery3(packet)
			packet = advance(packet, 40)
		}
		p = advance(p, 36)
	}
	return packet
}

// f4: color; normal and vertex 0; vertices 1 and 2; vertex 3.
//
// libps 0x800368B0, 0x80036A28 and 0x80036744
//
//go:noinline
func (sp *objSort) f4(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w2, w3 := word(p, 2), word(p, 3)
		s.project3(w2>>16, w3&0xffff, w3>>16)
		c := word(p, 1) | s.ton
		s.setColor(c)
		if facing() && s.quad(word(p, 4)&0xffff, packet, 1) {
			otz := gteAVSZ4()
			s.light1(w2 & 0xffff)
			s.insert(packet, otz, 5)
			gteSXY2At(advance(packet, 20))
			s.lit1(c, advance(packet, 4))
			packet = advance(packet, 24)
		}
		p = advance(p, 20)
	}
	return packet
}

// nf4: color; vertices 0 and 1; vertices 2 and 3.
//
// libps 0x800365DC
//
//go:noinline
func (sp *objSort) nf4(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w2, w3 := word(p, 2), word(p, 3)
		s.project3(w2&0xffff, w2>>16, w3&0xffff)
		w := unsafe.Slice(packet, 6)
		w[1] = word(p, 1) | s.ton
		if facing() && s.quad(w3>>16, packet, 1) {
			otz := gteAVSZ4()
			s.insert(packet, otz, 5)
			gteSXY2At(advance(packet, 20))
			packet = advance(packet, 24)
		}
		p = advance(p, 16)
	}
	return packet
}

// g4: color; four normals and vertices. The fourth normal is lit on its
// own before the three, as the original lit it.
//
// libps 0x80037450, 0x80037630 and 0x800372DC
//
//go:noinline
func (sp *objSort) g4(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w2, w3, w4, w5 := word(p, 2), word(p, 3), word(p, 4), word(p, 5)
		s.project3(w2>>16, w3>>16, w4>>16)
		c := word(p, 1) | s.ton
		s.setColor(c)
		if facing() && s.quad(w5>>16, packet, 2) {
			otz := gteAVSZ4()
			s.light1(w5 & 0xffff)
			s.lit1(c, advance(packet, 28))
			s.light3(w2&0xffff, w3&0xffff, w4&0xffff)
			s.insert(packet, otz, 8)
			gteSXY2At(advance(packet, 32))
			s.lit3(c, packet, 2)
			packet = advance(packet, 36)
		}
		p = advance(p, 24)
	}
	return packet
}

// ng4: four colors; vertices 0 and 1; vertices 2 and 3.
//
// libps 0x8003715C
//
//go:noinline
func (sp *objSort) ng4(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w5, w6 := word(p, 5), word(p, 6)
		s.project3(w5&0xffff, w5>>16, w6&0xffff)
		w := unsafe.Slice(packet, 9)
		w[1], w[3], w[5], w[7] = word(p, 1)|s.ton, word(p, 2), word(p, 3), word(p, 4)
		if facing() && s.quad(w6>>16, packet, 2) {
			otz := gteAVSZ4()
			s.insert(packet, otz, 8)
			gteSXY2At(advance(packet, 32))
			packet = advance(packet, 36)
		}
		p = advance(p, 28)
	}
	return packet
}

// ft4: four texture coordinates; normal and vertex 0; vertices 1 and 2;
// vertex 3.
//
// libps 0x80038108, 0x800382B4 and 0x80037F6C
//
//go:noinline
func (sp *objSort) ft4(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w5, w6 := word(p, 5), word(p, 6)
		s.project3(w5>>16, w6&0xffff, w6>>16)
		c := s.texColor(word(p, 0) >> 24)
		s.setColor(c)
		w := unsafe.Slice(packet, 10)
		w[3], w[5], w[7], w[9] = word(p, 1), word(p, 2), word(p, 3), word(p, 4)
		if facing() && s.quad(word(p, 7)&0xffff, packet, 2) {
			otz := gteAVSZ4()
			s.light1(w5 & 0xffff)
			s.insert(packet, otz, 9)
			gteSXY2At(advance(packet, 32))
			s.lit1(c, advance(packet, 4))
			packet = advance(packet, 40)
		}
		p = advance(p, 32)
	}
	return packet
}

// nft4: four texture coordinates; color; vertices 0 and 1; vertices 2
// and 3.
//
// libps 0x80037DCC
//
//go:noinline
func (sp *objSort) nft4(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w6, w7 := word(p, 6), word(p, 7)
		s.project3(w6&0xffff, w6>>16, w7&0xffff)
		w := unsafe.Slice(packet, 10)
		w[1] = s.givenColor(word(p, 5), word(p, 0)>>24)
		w[3], w[5], w[7], w[9] = word(p, 1), word(p, 2), word(p, 3), word(p, 4)
		if facing() && s.quad(w7>>16, packet, 2) {
			otz := gteAVSZ4()
			s.insert(packet, otz, 9)
			gteSXY2At(advance(packet, 32))
			packet = advance(packet, 40)
		}
		p = advance(p, 32)
	}
	return packet
}

// gt4: four texture coordinates; four normals and vertices. Unlit, the
// packet is a flat textured quad with its texture as it is.
//
// libps 0x80038E00, 0x8003900C and 0x80038C68
//
//go:noinline
func (sp *objSort) gt4(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w5, w6, w7, w8 := word(p, 5), word(p, 6), word(p, 7), word(p, 8)
		s.project3(w5>>16, w6>>16, w7>>16)
		mode := word(p, 0) >> 24
		c := s.texColor(mode)
		s.setColor(c)
		if s.kind == 2 {
			w := unsafe.Slice(packet, 10)
			w[1] = 0x2d808080 | (mode&2|s.code1)<<24
			w[3], w[5], w[7], w[9] = word(p, 1), word(p, 2), word(p, 3), word(p, 4)
			if facing() && s.quad(w8>>16, packet, 2) {
				otz := gteAVSZ4()
				s.insert(packet, otz, 9)
				gteSXY2At(advance(packet, 32))
				packet = advance(packet, 40)
			}
		} else {
			w := unsafe.Slice(packet, 13)
			w[3], w[6], w[9], w[12] = word(p, 1), word(p, 2), word(p, 3), word(p, 4)
			if facing() && s.quad(w8>>16, packet, 3) {
				otz := gteAVSZ4()
				s.light1(w8 & 0xffff)
				s.lit1(c, advance(packet, 40))
				s.light3(w5&0xffff, w6&0xffff, w7&0xffff)
				s.insert(packet, otz, 12)
				gteSXY2At(advance(packet, 44))
				s.lit3(c, packet, 3)
				packet = advance(packet, 52)
			}
		}
		p = advance(p, 36)
	}
	return packet
}

// ngt4: four texture coordinates; four colors; vertices 0 and 1;
// vertices 2 and 3.
//
// libps 0x80038AAC
//
//go:noinline
func (sp *objSort) ngt4(p, packet *uint32, n uint32) *uint32 {
	s := *sp
	for ; n > 0; n-- {
		w9, w10 := word(p, 9), word(p, 10)
		s.project3(w9&0xffff, w9>>16, w10&0xffff)
		w := unsafe.Slice(packet, 13)
		w[1] = s.givenColor(word(p, 5), word(p, 0)>>24)
		w[4], w[7], w[10] = word(p, 6), word(p, 7), word(p, 8)
		w[3], w[6], w[9], w[12] = word(p, 1), word(p, 2), word(p, 3), word(p, 4)
		if facing() && s.quad(w10>>16, packet, 3) {
			otz := gteAVSZ4()
			s.insert(packet, otz, 12)
			gteSXY2At(advance(packet, 44))
			packet = advance(packet, 52)
		}
		p = advance(p, 44)
	}
	return packet
}

// GsSortObject4 sorts the model objp into ot, with depths shifted right
// by shift to index the table, and scratch as working memory for the
// subdivision the attribute's GsDIV bits ask for. The attribute's other
// bits turn the model off, make it translucent, or set its lighting: not
// lit at all, or lit with fog when the library's light mode or the
// model's own says so.
//
// libps 0x80023E58, 1988 bytes
//
//export GsSortObject4
func GsSortObject4(objp *GsDOBJ2, ot *GsOT, shift int32, scratch *uint32) {
	attr := objp.Attribute
	if int32(attr) < 0 {
		return
	}
	gsLMODE = int32(attr >> 3 & 3)
	gsLIGNR = int32(attr >> 5 & 1)
	gsLIOFF = int32(attr >> 6 & 1)
	gsNDIV = int32(attr >> 9 & 7)
	gsTON = attr >> 30 & 1
	div := gsNDIV != 0
	if div {
		sc := unsafe.Slice(scratch, 3)
		sc[0], sc[1], sc[2] = uint32(gsNDIV), screenW, screenH
	}
	kind := int32(0)
	switch {
	case gsLIOFF == 1:
		kind = 2
	case gsLIGNR == 0 && gsLightMode == 1, gsLIGNR == 1 && gsLMODE == 1:
		kind = 1
	}
	obj := unsafe.Slice(objp.Tmd, 6)
	left := obj[5]
	if left == 0 {
		return
	}
	s := objSort{
		verts: at(uintptr(obj[0])), normals: at(uintptr(obj[2])),
		org: unsafe.Pointer(ot.Org), offset: int32(ot.Offset), shift: uint32(shift),
		kind: kind, ton: gsTON << 25, code1: gsTON << 1,
	}
	if div {
		s.setupDivider()
	}
	p := (*uint32)(at(uintptr(obj[4])))
	packet := (*uint32)(unsafe.Pointer(GsOUT_PACKET_P))
	for left != 0 {
		hdr := *p
		n := hdr & 0xffff
		grd := hdr>>16&4 != 0
		code := hdr >> 24 &^ 2
		var size uintptr
		switch code {
		case 0x20:
			switch {
			case grd:
				packet, size = s.f3grd(p, packet, n), 24
			case div:
				packet, size = s.f3div(p, packet, n), 16
			default:
				packet, size = s.f3(p, packet, n), 16
			}
		case 0x21:
			if div {
				packet, size = s.nf3div(p, packet, n), 16
			} else {
				packet, size = s.nf3(p, packet, n), 16
			}
		case 0x24:
			if div {
				packet, size = s.ft3div(p, packet, n), 24
			} else {
				packet, size = s.ft3(p, packet, n), 24
			}
		case 0x25:
			if div {
				packet, size = s.nft3div(p, packet, n), 28
			} else {
				packet, size = s.nft3(p, packet, n), 28
			}
		case 0x28:
			if div {
				packet, size = s.f4div(p, packet, n), 20
			} else {
				packet, size = s.f4(p, packet, n), 20
			}
		case 0x29:
			if div {
				packet, size = s.nf4div(p, packet, n), 16
			} else {
				packet, size = s.nf4(p, packet, n), 16
			}
		case 0x2c:
			if div {
				packet, size = s.ft4div(p, packet, n), 32
			} else {
				packet, size = s.ft4(p, packet, n), 32
			}
		case 0x2d:
			if div {
				packet, size = s.nft4div(p, packet, n), 32
			} else {
				packet, size = s.nft4(p, packet, n), 32
			}
		case 0x30:
			switch {
			case grd:
				packet, size = s.g3grd(p, packet, n), 28
			case div:
				packet, size = s.g3div(p, packet, n), 20
			default:
				packet, size = s.g3(p, packet, n), 20
			}
		case 0x31:
			if div {
				packet, size = s.ng3div(p, packet, n), 24
			} else {
				packet, size = s.ng3(p, packet, n), 24
			}
		case 0x34:
			if div {
				packet, size = s.gt3div(p, packet, n), 28
			} else {
				packet, size = s.gt3(p, packet, n), 28
			}
		case 0x35:
			if div {
				packet, size = s.ngt3div(p, packet, n), 36
			} else {
				packet, size = s.ngt3(p, packet, n), 36
			}
		case 0x38:
			if div {
				packet, size = s.g4div(p, packet, n), 24
			} else {
				packet, size = s.g4(p, packet, n), 24
			}
		case 0x39:
			if div {
				packet, size = s.ng4div(p, packet, n), 28
			} else {
				packet, size = s.ng4(p, packet, n), 28
			}
		case 0x3c:
			if div {
				packet, size = s.gt4div(p, packet, n), 36
			} else {
				packet, size = s.gt4(p, packet, n), 36
			}
		case 0x3d:
			if div {
				packet, size = s.ngt4div(p, packet, n), 44
			} else {
				packet, size = s.ngt4(p, packet, n), 44
			}
		default:
			// The original printed this and went round again on the same
			// primitive, forever.
			kprintf("non supported code %x %x\n", code, uint32(uintptr(unsafe.Pointer(p))))
			return
		}
		GsOUT_PACKET_P = (*PACKET)(unsafe.Pointer(packet))
		left -= n
		p = advance(p, uintptr(n)*size)
	}
}
