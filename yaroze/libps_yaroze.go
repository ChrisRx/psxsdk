//go:build yaroze

package yaroze

import "unsafe"

// What hdr2go cannot generate, bound by hand. A few of the library's entry
// points are declared in the header with an empty parameter list, which in
// this vintage of C means "unspecified" rather than "none"; generating a
// zero-argument binding for those would be worse than useless, since calling
// one would compile and then pass nothing. They are written out below with
// the signature the SDK documents. One, GsLinkObject4, is declared with a
// pointer typed as an integer, and gets a Go signature instead. The stubs
// for every function here are in libps_stub.go, with the same comments.

// FntPrint writes s to the current print stream, for FntFlush to draw.
//
// In C it is printf-style. Formatting happens in Go instead, so it takes a
// finished C string, which Buffer builds without allocating and CString
// makes from a literal; it must contain no percent directives, or the
// library will read arguments that were never passed. It returns the number
// of characters in the stream.
//
//export FntPrint
func FntPrint(s *byte) int32

// KanjiFntPrint writes the Shift JIS string s to the current Kanji print
// stream, for KanjiFntFlush to draw, with the same caution about percent
// directives as FntPrint. It returns the number of characters in the
// stream.
//
//export KanjiFntPrint
func KanjiFntPrint(s *byte) int32

// VSyncCallback installs the C function at f to run at the start of every
// vertical blank, or removes the callback with f 0. A Go program uses
// OnVSync, which installs one that calls a Go func.
//
//export VSyncCallback
func VSyncCallback(f uintptr)

// OnVSync has f run at the start of every vertical blank, from the
// library's interrupt, or stops it with nil. Interrupts are off while f
// runs, so it must be quick, and it must not allocate or wait for anything,
// which the build refuses in f and in what it calls.
func OnVSync(f func()) {
	VSyncCallback(0) // f is two words, not to be called half written
	vsyncFunc = f
	if f != nil {
		h := vsyncHandler
		VSyncCallback((*[2]uintptr)(unsafe.Pointer(&h))[1])
	}
}

// vsyncFunc is what OnVSync was given.
var vsyncFunc func()

// vsyncHandler is what the library calls, a function and not a closure, so
// that its address alone is enough to call it: a Go func value is the
// address and a context, which a C caller does not pass, and which a
// function that is not a closure does not use.
func vsyncHandler() {
	if f := vsyncFunc; f != nil {
		f()
	}
}

// GsGetWorkBase returns the next free byte of the packet area handed to
// GsSetWorkBase, which is how much of it the frame has used so far.
//
//export GsGetWorkBase
func GsGetWorkBase() *PACKET

// PushMatrix saves the GTE's current rotation matrix on a stack of twenty.
//
//export PushMatrix
func PushMatrix()

// PopMatrix restores the GTE's rotation matrix from the stack PushMatrix
// saved it on.
//
//export PopMatrix
func PopMatrix()

// Rand returns a pseudo-random number from 0 to 32767, from the C library
// the resident library carries. It takes a capital so that Go can see it.
//
//export rand
func Rand() int32

// Srand seeds Rand; a seed of 1 is where it starts. It takes a capital so
// that Go can see it.
//
//export srand
func Srand(seed uint32)

//export GsLinkObject4
func gsLinkObject4(tmd_base uint32, objp *GsDOBJ2, n int32)

// GsLinkObject4 ties the object objp to the nth model in the TMD whose
// object table is at tmd, after GsMapModelingData has turned the file's
// offsets into addresses, so that GsSortObject4 can draw it. The object
// table is the file's fourth word, after the ID, the flags and the count.
//
// The header types the table as an integer; this takes the pointer a
// program has, and inlines to the same call.
//
//go:inline
func GsLinkObject4(tmd *uint32, objp *GsDOBJ2, n int32) {
	gsLinkObject4(uint32(uintptr(unsafe.Pointer(tmd))), objp, n)
}

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

//go:extern POSITION
var position _GsPOSITION

//go:extern GsOUT_PACKET_P
var gsOUT_PACKET_P *PACKET

// GsDRAWENV returns the drawing environment the library draws with, which a
// program changes and then registers with PutDrawEnv.
func GsDRAWENV() *DRAWENV { return &gsDRAWENV }

// GsWSMATRIX returns the world-screen matrix GsSetRefView2 and GsSetView2
// last worked out.
func GsWSMATRIX() *MATRIX { return &gsWSMATRIX }

// GsDISPENV returns the display environment GsSwapDispBuff puts up each
// frame, which a program changes to move the picture on the screen.
func GsDISPENV() *DISPENV { return &gsDISPENV }

// POSITION returns the screen origin the library draws from in GsOFSGPU
// mode: the offset GsSwapDispBuff adds to every coordinate, which
// GsInitGraph sets to the top left corner and GsInit3D to the middle.
func POSITION() *GsPOSITION { return &position }

// PacketPointer returns GsOUT_PACKET_P, where the library builds the next
// GPU packet in the space GsSetWorkBase gave it, and SetPacketPointer moves
// it, for a program that builds packets of its own there; AllocPacket does
// both.
func PacketPointer() *PACKET { return gsOUT_PACKET_P }

// SetPacketPointer sets GsOUT_PACKET_P; see PacketPointer.
func SetPacketPointer(p *PACKET) { gsOUT_PACKET_P = p }

// NativeVideoMode returns the video mode the console is made for, MODE_PAL
// for a European one and MODE_NTSC for any other, from the region letter
// at the end of the version string Sony's BIOS carries: "System ROM
// Version 3.0 01/06/97 E". A BIOS without one, the first Japanese one or
// an OpenBIOS, is taken for NTSC.
func NativeVideoMode() int32 {
	if *(*byte)(Pointer(0xbfc7ff52)) == 'E' {
		return MODE_PAL
	}
	return MODE_NTSC
}

// The CD-ROM calls the resident library has beyond what libps.h declares,
// which together read a file whose sector is already known, as package disc
// records it, without the search of the directory CdReadFile makes. CdRead
// is in symusr.o; the other four are at the addresses the library's own
// CdReadFile calls, which the TinyGo target's link script provides, since
// Sony exported CdRead but not the CdControl that positions the drive for
// it. Signatures are libcd's.

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
// The library goes on with buf after it returns, until CdReadSync, so a
// variable passed as buf has to outlive that: one of a function other than
// main needs CdReadSync before the function returns. yaroze vet and the
// build say where a program does otherwise.
//
//export CdRead
//go:keeps buf CdReadSync
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
