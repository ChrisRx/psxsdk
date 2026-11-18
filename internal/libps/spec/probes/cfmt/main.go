//go:build yaroze

// Command cfmt is a probe for spec 17: the integer, character and string conversions
// of sprintf, sprintf2, printf and printf2 over grids of flags, widths, precisions
// and length modifiers, every byte as a conversion letter, and malformed directives.
// Each line is the format, the argument words (S for a pointer to "hello", E for a
// pointer to ""), the return value, and the text, between quotes with anything
// unprintable escaped and how many bytes of the guarded destination changed for the
// sprintf forms, or between brackets as the printf forms print it.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export sprintf
func sprintf(s *byte, f *byte, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11 uint32) int32

//export sprintf2
func sprintf2(s *byte, f *byte, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11 uint32) int32

//export printf
func printf(f *byte, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11 uint32) int32

//export printf2
func printf2(f *byte, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11 uint32) int32

const hexDigits = "0123456789abcdef"

var (
	line  [8192]byte
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
	}
	u := uint32(v)
	if v < 0 {
		u = -u
	}
	var d [12]byte
	k := 0
	for {
		d[k] = byte('0' + u%10)
		k++
		u /= 10
		if u == 0 {
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
	fbuf  [256]byte
	out   [600]byte
	s1    = [...]byte{'h', 'e', 'l', 'l', 'o', 0}
	empty = [...]byte{0}
)

const guard = 0xa5

func z(f string) *byte {
	n := copy(fbuf[:len(fbuf)-1], f)
	fbuf[n] = 0
	return &fbuf[0]
}

func strp() uint32   { return uint32(uintptr(unsafe.Pointer(&s1[0]))) }
func emptyp() uint32 { return uint32(uintptr(unsafe.Pointer(&empty[0]))) }

// family is the entry under test: 0 sprintf, 1 sprintf2, 2 printf, 3 printf2.
var family int

func argEsc(f string, a []uint32) {
	ps("\"")
	for i := 0; i < len(f); i++ {
		esc(f[i])
	}
	ps("\"")
	for _, v := range a {
		if v == strp() {
			ps(" S")
			continue
		}
		if v == emptyp() {
			ps(" E")
			continue
		}
		putc(' ')
		hex32(v)
	}
}

// t runs one call and prints its line.
func t(f string, a ...uint32) {
	var w [10]uint32
	copy(w[:], a)
	var r int32
	if family >= 2 {
		argEsc(f, a)
		ps(" [")
		yaroze.Write(1, &line[0], int32(nline))
		nline = 0
		if family == 2 {
			r = printf(z(f), w[0], w[1], w[2], w[3], w[4], w[5], w[6], w[7], w[8], w[9], 0)
		} else {
			r = printf2(z(f), w[0], w[1], w[2], w[3], w[4], w[5], w[6], w[7], w[8], w[9], 0)
		}
		ps("] -> ")
		pi(r)
		putc('\n')
		return
	}
	for i := range out {
		out[i] = guard
	}
	if family == 0 {
		r = sprintf(&out[0], z(f), w[0], w[1], w[2], w[3], w[4], w[5], w[6], w[7], w[8], w[9])
	} else {
		r = sprintf2(&out[0], z(f), w[0], w[1], w[2], w[3], w[4], w[5], w[6], w[7], w[8], w[9])
	}
	last := -1
	for i := range out {
		if out[i] != guard {
			last = i
		}
	}
	argEsc(f, a)
	ps(" -> ")
	pi(r)
	ps(" \"")
	for i := 0; i <= last; i++ {
		esc(out[i])
	}
	ps("\" w")
	pi(int32(last + 1))
	putc('\n')
}

// sb builds a format in place, with no allocation.
var (
	sb  [64]byte
	nsb int
)

func add(s string) {
	nsb += copy(sb[nsb:], s)
}

func cur() string { return unsafe.String(&sb[0], nsb) }

var (
	allFlags = [...]string{
		"", "-", "+", " ", "#", "0", "-+", "- ", "-#", "-0", "+ ", "+#", "+0", " #", " 0", "#0",
		"-+ ", "-+#", "-+0", "- #", "- 0", "-#0", "+ #", "+ 0", "+#0", " #0",
		"-+ #", "-+ 0", "-+#0", "- #0", "+ #0", "-+ #0",
	}
	fewFlags = [...]string{"", "-", "0", "+", " ", "#"}
)

type conv struct {
	c    string
	vals []uint32
}

// vals runs every value of conversion c on the format so far, with any
// star arguments stars first.
func vals(c conv, stars ...uint32) {
	var a [6]uint32
	n := copy(a[:], stars)
	save := nsb
	add(c.c)
	for _, v := range c.vals {
		a[n] = v
		t(cur(), a[:n+1]...)
	}
	nsb = save
}

func gridA(convs []conv) {
	for fi := range allFlags {
		for _, w := range [...]string{"", "6"} {
			for _, p := range [...]string{"", ".0", ".4"} {
				for ci := range convs {
					nsb = 0
					add("%")
					add(allFlags[fi])
					add(w)
					add(p)
					vals(convs[ci])
				}
			}
		}
	}
}

func gridB(convs []conv) {
	for fi := range fewFlags {
		for _, w := range [...]string{"", "1", "12"} {
			for _, p := range [...]string{"", ".", ".1", ".8", ".12"} {
				for ci := range convs {
					nsb = 0
					add("%")
					add(fewFlags[fi])
					add(w)
					add(p)
					vals(convs[ci])
				}
			}
		}
		// Stars.
		for ci := range convs {
			nsb = 0
			add("%")
			add(fewFlags[fi])
			add("*")
			for _, st := range [...]uint32{0, 6, 0xfffffffa} {
				vals(convs[ci], st)
			}
			nsb = 0
			add("%")
			add(fewFlags[fi])
			add(".*")
			for _, st := range [...]uint32{0, 3, 0xffffffff} {
				vals(convs[ci], st)
			}
			nsb = 0
			add("%")
			add(fewFlags[fi])
			add("*.*")
			vals(convs[ci], 8, 3)
			vals(convs[ci], 0xfffffff8, 3)
		}
	}
}

var (
	cnt [4]uint32
	lo  = [...]byte{'h', 'e', 'l', 'l', 'o', 0}
)

func run(convs []conv) {
	ps("== A\n")
	gridA(convs)
	ps("== B\n")
	gridB(convs)
	ps("== C lengths\n")
	for _, l := range [...]string{"h", "l", "L", "ll", "hh", "lh", "hl", "q", "j", "z", "t", "hL", "Ll", "lL"} {
		for ci := range convs {
			for _, fl := range [...]string{"", "-6", "06.3", "+", "#"} {
				nsb = 0
				add("%")
				add(fl)
				add(l)
				add(convs[ci].c)
				save := nsb
				add("|%d")
				for _, v := range convs[ci].vals {
					t(cur(), v, 0x7777, 0x8888)
				}
				nsb = save
			}
		}
		nsb = 0
		add("%")
		add(l)
		add("d")
		for _, v := range [...]uint32{0x12345, 0x18000, 0x1ffff, 0x10000} {
			t(cur(), v)
		}
		nsb = 0
		add("%")
		add(l)
		add("x")
		t(cur(), 0x123456, 0x789)
		nsb = 0
		add("%")
		add(l)
		add("o")
		t(cur(), 0x123456, 0x789)
		nsb = 0
		add("%")
		add(l)
		add("u")
		t(cur(), 0x18000, 0x789)
	}
	ps("== D every byte\n")
	cp := uint32(uintptr(unsafe.Pointer(&cnt[1])))
	for c := 1; c < 256; c++ {
		a0 := uint32(0x41)
		if c == 'n' {
			a0 = cp
		}
		if c == 's' {
			a0 = strp()
		}
		sb[0] = '%'
		sb[1] = byte(c)
		copy(sb[2:], "|%d")
		nsb = 5
		t(cur(), a0, 0x2b)
		sb[0] = '%'
		copy(sb[1:], "-6")
		sb[3] = byte(c)
		copy(sb[4:], "|%d")
		nsb = 7
		t(cur(), a0, 0x2b)
		sb[0] = '%'
		copy(sb[1:], "l")
		sb[2] = byte(c)
		copy(sb[3:], "|%d")
		nsb = 6
		t(cur(), a0, 0x2b)
		sb[0] = '%'
		sb[1] = byte(c)
		nsb = 2
		t(cur(), a0, 0x2b)
	}
	ps("== E malformed\n")
	for _, f := range [...]string{
		"%", "a%", "%%", "%%%", "%%%%", "%5%", "%-5%|", "%05%", "%.3%", "%l%", "%h%", "%+%",
		"%-", "%+", "% ", "%#", "%0", "%5", "%.", "%.5", "%h", "%l", "%L", "%ll", "%-5", "%5.",
		"%*", "%.*",
		"%0-5d|", "%-05d|", "%5-d|", "%5+d|", "%5 d|", "%5#x|", "%50d|", "%005d|", "%0005d|",
		"%.5.3d|", "%5.3.2d|", "%.-3d|", "%.+3d|", "%. 3d|", "%.#3x|", "%.03d|", "%.003d|", "%**d|", "%*5d|", "%5*d|",
		"%.**d|", "%.*5d|", "%.5*d|", "%l5d|", "%h5d|", "%l.3d|", "%l-5d|", "%lh5d|", "%--5d|", "%++d|", "%  d|", "%##x|", "%00d|",
		"%+-5d|", "%-+5d|", "% +d|", "%+ d|", "%0+5d|", "%+05d|", "%0 5d|", "% 05d|",
		"%2147483647", "%4294967296d|", "%4294967301d|",
		"%\n", "%\t", "\n", "a\nb", "\t", "\r", "\x7f", "\x80\xff",
	} {
		t(f, 7, 9, 11, 13)
	}
	ps("== F n\n")
	for _, f := range [...]string{"%n", "abc%n", "abc%nxyz", "%5d%n", "abc%hn", "abc%ln", "abc%5n", "abc%-5n", "abc%.2n", "abc%ln|", "abc%Ln", "abc%hhn", "%%%n", "%c%n", "abc%d%n"} {
		cnt[0], cnt[1], cnt[2], cnt[3] = 0x11111111, 0x22222222, 0x33333333, 0x44444444
		switch f {
		case "%5d%n", "%c%n", "abc%d%n":
			t(f, 42, cp)
		default:
			t(f, cp)
		}
		ps("  cnt ")
		for _, v := range cnt {
			hex32(v)
			putc(' ')
		}
		putc('\n')
	}
}

func main() {
	full := []conv{
		{"d", []uint32{0, 1, 42, 0xffffffd6, 65537, 0x80000000, 0x7fffffff}},
		{"i", []uint32{0, 42, 0xffffffd6}},
		{"u", []uint32{0, 42, 0xffffffd6, 65537}},
		{"o", []uint32{0, 8, 0xffffffd6}},
		{"x", []uint32{0, 0xbeef, 0xffffffd6}},
		{"X", []uint32{0, 0xbeef, 0xffffffd6}},
		{"p", []uint32{0, 0xbeef, 0xffffffd6}},
		{"c", []uint32{65, 0, 0x141}},
		{"s", []uint32{strp(), emptyp(), 0}},
	}
	thin := []conv{
		{"d", []uint32{0, 42, 0xffffffd6, 0x80000000}},
		{"i", []uint32{42}},
		{"u", []uint32{0, 0xffffffd6}},
		{"o", []uint32{0, 8}},
		{"x", []uint32{0, 0xbeef}},
		{"X", []uint32{0xbeef}},
		{"p", []uint32{0, 0xbeef}},
		{"c", []uint32{65, 0}},
		{"s", []uint32{strp(), emptyp(), 0}},
	}
	for family = range 4 {
		ps("=== family ")
		pi(int32(family))
		putc('\n')
		if family < 2 {
			run(full)
		} else {
			run(thin)
		}
	}
	ps("END\n")
}
