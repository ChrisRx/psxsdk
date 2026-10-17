//go:build !yaroze

package yaroze

// The stubs of what libps_yaroze.go binds by hand, for every build that is
// not for the console, where an editor and go vet see the package. Each
// panics through stub, as the generated ones in zz_generated_stub.go do.

// stub is what every function of the library panics with when called in a
// build that is not for the console: the library is only there in a program
// built with the playstation-yaroze TinyGo target.
func stub(name string) string {
	return "yaroze." + name + ": the Net Yaroze library is only present in a program built with tinygo -target=playstation-yaroze"
}

// FntPrint writes s to the current print stream, for FntFlush to draw.
//
// In C it is printf-style. Formatting happens in Go instead, so it takes a
// finished C string, which Buffer builds without allocating and CString
// makes from a literal; it must contain no percent directives, or the
// library will read arguments that were never passed. It returns the number
// of characters in the stream.
func FntPrint(s *byte) int32 { panic(stub("FntPrint")) }

// KanjiFntPrint writes the Shift JIS string s to the current Kanji print
// stream, for KanjiFntFlush to draw, with the same caution about percent
// directives as FntPrint. It returns the number of characters in the
// stream.
func KanjiFntPrint(s *byte) int32 { panic(stub("KanjiFntPrint")) }

// VSyncCallback installs f to run at the start of every vertical blank,
// or removes the callback with f 0. It takes an address rather than a Go
// func, because the caller is C and a Go func value is a pair. Interrupts
// are off while f runs, so it must be quick, and it must not allocate or
// block.
func VSyncCallback(f uintptr) { panic(stub("VSyncCallback")) }

// GsGetWorkBase returns the next free byte of the packet area handed to
// GsSetWorkBase, which is how much of it the frame has used so far.
func GsGetWorkBase() *PACKET { panic(stub("GsGetWorkBase")) }

// PushMatrix saves the GTE's current rotation matrix on a stack of twenty.
func PushMatrix() { panic(stub("PushMatrix")) }

// PopMatrix restores the GTE's rotation matrix from the stack PushMatrix
// saved it on.
func PopMatrix() { panic(stub("PopMatrix")) }

// Rand returns a pseudo-random number from 0 to 32767, from the C library
// the resident library carries. It takes a capital so that Go can see it.
func Rand() int32 { panic(stub("Rand")) }

// Srand seeds Rand; a seed of 1 is where it starts. It takes a capital so
// that Go can see it.
func Srand(seed uint32) { panic(stub("Srand")) }

// GsLinkObject4 ties the object objp to the nth model in the TMD whose
// object table is at tmd, after GsMapModelingData has turned the file's
// offsets into addresses, so that GsSortObject4 can draw it. The object
// table is the file's fourth word, after the ID, the flags and the count.
//
// The header types the table as an integer; this takes the pointer a
// program has, and inlines to the same call.
func GsLinkObject4(tmd *uint32, objp *GsDOBJ2, n int32) { panic(stub("GsLinkObject4")) }

// GsDRAWENV returns the drawing environment the library draws with, which a
// program changes and then registers with PutDrawEnv.
func GsDRAWENV() *DRAWENV { panic(stub("GsDRAWENV")) }

// GsWSMATRIX returns the world-screen matrix GsSetRefView2 and GsSetView2
// last worked out.
func GsWSMATRIX() *MATRIX { panic(stub("GsWSMATRIX")) }

// GsDISPENV returns the display environment GsSwapDispBuff puts up each
// frame, which a program changes to move the picture on the screen.
func GsDISPENV() *DISPENV { panic(stub("GsDISPENV")) }

// CdControl issues a command to the drive and returns without waiting for
// it to complete. param is the command's parameter, if it takes one, and
// result receives the drive's reply when it is not nil. It returns 0 if the
// command could not be issued.
func CdControl(com uint8, param *uint8, result *uint8) int32 { panic(stub("CdControl")) }

// CdControlB is CdControl waiting for the command to complete.
func CdControlB(com uint8, param *uint8, result *uint8) int32 { panic(stub("CdControlB")) }

// CdRead reads sectors into buf from the position the last CdlSetloc gave,
// in the given mode, and returns 0 if the read could not be started. The
// read goes on in the background, and CdReadSync waits for it.
func CdRead(sectors int32, buf *uint32, mode int32) int32 { panic(stub("CdRead")) }

// CdIntToPos converts a sector number to the position the drive takes, in
// binary coded minutes, seconds and sectors, writing it to p and returning
// p.
func CdIntToPos(i int32, p *CdlLOC) *CdlLOC { panic(stub("CdIntToPos")) }

// CdPosToInt converts a position back to its sector number.
func CdPosToInt(p *CdlLOC) int32 { panic(stub("CdPosToInt")) }
