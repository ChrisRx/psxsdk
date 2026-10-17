package yaroze

// A TMD is a model file in memory, as the drive delivers it: the words of
// the file, which after Map hold the addresses of each object's vertices,
// normals and primitives. A program reads one with package disc and links
// objects to its models:
//
//	model := yaroze.TMD(modelFile.Data())
//	model.Map()
//	model.Link(&cube, 0)
//
// The file is an ID word, a flag word, a count of objects, and the object
// table, which is what the library wants pointers to.
type TMD []uint32

// Map is GsMapModelingData: it turns the file's offsets into addresses, in
// place, which has to happen once before anything is linked to it. The
// file records that it has been done, so doing it again is harmless.
//
//go:inline
func (t TMD) Map() { GsMapModelingData(&t[1]) }

// Objects returns how many models the file holds.
//
//go:inline
func (t TMD) Objects() int { return int(t[2]) }

// Link is GsLinkObject4: it ties obj to the nth model in the file, after
// Map, so that GsSortObject4 can draw it.
//
//go:inline
func (t TMD) Link(obj *GsDOBJ2, n int32) { GsLinkObject4(&t[3], obj, n) }

// Table returns the object table, the fourth word of the file, for code
// that reads the models' layout itself.
//
//go:inline
func (t TMD) Table() *uint32 { return &t[3] }
