//go:build yaroze

package libps

import "unsafe"

// Loading and running programs (spec 14): CdReadExec, built on the
// library's own CD reads, and Exec, Load and LoadTest, the kernel's own,
// passed straight through.

// The kernel functions the loading calls use.
const (
	biosLoadTest = 0x41 // A0: reads an executable's header
	biosLoad     = 0x42 // A0: loads an executable
	biosExec     = 0x43 // A0: runs one
)

// EXEC describes an executable, PSX-EXE, as the file's header carries it
// from offset 10h and as Load and CdReadExec fill it in. Exec runs one,
// keeping the caller's registers in the five fields at the end.
type EXEC struct {
	Pc0    uint32
	Gp0    uint32
	T_addr uint32
	T_size uint32
	D_addr uint32
	D_size uint32
	B_addr uint32
	B_size uint32
	S_addr uint32
	S_size uint32
	Sp     uint32
	Fp     uint32
	Gp     uint32
	Ret    uint32
	Base   uint32
}

// execHeader is where a PSX-EXE's header holds the EXEC's first eight
// fields, Pc0 through B_size.
const execHeader = 0x10

var loader struct {
	// exec is the EXEC CdReadExec fills in and returns, the same one on
	// every call.
	exec EXEC

	// head is where it reads the file's first sector, the header.
	head [sectorSize / 4]uint32
}

// CdReadExec reads the executable named file, an absolute path such as
// "\CHILD.EXE;1" (the version optional, case ignored), to the address in
// its header, for Exec to run, and returns the library's EXEC for it,
// filled in from the header: Pc0 through B_size, with no stack (S_addr
// and S_size 0 whatever the header asks) and the saved registers 0. The
// name is looked up through CdSearchFile's tables, so a second load reads
// no directory. The read is done when it returns. Whole sectors are read,
// as many as the file has, so one more sector than the text lands after
// it. The header's magic is not checked. It returns nil for a name the
// disc does not have, with nothing written.
//
//export CdReadExec
func CdReadExec(file *byte) *EXEC {
	l := &loader
	sector, size, ok := cdFiles.search(file)
	if !ok || !cd.readAt(sector, 1, &l.head[0]) {
		return nil
	}
	CdReadSync(0, nil)
	h := (*[8]uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(&l.head[0])) + execHeader))
	l.exec = EXEC{
		Pc0:    h[0],
		Gp0:    h[1],
		T_addr: h[2],
		T_size: h[3],
		D_addr: h[4],
		D_size: h[5],
		B_addr: h[6],
		B_size: h[7],
	}
	// The rest of the file, from the sector after the header, in whole
	// sectors, where a CdReadFile without a name carries on from.
	n := (size + sectorSize - 1) / sectorSize
	if !cd.readAt(sector+1, int32(n), (*uint32)(unsafe.Pointer(uintptr(l.exec.T_addr)))) {
		return nil
	}
	cd.next = sector + 1 + n
	CdReadSync(0, nil)
	return &l.exec
}

// Exec runs the executable e describes with argc and argv, through the
// kernel, which clears B_size bytes at B_addr, sets gp to Gp0 and the
// stack to S_addr + S_size if S_addr is not 0, and keeps the caller's
// registers in e, restoring them when the program returns. It returns
// the kernel's result, 1.
//
//export Exec
//go:noinline
func Exec(e *EXEC, argc int32, argv **byte) int32 {
	return libps_biosA(biosExec, uintptr(unsafe.Pointer(e)), uintptr(argc), uintptr(unsafe.Pointer(argv)), 0)
}

// Load reads the executable named name through the kernel, from one of
// its devices ("bu00:" and so on), to the address in its header and
// describes it in e: 1, or the kernel's 0 with e untouched. A name on a
// memory card starts the card driver first, as open does (spec 11).
//
//export Load
//go:noinline
func Load(name *byte, e *EXEC) int32 {
	cardFile(name)
	return libps_biosA(biosLoad, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(e)), 0, 0)
}

// LoadTest reads the header of the executable named name into e through
// the kernel, without loading it: 1, or the kernel's 0 with e untouched.
//
//export LoadTest
//go:noinline
func LoadTest(name *byte, e *EXEC) int32 {
	cardFile(name)
	return libps_biosA(biosLoadTest, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(e)), 0, 0)
}
