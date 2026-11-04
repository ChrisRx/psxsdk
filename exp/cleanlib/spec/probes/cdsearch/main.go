//go:build yaroze

// Command cdsearch is a probe for the spec: CdPosToInt and CdIntToPos over
// positions and sector numbers, including ones that are not valid BCD;
// CdSearchFile over files in the root, nested directories, a directory
// of more than one sector, and names that are not there or are written
// otherwise; and CdControlB over commands with one and two responses,
// against CdControl. Each result goes to the TTY. A file's position is
// printed as its distance from where mkdisc placed it, so that builds
// whose executables differ in size compare.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

func hex2(b uint8) string {
	const h = "0123456789abcdef"
	return string([]byte{h[b>>4], h[b&15]})
}

// same reports whether a and b are the same address, out of the
// compiler's sight: it may take an argument it did not see escape to be
// different from anything a call returns.
//
//go:noinline
func same(a, b unsafe.Pointer) bool { return a == b }

func hexw(v uintptr) string {
	return hex2(uint8(v>>24)) + hex2(uint8(v>>16)) + hex2(uint8(v>>8)) + hex2(uint8(v))
}

func showLoc(l *yaroze.CdlLOC) string {
	return hex2(l.Minute) + ":" + hex2(l.Second) + ":" + hex2(l.Sector) + "." + hex2(l.Track)
}

// cstr makes a NUL-terminated copy of s.
func cstr(buf []byte, s string) *byte {
	n := copy(buf, s)
	buf[n] = 0
	return &buf[0]
}

func search(name string, want int) {
	var f *disc.File
	if want >= 0 {
		f = files[want]
	}
	placed = want == -2
	searchFile(name, f)
	placed = false
}

// placed is set for a file mkdisc placed that the probe has no File for.
var placed bool

func searchFile(name string, want *disc.File) {
	var buf [64]byte
	var fp yaroze.CdlFILE
	for i := range fp.Name {
		fp.Name[i] = '#'
	}
	fp.Size = 0xdeadbeef
	fp.Pos = yaroze.CdlLOC{Minute: 0xee, Second: 0xee, Sector: 0xee, Track: 0xee}
	r := yaroze.CdSearchFile(&fp, cstr(buf[:], name))
	print("search ", name, ": ")
	switch {
	case same(unsafe.Pointer(r), unsafe.Pointer(&fp)):
		print("fp")
	case r == nil:
		print("nil")
	case uintptr(unsafe.Pointer(r)) == ^uintptr(0):
		print("-1")
	case *r == fp:
		// A table entry of the library's holding what fp got.
		print("entry")
	default:
		print("other")
	}
	// Where mkdisc puts a file, and the executable's size, depend on the
	// executable: a file is shown against where it went, and those two
	// only by name.
	switch {
	case want != nil && fp.Size == 0xdeadbeef:
		print(" untouched")
	case want != nil:
		print(" delta ", yaroze.CdPosToInt(&fp.Pos)-int32(want.Sector()), " size ", fp.Size, " want ", want.Size())
	case !placed:
		print(" pos ", showLoc(&fp.Pos), " size ", fp.Size)
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

func main() {
	yaroze.ResetGraph(0)

	// CdPosToInt.
	locs := []yaroze.CdlLOC{
		{0, 0, 0, 0}, {0, 2, 0, 0}, {0, 2, 1, 0}, {0, 2, 0x74, 0}, {0, 3, 0, 0},
		{1, 0, 0, 0}, {0x10, 0x20, 0x30, 0}, {0x74, 0x59, 0x74, 0}, {0x99, 0x99, 0x99, 0x99},
		{0, 0x0a, 0x0b, 0}, {0xff, 0xff, 0xff, 0xff}, {0, 2, 0x75, 0}, {0, 0x60, 0, 0}, {0x1a, 0, 0, 0},
		{0, 2, 0, 0x55},
	}
	for i := range locs {
		println("postoint", showLoc(&locs[i]), yaroze.CdPosToInt(&locs[i]))
	}
	// CdIntToPos and back.
	ints := []int32{0, 1, 74, 75, 149, 150, 4499, 4500, 22, 16, 333000, 449999, -1, -150, -151, 1 << 20, -(1 << 20), 0x7fffffff}
	for _, i := range ints {
		var l yaroze.CdlLOC
		l.Track = 0x77
		r := yaroze.CdIntToPos(i, &l)
		println("inttopos", i, showLoc(&l), same(unsafe.Pointer(r), unsafe.Pointer(&l)), yaroze.CdPosToInt(&l))
	}

	// CdSearchFile.
	for i := range paths {
		search("\\"+backslash(paths[i])+";1", i)
	}
	search("\\DATA\\ONE.BIN;1", 1)
	search("\\data\\one.bin;1", 1)
	search("\\DATA\\ONE.BIN", 1)
	search("DATA\\ONE.BIN;1", 1)
	search("\\DATA\\ONE.BIN;2", 1)
	search("\\DATA\\NONE.BIN;1", -1)
	search("\\NODIR\\ONE.BIN;1", -1)
	search("\\DATA\\SUB;1", -1)
	search("\\DATA\\SUB\\TWO.BIN;1", 2)
	search("\\MAIN.EXE;1", -2)
	search("\\SYSTEM.CNF;1", -2)
	search("\\ROOT.BIN;1", 0)
	search("\\MANY\\F59.BIN;1", 64)
	search("\\MANY\\F00.BIN;1", 5)
	search("\\", -1)
	search("", -1)
	search("\\DATA\\\\ONE.BIN;1", 1)
	search("\\DATA\\SUB\\DEEP\\LONGNAME.DAT;1", 4)
	search("\\ROOT.BIN;1", 0)
	// Names in entries a directory read before left in the table.
	search("\\DATA\\SUB\\DEEP\\LONGNAME.DAT;1", 4)
	search("\\DATA\\SUB\\DEEP\\ONE.BIN;1", -1)
	search("\\DATA\\SUB\\DEEP\\SYSTEM.CNF;1", -1)
	search("\\DATA\\SUB\\DEEP\\ROOT.BIN;1", -1)
	search("\\DATA\\SUB\\DEEP\\MANY;1", -1)
	search("\\DATA\\SUB\\DEEP\\LONGNAME.DAT;1", 4)
	search("\\DATA\\SUB\\DEEP\\.", -1)
	search("\\DATA\\SUB\\DEEP\\\x01", -1)
	search("\\DATA\\SUB\\DEEP\\", -1)
	search("\\MANY\\ZZZ\\LATE.BIN;1", 65)
	search("\\DATA\\EMPTY.BIN;1", 3)
	search("\\DATA\\SUB", -1)
	search("\\DATA\\SUB\\DEEP\\..", -1)
	search("\\\\ROOT.BIN;1", 0)
	search("\\DATA\\SUB\\..\\ONE.BIN;1", -1)
	search("\\data\\ONE.BIN;1", -1)
	search("\\DATA\\SUB\\DEEP\\LONGNAME.DAT;1\\X", -1)
	search("\\ROOT.BIN;1\\", -1)
	search("\\MANY\\F33.BIN;1", 38)
	search("\\MANY\\ZZZ", -1)
	// A directory of more directories than a sector holds.
	for i := range bigs {
		searchFile("\\BIG\\D"+string(rune('0'+i/10))+string(rune('0'+i%10))+"\\X.BIN;1", bigs[i])
	}

	// CdControlB, and CdControl, over commands.
	type cmd struct {
		com   uint8
		param [4]uint8
	}
	cmds := []cmd{
		{0x00 + 0x01, [4]uint8{}},
		{0x13, [4]uint8{}},     // GetTN
		{0x14, [4]uint8{2}},    // GetTD past the only track; track 0, the end, depends on the disc's size
		{0x14, [4]uint8{1}},    // GetTD
		{0x14, [4]uint8{0x99}}, // GetTD past the last track
		{0x0e, [4]uint8{0x80}},
		{0x0f, [4]uint8{}}, // Getparam
		{yaroze.CdlSetloc, [4]uint8{0, 2, 0x16}},
		{0x15, [4]uint8{}}, // SeekL
		{0x09, [4]uint8{}},
		{0x09, [4]uint8{}},
		{0x0b, [4]uint8{}},     // Mute
		{0x0c, [4]uint8{}},     // Demute
		{0x1f, [4]uint8{}},     // not a command
		{0x00, [4]uint8{}},     // not a command
		{0x19, [4]uint8{0x20}}, // Test: the drive's date
		{0x00 + 0x01, [4]uint8{}},
	}
	for round := range 2 {
		for i := range cmds {
			c := &cmds[i]
			var res [8]uint8
			for k := range res {
				res[k] = 0xcc
			}
			var r int32
			p := &c.param[0]
			if round == 0 {
				r = yaroze.CdControlB(c.com, p, &res[0])
			} else {
				r = yaroze.CdControl(c.com, p, &res[0])
				for range 30 {
					yaroze.VSync(0)
				}
			}
			print("control", round, " ", hex2(c.com), " ret ", r, " res")
			for k := range res {
				print(" ", hex2(res[k]))
			}
			println()
		}
	}
	// CdControlB with a nil result.
	println("nil result", yaroze.CdControlB(0x00+0x01, nil, nil))
	println("done")
	for {
		yaroze.VSync(0)
	}
}

func backslash(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] == '/' {
			b[i] = '\\'
		}
	}
	return string(b)
}
