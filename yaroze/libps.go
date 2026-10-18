// Package yaroze binds the Net Yaroze library, the one the boot disc leaves
// resident in the console's RAM, for programs written in Go.
//
// Most of the package is generated from the SDK's libps.h by hdr2go: the
// struct layouts come from clang's record layout dump for the target, so a
// Go struct that disagrees with the C one by a byte is a compile error
// rather than a disc that draws nothing. Names are the C ones, so the SDK
// documentation reads across directly, and the comments are condensed from
// it. What cannot be generated is bound by hand in libps_yaroze.go, and what
// the library leaves to the program, reading the controllers and reaching
// data at fixed addresses, is in pad.go and memory.go.
//
// The library is only there on the console, so the package has two
// builds. Under the yaroze tag, which the playstation-yaroze TinyGo target
// sets, each function is a bodyless declaration carrying //export, which in
// TinyGo imports a C symbol rather than exporting a Go one, and the linker
// resolves it against the fixed addresses in the target's symusr.S exactly
// as it does for a C program. In every other build, which is what an editor,
// go vet and go build see, each function is a stub that panics, naming
// itself and saying where the real one is: a program compiles and
// type-checks on a host, and fails at its first call into the library
// rather than at the build. The host side of the toolchain, which combines
// a program with the library's image and loads one over the serial cable,
// is in the same package under the !yaroze tag.
//
// Types follow the o32 ABI the console uses: C int and long are both 32
// bits, and pointers are 32 bits. On a 64-bit host the structs that hold
// pointers are wider, and nothing there depends on their size.
package yaroze

import "unsafe"

// The header's constants are #defines, which are gone by the time clang
// produces the AST hdr2go reads, so the ones a program is likely to need
// are written out here under their C names.

// A GsOT_TAG is one entry of an ordering table.
//
// In C it is two bitfields in a word, a 24-bit pointer to the next entry and
// an 8-bit length. Go has no bitfields, so it is the word itself; nothing
// outside the library needs to look inside one, and the fields it would
// expose are addresses the library owns.
type GsOT_TAG uint32

// GsPOSITION is the drawing offset the library keeps the screen origin in,
// which POSITION returns; the struct is _GsPOSITION.
type GsPOSITION = _GsPOSITION

// Video modes for SetVideoMode.
const (
	MODE_NTSC = 0
	MODE_PAL  = 1
)

// ONE is the fixed point scale the GTE and the sprite routines use: 4096
// represents 1.0.
const ONE = 4096

// WORLD is the world coordinate system, the parent of a top-level
// GsCOORDINATE2, which the library represents as a nil pointer.
var WORLD *GsCOORDINATE2

// IDMATRIX is the identity matrix, for a coordinate system or a view that
// starts out unturned. The library has its own, GsIDMATRIX, which
// GsInitGraph sets up; this one is there from the start.
var IDMATRIX = MATRIX{M: [3][3]int16{
	{ONE, 0, 0},
	{0, ONE, 0},
	{0, 0, ONE},
}}

// GsInitGraph's interlace and offset modes.
const (
	GsNONINTER = 0
	GsINTER    = 1
	GsOFSGTE   = 0
	GsOFSGPU   = 4
)

// Attribute bits of GsDOBJ2 and GsSPRITE.
const (
	GsFOG    = 1 << 3  // light with fog, when GsLLMOD is set
	GsLLMOD  = 1 << 5  // the object's own GsFOG bit decides, not GsSetLightMode
	GsLOFF   = 1 << 6  // no lighting
	GsDIV1   = 1 << 9  // subdivide polygons 2x2
	GsDIV2   = 2 << 9  // subdivide polygons 4x4
	GsDIV3   = 3 << 9  // subdivide polygons 8x8
	GsDIV4   = 4 << 9  // subdivide polygons 16x16
	GsDIV5   = 5 << 9  // subdivide polygons 32x32
	GsROTOFF = 1 << 27 // a sprite or background drawn without rotation or scaling
	GsALON   = 1 << 30 // semi-transparent
	GsDOFF   = 1 << 31 // not drawn
)

// Lighting modes for GsSetLightMode.
const (
	GsLMODE_NORMAL = 0
	GsLMODE_FOG    = 1
	GsLMODE_LOFF   = 2
)

// Play mode and repeat count for SsSeqPlay.
const (
	SSPLAY_PAUSE    = 0
	SSPLAY_PLAY     = 1
	SSPLAY_INFINITY = 0
)

// Reverb types for SsUtSetReverbType.
const (
	SS_REV_TYPE_OFF      = 0
	SS_REV_TYPE_ROOM     = 1
	SS_REV_TYPE_STUDIO_A = 2
	SS_REV_TYPE_STUDIO_B = 3
	SS_REV_TYPE_STUDIO_C = 4
	SS_REV_TYPE_HALL     = 5
	SS_REV_TYPE_SPACE    = 6
	SS_REV_TYPE_ECHO     = 7
	SS_REV_TYPE_DELAY    = 8
	SS_REV_TYPE_PIPE     = 9
)

// Mute modes for SsSetMute, and the disc audio's attributes and modes for
// SsSetSerialAttr.
const (
	SS_MUTE_OFF = 0
	SS_MUTE_ON  = 1
	SS_CD       = 0 // the disc, the one serial source
	SS_SOFF     = 0
	SS_SON      = 1
	SS_MIX      = 0
	SS_REV      = 1
)

// RCntCNT2 is the root counter that counts the system clock divided by
// eight, for GetRCnt.
const RCntCNT2 = 0xf2000002

// Commands and modes for CdControl and CdRead, from libcd.h.
const (
	CdlSetloc    = 0x02 // the parameter is the CdlLOC to read from next
	CdlModeSpeed = 0x80 // read at double speed, as CdReadFile does
)

// Packets gives GsSetWorkBase packet space declared as words, the first of
// which it is passed. The library stores words into the space, and the
// compiler aligns an array of bytes only to a byte, so a packet area
// declared as [n]PACKET works or crashes with a misaligned store by where
// the linker happens to put it.
//
//go:inline
func Packets(p *uint32) *PACKET { return (*PACKET)(unsafe.Pointer(p)) }
