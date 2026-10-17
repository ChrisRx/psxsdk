package disc

// The runtime half of the package. Nothing here allocates, and nothing runs
// unless the program asks a File where it is. It builds on a host too, where
// the yaroze package's functions are stubs, so that a program using it
// type-checks there; disc_yaroze.go holds what only means anything on the
// console.

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

// A File is a file on the disc, declared by Register, and the memory it is
// read into, if Into gave it some.
//
// Its layout is the record mkpsxiso looks for in the executable, so the
// fields, their order and their types are part of the contract with the host
// half of the package. layout.go spells the record out, and disc_yaroze.go
// fails the build if the two drift apart.
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

	// dst is the memory the file is read into, from Into, or nil. mkpsxiso
	// reads its address and length too, to check that the file fits.
	dst []uint32
}

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

// Into gives the file the memory it is read into, for Load, and tells
// mkpsxiso where that is, so that a file too large for it fails the build
// rather than overrunning it. The drive delivers whole sectors of 2048
// bytes, so the buffer must hold the file's length rounded up to one. It is
// declared as words because the read needs word alignment, and it must be
// a package-level array, since the registration is constant data:
//
//	var modelData [2048]uint32
//	var model = disc.Register("DATA/CUBE.TMD", "data/CUBE.TMD").Into(modelData[:])
func (f File) Into(buf []uint32) File {
	f.dst = buf
	return f
}

// Data is the memory Into gave the file, as words, or nil.
func (f *File) Data() []uint32 { return f.dst }

// Bytes is the memory Into gave the file, as the library takes a sound bank,
// or nil.
func (f *File) Bytes() *uint8 {
	if f.dst == nil {
		return nil
	}
	return (*uint8)(unsafe.Pointer(&f.dst[0]))
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

// pos is where the drive is sent for a placed file, and pathBuf is where
// Load writes the name of one that was not. They are not on the stack
// because their addresses are handed to the library, which would make the
// compiler allocate them, and a program loading its data should not have to
// have a heap for that.
var (
	pos     yaroze.CdlLOC
	pathBuf [64]byte
)

// Read starts reading the file into memory at addr, and reports whether the
// read began. A file mkpsxiso placed is read from the sector it recorded,
// with no search of the disc's directory. One it did not is read by name
// through CdReadFile, which searches for it, with the name written into
// buf, which has to stay allocated while the library may read it. Either
// way the read goes on in the background, and the caller waits for it with
// Wait, or with CdReadSync as after CdReadFile.
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

// Load starts reading the file into the memory Into gave it, and reports
// whether the read began; it is Read with that memory. A file without any
// does not start. The read goes on in the background, and Wait waits for
// it:
//
//	if !model.Load() || !disc.Wait(300) {
//		// the disc is missing or unreadable
//	}
func (f *File) Load() bool {
	if f.dst == nil {
		return false
	}
	return f.Read(&f.dst[0], pathBuf[:])
}

// Wait waits for the read in progress to finish, polling once a frame for
// up to frames frames, and reports whether it did. Giving up turns a
// missing or unreadable disc into something a program can show, rather
// than a hang; a few seconds, 300 frames say, is longer than any read of a
// file that is there takes. The drive reads one file at a time, so the
// read waited for is the last one started.
func Wait(frames int) bool {
	for range frames {
		switch n := yaroze.CdReadSync(1, nil); {
		case n == 0:
			return true
		case n < 0:
			return false
		}
		yaroze.VSync(0)
	}
	return false
}
