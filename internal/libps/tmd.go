//go:build yaroze

package libps

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
// size the walk stays on it to the end. In the target's assembly, for its time.
//
//export GsLinkObject4
func GsLinkObject4(table uint32, obj *GsDOBJ2, n int32)

// printUnassigned prints the message for a mode with no size, n times, for
// GsLinkObject4's assembly.
//
//export libps_link_unassigned
func printUnassigned(mode, n uint32) {
	for range n {
		libps_biosA(biosPrintf, uintptr(unsafe.Pointer(unsafe.StringData(primUnassigned))), uintptr(mode), 0, 0)
	}
}

// primUnassigned is what GsLinkObject4 has the kernel print for a mode with no
// size.
const primUnassigned = "GPU CODE %02xH not assigned.\n\x00"

// GsSortObject4 transforms the primitives of obj to the screen with the
// matrix in the GTE, leaves out each that is behind the viewpoint or faces
// away, and sorts a GPU packet for each of the rest into ot at its average
// depth shifted right by shift. The flag byte's double-sided bit is one the
// Yaroze library does not support (the member FAQ's question 14), but for a
// textured flat triangle that is cut smaller. A polygon being cut works in scratch,
// where it leaves what Sony's does (spec 31).
// The depth is not clamped: an average of 0 goes into entry 0, and one
// past the table writes the word that many entries past Org, as the 2D
// sorts do (spec 04, "Depth 0 and an entry past the table").
//
// It draws polygons, lit and unlit: flat and gouraud, triangles and
// quads, with texture or without, cut smaller when the attribute asks
// (spec 04). Lit triangles with a color for each corner are never cut, and lines
// and sprites are skipped. A polygon is dropped
// when its perspective transformations erred (GTE FLAG bit 31: a point out of
// range, or a vertex too near for the division), as the reference drops it,
// unless it is subdivided.
//
// A lit polygon's colors come from the GTE: NCCS for a flat one's normal,
// NCCT (and NCCS for a quad's fourth) for a gouraud one's, the fog forms
// NCDS and NCDT in fog mode, on RGBC, the polygon's color, or 808080h with
// the mode as its code for a textured one. Lighting off copies RGBC, and takes a
// textured gouraud polygon that is not cut to a flat packet.
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
// All of that is in the target's assembly, playstation-yaroze-clean.S, each kind's loop
// spending Sony's cycles on each way through: its points bad, turned away, or sorted
// (spec 25, spec 31).
//
//export GsSortObject4
func GsSortObject4(obj *GsDOBJ2, ot *GsOT, shift int32, scratch *uint32)

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
