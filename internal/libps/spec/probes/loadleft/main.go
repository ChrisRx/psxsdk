//go:build yaroze

// Command loadleft is a probe for spec 37: what the image calls leave in the argument
// registers on return, called with every register holding a known value. A program
// that passes a function no argument, such as srand(), gets what the last call left in
// a0. Each line prints v0, v1 and a0 on return. Cases are by the size of the image and by
// the interrupt mask the call finds.
package main

import (
	"device"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	entryLoad  = 0x80028374
	entryStore = 0x800283d8
	entryMove  = 0x8002843c
	entryClear = 0x800282e0
)

var iMask = (*uint32)(unsafe.Pointer(uintptr(0x1f801074)))

var (
	line   [256]byte
	nline  int
	cx     [160]uint32
	rect   yaroze.RECT
	pixels [64 * 1024]uint32
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

func reg(name string, r int, sentinel uint32) {
	v := cxr(64 + r)
	ps(" ")
	ps(name)
	ps("=")
	switch v {
	case sentinel:
		ps("K")
	case uint32(uintptr(unsafe.Pointer(&rect))):
		ps("R")
	case uint32(uintptr(unsafe.Pointer(&pixels[0]))):
		ps("P")
	default:
		hx(v)
	}
}

func call(name string, fn, a0, a1, a2, a3 uint32) {
	for r := range 32 {
		cxw(32+r, 0x41000000|uint32(r)<<16|0xa5a4)
	}
	cxw(128, fn)
	cxw(129, a0)
	cxw(130, a1)
	cxw(131, a2)
	cxw(132, a3)
	callThunk()
	ps(name)
	reg("v0", 2, 0x4102a5a4)
	reg("v1", 3, 0x4103a5a4)
	reg("a0", 4, 0x4104a5a4)
	// a1 to a3 aren't printed: spec 37 has what they hold on Sony's, unmatched.
	ps(" mask=")
	hx(volatile.LoadUint32(iMask))
	putc('\n')
}

func rectPtr() uint32 { return uint32(uintptr(unsafe.Pointer(&rect))) }

func pixPtr() uint32 { return uint32(uintptr(unsafe.Pointer(&pixels[0]))) }

func load(name string, x, y, w, h int16) {
	rect = yaroze.RECT{X: x, Y: y, W: w, H: h}
	call(name, entryLoad, rectPtr(), pixPtr(), 0, 0)
	yaroze.DrawSync(0)
}

func main() {
	yaroze.ResetGraph(0)
	load("load 16x1", 0, 0, 16, 1)
	load("load 32x16", 0, 0, 32, 16)
	load("load 33x3", 0, 0, 33, 3)
	load("load 256x240", 0, 0, 256, 240)
	load("load 320x240", 0, 240, 320, 240)
	load("load 1x1", 16, 16, 1, 1)
	load("load 0 wide", 0, 0, 0, 16)
	load("load 0 high", 0, 0, 16, 0)
	load("load negative", 0, 0, -3, 16)

	old := volatile.LoadUint32(iMask)
	for _, m := range [...]uint32{0xd, 9, 1, 0x4d, 0x8d, 0x7ff, 0} {
		volatile.StoreUint32(iMask, m)
		rect = yaroze.RECT{W: 16, H: 1}
		ps("set ")
		hx(m)
		call(" load", entryLoad, rectPtr(), pixPtr(), 0, 0)
		volatile.StoreUint32(iMask, old)
		yaroze.DrawSync(0)
	}

	rect = yaroze.RECT{W: 16, H: 16}
	call("store 16x16", entryStore, rectPtr(), pixPtr(), 0, 0)
	yaroze.DrawSync(0)
	rect = yaroze.RECT{W: 0, H: 16}
	call("store 0 wide", entryStore, rectPtr(), pixPtr(), 0, 0)
	yaroze.DrawSync(0)
	rect = yaroze.RECT{W: 16, H: 16}
	call("move", entryMove, rectPtr(), 64, 64, 0)
	yaroze.DrawSync(0)
	call("clear", entryClear, rectPtr(), 1, 2, 3)
	yaroze.DrawSync(0)
	println("END")
	for {
	}
}
