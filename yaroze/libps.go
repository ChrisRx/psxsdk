//go:build yaroze

// Package libps binds the Net Yaroze library that the boot disc leaves
// resident in the console's RAM.
//
// Nothing here is implemented in Go. Each function is a bodyless declaration
// carrying //export, which in TinyGo imports a C symbol rather than exporting
// a Go one, and the linker resolves it against the fixed addresses in symusr.S
// exactly as it does for a C program.
//
// Most of the package is generated from the SDK's libps.h by hdr2go and
// lives in zz_generated.go: the struct layouts come from clang's record
// layout dump for the target, so a Go struct that disagrees with the C one
// by a byte is a compile error rather than a disc that draws nothing. Names
// are the C ones, so the SDK documentation reads across directly.
//
// This file holds what cannot be generated. A few of the library's entry
// points are declared in the header with an empty parameter list, which in
// this vintage of C means "unspecified" rather than "none"; generating a
// zero-argument binding for those would be worse than useless, since calling
// one would compile and then pass nothing. They are written out by hand
// below, with the signature the SDK documents.
//
// Types follow the o32 ABI the console uses: C int and long are both 32 bits,
// and pointers are 32 bits.
package yaroze

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

// Video modes for SetVideoMode.
const (
	MODE_NTSC = 0
	MODE_PAL  = 1
)

// ONE is the fixed point scale the GTE and the sprite routines use: 4096
// represents 1.0.
const ONE = 4096

// Play mode and repeat count for SsSeqPlay.
const (
	SSPLAY_PAUSE    = 0
	SSPLAY_PLAY     = 1
	SSPLAY_INFINITY = 0
)

// FntPrint and KanjiFntPrint are printf-style in C. Formatting happens in Go
// instead, so these take a finished string; it must contain no percent
// directives, or the library will read arguments that were never passed.
// Buffer in text.go produces one without allocating.

//export FntPrint
func FntPrint(s *byte) int32

//export KanjiFntPrint
func KanjiFntPrint(s *byte) int32

// VSyncCallback installs a function to run on every vertical blank. It takes
// an address rather than a Go func, because the caller is C and a Go func
// value is a pair. The callback runs in interrupt context, so it must not
// allocate or block.

//export VSyncCallback
func VSyncCallback(f uintptr)

// GsGetWorkBase returns the next free byte of the packet area handed to
// GsSetWorkBase, which is how much of it the last frame used.

//export GsGetWorkBase
func GsGetWorkBase() *PACKET

// PushMatrix and PopMatrix save and restore the GTE's current
// transformation on an internal stack.

//export PushMatrix
func PushMatrix()

//export PopMatrix
func PopMatrix()

// Rand and Srand are the C library's rand and srand, which the library
// carries resident: a pseudo-random number from 0 to 32767, and the seed
// for them. They take capitals so that Go can see them.

//export rand
func Rand() int32

//export srand
func Srand(seed uint32)

// The library's own state that programs read and change directly: variables
// in the resident library, at the addresses symusr.S gives them. TinyGo only
// gives a //go:extern variable its C name where it is used in the package
// that declares it, so each is reached through a function returning its
// address, under its C name.

//go:extern GsDRAWENV
var gsDRAWENV DRAWENV

//go:extern GsWSMATRIX
var gsWSMATRIX MATRIX

// GsDRAWENV returns the drawing environment the library draws with, which a
// program changes and then registers with PutDrawEnv.
func GsDRAWENV() *DRAWENV { return &gsDRAWENV }

// GsWSMATRIX returns the world-screen matrix GsSetRefView2 and GsSetView2
// last worked out.
func GsWSMATRIX() *MATRIX { return &gsWSMATRIX }

// GsInitGraph's interlace and offset modes.
const (
	GsNONINTER = 0
	GsINTER    = 1
	GsOFSGTE   = 0
	GsOFSGPU   = 4
)

// Attribute bits of GsDOBJ2 and GsSPRITE.
const (
	GsLOFF = 1 << 6  // no lighting
	GsDIV1 = 1 << 9  // subdivide polygons 2x2
	GsALON = 1 << 30 // semi-transparent
	GsDOFF = 1 << 31 // not drawn
)
