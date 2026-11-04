//go:build yaroze

// Command seqexpr is a probe for the spec, on the bank and SEQs gen.go
// makes (assets/PROBE3.VH and PROBE3.VB: programs 0 and 1 with one tone
// each, program 2 with eight tones told apart by their centers, and four
// looping VAGs of 80, 144, 208 and 272 bytes, so that each VAG has a start
// address of its own). Three parts, each printing "case" lines over the
// TTY: a case plays a sequence and logs, frame by frame from SsSeqPlay,
// every voice whose registers (volumes, pitch, start address, ADSR)
// changed, with the envelope's state at that frame.
//
//   - expr N: controller 11. Where the expression lives and how long:
//     the next note on the channel, a key on by SsUtKeyOn while the
//     sequence plays, the same sequence played again, looped, and closed
//     and opened again, another sequence after it, the other channel's
//     note, controller 7 before and after it, and which header bytes the
//     controllers change (the header is put back after each case, except
//     across the SsUtKeyOn cases that follow the plays).
//   - vag N: the sequencer's start address for tones whose VAG index is
//     0, past the bank's count, negative or with a high byte, read from
//     the eight voices a note on program 2 keys (the pitch names the tone)
//     or from its tone 0 alone, with the halves past the VAGs' marked, and
//     SsUtKeyOn's result and start address for the same indices.
//   - tempo: SsSetTempo(id, 0, 0) on a playing sequence, with a line
//     printed before and after the call, last since Sony's library stops
//     at a break there.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

const vhSize = 4832

var (
	vhData    [(vhSize + 2047) / 2048 * 512]uint32
	vbData    [(704 + 2047) / 2048 * 512]uint32
	exprData  [512]uint32
	afterData [512]uint32
	plainData [512]uint32
	chData    [512]uint32
	c7Data    [512]uint32
	c7aData   [512]uint32
	c7bData   [512]uint32
	p2Data    [512]uint32

	vh    = disc.Register("ASSETS/PROBE3.VH", "assets/PROBE3.VH").Into(vhData[:])
	vb    = disc.Register("ASSETS/PROBE3.VB", "assets/PROBE3.VB").Into(vbData[:])
	expr  = disc.Register("ASSETS/EXPR.SEQ", "assets/EXPR.SEQ").Into(exprData[:])
	after = disc.Register("ASSETS/AFTER.SEQ", "assets/AFTER.SEQ").Into(afterData[:])
	plain = disc.Register("ASSETS/PLAIN.SEQ", "assets/PLAIN.SEQ").Into(plainData[:])
	chSeq = disc.Register("ASSETS/CH.SEQ", "assets/CH.SEQ").Into(chData[:])
	c7    = disc.Register("ASSETS/C7.SEQ", "assets/C7.SEQ").Into(c7Data[:])
	c7a   = disc.Register("ASSETS/C7A.SEQ", "assets/C7A.SEQ").Into(c7aData[:])
	c7b   = disc.Register("ASSETS/C7B.SEQ", "assets/C7B.SEQ").Into(c7bData[:])
	p2    = disc.Register("ASSETS/P2.SEQ", "assets/P2.SEQ").Into(p2Data[:])

	// saved is the header as SsVabTransfer left it, put back after each
	// case.
	saved [vhSize]byte
	vab   int16

	seqExpr, seqAfter, seqPlain, seqCH, seqC7, seqC7A, seqC7B, seqP2 int16
)

// Header offsets: program entries are 16 bytes from 20h, tone rows 32
// bytes from 820h (row 16·slot + t for the program's tone t, the slot
// being the program's index among the programs with tones: 0, 1, 2 here).
// A row's VAG index is its half at 22.
const (
	entry0 = 0x20
	entry2 = 0x40
	row32  = 0x820 + 32*32
	rowVag = 22
)

func hdr() []byte { return unsafe.Slice((*byte)(unsafe.Pointer(&vhData[0])), vhSize) }

func set(off int, b ...byte) {
	h := hdr()
	for i, v := range b {
		volatile.StoreUint8(&h[off+i], v)
	}
}

// setVag sets program 2's tone t's VAG index.
func setVag(t int, v int16) {
	set(row32+32*t+rowVag, byte(v), byte(v>>8))
}

// hdrDiff prints every header byte that differs from saved, with its
// offset and value, or "none".
func hdrDiff() {
	h := hdr()
	any := false
	for i := range saved {
		if b := volatile.LoadUint8(&h[i]); b != saved[i] {
			any = true
			print(" ", i, "=", b)
		}
	}
	if !any {
		print(" none")
	}
	println()
}

func restore() {
	h := hdr()
	for i := range saved {
		volatile.StoreUint8(&h[i], saved[i])
	}
}

func reg(v int16, off uintptr) uint16 {
	return volatile.LoadUint16((*uint16)(yaroze.Pointer(0x1f801c00 + uintptr(v)*16 + off)))
}

var hex = "0123456789abcdef"

func word(w uint16) {
	var b [4]byte
	for i := range b {
		b[i] = hex[w>>(12-4*i)&15]
	}
	print(string(b[:]))
}

// regs is a voice's six registers: volumes, pitch, start address, ADSR.
type regs [6]uint16

var offs = [6]uintptr{0, 2, 4, 6, 8, 0xa}

func read(v int16) (r regs) {
	for i, off := range offs {
		r[i] = reg(v, off)
	}
	return r
}

type snapshot [24]regs

func snap() (s snapshot) {
	for v := range int16(24) {
		s[v] = read(v)
	}
	return s
}

func printRegs(v int16, r regs) {
	print(" v", v)
	for _, w := range r {
		print(" ")
		word(w)
	}
	print(" env ", reg(v, 0xc) != 0)
}

// changes is a case's log: each voice whose registers changed in a frame,
// printed after the case so that printing does not stretch its timing.
var changes [96]struct {
	frame int
	v     int16
	r     regs
	env   bool
}

var logged int

// watch runs frames frames from now, logging every change of a voice's
// registers, and calls at(f) before frame f's blank.
func watch(frames int, at func(f int)) {
	logged = 0
	prev := snap()
	for f := range frames {
		if at != nil {
			at(f)
		}
		yaroze.VSync(0)
		for v := range int16(24) {
			r := read(v)
			if r == prev[v] {
				continue
			}
			prev[v] = r
			if logged < len(changes) {
				changes[logged].frame = f
				changes[logged].v = v
				changes[logged].r = r
				changes[logged].env = reg(v, 0xc) != 0
				logged++
			}
		}
	}
}

func printChanges() {
	if logged == 0 {
		print(" none")
	}
	for _, c := range changes[:logged] {
		print(" f", c.frame, " v", c.v)
		for _, w := range c.r {
			print(" ")
			word(w)
		}
		print(" env ", c.env, ";")
	}
	println()
}

// sounding reports whether any voice's envelope is above 0.
func sounding() bool {
	for v := int16(0); v < 24; v++ {
		if reg(v, 0xc) != 0 {
			return true
		}
	}
	return false
}

// quiet parks every voice and waits for the envelopes to fall, up to a
// second.
func quiet() {
	yaroze.SsUtAllKeyOff(0)
	for n := 0; sounding() && n < 60; n++ {
		yaroze.VSync(0)
	}
	for range 4 {
		yaroze.VSync(0)
	}
}

func wait(frames int) {
	for range frames {
		yaroze.VSync(0)
	}
}

var n int

// seqCase plays s loops times and watches frames frames, calling at(f)
// before each blank, then prints the changes, stops s, puts the header
// back and quiets.
func seqCase(part, label string, s int16, loops int16, frames int, at func(f int)) {
	yaroze.SsSeqPlay(s, 1, loops)
	watch(frames, at)
	print(part, " ", n, " ", label, ":")
	printChanges()
	n++
	yaroze.SsSeqStop(s)
	restore()
	quiet()
}

// utCase keys program p tone t on at note 60 with SsUtKeyOn after patch,
// prints the result and the voice's registers two frames later, and keys
// it off.
func utCase(label string, p, t int16, patch func()) {
	patch()
	v := yaroze.SsUtKeyOn(vab, p, t, 60, 0, 100, 100)
	wait(2)
	print("vag ", n, " ", label, ": r=", v)
	if v >= 0 && v < 24 {
		printRegs(v, read(v))
		yaroze.SsUtKeyOff(v, vab, p, t, 60)
	}
	println()
	n++
	restore()
	quiet()
}

// hexWord is w in hex, four digits.
func hexWord(w uint16) string {
	var b [4]byte
	for i := range b {
		b[i] = hex[w>>(12-4*i)&15]
	}
	return string(b[:])
}

// utVol keys program p tone 0 on at note 60 with SsUtKeyOn, returns the
// voice's left volume in hex two frames later and keys it off.
func utVol(p int16) string {
	v := yaroze.SsUtKeyOn(vab, p, 0, 60, 0, 100, 100)
	wait(2)
	if v < 0 || v >= 24 {
		return "refused"
	}
	s := hexWord(reg(v, 0))
	yaroze.SsUtKeyOff(v, vab, p, 0, 60)
	return s
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	if f := disc.LoadAll(300); f != nil {
		println("load failed")
	}
	vab = yaroze.SsVabTransfer(vh.Bytes(), vb.Bytes(), -1, 1)
	yaroze.SsSetMVol(127, 127)
	copy(saved[:], hdr())
	seqExpr = yaroze.SsSeqOpen(&exprData[0], vab)
	seqAfter = yaroze.SsSeqOpen(&afterData[0], vab)
	seqPlain = yaroze.SsSeqOpen(&plainData[0], vab)
	seqCH = yaroze.SsSeqOpen(&chData[0], vab)
	seqC7 = yaroze.SsSeqOpen(&c7Data[0], vab)
	seqC7A = yaroze.SsSeqOpen(&c7aData[0], vab)
	seqC7B = yaroze.SsSeqOpen(&c7bData[0], vab)
	seqP2 = yaroze.SsSeqOpen(&p2Data[0], vab)
	println("vab", vab, "seq", seqExpr, seqAfter, seqPlain, seqCH, seqC7, seqC7A, seqC7B, seqP2)
	for _, e := range []int{0, 1, 2, 3, 30, 31, 32, 33} {
		print("entry", e)
		for _, b := range saved[entry0+16*e : entry0+16*e+16] {
			print(" ")
			word(uint16(b))
		}
		println()
	}
	yaroze.SsUtAllKeyOff(0)
	quiet()

	// Part 1: the expression. Ticks 48, 96, 144 and 192 are blanks 16,
	// 31, 47 and 63 after the start; each case watches past its end.
	seqCase("expr", "PLAIN", seqPlain, 1, 40, nil)
	seqCase("expr", "EXPR: expression 64 at 0, notes at 48 and 144", seqExpr, 1, 70, nil)
	utVoice := int16(-1)
	seqCase("expr", "EXPR with SsUtKeyOn program 0 tone 0 note 60 in frame 20", seqExpr, 1, 40, func(f int) {
		if f == 20 {
			utVoice = yaroze.SsUtKeyOn(vab, 0, 0, 60, 0, 100, 100)
		}
	})
	println("expr ", n, " SsUtKeyOn's voice: ", utVoice)
	n++
	seqCase("expr", "AFTER: notes at 48 and 144, expression 64 at 120", seqAfter, 1, 70, nil)
	seqCase("expr", "AFTER played again", seqAfter, 1, 70, nil)
	seqCase("expr", "PLAIN after AFTER", seqPlain, 1, 40, nil)
	seqCase("expr", "AFTER twice (loops 2)", seqAfter, 2, 150, nil)
	yaroze.SsSeqClose(seqAfter)
	seqAfter = yaroze.SsSeqOpen(&afterData[0], vab)
	println("expr ", n, " AFTER closed and opened again: ", seqAfter)
	n++
	seqCase("expr", "AFTER after the reopen", seqAfter, 1, 70, nil)
	seqCase("expr", "CH: expression 64 ch0, notes 60 ch0 and 62 ch1 at 48, expression 32 ch0 at 96, 16 ch1 at 144", seqCH, 1, 70, nil)
	seqCase("expr", "C7: volume 64", seqC7, 1, 40, nil)
	seqCase("expr", "C7A: expression 64 then volume 64", seqC7A, 1, 40, nil)
	seqCase("expr", "C7B: volume 64 then expression 64", seqC7B, 1, 40, nil)
	seqCase("expr", "PLAIN again", seqPlain, 1, 40, nil)

	// Part 1b: where the expression lives and what ends it. The header
	// is not put back between these cases (seqCase puts it back). Each
	// keys program 0 (or 1) tone 0 on at note 60 with SsUtKeyOn at points
	// of a play of EXPR (expression 64 on program 0, its first note at
	// blank 16) and prints the voice's volume.
	println("expr ", n, " before EXPR: ", utVol(0))
	n++
	yaroze.SsSeqPlay(seqExpr, 1, 1)
	wait(40)
	println("expr ", n, " EXPR playing: ", utVol(0), " program 1: ", utVol(1))
	n++
	print("expr ", n, " header bytes changed by EXPR:")
	hdrDiff()
	n++
	yaroze.SsSeqPause(seqExpr)
	wait(2)
	println("expr ", n, " EXPR paused: ", utVol(0))
	n++
	yaroze.SsSeqReplay(seqExpr)
	wait(2)
	println("expr ", n, " EXPR replayed: ", utVol(0))
	n++
	yaroze.SsSeqStop(seqExpr)
	wait(2)
	println("expr ", n, " EXPR stopped: ", utVol(0))
	n++
	yaroze.SsUtAllKeyOff(0)
	wait(2)
	println("expr ", n, " after SsUtAllKeyOff: ", utVol(0))
	n++
	quiet()
	yaroze.SsSeqPlay(seqExpr, 1, 1)
	wait(90)
	println("expr ", n, " EXPR ended (frame 90): ", utVol(0))
	n++
	yaroze.SsSeqStop(seqExpr)
	quiet()
	yaroze.SsSeqPlay(seqExpr, 1, 1)
	wait(40)
	yaroze.SsSeqClose(seqExpr)
	wait(2)
	println("expr ", n, " EXPR closed while playing: ", utVol(0))
	n++
	seqExpr = yaroze.SsSeqOpen(&exprData[0], vab)
	wait(2)
	println("expr ", n, " EXPR opened again (", seqExpr, "): ", utVol(0))
	n++
	quiet()
	yaroze.SsSeqPlay(seqExpr, 1, 1)
	wait(40)
	yaroze.SsSeqPlay(seqP2, 1, 1)
	wait(2)
	println("expr ", n, " EXPR playing, P2 started two frames ago: ", utVol(0))
	n++
	yaroze.SsSeqStop(seqP2)
	yaroze.SsSeqStop(seqExpr)
	quiet()
	seqCase("expr", "EXPR, PLAIN started in frame 20", seqExpr, 1, 70, func(f int) {
		if f == 20 {
			yaroze.SsSeqPlay(seqPlain, 1, 1)
		}
	})
	yaroze.SsSeqStop(seqPlain)
	quiet()
	seqCase("expr", "EXPR, program 0 volume byte 64 in frame 8", seqExpr, 1, 40, func(f int) {
		if f == 8 {
			set(entry0+1, 64)
		}
	})
	seqCase("expr", "PLAIN once more", seqPlain, 1, 40, nil)
	// The program's volume byte set to 100 before the play: does the
	// expression replace it or scale it?
	yaroze.SsSeqPlay(seqExpr, 1, 1)
	set(entry0+1, 100)
	watch(40, nil)
	print("expr ", n, " EXPR with program 0 volume 100 set before the play:")
	printChanges()
	n++
	print("expr ", n, " header bytes changed:")
	hdrDiff()
	n++
	yaroze.SsSeqStop(seqExpr)
	restore()
	quiet()
	yaroze.SsSeqPlay(seqC7, 1, 1)
	wait(40)
	print("expr ", n, " header bytes changed by C7 (volume 64):")
	hdrDiff()
	n++
	yaroze.SsSeqStop(seqC7)
	restore()
	quiet()
	yaroze.SsSeqPlay(seqCH, 1, 1)
	wait(70)
	print("expr ", n, " header bytes changed by CH (expression 64, 32, 16):")
	hdrDiff()
	n++
	yaroze.SsSeqStop(seqCH)
	restore()
	quiet()

	// Part 2: the VAG index. Program 2's tones 0 to 7 have centers 60,
	// 72, 84, 96, 108, 120, 48 and 54 (pitches 1000h, 0800h, 0400h,
	// 0200h, 0100h, 0080h, 2000h, 16a0h) and name VAG 1. The bank's VAGs
	// start at 0202h, 020ch, 021eh and 0238h and end at 025ah (halves 0 to
	// 4: entry 0's two, entry 1's two, entry 2's first). The patch is made
	// in frame 8, after the program change, before the note.
	vagCase := func(label string, patch func()) {
		seqCase("vag", label, seqP2, 1, 40, func(f int) {
			if f == 8 && patch != nil {
				patch()
			}
		})
	}
	vagCase("P2 base, all VAG 1", nil)
	vagCase("VAG 1 2 3 4 0 5 6 7", func() {
		for t, v := range []int16{1, 2, 3, 4, 0, 5, 6, 7} {
			setVag(t, v)
		}
	})
	// Entry 0's bytes 4 to 11 are given values of their own, so that a
	// half read before entry 0's first half (half -1 is its bytes 10 and
	// 11, -2 bytes 8 and 9, and so on) shows.
	marks := func() { set(entry0+4, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab) }
	vagCase("VAG 0 -1 -2 -3 -4 -5 -6 1, entry 0 bytes 4 to 11 marked", func() {
		marks()
		for t, v := range []int16{0, -1, -2, -3, -4, -5, -6, 1} {
			setVag(t, v)
		}
	})
	vagCase("VAG 0, entry 0 halves 0300h 0400h", func() {
		setVag(0, 0)
		set(entry0+12, 0x00, 0x03, 0x00, 0x04)
	})
	// One tone (program 2's tone count set to 1) with each index.
	for _, v := range []int16{8, -1, -2, 0x00ff, 0x0100, 0x0101, 0x0102, 0x0103, 0x0200, 0x7f01, 0x7fff, -32768, -32767, -32766, 0x1001} {
		vagCase("tone 0 alone, VAG "+hexWord(uint16(v))+", entry 0 marked", func() {
			marks()
			set(entry2, 1)
			setVag(0, v)
		})
	}
	// Halves past the VAGs' and the entries far up the table marked:
	// half 4 (entry 2's first, the end) 0444h, half 5 0555h, halves 6 and
	// 7 (entry 3's) 0666h and 0777h, entry 31's halves 8c8dh 8e8fh and
	// entry 32's bytes 90h to 9fh; with the header's VAG count set to 1.
	far := func() {
		set(entry2+12, 0x44, 0x04, 0x55, 0x05, 0x66, 0x06, 0x77, 0x07)
		set(entry0+16*31+12, 0x8c, 0x8d, 0x8e, 0x8f)
		set(entry0+16*32, 0x90, 0x91, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0x9b, 0x9c, 0x9d, 0x9e, 0x9f)
		set(0x16, 1, 0)
	}
	for _, v := range []int16{4, 5, 6, 7, 8, 0x00fa, 0x00fc, 0x00fe} {
		vagCase("tone 0 alone, VAG "+hexWord(uint16(v))+", far halves marked, VAG count 1", func() {
			far()
			set(entry2, 1)
			setVag(0, v)
		})
	}
	for _, v := range []int16{4, 5, 6, 7, 8, 0x00fa, 0x00fc, 0x00fe, 0x0100} {
		utCase("SsUtKeyOn program 2 tone 0 VAG "+hexWord(uint16(v))+", far halves marked, VAG count 1", 2, 0, func() {
			far()
			setVag(0, v)
		})
	}
	// The header's program count against the halves read from it.
	vagCase("tone 0 alone, VAG 7, far halves marked, programs 4", func() {
		far()
		set(0x12, 4, 0)
		set(entry2, 1)
		setVag(0, 7)
	})
	utCase("SsUtKeyOn program 2 tone 0 VAG 7, far halves marked, programs 4", 2, 0, func() {
		far()
		set(0x12, 4, 0)
		setVag(0, 7)
	})
	utCase("SsUtKeyOn program 2 tone 0 VAG 5, far halves marked, programs 2", 2, 0, func() {
		far()
		set(0x12, 2, 0)
		setVag(0, 5)
	})
	utCase("SsUtKeyOn program 2 tone 0 VAG 3, far halves marked, programs 1", 2, 0, func() {
		far()
		set(0x12, 1, 0)
		setVag(0, 3)
	})
	for _, v := range []int16{1, 0, 2, 5, 6, 7, -1, -2, 0x00ff, 0x0100, 0x0102, 0x7fff, -32768, -32766} {
		utCase("SsUtKeyOn program 2 tone 0 VAG "+hexWord(uint16(v))+", entry 0 marked", 2, 0, func() {
			marks()
			setVag(0, v)
		})
	}

	// Part 3: a tempo of 0 on a playing sequence, in frame 2 of the play.
	yaroze.SsSeqPlay(seqPlain, 1, 1)
	wait(2)
	println("tempo: before SsSetTempo(PLAIN, 0, 0)")
	yaroze.SsSetTempo(seqPlain, 0, 0)
	println("tempo: after the call")
	before := snap()
	for f := range 40 {
		yaroze.VSync(0)
		if snap() != before {
			println("tempo: a voice changed in frame ", f+3)
			before = snap()
		}
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}
