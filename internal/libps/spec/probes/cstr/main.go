//go:build yaroze

// Command cstr is a probe for spec 16: results of the library's C string
// calls over ordinary inputs and the corners, each destination between guard
// bytes (EE), with a known string at address 0 to tell a read through a null
// pointer from a check for one. strtok's first call of the run, with a null
// string, comes first. The calls with a negative length that may run away are
// in probe cneg.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// strlen takes a pointer, unlike the rest: TinyGo's runtime declares the same
// symbol as reading only memory through its argument, so with an integer the
// compiler reuses an earlier result.
//
//export strlen
func strlenp(s unsafe.Pointer) int32

func strlen(s uint32) int32 { return strlenp(unsafe.Pointer(uintptr(s))) }

//export strcpy
func strcpy(d, s uint32) uint32

//export strncpy
func strncpy(d, s uint32, n int32) uint32

//export strcat
func strcat(d, s uint32) uint32

//export strncat
func strncat(d, s uint32, n int32) uint32

//export strcmp
func strcmp(a, b uint32) int32

//export strncmp
func strncmp(a, b uint32, n int32) int32

//export strchr
func strchr(s uint32, c int32) uint32

//export strrchr
func strrchr(s uint32, c int32) uint32

//export strpbrk
func strpbrk(s, set uint32) uint32

//export strspn
func strspn(s, set uint32) int32

//export strcspn
func strcspn(s, set uint32) int32

//export strstr
func strstr(s, sub uint32) uint32

//export strtok
func strtok(s, set uint32) uint32

//export toupper
func toupper(c int32) int32

//export tolower
func tolower(c int32) int32

// The probe prints a line at a time with Write, which is much faster than
// println. Results that point into the arena print as a+N, so the output is the
// same whichever image runs.
var (
	line  [512]byte
	nline int
	arena [1024]uint32
)

const hexDigits = "0123456789abcdef"

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

func pd(v int32) {
	u := uint32(v)
	if v < 0 {
		putc('-')
		u = -u
	}
	var d [10]byte
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

func hb(b byte) {
	putc(hexDigits[b>>4])
	putc(hexDigits[b&15])
}

func hx(v uint32) {
	for i := range 8 {
		putc(hexDigits[v>>(28-4*i)&15])
	}
}

func base() uint32 { return uint32(uintptr(unsafe.Pointer(&arena[0]))) }

func at(off int) uint32 { return base() + uint32(off) }

// rb and wb read and write the arena. They are volatile since the compiler sees
// only an integer go to the C call and may otherwise treat the arena as untouched
// by it (a store just before strlen was dropped without this).
func rb(off int) byte {
	return volatile.LoadUint8((*uint8)(unsafe.Pointer(uintptr(base()) + uintptr(off))))
}

func wb(off int, v byte) {
	volatile.StoreUint8((*uint8)(unsafe.Pointer(uintptr(base())+uintptr(off))), v)
}

// fill sets the first n bytes of the arena to the guard byte.
func fill(n int) {
	for i := range n {
		wb(i, 0xee)
	}
}

// put places s and a terminator in the arena and returns its address.
func put(off int, s string) uint32 {
	for i := 0; i < len(s); i++ {
		wb(off+i, s[i])
	}
	wb(off+len(s), 0)
	return at(off)
}

// nb reads and sb writes memory at address 0, through its cached mirror.
func nb(i int) byte { return volatile.LoadUint8((*uint8)(yaroze.Pointer(0x80000000 + uintptr(i)))) }

func sb(i int, v byte) { volatile.StoreUint8((*uint8)(yaroze.Pointer(0x80000000+uintptr(i))), v) }

// setNull puts a known string at address 0, so that a call that reads through a
// null pointer can be told from one that checks for it.
func setNull(s string) {
	for i := range 16 {
		sb(i, 0xdd)
	}
	for i := 0; i < len(s); i++ {
		sb(i, s[i])
	}
	sb(len(s), 0)
}

// pr prints a result: 0, an offset in the arena, an offset from address 0, or
// the value. Anything inside the library prints as lib.
func pr(v uint32) {
	b := base()
	switch {
	case v == 0:
		putc('0')
	case v >= b && v < b+4096:
		ps("a+")
		pd(int32(v - b))
	case v >= 0x80010000 && v < 0x80090000:
		ps("lib")
	case v < 16:
		ps("null+")
		pd(int32(v))
	default:
		hx(v)
	}
}

// dump prints n bytes of the arena from off.
func dump(off, n int) {
	for i := range n {
		hb(rb(off + i))
	}
}

func dumpNull() {
	for i := 0; i < 8; i++ {
		hb(nb(i))
	}
}

func end() {
	ps("END\n")
	for {
		yaroze.VSync(0)
	}
}

// The strings the calls read, placed past the 96 bytes each destination may
// touch. a and b hold the two arguments of a two-string call.
const (
	dst  = 8
	srcA = 128
	srcB = 512
	long = 1024
)

func a(s string) uint32 { return put(srcA, s) }
func b(s string) uint32 { return put(srcB, s) }

func wr(name string, ret uint32) {
	ps(name)
	ps(" ret=")
	pr(ret)
	ps(" mem=")
	dump(0, 40)
	ps(" null=")
	dumpNull()
	putc('\n')
}

func in(name string, v int32) {
	ps(name)
	ps(" = ")
	pd(v)
	putc('\n')
}

func ptr(name string, v uint32) {
	ps(name)
	ps(" = ")
	pr(v)
	putc('\n')
}

func reset() {
	fill(srcA)
	setNull("NULLY")
}

func runStrlen() {
	in("strlen empty", strlen(a("")))
	in("strlen a", strlen(a("a")))
	in("strlen abc", strlen(a("abc")))
	in("strlen bit7", strlen(a("\x80\xff\x01")))
	for _, n := range [...]int{1, 2, 3, 4, 5, 15, 16, 255, 256, 300, 1000} {
		for i := range n {
			wb(long+i, 'x')
		}
		wb(long+n, 0)
		ps("strlen long ")
		pd(int32(n))
		in("", strlen(at(long)))
	}
	put(srcA, "abcdefgh")
	in("strlen +1", strlen(at(srcA+1)))
	in("strlen +2", strlen(at(srcA+2)))
	in("strlen +3", strlen(at(srcA+3)))
	setNull("NULLY")
	in("strlen null", strlen(0))
	setNull("")
	in("strlen null, empty there", strlen(0))
}

func runStrcpy() {
	reset()
	wr("strcpy abc", strcpy(at(dst), a("abc")))
	reset()
	wr("strcpy empty", strcpy(at(dst), a("")))
	reset()
	wr("strcpy long", strcpy(at(dst), a("abcdefghijklmnopqrstuvwxyz")))
	reset()
	wr("strcpy d+1", strcpy(at(dst+1), a("abcde")))
	reset()
	wr("strcpy d+2", strcpy(at(dst+2), a("abcde")))
	reset()
	wr("strcpy d+3", strcpy(at(dst+3), a("abcde")))
	reset()
	a("xabcdefg")
	wr("strcpy s+1", strcpy(at(dst), at(srcA+1)))
	reset()
	a("xyabcdefg")
	wr("strcpy s+2", strcpy(at(dst), at(srcA+2)))
	reset()
	a("xyzabcdefg")
	wr("strcpy s+3", strcpy(at(dst), at(srcA+3)))
	reset()
	wr("strcpy bit7", strcpy(at(dst), a("\x80\xff\x7f")))
	// Overlap with the destination before the source.
	reset()
	put(dst+1, "abcdefgh")
	wr("strcpy overlap d=s-1", strcpy(at(dst), at(dst+1)))
	reset()
	put(dst+3, "abcdefgh")
	wr("strcpy overlap d=s-3", strcpy(at(dst), at(dst+3)))
	reset()
	put(dst, "abcdefgh")
	wr("strcpy same", strcpy(at(dst), at(dst)))
	// Null in either argument.
	reset()
	wr("strcpy d=null", strcpy(0, a("abc")))
	reset()
	wr("strcpy s=null", strcpy(at(dst), 0))
	reset()
	wr("strcpy both null", strcpy(0, 0))
}

func runStrncpy() {
	for _, n := range [...]int32{0, 1, 2, 3, 4, 5, 8, 16} {
		reset()
		ps("strncpy abc n=")
		pd(n)
		wr("", strncpy(at(dst), a("abc"), n))
	}
	for _, n := range [...]int32{0, 1, 4} {
		reset()
		ps("strncpy empty n=")
		pd(n)
		wr("", strncpy(at(dst), a(""), n))
	}
	reset()
	wr("strncpy d+1 n=9", strncpy(at(dst+1), a("abcde"), 9))
	reset()
	a("xabcdefg")
	wr("strncpy s+1 n=4", strncpy(at(dst), at(srcA+1), 4))
	reset()
	put(dst+1, "abcdefgh")
	wr("strncpy overlap d=s-1 n=20", strncpy(at(dst), at(dst+1), 20))
	reset()
	put(dst, "abcdefgh")
	wr("strncpy overlap d=s+1 n=5", strncpy(at(dst+1), at(dst), 5))
	reset()
	wr("strncpy d=null n=3", strncpy(0, a("abc"), 3))
	reset()
	wr("strncpy s=null n=3", strncpy(at(dst), 0, 3))
	reset()
	wr("strncpy s=null n=10", strncpy(at(dst), 0, 10))
	reset()
	wr("strncpy both null", strncpy(0, 0, 3))
	reset()
	wr("strncpy d=null n=0", strncpy(0, a("abc"), 0))
}

func runStrcat() {
	reset()
	put(dst, "ab")
	wr("strcat ab cd", strcat(at(dst), a("cd")))
	reset()
	put(dst, "")
	wr("strcat empty cd", strcat(at(dst), a("cd")))
	reset()
	put(dst, "ab")
	wr("strcat ab empty", strcat(at(dst), a("")))
	reset()
	put(dst+1, "ab")
	wr("strcat d+1", strcat(at(dst+1), a("cdefg")))
	reset()
	put(dst, "abc")
	a("xcdefg")
	wr("strcat s+1", strcat(at(dst), at(srcA+1)))
	reset()
	put(dst, "abc")
	put(dst+4, "XY")
	wr("strcat overlap past end", strcat(at(dst), at(dst+4)))
	reset()
	put(dst, "ab")
	wr("strcat d=null", strcat(0, a("cd")))
	reset()
	put(dst, "ab")
	wr("strcat s=null", strcat(at(dst), 0))
	reset()
	wr("strcat both null", strcat(0, 0))
}

func runStrncat() {
	for _, n := range [...]int32{0, 1, 2, 3, 4, 10} {
		reset()
		put(dst, "ab")
		ps("strncat ab cde n=")
		pd(n)
		wr("", strncat(at(dst), a("cde"), n))
	}
	reset()
	put(dst, "")
	wr("strncat empty n=2", strncat(at(dst), a("cde"), 2))
	reset()
	put(dst, "ab")
	wr("strncat empty s n=2", strncat(at(dst), a(""), 2))
	reset()
	put(dst, "ab")
	wr("strncat d=null", strncat(0, a("cde"), 2))
	reset()
	put(dst, "ab")
	wr("strncat s=null", strncat(at(dst), 0, 2))
	reset()
	wr("strncat both null", strncat(0, 0, 2))
	reset()
	put(dst, "ab")
	wr("strncat n=-1", strncat(at(dst), a("cde"), -1))
}

var pairs = [...][2]string{
	{"", ""}, {"", "a"}, {"a", ""}, {"a", "a"}, {"a", "b"}, {"b", "a"}, {"a", "c"}, {"c", "a"},
	{"a", "z"}, {"z", "a"}, {"abc", "abd"}, {"abd", "abc"}, {"abc", "ab"}, {"ab", "abc"},
	{"\x80", "a"}, {"a", "\x80"}, {"\xff", "\x01"}, {"\x01", "\xff"}, {"\xff", "\x7f"}, {"\x80", "\xff"},
	{"\xff", ""}, {"", "\xff"}, {"a\x80", "a\x7f"}, {"A", "a"}, {"a", "A"},
}

func quote(s string) {
	putc('"')
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] >= 0x7f {
			ps("\\x")
			hb(s[i])
		} else {
			putc(s[i])
		}
	}
	putc('"')
}

func runStrcmp() {
	setNull("NULLY")
	for _, p := range pairs {
		ps("strcmp ")
		quote(p[0])
		putc(' ')
		quote(p[1])
		in("", strcmp(a(p[0]), b(p[1])))
	}
	setNull("NULLY")
	in("strcmp null null", strcmp(0, 0))
	in("strcmp null a", strcmp(0, a("a")))
	in("strcmp a null", strcmp(a("a"), 0))
	in("strcmp null NULLY", strcmp(0, a("NULLY")))
	in("strcmp NULLY null", strcmp(a("NULLY"), 0))
	in("strcmp null empty", strcmp(0, a("")))
	in("strcmp empty null", strcmp(a(""), 0))
	a("xabc")
	b("xyabd")
	in("strcmp +1 +2", strcmp(at(srcA+1), at(srcB+2)))
}

func runStrncmp() {
	setNull("NULLY")
	for _, n := range [...]int32{0, 1, 2, 3, 100, -1} {
		for _, p := range pairs {
			ps("strncmp ")
			quote(p[0])
			putc(' ')
			quote(p[1])
			putc(' ')
			pd(n)
			in("", strncmp(a(p[0]), b(p[1]), n))
		}
	}
	// Bytes past the terminators do not count.
	put(srcA, "a\x00b")
	put(srcB, "a\x00c")
	in("strncmp past terminator n=3", strncmp(at(srcA), at(srcB), 3))
	setNull("NULLY")
	in("strncmp null null 3", strncmp(0, 0, 3))
	in("strncmp null a 3", strncmp(0, a("a"), 3))
	in("strncmp a null 3", strncmp(a("a"), 0, 3))
	in("strncmp null a 0", strncmp(0, a("a"), 0))
	in("strncmp a null 0", strncmp(a("a"), 0, 0))
	in("strncmp null null 0", strncmp(0, 0, 0))
	in("strncmp null NULLY 5", strncmp(0, a("NULLY"), 5))
	in("strncmp NULLY null 5", strncmp(a("NULLY"), 0, 5))
}

func runChr() {
	s := "abcabc\x80\xffz"
	for _, c := range [...]int32{'a', 'c', 'z', 'q', 0, 0x80, 0xff, -128, -1, 0x161, 0x100, 0x180} {
		put(srcA, s)
		ps("strchr c=")
		pd(c)
		ptr("", strchr(at(srcA), c))
		ps("strrchr c=")
		pd(c)
		ptr("", strrchr(at(srcA), c))
	}
	ptr("strchr empty a", strchr(a(""), 'a'))
	ptr("strchr empty 0", strchr(a(""), 0))
	ptr("strrchr empty a", strrchr(a(""), 'a'))
	ptr("strrchr empty 0", strrchr(a(""), 0))
	put(srcA, s)
	ptr("strchr +1 a", strchr(at(srcA+1), 'a'))
	ptr("strrchr +7 a", strrchr(at(srcA+7), 'a'))
	setNull("NULLY")
	ptr("strchr null L", strchr(0, 'L'))
	ptr("strchr null q", strchr(0, 'q'))
	ptr("strchr null 0", strchr(0, 0))
	ptr("strrchr null L", strrchr(0, 'L'))
	ptr("strrchr null q", strrchr(0, 'q'))
	ptr("strrchr null 0", strrchr(0, 0))
}

func runSets() {
	type c struct{ s, set string }
	for _, t := range [...]c{
		{"hello", "l"}, {"hello", "xyz"}, {"hello", ""}, {"", "abc"}, {"", ""}, {"hello", "oh"}, {"hello", "he"},
		{"\x80a", "\x80"}, {"a\xff", "\xff"}, {"aaaa", "a"}, {"abab", "ba"},
	} {
		a(t.s)
		b(t.set)
		ps("sets ")
		quote(t.s)
		putc(' ')
		quote(t.set)
		ps(" strpbrk=")
		pr(strpbrk(at(srcA), at(srcB)))
		ps(" strspn=")
		pd(strspn(at(srcA), at(srcB)))
		ps(" strcspn=")
		pd(strcspn(at(srcA), at(srcB)))
		putc('\n')
	}
	setNull("LLY")
	a("hello")
	ps("sets null s, set hello: strpbrk=")
	pr(strpbrk(0, at(srcA)))
	ps(" strspn=")
	pd(strspn(0, at(srcA)))
	ps(" strcspn=")
	pd(strcspn(0, at(srcA)))
	putc('\n')
	ps("sets s hello, null set: strpbrk=")
	pr(strpbrk(at(srcA), 0))
	ps(" strspn=")
	pd(strspn(at(srcA), 0))
	ps(" strcspn=")
	pd(strcspn(at(srcA), 0))
	putc('\n')
	setNull("xLLY")
	a("L")
	ps("sets null s holding xLLY, set L: strpbrk=")
	pr(strpbrk(0, at(srcA)))
	ps(" strspn=")
	pd(strspn(0, at(srcA)))
	ps(" strcspn=")
	pd(strcspn(0, at(srcA)))
	putc('\n')
	setNull("LLY")
	a("LLLYx")
	ps("sets null s, set LLLYx: strpbrk=")
	pr(strpbrk(0, at(srcA)))
	ps(" strspn=")
	pd(strspn(0, at(srcA)))
	ps(" strcspn=")
	pd(strcspn(0, at(srcA)))
	putc('\n')
	ps("sets s LLLYx, null set: strpbrk=")
	pr(strpbrk(at(srcA), 0))
	ps(" strspn=")
	pd(strspn(at(srcA), 0))
	ps(" strcspn=")
	pd(strcspn(at(srcA), 0))
	putc('\n')
	ps("sets null null: strpbrk=")
	pr(strpbrk(0, 0))
	ps(" strspn=")
	pd(strspn(0, 0))
	ps(" strcspn=")
	pd(strcspn(0, 0))
	putc('\n')
}

func runStrstr() {
	type c struct{ s, sub string }
	for _, t := range [...]c{
		{"hello", "ll"}, {"hello", "hello"}, {"hello", "helloo"}, {"hello", ""}, {"", "a"}, {"", ""},
		{"hello", "o"}, {"hello", "x"}, {"aaab", "aab"}, {"ababac", "abac"}, {"abc", "c"}, {"abc", "cd"},
		{"\x80\xff", "\xff"}, {"mississippi", "issip"},
		{"xa", "a"}, {"xya", "a"}, {"xyab", "a"}, {"xyab", "b"}, {"xyab", "ab"}, {"xyab", "yab"}, {"xyab", "xyab"},
		{"hello", "l"}, {"hello", "h"}, {"hello", "e"}, {"hello", "lo"}, {"hello", "llo"}, {"hello", "el"}, {"hello", "he"},
		{"aab", "ab"}, {"aab", "b"}, {"aaa", "a"}, {"aaa", "aa"}, {"aaa", "aaa"}, {"aaa", "aaaa"}, {"abcabd", "abd"},
		{"abcabd", "bd"}, {"abcabd", "d"}, {"xxabxabc", "abc"}, {"xabxyz", "xyz"}, {"abxa", "a"}, {"abxb", "b"},
		{"abcd", "bd"}, {"abcd", "ac"}, {"abcd", "cd"}, {"abcde", "cd"}, {"abcde", "d"}, {"abab", "ab"}, {"abab", "ba"}, {"abab", "bab"},
	} {
		a(t.s)
		b(t.sub)
		ps("strstr ")
		quote(t.s)
		putc(' ')
		quote(t.sub)
		ptr("", strstr(at(srcA), at(srcB)))
	}
	// Strings over two letters, from a fixed sequence, for a rule to meet.
	seed := uint32(0x12345678)
	next := func() uint32 {
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		return seed
	}
	var hay, sub [10]byte
	for range 300 {
		n := int(next() % 9)
		for i := range n {
			hay[i] = 'a' + byte(next()>>8&1)
		}
		m := int(next() % 5)
		for i := range m {
			sub[i] = 'a' + byte(next()>>8&1)
		}
		ps("strstr ")
		h := unsafe.String(&hay[0], n)
		t := unsafe.String(&sub[0], m)
		quote(h)
		putc(' ')
		quote(t)
		ptr("", strstr(a(h), b(t)))
	}
	setNull("NULLY")
	ptr("strstr null LL", strstr(0, a("LL")))
	ptr("strstr null q", strstr(0, a("q")))
	ptr("strstr LL null", strstr(a("xNULLYx"), 0))
	ptr("strstr x null", strstr(a("x"), 0))
	ptr("strstr null null", strstr(0, 0))
	ptr("strstr null empty", strstr(0, a("")))
	ptr("strstr empty null", strstr(a(""), 0))
}

func tok(name string, s, set uint32) {
	ps(name)
	ps(" ret=")
	pr(strtok(s, set))
	ps(" mem=")
	dump(srcA, 12)
	ps(" null=")
	dumpNull()
	putc('\n')
}

func runStrtok() {
	put(srcB, ",;")
	set := at(srcB)
	put(srcA, "a,b;;c,")
	tok("strtok start", at(srcA), set)
	tok("strtok next", 0, set)
	tok("strtok next", 0, set)
	tok("strtok next (end)", 0, set)
	tok("strtok next (past end)", 0, set)
	tok("strtok next (past end)", 0, set)
	put(srcA, ",,a,,b")
	tok("strtok leading", at(srcA), set)
	tok("strtok next", 0, set)
	tok("strtok next (end)", 0, set)
	put(srcA, ",;,")
	tok("strtok all delimiters", at(srcA), set)
	tok("strtok next", 0, set)
	put(srcA, "")
	tok("strtok empty", at(srcA), set)
	tok("strtok next", 0, set)
	put(srcA, "abc")
	tok("strtok no delimiter", at(srcA), set)
	tok("strtok next", 0, set)
	put(srcA, "abc")
	put(srcB+32, "")
	tok("strtok empty set", at(srcA), at(srcB+32))
	tok("strtok next empty set", 0, at(srcB+32))
	// The set may change from call to call.
	put(srcA, "a,b;c,d")
	put(srcB+32, ";")
	tok("strtok set ,;", at(srcA), set)
	tok("strtok set ;", 0, at(srcB+32))
	tok("strtok set ,;", 0, set)
	tok("strtok set ,;", 0, set)
	tok("strtok next", 0, set)
	// bit 7 in both.
	put(srcA, "a\x80b\xffc")
	put(srcB+32, "\x80\xff")
	tok("strtok bit7", at(srcA), at(srcB+32))
	tok("strtok bit7", 0, at(srcB+32))
	tok("strtok bit7", 0, at(srcB+32))
	tok("strtok bit7", 0, at(srcB+32))
	// A null set.
	setNull("NULLY")
	put(srcA, "xLyNz")
	tok("strtok set=null", at(srcA), 0)
	tok("strtok next set=null", 0, 0)
	tok("strtok next set=null", 0, 0)
	// A new string replaces what was left of the last one.
	put(srcA, "a,b")
	tok("strtok restart", at(srcA), set)
	put(srcA+4, "x,y")
	tok("strtok restart 2", at(srcA+4), set)
	tok("strtok next", 0, set)
	tok("strtok next", 0, set)
}

func runCase() {
	ps("toupper 00-ff:\n")
	for i := int32(0); i < 256; i++ {
		hx(uint32(toupper(i)))
		if i%16 == 15 {
			putc('\n')
		} else {
			putc(' ')
		}
	}
	ps("tolower 00-ff:\n")
	for i := int32(0); i < 256; i++ {
		hx(uint32(tolower(i)))
		if i%16 == 15 {
			putc('\n')
		} else {
			putc(' ')
		}
	}
	for _, c := range [...]int32{-1, -128, -159, -191, -97, -65, 0x100, 0x161, 0x141, 0x7fffffff, -2147483648, 0x10061, 0x10041, 0x1000061, 0x41000061} {
		ps("case ")
		pd(c)
		ps(" toupper=")
		pd(toupper(c))
		ps(" tolower=")
		pd(tolower(c))
		putc('\n')
	}
}

func main() {
	// Before any other strtok: a null string with nothing left over from an
	// earlier call.
	setNull(",x,y")
	put(srcB, ",")
	tok("strtok first ever, null", 0, at(srcB))
	tok("strtok second, null", 0, at(srcB))
	runStrlen()
	runStrcpy()
	runStrncpy()
	runStrcat()
	runStrncat()
	runStrcmp()
	runStrncmp()
	runChr()
	runSets()
	runStrstr()
	runStrtok()
	runCase()
	end()
}
