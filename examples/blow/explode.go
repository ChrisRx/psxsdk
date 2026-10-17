package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

// The explosion: the demo's blowup.c. A model's vertices are shared
// between its polygons, so to let each polygon fly off on its own the
// model is copied with a separate set of vertices and normals for every
// primitive, and the copy is what moves. Its vertices and colors are reset
// from the original when the explosion ends.

// vertAt returns the i'th vertex of a mapped object.
func vertAt(o *demo.TMD_OBJ, i uint16) *demo.TMD_VERT {
	return (*demo.TMD_VERT)(unsafe.Pointer(uintptr(o.VertTop) + uintptr(i)*unsafe.Sizeof(demo.TMD_VERT{})))
}

// firstPrim returns the first primitive of a mapped object.
func firstPrim(o *demo.TMD_OBJ) *demo.PRIM_HDR {
	return (*demo.PRIM_HDR)(unsafe.Pointer(uintptr(o.PrimTop)))
}

// objectTable returns the objects of the TMD at addr, which has to be
// mapped.
func objectTable(addr uintptr) *[maxNumberOfObjects]demo.TMD_OBJ {
	return (*[maxNumberOfObjects]demo.TMD_OBJ)(unsafe.Pointer(addr + unsafe.Sizeof(demo.TMD_HDR{})))
}

// put writes v at the cursor and moves it on, returning where v went.
func put[T any](cursor *uintptr, v T) *T {
	p := (*T)(unsafe.Pointer(*cursor))
	*p = v
	*cursor += unsafe.Sizeof(v)
	return p
}

// blowVertices takes the explosion a step: every primitive moves along
// the average of its normals, by less the higher the speed and by an
// amount that varies from one primitive to the next so that they fly apart
// unevenly, and its colors, or the palette of the first texture found, go
// through white, yellow and red to black as count rises from 0 to 314.
// Speed runs from 1, the fastest, to 13, which does nothing.
func blowVertices(obj *yaroze.GsDOBJ2, speed uint8, count int32) {
	speed = uint8(max(1, min(int(speed), 13)))

	object := (*demo.TMD_OBJ)(unsafe.Pointer(obj.Tmd))
	prim := firstPrim(object)

	var vertex, normal [4]uint16
	var col [4]yaroze.CVECTOR
	var clut [256]uint16
	doneTex := false

	for j := uint32(0); j < object.NPrim; j++ {
		nvert := vertices(prim, &vertex)
		if nvert == 0 { // not a primitive this knows
			return
		}
		nnorm := normals(prim, &normal)

		// The primitive moves by its average normal, scaled down. One
		// without normals stays where it is, where the original divides
		// by their count.
		if nnorm > 0 {
			var snx, sny, snz int32
			for i := range nnorm {
				n := object.Norm(normal[i])
				snx += int32(n.Nx)
				sny += int32(n.Ny)
				snz += int32(n.Nz)
			}
			sd := int32(1<<speed) * (int32(j)%(int32(speed>>1)+1) + 1)
			avx := int16(snx / (sd * int32(nnorm)))
			avy := int16(sny / (sd * int32(nnorm)))
			avz := int16(snz / (sd * int32(nnorm)))
			for i := range nvert {
				v := vertAt(object, vertex[i])
				v.Vx += avx
				v.Vy += avy
				v.Vz += avz
			}
		}

		if ncol := colors(prim, &col); ncol > 0 {
			for i := range ncol {
				explosionColor(count, &col[i])
			}
			setColors(prim, &col)
		} else if !doneTex {
			if cba, tsb, ok := textureInfo(prim); ok {
				clutMode := (tsb & 0x180) >> 7
				rect := yaroze.RECT{X: int16((cba & 63) << 4), Y: int16(cba >> 6), W: 0, H: 1}
				switch clutMode {
				case 0:
					rect.W = 16
				case 1:
					rect.W = 256
				}
				if rect.W != 0 {
					words := (*uint32)(unsafe.Pointer(&clut[0]))
					yaroze.StoreImage(&rect, words)
					yaroze.DrawSync(0)
					for i := range rect.W {
						explosionClut(count, &clut[i])
					}
					yaroze.LoadImage(&rect, words)
					yaroze.DrawSync(0)
				}
				doneTex = true
			}
		}

		prim = next(prim)
	}
}

// explosionColor takes a color a step through the explosion: to white
// over the first 25 steps, to yellow over the next 35, then red, then
// black.
func explosionColor(c int32, col *yaroze.CVECTOR) {
	switch {
	case c < 25:
		if col.R < 230 {
			col.R += 25
		}
		if col.G < 230 {
			col.G += 25
		}
		if col.B < 230 {
			col.B += 25
		}
	case c < 60:
		if col.B > 9 {
			col.B -= 10
		}
		if col.G > 2 {
			col.G -= 3
		}
	default:
		if col.R > 0 {
			col.R -= 1
		}
		if col.G > 1 {
			col.G -= 2
		}
	}
}

// explosionClut is explosionColor for a palette entry, five bits a
// component.
func explosionClut(c int32, clut *uint16) {
	switch {
	case c < 25:
		if *clut&0x1f < 29 {
			*clut += 3
		}
		if *clut&0x3e0 < 928 {
			*clut += 96
		}
		if *clut&0x7c00 < 29696 {
			*clut += 3072
		}
	case c < 60:
		if *clut&0x7c00 > 0 {
			*clut -= 1024
		}
	case c < 95:
		if *clut&0x3e0 > 0 {
			*clut -= 32
		}
	default:
		if *clut&0x1f > 0 {
			*clut -= 1
		}
	}
}

// separateVertices copies the mapped TMD at in to out, giving every
// primitive vertices and normals of its own so that they can move
// independently, and returns the address past the copy. The copy's object
// table points into the copy, so the copy can be linked and drawn like the
// original, and reset from it after it is destroyed.
func separateVertices(in, out uintptr) uintptr {
	header := (*demo.TMD_HDR)(unsafe.Pointer(in))
	cursor := out
	put(&cursor, *header)

	objects := objectTable(in)
	newObjects := objectTable(out)
	for k := range header.NObjs {
		put(&cursor, objects[k])
	}

	var vertex, normal [4]uint16

	// The primitives, copied as they are.
	for k := range header.NObjs {
		prim := firstPrim(&objects[k])
		newPrim := (*demo.PRIM_HDR)(unsafe.Pointer(cursor))
		newObjects[k].PrimTop = uint32(cursor)
		for range objects[k].NPrim {
			prim, newPrim = copyPrim(prim, newPrim)
		}
		cursor = uintptr(unsafe.Pointer(newPrim))
	}

	// The vertices, one set per primitive, with the copy's primitives
	// pointed at them.
	for k := range header.NObjs {
		nv := 0
		prim := firstPrim(&objects[k])
		newObjects[k].VertTop = uint32(cursor)
		newPrim := firstPrim(&newObjects[k])
		for range objects[k].NPrim {
			n := vertices(prim, &vertex)
			for i := range n {
				put(&cursor, *vertAt(&objects[k], vertex[i]))
				vertex[i] = uint16(nv)
				nv++
			}
			setVertices(newPrim, &vertex)
			prim = next(prim)
			newPrim = next(newPrim)
		}
		newObjects[k].NVert = uint32(nv)
	}

	// The normals, likewise.
	for k := range header.NObjs {
		nn := 0
		prim := firstPrim(&objects[k])
		newObjects[k].NormTop = uint32(cursor)
		newPrim := firstPrim(&newObjects[k])
		for range objects[k].NPrim {
			n := normals(prim, &normal)
			for i := range n {
				put(&cursor, *objects[k].Norm(normal[i]))
				normal[i] = uint16(nn)
				nn++
			}
			setNormals(newPrim, &normal)
			prim = next(prim)
			newPrim = next(newPrim)
		}
		newObjects[k].NNorm = uint32(nn)
	}
	return cursor
}

// resetVertices puts one object's vertices in the copy at out back as they
// are in the original at in, which has to be the one the copy was made
// from.
func resetVertices(in, out uintptr, objectNumber int32) {
	object := &objectTable(in)[objectNumber]
	newObj := &objectTable(out)[objectNumber]

	var vertex [4]uint16
	prim := firstPrim(object)
	cursor := uintptr(newObj.VertTop)
	for range object.NPrim {
		n := vertices(prim, &vertex)
		for i := range n {
			put(&cursor, *vertAt(object, vertex[i]))
		}
		prim = next(prim)
	}
}

// resetColors puts one object's colors in the copy back as they are in
// the original.
func resetColors(in, out uintptr, objectNumber int32) {
	object := &objectTable(in)[objectNumber]
	newObj := &objectTable(out)[objectNumber]

	var col [4]yaroze.CVECTOR
	prim := firstPrim(object)
	newPrim := firstPrim(newObj)
	for range object.NPrim {
		colors(prim, &col)
		setColors(newPrim, &col)
		prim = next(prim)
		newPrim = next(newPrim)
	}
}
