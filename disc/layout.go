// Package disc declares the files a program reads off the disc, in a form
// mkpsxiso can see.
//
// A program calls Register in a package-level declaration, with two
// constants: the path the file is to have on the disc, and the file on the
// host that goes there. TinyGo evaluates the call when it compiles the
// program, so every registration becomes a record of constant data in the
// executable. mkpsxiso finds those records in the executable it is given to
// boot, adds each file to the image at its path, and once the image is laid
// out writes the file's first sector and its length back into the record. The
// program then reads the file from the sector it was told, without searching
// the disc's directory for it, and the one declaration is what both the
// program and the image are built from.
//
// A declaration can also say where the file goes in memory, with Into, and
// then mkpsxiso checks at build time that the file fits, and the program
// reads it with Load and no address at all:
//
//	var modelData [2048]uint32
//	var model = disc.Register("DATA/CUBE.TMD", "data/CUBE.TMD").Into(modelData[:])
//
// The runtime half of the package is Register, the File it returns, and the
// reads. It builds on a host too, where the yaroze package's functions are
// stubs, so that a program using it type-checks there. The host half, which
// builds under the ordinary Go toolchain, is what mkpsxiso uses to find and
// patch the records.
package disc

// magic begins every record. Read as a little-endian word it is the bytes
// "PSXD", which is how it looks in a hex dump of the executable.
const magic = 0x44585350

// The record is a File as TinyGo lays it out on a 32-bit target: the magic
// word, the disc path and the host path each as a pointer and a length, the
// sector and the length that mkpsxiso fills in, and the destination as a
// slice, a pointer, a length and a capacity, all zero when there is none.
// disc_yaroze.go asserts that the type matches.
const (
	recordSize = 40
	offName    = 4
	offSrc     = 12
	offSector  = 20
	offSize    = 24
	offDst     = 28
	offDstLen  = 32
)
