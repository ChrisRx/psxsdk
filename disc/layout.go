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
// The runtime half of the package, which builds under the yaroze tag, is
// Register and the File it returns, whose Read is that direct read. The host
// half, which builds under the ordinary Go toolchain, is what mkpsxiso uses
// to find and patch the records.
package disc

// magic begins every record. Read as a little-endian word it is the bytes
// "PSXD", which is how it looks in a hex dump of the executable.
const magic = 0x44585350

// The record is a File as TinyGo lays it out on a 32-bit target: the magic
// word, the disc path and the host path each as a pointer and a length, then
// the sector and the length that mkpsxiso fills in. disc.go asserts that the
// type matches.
const (
	recordSize = 28
	offName    = 4
	offSrc     = 12
	offSector  = 20
	offSize    = 24
)
