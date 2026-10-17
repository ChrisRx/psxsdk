package demo

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// TMD layouts, from the demos' tmd.h. A TMD is a header, a table of
// objects, and each object's vertices, normals and primitives.
// GsMapModelingData turns the table's offsets into addresses in place, so
// they are kept here as words rather than Go pointers.

// TMD_HDR is a TMD's header.
type TMD_HDR struct {
	ID    uint32 // 0x41
	Flag  uint32 // 1 once the offsets are addresses
	NObjs uint32
}

// TMD_OBJ is one object in a TMD's object table.
type TMD_OBJ struct {
	VertTop uint32
	NVert   uint32
	NormTop uint32
	NNorm   uint32
	PrimTop uint32
	NPrim   uint32
	Scale   int32
}

// PRIM_HDR heads every primitive: its lengths in words, and its kind.
type PRIM_HDR struct {
	Olen, Ilen, Flag, Mode uint8
}

// TMD_VERT is a vertex.
type TMD_VERT struct {
	Vx, Vy, Vz, Pad int16
}

// TMD_NORM is a normal, in ONE units.
type TMD_NORM struct {
	Nx, Ny, Nz, Pad int16
}

// TMD_G_4T is a gouraud-shaded, textured quad.
type TMD_G_4T struct {
	U0, V0       uint8
	Cba          uint16
	U1, V1       uint8
	Tsb          uint16
	U2, V2       uint8
	Pad0         uint16
	U3, V3       uint8
	Pad1         uint16
	Norm0, Vert0 uint16
	Norm1, Vert1 uint16
	Norm2, Vert2 uint16
	Norm3, Vert3 uint16
}

// Primitive modes, the PRIM_HDR mode byte.
const (
	G_4T = 0x3c // quad, gouraud, textured
)

// TMDObject returns the first object of a mapped TMD, the start of its
// object table.
func TMDObject(t yaroze.TMD) *TMD_OBJ {
	return (*TMD_OBJ)(unsafe.Pointer(t.Table()))
}

// Norm returns the i'th normal of a mapped object.
func (o *TMD_OBJ) Norm(i uint16) *TMD_NORM {
	return (*TMD_NORM)(yaroze.Pointer(uintptr(o.NormTop) + uintptr(i)*unsafe.Sizeof(TMD_NORM{})))
}

// Prim returns the body of the i'th primitive of a mapped object, past its
// header, for an object whose primitives are all a T, as the demos assume
// of the ones they walk.
func Prim[T any](o *TMD_OBJ, i uint32) *T {
	var body T
	size := unsafe.Sizeof(PRIM_HDR{}) + unsafe.Sizeof(body)
	return (*T)(yaroze.Pointer(uintptr(o.PrimTop) + uintptr(i)*size + unsafe.Sizeof(PRIM_HDR{})))
}
