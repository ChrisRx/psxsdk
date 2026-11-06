//go:build yaroze && fidelity

package cleanlib

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// Everything that exists only to match Sony's time or leftover memory goes
// through the operations in this file, on the resident image and the build
// compared with a reference (spec 30). nofidelity.go has the same operations
// empty, for a program's static build, so that the compiler removes them.
// They spend time and leave memory, and a result never depends on them.

// spinCount is what spin counts on, so that the loop is not optimized
// away.
var spinCount uint32

// spinCycles busy-waits for about c CPU cycles, as the emulator counts
// them, longer if interrupts come in between.
func spinCycles(c uint32) { spin(c / spinTurn) }

// spinTurn is the cycles one turn of spin's loop takes in the emulator: a
// run of 512 turns spans 6,132 cycles in a trace of spec/probes/drawq
// (FntOpen's spin).
const spinTurn = 12

// spin busy-waits for about n of its loops, spinTurn cycles each.
//
//go:noinline
func spin(n uint32) {
	for range n {
		volatile.StoreUint32(&spinCount, volatile.LoadUint32(&spinCount)+1)
	}
}

// spinCall is what a call of spin costs beyond its turns, measured on
// spec/probes/drawq: a spin(1) at a call's end lengthens it by 26 cycles
// and a spin(2) by 38.
const spinCall = 14

// pace busy-waits so that the call takes about c cycles, the call itself
// included: whole turns of spin, so c less spinCall should be a multiple
// of spinTurn. c is a constant at every call and pace is inlined, so the
// turns are computed by the compiler. The library's calls end in one,
// sized so that each takes as long as Sony's (spec 08, "Pacing to Sony's
// times"); an interrupt in between lengthens it as it would Sony's.
//
//go:inline
func pace(c uint32) { spin((c - spinCall) / spinTurn) }

// spacer is the word the short spacers store to: a store to memory costs
// 2 cycles in the emulator, which spaces register accesses more finely
// than spin's turns.
var spacer uint32

// space spends 2 cycles a store on n stores to spacer. n is a constant at
// every call, so that the loop unrolls into plain stores.
func space(n int) {
	for range n {
		volatile.StoreUint32(&spacer, 0)
	}
}

// spaceWith is space storing v, a value live on its path: when two paths
// pad with stores of the same value the compiler merges their tails, and
// then a change to one path's count moves the other's time.
func spaceWith(n int, v uint32) {
	for range n {
		volatile.StoreUint32(&spacer, v)
	}
}

// spacer2 is a second word for a pad that ends where another path's pad
// does: the compiler sinks pads that store to the same word into one block
// after the paths join, feeding the stores with whichever value each path
// had, and then their counts move together. A pad that stores another
// value to another word stays where it is.
var spacer2 uint32

// spaceApart is spaceWith storing v to spacer2 (see spacer2).
func spaceApart(n int, v uint32) {
	for range n {
		volatile.StoreUint32(&spacer2, v)
	}
}

// spaceAt spends one store, to a word of the caller's that is at hand: its
// address costs nothing to load, where spacer's costs 2 more.
//
//go:inline
func spaceAt(p *uint32) { volatile.StoreUint32(p, 0) }

// delay spends n instructions and the call. In assembly, for what a Go call needs to
// spend exactly.
//
//export libps_delay
func delay(n uint32)

// The flush's working memory takes the stack as Sony's does (see kanjiFrame).
const (
	kanjiFrameBytes = 65536

	// kanjiFrameExtra is what the assembly takes off the stack besides the
	// frame: the callee's argument home at the bottom and, above the frame,
	// what brings the image to Sony's depth below the caller's sp.
	kanjiFrameExtra = 16 + 40
)

// What Krom2Tim and Krom2Tim2 leave below the caller's sp, from a pattern laid
// there by a probe (spec 30). Sony's frame is 73,872 bytes. Every call, one that
// fails too, clears 73,728 of them, from 73,784 below the caller's sp to 56 below
// it, at 8 cycles a byte. From the bottom of that, each cell has 32 bytes for the
// 30 ROM bytes of its glyph, 16 cells to a row, for the 16 rows, and then at 65,592
// below sp a successful call has the image, in rows of 16 cells however wide the
// TIM is. The rest of the frame stays as the stack had it, but for what its calls
// leave at the very bottom.
const (
	kromClearLow = 73784
	kromImageAt  = 65592
)

// clearKromStack clears the memory below the stack as Sony's frame does, frame
// being how far the calling function's sp is below its caller's. It stops at sp:
// the few bytes between that and Sony's top are the caller's own.
//
//go:inline
func clearKromStack(frame uint32) { libps_krom_clear(kromClearLow - frame) }

// leaveKromGlyph leaves the 30 ROM bytes of the glyph at addr where Sony's frame
// has them for the cell at x, y, or spends the time of it for an addr of 0.
//
//go:inline
func leaveKromGlyph(frame uint32, x, y int, addr uintptr) {
	libps_krom_glyph(int32(frame)-kromClearLow, x, y, addr)
}

// leaveKromGlyphs leaves the glyphs of the string at s up to the character that
// made Krom2Tim fail.
func leaveKromGlyphs(frame uint32, s *byte) {
	var at kanjiCursor
	for c, p := nextKanji(unsafe.Pointer(s)); c != 0; c, p = nextKanji(p) {
		if c != '\n' {
			addr, kind := glyphFor(c, false)
			if kind != glyphROM && kind != glyphBlank {
				return
			}
			leaveKromGlyph(frame, at.x, at.y, addr)
		}
		at = at.past(c)
	}
}

// leaveKromImage leaves the pixels of the TIM at t where Sony's frame has the
// image, by copying them.
//
//go:inline
func leaveKromImage(frame uint32, t *uint32, shape kromShape) {
	pix := unsafe.Add(unsafe.Pointer(t), 16*4)
	libps_krom_image(int32(frame)-kromImageAt, pix, shape.cols*8, shape.rows*kanjiCell)
}

//export libps_krom_clear
func libps_krom_clear(n uint32)

//export libps_krom_glyph
func libps_krom_glyph(at int32, x, y int, addr uintptr)

//export libps_krom_image
func libps_krom_image(at int32, pix unsafe.Pointer, stride, lines uint32)
