//go:build yaroze

package libps

import (
	"unsafe"
)

// The C library's string, memory and number calls (spec 16). They keep the
// resident library's results where those differ from standard C: its strstr
// is not a correct search, its bsearch can miss, its memset and memmove have
// their own return values. Original programs call them at fixed addresses, so
// they take pointers as integers: C reads address 0 like any other, and a
// null pointer is what each call's own check says it is.

// What Sony's calls take beyond the work done the same way here, in spin's turns
// (or stores, for the Space ones), from probe ccost (spec 16). A figure's call
// costs 14 cycles over its turns.
const (
	strcatCost    = 4
	parseDigit    = 8  // stores a digit: Sony's 48 a digit
	sortBaseCost  = 11 // qsort itself
	sortRangeCost = 9  // each range sort works on
	sortSwapCost  = 2  // each swap
	sortByteSpace = 10 // stores each byte of a swap: Sony's 36 a byte
	sortCmpSpace  = 3  // stores each compare
)

// cBytes is memory from address p on as bytes. A load through a pointer in Go
// carries a nil check, where C reads address 0; a load through a slice, in a
// function marked go:nobounds, is the load alone. Every function that indexes
// the result is so marked. It takes the bound off here too.
//
//go:inline
//go:nobounds
func cBytes(p uintptr) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(p)), 0)[:1<<31-1]
}

// cHalves and cWords are cBytes for 16 and 32-bit values.
//
//go:inline
//go:nobounds
func cHalves(p uintptr) []uint16 {
	return unsafe.Slice((*uint16)(unsafe.Pointer(p)), 0)[:1<<30-1]
}

//go:inline
//go:nobounds
func cWords(p uintptr) []uint32 {
	return unsafe.Slice((*uint32)(unsafe.Pointer(p)), 0)[:1<<29-1]
}

//export libps_c_strlen
//go:nobounds
func libStrlen(s uintptr) int32 {
	if s == 0 {
		return 0
	}
	b := cBytes(s)
	n := 0
	for b[n] != 0 {
		// The store is Sony's time a byte, and keeps the compiler from
		// making a call of the loop: the image has no strlen but this.
		space(1)
		n++
	}
	return int32(n)
}

// strcpy copies s and its terminator forward, a byte at a time.
//
//export strcpy
//go:nobounds
func libStrcpy(d, s uintptr) uintptr {
	if d == 0 || s == 0 {
		return 0
	}
	to, from := cBytes(d), cBytes(s)
	for i := 0; ; i++ {
		c := from[i]
		to[i] = c
		if c == 0 {
			return d
		}
	}
}

// strncpy copies at most n bytes of s, the rest of n being zeros. A string of
// n or longer gets no terminator.
//
//export strncpy
//go:nobounds
func libStrncpy(d, s uintptr, n int32) uintptr {
	if d == 0 || s == 0 {
		return 0
	}
	to, from := cBytes(d), cBytes(s)
	i := int32(0)
	for i < n {
		c := from[i]
		to[i] = c
		i++
		if c == 0 {
			break
		}
	}
	for ; i < n; i++ {
		to[i] = 0
	}
	return d
}

//export strcat
//go:nobounds
func libStrcat(d, s uintptr) uintptr {
	if d == 0 || s == 0 {
		return 0
	}
	to, from := cBytes(d), cBytes(s)
	// Sony's strcat takes 106 cycles, 18 a byte of d and 20 of s (spec 16).
	spin(strcatCost)
	n := 0
	for to[n] != 0 {
		space(4)
		n++
	}
	for i := 0; ; i++ {
		c := from[i]
		to[n+i] = c
		if c == 0 {
			return d
		}
		space(4)
	}
}

// strncat appends at most n bytes of s and always a terminator, with no
// padding. n of 0 or below changes nothing.
//
//export strncat
//go:nobounds
func libStrncat(d, s uintptr, n int32) uintptr {
	if d == 0 || s == 0 {
		return 0
	}
	to, from := cBytes(d), cBytes(s)
	if n <= 0 {
		return d
	}
	k := 0
	for to[k] != 0 {
		space(1)
		k++
	}
	for i := int32(0); i < n; i++ {
		c := from[i]
		if c == 0 {
			break
		}
		to[k] = c
		k++
	}
	to[k] = 0
	return d
}

// cmpNull is strcmp's and strncmp's answer for a null pointer: both null are
// equal, a null a is less and a null b greater.
func cmpNull(a, b uintptr) int32 {
	switch {
	case a == b:
		return 0
	case a == 0:
		return -1
	}
	return 1
}

//export strcmp
//go:nobounds
func libStrcmp(a, b uintptr) int32 {
	if a == 0 || b == 0 {
		return cmpNull(a, b)
	}
	x, y := cBytes(a), cBytes(b)
	for i := 0; ; i++ {
		c, d := x[i], y[i]
		if c != d {
			return int32(c) - int32(d)
		}
		if c == 0 {
			return 0
		}
	}
}

//export strncmp
//go:nobounds
func libStrncmp(a, b uintptr, n int32) int32 {
	if a == 0 || b == 0 {
		return cmpNull(a, b)
	}
	x, y := cBytes(a), cBytes(b)
	for i := int32(0); i < n; i++ {
		c, d := x[i], y[i]
		if c != d {
			return int32(c) - int32(d)
		}
		if c == 0 {
			return 0
		}
	}
	return 0
}

// strchr's c of 0 finds the terminator.
//
//export strchr
//go:nobounds
func libStrchr(s uintptr, c int32) uintptr {
	if s == 0 {
		return 0
	}
	b := cBytes(s)
	want := byte(c)
	for i := 0; ; i++ {
		v := b[i]
		if v == want {
			return s + uintptr(i)
		}
		if v == 0 {
			return 0
		}
	}
}

//export strrchr
//go:nobounds
func libStrrchr(s uintptr, c int32) uintptr {
	if s == 0 {
		return 0
	}
	b := cBytes(s)
	want := byte(c)
	found := uintptr(0)
	for i := 0; ; i++ {
		v := b[i]
		if v == want {
			found = s + uintptr(i)
		}
		if v == 0 {
			return found
		}
	}
}

// inSet reports whether c, not a terminator, is among the bytes of the string
// at set.
//
//go:nobounds
func inSet(set uintptr, c byte) bool {
	b := cBytes(set)
	for i := 0; ; i++ {
		v := b[i]
		if v == 0 {
			return false
		}
		if v == c {
			return true
		}
	}
}

//export strpbrk
//go:nobounds
func libStrpbrk(s, set uintptr) uintptr {
	b := cBytes(s)
	for i := 0; ; i++ {
		c := b[i]
		if c == 0 {
			return 0
		}
		if inSet(set, c) {
			return s + uintptr(i)
		}
	}
}

//export strspn
//go:nobounds
func libStrspn(s, set uintptr) int32 {
	b := cBytes(s)
	n := 0
	for b[n] != 0 && inSet(set, b[n]) {
		n++
	}
	return int32(n)
}

//export strcspn
//go:nobounds
func libStrcspn(s, set uintptr) int32 {
	b := cBytes(s)
	n := 0
	for b[n] != 0 && !inSet(set, b[n]) {
		n++
	}
	return int32(n)
}

// strstr is spec 16's rule: a byte that breaks a partial match is not tried
// again as the first of a new one, and the place returned is only moved at the
// second byte matched, so a one-byte string returns s wherever it is found.
//
//export strstr
//go:nobounds
func libStrstr(s, sub uintptr) uintptr {
	b, want := cBytes(s), cBytes(sub)
	r := s
	k := 0
	for i := 0; ; i++ {
		c := b[i]
		if c == 0 {
			return 0
		}
		if c != want[k] {
			k = 0
			continue
		}
		k++
		if want[k] == 0 {
			return r
		}
		if k == 1 {
			r = s + uintptr(i)
		}
	}
}

// strtokNext is where strtok goes on from: 0 until a call gives it a string.
var strtokNext uintptr

//export strtok
//go:nobounds
func libStrtok(s, set uintptr) uintptr {
	if s != 0 {
		strtokNext = s
	}
	if strtokNext == 0 {
		return 0
	}
	p := strtokNext
	b := cBytes(p)
	i := 0
	for b[i] != 0 && inSet(set, b[i]) {
		i++
	}
	if b[i] == 0 {
		strtokNext = p + uintptr(i)
		return 0
	}
	start := p + uintptr(i)
	for b[i] != 0 && !inSet(set, b[i]) {
		i++
	}
	if b[i] == 0 {
		strtokNext = p + uintptr(i)
	} else {
		b[i] = 0
		strtokNext = p + uintptr(i) + 1
	}
	return start
}

//export toupper
func libToupper(c int32) int32 {
	b := byte(c)
	switch {
	case b >= 'a' && b <= 'z':
		b -= 32
	case highClass(b)&2 != 0:
		b -= 32
	}
	return int32(b)
}

//export tolower
func libTolower(c int32) int32 {
	b := byte(c)
	switch {
	case b >= 'A' && b <= 'Z':
		b += 32
	case highClass(b)&1 != 0:
		b += 32
	}
	return int32(b)
}

// memcpy copies forward by bytes, so it smears the start of s over a d just
// after it. A null d returns 0, and an n of 0 or below d.
//
//export libps_c_memcpy
func libMemcpy(d, s uintptr, n int32) uintptr

// copyForward copies n bytes from s to d a byte at a time, from the start.
//
//go:inline
//go:nobounds
func copyForward(d, s uintptr, n int32) {
	to, from := cBytes(d), cBytes(s)
	for i := int32(0); i < n; i++ {
		to[i] = from[i]
	}
}

// memmove is right on any overlap, and returns d+n for a d below s and d
// otherwise, 3 for memmove(0, s, 3).
//
//export libps_c_memmove
//go:nobounds
func libMemmove(d, s uintptr, n int32) uintptr {
	if n <= 0 {
		return d
	}
	to, from := cBytes(d), cBytes(s)
	if d < s {
		for i := int32(0); i < n; i++ {
			to[i] = from[i]
			space(1)
		}
		return d + uintptr(n)
	}
	// By address: to count an index down takes three more instructions a byte.
	to2, from2 := d+uintptr(n), s+uintptr(n)
	for to2 != d {
		to2--
		from2--
		setByte(to2, byteAt(from2))
		space(1)
	}
	return d
}

// bcopy copies forward by bytes, memset and bzero write n bytes, and memcmp compares a
// byte before it looks at n, so an n of 0 or below compares one. It goes on while n, less
// 1 a byte, is above 0, wrapping, so the most negative n runs on until a difference.
// memset returns d, but 0 for an n of 0 or below as for a null d. All in assembly.
//
//export bcopy
func libBcopy(s, d uintptr, n int32)

//export libps_c_memset
func libMemset(d uintptr, c, n int32) uintptr

//export bzero
func libBzero(d uintptr, n int32)

//export libps_c_memcmp
func libMemcmp(a, b uintptr, n int32) int32

//export bcmp
func libBcmp(a, b uintptr, n int32) int32

//export memchr
//go:nobounds
func libMemchr(s uintptr, c, n int32) uintptr {
	if s == 0 {
		return 0
	}
	b := cBytes(s)
	want := byte(c)
	for i := int32(0); i < n; i++ {
		if b[i] == want {
			return s + uintptr(i)
		}
	}
	return 0
}

// parseInt is the steps strtol, strtoul, atoi and atol share (spec 16): base
// is the argument's, or below 0 for atoi's and atol's own test of a leading
// zero, and signed whether minus signs count. No limit applies: the value wraps
// at 32 bits. A null string returns 0 and leaves the end pointer.
//
//go:nobounds
func parseInt(s, end uintptr, base int32, signed bool) uint32 {
	if s == 0 {
		return 0
	}
	b := cBytes(s)
	i := 0
	for b[i] == ' ' || b[i] >= 9 && b[i] <= 13 {
		i++
	}
	neg := false
	if signed {
		for b[i] == '-' {
			neg = !neg
			i++
		}
	}
	switch {
	case b[i] == '0' && (b[i+1] == 'x' || b[i+1] == 'X'):
		base = 16
		i += 2
	case b[i] == '0' && (b[i+1] == 'b' || b[i+1] == 'B'):
		base = 2
		i += 2
	case base < 0:
		base = 10
		if b[i] == '0' {
			base = 8
		}
	case base < 2 || base > 36:
		base = 10
	}
	var v uint32
	for {
		c := b[i]
		d := int32(99)
		switch {
		case c >= '0' && c <= '9':
			d = int32(c - '0')
		case c >= 'a' && c <= 'z':
			d = int32(c-'a') + 10
		case c >= 'A' && c <= 'Z':
			d = int32(c-'A') + 10
		}
		if d >= base {
			break
		}
		v = v*uint32(base) + uint32(d)
		i++
		space(parseDigit)
	}
	if neg {
		v = -v
	}
	if end != 0 {
		cWords(end)[0] = uint32(s) + uint32(i)
	}
	return v
}

//export strtol
func libStrtol(s, end uintptr, base int32) int32 {
	return int32(parseInt(s, end, max(base, 0), true))
}

//export strtoul
func libStrtoul(s, end uintptr, base int32) uint32 {
	return parseInt(s, end, max(base, 0), false)
}

//export atoi
func libAtoi(s uintptr) int32 { return int32(parseInt(s, 0, -1, true)) }

//export atol
func libAtol(s uintptr) int32 { return int32(parseInt(s, 0, -1, true)) }

// cCompare is the C function at f as a Go func value, as callC does it. The
// context goes in a register the C function ignores.
//
//go:inline
func cCompare(f uintptr) func(a, b uintptr) int32 {
	var fn func(a, b uintptr) int32
	(*[2]uintptr)(unsafe.Pointer(&fn))[1] = f
	return fn
}

// sortSwap exchanges two elements a byte at a time, even an element with itself.
//
//go:nobounds
func sortSwap(a, b, size uintptr) {
	x, y := cBytes(a), cBytes(b)
	for k := uintptr(0); k < size; k++ {
		x[k], y[k] = y[k], x[k]
		space(sortByteSpace)
	}
	spin(sortSwapCost)
}

// sortRange is spec 16's sort, whose exact compares a program can see. Sony's
// calls itself on (lo, i-1) with i at 0 in unsigned arithmetic and does not come
// back from that: here it is an empty range, which sorts those inputs by the same
// rule. The right side, the last thing done, is a loop rather than a call, to keep
// the stack down.
func sortRange(base, size, cmp uintptr, lo, hi uint32) {
	fn := cCompare(cmp)
	for lo < hi {
		spin(sortRangeCost)
		pivot := base + uintptr(hi)*size
		sortSwap(base+uintptr((lo+hi)/2)*size, pivot, size)
		i, j := lo, hi-1
		for {
			for fn(base+uintptr(i)*size, pivot) < 0 {
				space(sortCmpSpace)
				i++
			}
			space(sortCmpSpace)
			for j > i && fn(base+uintptr(j)*size, pivot) > 0 {
				space(sortCmpSpace)
				j--
			}
			if i >= j {
				break
			}
			sortSwap(base+uintptr(i)*size, base+uintptr(j)*size, size)
			i++
			j--
		}
		sortSwap(base+uintptr(i)*size, pivot, size)
		if i > 0 {
			sortRange(base, size, cmp, lo, i-1)
		}
		lo = i + 1
	}
}

//export qsort
func libQsort(base uintptr, n uint32, size, cmp uintptr) {
	if size != 0 {
		spin(sortBaseCost)
	}
	if size != 0 && n != 0 {
		sortRange(base, size, cmp, 0, n-1)
	}
}

// bsearch is spec 16's rule: a compare of 0 does not stop it, it always makes
// one compare, even at n of 0, and it may compare past the end. It can miss a
// key that is there.
//
//export bsearch
func libBsearch(key, base uintptr, n uint32, size, cmp uintptr) uintptr {
	fn := cCompare(cmp)
	p := base
	for {
		r := fn(p, key)
		if n == 0 {
			if r == 0 {
				return p
			}
			return 0
		}
		if r < 0 {
			p += uintptr(n/2+1) * size
			n = (n - 1) / 2
		} else {
			n /= 2
			space(3)
		}
	}
}
