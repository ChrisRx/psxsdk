//go:build yaroze

package disc

// The runtime half of the package. Nothing here allocates, and nothing runs
// unless the program asks a File where it is.

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// A File is a file on the disc, declared by Register.
//
// Its layout is the record mkpsxiso looks for in the executable, so the
// fields, their order and their types are part of the contract with the host
// half of the package. layout.go spells the record out, and the checks below
// fail the build if the two drift apart.
type File struct {
	magic uint32

	// Name is the path on the disc, as it was registered.
	Name string

	// src is the host file, read by mkpsxiso and never by the program. It is
	// kept in the record rather than dropped because the record is how
	// mkpsxiso learns of the file at all.
	src string

	// sector and size are zero when the program is compiled and are written
	// by mkpsxiso once the image is laid out. They are only ever read as
	// volatile, below: the compiler knows they were zero when it saw them,
	// and a plain load would be folded to the zero it remembers, which would
	// also let it drop the record from the executable altogether.
	sector uint32
	size   uint32
}

var (
	_ = [1]struct{}{}[unsafe.Sizeof(File{})-recordSize]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.Name)-offName]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.src)-offSrc]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.sector)-offSector]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.size)-offSize]
)

// Register declares that the host file src is to be placed on the disc at
// path, and returns the File the program reads it through.
//
// path is written with forward slashes, from the root of the disc, as in
// "DATA/WALL0.TMD". mkpsxiso folds it to the uppercase ISO 9660 form and
// adds the ";1" version suffix, as it does for every file it places. src is
// relative to the directory mkpsxiso is told the program's files live in,
// which is normally the package directory.
//
// Both arguments must be constants, and the call must initialize a
// package-level variable, directly or as an element of a composite literal:
//
//	var stone = disc.Register("DATA/STONE.TIM", "assets/stone.tim")
//
// TinyGo evaluates such a call at compile time and the File becomes constant
// data in the executable, which is where mkpsxiso finds it. A call made at
// run time instead produces a File that mkpsxiso never sees, so the file is
// not on the disc and Placed reports false.
func Register(path, src string) File {
	return File{magic: magic, Name: path, src: src}
}

// Sector is the file's first sector on the disc, as a logical block address.
// It is zero until mkpsxiso has placed the file.
func (f *File) Sector() uint32 { return volatile.LoadUint32(&f.sector) }

// Size is the file's length in bytes. For a file of raw sectors, an .XA or
// .STR, it counts the 2336 bytes of each. It is zero until mkpsxiso has
// placed the file.
func (f *File) Size() uint32 { return volatile.LoadUint32(&f.size) }

// Path writes the file's name into buf in the form the library's
// CdSearchFile and CdReadFile take, "\DATA\WALL0.TMD;1" for a file
// registered as "DATA/WALL0.TMD", NUL terminated, and returns it for passing
// to them. buf has to stay allocated while the library may read it.
func (f *File) Path(buf []byte) *byte {
	cdPath(buf, f.Name)
	return &buf[0]
}

// Placed reports whether mkpsxiso put the file on the disc and wrote where.
// A file's first sector is never zero, since the first sixteen sectors are
// the system area, so a zero sector means it was never placed: the program
// was not built onto a disc by mkpsxiso, or Register was called at run time.
func (f *File) Placed() bool { return f.Sector() != 0 }

// pos is where the drive is sent for a placed file. It is not on the stack
// because its address is handed to the library, which would make the
// compiler allocate it, and a program loading its data should not have to
// have a heap for that.
var pos yaroze.CdlLOC

// Read starts reading the file into memory at addr, and reports whether the
// read began. A file mkpsxiso placed is read from the sector it recorded,
// with no search of the disc's directory. One it did not is read by name
// through CdReadFile, which searches for it, with the name written into
// buf, which has to stay allocated while the library may read it. Either
// way the read goes on in the background, and the caller waits for it with
// CdReadSync, as after CdReadFile.
//
// This is for a data file. A stream, an .XA or .STR, is sectors of another
// form that CdRead does not deliver this way, and is played through the
// drive's own streaming instead.
func (f *File) Read(addr *uint32, buf []byte) bool {
	if !f.Placed() {
		return yaroze.CdReadFile(f.Path(buf), addr, 0) != 0
	}
	yaroze.CdIntToPos(int32(f.Sector()), &pos)
	if yaroze.CdControl(yaroze.CdlSetloc, &pos.Minute, nil) == 0 {
		return false
	}
	return yaroze.CdRead(int32((f.Size()+2047)/2048), addr, yaroze.CdlModeSpeed) != 0
}
