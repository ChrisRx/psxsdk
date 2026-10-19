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
// read into, which Into gives it or Load allocates.
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

	// dst is the memory the file is read into, from Into, or from Load,
	// which allocates it when Into gave none, or nil. mkpsxiso reads its
	// address and length too, to check that a file given memory by Into
	// fits.
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

// TrackName is the disc path Track registers an audio track under, which
// mkpsxiso takes to mean a track of CD audio rather than a file.
const TrackName = "#CDDA"

// Track declares that the host file src, a WAV or raw 44.1 kHz 16-bit
// stereo samples, is to be recorded on the disc as a track of CD audio,
// after the data track and any tracks before it, and returns the File that
// says which track it became:
//
//	var music = disc.Track("audio/theme.wav")
//
//	tracks := [2]int32{music.TrackNumber(), 0}
//	yaroze.CdPlay(2, &tracks[0], 0)
//
// Like Register, the call must initialize a package-level variable with a
// constant. Tracks are numbered in the order the program declares them.
func Track(src string) File {
	return File{magic: magic, Name: TrackName, src: src}
}

// TrackNumber is the number of the audio track Track registered, 2 for the
// first after the data track, as CdPlay takes it, or 0 if it was not
// recorded. A track's Size is its length in sectors of 2352 bytes, 75 to a
// second.
func (f *File) TrackNumber() int32 { return int32(f.Sector()) }

// Into gives the file the memory it is read into, in place of what Load
// would allocate, and tells mkpsxiso where that is, so that a file too
// large for it fails the build rather than overrunning it. The drive delivers whole sectors of 2048
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

// Data is the memory the file is read into, as words: what Into gave it,
// or what Load allocated, or nil before either. It is whole sectors, so
// past the file's Size it holds whatever the drive delivered or zeros.
func (f *File) Data() []uint32 { return f.dst }

// Bytes is the memory the file is read into, as the library takes a sound
// bank, or nil.
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

// Load starts reading the file into its memory, and reports whether the
// read began; it is Read with that memory. A file Into gave none gets it
// here, the first time, from the heap: the whole sectors of the file's
// Size, which the program keeps for good, since nothing is freed. A file
// mkpsxiso did not place has no Size to allocate by, and does not start
// without Into. The read goes on in the background, and Wait waits for it:
//
//	if !model.Load() || !disc.Wait(300) {
//		// the disc is missing or unreadable
//	}
//
// The heap is the memory between the program's own and the stack, and a
// program whose files do not fit in it stops at the allocation with "out
// of memory".
func (f *File) Load() bool {
	if f.dst == nil {
		if !f.Placed() {
			return false
		}
		f.dst = make([]uint32, (f.Size()+2047)/2048*512)
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

// LoadAll reads each of files into its memory, as Load does, one after the
// other, waiting up to frames frames for each as Wait does, and returns the
// first that could not be read, or nil once they all are:
//
//	if f := disc.LoadAll(300, &model, &texture); f != nil {
//		// f.Name is missing or unreadable
//	}
func LoadAll(frames int, files ...*File) *File {
	for _, f := range files {
		if !f.Load() || !Wait(frames) {
			return f
		}
	}
	return nil
}

// A Loader reads files while the frames go on, a step a frame, so that a
// program can draw something, a bar or a message, while its data arrives,
// rather than stopping in LoadAll. Start gives it the files, and Step,
// once a frame, starts each read in turn and sees whether it has finished:
//
//	loader.Start(300, files...)
//	for ot := range screen.Frames() {
//		if !loader.Step() {
//			bar.W = uint16(200 * loader.Loaded() / len(files))
//			bar.Sort(ot, 0)
//			continue
//		}
//		if f := loader.Failed(); f != nil {
//			// f.Name is missing or unreadable
//		}
//		...
//	}
//
// Each file is read as Load reads it, into memory Into gave it or memory
// allocated as its read starts.
type Loader struct {
	files   []*File
	frames  int // the frames a read may take before it is given up on
	next    int // the file being read, or the one to read next
	waited  int // frames the read in progress has taken
	reading bool
	failed  *File
}

// Start sets the loader to read files, each given up on after frames
// frames, as Wait gives up. Nothing is read until Step.
func (l *Loader) Start(frames int, files ...*File) {
	*l = Loader{files: files, frames: frames}
}

// Step moves the reads on, once a frame: it starts the next file's read,
// or sees whether the one in progress has finished, and starts the next as
// soon as it has. It reports whether the loader is done, with every file
// read or one failed, which Failed says.
func (l *Loader) Step() bool {
	for l.failed == nil && l.next < len(l.files) {
		f := l.files[l.next]
		if !l.reading {
			if !f.Load() {
				l.failed = f
				break
			}
			l.reading, l.waited = true, 0
		}
		switch n := yaroze.CdReadSync(1, nil); {
		case n < 0:
			l.failed = f
		case n == 0:
			l.reading = false
			l.next++
			continue
		default:
			if l.waited++; l.waited > l.frames {
				l.failed = f
			}
		}
		break
	}
	return l.Done()
}

// Done reports whether every file has been read or one has failed.
func (l *Loader) Done() bool { return l.failed != nil || l.next >= len(l.files) }

// Loaded returns how many of the files have been read.
func (l *Loader) Loaded() int { return l.next }

// Failed returns the file that could not be read, or nil.
func (l *Loader) Failed() *File { return l.failed }
