//go:build yaroze

// Command cjmp is a probe for spec 16: setjmp and longjmp, called from inline
// assembly with every register holding a known value, and a jmp_buf between guard
// words. It prints what each word of the buffer holds, by the register whose value
// it is, and for each register what it held at setjmp's first return and at its
// second: its value before setjmp (A), its value before longjmp (B), or the
// raw value. Then the calls from Go: a nested use and a jump out of a deeper
// function.
package main

import (
	"device"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export setjmp
func setjmp(b uint32) int32

//export longjmp
func longjmp(b uint32, v int32)

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

func addrSetjmp() uint32 {
	return uint32(device.AsmFull(`la {}, setjmp`, nil))
}

func addrLongjmp() uint32 {
	return uint32(device.AsmFull(`la {}, longjmp`, nil))
}

func jmpThunk() {
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
	sw $$0, 544($$27)
	lw $$25, 512($$27)
	lw $$4, 516($$27)
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
	lw $$26, 544($$27)
	nop
	bne $$26, $$0, 2f
	nop
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
	addiu $$26, $$0, 1
	sw $$26, 544($$27)
	lw $$25, 532($$27)
	lw $$4, 536($$27)
	lw $$5, 540($$27)
	lw $$1, 644($$27)
	lw $$2, 648($$27)
	lw $$3, 652($$27)
	lw $$8, 672($$27)
	lw $$9, 676($$27)
	lw $$10, 680($$27)
	lw $$11, 684($$27)
	lw $$12, 688($$27)
	lw $$13, 692($$27)
	lw $$14, 696($$27)
	lw $$15, 700($$27)
	lw $$16, 704($$27)
	lw $$17, 708($$27)
	lw $$18, 712($$27)
	lw $$19, 716($$27)
	lw $$20, 720($$27)
	lw $$21, 724($$27)
	lw $$22, 728($$27)
	lw $$23, 732($$27)
	lw $$24, 736($$27)
	lw $$28, 752($$27)
	lw $$30, 760($$27)
	addiu $$29, $$29, -64
	lui $$26, 0x1f80
	lw $$1, 0x1120($$26)
	nop
	sw $$1, 548($$27)
	jalr $$25
	nop
	lw $$27, 16($$29)
	addiu $$26, $$0, 3
	sw $$26, 544($$27)
	beq $$0, $$0, 3f
	nop
	2:
	lui $$26, 0x1f80
	lw $$1, 0x1120($$26)
	nop
	sw $$1, 552($$27)
	addiu $$26, $$0, 2
	sw $$26, 544($$27)
	sw $$1, 388($$27)
	sw $$2, 392($$27)
	sw $$3, 396($$27)
	sw $$4, 400($$27)
	sw $$5, 404($$27)
	sw $$6, 408($$27)
	sw $$7, 412($$27)
	sw $$8, 416($$27)
	sw $$9, 420($$27)
	sw $$10, 424($$27)
	sw $$11, 428($$27)
	sw $$12, 432($$27)
	sw $$13, 436($$27)
	sw $$14, 440($$27)
	sw $$15, 444($$27)
	sw $$16, 448($$27)
	sw $$17, 452($$27)
	sw $$18, 456($$27)
	sw $$19, 460($$27)
	sw $$20, 464($$27)
	sw $$21, 468($$27)
	sw $$22, 472($$27)
	sw $$23, 476($$27)
	sw $$24, 480($$27)
	sw $$25, 484($$27)
	sw $$28, 496($$27)
	sw $$29, 500($$27)
	sw $$30, 504($$27)
	sw $$31, 508($$27)
	3:
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

// canA and canB are the values each register holds before setjmp and before
// longjmp.
func canA(r int) uint32 { return 0x41000000 | uint32(r)<<16 | 0xa5a4 }
func canB(r int) uint32 { return 0x42000000 | uint32(r)<<16 | 0xb5b4 }

func wrw(off int, v uint32) {
	volatile.StoreUint32((*uint32)(unsafe.Pointer(uintptr(base())+uintptr(off))), v)
}

func rdw(off int) uint32 {
	return volatile.LoadUint32((*uint32)(unsafe.Pointer(uintptr(base()) + uintptr(off))))
}

const (
	buf    = 16
	nwords = 24
)

func class(v uint32, r int, sp uint32) {
	switch v {
	case canA(r):
		putc('A')
		return
	case canB(r):
		putc('B')
		return
	}
	if r == 29 && v == sp {
		ps("spA")
		return
	}
	if r == 29 && v == sp-64 {
		ps("spB")
		return
	}
	// Registers the call may use freely hold what it left, which is the
	// library's business: only their being changed prints.
	if r == 31 {
		ps("ret")
		return
	}
	putc('x')
}

// run calls setjmp and then longjmp with val.
func run(val int32) {
	for i := range 128 {
		wrw(i*4, 0xeeeeeeee)
	}
	for r := range 32 {
		cxw(32+r, canA(r))
		cxw(160+r, canB(r))
	}
	cxw(128, addrSetjmp())
	cxw(129, at(buf))
	cxw(133, addrLongjmp())
	cxw(134, at(buf))
	cxw(135, uint32(val))
	yaroze.EnterCriticalSection()
	jmpThunk()
	yaroze.ExitCriticalSection()
	sp := cxr(64 + 29)
	ra := cxr(64 + 31)
	ps("val=")
	pd(val)
	ps(" state=")
	pd(int32(cxr(136)))
	ps(" first v0=")
	pd(int32(cxr(64 + 2)))
	ps(" second v0=")
	pd(int32(cxr(96 + 2)))
	ps(" longjmp cycles=")
	pd(int32((cxr(138) - cxr(137)) & 0xffff))
	putc('\n')
	ps(" buffer:")
	for i := range nwords {
		v := rdw(buf - 8 + i*4)
		putc(' ')
		pd(int32(i) - 2)
		putc('=')
		switch {
		case v == 0xeeeeeeee:
			ps("guard")
		case v == sp:
			ps("sp")
		case v == ra:
			ps("ra")
		default:
			found := false
			for r := range 32 {
				if v == canA(r) {
					ps(names[r])
					found = true
				}
			}
			if !found {
				hx(v)
			}
		}
	}
	putc('\n')
	ps(" registers:")
	for r := 1; r < 32; r++ {
		if r == 26 || r == 27 {
			continue
		}
		putc(' ')
		ps(names[r])
		putc('=')
		class(cxr(64+r), r, sp)
		putc('/')
		class(cxr(96+r), r, sp)
	}
	putc('\n')
}

// From Go: nothing here depends on registers surviving a second return, as all
// state is in globals.
var (
	state int
	depth int
)

//go:noinline
func deeper(n int, b uint32, v int32) {
	depth++
	if n == 0 {
		longjmp(b, v)
	}
	deeper(n-1, b, v)
	depth--
}

func goCalls() {
	state = 0
	r := setjmp(at(buf))
	state++
	ps("go setjmp ")
	pd(r)
	ps(" pass ")
	pd(int32(state))
	putc('\n')
	if state == 1 {
		deeper(5, at(buf), 9)
	}
	ps("go depth ")
	pd(int32(depth))
	putc('\n')
}

func main() {
	for _, v := range [...]int32{7, 1, 0, -1, 0x12345678} {
		run(v)
	}
	goCalls()
	end()
}
