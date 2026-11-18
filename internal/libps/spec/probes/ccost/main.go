//go:build yaroze

// Command ccost is a probe for spec 16: the cost in CPU cycles of each C library
// call, from root counter 2 in system clock mode read just before the call and
// just after, in inline assembly with interrupts off, less the same reading around
// a call of an empty function. Each figure is the smallest of six runs. A second
// table calls each entry with every register holding a known value and names any
// register the C convention keeps that came back changed.
package main

import (
	"device"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// Each entry's address comes from the assembler, so that no compiler knowledge
// of a function by its name gets in the way.
func addrStrlen() uint32 { return uint32(device.AsmFull(`la {}, strlen`, nil)) }

func addrStrcpy() uint32 { return uint32(device.AsmFull(`la {}, strcpy`, nil)) }

func addrStrncpy() uint32 { return uint32(device.AsmFull(`la {}, strncpy`, nil)) }

func addrStrcat() uint32 { return uint32(device.AsmFull(`la {}, strcat`, nil)) }

func addrStrncat() uint32 { return uint32(device.AsmFull(`la {}, strncat`, nil)) }

func addrStrcmp() uint32 { return uint32(device.AsmFull(`la {}, strcmp`, nil)) }

func addrStrncmp() uint32 { return uint32(device.AsmFull(`la {}, strncmp`, nil)) }

func addrStrchr() uint32 { return uint32(device.AsmFull(`la {}, strchr`, nil)) }

func addrStrrchr() uint32 { return uint32(device.AsmFull(`la {}, strrchr`, nil)) }

func addrStrpbrk() uint32 { return uint32(device.AsmFull(`la {}, strpbrk`, nil)) }

func addrStrspn() uint32 { return uint32(device.AsmFull(`la {}, strspn`, nil)) }

func addrStrcspn() uint32 { return uint32(device.AsmFull(`la {}, strcspn`, nil)) }

func addrStrstr() uint32 { return uint32(device.AsmFull(`la {}, strstr`, nil)) }

func addrStrtok() uint32 { return uint32(device.AsmFull(`la {}, strtok`, nil)) }

func addrMemchr() uint32 { return uint32(device.AsmFull(`la {}, memchr`, nil)) }

func addrMemcpy() uint32 { return uint32(device.AsmFull(`la {}, memcpy`, nil)) }

func addrMemmove() uint32 { return uint32(device.AsmFull(`la {}, memmove`, nil)) }

func addrMemset() uint32 { return uint32(device.AsmFull(`la {}, memset`, nil)) }

func addrMemcmp() uint32 { return uint32(device.AsmFull(`la {}, memcmp`, nil)) }

func addrBcopy() uint32 { return uint32(device.AsmFull(`la {}, bcopy`, nil)) }

func addrBzero() uint32 { return uint32(device.AsmFull(`la {}, bzero`, nil)) }

func addrBcmp() uint32 { return uint32(device.AsmFull(`la {}, bcmp`, nil)) }

func addrToupper() uint32 { return uint32(device.AsmFull(`la {}, toupper`, nil)) }

func addrTolower() uint32 { return uint32(device.AsmFull(`la {}, tolower`, nil)) }

func addrAtoi() uint32 { return uint32(device.AsmFull(`la {}, atoi`, nil)) }

func addrAtol() uint32 { return uint32(device.AsmFull(`la {}, atol`, nil)) }

func addrStrtol() uint32 { return uint32(device.AsmFull(`la {}, strtol`, nil)) }

func addrStrtoul() uint32 { return uint32(device.AsmFull(`la {}, strtoul`, nil)) }

func addrQsort() uint32 { return uint32(device.AsmFull(`la {}, qsort`, nil)) }

func addrBsearch() uint32 { return uint32(device.AsmFull(`la {}, bsearch`, nil)) }

func addrSetjmp() uint32 { return uint32(device.AsmFull(`la {}, setjmp`, nil)) }

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

var cx [256]uint32

func cxr(i int) uint32 { return volatile.LoadUint32(&cx[i]) }

func cxw(i int, v uint32) { volatile.StoreUint32(&cx[i], v) }

func callThunk() {
	ctx := uintptr(unsafe.Pointer(&cx[0]))
	device.AsmFull(`.set push
	.set noreorder
	.set noat
	move $$27, {ctx}
	sw $$1, 4($$27)
	sw $$2, 8($$27)
	sw $$3, 12($$27)
	sw $$4, 16($$27)
	sw $$5, 20($$27)
	sw $$6, 24($$27)
	sw $$7, 28($$27)
	sw $$8, 32($$27)
	sw $$9, 36($$27)
	sw $$10, 40($$27)
	sw $$11, 44($$27)
	sw $$12, 48($$27)
	sw $$13, 52($$27)
	sw $$14, 56($$27)
	sw $$15, 60($$27)
	sw $$16, 64($$27)
	sw $$17, 68($$27)
	sw $$18, 72($$27)
	sw $$19, 76($$27)
	sw $$20, 80($$27)
	sw $$21, 84($$27)
	sw $$22, 88($$27)
	sw $$23, 92($$27)
	sw $$24, 96($$27)
	sw $$25, 100($$27)
	sw $$28, 112($$27)
	sw $$29, 116($$27)
	sw $$30, 120($$27)
	sw $$31, 124($$27)
	addiu $$29, $$29, -32
	sw $$27, 16($$29)
	lw $$25, 512($$27)
	lw $$4, 516($$27)
	lw $$5, 520($$27)
	lw $$6, 524($$27)
	lw $$7, 528($$27)
	lw $$1, 132($$27)
	lw $$2, 136($$27)
	lw $$3, 140($$27)
	lw $$8, 160($$27)
	lw $$9, 164($$27)
	lw $$10, 168($$27)
	lw $$11, 172($$27)
	lw $$12, 176($$27)
	lw $$13, 180($$27)
	lw $$14, 184($$27)
	lw $$15, 188($$27)
	lw $$16, 192($$27)
	lw $$17, 196($$27)
	lw $$18, 200($$27)
	lw $$19, 204($$27)
	lw $$20, 208($$27)
	lw $$21, 212($$27)
	lw $$22, 216($$27)
	lw $$23, 220($$27)
	lw $$24, 224($$27)
	lw $$28, 240($$27)
	lw $$30, 248($$27)
	jalr $$25
	nop
	lw $$27, 16($$29)
	sw $$1, 260($$27)
	sw $$2, 264($$27)
	sw $$3, 268($$27)
	sw $$4, 272($$27)
	sw $$5, 276($$27)
	sw $$6, 280($$27)
	sw $$7, 284($$27)
	sw $$8, 288($$27)
	sw $$9, 292($$27)
	sw $$10, 296($$27)
	sw $$11, 300($$27)
	sw $$12, 304($$27)
	sw $$13, 308($$27)
	sw $$14, 312($$27)
	sw $$15, 316($$27)
	sw $$16, 320($$27)
	sw $$17, 324($$27)
	sw $$18, 328($$27)
	sw $$19, 332($$27)
	sw $$20, 336($$27)
	sw $$21, 340($$27)
	sw $$22, 344($$27)
	sw $$23, 348($$27)
	sw $$24, 352($$27)
	sw $$25, 356($$27)
	sw $$28, 368($$27)
	sw $$29, 372($$27)
	sw $$30, 376($$27)
	sw $$31, 380($$27)
	lw $$1, 4($$27)
	lw $$2, 8($$27)
	lw $$3, 12($$27)
	lw $$4, 16($$27)
	lw $$5, 20($$27)
	lw $$6, 24($$27)
	lw $$7, 28($$27)
	lw $$8, 32($$27)
	lw $$9, 36($$27)
	lw $$10, 40($$27)
	lw $$11, 44($$27)
	lw $$12, 48($$27)
	lw $$13, 52($$27)
	lw $$14, 56($$27)
	lw $$15, 60($$27)
	lw $$16, 64($$27)
	lw $$17, 68($$27)
	lw $$18, 72($$27)
	lw $$19, 76($$27)
	lw $$20, 80($$27)
	lw $$21, 84($$27)
	lw $$22, 88($$27)
	lw $$23, 92($$27)
	lw $$24, 96($$27)
	lw $$25, 100($$27)
	lw $$28, 112($$27)
	lw $$29, 116($$27)
	lw $$30, 120($$27)
	lw $$31, 124($$27)
	.set pop`, map[string]interface{}{"ctx": ctx})
}

func timedThunk() {
	ctx := uintptr(unsafe.Pointer(&cx[0]))
	device.AsmFull(`.set push
	.set noreorder
	.set noat
	move $$27, {ctx}
	sw $$1, 4($$27)
	sw $$2, 8($$27)
	sw $$3, 12($$27)
	sw $$4, 16($$27)
	sw $$5, 20($$27)
	sw $$6, 24($$27)
	sw $$7, 28($$27)
	sw $$8, 32($$27)
	sw $$9, 36($$27)
	sw $$10, 40($$27)
	sw $$11, 44($$27)
	sw $$12, 48($$27)
	sw $$13, 52($$27)
	sw $$14, 56($$27)
	sw $$15, 60($$27)
	sw $$16, 64($$27)
	sw $$17, 68($$27)
	sw $$18, 72($$27)
	sw $$19, 76($$27)
	sw $$20, 80($$27)
	sw $$21, 84($$27)
	sw $$22, 88($$27)
	sw $$23, 92($$27)
	sw $$24, 96($$27)
	sw $$25, 100($$27)
	sw $$28, 112($$27)
	sw $$29, 116($$27)
	sw $$30, 120($$27)
	sw $$31, 124($$27)
	addiu $$29, $$29, -32
	sw $$27, 16($$29)
	lw $$25, 512($$27)
	lw $$4, 516($$27)
	lw $$5, 520($$27)
	lw $$6, 524($$27)
	lw $$7, 528($$27)
	lui $$23, 0x1f80
	lw $$22, 0x1120($$23)
	jalr $$25
	nop
	lw $$3, 0x1120($$23)
	lw $$27, 16($$29)
	sw $$22, 548($$27)
	sw $$3, 552($$27)
	lw $$1, 4($$27)
	lw $$2, 8($$27)
	lw $$3, 12($$27)
	lw $$4, 16($$27)
	lw $$5, 20($$27)
	lw $$6, 24($$27)
	lw $$7, 28($$27)
	lw $$8, 32($$27)
	lw $$9, 36($$27)
	lw $$10, 40($$27)
	lw $$11, 44($$27)
	lw $$12, 48($$27)
	lw $$13, 52($$27)
	lw $$14, 56($$27)
	lw $$15, 60($$27)
	lw $$16, 64($$27)
	lw $$17, 68($$27)
	lw $$18, 72($$27)
	lw $$19, 76($$27)
	lw $$20, 80($$27)
	lw $$21, 84($$27)
	lw $$22, 88($$27)
	lw $$23, 92($$27)
	lw $$24, 96($$27)
	lw $$25, 100($$27)
	lw $$28, 112($$27)
	lw $$29, 116($$27)
	lw $$30, 120($$27)
	lw $$31, 124($$27)
	.set pop`, map[string]interface{}{"ctx": ctx})
}

func emptyFn() {}

func addrEmpty() uint32 {
	f := emptyFn
	return (*[2]uint32)(unsafe.Pointer(&f))[1]
}

func cmpU8(a, b uint32) int32 {
	ncmp++
	return int32(volatile.LoadUint8((*uint8)(unsafe.Pointer(uintptr(a))))) - int32(volatile.LoadUint8((*uint8)(unsafe.Pointer(uintptr(b)))))
}

func addrCmp() uint32 {
	f := cmpU8
	return (*[2]uint32)(unsafe.Pointer(&f))[1]
}

var (
	ncmp  int
	empty uint32
)

func setup() {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801124)), 0)
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801128)), 0xffff)
	yaroze.EnterCriticalSection()
}

func once(fn, a0, a1, a2, a3 uint32) uint32 {
	cxw(128, fn)
	cxw(129, a0)
	cxw(130, a1)
	cxw(131, a2)
	cxw(132, a3)
	timedThunk()
	return (cxr(138) - cxr(137)) & 0xffff
}

// cost runs prep and then the call, six times, and returns the fewest cycles past
// an empty call's.
func cost(prep func(), fn, a0, a1, a2, a3 uint32) int32 {
	best := uint32(0xffffffff)
	for range 6 {
		if prep != nil {
			prep()
		}
		if c := once(fn, a0, a1, a2, a3); c < best {
			best = c
		}
	}
	return int32(best) - int32(empty)
}

const (
	dst = 16
	src = 1024
	sb2 = 1536
	nul = 2048
)

var lens = [...]int{0, 1, 2, 4, 16, 256}

// string of n x's with a terminator.
func xs(off, n int) uint32 {
	for i := range n {
		wb(off+i, 'x')
	}
	wb(off+n, 0)
	return at(off)
}

func row(name string) {
	ps(name)
	ps(":")
}

func num(c int32) {
	putc(' ')
	pd(c)
}

// byLen prints the cost of a call over the lengths, prep making its strings.
func byLen(name string, call func(n int) int32) {
	row(name)
	for _, n := range lens {
		num(call(n))
	}
	putc('\n')
}

// costs measures every call with lengths. In the names: L is the length of the
// string, and the part after the name says what the call is given.
func costs() {
	ps("cycles over lengths 0 1 2 4 16 256\n")
	byLen("strlen", func(n int) int32 { return cost(nil, addrStrlen(), xs(src, n), 0, 0, 0) })
	byLen("strcpy", func(n int) int32 { return cost(nil, addrStrcpy(), at(dst), xs(src, n), 0, 0) })
	byLen("strcpy d+1", func(n int) int32 { return cost(nil, addrStrcpy(), at(dst+1), xs(src, n), 0, 0) })
	byLen("strncpy n=L", func(n int) int32 { return cost(nil, addrStrncpy(), at(dst), xs(src, 300), uint32(n), 0) })
	byLen("strncpy short s, n=L", func(n int) int32 { return cost(nil, addrStrncpy(), at(dst), xs(src, 0), uint32(n), 0) })
	byLen("strncpy s=L n=300", func(n int) int32 { return cost(nil, addrStrncpy(), at(dst), xs(src, n), 300, 0) })
	byLen("strcat d=empty s=L", func(n int) int32 {
		xs(src, n)
		return cost(func() { wb(dst, 0) }, addrStrcat(), at(dst), at(src), 0, 0)
	})
	byLen("strcat d=L s=empty", func(n int) int32 {
		xs(dst, n)
		wb(src, 0)
		return cost(nil, addrStrcat(), at(dst), at(src), 0, 0)
	})
	byLen("strncat d=empty n=L", func(n int) int32 {
		xs(src, 300)
		return cost(func() { wb(dst, 0) }, addrStrncat(), at(dst), at(src), uint32(n), 0)
	})
	byLen("strncat d=empty s=L n=300", func(n int) int32 {
		xs(src, n)
		return cost(func() { wb(dst, 0) }, addrStrncat(), at(dst), at(src), 300, 0)
	})
	byLen("strncat d=L s=empty n=1", func(n int) int32 {
		xs(dst, n)
		wb(src, 0)
		return cost(nil, addrStrncat(), at(dst), at(src), 1, 0)
	})
	byLen("strcmp equal L", func(n int) int32 { return cost(nil, addrStrcmp(), xs(src, n), xs(sb2, n), 0, 0) })
	byLen("strcmp differ at L", func(n int) int32 {
		xs(src, n)
		xs(sb2, n)
		wb(src+n, 'a')
		wb(src+n+1, 0)
		wb(sb2+n, 'b')
		wb(sb2+n+1, 0)
		return cost(nil, addrStrcmp(), at(src), at(sb2), 0, 0)
	})
	byLen("strncmp equal n=L", func(n int) int32 { return cost(nil, addrStrncmp(), xs(src, 300), xs(sb2, 300), uint32(n), 0) })
	byLen("strncmp equal L n=300", func(n int) int32 { return cost(nil, addrStrncmp(), xs(src, n), xs(sb2, n), 300, 0) })
	byLen("strchr miss L", func(n int) int32 { return cost(nil, addrStrchr(), xs(src, n), 'q', 0, 0) })
	byLen("strchr at L", func(n int) int32 {
		xs(src, n+1)
		wb(src+n, 'q')
		return cost(nil, addrStrchr(), at(src), 'q', 0, 0)
	})
	byLen("strrchr miss L", func(n int) int32 { return cost(nil, addrStrrchr(), xs(src, n), 'q', 0, 0) })
	byLen("strrchr hit L", func(n int) int32 { return cost(nil, addrStrrchr(), xs(src, n), 'x', 0, 0) })
	byLen("strpbrk miss L set=1", func(n int) int32 { return cost(nil, addrStrpbrk(), xs(src, n), put(sb2, "q"), 0, 0) })
	byLen("strpbrk miss L set=4", func(n int) int32 { return cost(nil, addrStrpbrk(), xs(src, n), put(sb2, "qrst"), 0, 0) })
	byLen("strpbrk miss s=1 set=L", func(n int) int32 { return cost(nil, addrStrpbrk(), put(src, "q"), xs(sb2, n), 0, 0) })
	byLen("strspn all L set=1", func(n int) int32 { return cost(nil, addrStrspn(), xs(src, n), put(sb2, "x"), 0, 0) })
	byLen("strspn all L set=4", func(n int) int32 { return cost(nil, addrStrspn(), xs(src, n), put(sb2, "qrsx"), 0, 0) })
	byLen("strcspn all L set=1", func(n int) int32 { return cost(nil, addrStrcspn(), xs(src, n), put(sb2, "q"), 0, 0) })
	byLen("strcspn all L set=4", func(n int) int32 { return cost(nil, addrStrcspn(), xs(src, n), put(sb2, "qrst"), 0, 0) })
	byLen("strstr miss L", func(n int) int32 { return cost(nil, addrStrstr(), xs(src, n), put(sb2, "q"), 0, 0) })
	byLen("strstr hit L", func(n int) int32 {
		put(sb2, "qz")
		xs(src, n+2)
		wb(src+n, 'q')
		wb(src+n+1, 'z')
		return cost(nil, addrStrstr(), at(src), at(sb2), 0, 0)
	})
	byLen("strtok token L", func(n int) int32 {
		put(sb2, ",")
		return cost(func() { xs(src, n); wb(src+n, ','); wb(src+n+1, 'y'); wb(src+n+2, 0) }, addrStrtok(), at(src), at(sb2), 0, 0)
	})
	byLen("strtok leading L", func(n int) int32 {
		put(sb2, "x")
		return cost(func() { xs(src, n); wb(src+n, 'y'); wb(src+n+1, 0) }, addrStrtok(), at(src), at(sb2), 0, 0)
	})
	byLen("memchr miss n=L", func(n int) int32 { return cost(nil, addrMemchr(), xs(src, 300), 'q', uint32(n), 0) })
	byLen("memcpy n=L", func(n int) int32 { xs(src, 300); return cost(nil, addrMemcpy(), at(dst), at(src), uint32(n), 0) })
	byLen("memcpy d+1 n=L", func(n int) int32 { xs(src, 300); return cost(nil, addrMemcpy(), at(dst+1), at(src), uint32(n), 0) })
	byLen("memmove d<s n=L", func(n int) int32 { xs(src, 300); return cost(nil, addrMemmove(), at(dst), at(src), uint32(n), 0) })
	byLen("memmove d>s n=L", func(n int) int32 { xs(dst, 300); return cost(nil, addrMemmove(), at(src), at(dst), uint32(n), 0) })
	byLen("memmove d=s n=L", func(n int) int32 { xs(src, 300); return cost(nil, addrMemmove(), at(src), at(src), uint32(n), 0) })
	byLen("memset n=L", func(n int) int32 { return cost(nil, addrMemset(), at(dst), 0x41, uint32(n), 0) })
	byLen("memset d+1 n=L", func(n int) int32 { return cost(nil, addrMemset(), at(dst+1), 0x41, uint32(n), 0) })
	byLen("memcmp equal n=L", func(n int) int32 { return cost(nil, addrMemcmp(), xs(src, 300), xs(sb2, 300), uint32(n), 0) })
	byLen("bcopy n=L", func(n int) int32 { xs(src, 300); return cost(nil, addrBcopy(), at(src), at(dst), uint32(n), 0) })
	byLen("bcopy d>s n=L", func(n int) int32 { xs(dst, 300); return cost(nil, addrBcopy(), at(dst), at(src), uint32(n), 0) })
	byLen("bzero n=L", func(n int) int32 { return cost(nil, addrBzero(), at(dst), uint32(n), 0, 0) })
	byLen("bcmp equal n=L", func(n int) int32 { return cost(nil, addrBcmp(), xs(src, 300), xs(sb2, 300), uint32(n), 0) })
	// Digits, in place of x's.
	digits := func(n int) uint32 {
		for i := range n {
			wb(src+i, '1')
		}
		wb(src+n, 0)
		return at(src)
	}
	byLen("atoi digits L", func(n int) int32 { return cost(nil, addrAtoi(), digits(n), 0, 0, 0) })
	byLen("atol digits L", func(n int) int32 { return cost(nil, addrAtol(), digits(n), 0, 0, 0) })
	byLen("strtol base 10 digits L", func(n int) int32 { return cost(nil, addrStrtol(), digits(n), 0, 10, 0) })
	byLen("strtol base 16 digits L", func(n int) int32 { return cost(nil, addrStrtol(), digits(n), 0, 16, 0) })
	byLen("strtol base 0 digits L", func(n int) int32 { return cost(nil, addrStrtol(), digits(n), 0, 0, 0) })
	byLen("strtol end base 10 digits L", func(n int) int32 { return cost(nil, addrStrtol(), digits(n), at(dst), 10, 0) })
	byLen("strtoul base 10 digits L", func(n int) int32 { return cost(nil, addrStrtoul(), digits(n), 0, 10, 0) })
	byLen("strtoul base 16 digits L", func(n int) int32 { return cost(nil, addrStrtoul(), digits(n), 0, 16, 0) })
	byLen("strtoul end base 10 digits L", func(n int) int32 { return cost(nil, addrStrtoul(), digits(n), at(dst), 10, 0) })
	spaces := func(n int) uint32 {
		for i := range n {
			wb(src+i, ' ')
		}
		wb(src+n, '1')
		wb(src+n+1, 0)
		return at(src)
	}
	byLen("atoi spaces L", func(n int) int32 { return cost(nil, addrAtoi(), spaces(n), 0, 0, 0) })
	byLen("strtol spaces L", func(n int) int32 { return cost(nil, addrStrtol(), spaces(n), 0, 10, 0) })
	row("toupper a,A,0,ff,-1")
	for _, c := range [...]uint32{'a', 'A', '0', 0xff, 0xffffffff, 0xa5, 0xc7} {
		num(cost(nil, addrToupper(), c, 0, 0, 0))
	}
	putc('\n')
	row("tolower a,A,0,ff,-1")
	for _, c := range [...]uint32{'a', 'A', '0', 0xff, 0xffffffff, 0xa5, 0xc7} {
		num(cost(nil, addrTolower(), c, 0, 0, 0))
	}
	putc('\n')
	row("setjmp")
	num(cost(nil, addrSetjmp(), at(dst), 0, 0, 0))
	putc('\n')
	// Null arguments, where the call returns at once.
	row("null strlen strcpy strcat strcmp strchr memcpy")
	num(cost(nil, addrStrlen(), 0, 0, 0, 0))
	num(cost(nil, addrStrcpy(), 0, at(src), 0, 0))
	num(cost(nil, addrStrcat(), 0, at(src), 0, 0))
	num(cost(nil, addrStrcmp(), 0, 0, 0, 0))
	num(cost(nil, addrStrchr(), 0, 'x', 0, 0))
	num(cost(nil, addrMemcpy(), 0, at(src), 4, 0))
	putc('\n')
	// qsort and bsearch, with a short compare: its cost, then each run's with
	// the number of compares.
	row("compare alone")
	xs(src, 4)
	num(cost(nil, addrCmp(), at(src), at(src+1), 0, 0))
	putc('\n')
	// Descending keys, an input qsort comes back from (spec 16).
	for _, n := range [...]int{1, 2, 3, 4, 5, 8, 16, 32} {
		prep := func() {
			for i := range n {
				wb(src+i*4, byte(n-i))
			}
			ncmp = 0
		}
		ps("qsort desc n=")
		pd(int32(n))
		num(cost(prep, addrQsort(), at(src), uint32(n), 4, addrCmp()))
		ps(" compares ")
		pd(int32(ncmp))
		putc('\n')
	}
	// Other element sizes, over the same descending keys.
	for _, sz := range [...]int{1, 2, 8, 12, 16} {
		prep := func() {
			for i := range 8 {
				wb(src+i*sz, byte(8-i))
			}
			ncmp = 0
		}
		ps("qsort desc n=8 size=")
		pd(int32(sz))
		num(cost(prep, addrQsort(), at(src), 8, uint32(sz), addrCmp()))
		ps(" compares ")
		pd(int32(ncmp))
		putc('\n')
	}
	for _, key := range [...]byte{0, 255} {
		for _, n := range [...]int{0, 1, 2, 4, 8, 16, 32, 64, 128} {
			for i := range 129 {
				wb(src+i*4, byte(i%127*2+1))
			}
			wb(dst, key)
			ncmp = 0
			ps("bsearch key=")
			pd(int32(key))
			ps(" n=")
			pd(int32(n))
			cxw(139, addrCmp())
			num(costBsearch(n))
			ps(" compares ")
			pd(int32(ncmp))
			putc('\n')
		}
	}
}

// bsearchCmp is what the timed thunk's stack slot should hold: see costBsearch.
func costBsearch(n int) int32 {
	best := uint32(0xffffffff)
	for range 6 {
		ncmp = 0
		cxw(128, addrBsearch())
		cxw(129, at(dst))
		cxw(130, at(src))
		cxw(131, uint32(n))
		cxw(132, 4)
		// The thunk's frame is 32 bytes and the fifth argument sits at 16(sp),
		// where the thunk keeps its context, so the thunk with the stack slot
		// of its own is used.
		bsThunk()
		if c := (cxr(138) - cxr(137)) & 0xffff; c < best {
			best = c
		}
	}
	return int32(best) - int32(empty)
}

func bsThunk() {
	ctx := uintptr(unsafe.Pointer(&cx[0]))
	device.AsmFull(`.set push
	.set noreorder
	.set noat
	move $$27, {ctx}
	sw $$1, 4($$27)
	sw $$2, 8($$27)
	sw $$3, 12($$27)
	sw $$4, 16($$27)
	sw $$5, 20($$27)
	sw $$6, 24($$27)
	sw $$7, 28($$27)
	sw $$8, 32($$27)
	sw $$9, 36($$27)
	sw $$10, 40($$27)
	sw $$11, 44($$27)
	sw $$12, 48($$27)
	sw $$13, 52($$27)
	sw $$14, 56($$27)
	sw $$15, 60($$27)
	sw $$16, 64($$27)
	sw $$17, 68($$27)
	sw $$18, 72($$27)
	sw $$19, 76($$27)
	sw $$20, 80($$27)
	sw $$21, 84($$27)
	sw $$22, 88($$27)
	sw $$23, 92($$27)
	sw $$24, 96($$27)
	sw $$25, 100($$27)
	sw $$28, 112($$27)
	sw $$29, 116($$27)
	sw $$30, 120($$27)
	sw $$31, 124($$27)
	addiu $$29, $$29, -32
	lw $$1, 556($$27)
	sw $$27, 24($$29)
	sw $$1, 16($$29)
	lw $$25, 512($$27)
	lw $$4, 516($$27)
	lw $$5, 520($$27)
	lw $$6, 524($$27)
	lw $$7, 528($$27)
	lui $$23, 0x1f80
	lw $$22, 0x1120($$23)
	jalr $$25
	nop
	lw $$3, 0x1120($$23)
	lw $$27, 24($$29)
	sw $$22, 548($$27)
	sw $$3, 552($$27)
	lw $$1, 4($$27)
	lw $$2, 8($$27)
	lw $$3, 12($$27)
	lw $$4, 16($$27)
	lw $$5, 20($$27)
	lw $$6, 24($$27)
	lw $$7, 28($$27)
	lw $$8, 32($$27)
	lw $$9, 36($$27)
	lw $$10, 40($$27)
	lw $$11, 44($$27)
	lw $$12, 48($$27)
	lw $$13, 52($$27)
	lw $$14, 56($$27)
	lw $$15, 60($$27)
	lw $$16, 64($$27)
	lw $$17, 68($$27)
	lw $$18, 72($$27)
	lw $$19, 76($$27)
	lw $$20, 80($$27)
	lw $$21, 84($$27)
	lw $$22, 88($$27)
	lw $$23, 92($$27)
	lw $$24, 96($$27)
	lw $$25, 100($$27)
	lw $$28, 112($$27)
	lw $$29, 116($$27)
	lw $$30, 120($$27)
	lw $$31, 124($$27)
	.set pop`, map[string]interface{}{"ctx": ctx})
}

var names = [32]string{
	"zero", "at", "v0", "v1", "a0", "a1", "a2", "a3", "t0", "t1", "t2", "t3", "t4", "t5", "t6", "t7",
	"s0", "s1", "s2", "s3", "s4", "s5", "s6", "s7", "t8", "t9", "k0", "k1", "gp", "sp", "fp", "ra",
}

// kept calls an entry with known values in every register and lists the registers
// the C convention keeps that came back different.
func kept(name string, fn, a0, a1, a2, a3 uint32) {
	for r := range 32 {
		cxw(32+r, 0x41000000|uint32(r)<<16|0xa5a4)
	}
	cxw(128, fn)
	cxw(129, a0)
	cxw(130, a1)
	cxw(131, a2)
	cxw(132, a3)
	callThunk()
	ps("kept ")
	ps(name)
	ps(":")
	for _, r := range [...]int{16, 17, 18, 19, 20, 21, 22, 23, 28, 30} {
		if cxr(64+r) != cxr(32+r) {
			putc(' ')
			ps(names[r])
			putc('=')
			hx(cxr(64 + r))
		}
	}
	// The rest are the call's to use, and only change is noted, by bit.
	putc(' ')
	ps("changed")
	for _, r := range [...]int{1, 3, 8, 9, 10, 11, 12, 13, 14, 15, 24} {
		if cxr(64+r) != cxr(32+r) {
			putc(' ')
			ps(names[r])
		}
	}
	putc('\n')
}

func runKept() {
	xs(src, 8)
	put(sb2, "x")
	kept("strlen", addrStrlen(), at(src), 0, 0, 0)
	kept("strcpy", addrStrcpy(), at(dst), at(src), 0, 0)
	kept("strncpy", addrStrncpy(), at(dst), at(src), 20, 0)
	wb(dst, 0)
	kept("strcat", addrStrcat(), at(dst), at(src), 0, 0)
	wb(dst, 0)
	kept("strncat", addrStrncat(), at(dst), at(src), 4, 0)
	kept("strcmp", addrStrcmp(), at(src), at(sb2), 0, 0)
	kept("strncmp", addrStrncmp(), at(src), at(sb2), 4, 0)
	kept("strchr", addrStrchr(), at(src), 'x', 0, 0)
	kept("strrchr", addrStrrchr(), at(src), 'x', 0, 0)
	kept("strpbrk", addrStrpbrk(), at(src), at(sb2), 0, 0)
	kept("strspn", addrStrspn(), at(src), at(sb2), 0, 0)
	kept("strcspn", addrStrcspn(), at(src), at(sb2), 0, 0)
	kept("strstr", addrStrstr(), at(src), at(sb2), 0, 0)
	kept("strtok", addrStrtok(), at(src), at(sb2), 0, 0)
	kept("memchr", addrMemchr(), at(src), 'x', 8, 0)
	kept("memcpy", addrMemcpy(), at(dst), at(src), 8, 0)
	kept("memmove", addrMemmove(), at(dst), at(src), 8, 0)
	kept("memset", addrMemset(), at(dst), 1, 8, 0)
	kept("memcmp", addrMemcmp(), at(src), at(sb2), 8, 0)
	kept("bcopy", addrBcopy(), at(src), at(dst), 8, 0)
	kept("bzero", addrBzero(), at(dst), 8, 0, 0)
	kept("bcmp", addrBcmp(), at(src), at(sb2), 8, 0)
	kept("toupper", addrToupper(), 'a', 0, 0, 0)
	kept("tolower", addrTolower(), 'A', 0, 0, 0)
	put(src, "123")
	kept("atoi", addrAtoi(), at(src), 0, 0, 0)
	kept("atol", addrAtol(), at(src), 0, 0, 0)
	kept("strtol", addrStrtol(), at(src), 0, 10, 0)
	kept("strtoul", addrStrtoul(), at(src), 0, 10, 0)
	wb(src, 2)
	wb(src+4, 1)
	kept("qsort", addrQsort(), at(src), 2, 4, addrCmp())
	kept("setjmp", addrSetjmp(), at(dst), 0, 0, 0)
}

func main() {
	setup()
	empty = 0
	empty = uint32(cost(nil, addrEmpty(), 0, 0, 0, 0))
	yaroze.ExitCriticalSection()
	ps("empty call ")
	pd(int32(empty))
	putc('\n')
	yaroze.EnterCriticalSection()
	costs()
	runKept()
	yaroze.ExitCriticalSection()
	end()
}
