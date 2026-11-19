//go:build yaroze

// Command heapv0 is a probe for spec 18: what realloc(p, 0) leaves in v0 and
// free in v0 and v1, called with every register holding a known value, since heap
// and heaplow saw different results from the same calls. Each line prints what
// v0 and v1 held on return, as "A+N" for the arena, the value written (K) if it is
// the known value it went in with, or the raw word.
package main

import (
	"device"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var (
	initHeap func(p, n uint32)
	malloc   func(n uint32) uint32
)

const (
	entryInit    = 0x80010700
	entryMalloc  = 0x80010754
	entryFree    = 0x80010924
	entryRealloc = 0x8001095c
)

var (
	line  [256]byte
	nline int
	arena [1024]uint32
	cx    [160]uint32
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

func hx(v uint32) {
	for i := range 8 {
		putc("0123456789abcdef"[v>>(28-4*i)&15])
	}
}

func pd(v int32) {
	if v >= 10 {
		pd(v / 10)
	}
	putc(byte('0' + v%10))
}

func base() uint32 { return uint32(uintptr(unsafe.Pointer(&arena[0]))) }

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

// reg prints what a register came back with.
func reg(name string, r int, sentinel uint32) {
	v := cxr(64 + r)
	ps(" ")
	ps(name)
	ps("=")
	switch {
	case v == sentinel:
		ps("K")
	case v >= base() && v <= base()+uint32(len(arena))*4:
		ps("A+")
		pd(int32(v - base()))
	default:
		hx(v)
	}
}

func call(name string, fn, a0, a1, v0 uint32) {
	for r := range 32 {
		cxw(32+r, 0x41000000|uint32(r)<<16|0xa5a4)
	}
	cxw(32+2, v0)
	cxw(128, fn)
	cxw(129, a0)
	cxw(130, a1)
	callThunk()
	ps(name)
	reg("v0", 2, v0)
	reg("v1", 3, 0x4103a5a4)
	reg("a0", 4, 0x4104a5a4)
	putc('\n')
}

func start() {
	initHeap(base(), 4096)
	malloc(16)
	malloc(16)
	malloc(16)
}

func main() {
	(*[2]uintptr)(unsafe.Pointer(&initHeap))[1] = entryInit
	(*[2]uintptr)(unsafe.Pointer(&malloc))[1] = entryMalloc
	for _, v0 := range [...]uint32{0x12345678, 0, 0x80001234} {
		ps("v0 going in ")
		hx(v0)
		putc('\n')
		start()
		call("realloc first, 0", entryRealloc, base()+4, 0, v0)
		start()
		call("realloc middle, 0", entryRealloc, base()+24, 0, v0)
		start()
		call("realloc last, 0", entryRealloc, base()+44, 0, v0)
		call("realloc again, 0", entryRealloc, base()+44, 0, v0)
		start()
		call("free first", entryFree, base()+4, 0, v0)
		call("free first again", entryFree, base()+4, 0, v0)
		call("free(0)", entryFree, 0, 0, v0)
		call("realloc(0, 0)", entryRealloc, 0, 0, v0)
	}
	ps("END\n")
	for {
	}
}
