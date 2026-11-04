//go:build yaroze

// Command cdlimits is a probe for the spec: CdSearchFile over a disc laid
// out to find the limits of its tables (300 directories under one, so that
// the path table takes two sectors; listings whose first sector holds 40
// and 42 records; directory and file names of 15 to 40 bytes; and how much
// of a name the match compares), and CdControlB over two-response commands
// whose second response is an error (a seek or a read past the disc's end,
// and a seek into the audio track the disc build records). Each result goes
// to the TTY; a file's position is printed as its distance from where the
// disc build placed it, so that builds whose executables differ in size
// compare. The frames a CdControlB takes are counted with VSyncCallback.
// The long names that may crash the library come last.
//
// files.go, the registrations, and files/track.raw, the audio track, are
// written by mkfiles (go run ./exp/cleanlib/spec/probes/cdlimits/mkfiles
// exp/cleanlib/spec/probes/cdlimits), whose comment gives the disc's
// layout. Build the disc with the track:
//
//	go run ./cmd/yaroze disc --opt=2 --audio exp/cleanlib/spec/probes/cdlimits/files/track.raw \
//	    --elf .build/cdlimits-sony/prog.elf -o .build/cdlimits-sony/cdlimits-sony.bin \
//	    exp/cleanlib/spec/probes/cdlimits
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

func hex2(b uint8) string {
	const h = "0123456789abcdef"
	return string([]byte{h[b>>4], h[b&15]})
}

// same reports whether a and b are the same address, out of the
// compiler's sight.
//
//go:noinline
func same(a, b unsafe.Pointer) bool { return a == b }

// cstr makes a NUL-terminated copy of s.
func cstr(buf []byte, s string) *byte {
	n := copy(buf, s)
	buf[n] = 0
	return &buf[0]
}

var ticks uint32

// tick is the vertical blank callback.
func tick() {
	volatile.StoreUint32(&ticks, volatile.LoadUint32(&ticks)+1)
}

// addr is the address of a top-level function's code.
func addr(f func()) uintptr {
	return (*[2]uintptr)(unsafe.Pointer(&f))[1]
}

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

// dpath turns registration i's path into the disc path of its file.
func dpath(i int) string {
	b := []byte(paths[i])
	for k := range b {
		if b[k] == '/' {
			b[k] = '\\'
		}
	}
	return "\\" + string(b) + ";1"
}

// ident is registration i's identifier, the file's name with its version.
func ident(i int) string {
	p := paths[i]
	for k := len(p) - 1; k >= 0; k-- {
		if p[k] == '/' {
			return p[k+1:] + ";1"
		}
	}
	return p + ";1"
}

// dir is registration i's directory, as a disc path with its backslashes.
func dir(i int) string {
	p := dpath(i)
	for k := len(p) - 1; k >= 0; k-- {
		if p[k] == '\\' {
			return p[:k+1]
		}
	}
	return p
}

// lookup searches name, with fp filled with marker bytes, and returns what
// CdSearchFile returned, its kind, and fp.
func lookup(name string) (r *yaroze.CdlFILE, kind string, fp yaroze.CdlFILE) {
	var buf [96]byte
	for i := range fp.Name {
		fp.Name[i] = '#'
	}
	fp.Size = 0xdeadbeef
	fp.Pos = yaroze.CdlLOC{Minute: 0xee, Second: 0xee, Sector: 0xee, Track: 0xee}
	r = yaroze.CdSearchFile(&fp, cstr(buf[:], name))
	switch {
	case same(unsafe.Pointer(r), unsafe.Pointer(&fp)):
		kind = "fp"
	case r == nil:
		kind = "nil"
	case uintptr(unsafe.Pointer(r)) == ^uintptr(0):
		kind = "-1"
	case *r == fp:
		kind = "entry"
	default:
		kind = "other"
	}
	return
}

// search prints a search for name, against the file want if it is one of
// the disc's (a directory's position differs between builds and is not
// printed).
func search(name string, want *disc.File) {
	_, kind, fp := lookup(name)
	print("search ", name, ": ", kind)
	switch {
	case fp.Size == 0xdeadbeef && fp.Pos.Minute == 0xee:
		print(" untouched")
	case want != nil:
		print(" delta ", yaroze.CdPosToInt(&fp.Pos)-int32(want.Sector()), " size ", fp.Size, " want ", want.Size())
	default:
		print(" size ", fp.Size)
	}
	print(" name ")
	for _, c := range fp.Name {
		switch {
		case c == 0:
			print("_")
		case c < 32 || c > 126:
			print("?")
		default:
			print(string(rune(c)))
		}
	}
	println()
}

// found reports whether the file i was found as an entry with the right
// position and size: e for yes, n for nil with fp untouched, x otherwise.
func found(i int) string {
	f := files[i]
	_, kind, fp := lookup(dpath(i))
	switch {
	case kind == "entry" && yaroze.CdPosToInt(&fp.Pos) == int32(f.Sector()) && fp.Size == f.Size():
		return "e"
	case kind == "nil" && fp.Size == 0xdeadbeef:
		return "n"
	}
	return "x"
}

// runs prints a character per file from first, n of them, 50 to a line.
func runs(what string, first, n int) {
	line := ""
	for i := range n {
		line += found(first + i)
		if i%50 == 49 || i == n-1 {
			println(what, i-len(line)+1, "to", i, line)
			line = ""
		}
	}
}

// prefixes searches file i by the first k bytes of its identifier for
// each k given, and by those with a wrong last byte.
func prefixes(i int, ks ...int) {
	id := ident(i)
	for _, k := range ks {
		if k < len(id) {
			search(dir(i)+id[:k], files[i])
		}
		if k <= len(id) {
			search(dir(i)+id[:k-1]+"x", files[i])
		}
	}
}

// ctl runs com through CdControlB and prints its return, its 8 result
// bytes and the frames it took.
func ctl(tag string, com uint8, param ...uint8) {
	var p [4]uint8
	copy(p[:], param)
	var res [8]uint8
	for k := range res {
		res[k] = 0xcc
	}
	t := volatile.LoadUint32(&ticks)
	r := yaroze.CdControlB(com, &p[0], &res[0])
	t = volatile.LoadUint32(&ticks) - t
	print("controlB ", tag, " ", hex2(com), " ret ", r, " res")
	for k := range res {
		print(" ", hex2(res[k]))
	}
	println(" frames", t)
}

// ctlA is ctl over CdControl, which does not wait, then 60 frames.
func ctlA(tag string, com uint8, param ...uint8) {
	var p [4]uint8
	copy(p[:], param)
	var res [8]uint8
	for k := range res {
		res[k] = 0xcc
	}
	r := yaroze.CdControl(com, &p[0], &res[0])
	print("control ", tag, " ", hex2(com), " ret ", r, " res")
	for k := range res {
		print(" ", hex2(res[k]))
	}
	println()
	for range 60 {
		yaroze.VSync(0)
	}
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.VSyncCallback(addr(tick))
	println("files", len(files), nFiles)

	// Directory names of 16 and 24 bytes, and the directory after them in
	// the path table.
	println("case names-dir")
	search(dpath(dirA16), files[dirA16])
	search(dpath(dirA24), files[dirA24])
	search(dpath(dirB), files[dirB])
	search("\\ABCDEFGHIJKLMNO\\X.BIN;1", nil)
	search("\\ABCDEFGHIJKLMNOPQRSTUVW\\X.BIN;1", nil)
	search("\\ABCDEFGHIJKLMNOPQRSTUVWXY\\X.BIN;1", nil)
	search("\\abcdefghijklmnop\\X.BIN;1", nil)

	// The 300 directories under Q: entry 128 of the path table is Q/068,
	// and its first sector ends inside Q's children.
	println("case dirs")
	runs("dirs", qFirst, 300)
	search(dpath(qFirst), files[qFirst])
	search(dpath(qFirst+68), files[qFirst+68])
	search(dpath(qFirst+69), files[qFirst+69])
	search(dpath(qFirst+299), files[qFirst+299])
	search(dpath(dirB), files[dirB])

	// Q's own records: its listing's first sector holds "." and ".." and
	// 000 to 038.
	println("case dir-records")
	search("\\Q\\000", nil)
	search("\\Q\\038", nil)
	search("\\Q\\039", nil)
	search("\\Q\\299", nil)
	search("\\Q", nil)

	// F's 45 files: 40 records in the first sector of its listing.
	println("case files")
	for i := range 45 {
		search(dpath(fFirst+i), files[fFirst+i])
	}
	search(dpath(fFirst), files[fFirst])

	// G's 41 directories: 42 records in the first sector of its listing,
	// as many as a sector holds. Their files are found through the path
	// table; the records are G's listing's.
	println("case records")
	runs("g", gFirst, 41)
	search("\\G\\_C", nil)
	search("\\G\\_D", nil)
	search("\\G\\0", nil)
	search(dpath(gFirst+40), files[gFirst+40])

	// The table's stride: the entries of two records in a row.
	println("case stride")
	r0, k0, _ := lookup(dpath(lFirst))
	r1, k1, _ := lookup(dpath(lFirst + 1))
	if k0 == "entry" && k1 == "entry" {
		println("stride", int32(uintptr(unsafe.Pointer(r1))-uintptr(unsafe.Pointer(r0))))
	} else {
		println("stride", k0, k1)
	}

	// L's names of 7 to 26 bytes, each long one in lower case, without its
	// version, and by its prefixes.
	println("case names")
	for i := range 11 {
		f := files[lFirst+i]
		search(dpath(lFirst+i), f)
		if i%2 == 1 {
			id := ident(lFirst + i)
			search("\\L\\"+lower(id), f)
			search("\\L\\"+id[:len(id)-2], f)
			prefixes(lFirst+i, 15, 16, 17)
		}
	}
	search("\\L\\A.BIN;1XXXXXXXXXXXXXXXX", files[lFirst])
	for i := range 11 {
		search(dpath(lFirst+i), files[lFirst+i])
	}

	// How many bytes the match compares: prefixes of a 20-byte identifier
	// and of 7- and 15-byte ones, each with a wrong byte at the end.
	println("case compare")
	prefixes(lFirst+7, 6, 7, 8, 9, 10, 11, 12, 13, 14)
	prefixes(lFirst, 3, 4, 5, 6, 7)
	prefixes(lFirst+1, 10, 11, 12, 13, 14, 15)
	prefixes(lFirst+3, 11, 12, 13, 14, 15, 16)
	search("\\L\\K.BIN;1", files[lFirst+10])

	// CdControlB over two-response commands. SeekL, SeekP and ReadN take
	// the position as their parameter (the library sends it as a Setloc
	// first); 70:00:00 is past the disc's end, and the audio track's place
	// comes from GetTD.
	println("case control")
	mark(0)
	ctl("stat", 0x01)
	mark(1)
	ctl("seekl-ok", 0x15, 0, 2, 0x16)
	ctl("stat", 0x01)
	ctl("getlocp", 0x11)
	mark(2)
	ctl("seekl-far", 0x15, 0x70, 0, 0)
	ctl("stat", 0x01)
	ctl("stat", 0x01)
	ctl("getlocp", 0x11)
	mark(3)
	ctl("seekp-far", 0x16, 0x70, 0, 0)
	ctl("stat", 0x01)
	ctl("getlocp", 0x11)
	mark(4)
	ctl("readn-far", 0x06, 0x70, 0, 0)
	ctl("stat", 0x01)
	ctl("pause", 0x09)
	ctl("stat", 0x01)
	ctl("getlocp", 0x11)
	mark(5)
	ctl("readn-ok", 0x06, 0, 2, 0x16)
	ctl("pause", 0x09)
	ctl("stat", 0x01)
	ctl("getlocp", 0x11)
	mark(6)
	ctl("gettn", 0x13)
	var td [8]uint8
	p := [4]uint8{2}
	r := yaroze.CdControlB(0x14, &p[0], &td[0])
	println("gettd 2 ret", r, "res", hex2(td[0]), hex2(td[1]), hex2(td[2]))
	// GetTD gives the start of the track's pregap; the audio is two
	// seconds on.
	ss := bcdAdd(td[2], 2)
	ctl("seekl-pregap", 0x15, td[1], td[2], 0x10)
	ctl("stat", 0x01)
	ctl("getlocp", 0x11)
	ctl("seekl-audio", 0x15, td[1], ss, 0x10)
	ctl("stat", 0x01)
	ctl("getlocp", 0x11)
	ctl("seekp-audio", 0x16, td[1], ss, 0x10)
	ctl("stat", 0x01)
	ctl("getlocp", 0x11)
	mark(7)
	ctlA("seekl-far", 0x15, 0x70, 0, 0)
	ctl("stat", 0x01)
	ctl("getlocp", 0x11)
	mark(8)
	ctl("setloc-bad", yaroze.CdlSetloc, 0xaa, 0, 0)
	ctl("seekl-bad", 0x15, 0xaa, 0, 0)
	ctl("stat", 0x01)
	ctl("getlocp", 0x11)
	mark(9)
	// A Setloc of its own before a SeekL with another position.
	ctl("setloc-far", yaroze.CdlSetloc, 0x70, 0, 0)
	ctl("seekl-ok", 0x15, 0, 2, 0x16)
	ctl("stat", 0x01)
	ctl("getlocp", 0x11)
	mark(10)

	// Names of 28 to 40 bytes as the last record of a listing, from the
	// shortest; N's records after P32 show what the copy of its name left
	// past its entry; a directory part of 33 bytes that is not there; then
	// a 40-byte name with a record after it.
	println("case long")
	for k := range pLengths {
		a := pFirst + 2*k
		search(dpath(a), files[a])
		search(dpath(a+1), files[a+1])
		id := ident(a + 1)
		search(dir(a+1)+id[:16], files[a+1])
		search(dpath(a), files[a])
		if pLengths[k] == 32 {
			search(dpath(nFirst+2), files[nFirst+2])
			search(dpath(nFirst+1), files[nFirst+1])
			search(dpath(nFirst), files[nFirst])
			search("\\Z23456789012345678901234567890123\\X.BIN;1", nil)
			search(dpath(dirB), files[dirB])
		}
	}
	search(dpath(oFirst), files[oFirst])
	search(dpath(oFirst+1), files[oFirst+1])
	search(dpath(oFirst+2), files[oFirst+2])
	println("done")
	for {
		yaroze.VSync(0)
	}
}

// pLengths are the P directories' long identifiers' lengths, as mkfiles
// has them.
var pLengths = [...]int{28, 30, 31, 32, 33, 34, 36, 40}

// bcdAdd adds n to the BCD byte b.
func bcdAdd(b uint8, n uint8) uint8 {
	v := uint8(b>>4)*10 + b&15 + n
	return v/10<<4 | v%10
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if 'A' <= b[i] && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
