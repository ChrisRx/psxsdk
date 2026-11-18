//go:build yaroze

// Command seqhdr is a probe for the spec, on the voices probe's bank
// (assets/PROBE.VH and PROBE.VB) and the SEQs gen.go makes. Four parts,
// each printing "case" lines over the TTY; the SPU writes the harness
// records show the rest:
//
//   - seq N: what the sequencer reads from the VAB header at a note on.
//     Each case plays ONE (a note on program 1 at tick 48, blank 16 at 120
//     beats a minute) or P0 (the same on program 0, whose five tones all
//     cover the note), changes bytes of the header eight frames after
//     SsSeqPlay (after the program change at tick 0 has been played, before
//     the note), and prints at frame 26 every voice whose registers
//     changed since the play (volumes, pitch, start address, ADSR, envelope
//     up). The header is put back after each case.
//   - expr N: controller 11 and the program volume: the header's byte, and
//     SsUtKeyOn's volumes, before and after a sequence with expression 64
//     on program 1's channel, and after the header is put back.
//   - koff N: SsUtKeyOff's result and the voice's envelope one and three
//     frames later, with the right arguments, each wrong one, other voice
//     numbers, the same frame as the key on, twice, a sequence's voice, and
//     a zeroed header. Each case prints a short marker before it so its
//     frames can be found in the harness's dump (the "tty" count). The
//     envelope a frame after a key off depends on where in the frame it is
//     read (4000h or 8000h); the dump's key off words are the evidence.
//   - tempo N: a control, then SsSetTempo(id, 0, 0) on the playing
//     sequence, last since Sony's library stops at a break there. Each
//     prints the frame, from SsSeqPlay, in which the note's voice registers
//     changed (-1 if never in 150). The tempo0 probe has the other ways of
//     giving a tempo of 0.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

const vhSize = 3616

var (
	vhData   [(vhSize + 2047) / 2048 * 512]uint32
	vbData   [(1008 + 2047) / 2048 * 512]uint32
	oneData  [512]uint32
	p0Data   [512]uint32
	cc11Data [512]uint32
	nopcData [512]uint32
	tzData   [512]uint32

	vh   = disc.Register("ASSETS/PROBE.VH", "assets/PROBE.VH").Into(vhData[:])
	vb   = disc.Register("ASSETS/PROBE.VB", "assets/PROBE.VB").Into(vbData[:])
	one  = disc.Register("ASSETS/ONE.SEQ", "assets/ONE.SEQ").Into(oneData[:])
	p0   = disc.Register("ASSETS/P0.SEQ", "assets/P0.SEQ").Into(p0Data[:])
	cc11 = disc.Register("ASSETS/CC11.SEQ", "assets/CC11.SEQ").Into(cc11Data[:])
	nopc = disc.Register("ASSETS/NOPC.SEQ", "assets/NOPC.SEQ").Into(nopcData[:])
	tz   = disc.Register("ASSETS/TZ.SEQ", "assets/TZ.SEQ").Into(tzData[:])

	// saved is the header as SsVabTransfer left it, put back after each
	// case.
	saved [vhSize]byte
	vab   int16

	// The sequences, opened once.
	seqOne, seqP0, seqCC11, seqNOPC, seqTZ int16
)

// Header offsets: program entries are 16 bytes from 20h, tone rows 32
// bytes from 820h (row 16·p + t for program p's tone t as transferred).
const (
	entry0 = 0x20
	entry1 = 0x30
	row0   = 0x820
	row16  = 0x820 + 16*32
	row17  = 0x820 + 17*32
)

func hdr() []byte { return unsafe.Slice((*byte)(unsafe.Pointer(&vhData[0])), vhSize) }

func set(off int, b ...byte) {
	h := hdr()
	for i, v := range b {
		volatile.StoreUint8(&h[off+i], v)
	}
}

func get(off int) byte { return volatile.LoadUint8(&hdr()[off]) }

func zero() {
	h := hdr()
	for i := range h {
		volatile.StoreUint8(&h[i], 0)
	}
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

// changed reports whether any voice's registers differ from s.
func changed(s *snapshot) bool {
	for v := range int16(24) {
		if read(v) != s[v] {
			return true
		}
	}
	return false
}

// printChanged prints every voice whose registers differ from s, with
// its registers and whether its envelope is up; "none" if no voice does.
func printChanged(s *snapshot) {
	any := false
	for v := range int16(24) {
		r := read(v)
		if r == s[v] {
			continue
		}
		any = true
		print(" v", v, ":")
		for _, w := range r {
			print(" ")
			word(w)
		}
		print(" env ", reg(v, 0xc) != 0)
	}
	if !any {
		print(" none")
	}
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

// seqCase plays s, calls patch eight frames later, prints the voices that
// changed at frame 26, stops s, puts the header back and quiets. With
// late set the patch is made at frame 22 instead, after the note.
func seqCase(label string, s int16, late bool, patch func()) {
	before := snap()
	yaroze.SsSeqPlay(s, 1, 1)
	at := 8
	if late {
		at = 22
	}
	wait(at)
	if patch != nil {
		patch()
	}
	wait(26 - at)
	print("seq ", n, " ", label, ":")
	printChanged(&before)
	println()
	n++
	yaroze.SsSeqStop(s)
	restore()
	quiet()
}

// utKeyOn keys program p tone t on at note note and prints what the tick
// wrote for the voice, then keys it off.
func utKeyOn(label string, p, t, note int16) {
	v := yaroze.SsUtKeyOn(vab, p, t, note, 0, 100, 100)
	wait(2)
	print("expr ", n, " ", label, ": v=", v)
	if v >= 0 && v < 24 {
		for _, w := range read(v) {
			print(" ")
			word(w)
		}
		print(" env ", reg(v, 0xc) != 0)
		yaroze.SsUtKeyOff(v, vab, p, t, note)
	}
	println()
	n++
	quiet()
}

// koffCase keys program 1 tone 0 on at note 60 (unless on is false), waits
// two frames, calls off with the voice (which off may change to the voice
// it keyed on itself), and prints off's result and the voice's (and its
// neighbor's) envelope one and three frames later. The marker line before
// lets the frames be found in the dump.
func koffCase(label string, on bool, off func(v *int16) int16) {
	println("k", n)
	v := int16(0)
	if on {
		v = yaroze.SsUtKeyOn(vab, 1, 0, 60, 0, 100, 100)
		wait(2)
	}
	r := off(&v)
	wait(1)
	e1, f1 := reg(v, 0xc), reg(v+1, 0xc)
	wait(2)
	e3, f3 := reg(v, 0xc), reg(v+1, 0xc)
	print("koff ", n, " ", label, ": v=", v, " r=", r, " env ")
	word(e1)
	print(" ")
	word(e3)
	print(" next ")
	word(f1)
	print(" ")
	word(f3)
	println()
	n++
	quiet()
}

// tempoCase plays s, calls setup in frame at from the play (before the
// play when at is 0), and prints the frame in which a voice's registers
// first changed, then stops it.
func tempoCase(label string, s int16, at int, setup func()) {
	println("t", n)
	before := snap()
	if setup != nil && at == 0 {
		setup()
	}
	yaroze.SsSeqPlay(s, 1, 1)
	first := -1
	for f := range 150 {
		if setup != nil && f == at {
			setup()
		}
		if first < 0 && changed(&before) {
			first = f
		}
		yaroze.VSync(0)
	}
	print("tempo ", n, " ", label, ": first change at frame ", first, ":")
	printChanged(&before)
	println()
	n++
	yaroze.SsSeqStop(s)
	quiet()
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
	seqOne = yaroze.SsSeqOpen(&oneData[0], vab)
	seqP0 = yaroze.SsSeqOpen(&p0Data[0], vab)
	seqCC11 = yaroze.SsSeqOpen(&cc11Data[0], vab)
	seqNOPC = yaroze.SsSeqOpen(&nopcData[0], vab)
	seqTZ = yaroze.SsSeqOpen(&tzData[0], vab)
	println("vab", vab, "seq", seqOne, seqP0, seqCC11, seqNOPC, seqTZ)
	print("entry1")
	for _, b := range saved[entry1 : entry1+16] {
		print(" ")
		word(uint16(b))
	}
	println()
	yaroze.SsUtAllKeyOff(0)
	quiet()

	// Part 1: the sequencer and the header. Program 1's entry is at 30h
	// (tones, volume, priority, mode, pan, then the bytes SsVabTransfer
	// wrote: the slot word at 8, the VAG address halves at 12), its tone 0
	// row 16 at a20h (priority, mode, volume, pan, center, shift, min, max
	// at 0 to 7, ADSR at 16 to 19, program at 20, VAG at 22).
	seqCase("base", seqOne, false, nil)
	seqCase("row16 adsr 0f0f 1f1f", seqOne, false, func() { set(row16+16, 0x0f, 0x0f, 0x1f, 0x1f) })
	seqCase("row16 center 48", seqOne, false, func() { set(row16+4, 48) })
	seqCase("row16 shift 64", seqOne, false, func() { set(row16+5, 64) })
	seqCase("row16 vol 20h pan 0", seqOne, false, func() { set(row16+2, 0x20, 0x00) })
	seqCase("row16 vag 2", seqOne, false, func() { set(row16+22, 2, 0) })
	seqCase("row16 min 7fh", seqOne, false, func() { set(row16+6, 0x7f) })
	seqCase("row16 max 10h", seqOne, false, func() { set(row16+7, 0x10) })
	seqCase("row16 min 61", seqOne, false, func() { set(row16+6, 61) })
	seqCase("row16 max 59", seqOne, false, func() { set(row16+7, 59) })
	seqCase("row16 min 60 max 60", seqOne, false, func() { set(row16+6, 60, 60) })
	seqCase("row16 mode 1 vol 2", seqOne, false, func() { set(row16+1, 0x01, 0x02) })
	seqCase("row16 mode 4", seqOne, false, func() { set(row16+1, 0x04) })
	seqCase("row16 priority 7fh", seqOne, false, func() { set(row16, 0x7f) })
	seqCase("program 1 vol 20h", seqOne, false, func() { set(entry1+1, 0x20) })
	seqCase("program 1 pan 7fh", seqOne, false, func() { set(entry1+4, 0x7f) })
	seqCase("program 1 tones 0", seqOne, false, func() { set(entry1, 0) })
	seqCase("program 1 tones 2", seqOne, false, func() { set(entry1, 2) })
	seqCase("program 1 tones 2, row17 = row16 with center 48", seqOne, false, func() {
		set(entry1, 2)
		for i := range 32 {
			set(row17+i, saved[row16+i])
		}
		set(row17+4, 48)
	})
	seqCase("entry 1 word at 8 = 0", seqOne, false, func() { set(entry1+8, 0, 0, 0, 0) })
	seqCase("entry 1 word at 8 = 0, row 0 center 48", seqOne, false, func() {
		set(entry1+8, 0, 0, 0, 0)
		set(row0+4, 48)
	})
	seqCase("entry 1 byte at 8 = 2 (rows 32 up, zero)", seqOne, false, func() { set(entry1+8, 2) })
	seqCase("entry 0 halves at 12 = 0300h 0400h", seqOne, false, func() { set(entry0+12, 0x00, 0x03, 0x00, 0x04) })
	seqCase("entry 1 halves at 12 = 0500h 0600h", seqOne, false, func() { set(entry1+12, 0x00, 0x05, 0x00, 0x06) })
	seqCase("header programs 0", seqOne, false, func() { set(0x12, 0, 0) })
	seqCase("header programs 1", seqOne, false, func() { set(0x12, 1, 0) })
	seqCase("header programs 3", seqOne, false, func() { set(0x12, 3, 0) })
	seqCase("header vags 0", seqOne, false, func() { set(0x16, 0, 0) })
	seqCase("vag table entry 1 = 16", seqOne, false, func() { set(0xc22, 0x10, 0x00) })
	seqCase("magic zeroed", seqOne, false, func() { set(0, 0, 0, 0, 0) })
	seqCase("header zeroed", seqOne, false, zero)
	seqCase("row16 vag 0", seqOne, false, func() { set(row16+22, 0, 0) })
	seqCase("row16 vag 0, entry 0 halves 0300h 0400h", seqOne, false, func() {
		set(row16+22, 0, 0)
		set(entry0+12, 0x00, 0x03, 0x00, 0x04)
	})
	seqCase("row16 vag 0, entry 0 word at 8 = 05550000h", seqOne, false, func() {
		set(row16+22, 0, 0)
		set(entry0+8, 0x00, 0x00, 0x55, 0x05)
	})
	seqCase("row16 vag 0, entry 1 halves 0500h 0600h", seqOne, false, func() {
		set(row16+22, 0, 0)
		set(entry1+12, 0x00, 0x05, 0x00, 0x06)
	})
	seqCase("row16 vag 0, row16 center 48", seqOne, false, func() {
		set(row16+22, 0, 0)
		set(row16+4, 48)
	})
	seqCase("row16 vag 3", seqOne, false, func() { set(row16+22, 3, 0) })
	seqCase("row16 vag 4 (past the end half)", seqOne, false, func() { set(row16+22, 4, 0) })
	seqCase("row16 vag 2, entry 0 halves 0300h 0400h", seqOne, false, func() {
		set(row16+22, 2, 0)
		set(entry0+12, 0x00, 0x03, 0x00, 0x04)
	})
	seqCase("entry 0 halves 0", seqOne, false, func() { set(entry0+12, 0, 0, 0, 0) })
	seqCase("header zeroed but entry 1 tones 1 slot 1, entry 0 halves, row16 vag 1 max 7fh", seqOne, false, func() {
		zero()
		set(entry1, 1)
		set(entry1+8, 1)
		set(entry0+12, saved[entry0+12], saved[entry0+13], saved[entry0+14], saved[entry0+15])
		set(row16+7, 0x7f)
		set(row16+22, 1, 0)
	})
	seqCase("header zeroed but programs 2, entry 1 tones 1 slot 1, entry 0 halves, row16 vag 1 max 7fh", seqOne, false, func() {
		zero()
		set(0x12, 2)
		set(entry1, 1)
		set(entry1+8, 1)
		set(entry0+12, saved[entry0+12], saved[entry0+13], saved[entry0+14], saved[entry0+15])
		set(row16+7, 0x7f)
		set(row16+22, 1, 0)
	})
	seqCase("row16 center 48, patched after the note", seqOne, true, func() { set(row16+4, 48) })
	seqCase("base again", seqOne, false, nil)
	seqCase("P0 base", seqP0, false, nil)
	seqCase("P0 row 0 max 10h", seqP0, false, func() { set(row0+7, 0x10) })
	seqCase("P0 program 0 tones 2", seqP0, false, func() { set(entry0, 2) })
	seqCase("P0 program 0 tones 0", seqP0, false, func() { set(entry0, 0) })

	// Part 2: controller 11 and the program volume.
	println("expr ", n, " header program 1 vol: ", get(entry1+1))
	n++
	utKeyOn("before", 1, 0, 60)
	seqCase("NOPC before CC11", seqNOPC, false, nil)
	seqCase("CC11: expression 64 on program 1", seqCC11, false, nil)
	seqCase("NOPC after CC11", seqNOPC, false, nil)
	println("expr ", n, " header program 1 vol after CC11: ", get(entry1+1), " program 0: ", get(entry0+1))
	n++
	utKeyOn("after CC11", 1, 0, 60)
	restore()
	println("expr ", n, " header program 1 vol after restore: ", get(entry1+1))
	n++
	utKeyOn("after restore", 1, 0, 60)
	seqCase("ONE after restore", seqOne, false, nil)
	utKeyOn("after ONE", 1, 0, 60)

	// Part 3: SsUtKeyOff.
	koffCase("right", true, func(v *int16) int16 { return yaroze.SsUtKeyOff(*v, vab, 1, 0, 60) })
	koffCase("note 62", true, func(v *int16) int16 { return yaroze.SsUtKeyOff(*v, vab, 1, 0, 62) })
	koffCase("program 0", true, func(v *int16) int16 { return yaroze.SsUtKeyOff(*v, vab, 0, 0, 60) })
	koffCase("tone 1", true, func(v *int16) int16 { return yaroze.SsUtKeyOff(*v, vab, 1, 1, 60) })
	koffCase("vab+1", true, func(v *int16) int16 { return yaroze.SsUtKeyOff(*v, vab+1, 1, 0, 60) })
	koffCase("voice+1", true, func(v *int16) int16 { return yaroze.SsUtKeyOff(*v+1, vab, 1, 0, 60) })
	koffCase("voice 24", true, func(v *int16) int16 { return yaroze.SsUtKeyOff(24, vab, 1, 0, 60) })
	koffCase("voice -1", true, func(v *int16) int16 { return yaroze.SsUtKeyOff(-1, vab, 1, 0, 60) })
	koffCase("twice", true, func(v *int16) int16 {
		yaroze.SsUtKeyOff(*v, vab, 1, 0, 60)
		return yaroze.SsUtKeyOff(*v, vab, 1, 0, 60)
	})
	koffCase("twice, a frame apart", true, func(v *int16) int16 {
		yaroze.SsUtKeyOff(*v, vab, 1, 0, 60)
		wait(1)
		return yaroze.SsUtKeyOff(*v, vab, 1, 0, 60)
	})
	koffCase("free voice 3, right arguments", false, func(v *int16) int16 {
		*v = 3
		return yaroze.SsUtKeyOff(3, vab, 1, 0, 60)
	})
	koffCase("same frame as the key on", false, func(v *int16) int16 {
		*v = yaroze.SsUtKeyOn(vab, 1, 0, 60, 0, 100, 100)
		return yaroze.SsUtKeyOff(*v, vab, 1, 0, 60)
	})
	koffCase("header zeroed", true, func(v *int16) int16 {
		zero()
		r := yaroze.SsUtKeyOff(*v, vab, 1, 0, 60)
		restore()
		return r
	})
	koffCase("two voices same tone, off the second", true, func(v *int16) int16 {
		yaroze.SsUtKeyOn(vab, 1, 0, 60, 0, 100, 100)
		wait(2)
		return yaroze.SsUtKeyOff(*v+1, vab, 1, 0, 60)
	})
	koffCase("after SsUtAllKeyOff", true, func(v *int16) int16 {
		yaroze.SsUtAllKeyOff(0)
		wait(2)
		return yaroze.SsUtKeyOff(*v, vab, 1, 0, 60)
	})
	koffCase("released and freed (after 40 frames)", true, func(v *int16) int16 {
		yaroze.SsUtKeyOff(*v, vab, 1, 0, 60)
		wait(40)
		return yaroze.SsUtKeyOff(*v, vab, 1, 0, 60)
	})
	// A sequence's voice: ONE's note is on from blank 16; the voice is the
	// one whose registers changed.
	{
		println("k", n)
		before := snap()
		yaroze.SsSeqPlay(seqOne, 1, 1)
		wait(22)
		v := int16(-1)
		for w := range int16(24) {
			if read(w) != before[w] {
				v = w
				break
			}
		}
		r := yaroze.SsUtKeyOff(v, vab, 1, 0, 60)
		wait(1)
		e1 := reg(v, 0xc)
		wait(2)
		e3 := reg(v, 0xc)
		print("koff ", n, " sequence voice: v=", v, " r=", r, " env ")
		word(e1)
		print(" ")
		word(e3)
		println()
		n++
		yaroze.SsSeqStop(seqOne)
		quiet()
	}

	// Part 4: a tempo of 0.
	tempoCase("ONE, control", seqOne, 0, nil)
	tempoCase("ONE, SsSetTempo 0 in frame 2 of the play", seqOne, 2, func() { yaroze.SsSetTempo(seqOne, 0, 0) })
	println("done")
	for {
		yaroze.VSync(0)
	}
}
