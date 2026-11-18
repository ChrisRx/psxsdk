//go:build yaroze

// Command cfntk is a probe for spec 17: KanjiFntPrint as C calls it. Each case makes
// a kanji stream, prints a format with arguments, flushes, and hashes the stream's
// glyph image read back from video memory, then does the same with sprintf's text
// for the same arguments as a literal. Equal hashes say the text was the same.
// Printed: the two returns, how many bytes sprintf made, and the two hashes. Later
// groups compare glyph cells alone, as an image's size follows the count.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export KanjiFntPrint
func kprint(a0, a1, a2, a3, a4, a5, a6, a7 uint32) int32

//export sprintf
func sprintf(s *byte, f *byte, a2, a3, a4, a5, a6, a7, a8, a9 uint32) int32

const hexDigits = "0123456789abcdef"

var (
	line  [512]byte
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

var (
	fbuf  [128]byte
	out   [1024]byte
	s1    = [...]byte{'h', 'e', 'l', 'l', 'o', 0}
	image [256 * 48]uint16
)

func addr(b *byte) uint32 { return uint32(uintptr(unsafe.Pointer(b))) }

func z(f string) uint32 {
	n := copy(fbuf[:len(fbuf)-1], f)
	fbuf[n] = 0
	return addr(&fbuf[0])
}

func open() {
	yaroze.KanjiFntClose()
	yaroze.KanjiFntOpen(0, 0, 320, 240, 640, 0, 0, 480, 1, 256)
	yaroze.DrawSync(0)
}

// flush flushes stream 0 and hashes its glyph image from video memory.
func flush() uint32 {
	yaroze.KanjiFntFlush(0)
	yaroze.DrawSync(0)
	r := yaroze.RECT{X: 640, Y: 0, W: 256, H: 16 * 3}
	for i := range image {
		image[i] = 0
	}
	yaroze.StoreImage(&r, (*uint32)(unsafe.Pointer(&image[0])))
	yaroze.DrawSync(0)
	h := uint32(2166136261)
	for i := range 256 * 3 * 16 {
		h = (h ^ uint32(image[i])) * 16777619
	}
	return h
}

func clearVRAM() {
	r := yaroze.RECT{X: 640, Y: 0, W: 256, H: 48}
	yaroze.ClearImage(&r, 0, 0, 0)
	yaroze.DrawSync(0)
}

func fm(f string, a ...uint32) {
	var w [6]uint32
	copy(w[:], a)
	ps("\"")
	for i := 0; i < len(f); i++ {
		esc(f[i])
	}
	ps("\" sprintf ")
	z(f)
	n := sprintf(&out[0], &fbuf[0], w[0], w[1], w[2], w[3], w[4], w[5], 0, 0)
	pi(n)
	ps(" \"")
	for i := range n {
		esc(out[i])
	}
	ps("\" kanji ")
	clearVRAM()
	open()
	r := kprint(addr(&fbuf[0]), w[0], w[1], w[2], w[3], w[4], w[5], 0)
	pi(r)
	h1 := flush()
	ps(" literal ")
	clearVRAM()
	open()
	r = kprint(addr(&out[0]), 0, 0, 0, 0, 0, 0, 0)
	pi(r)
	h2 := flush()
	ps(" hash ")
	hex32(h1)
	putc(' ')
	hex32(h2)
	if h1 == h2 {
		ps(" same\n")
	} else {
		ps(" differ\n")
	}
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)

	str := addr(&s1[0])
	fm("plain")
	fm("")
	fm("%d", 42)
	fm("%d", 0xffffffd6)
	fm("%d", 12345678)
	fm("%i", 42)
	fm("%u", 42)
	fm("%x", 0xbeef)
	fm("%X", 0xbeef)
	fm("%o", 8)
	fm("%c", 65)
	fm("%s", str)
	fm("%s", 0)
	fm("%p", 0xbeef)
	fm("%5d", 42)
	fm("%-5d|", 42)
	fm("%05d", 42)
	fm("%+d", 42)
	fm("%.3d", 42)
	fm("%5.3d", 42)
	fm("%#x", 42)
	fm("%ld", 42)
	fm("%hd", 42)
	fm("%8s", str)
	fm("%.2s", str)
	fm("%%")
	fm("a%%b")
	fm("%y", 42)
	fm("a%d", 42)
	fm("a%db", 42)
	fm("abc%d", 42)
	fm("%dabc", 42)
	fm("a%db%d", 42, 7)
	fm("%s%s", str, str)
	fm("%5d%5d", 1, 2)
	fm("%c%c", 65, 66)

	ps("== candidates\n")
	// What a format's text is, against literals.
	lit := func(l string) {
		var buf [32]byte
		copy(buf[:], l)
		clearVRAM()
		open()
		r := kprint(addr(&buf[0]), 0, 0, 0, 0, 0, 0, 0)
		h := flush()
		ps("  literal \"")
		for i := 0; i < len(l); i++ {
			esc(l[i])
		}
		ps("\" ")
		pi(r)
		putc(' ')
		hex32(h)
		putc('\n')
	}
	one := func(f string, a ...uint32) {
		var w [6]uint32
		copy(w[:], a)
		clearVRAM()
		open()
		r := kprint(z(f), w[0], w[1], w[2], w[3], w[4], w[5], 0)
		h := flush()
		ps("format \"")
		ps(f)
		ps("\" ")
		pi(r)
		putc(' ')
		hex32(h)
		putc('\n')
	}
	one("%d", 0xffffffd6)
	for _, l := range [...]string{"-42", "- 42", "42-", "--42", "42", " 42", "-  42", " -42", "-4 2", "4-2", "-", "\x81\x7c42"} {
		lit(l)
	}
	one("%5d", 0xffffffd6)
	lit("  -42")
	one("%05d", 42)
	lit("   42")
	lit("00042")
	one("%x", 0xbeef)
	lit("beef")
	lit("BEEF")
	one("%X", 0xbeef)
	one("%x", 0xffffffd6)
	lit("FFFFFFD6")
	lit("ffffffd6")
	one("%c", 0x61)
	lit("a")
	lit("A")
	one("%s", 0)
	lit("")
	lit(" ")
	one("%y", 42)
	lit(" ")
	lit("")
	one("%%")
	one("%5s", addr(&s1[0]))
	lit("hello")
	one("%6s", addr(&s1[0]))
	lit(" hello")
	one("%2s", addr(&s1[0]))
	one("%6c", 0x41)
	lit("     A")
	one("%6x", 0xbeef)
	lit("  BEEF")
	one("%-6d", 42)
	ps("== pairs\n")
	// The image's size follows the count, so only the cells of the text compare.
	cells := func(n int) uint32 {
		h := uint32(2166136261)
		for y := range 16 {
			for x := range 4 * n {
				h = (h ^ uint32(image[y*256+x])) * 16777619
			}
		}
		return h
	}
	pair := func(n int, l, f string, a ...uint32) {
		var w [6]uint32
		copy(w[:], a)
		clearVRAM()
		open()
		kprint(z(f), w[0], w[1], w[2], w[3], w[4], w[5], 0)
		flush()
		h1 := cells(n)
		var buf [32]byte
		copy(buf[:], l)
		clearVRAM()
		open()
		kprint(addr(&buf[0]), 0, 0, 0, 0, 0, 0, 0)
		flush()
		h2 := cells(n)
		ps("\"")
		ps(f)
		ps("\" against \"")
		ps(l)
		if h1 == h2 {
			ps("\" same\n")
		} else {
			ps("\" differ\n")
		}
	}
	pair(3, "-42", "%d", 0xffffffd6)
	pair(3, "42-", "%d", 0xffffffd6)
	pair(3, "-42", "%5d", 0xffffffd6)
	pair(2, "42", "%d", 42)
	pair(2, "42", "%05d", 42)
	pair(5, "00042", "%05d", 42)
	pair(4, "BEEF", "%x", 0xbeef)
	pair(4, "beef", "%x", 0xbeef)
	pair(4, "BEEF", "%X", 0xbeef)
	pair(1, "a", "%c", 0x61)
	pair(1, "A", "%c", 0x61)
	pair(5, "hello", "%6s", addr(&s1[0]))
	pair(2, "6d", "%-6d", 42)
	pair(3, "3d", "%.3d", 42)
	pair(2, "42", "%4x", 0x42)
	pair(4, "BEEF", "%6x", 0xbeef)
	pair(5, "0BEEF", "%05x", 0xbeef)
	pair(4, "BEEF", "%05x", 0xbeef)
	pair(2, "12", "%d%d", 1, 2)
	ps("== negative\n")
	// Each cell of %d of -42, against single characters.
	cell := func(i int) uint32 {
		h := uint32(2166136261)
		for y := range 16 {
			for x := range 4 {
				h = (h ^ uint32(image[y*256+4*i+x])) * 16777619
			}
		}
		return h
	}
	clearVRAM()
	open()
	kprint(z("%d"), 0xffffffd6, 0, 0, 0, 0, 0, 0)
	flush()
	for i := range 6 {
		ps("cell ")
		pi(int32(i))
		putc(' ')
		hex32(cell(i))
		putc('\n')
	}
	for _, l := range [...]string{"-", "4", "2", "0", "a", "\x81\x7c"} {
		var buf [4]byte
		copy(buf[:], l)
		clearVRAM()
		open()
		kprint(addr(&buf[0]), 0, 0, 0, 0, 0, 0, 0)
		flush()
		ps("literal ")
		esc(buf[0])
		putc(' ')
		hex32(cell(0))
		putc('\n')
	}
	want := uint32(0)
	{
		clearVRAM()
		open()
		kprint(z("%d"), 0xffffffd6, 0, 0, 0, 0, 0, 0)
		flush()
		want = cell(0)
		for c := 0x21; c < 0x7f; c++ {
			if c == '%' || c == '~' {
				continue
			}
			var buf [4]byte
			buf[0] = byte(c)
			clearVRAM()
			open()
			kprint(addr(&buf[0]), 0, 0, 0, 0, 0, 0, 0)
			flush()
			if cell(0) == want {
				ps("sign cell matches byte ")
				pi(int32(c))
				putc('\n')
			}
		}
		// The cell's pixels, four words a row.
		clearVRAM()
		open()
		kprint(z("%d"), 0xffffffd6, 0, 0, 0, 0, 0, 0)
		flush()
		for y := range 16 {
			for x := range 4 {
				hex32(uint32(image[y*256+x]))
				putc(' ')
			}
			putc('\n')
		}
	}
	ps("== counts\n")
	for _, f := range [...]string{"%1d", "%2d", "%3d", "%4d", "%10d", "%1x", "%4x", "%8x", "%6s", "%7s", "%10s", "%2s", "%c", "%2c", "%6c", "%5%", "%%", "%2y"} {
		var v uint32 = 42
		switch f[len(f)-1] {
		case 's':
			v = addr(&s1[0])
		case 'c':
			v = 65
		}
		open()
		ps(f)
		ps(" -> ")
		pi(kprint(z(f), v, 0, 0, 0, 0, 0, 0))
		putc('\n')
	}
	ps("== strings\n")
	// %s and %c widths against the string's length, and after a number of each size.
	short := [...][8]byte{{'a'}, {'a', 'b'}, {'a', 'b', 'c'}, {'a', 'b', 'c', 'd', 'e', 'f', 'g'}}
	for i := range short {
		for _, f := range [...]string{"%s", "%1s", "%2s", "%3s", "%4s", "%6s", "%10s"} {
			open()
			ps(f)
			ps(" len ")
			pi(int32(i + 1))
			ps(" -> ")
			pi(kprint(z(f), addr(&short[i][0]), 0, 0, 0, 0, 0, 0))
			putc('\n')
		}
	}
	for _, d := range [...]uint32{7, 123456} {
		open()
		pi(kprint(z("%d"), d, 0, 0, 0, 0, 0, 0))
		ps(" then ")
		open()
		pi(kprint(z("%6s"), addr(&short[0][0]), 0, 0, 0, 0, 0, 0))
		ps(" together ")
		open()
		pi(kprint(z("%d%6s"), d, addr(&short[0][0]), 0, 0, 0, 0, 0))
		putc('\n')
	}
	for _, c := range [...]uint32{0x41, 0x7f, 0x20} {
		for _, f := range [...]string{"%c", "%1c", "%2c", "%3c", "%6c"} {
			open()
			ps(f)
			ps(" -> ")
			pi(kprint(z(f), c, 0, 0, 0, 0, 0, 0))
			putc(' ')
		}
		putc('\n')
	}
	// The number's width against its digits.
	for _, f := range [...]string{"%1d", "%2d", "%3d", "%6d", "%7d", "%10d"} {
		for _, d := range [...]uint32{7, 12345, 0xffffffd6} {
			open()
			ps(f)
			ps(" -> ")
			pi(kprint(z(f), d, 0, 0, 0, 0, 0, 0))
			putc(' ')
		}
		putc('\n')
	}
	// Each digit and hex letter's glyph, against the full-width codes.
	for _, l := range [...][2]string{
		{"%d", "\x82\x53\x82\x51"}, {"%x", "\x82\x60\x82\x62"}, {"%X", "\x82\x60\x82\x62"},
	} {
		var v uint32 = 42
		if l[0] != "%d" {
			v = 0xac
		}
		var buf [8]byte
		copy(buf[:], l[1])
		clearVRAM()
		open()
		kprint(z(l[0]), v, 0, 0, 0, 0, 0, 0)
		flush()
		h1 := cells(2)
		clearVRAM()
		open()
		r := kprint(addr(&buf[0]), 0, 0, 0, 0, 0, 0, 0)
		flush()
		h2 := cells(2)
		ps(l[0])
		ps(" against full-width: count ")
		pi(r)
		if h1 == h2 {
			ps(" same\n")
		} else {
			ps(" differ\n")
		}
	}
	ps("== id\n")
	// A small first argument: is it taken as an id?
	ok := [...]byte{'O', 'K', 0}
	for _, a0 := range [...]uint32{0, 1, 2, 3, 4, 7, 8} {
		yaroze.KanjiFntClose()
		yaroze.KanjiFntOpen(0, 0, 320, 240, 640, 0, 0, 480, 1, 64)
		yaroze.KanjiFntOpen(0, 64, 320, 64, 640, 64, 0, 481, 1, 64)
		yaroze.DrawSync(0)
		ps("a0 ")
		pi(int32(a0))
		ps(" -> ")
		pi(kprint(a0, addr(&ok[0]), 0, 0, 0, 0, 0, 0))
		ps(" stream 0 ")
		pi(kprint(z("A"), 0, 0, 0, 0, 0, 0, 0))
		ps(" stream 1 ")
		pi(kprint(1, z("B"), 0, 0, 0, 0, 0, 0))
		putc('\n')
	}
	yaroze.KanjiFntClose()
	ps("closed ")
	pi(kprint(z("A"), 0, 0, 0, 0, 0, 0, 0))
	ps(" id ")
	pi(kprint(0, z("A"), 0, 0, 0, 0, 0, 0))
	putc('\n')
	ps("== zero fill\n")
	for _, f := range [...]string{"%05x", "%08x", "%05X", "%05d", "%08d"} {
		open()
		ps(f)
		ps(" -> ")
		pi(kprint(z(f), 0xbeef, 0, 0, 0, 0, 0, 0))
		putc('\n')
	}
	ps("END\n")
	// A conversion FntPrint lacks, with a zero first argument: Kanji text past this
	// did not come back in 2500 frames, so it runs after the END line.
	fm("%f", 0, 0x3ff80000)
}
