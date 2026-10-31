//go:build yaroze

package libps

import "unsafe"

// The GTE, as far as the library sets it up: libgte's InitGeom and the
// control registers the 2D library writes. The register moves are in
// targets/playstation-yaroze-libps-gte.S.

//export libps_initGeom
func gteInitGeom()

//export libps_setGeomOffset
func setGeomOffset(x, y int32)

//export libps_setBackColor
func setBackColor(r, g, b int32)

// initGeom patches the kernel's exception handler where it is Sony's and
// enables the GTE with its constants.
//
// libps 0x8002C3C8, 128 bytes
func initGeom() {
	patchGTE()
	gteInitGeom()
}

// sonyHandlerHash is the FNV-1a hash of the first fourteen words of
// Sony's kernel exception handler, entry 6 of the C table at 0xc80: four
// nops, then the saved registers found through the current thread's
// control block, then a call that reads the cause and the return
// address. The versions run so far, 2.2, 3.0 and 4.1, begin alike.
const sonyHandlerHash = 0x4d92813c

// gtePatch is the start of the handler as libgte wants it: the same
// prologue two words earlier, and a read of the cause register into v0
// before the kernel's call, for the kernel's handling of a GTE
// instruction the exception interrupted. It is assembled here.
//
// libps 0x80036094
var gtePatch = [14]uint32{
	nop, nop,
	addiu(rK0, rZero, 0x100),
	lw(rK0, 8, rK0),
	nop,
	lw(rK0, 0, rK0),
	nop,
	addi(rK0, rK0, 8),
	sw(rAT, 4, rK0),
	sw(rV0, 8, rK0),
	sw(rV1, 12, rK0),
	sw(rRA, 0x7c, rK0),
	mfc0(rV0, 13),
	nop,
}

// The MIPS instructions the patch is made of.
const (
	rZero, rAT, rV0, rV1, rK0, rRA = 0, 1, 2, 3, 26, 31

	nop uint32 = 0
)

// immediate encodes an I-type instruction: the opcode, the source and
// target registers and the 16-bit immediate.
func immediate(op, rs, rt uint32, imm int32) uint32 {
	return op<<26 | rs<<21 | rt<<16 | uint32(uint16(imm))
}

func addi(rt, rs uint32, imm int32) uint32      { return immediate(0x08, rs, rt, imm) }
func addiu(rt, rs uint32, imm int32) uint32     { return immediate(0x09, rs, rt, imm) }
func lw(rt uint32, off int32, rs uint32) uint32 { return immediate(0x23, rs, rt, off) }
func sw(rt uint32, off int32, rs uint32) uint32 { return immediate(0x2b, rs, rt, off) }

// mfc0 moves coprocessor 0's register rd to rt.
func mfc0(rt, rd uint32) uint32 { return 0x10<<26 | rt<<16 | rd<<11 }

// fnv1a is the FNV-1a hash of words, byte by byte, little-endian.
func fnv1a(words []uint32) uint32 {
	h := uint32(0x811c9dc5)
	for _, w := range words {
		for i := range 4 {
			h = (h ^ (w >> (8 * i) & 0xff)) * 0x01000193
		}
	}
	return h
}

// patchGTE writes the patch over the start of the kernel's exception
// handler, with interrupts off, and flushes the instruction cache, when
// the handler is Sony's.
//
// The original asked the kernel for the C table (GetC0Table, B 0x56)
// and wrote whatever was there. OpenBIOS, which this project runs by
// default, answers that call by hashing the code that made it, stands
// in for the patches it knows, the original's among them, and stops the
// program on a caller it does not know, which is what the Go library's
// call had been taken for. So the table is read where every Sony kernel
// keeps it, 0x674, and the handler is looked at before anything is
// written: under OpenBIOS the entry is no address, and nothing is.
//
// libps 0x8003602C, 104 bytes
//
//go:noinline
func patchGTE() {
	EnterCriticalSection()
	handler := *(*uint32)(at(0x674 + 24))
	if handler&0x7fe00000 == 0 {
		words := unsafe.Slice((*uint32)(at(uintptr(handler))), len(gtePatch))
		if fnv1a(words) == sonyHandlerHash {
			copy(words, gtePatch[:])
			biosA(0x44, 0, 0, 0, 0)
		}
	}
	ExitCriticalSection()
}

// Keep patchGTE linked for a reader; nothing calls it.
var _ = patchGTE

//export libps_getGeomScreen
func getGeomScreen() int32

//export libps_rotTransPers4
func rotTransPers4(v0, v1, v2, v3 *SVECTOR, sxy0, sxy1, sxy2, sxy3 *uint32, p *uint32, flag *uint32) int32

// The rotation and translation matrix moves the yaroze package's target
// files define, targets/playstation-yaroze-gte.S, shared with it.

//export SetRotMatrix
func setRotMatrix(m *MATRIX)

//export SetTransMatrix
func setTransMatrix(m *MATRIX)
