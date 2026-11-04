//go:build yaroze

// Command cdmore is a probe for the spec, after cdlimits: CdControl and
// CdControlB over the commands that take a position (SeekL, SeekP, Play and
// ReadS), a refused Setloc through CdControl, and Play with no parameter and
// with a track number, each with the kernel events the library delivers
// counted (two events of class F0000003h, 20h and 40h, opened with
// callbacks); then CdSearchFile over a listing whose first sector holds 59
// records, the most a sector can show (the disc is built with --no-xa, see
// mkfiles), to find where the library's file table ends; and a listing
// whose last record the table takes has a long name. Each result goes to
// the TTY; a file's position is printed as its distance from where the
// disc build placed it, and a directory record's from the listing's own
// "." record, so that builds whose executables differ in size compare.
//
// files.go, the registrations, and files/track.raw, the audio track, are
// written by mkfiles (go run ./exp/cleanlib/spec/probes/cdmore/mkfiles
// exp/cleanlib/spec/probes/cdmore), whose comment gives the disc's layout.
// Build the disc with the track and without the XA area:
//
//	go run ./cmd/yaroze disc --opt=2 --no-xa --audio exp/cleanlib/spec/probes/cdmore/files/track.raw \
//	    --elf .build/cdmore-sony/prog.elf -o .build/cdmore-sony/cdmore-sony.bin \
//	    exp/cleanlib/spec/probes/cdmore
//
// The commands each call sends, and the events next to the drive's
// responses, are read from an I/O trace (.build/iotrace, TestIO with
// CALLS=CdControl,CdControlB); the GP1 marker words (10000100h plus the
// case) separate the cases in it.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

// libps_biosB calls function fn of the kernel's B0 table.
//
//export libps_biosB
func libps_biosB(fn, a0, a1, a2, a3 uintptr) int32

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

// ev20 and ev40 count the kernel events of class F0000003h with specs 20h
// and 40h the library delivered, through the callbacks of two events the
// probe opens.
var ev20, ev40 uint32

func on20() { volatile.StoreUint32(&ev20, volatile.LoadUint32(&ev20)+1) }
func on40() { volatile.StoreUint32(&ev40, volatile.LoadUint32(&ev40)+1) }

// events opens the two events, with callbacks (mode 1000h), and enables
// them.
func events() {
	const openEvent, enableEvent = 0x08, 0x0c
	for _, e := range [...]struct {
		spec uintptr
		f    func()
	}{{0x20, on20}, {0x40, on40}} {
		ev := libps_biosB(openEvent, 0xf0000003, e.spec, 0x1000, addr(e.f))
		libps_biosB(enableEvent, uintptr(ev), 0, 0, 0)
	}
}

// evs prints the events counted since the last call.
func evs() {
	print(" ev20 ", volatile.LoadUint32(&ev20), " ev40 ", volatile.LoadUint32(&ev40))
	volatile.StoreUint32(&ev20, 0)
	volatile.StoreUint32(&ev40, 0)
}

// addr is the address of a top-level function's code.
func addr(f func()) uintptr {
	return (*[2]uintptr)(unsafe.Pointer(&f))[1]
}

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

// wait waits n frames.
func wait(n int) {
	for range n {
		yaroze.VSync(0)
	}
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

// printName prints the 16 name bytes of fp.
func printName(fp *yaroze.CdlFILE) {
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
}

// search prints a search for name, against the file want if it is one of
// the disc's.
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
	printName(&fp)
	println()
}

// records searches each record of the listing of dir, a path with its
// backslashes, by the names given with suffix (";1" for files) and prints each one's kind, the index
// of its entry in the table (its distance from the "." record's entry in
// entries of 24 bytes), its position against the "." record's and its size,
// and its name bytes.
func records(dir, suffix string, names []string, files []*disc.File) {
	r0, k0, fp0 := lookup(dir + "\\.")
	println("dot", k0, "size", fp0.Size)
	for i, n := range names {
		r, kind, fp := lookup(dir + "\\" + n + suffix)
		print(n, " ", kind)
		if kind == "entry" {
			print(" idx ", (uintptr(unsafe.Pointer(r))-uintptr(unsafe.Pointer(r0)))/24)
			if files != nil {
				print(" delta ", yaroze.CdPosToInt(&fp.Pos)-int32(files[i].Sector()))
			} else {
				print(" delta ", yaroze.CdPosToInt(&fp.Pos)-yaroze.CdPosToInt(&fp0.Pos))
			}
			print(" size ", fp.Size)
		}
		printName(&fp)
		println()
	}
}

// ctl runs com through CdControlB, from a vertical blank so that a short
// call does not straddle one, and prints its return, its 8 result bytes,
// the frames it took and the events delivered.
func ctl(tag string, com uint8, param ...uint8) {
	var p [4]uint8
	copy(p[:], param)
	var res [8]uint8
	for k := range res {
		res[k] = 0xcc
	}
	yaroze.VSync(0)
	t := volatile.LoadUint32(&ticks)
	r := yaroze.CdControlB(com, &p[0], &res[0])
	t = volatile.LoadUint32(&ticks) - t
	print("controlB ", tag, " ", hex2(com), " ret ", r, " res")
	for k := range res {
		print(" ", hex2(res[k]))
	}
	print(" frames ", t)
	evs()
	println()
}

// ctlA is ctl over CdControl, which does not wait, then 30 frames, with the
// events counted over the 30 frames too; control is the same with the
// parameter pointer given, nil among them.
func ctlA(tag string, com uint8, param ...uint8) {
	var p [4]uint8
	copy(p[:], param)
	control(tag, com, &p[0])
}

func control(tag string, com uint8, p *uint8) {
	var res [8]uint8
	for k := range res {
		res[k] = 0xcc
	}
	yaroze.VSync(0)
	r := yaroze.CdControl(com, p, &res[0])
	print("control ", tag, " ", hex2(com), " ret ", r, " res")
	for k := range res {
		print(" ", hex2(res[k]))
	}
	evs()
	wait(30)
	print(" after")
	evs()
	println()
}

// gettd returns GetTD's result for track t.
func gettd(t uint8) (td [8]uint8) {
	p := [4]uint8{t}
	r := yaroze.CdControlB(0x14, &p[0], &td[0])
	println("gettd", t, "ret", r, "res", hex2(td[0]), hex2(td[1]), hex2(td[2]))
	return
}

// marks is the next GP1 marker's number.
var marks uint32

// next writes the next GP1 marker word.
func next() {
	mark(marks)
	marks++
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.VSyncCallback(addr(tick))
	events()
	println("files", len(files), nFiles)

	// The commands, each between GP1 marker words for the trace. The audio
	// track's place comes from GetTD, which gives the start of its pregap;
	// the audio is two seconds on.
	println("case control")
	next()
	ctl("stat", 0x01)
	ctl("gettn", 0x13)
	td := gettd(2)
	ss := bcdAdd(td[2], 2)
	evs()
	println()
	// A refused Setloc, on its own and before a SeekL, through CdControl
	// and CdControlB.
	next()
	ctlA("setloc-bad", yaroze.CdlSetloc, 0xaa, 0, 0)
	ctl("stat", 0x01)
	next()
	ctlA("seekl-bad", 0x15, 0xaa, 0, 0)
	ctl("stat", 0x01)
	next()
	ctl("setloc-bad", yaroze.CdlSetloc, 0xaa, 0, 0)
	ctl("stat", 0x01)
	next()
	ctl("seekl-bad", 0x15, 0xaa, 0, 0)
	ctl("stat", 0x01)
	// A SeekL whose second response is an error, past the disc's end.
	next()
	ctl("seekl-far", 0x15, 0x70, 0, 0)
	ctl("stat", 0x01)
	ctl("getlocp", 0x11)
	next()
	ctlA("seekl-far", 0x15, 0x70, 0, 0)
	ctl("stat", 0x01)
	ctl("getlocp", 0x11)
	// Setloc on its own, and the Setloc before SeekL and SeekP, through
	// CdControlB and CdControl.
	next()
	ctl("setloc", yaroze.CdlSetloc, 0, 2, 0x16)
	next()
	ctl("seekl-ok", 0x15, 0, 2, 0x16)
	ctl("getlocp", 0x11)
	next()
	ctlA("seekl-ok", 0x15, 0, 2, 0x20)
	ctl("getlocp", 0x11)
	next()
	ctl("seekp-ok", 0x16, 0, 2, 0x16)
	ctl("getlocp", 0x11)
	next()
	ctlA("seekp-ok", 0x16, 0, 2, 0x20)
	ctl("getlocp", 0x11)
	// Play with a position, through CdControlB and CdControl, each paused
	// after 30 frames.
	next()
	ctl("play-pos", 0x03, td[1], ss, 0x10)
	wait(30)
	ctl("getlocp", 0x11)
	ctl("stat", 0x01)
	ctl("pause", 0x09)
	next()
	ctlA("play-pos", 0x03, td[1], ss, 0x10)
	ctl("getlocp", 0x11)
	ctl("pause", 0x09)
	// Play with a track number, through CdControlB and CdControl, and with
	// no parameter at all.
	next()
	ctl("play-track", 0x03, 0x02)
	wait(30)
	ctl("getlocp", 0x11)
	ctl("pause", 0x09)
	next()
	ctlA("play-track", 0x03, 0x02)
	ctl("getlocp", 0x11)
	ctl("pause", 0x09)
	next()
	control("play-nil", 0x03, nil)
	ctl("getlocp", 0x11)
	ctl("pause", 0x09)
	// ReadN and ReadS with a position, through CdControlB and CdControl.
	next()
	ctl("readn-pos", 0x06, 0, 2, 0x16)
	ctl("stat", 0x01)
	ctl("pause", 0x09)
	ctl("getlocp", 0x11)
	next()
	ctlA("readn-pos", 0x06, 0, 2, 0x20)
	ctl("stat", 0x01)
	ctl("pause", 0x09)
	ctl("getlocp", 0x11)
	next()
	ctl("reads-pos", 0x1b, 0, 2, 0x16)
	ctl("stat", 0x01)
	ctl("pause", 0x09)
	ctl("getlocp", 0x11)
	next()
	ctlA("reads-pos", 0x1b, 0, 2, 0x20)
	ctl("stat", 0x01)
	ctl("pause", 0x09)
	ctl("getlocp", 0x11)
	next()

	// R's listing: 59 records in its first sector, "." and ".." and the
	// first 57 directories; _K, _L and _M lie in the second. Each record is
	// searched by its name, and each directory's file through the path
	// table.
	println("case table")
	records("\\R", "", rNames[:], nil)
	for i := range rNames {
		_, kind, fp := lookup(dpath(rFirst + i))
		print(rNames[i], " file ", kind)
		if kind == "entry" {
			print(" delta ", yaroze.CdPosToInt(&fp.Pos)-int32(files[rFirst+i].Sector()), " size ", fp.Size)
		}
		println()
	}
	search("\\R\\..", nil)
	search("\\R\\_J", nil)
	search("\\R\\_K", nil)
	println("done")
	for {
		yaroze.VSync(0)
	}
}

// bcdAdd adds n to the BCD byte b.
func bcdAdd(b uint8, n uint8) uint8 {
	v := uint8(b>>4)*10 + b&15 + n
	return v/10<<4 | v%10
}
