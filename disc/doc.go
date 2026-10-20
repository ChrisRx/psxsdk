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
// The program reads the file with Load, into memory Load allocates the
// first time, as many whole sectors as mkpsxiso recorded the file has, and
// Data is that memory:
//
//	var model = disc.Register("ASSETS/CUBE.TMD", "assets/CUBE.TMD")
//
//	if !model.Load() || !disc.Wait(300) { ... }
//	cube := yaroze.TMD(model.Data())
//
// Or a declaration says where the file goes, with Into, and mkpsxiso checks
// at build time that the file fits: memory the program declares itself,
// which it can see the size of, share between files read at different
// times, or keep out of the heap.
//
//	var modelData [2048]uint32
//	var model = disc.Register("ASSETS/CUBE.TMD", "assets/CUBE.TMD").Into(modelData[:])
//
// The runtime half of the package is Register, the File it returns, and the
// reads. It builds on a host too, where the yaroze package's functions are
// stubs, so that a program using it type-checks there. The host half, which
// finds and patches the records, is mkpsxiso's, in internal/discrecord.
package disc
