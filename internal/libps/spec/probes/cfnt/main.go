//go:build yaroze

// Command cfnt is a probe for spec 17: FntPrint as C calls it, with printf arguments
// and an optional stream id first. A case prints what the call returned, and what a
// flush of each open stream draws, decoded from its sprites: the text with a space
// for each gap and / for each new row. The id group prints only whether "OK" was
// drawn and where, as a first argument treated as a pointer draws whatever is there.
// The format group prints sprintf's text beside it, for the same arguments. Limits
// come last, so a crash there leaves the rest.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export FntPrint
func fntPrint(a0, a1, a2, a3, a4, a5, a6, a7 uint32) int32

//export sprintf
func sprintf(s *byte, f *byte, a2, a3, a4, a5, a6, a7, a8, a9 uint32) int32

const hexDigits = "0123456789abcdef"

var (
	line  [4096]byte
	nline int
)

func putc(c byte) {
	if nline < len(line) {
		line[nline] = c
		nline++
	}
	if c == '\n' {
		yaroze.Write(1, &line[0], int32(nline))
		nline = 0
	}
}

func ps(s string) {
	for i := 0; i < len(s); i++ {
		putc(s[i])
	}
}

func pi(v int32) {
	if v < 0 {
		putc('-')
		v = -v
	}
	var d [12]byte
	k := 0
	for {
		d[k] = byte('0' + v%10)
		k++
		v /= 10
		if v == 0 {
			break
		}
	}
	for k > 0 {
		k--
		putc(d[k])
	}
}

func hex32(v uint32) {
	for i := range 8 {
		putc(hexDigits[v>>(28-4*i)&15])
	}
}

func esc(c byte) {
	switch {
	case c == '\\' || c == '"':
		putc('\\')
		putc(c)
	case c == '\n':
		ps("\\n")
	case c < 32 || c >= 127:
		ps("\\x")
		putc(hexDigits[c>>4])
		putc(hexDigits[c&15])
	default:
		putc(c)
	}
}

// One glyph of a flush.
type glyph struct{ x, y, c int32 }

var (
	glyphs [1100]glyph
	ng     int
)

// flush flushes id and leaves its sprites in glyphs, first character first.
func flush(id int32) {
	ng = 0
	head := yaroze.FntFlush(id)
	yaroze.DrawSync(0)
	if head == nil {
		return
	}
	p := head
	for n := 0; n < 1100 && p != nil; n++ {
		tag := *p
		size := int(tag >> 24)
		if size == 3 && ng < len(glyphs) {
			xy := *(*uint32)(unsafe.Add(unsafe.Pointer(p), 8))
			uv := *(*uint32)(unsafe.Add(unsafe.Pointer(p), 12))
			k := int32(uv>>8&255)/8*16 + int32(uv&255)/8
			glyphs[ng] = glyph{int32(xy & 0xffff), int32(xy >> 16), k + 32}
			ng++
		}
		next := tag & 0xffffff
		if next == 0xffffff {
			break
		}
		p = (*uint32)(unsafe.Pointer(uintptr(0x80000000 | next)))
	}
}

// text prints glyphs as text.
func text() {
	x, y := int32(0), int32(0)
	for i := ng - 1; i >= 0; i-- {
		g := glyphs[i]
		for y < g.y {
			putc('/')
			y += 8
			x = 0
		}
		for x < g.x {
			putc(' ')
			x += 8
		}
		// A conversion FntPrint lacks can draw stale stack bytes, so only text prints.
		if g.c >= 32 && g.c < 127 {
			esc(byte(g.c))
		} else {
			putc('?')
		}
		x += 8
	}
}

var (
	fbuf [256]byte
	ok   = [...]byte{'O', 'K', 0}
	s1   = [...]byte{'h', 'e', 'l', 'l', 'o', 0}
	out  [2048]byte
	long [2000]byte
)

func addr(b *byte) uint32 { return uint32(uintptr(unsafe.Pointer(b))) }

func z(f string) uint32 {
	n := copy(fbuf[:len(fbuf)-1], f)
	fbuf[n] = 0
	return addr(&fbuf[0])
}

func open(n int32, streams int) {
	yaroze.FntLoad(960, 256)
	for range streams {
		yaroze.FntOpen(0, 0, 320, 240, 0, n)
	}
	yaroze.DrawSync(0)
}

func idCase(a0 uint32) {
	open(64, 4)
	r := fntPrint(a0, addr(&ok[0]), 0, 0, 0, 0, 0, 0)
	ps("id ")
	hex32(a0)
	ps(" -> ")
	pi(r)
	ps(" streams")
	for i := range int32(4) {
		flush(i)
		putc(' ')
		switch {
		case ng == 2 && glyphs[1].c == 'O' && glyphs[0].c == 'K':
			ps("OK")
		case ng == 0:
			ps("-")
		default:
			ps("other")
		}
	}
	putc('\n')
}

// fm runs a format both ways and prints sprintf's text beside them.
func fm(f string, a ...uint32) {
	var w [6]uint32
	copy(w[:], a)
	ps("\"")
	for i := 0; i < len(f); i++ {
		esc(f[i])
	}
	ps("\" sprintf ")
	z(f)
	r := sprintf(&out[0], &fbuf[0], w[0], w[1], w[2], w[3], w[4], w[5], 0, 0)
	pi(r)
	ps(" \"")
	for i := range r {
		esc(out[i])
	}
	ps("\" plain ")
	open(512, 1)
	r = fntPrint(addr(&fbuf[0]), w[0], w[1], w[2], w[3], w[4], w[5], 0)
	pi(r)
	ps(" \"")
	flush(0)
	text()
	ps("\" id0 ")
	open(512, 1)
	r = fntPrint(0, addr(&fbuf[0]), w[0], w[1], w[2], w[3], w[4], w[5])
	pi(r)
	ps(" \"")
	flush(0)
	text()
	ps("\"\n")
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	// The scratchpad holds a string, as a pointer in that range would read it.
	sp := (*[4]byte)(yaroze.Pointer(0x1f800000))
	sp[0], sp[1], sp[2] = 'S', 'P', 0

	ps("== id\n")
	for _, a := range [...]uint32{
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 15, 16, 31, 32, 63, 64, 100, 255, 256, 511, 512, 1023, 1024, 4095, 4096, 0xffff,
		0x200000, 0x1f800000, 0x80000000, 0xa0000000, 0xfffffff8, 0xfffffffe, 0xffffffff,
	} {
		idCase(a)
	}
	ps("== streams\n")
	// A format pointer and an id, on stream 2's text.
	open(64, 4)
	ps("id2 ")
	pi(fntPrint(2, z("X%d"), 7, 0, 0, 0, 0, 0))
	ps(" again ")
	pi(fntPrint(2, z("Y%d"), 8, 0, 0, 0, 0, 0))
	ps(" plain ")
	pi(fntPrint(z("Z%d"), 9, 0, 0, 0, 0, 0, 0))
	ps(" id0 ")
	pi(fntPrint(0, z("W%d"), 1, 0, 0, 0, 0, 0))
	ps(" : ")
	for i := range int32(4) {
		flush(i)
		ps("[")
		text()
		ps("] ")
	}
	putc('\n')
	// No stream open.
	yaroze.FntLoad(960, 256)
	ps("closed plain ")
	pi(fntPrint(z("A%d"), 1, 0, 0, 0, 0, 0, 0))
	ps(" id0 ")
	pi(fntPrint(0, z("A%d"), 1, 0, 0, 0, 0, 0))
	ps(" id1 ")
	pi(fntPrint(1, z("A%d"), 1, 0, 0, 0, 0, 0))
	putc('\n')

	ps("== formats\n")
	str := addr(&s1[0])
	fm("plain")
	fm("")
	fm("%d", 42)
	fm("%d", 0xffffffd6)
	fm("%d", 0x80000000)
	fm("%i", 42)
	fm("%u", 0xffffffd6)
	fm("%o", 8)
	fm("%x", 0xbeef)
	fm("%X", 0xbeef)
	fm("%c", 65)
	fm("%c", 0x61)
	fm("%s", str)
	fm("%s", 0)
	fm("%p", 0xbeef)
	fm("%5d", 42)
	fm("%-5d|", 42)
	fm("%05d", 42)
	fm("%+d", 42)
	fm("% d", 42)
	fm("%.3d", 42)
	fm("%.0d", 0)
	fm("%5.3d", 42)
	fm("%#x", 42)
	fm("%#X", 42)
	fm("%#o", 42)
	fm("%#p", 42)
	fm("%*d", 5, 42)
	fm("%*d", 0xfffffffb, 42)
	fm("%.*d", 5, 42)
	fm("%ld", 42)
	fm("%hd", 0x12345)
	fm("%lld", 42, 43)
	fm("%Ld", 42, 43)
	fm("%.2s", str)
	fm("%8s", str)
	fm("%-8s|", str)
	fm("%%")
	fm("a%%b")
	fm("a%")
	fm("%")
	fm("%y", 42)
	fm("a%yb%d", 42, 43)
	fm("%f", 0, 0x3ff80000)
	fm("%e", 0, 0x3ff80000)
	fm("%g", 0, 0x3ff80000)
	fm("%E", 0, 0x3ff80000)
	fm("%G", 0, 0x3ff80000)
	fm("a%fb%d", 7, 8)
	fm("%d %d %d %d %d", 1, 2, 3, 4, 5)
	fm("a\nb%d", 42)
	fm("~c700%d", 42)
	fm("%s~", str)
	fm("%c", 0x7e)
	fm("%c%c", 0x7e, 0x41)
	fm("%5x", 0xbeef)
	fm("%5X", 0xbeef)
	fm("%2x", 0xbeef)
	fm("%x", 0xffffffd6)
	fm("%5c", 65)
	fm("%2c", 65)
	fm("%5%")
	fm("%5s", 0)
	fm("%05s", str)
	fm("%05x", 0xbeef)
	fm("%2d", 12345)
	fm("%c|", 0)
	fm("%s|", 0)
	fm("%5y|", 7)
	fm("%05X", 0xbeef)
	fm("%08x", 0xbeef)
	fm("%08d", 42)
	fm("%08d", 0xffffffd6)
	fm("%0d", 42)
	fm("%03c", 65)
	fm("%07s", str)
	fm("%10d", 0x80000000)
	fm("%12x", 0xffffffd6)
	fm("%2c%3s", 65, str)

	// %n through a real pointer.
	var cnt [2]uint32
	open(512, 1)
	cnt[0], cnt[1] = 0x11111111, 0x22222222
	ps("%n ")
	pi(fntPrint(z("abc%n"), uint32(uintptr(unsafe.Pointer(&cnt[0]))), 0, 0, 0, 0, 0, 0))
	putc(' ')
	hex32(cnt[0])
	putc(' ')
	hex32(cnt[1])
	putc('\n')

	ps("== counts\n")
	// Counts over several prints, on a stream of n.
	seq := func(n int32, fs ...string) {
		open(n, 2)
		ps("n ")
		pi(n)
		for _, f := range fs {
			ps(" \"")
			ps(f)
			ps("\" ")
			pi(fntPrint(z(f), 123456, 0, 0, 0, 0, 0, 0))
		}
		ps(" : ")
		flush(0)
		ps("[")
		text()
		ps("] ng ")
		pi(int32(ng))
		ps(" next ")
		flush(1)
		ps("[")
		text()
		ps("]\n")
	}
	seq(8, "ab")
	seq(8, "ab", "cd")
	seq(8, "%d")
	seq(8, "%d", "%d")
	seq(6, "%d")
	seq(6, "%d", "")
	seq(5, "%d")
	seq(5, "%d", "x")
	seq(5, "%d", "")
	seq(8, "ab", "%d", "x")
	seq(8, "ab", "%d")
	seq(8, "%10d")
	seq(8, "%-10d")
	seq(8, "abcdefgh")
	seq(8, "abcdefgh", "")
	seq(8, "abcdefgh", "x")
	seq(8, "abcdefghijk", "x")
	seq(8, "a%%%%%%%%b")
	seq(1, "%d")
	seq(0, "%d")
	seq(0, "")
	seq(1, "")

	ps("== long\n")
	for i := range long {
		long[i] = 'A' + byte(i%26)
	}
	lp := addr(&long[0])
	for _, n := range [...]int{100, 200, 255, 256, 257, 300, 400, 500, 511, 512, 513, 600, 1000, 1023, 1024, 1025} {
		long[n] = 0
		open(1024, 1)
		ps("%s len ")
		pi(int32(n))
		r := fntPrint(z("%s"), lp, 0, 0, 0, 0, 0, 0)
		ps(" -> ")
		pi(r)
		flush(0)
		ps(" ng ")
		pi(int32(ng))
		ps(" last ")
		if ng > 0 {
			esc(byte(glyphs[0].c))
		}
		putc('\n')
		long[n] = 'A' + byte(n%26)
		// As literal text, the same length.
		long[n] = 0
		open(1024, 1)
		ps("literal len ")
		pi(int32(n))
		r = fntPrint(lp, 0, 0, 0, 0, 0, 0, 0)
		ps(" -> ")
		pi(r)
		flush(0)
		ps(" ng ")
		pi(int32(ng))
		putc('\n')
		long[n] = 'A' + byte(n%26)
	}
	for _, f := range [...]string{"%100d", "%255d", "%256d", "%300d", "%511d", "%512d", "%513d", "%600d", "%1000d", "%1023d", "%1024d", "%1025d", "%.300d", "%.512d", "%.1000d", "%-1000d|"} {
		open(1024, 1)
		ps(f)
		ps(" -> ")
		pi(fntPrint(z(f), 42, 0, 0, 0, 0, 0, 0))
		flush(0)
		ps(" ng ")
		pi(int32(ng))
		putc('\n')
	}
	ps("END\n")
}
