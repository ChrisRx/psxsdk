//go:build yaroze

// Command cprintf is a probe for spec 17: printf and printf2, what they print and
// what they return. Each case prints a line naming the format, the text the call
// makes between brackets, and its return. In the harness's kernel log, the write
// calls between the brackets are the call's. The last group calls sprintf, printf
// and their 2 forms and FntPrint with a null format, which is the probe's end: a
// call that does not return shows there.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export printf
func printf(f *byte, a1, a2, a3, a4, a5, a6, a7 uint32) int32

//export printf2
func printf2(f *byte, a1, a2, a3, a4, a5, a6, a7 uint32) int32

var (
	line  [256]byte
	nline int
)

func flush() {
	yaroze.Write(1, &line[0], int32(nline))
	nline = 0
}

func ps(s string) {
	nline += copy(line[nline:], s)
}

func pi(v int32) {
	if v < 0 {
		ps("-")
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
		line[nline] = d[k]
		nline++
	}
}

var (
	fbuf [64]byte
	s1   = [...]byte{'h', 'e', 'l', 'l', 'o', 0}
	two  bool
)

func z(f string) *byte {
	n := copy(fbuf[:len(fbuf)-1], f)
	fbuf[n] = 0
	return &fbuf[0]
}

func t(f string, a ...uint32) {
	var w [7]uint32
	copy(w[:], a)
	if two {
		ps("printf2 ")
	} else {
		ps("printf ")
	}
	for i := 0; i < len(f); i++ {
		if f[i] == '\n' {
			ps("\\n")
		} else {
			line[nline] = f[i]
			nline++
		}
	}
	ps(" [")
	flush()
	var r int32
	if two {
		r = printf2(z(f), w[0], w[1], w[2], w[3], w[4], w[5], w[6])
	} else {
		r = printf(z(f), w[0], w[1], w[2], w[3], w[4], w[5], w[6])
	}
	ps("] -> ")
	pi(r)
	ps("\n")
	flush()
}

func cases() {
	str := uint32(uintptr(unsafe.Pointer(&s1[0])))
	t("")
	t("a")
	t("plain")
	t("\n")
	t("a\nb")
	t("%%")
	t("a%%b")
	t("%")
	t("a%")
	t("%d", 42)
	t("%d", 0)
	t("%d", 12345)
	t("%d", 0xffffffd6)
	t("a%d", 42)
	t("%db", 42)
	t("a%db", 42)
	t("%d%d", 1, 2)
	t("%d %d", 1, 2)
	t("%x", 0xbeef)
	t("%c", 65)
	t("%c", 0)
	t("%c", 10)
	t("%s", str)
	t("%s", 0)
	t("a%sb", str)
	t("%p", 0xbeef)
	t("%5d", 42)
	t("%-5d|", 42)
	t("%05d", 42)
	t("%+d", 42)
	t("%.3d", 42)
	t("%8s", str)
	t("%.2s", str)
	t("%y", 42)
	t("a%yb", 42)
	t("n=%d x=%x s=%s\n", 42, 0xbeef, str)
	t("abc%n", 0)
	t("%f", 0, 0, 0x3ff80000)
	t("a%fb", 0, 0, 0x3ff80000)
	t("%e", 0, 0, 0x3ff80000)
	t("%g", 0, 0, 0x3ff80000)
	t("%.3f|", 0, 0, 0x3ff80000)
	t("%d|%f|", 7, 0, 0x3ff80000)
	t("%f|%d|", 0, 0, 0x3ff80000, 7)
}

func main() {
	cases()
	two = true
	cases()
	ps("END\n")
	flush()
}
