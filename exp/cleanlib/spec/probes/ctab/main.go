//go:build yaroze

// Command ctab is a probe for spec 18: gets echoed a tab as spaces on Sony's
// library, up to a stop of 8, so something counts columns. This puts a tab through
// each way out after each count of letters, and after the characters that might
// move the column, and looks at what each call returns. Results follow on a line of
// their own, written with Write, which goes round the library's own count (lines
// made with Write show whether it sees them).
package main

import "go.chrisrx.dev/psx/yaroze"

//export putc
func putc(c, fd int32) int32

//export putchar
func putchar(c int32) int32

//export printf
func printf(f *byte, a1, a2, a3 uint32) int32

//export printf2
func printf2(f *byte, a1, a2, a3 uint32) int32

var (
	line  [256]byte
	nline int
	fbuf  [64]byte
)

func flush() {
	yaroze.Write(1, &line[0], int32(nline))
	nline = 0
}

func ps(s string) {
	for i := 0; i < len(s); i++ {
		line[nline] = s[i]
		nline++
	}
}

func pi(v int32) {
	if v < 0 {
		ps("-")
		v = -v
	}
	if v >= 10 {
		pi(v / 10)
	}
	line[nline] = byte('0' + v%10)
	nline++
}

func z(f string) *byte {
	n := copy(fbuf[:len(fbuf)-1], f)
	fbuf[n] = 0
	return &fbuf[0]
}

// mark starts a case at the first column by the library's own count, and marks
// the end of its output.
func start(what string, n int32) {
	putchar('\n')
	ps("== ")
	ps(what)
	ps(" ")
	pi(n)
	ps("\r\n")
	flush()
}

func result(r int32) {
	putchar('|')
	putchar('\n')
	ps("returned ")
	pi(r)
	ps("\r\n")
	flush()
}

func letters(n int32) {
	for range n {
		putchar('a')
	}
}

func main() {
	for n := range int32(19) {
		start("letters then putchar tab", n)
		letters(n)
		result(putchar('\t'))
	}
	for n := range int32(10) {
		start("letters then putc tab", n)
		letters(n)
		result(putc('\t', 1))
	}
	for n := range int32(10) {
		start("letters then printf %c tab", n)
		letters(n)
		result(printf(z("%c"), '\t', 0, 0))
	}
	for n := range int32(10) {
		start("letters then printf literal tab", n)
		letters(n)
		result(printf(z("\t"), 0, 0, 0))
	}
	for n := range int32(10) {
		start("letters then printf2 literal tab", n)
		letters(n)
		result(printf2(z("\t"), 0, 0, 0))
	}
	for n := range int32(10) {
		start("printf letters then tab", n)
		for range n {
			printf(z("a"), 0, 0, 0)
		}
		result(putchar('\t'))
	}
	for n := range int32(10) {
		start("putc letters then tab", n)
		for range n {
			putc('a', 1)
		}
		result(putchar('\t'))
	}
	for n := range int32(10) {
		start("Write letters then tab", n)
		for range n {
			ps("a")
		}
		flush()
		result(putchar('\t'))
	}
	for _, c := range [...]int32{'\n', '\r', 8, 0x7f, 7, 0, 1, 0x1b, 0x80, 0xff, 0x141, -1, '\t'} {
		for n := range int32(4) {
			start("3 letters, then this", c)
			letters(n)
			letters(3)
			putchar(c)
			ps("n ")
			pi(n)
			ps("\r\n")
			flush()
			result(putchar('\t'))
		}
	}
	for n := range int32(4) {
		start("two tabs after letters", n)
		letters(n)
		putchar('\t')
		result(putchar('\t'))
	}
	start("a long line", 0)
	letters(300)
	result(putchar('\t'))
	start("past a wrap of 16 bits", 0)
	letters(10)
	putchar('\n')
	result(putchar('\t'))
	for c := int32(0); c < 256; c++ {
		if c != '\t' && c != '\n' {
			start("each byte", c)
			putchar(c)
			result(putchar('\t'))
		}
	}
	for _, c := range [...]int32{0x10a, 0x109, -246, -247, 0x20a, 0x120, 0x17e, 0x17f} {
		start("high bits", c)
		letters(3)
		putchar(c)
		result(putchar('\t'))
	}
	start("the count over 65536", 0)
	letters(65536 + 3)
	result(putchar('\t'))
	ps("END\n")
	flush()
	for {
	}
}
