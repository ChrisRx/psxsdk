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

//go:extern GsDISPENV
var gsDISPENV DISPENV

// GsDRAWENV returns the drawing environment the library draws with, which a
// program changes and then registers with PutDrawEnv.
func GsDRAWENV() *DRAWENV { return &gsDRAWENV }

// GsWSMATRIX returns the world-screen matrix GsSetRefView2 and GsSetView2
// last worked out.
func GsWSMATRIX() *MATRIX { return &gsWSMATRIX }

// GsDISPENV returns the display environment GsSwapDispBuff puts up each
// frame, which a program changes to move the picture on the screen.
func GsDISPENV() *DISPENV { return &gsDISPENV }

// Packets gives GsSetWorkBase packet space declared as words, the first of
// which it is passed. The library stores words into the space, and the
// compiler aligns an array of bytes only to a byte, so a packet area
// declared as [n]PACKET works or crashes with a misaligned store by where
// the linker happens to put it.
func Packets(p *uint32) *PACKET { return (*PACKET)(unsafe.Pointer(p)) }

// The CD-ROM calls the resident library has beyond what libps.h declares,
// which together read a file whose sector is already known, as package disc
// records it, without the search of the directory CdReadFile makes. CdRead
// is in symusr.o; the other four are at the addresses the library's own
// CdReadFile calls, which the TinyGo target's link script provides, since
// Sony exported CdRead but not the CdControl that positions the drive for
// it. Signatures are libcd's.

// Commands and modes for CdControl and CdRead, from libcd.h.
const (
	CdlSetloc    = 0x02 // the parameter is the CdlLOC to read from next
	CdlModeSpeed = 0x80 // read at double speed, as CdReadFile does
)

// CdControl issues a command to the drive and returns without waiting for
// it to complete. param is the command's parameter, if it takes one, and
// result receives the drive's reply when it is not nil. It returns 0 if the
// command could not be issued.
//
//export CdControl
func CdControl(com uint8, param *uint8, result *uint8) int32

// CdControlB is CdControl waiting for the command to complete.
//
//export CdControlB
func CdControlB(com uint8, param *uint8, result *uint8) int32

// CdRead reads sectors into buf from the position the last CdlSetloc gave,
// in the given mode, and returns 0 if the read could not be started. The
// read goes on in the background, and CdReadSync waits for it.
//
//export CdRead
func CdRead(sectors int32, buf *uint32, mode int32) int32

// CdIntToPos converts a sector number to the position the drive takes, in
// binary coded minutes, seconds and sectors, writing it to p and returning
// p.
//
//export CdIntToPos
func CdIntToPos(i int32, p *CdlLOC) *CdlLOC

// CdPosToInt converts a position back to its sector number.
//
//export CdPosToInt
func CdPosToInt(p *CdlLOC) int32

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
	GsDIV2 = 2 << 9  // subdivide polygons 4x4
	GsDIV3 = 3 << 9  // subdivide polygons 8x8
	GsALON = 1 << 30 // semi-transparent
	GsDOFF = 1 << 31 // not drawn
)
