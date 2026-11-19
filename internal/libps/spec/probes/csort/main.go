//go:build yaroze

// Command csort is a probe for spec 16: the compare calls qsort makes, from a Go
// function passed as the C function pointer. Each case is on an input qsort comes
// back from (most inputs stop the run: see spec 16 and probe cqedge, and the
// list here is chosen by the spec's rule). A compare prints as a:b, indexes from
// the start of the array, followed in the first group by the keys as they stand
// at that compare. In the second group elements hold a key, then their starting
// index, then a pattern that must travel with them.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export qsort
func qsort(base uint32, n, size int32, cmp uint32)

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

const arr = 64

// tiny holds arrays of one-letter keys, for the first group.
var tiny = [...]string{
	"1", "21", "123", "231", "321", "111", "211", "122", "221", "1234",
	"1243", "1432", "2314", "2341", "2413", "3214", "3241", "3412", "4213", "4231",
	"4321", "2121", "2222", "1112", "2111",
}

type bigCase struct {
	size, skew int32
	keys       string
}

var big = [...]bigCase{
	{4, 0, "vhmyk"},
	{4, 0, "cbbbc"},
	{4, 0, "ubgezb"},
	{4, 0, "ababab"},
	{4, 0, "tjeqlym"},
	{4, 0, "caabbbc"},
	{4, 0, "utajxzrz"},
	{4, 0, "aaacccba"},
	{4, 0, "krppxdhdl"},
	{4, 0, "caacbbaab"},
	{4, 0, "ztasbgllrh"},
	{4, 0, "bccbbbbcac"},
	{4, 0, "kflkwdnxdkg"},
	{4, 0, "cbaccbabaab"},
	{4, 0, "rjzuhjdkvebi"},
	{4, 0, "aaababbababa"},
	{4, 0, "omtuyqgudmzrp"},
	{4, 0, "bbacbcaaaccab"},
	{4, 0, "tbrawrdfawdgglvv"},
	{4, 0, "bbcbaabcaababacc"},
	{4, 0, "gumypzhxvppcwqrwe"},
	{4, 0, "cacbabcaccbbbbcbc"},
	{4, 0, "vgqmvlqyuakuddmeawdeanyb"},
	{4, 0, "bbbbcccbaabcabbacaaacaca"},
	{4, 0, "sefwkmdemixenrwqxpyjnndptddpdqov"},
	{4, 0, "cacccabcaabcacaabcacaaacccbcbabc"},
	{4, 0, "bahxoilgmwmttlvfkoehahjjxgzfqjsit"},
	{4, 0, "babababccabbbccacbaaccbbbaaacaabb"},
	{4, 0, "zntdhjahwdxihqxubamqgujizmirdxbfcahzjfawxcxcfvcf"},
	{4, 0, "ifzlmsxmcxavyimmoknyjnysflcnisvedmnkvgbypdxwbvmkbbkfkdwmomytcuch"},
	{4, 0, "ponmlkjihgfedcba"},
	{4, 0, "ONMLKJIHGFEDCBA@?>=<;:9876543210"},
	{1, 0, "fwrmhxoge"},
	{1, 0, "cacadcadacaccdcd"},
	{2, 0, "xthmxtmok"},
	{2, 0, "bbdccdbcbccdbadc"},
	{3, 0, "gxbalwgmi"},
	{3, 0, "dccdaadcacddbcbd"},
	{8, 0, "zvzplceta"},
	{8, 0, "ccabdaccccbccbbc"},
	{12, 0, "txamcmfbu"},
	{12, 0, "dadcdccddadcddcb"},
	{16, 0, "cktfcybxz"},
	{16, 0, "cddbdcababaddcdc"},
	{5, 0, "mforpyfpq"},
	{5, 0, "cbbacabcbaacaadb"},
	{4, 1, "uvhhwwmrw"},
	{4, 2, "iqdlfriiv"},
	{2, 1, "epwoxutqy"},
	{8, 3, "fmrolrccd"},
}

var (
	offset int32
	size   int32
	count  int32
	states bool
	large  bool
	column int
)

func keyAt(i int32) byte { return rb(arr + int(offset) + int(i*size)) }

func index(p uint32) int32 {
	return (int32(p) - int32(at(arr)) - offset) / size
}

func cmp(a, b uint32) int32 {
	x := int32(volatile.LoadUint8((*uint8)(yaroze.Pointer(uintptr(a | 0x80000000)))))
	y := int32(volatile.LoadUint8((*uint8)(yaroze.Pointer(uintptr(b | 0x80000000)))))
	pd(index(a))
	putc(':')
	pd(index(b))
	if states {
		putc('=')
		for j := int32(0); j < count; j++ {
			putc(keyAt(j))
		}
		putc('\n')
	} else {
		column++
		if column == 24 {
			column = 0
			putc('\n')
		} else {
			putc(' ')
		}
	}
	d := x - y
	if large {
		if d < 0 {
			return -0x7fffffff
		}
		if d > 0 {
			return 0x7fffffff
		}
	}
	return d
}

func cmpAddr() uint32 {
	f := cmp
	return (*[2]uint32)(unsafe.Pointer(&f))[1]
}

func runBig(c *bigCase, f uint32) {
	size = c.size
	offset = c.skew
	count = int32(len(c.keys))
	base := arr + int(offset)
	for i := int32(0); i < count; i++ {
		p := base + int(i*size)
		wb(p, c.keys[i])
		for j := int32(1); j < size; j++ {
			wb(p+int(j), byte(i)^byte(j-1)<<4)
		}
	}
	wb(base+int(count*size), 0xee)
	wb(base-1, 0xee)
	ps("case size=")
	pd(size)
	ps(" skew=")
	pd(offset)
	ps(" keys=")
	ps(c.keys)
	putc('\n')
	column = 0
	qsort(at(base), count, size, f)
	if column != 0 {
		putc('\n')
	}
	ps("out")
	for i := int32(0); i < count; i++ {
		p := base + int(i*size)
		putc(' ')
		putc(rb(p))
		if size > 1 {
			hb(rb(p + 1))
		}
		// The rest of the element must be as it started.
		for j := int32(2); j < size; j++ {
			if rb(p+int(j)) != rb(p+1)^byte(j-1)<<4 {
				ps("TORN")
			}
		}
	}
	ps(" guards ")
	hb(rb(base - 1))
	hb(rb(base + int(count*size)))
	putc('\n')
}

func main() {
	f := cmpAddr()
	size = 4
	states = true
	for _, c := range tiny {
		count = int32(len(c))
		for j := int32(0); j < count; j++ {
			wb(arr+int(j*4), c[j])
		}
		ps("case ")
		ps(c)
		putc('\n')
		qsort(at(arr), count, 4, f)
		ps("out ")
		for j := int32(0); j < count; j++ {
			putc(keyAt(j))
		}
		putc('\n')
	}
	states = false
	for i := range big {
		runBig(&big[i], f)
	}
	// The same case again with only the sign of the compare's result.
	large = true
	runBig(&big[0], f)
	large = false
	end()
}
