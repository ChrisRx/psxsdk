//go:build yaroze

// Command cfmtdbl is a probe for spec 17: sprintf2's e, E, f, g and G over doubles
// given by their bits. Hand-picked values go through a list of formats, an exponent
// sweep and random doubles through long precisions, to show how digits are made.
// Each line is the format, the double's bits, the return value, the text and how
// many bytes of the guarded destination changed.
package main

import (
	"go.chrisrx.dev/psx/yaroze"
)

//export sprintf2
func sprintf2(s *byte, f *byte, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11 uint32) int32

const hexDigits = "0123456789abcdef"

var (
	line  [2048]byte
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

func hex64(v uint64) {
	for i := range 16 {
		putc(hexDigits[v>>(60-4*i)&15])
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
	fbuf [64]byte
	out  [512]byte
)

const guard = 0xa5

func z(f string) *byte {
	n := copy(fbuf[:len(fbuf)-1], f)
	fbuf[n] = 0
	return &fbuf[0]
}

// t formats d by f and prints the line.
func t(f string, d uint64) {
	for i := range out {
		out[i] = guard
	}
	r := sprintf2(&out[0], z(f), uint32(d), uint32(d>>32), 0x1111, 0x2222, 0x3333, 0x4444, 0, 0, 0, 0)
	last := -1
	for i := range out {
		if out[i] != guard {
			last = i
		}
	}
	ps("\"")
	ps(f)
	ps("\" ")
	hex64(d)
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

// w is t with the words given.
func w(f string, a ...uint32) {
	var v [10]uint32
	copy(v[:], a)
	for i := range out {
		out[i] = guard
	}
	r := sprintf2(&out[0], z(f), v[0], v[1], v[2], v[3], v[4], v[5], v[6], v[7], v[8], v[9])
	last := -1
	for i := range out {
		if out[i] != guard {
			last = i
		}
	}
	ps("\"")
	ps(f)
	ps("\" words -> ")
	pi(r)
	ps(" \"")
	for i := 0; i <= last; i++ {
		esc(out[i])
	}
	ps("\" w")
	pi(int32(last + 1))
	putc('\n')
}

var seed uint32 = 0x12345678

func rnd() uint32 {
	seed ^= seed << 13
	seed ^= seed >> 17
	seed ^= seed << 5
	return seed
}

var values = [...]uint64{
	0x0000000000000000, 0x8000000000000000, // zeros
	0x3ff0000000000000, 0xbff0000000000000, 0x3fe0000000000000, 0x3ff8000000000000, 0x4004000000000000,
	0x3fb999999999999a, 0x3fc999999999999a, 0x3fd3333333333333, 0x3fd5555555555555, 0x3fe5555555555555,
	0x400921fb54442d18, 0x4005bf0a8b145769,
	0x4024000000000000, 0x4059000000000000, 0x408f400000000000, 0x40f86a0000000000, 0x412e848000000000,
	0x412e847e00000000, 0x412e847f00000000, // 999999, 999999.5
	0x3f1a36e2eb1c432d, 0x3ee4f8b588e368f1, 0x3ee4f8b1e0b4c0cc,
	0x4202a05f20000000, 0x430c6bf526340000, 0x4341c37937e08000, 0x4376345785d8a000, 0x4415af1d78b58c40,
	0x44b52d02c7e14af6, 0x44b52d02c7e14af7, 0x54b249ad2594c37d, 0x7e37e43c8800759c,
	0x7fefffffffffffff, 0xffefffffffffffff, 0x0010000000000000, 0x8010000000000000,
	0x0000000000000001, 0x8000000000000001, 0x000fffffffffffff, 0x0008000000000000,
	0x7ff0000000000000, 0xfff0000000000000, 0x7ff8000000000000, 0xfff8000000000000, 0x7ff0000000000001, 0x7fffffffffffffff,
	0x4023000000000000, 0x4058e00000000000, 0x3fee666666666666, 0x3fb851eb851eb852,
	0x3ff4000000000000, 0x3ff599999999999a, 0x4340000000000000, 0x4330000000000000,
	0x405edd2f1a9fbe77, // 123.456
	0x40c81c8000000000, 0x3fdfffffffffffff, 0x3ff0000000000001, 0x3fefffffffffffff,
	0x4022000000000000, 0x4023ffffffffffff, 0x4058ffffffffffff,
	0x3f50624dd2f1a9fc, 0x3f747ae147ae147b, 0x3f847ae147ae147b, 0x3eb0c6f7a0b5ed8d, 0x3ed4f8b588e368f1,
}

var formats = [...]string{
	"%e", "%E", "%f", "%g", "%G",
	"%.0e", "%.1e", "%.3e", "%.15e", "%.16e", "%.17e", "%.18e", "%.30e",
	"%.0f", "%.1f", "%.2f", "%.3f", "%.15f", "%.20f", "%.40f",
	"%.0g", "%.1g", "%.2g", "%.3g", "%.6g", "%.10g", "%.15g", "%.17g", "%.18g", "%.30g",
	"%#e", "%#f", "%#g", "%#.0e", "%#.0f", "%#.0g", "%#.3g", "%#G",
	"%12.4f|", "%-12.4f|", "%012.4f|", "%+f", "% f", "%+012.4e|", "%-+12.3e|", "% 012.3g|", "%-012.3f|",
	"%lf", "%Lf", "%hf", "%le", "%Lg",
	"%.f", "%.e", "%.g", "%1f", "%30e|",
}

func main() {
	ps("== negative zero and empty\n")
	for _, f := range [...]string{"%.3f", "%.0f", "%.1f", "%.0e", "%.1e", "%g", "%.0g", "%.1g", "%+.3f", "%#.0f"} {
		for _, d := range [...]uint64{0xbf3a36e2eb1c432d, 0xbfd999999999999a, 0xbfa47ae147ae147b, 0xbfe0000000000000, 0xc023000000000000} {
			t(f, d)
		}
	}
	w("")
	w("%%")
	w("a")
	ps("== grid\n")
	for fi := range formats {
		for vi := range values {
			t(formats[fi], values[vi])
		}
	}
	ps("== args\n")
	// Which words a double takes after other arguments.
	w("%d|%f", 7, 0x11111111, 0, 0x3ff80000, 9)
	w("%f|%d", 0, 0x3ff80000, 7, 9)
	w("%d|%d|%f", 7, 8, 0, 0x3ff80000, 9)
	w("%f|%f", 0, 0x3ff80000, 0, 0x40040000)
	w("%d|%f|%d|%f", 7, 0x11111111, 0, 0x3ff80000, 8, 0x22222222, 0, 0x40040000)
	w("%s|%f", 0, 0x11111111, 0, 0x3ff80000, 9)
	w("%Lf|%d", 0, 0x3ff80000, 7, 9)
	w("%lld|%f", 7, 0x11111111, 0, 0x3ff80000, 9)
	w("%*f", 12, 0x11111111, 0, 0x3ff80000)
	w("%*.*f", 12, 3, 0, 0x3ff80000)
	w("%.*f", 3, 0x11111111, 0, 0x3ff80000)
	w("%.*f", 0xffffffff, 0x11111111, 0, 0x3ff80000)
	w("%.*e", 0xffffffff, 0x11111111, 0, 0x3ff80000)
	w("%.*g", 0xffffffff, 0x11111111, 0, 0x3ff80000)
	w("%.*f", 0, 0x11111111, 0, 0x3ff80000)
	ps("== sweep\n")
	for e := uint64(0); e < 2048; e += 1 {
		t("%.25e", e<<52)
		t("%.25e", e<<52|0x5a5a5a5a5a5a5)
	}
	ps("== random\n")
	for range 2000 {
		d := uint64(rnd())<<32 | uint64(rnd())
		t("%.25e", d)
	}
	for range 1000 {
		// Ordinary magnitudes.
		d := uint64(1023-100+rnd()%200)<<52 | uint64(rnd()&0xfffff)<<32 | uint64(rnd())
		t("%.25e", d)
	}
	ps("END\n")
}
