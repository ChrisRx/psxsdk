//go:build yaroze

// Command cfmtlong is a probe for spec 17: very long output from sprintf and
// sprintf2, null strings over known low memory, and the guard bytes on the
// destination. Each line gives the length returned, how many bytes changed, a hash of
// the text, and its last bytes. The last group is the null format and the null
// destination, one call to a case, as those are the calls most likely to end a run.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export sprintf
func sprintf(s *byte, f *byte, a2, a3, a4, a5 uint32) int32

//export sprintf2
func sprintf2(s *byte, f *byte, a2, a3, a4, a5 uint32) int32

//export printf
func printf(f *byte, a1, a2, a3 uint32) int32

//export printf2
func printf2(f *byte, a1, a2, a3 uint32) int32

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
	case c < 32 || c >= 127:
		ps("\\x")
		putc(hexDigits[c>>4])
		putc(hexDigits[c&15])
	default:
		putc(c)
	}
}

const guard = 0xa5

var (
	fbuf [64]byte
	out  [12000]byte
	long [11000]byte
	two  bool
)

func z(f string) *byte {
	n := copy(fbuf[:len(fbuf)-1], f)
	fbuf[n] = 0
	return &fbuf[0]
}

func addr(b *byte) uint32 { return uint32(uintptr(unsafe.Pointer(b))) }

func t(f string, a0, a1 uint32) {
	for i := range out {
		out[i] = guard
	}
	var r int32
	if two {
		r = sprintf2(&out[0], z(f), a0, a1, 0, 0)
	} else {
		r = sprintf(&out[0], z(f), a0, a1, 0, 0)
	}
	last := -1
	for i := range out {
		if out[i] != guard {
			last = i
		}
	}
	h := uint32(2166136261)
	for i := 0; i <= last; i++ {
		h = (h ^ uint32(out[i])) * 16777619
	}
	ps(f)
	ps(" -> ")
	pi(r)
	ps(" w ")
	pi(int32(last + 1))
	ps(" hash ")
	hex32(h)
	ps(" head ")
	for i := 0; i < 8 && i <= last; i++ {
		esc(out[i])
	}
	ps(" tail ")
	st := last - 8
	if st < 0 {
		st = 0
	}
	for i := st; i <= last; i++ {
		esc(out[i])
	}
	putc('\n')
}

func group() {
	for _, f := range [...]string{
		"%100d", "%255d", "%256d", "%300d", "%511d", "%512d", "%513d", "%1000d", "%1024d", "%2000d", "%4096d", "%8192d", "%10000d",
		"%-1000d", "%01000d", "%.100d", "%.255d", "%.256d", "%.300d", "%.400d", "%.511d", "%.512d", "%.1000d", "%.2000d", "%.8000d", "%2000.1000d",
		"%#.1000x", "%#.1000o", "%+.1000d", "%1000x", "%1000c", "%-1000c", "%1000p", "%1000%",
	} {
		t(f, 42, 0)
	}
	for _, n := range [...]int{100, 255, 256, 300, 512, 1000, 1024, 4096, 10000} {
		long[n] = 0
		t("%s", addr(&long[0]), 0)
		t("%.50s", addr(&long[0]), 0)
		long[n] = 'A' + byte(n%26)
	}
	t("%.5000s%.5000s", addr(&long[0]), addr(&long[0]))
}

func main() {
	for i := range long {
		long[i] = 'A' + byte(i%26)
	}
	long[len(long)-1] = 0
	ps("== sprintf\n")
	group()
	ps("== sprintf2\n")
	two = true
	group()
	for _, f := range [...]string{
		"%.100f", "%.200f", "%.300f", "%.400f", "%.500f", "%.1000f", "%.100e", "%.300e", "%.1000e", "%.100g", "%.300g", "%.1000g",
		"%1000f", "%1000e", "%1000g", "%-1000f", "%01000f", "%#.100g", "%#.1000g",
	} {
		t(f, 0x54442d18, 0x400921fb)
		t(f, 0xffffffff, 0x7fefffff)
	}
	two = false
	ps("== low memory\n")
	low := (*[16]byte)(yaroze.Pointer(0x80000000))
	var save [16]byte
	copy(save[:], low[:])
	copy(low[:], "\x03BCDEFGHIJKLMNO\x00")
	for _, f := range [...]string{"%s", "%.4s", "%.0s", "%#s", "%#.2s", "%8s", "%8.4s", "%-8.4s"} {
		t(f, 0, 0)
		two = true
		t(f, 0, 0)
		two = false
	}
	copy(low[:], save[:])
	ps("printf %s null [")
	yaroze.Write(1, &line[0], int32(nline))
	nline = 0
	pi(printf(z("%s"), 0, 0, 0))
	ps("] printf2 [")
	yaroze.Write(1, &line[0], int32(nline))
	nline = 0
	pi(printf2(z("%s"), 0, 0, 0))
	ps("] printf %.2s [")
	yaroze.Write(1, &line[0], int32(nline))
	nline = 0
	pi(printf(z("%.2s"), 0, 0, 0))
	ps("] printf %8s [")
	yaroze.Write(1, &line[0], int32(nline))
	nline = 0
	pi(printf(z("%8s"), 0, 0, 0))
	putc('\n')
	ps("== null format\n")
	for i := range out[:16] {
		out[i] = guard
	}
	ps("sprintf ")
	pi(sprintf(&out[0], nil, 0, 0, 0, 0))
	ps(" byte ")
	hex32(uint32(out[0]))
	putc('\n')
	ps("sprintf2 ")
	pi(sprintf2(&out[0], nil, 0, 0, 0, 0))
	ps(" byte ")
	hex32(uint32(out[0]))
	putc('\n')
	ps("printf [")
	yaroze.Write(1, &line[0], int32(nline))
	nline = 0
	pi(printf(nil, 0, 0, 0))
	putc('\n')
	ps("printf2 [")
	yaroze.Write(1, &line[0], int32(nline))
	nline = 0
	pi(printf2(nil, 0, 0, 0))
	putc('\n')
	ps("== null destination\n")
	copy(low[:], save[:])
	ps("sprintf ")
	pi(sprintf(nil, z("A"), 0, 0, 0, 0))
	putc('\n')
	copy(low[:], save[:])
	ps("END\n")
}
