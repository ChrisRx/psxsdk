//go:build yaroze

package libps

import "unsafe"

// The library's structs, at the C layout, with the fields of the yaroze
// package's, which hdr2go checked against clang's record layouts. They
// are copied rather than imported because yaroze imports this package and
// defines methods on its own. The assertions keep the sizes honest.

// A RECT is a rectangle in video memory.
type RECT struct {
	X int16
	Y int16
	W int16
	H int16
}

// A DISPENV is the display environment: what is shown and how.
type DISPENV struct {
	Disp    RECT
	Screen  RECT
	Isinter uint8
	Isrgb24 uint8
	Pad0    uint8
	Pad1    uint8
}

// A DR_ENV is the drawing environment as a GPU packet.
type DR_ENV struct {
	Tag  uint32
	Code [15]uint32
}

// A DRAWENV is the drawing environment: where the GPU draws and how.
type DRAWENV struct {
	Clip   RECT
	Ofs    [2]int16
	Tw     RECT
	Tpage  uint16
	Dtd    uint8
	Dfe    uint8
	Isbg   uint8
	R0     uint8
	G0     uint8
	B0     uint8
	Dr_env DR_ENV
}

// A GsOT_TAG is one entry of an ordering table: a 24-bit pointer to the
// next and an 8-bit length.
type GsOT_TAG uint32

// A GsOT is an ordering table.
type GsOT struct {
	Length uint32
	Org    *GsOT_TAG
	Offset uint32
	Point  uint32
	Tag    *GsOT_TAG
}

// A PACKET is a byte of GPU packet space.
type PACKET uint8

// A GsSPRITE is a sprite.
type GsSPRITE struct {
	Attribute uint32
	X         int16
	Y         int16
	W         uint16
	H         uint16
	Tpage     uint16
	U         uint8
	V         uint8
	Cx        int16
	Cy        int16
	R         uint8
	G         uint8
	B         uint8
	_pad0     [1]byte
	Mx        int16
	My        int16
	Scalex    int16
	Scaley    int16
	Rotate    int32
}

// A GsBOXF is a filled box.
type GsBOXF struct {
	Attribute uint32
	X         int16
	Y         int16
	W         uint16
	H         uint16
	R         uint8
	G         uint8
	B         uint8
	_pad0     [1]byte
}

// A GsIMAGE describes a TIM's pixels and palette in video memory.
type GsIMAGE struct {
	Pmode uint32
	Px    int16
	Py    int16
	Pw    uint16
	Ph    uint16
	Pixel *uint32
	Cx    int16
	Cy    int16
	Cw    uint16
	Ch    uint16
	Clut  *uint32
}

// A MATRIX is a rotation in 4.12 fixed point with a translation.
type MATRIX struct {
	M     [3][3]int16
	_pad0 [2]byte
	T     [3]int32
}

// An SVECTOR is a short vector.
type SVECTOR struct {
	Vx  int16
	Vy  int16
	Vz  int16
	Pad int16
}

// A VECTOR is a long vector.
type VECTOR struct {
	Vx  int32
	Vy  int32
	Vz  int32
	Pad int32
}

var _ [unsafe.Sizeof(RECT{}) - 8]byte
var _ [8 - unsafe.Sizeof(RECT{})]byte
var _ [unsafe.Sizeof(DISPENV{}) - 20]byte
var _ [20 - unsafe.Sizeof(DISPENV{})]byte
var _ [unsafe.Sizeof(DRAWENV{}) - 92]byte
var _ [92 - unsafe.Sizeof(DRAWENV{})]byte
var _ [unsafe.Sizeof(GsOT{}) - 20]byte
var _ [20 - unsafe.Sizeof(GsOT{})]byte
var _ [unsafe.Sizeof(GsSPRITE{}) - 36]byte
var _ [36 - unsafe.Sizeof(GsSPRITE{})]byte
var _ [unsafe.Sizeof(GsBOXF{}) - 16]byte
var _ [16 - unsafe.Sizeof(GsBOXF{})]byte
var _ [unsafe.Sizeof(GsIMAGE{}) - 28]byte
var _ [28 - unsafe.Sizeof(GsIMAGE{})]byte
var _ [unsafe.Sizeof(MATRIX{}) - 32]byte
var _ [32 - unsafe.Sizeof(MATRIX{})]byte
var _ [unsafe.Sizeof(SVECTOR{}) - 8]byte
var _ [8 - unsafe.Sizeof(SVECTOR{})]byte
var _ [unsafe.Sizeof(VECTOR{}) - 16]byte
var _ [16 - unsafe.Sizeof(VECTOR{})]byte

// A CdlLOC is a place on the disc as the drive addresses it: minutes,
// seconds and sectors, each in binary coded decimal.
type CdlLOC struct {
	Minute uint8
	Second uint8
	Sector uint8
	Track  uint8
}

// A CdlFILE is a file on the disc as CdSearchFile finds it.
type CdlFILE struct {
	Pos  CdlLOC
	Size uint32
	Name [16]byte
}

var _ [unsafe.Sizeof(CdlLOC{}) - 4]byte
var _ [4 - unsafe.Sizeof(CdlLOC{})]byte
var _ [unsafe.Sizeof(CdlFILE{}) - 24]byte
var _ [24 - unsafe.Sizeof(CdlFILE{})]byte

// A GsCELL is one cell of a background's map: where its picture is in
// the texture page, its color table, and its page.
type GsCELL struct {
	U     uint8
	V     uint8
	Cba   uint16
	Flag  uint16
	Tpage uint16
}

// A GsMAP is a background's map: the cells, and the index of which cell
// is at each position.
type GsMAP struct {
	Cellw  uint8
	Cellh  uint8
	Ncellw uint16
	Ncellh uint16
	Pad    [2]byte
	Base   *GsCELL
	Index  *uint16
}

// A GsBG is a background: a window onto a scrolled map.
type GsBG struct {
	Attribute uint32
	X         int16
	Y         int16
	W         int16
	H         int16
	Scrollx   int16
	Scrolly   int16
	R         uint8
	G         uint8
	B         uint8
	Pad       uint8
	Map       *GsMAP
	Mx        int16
	My        int16
	Scalex    int16
	Scaley    int16
	Rotate    int32
}

// A DIRENTRY is a file of a memory card, as the kernel lists them.
type DIRENTRY struct {
	Name   [20]byte
	Attr   int32
	Size   int32
	Next   *DIRENTRY
	Head   int32
	System [4]byte
}

var _ [unsafe.Sizeof(GsCELL{}) - 8]byte
var _ [8 - unsafe.Sizeof(GsCELL{})]byte
var _ [unsafe.Sizeof(GsMAP{}) - 16]byte
var _ [16 - unsafe.Sizeof(GsMAP{})]byte
var _ [unsafe.Sizeof(GsBG{}) - 36]byte
var _ [36 - unsafe.Sizeof(GsBG{})]byte
var _ [unsafe.Sizeof(DIRENTRY{}) - 40]byte
var _ [40 - unsafe.Sizeof(DIRENTRY{})]byte

// A GsCOORD2PARAM is what a coordinate system's matrix was made from:
// its scale, its rotation and its position.
type GsCOORD2PARAM struct {
	Scale  VECTOR
	Rotate SVECTOR
	Trans  VECTOR
}

// A GsCOORDINATE2 is a coordinate system inside another: its matrix, the
// work matrix GsGetLw keeps of the whole chain, the parameters it came
// from, and the systems above and below it. Flg is 0 when the matrix
// changed, and the swap count when the work matrix was last made.
type GsCOORDINATE2 struct {
	Flg   uint32
	Coord MATRIX
	Workm MATRIX
	Param *GsCOORD2PARAM
	Super *GsCOORDINATE2
	Sub   *GsCOORDINATE2
}

// A GsDOBJ2 is a model to draw: its attribute, its coordinate system and
// its place in a TMD, which GsLinkObject4 fills in.
type GsDOBJ2 struct {
	Attribute uint32
	Coord2    *GsCOORDINATE2
	Tmd       *uint32
	Id        uint32
}

// A GsF_LIGHT is a parallel light: its direction and its color.
type GsF_LIGHT struct {
	Vx    int32
	Vy    int32
	Vz    int32
	R     uint8
	G     uint8
	B     uint8
	_pad0 [1]byte
}

// A GsRVIEW2 is a viewpoint looking at a reference point, rolled by rz,
// in a coordinate system.
type GsRVIEW2 struct {
	Vpx   int32
	Vpy   int32
	Vpz   int32
	Vrx   int32
	Vry   int32
	Vrz   int32
	Rz    int32
	Super *GsCOORDINATE2
}

// A GsVIEW2 is a viewpoint given as its matrix, in a coordinate system.
type GsVIEW2 struct {
	View  MATRIX
	Super *GsCOORDINATE2
}

// A GsFOGPARAM is the fog: how it thickens with depth, and its color.
type GsFOGPARAM struct {
	Dqa   int16
	_pad0 [2]byte
	Dqb   int32
	Rfc   uint8
	Gfc   uint8
	Bfc   uint8
	_pad1 [1]byte
}

// A GsLINE is a line of one color.
type GsLINE struct {
	Attribute uint32
	X0        int16
	Y0        int16
	X1        int16
	Y1        int16
	R         uint8
	G         uint8
	B         uint8
	_pad0     [1]byte
}

// A GsGLINE is a line shaded from one color to another.
type GsGLINE struct {
	Attribute uint32
	X0        int16
	Y0        int16
	X1        int16
	Y1        int16
	R0        uint8
	G0        uint8
	B0        uint8
	R1        uint8
	G1        uint8
	B1        uint8
	_pad0     [2]byte
}

var _ [unsafe.Sizeof(GsCOORDINATE2{}) - 80]byte
var _ [80 - unsafe.Sizeof(GsCOORDINATE2{})]byte
var _ [unsafe.Sizeof(GsDOBJ2{}) - 16]byte
var _ [16 - unsafe.Sizeof(GsDOBJ2{})]byte
var _ [unsafe.Sizeof(GsRVIEW2{}) - 32]byte
var _ [32 - unsafe.Sizeof(GsRVIEW2{})]byte
var _ [unsafe.Sizeof(GsFOGPARAM{}) - 12]byte
var _ [12 - unsafe.Sizeof(GsFOGPARAM{})]byte
var _ [unsafe.Sizeof(GsLINE{}) - 16]byte
var _ [16 - unsafe.Sizeof(GsLINE{})]byte
var _ [unsafe.Sizeof(GsGLINE{}) - 20]byte
var _ [20 - unsafe.Sizeof(GsGLINE{})]byte
