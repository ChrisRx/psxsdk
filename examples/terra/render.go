package main

// Drawing a model: the GPU packets its primitives become, built in the
// frame's packet space and linked into the ordering table at the depth the
// GTE averages for them, as the original's own routines did rather than
// through GsSortObject4. Each kind of primitive has a routine, chosen by
// the code in front of its run: a flat or gouraud, three or four sided,
// textured or plain polygon, lit from its normals; the kinds with bit 24
// set take vertices and normals a transforming record projected and lit
// earlier, shared between bones, and color them under the run's color
// through the rotation matrix set to that color.

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// A drawContext is what one node's primitives are drawn with. It holds
// the node's arrays as pointers, since the routines read them a word at a
// time and this is the hottest code in the game.
type drawContext struct {
	m      *model
	words  unsafe.Pointer // the model's first word
	ot     *yaroze.GsOT
	shift  uint32
	max    int32          // the deepest entry of the table
	list   []int          // the node's arrays, as word indexes into the model
	verts  unsafe.Pointer // the node's vertices, its second array
	norms  unsafe.Pointer // its normals, the third
	proj   unsafe.Pointer // its projected vertices, the third, for the shared kinds
	lights unsafe.Pointer // their lit normals, the fifth
}

// word returns word k from p on.
//
//go:inline
func word(p unsafe.Pointer, k int) uint32 { return *(*uint32)(unsafe.Add(p, 4*k)) }

// Working storage for the routines, which hand their addresses to the GTE.
var (
	sxy    [3]uint32
	sxy3   uint32
	rgb    [3]uint32
	sz     [4]uint32
	lit    [3]int32
	colorM yaroze.MATRIX
	envXYZ [4]yaroze.VECTOR
	envN   [4]yaroze.VECTOR
)

// drawActor draws the actor: its bones' chains under their matrices, or the
// model whole when it plays no animation, into ot with every depth moved
// down by offset entries, which puts the model behind what is sorted at
// the same depths without it.
func (a *actor) drawActor(ot *yaroze.GsOT, offset int32) {
	m := models[a.model]
	ctx := drawContext{
		m:     m,
		words: unsafe.Pointer(&m.words[0]),
		ot:    &shiftedOT,
		shift: 14 - ot.Length,
		max:   1<<ot.Length - 1 - offset,
	}
	shiftedOT = *ot
	shiftedOT.Org = (*yaroze.GsOT_TAG)(unsafe.Add(unsafe.Pointer(ot.Org), 4*offset))
	if a.anim == -1 {
		yaroze.GsGetLws(&a.root, &lwMatrix, &lsMatrix)
		yaroze.GsSetLightMatrix(&lwMatrix)
		yaroze.GsSetLsMatrix(&lsMatrix)
		ctx.drawChain(m.chains[1])
		return
	}
	for i := 1; i < m.nparts; i++ {
		chain := m.chains[i]
		if chain == 0 {
			continue
		}
		if i-1 < len(a.bones) {
			yaroze.GsGetLws(&a.bones[i-1], &lwMatrix, &lsMatrix)
			yaroze.GsSetLightMatrix(&lwMatrix)
			yaroze.GsSetLsMatrix(&lsMatrix)
		}
		ctx.drawChain(chain)
	}
}

var (
	shiftedOT yaroze.GsOT
	lwMatrix  yaroze.MATRIX
	lsMatrix  yaroze.MATRIX
)

// drawChain draws every node of the chain starting at word index node.
func (ctx *drawContext) drawChain(node int) {
	w := ctx.m.words
	for {
		ctx.list = ctx.m.list(node)
		ctx.verts, ctx.norms, ctx.proj, ctx.lights = nil, nil, nil, nil
		if len(ctx.list) > 1 {
			ctx.verts = unsafe.Add(ctx.words, 4*ctx.list[1])
		}
		if len(ctx.list) > 2 {
			ctx.norms = unsafe.Add(ctx.words, 4*ctx.list[2])
			ctx.proj = ctx.norms
		}
		if len(ctx.list) > 4 {
			ctx.lights = unsafe.Add(ctx.words, 4*ctx.list[4])
		}
		n := int(w[node+nodeCount] & 0x7fffffff)
		rec := node + nodePrims
		for ; n > 0; n-- {
			rec = ctx.drawPrims(rec)
		}
		next := int32(w[node+nodeNext])
		if next == -1 {
			return
		}
		node = int(next)
	}
}

// drawPrims draws the run of primitives recorded at word index rec and
// returns the next record.
func (ctx *drawContext) drawPrims(rec int) int {
	w := ctx.m.words
	code := w[rec+primCode] &^ 0x800000
	if code == primXform {
		ctx.transform(rec)
		return rec + primXformN
	}
	count := int(w[rec+primCount]&0x7fffffff) >> 16
	data := ctx.list[0] + int(w[rec+primData])
	switch code {
	case 0xc:
		ctx.flat3(data, count)
	case 0xd:
		ctx.gouraudTex3(data, count)
	case 0x14:
		ctx.flat4(data, count)
	case 0x15:
		if ctx.m.env {
			ctx.envTex4(data, count)
		} else {
			ctx.gouraudTex4(data, count)
		}
	case 0x100000c:
		ctx.sharedFlat3(data, count)
	case 0x100000d:
		ctx.sharedTex3(data, count)
	case 0x1000014:
		ctx.sharedFlat4(data, count)
	case 0x1000015:
		ctx.sharedTex4(data, count)
	case 0x200015:
		ctx.gouraudTex4(data, count)
	}
	return rec + primSize
}

// packet returns the next n words of packet space without taking them,
// so that a polygon the GTE rejects leaves nothing behind; commit takes
// them.
func packet(n int) []uint32 {
	return unsafe.Slice((*uint32)(unsafe.Pointer(yaroze.PacketPointer())), n)
}

// commit takes the n words packet returned.
func commit(n int) {
	yaroze.SetPacketPointer((*yaroze.PACKET)(unsafe.Add(unsafe.Pointer(yaroze.PacketPointer()), 4*n)))
}

// link puts the packet of n words into the table at the depth otz gives,
// which the original let run past the table and the port holds within it.
//
//go:inline
func (ctx *drawContext) link(p []uint32, n int, otz int32) {
	tag := (*uint32)(unsafe.Add(unsafe.Pointer(ctx.ot.Org), 4*min(otz>>ctx.shift, ctx.max)))
	p[0] = uint32(n-1)<<24 | *tag&0xffffff
	*tag = *tag&0xff000000 | uint32(uintptr(unsafe.Pointer(&p[0])))&0xffffff
	commit(n)
}

// vertex returns vertex i of the node's second array, and normal returns
// normal i of its third.
//
//go:inline
func (ctx *drawContext) vertex(i uint32) *yaroze.SVECTOR {
	return (*yaroze.SVECTOR)(unsafe.Add(ctx.verts, 8*i))
}

//go:inline
func (ctx *drawContext) normal(i uint32) *yaroze.SVECTOR {
	return (*yaroze.SVECTOR)(unsafe.Add(ctx.norms, 8*i))
}

// run returns a pointer to word index data, the start of a run.
//
//go:inline
func (ctx *drawContext) run(data int) unsafe.Pointer { return unsafe.Add(ctx.words, 4*data) }

// flat3 draws flat colored triangles, lit at each corner: a color word and
// three words of normal and vertex, four in all.
func (ctx *drawContext) flat3(data, count int) {
	d := ctx.run(data)
	for ; count > 0; count, d = count-1, unsafe.Add(d, 16) {
		color, w1, w2, w3 := word(d, 0), word(d, 1), word(d, 2), word(d, 3)
		flag := yaroze.RotTransPers3(ctx.vertex(w1>>16), ctx.vertex(w2>>16), ctx.vertex(w3>>16), &sxy)
		if flag < 0 || yaroze.NormalClip() <= 0 {
			continue
		}
		otz := yaroze.AverageZ3()
		yaroze.SetRGBC(color)
		yaroze.NormalColorCol3(ctx.normal(w1&0xffff), ctx.normal(w2&0xffff), ctx.normal(w3&0xffff), &rgb)
		p := packet(7)
		p[1], p[3], p[5] = rgb[0], rgb[1], rgb[2]
		p[2], p[4], p[6] = sxy[0], sxy[1], sxy[2]
		ctx.link(p, 7, otz)
	}
}

// flat4 draws flat colored quads, lit at each corner: a color word and
// four words of normal and vertex.
func (ctx *drawContext) flat4(data, count int) {
	d := ctx.run(data)
	for ; count > 0; count, d = count-1, unsafe.Add(d, 20) {
		color, w1, w2, w3, w4 := word(d, 0), word(d, 1), word(d, 2), word(d, 3), word(d, 4)
		flag := yaroze.RotTransPers3(ctx.vertex(w1>>16), ctx.vertex(w2>>16), ctx.vertex(w3>>16), &sxy)
		if flag < 0 || yaroze.NormalClip() <= 0 {
			continue
		}
		p := packet(9)
		p[2], p[4], p[6] = sxy[0], sxy[1], sxy[2]
		yaroze.RotTransPers(ctx.vertex(w4>>16), &sxy3)
		otz := yaroze.AverageZ4()
		yaroze.SetRGBC(color)
		p[7] = yaroze.NormalColorCol(ctx.normal(w4 & 0xffff))
		yaroze.NormalColorCol3(ctx.normal(w1&0xffff), ctx.normal(w2&0xffff), ctx.normal(w3&0xffff), &rgb)
		p[1], p[3], p[5] = rgb[0], rgb[1], rgb[2]
		p[8] = sxy3
		ctx.link(p, 9, otz)
	}
}

// gouraudTex3 draws textured triangles lit at each corner: the three
// texture words of a TMD's, then three words of normal and vertex.
func (ctx *drawContext) gouraudTex3(data, count int) {
	d := ctx.run(data)
	yaroze.SetRGBC(0x34808080)
	for ; count > 0; count, d = count-1, unsafe.Add(d, 24) {
		w3, w4, w5 := word(d, 3), word(d, 4), word(d, 5)
		flag := yaroze.RotTransPers3(ctx.vertex(w3>>16), ctx.vertex(w4>>16), ctx.vertex(w5>>16), &sxy)
		p := packet(10)
		p[3], p[6], p[9] = word(d, 0), word(d, 1), word(d, 2)&0xffff
		if flag < 0 || yaroze.NormalClip() <= 0 {
			continue
		}
		otz := yaroze.AverageZ3()
		yaroze.NormalColorCol3(ctx.normal(w3&0xffff), ctx.normal(w4&0xffff), ctx.normal(w5&0xffff), &rgb)
		p[2], p[5], p[8] = sxy[0], sxy[1], sxy[2]
		p[1], p[4], p[7] = rgb[0], rgb[1], rgb[2]
		ctx.link(p, 10, otz)
	}
}

// gouraudTex4 draws textured quads lit at each corner: seven words, the
// first normal packed over the third corner's texture coordinates and the
// first vertex over the fourth's.
func (ctx *drawContext) gouraudTex4(data, count int) {
	d := ctx.run(data)
	yaroze.SetRGBC(0x3c808080)
	for ; count > 0; count, d = count-1, unsafe.Add(d, 28) {
		w2, w3, w4, w5, w6 := word(d, 2), word(d, 3), word(d, 4), word(d, 5), word(d, 6)
		flag := yaroze.RotTransPers3(ctx.vertex(w3>>16), ctx.vertex(w4>>16), ctx.vertex(w5>>16), &sxy)
		p := packet(13)
		p[3], p[6], p[9] = word(d, 0), word(d, 1), w2&0xffff
		if flag < 0 || yaroze.NormalClip() <= 0 {
			continue
		}
		p[2], p[5], p[8] = sxy[0], sxy[1], sxy[2]
		yaroze.RotTransPers(ctx.vertex(w6>>16), &sxy3)
		p[12] = w3 & 0xffff
		otz := yaroze.AverageZ4()
		p[10] = yaroze.NormalColorCol(ctx.normal(w6 & 0xffff))
		yaroze.NormalColorCol3(ctx.normal(w2>>16), ctx.normal(w4&0xffff), ctx.normal(w5&0xffff), &rgb)
		p[11] = sxy3
		p[1], p[4], p[7] = rgb[0], rgb[1], rgb[2]
		ctx.link(p, 13, otz)
	}
}

// envTex4 draws the ship's quads: gouraudTex4 with the texture
// coordinates worked out from where each corner and its normal point in
// the view, so that the sea's texture slides over them like a reflection,
// from a fixed texture page and palette.
func (ctx *drawContext) envTex4(data, count int) {
	d := ctx.run(data)
	yaroze.SetRGBC(0x3c808080)
	for ; count > 0; count, d = count-1, unsafe.Add(d, 28) {
		w2, w3, w4, w5, w6 := word(d, 2), word(d, 3), word(d, 4), word(d, 5), word(d, 6)
		flag := yaroze.RotTransPers3(ctx.vertex(w3>>16), ctx.vertex(w4>>16), ctx.vertex(w5>>16), &sxy)
		p := packet(13)
		p[6] = 0xc << 16
		p[3] = 0x6800 << 16
		if flag < 0 || yaroze.NormalClip() <= 0 {
			continue
		}
		p[2], p[5], p[8] = sxy[0], sxy[1], sxy[2]
		yaroze.RotTransPers(ctx.vertex(w6>>16), &sxy3)
		otz := yaroze.AverageZ4()
		p[10] = yaroze.NormalColorCol(ctx.normal(w6 & 0xffff))
		yaroze.NormalColorCol3(ctx.normal(w2>>16), ctx.normal(w4&0xffff), ctx.normal(w5&0xffff), &rgb)
		p[11] = sxy3
		p[1], p[4], p[7] = rgb[0], rgb[1], rgb[2]
		yaroze.RotTrans(ctx.vertex(w3>>16), &envXYZ[0])
		yaroze.RotTrans(ctx.vertex(w4>>16), &envXYZ[1])
		yaroze.RotTrans(ctx.vertex(w5>>16), &envXYZ[2])
		yaroze.RotTrans(ctx.vertex(w6>>16), &envXYZ[3])
		yaroze.RotTrans(ctx.normal(w2>>16), &envN[0])
		yaroze.RotTrans(ctx.normal(w4&0xffff), &envN[1])
		yaroze.RotTrans(ctx.normal(w5&0xffff), &envN[2])
		yaroze.RotTrans(ctx.normal(w6&0xffff), &envN[3])
		for i := range 4 {
			v, n := &envXYZ[i], &envN[i]
			u := n.Vx >> 5
			t := n.Vy >> 5
			if v.Vz != 0 {
				u += (v.Vx << 5) / v.Vz
				t += (v.Vy << 5) / v.Vz
			}
			u = min(max(u+0x40, 0), 0x7f)
			t = min(max(t+0x40, 0), 0x7f)
			word := 3 + 3*i
			p[word] = p[word]&0xffff0000 | uint32(uint8(u)) | uint32(uint8(t+0x40))<<8
		}
		ctx.link(p, 13, otz)
	}
}

// transform projects the vertices and lights the normals a transforming
// record names, from the node's arrays into its buffers, for the shared
// kinds to draw from: each vertex's screen point and depth, or -1 for one
// the GTE could not project, and each normal's light.
func (ctx *drawContext) transform(rec int) {
	w := ctx.m.words
	n, src, dst := int(w[rec+2]), ctx.list[1]+2*int(w[rec+3]), ctx.list[2]+2*int(w[rec+4])
	for i := 0; i < n; i, src, dst = i+1, src+2, dst+2 {
		if yaroze.RotTransPers(ctx.m.svector(src), &w[dst]) < 0 {
			w[dst+1] = 0xffffffff
		} else {
			w[dst+1] = uint32(uint16(yaroze.ReadSZ3())) | uint32(uint16(yaroze.ReadMAC0()))<<16
		}
	}
	n, src, dst = int(w[rec+5]), ctx.list[3]+2*int(w[rec+6]), ctx.list[4]+2*int(w[rec+7])
	for i := 0; i < n; i, src, dst = i+1, src+2, dst+2 {
		yaroze.NormalColor(ctx.m.svector(src), (*[3]int16)(unsafe.Pointer(&w[dst])))
	}
}

// shared returns the screen point and depth word of transformed vertex i,
// from the node's third array.
//
//go:inline
func (ctx *drawContext) shared(i uint32) (sxy, sz uint32) {
	at := unsafe.Add(ctx.proj, 8*i)
	return word(at, 0), word(at, 1)
}

// light returns the light on transformed normal i, from the node's fifth
// array.
//
//go:inline
func (ctx *drawContext) light(i uint32) *[3]int16 {
	return (*[3]int16)(unsafe.Add(ctx.lights, 8*i))
}

// setColorMatrix puts the color on the rotation matrix's diagonal, for
// RotColor to light with.
func setColorMatrix(r, g, b int16) {
	colorM = yaroze.MATRIX{}
	colorM.M[0][0], colorM.M[1][1], colorM.M[2][2] = r, g, b
	yaroze.SetRotMatrix(&colorM)
}

// litColor returns the light on normal i under the color matrix, packed
// with the code, each component held to 255.
func (ctx *drawContext) litColor(i uint32, code uint32) uint32 {
	yaroze.RotColor(ctx.light(i), &lit)
	return code | uint32(min(lit[0], 0xff)) | uint32(min(lit[1], 0xff))<<8 | uint32(min(lit[2], 0xff))<<16
}

// sharedFlat3 draws flat colored triangles from transformed vertices: the
// layout of flat3, under the run's color.
func (ctx *drawContext) sharedFlat3(data, count int) {
	d := ctx.run(data)
	for ; count > 0; count, d = count-1, unsafe.Add(d, 16) {
		color, w1, w2, w3 := word(d, 0), word(d, 1), word(d, 2), word(d, 3)
		s0, z0 := ctx.shared(w1 >> 16)
		s1, z1 := ctx.shared(w2 >> 16)
		s2, z2 := ctx.shared(w3 >> 16)
		if z0 == 0xffffffff || z1 == 0xffffffff || z2 == 0xffffffff {
			continue
		}
		sxy[0], sxy[1], sxy[2] = s0, s1, s2
		sz[0], sz[1], sz[2] = z0&0xffff, z1&0xffff, z2&0xffff
		yaroze.LoadScreen3(&sxy, (*[3]uint32)(sz[:3]))
		if yaroze.NormalClip() <= 0 {
			continue
		}
		otz := yaroze.AverageZ3()
		p := packet(7)
		p[2], p[4], p[6] = s0, s1, s2
		setColorMatrix(int16(color&0xff), int16(color>>8&0xff), int16(color>>16&0xff))
		p[1] = ctx.litColor(w1&0xffff, 0x30000000)
		p[3] = ctx.litColor(w2&0xffff, 0x30000000)
		p[5] = ctx.litColor(w3&0xffff, 0x30000000)
		ctx.link(p, 7, otz)
	}
}

// sharedFlat4 draws flat colored quads from transformed vertices: the
// layout of flat4, under the run's color.
func (ctx *drawContext) sharedFlat4(data, count int) {
	d := ctx.run(data)
	for ; count > 0; count, d = count-1, unsafe.Add(d, 20) {
		color, w1, w2, w3, w4 := word(d, 0), word(d, 1), word(d, 2), word(d, 3), word(d, 4)
		s0, z0 := ctx.shared(w1 >> 16)
		s1, z1 := ctx.shared(w2 >> 16)
		s2, z2 := ctx.shared(w3 >> 16)
		s3, z3 := ctx.shared(w4 >> 16)
		if z0 == 0xffffffff || z1 == 0xffffffff || z2 == 0xffffffff || z3 == 0xffffffff {
			continue
		}
		sxy[0], sxy[1], sxy[2] = s0, s1, s2
		sz[0], sz[1], sz[2], sz[3] = z0&0xffff, z1&0xffff, z2&0xffff, z3&0xffff
		yaroze.LoadScreen4(&sxy, &sz)
		if yaroze.NormalClip() <= 0 {
			continue
		}
		otz := yaroze.AverageZ4()
		p := packet(9)
		p[2], p[4], p[6], p[8] = s0, s1, s2, s3
		setColorMatrix(int16(color&0xff), int16(color>>8&0xff), int16(color>>16&0xff))
		p[1] = ctx.litColor(w1&0xffff, 0x38000000)
		p[3] = ctx.litColor(w2&0xffff, 0x38000000)
		p[5] = ctx.litColor(w3&0xffff, 0x38000000)
		p[7] = ctx.litColor(w4&0xffff, 0x38000000)
		ctx.link(p, 9, otz)
	}
}

// sharedTex3 draws textured triangles from transformed vertices, the layout
// of gouraudTex3, at half brightness.
func (ctx *drawContext) sharedTex3(data, count int) {
	d := ctx.run(data)
	setColorMatrix(0x80, 0x80, 0x80)
	for ; count > 0; count, d = count-1, unsafe.Add(d, 24) {
		w3, w4, w5 := word(d, 3), word(d, 4), word(d, 5)
		s0, z0 := ctx.shared(w3 >> 16)
		s1, z1 := ctx.shared(w4 >> 16)
		s2, z2 := ctx.shared(w5 >> 16)
		if z0 == 0xffffffff || z1 == 0xffffffff || z2 == 0xffffffff {
			continue
		}
		sxy[0], sxy[1], sxy[2] = s0, s1, s2
		sz[0], sz[1], sz[2] = z0&0xffff, z1&0xffff, z2&0xffff
		yaroze.LoadScreen3(&sxy, (*[3]uint32)(sz[:3]))
		if yaroze.NormalClip() <= 0 {
			continue
		}
		otz := yaroze.AverageZ3()
		p := packet(10)
		p[2], p[5], p[8] = s0, s1, s2
		p[1] = ctx.litColor(w3&0xffff, 0x34000000)
		p[4] = ctx.litColor(w4&0xffff, 0x34000000)
		p[7] = ctx.litColor(w5&0xffff, 0x34000000)
		p[3], p[6], p[9] = word(d, 0), word(d, 1), word(d, 2)&0xffff
		ctx.link(p, 10, otz)
	}
}

// sharedTex4 draws textured quads from transformed vertices, the layout of
// gouraudTex4, at half brightness. The original lights the second, third
// and fourth corners by their vertex numbers rather than their normals',
// which this keeps.
func (ctx *drawContext) sharedTex4(data, count int) {
	d := ctx.run(data)
	setColorMatrix(0x80, 0x80, 0x80)
	for ; count > 0; count, d = count-1, unsafe.Add(d, 28) {
		w2, w3, w4, w5, w6 := word(d, 2), word(d, 3), word(d, 4), word(d, 5), word(d, 6)
		s0, z0 := ctx.shared(w3 >> 16)
		s1, z1 := ctx.shared(w4 >> 16)
		s2, z2 := ctx.shared(w5 >> 16)
		s3, z3 := ctx.shared(w6 >> 16)
		if z0 == 0xffffffff || z1 == 0xffffffff || z2 == 0xffffffff || z3 == 0xffffffff {
			continue
		}
		sxy[0], sxy[1], sxy[2] = s0, s1, s2
		sz[0], sz[1], sz[2], sz[3] = z0&0xffff, z1&0xffff, z2&0xffff, z3&0xffff
		yaroze.LoadScreen4(&sxy, &sz)
		if yaroze.NormalClip() <= 0 {
			continue
		}
		otz := yaroze.AverageZ4()
		p := packet(13)
		p[2], p[5], p[8], p[11] = s0, s1, s2, s3
		p[1] = ctx.litColor(w2>>16, 0x3c000000)
		p[4] = ctx.litColor(w3>>16, 0x3c000000)
		p[7] = ctx.litColor(w4>>16, 0x3c000000)
		p[10] = ctx.litColor(w5>>16, 0x3c000000)
		p[3], p[6], p[9], p[12] = word(d, 0), word(d, 1), w2&0xffff, w3&0xffff
		ctx.link(p, 13, otz)
	}
}
