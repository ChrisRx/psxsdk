//go:build yaroze

package main

import (
	"device"
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

// flushCache is the resident library's, which empties the instruction
// cache so that code written to RAM is what runs.
//
//export FlushCache
func flushCache()

// run starts the program at pc with gp and sp, and if it returns, starts
// the monitor again from its own startup, on its stack, leaving the
// library as the disc's startup left it, which does not take being
// initialized twice. It does not return.
func run(pc, gp, sp uint32) {
	device.AsmFull(`.set push
	.set noreorder
	.set noat
	move $$gp, {gp}
	move $$sp, {sp}
	jalr {pc}
	nop
	lui $$sp, 0x801f
	ori $$sp, $$sp, 0xff00
	lui $$t0, %hi(_start)
	addiu $$t0, $$t0, %lo(_start)
	jr $$t0
	nop
	.set pop`, map[string]interface{}{"pc": pc, "gp": gp, "sp": sp})
	for {
	}
}

// funcAddr returns the address of f's code. A func value is its context
// and the code's address, and the code takes the context as an extra last
// argument, which the kernel's callers leave as whatever is in the
// register, and which the handlers do not use.
func funcAddr[F any](f F) uint32 {
	return uint32((*[2]uintptr)(unsafe.Pointer(&f))[1])
}

// dispatch is the code the kernel's B-function stub is pointed at, which
// installHook writes. It is given a value so that it is in .data, which the
// monitor's startup leaves alone when it starts again after a program,
// rather than .bss, which it clears.
var dispatch = [64]uint32{1}

// asm writes code, into dispatch or the debugger's, an instruction at a time, with branches to labels
// filled in once their places are known.
type asm struct {
	code   []uint32
	n      int
	labels map[string]int
	fixups []fixup
}

type fixup struct {
	at    int
	label string
}

func (a *asm) emit(ins ...uint32) {
	for _, i := range ins {
		a.code[a.n] = i
		a.n++
	}
}

// branch emits a branch to label, made with op, and its delay slot.
func (a *asm) branch(op func(rs, rt uint32, off int) uint32, rs, rt uint32, label string) {
	a.fixups = append(a.fixups, fixup{a.n, label})
	a.emit(op(rs, rt, 0), nop)
}

func (a *asm) label(name string) { a.labels[name] = a.n }

// jump emits a jump to addr through t0, which the stub uses too.
func (a *asm) jump(addr uint32) {
	a.emit(lui(t0, (addr+0x8000)>>16), addiu(t0, t0, int32(int16(addr))), jr(t0), nop)
}

func (a *asm) resolve() {
	for _, f := range a.fixups {
		a.code[f.at] |= uint32(a.labels[f.label]-f.at-1) & 0xffff
	}
}

// installHook sends what programs print down the cable. A program prints
// through the kernel's B-functions, which it calls through the stub at B0h
// with the function's number in t1: putchar (3Dh), putc (3Bh), puts (3Fh)
// and write (35h) to the standard output, file 1. The stub's four
// instructions jump to the kernel's dispatcher; they are copied into
// dispatch, which sends those four functions to the monitor's own and
// everything else on through the copy, and the stub is replaced with a
// jump to dispatch. Both Sony's BIOS and OpenBIOS write a stub that runs as
// well from anywhere, so the copy jumps where the original did.
func installHook() {
	stub := (*[4]uint32)(yaroze.Pointer(0xb0))
	to := uint32(uintptr(unsafe.Pointer(&dispatch)))
	if volatile.LoadUint32(&stub[0]) == lui(t0, (to+0x8000)>>16) {
		return // in already, from before a program ran
	}
	a := asm{code: dispatch[:], labels: map[string]int{}}
	for _, f := range []struct {
		number int32
		label  string
	}{{0x3d, "putchar"}, {0x3b, "putc"}, {0x3f, "puts"}, {0x35, "write"}} {
		a.emit(addiu(t2, zero, f.number))
		a.branch(beq, t1, t2, f.label)
	}
	a.label("kernel")
	for i := range stub {
		a.emit(volatile.LoadUint32(&stub[i]))
	}
	a.label("putchar")
	a.jump(funcAddr(outPutchar))
	a.label("putc")
	a.jump(funcAddr(outPutc))
	a.label("puts")
	a.jump(funcAddr(outPuts))
	a.label("write")
	a.emit(addiu(t2, zero, 1))
	a.branch(bne, a0, t2, "kernel")
	a.jump(funcAddr(outWrite))
	a.resolve()

	volatile.StoreUint32(&stub[0], lui(t0, (to+0x8000)>>16))
	volatile.StoreUint32(&stub[1], addiu(t0, t0, int32(int16(to))))
	volatile.StoreUint32(&stub[2], jr(t0))
	volatile.StoreUint32(&stub[3], nop)
	flushCache()
}

// vector is the debugger's exception handler, which installVector points
// the kernel's exception vector at, and resumeCode the code that starts
// the stopped program again. Both are in .data, as dispatch is.
var (
	vector     = [80]uint32{1}
	resumeCode = [64]uint32{1}
)

// savedRegs are the general registers the handler saves and the resume code
// loads, all but zero and k0 and k1, which the handler and the kernel's own
// use.
func savedRegs(f func(r uint32)) {
	for r := uint32(1); r < 32; r++ {
		if r != k0 && r != k1 {
			f(r)
		}
	}
}

// installVector puts the debugger between the program and the kernel's
// exception handler, as installHook does for its output: the vector at 80h
// is the same four instructions of a jump, which are copied to the end of
// vector. Interrupts and system calls go on to the kernel. Anything else,
// a break or a fault, stops the program: its registers are saved in
// dbgRegs, and debugStopped runs on dbgStack.
func installVector() {
	stub := (*[4]uint32)(yaroze.Pointer(0x80))
	to := uint32(uintptr(unsafe.Pointer(&vector)))
	if volatile.LoadUint32(&stub[0]) != lui(k0, (to+0x8000)>>16) {
		a := asm{code: vector[:], labels: map[string]int{}}
		a.emit(mfc0(k0, cop0Cause), nop, andi(k0, k0, 0x7c))
		a.branch(beq, k0, zero, "kernel")
		a.emit(addiu(k1, zero, 8<<2))
		a.branch(beq, k0, k1, "kernel")
		regs := uint32(uintptr(unsafe.Pointer(&dbgRegs)))
		a.emit(lui(k0, (regs+0x8000)>>16), addiu(k0, k0, int32(int16(regs))))
		savedRegs(func(r uint32) { a.emit(sw(r, k0, int32(4*r))) })
		a.emit(mfhi(k1), sw(k1, k0, 4*dbgHI), mflo(k1), sw(k1, k0, 4*dbgLO))
		for _, c := range [3][2]uint32{{cop0SR, dbgSR}, {cop0Cause, dbgCause}, {cop0EPC, dbgEPC}} {
			a.emit(mfc0(k1, c[0]), nop, sw(k1, k0, int32(4*c[1])))
		}
		top := uint32(uintptr(unsafe.Pointer(&dbgStack))) + uint32(len(dbgStack))*4 - 16
		a.emit(lui(sp, (top+0x8000)>>16), addiu(sp, sp, int32(int16(top))))
		a.jump(funcAddr(debugStopped))
		a.label("kernel")
		for i := range stub {
			a.emit(volatile.LoadUint32(&stub[i]))
		}
		a.resolve()

		volatile.StoreUint32(&stub[0], lui(k0, (to+0x8000)>>16))
		volatile.StoreUint32(&stub[1], addiu(k0, k0, int32(int16(to))))
		volatile.StoreUint32(&stub[2], jr(k0))
		volatile.StoreUint32(&stub[3], nop)
	}

	// The resume code sets sr first, which has interrupts off until the rfe
	// in the last jump's delay slot, then loads the rest.
	regs := uint32(uintptr(unsafe.Pointer(&dbgRegs)))
	a := asm{code: resumeCode[:]}
	a.emit(lui(k0, (regs+0x8000)>>16), addiu(k0, k0, int32(int16(regs))))
	a.emit(lw(k1, k0, 4*dbgSR), nop, mtc0(k1, cop0SR))
	a.emit(lw(k1, k0, 4*dbgHI), nop, mthi(k1), lw(k1, k0, 4*dbgLO), nop, mtlo(k1))
	savedRegs(func(r uint32) { a.emit(lw(r, k0, int32(4*r))) })
	a.emit(lw(k1, k0, 4*dbgEPC), nop, jr(k1), rfe)
	flushCache()
}

// resume starts the stopped program again, with dbgRegs.
func resume() {
	call1(uint32(uintptr(unsafe.Pointer(&resumeCode))), 0)
}

// currentSR is the status register as it is.
func currentSR() uint32 {
	return uint32(device.AsmFull("mfc0 {}, $$12\n\tnop", nil))
}
