//go:build yaroze

// Command vabhdr is a probe for the spec: what SsUtKeyOn reads from the
// VAB header after SsVabTransfer, when the header's memory is changed
// afterwards. terra loads another bank's body over the buffer that held
// its effect bank's header, and the library keys the effects on with what
// it finds there. Each case changes bytes of the header (the voices
// probe's bank, assets/PROBE.VH and PROBE.VB), keys program 0 tone 0 on
// at note 60, waits for the tick, and prints the voice's registers:
// volumes, pitch, start address, ADSR and whether its envelope is up.
// The header is put back after each case.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

const vhSize = 3616

var (
	vhData [(vhSize + 2047) / 2048 * 512]uint32
	vbData [(1008 + 2047) / 2048 * 512]uint32

	vh = disc.Register("ASSETS/PROBE.VH", "assets/PROBE.VH").Into(vhData[:])
	vb = disc.Register("ASSETS/PROBE.VB", "assets/PROBE.VB").Into(vbData[:])

	// saved is the header as SsVabTransfer left it, put back after each
	// case.
	saved [vhSize]byte
	vab   int16
)

func hdr() []byte { return unsafe.Slice((*byte)(unsafe.Pointer(&vhData[0])), vhSize) }

func set(off int, b ...byte) {
	h := hdr()
	for i, v := range b {
		volatile.StoreUint8(&h[off+i], v)
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

// sounding reports whether any voice's envelope is above 0.
func sounding() bool {
	for v := int16(0); v < 24; v++ {
		if reg(v, 0xc) != 0 {
			return true
		}
	}
	return false
}

// quiet parks every voice (a case may have given one a release that
// never ends) and waits for the envelopes to fall, up to a second.
func quiet() {
	yaroze.SsUtAllKeyOff(0)
	for n := 0; sounding() && n < 60; n++ {
		yaroze.VSync(0)
	}
	for range 4 {
		yaroze.VSync(0)
	}
}

var n int

// keyOn keys program p tone t on at note note and prints what the tick
// wrote for the voice.
func keyOn(label string, p, t, note int16) {
	v := yaroze.SsUtKeyOn(vab, p, t, note, 0, 100, 100)
	yaroze.VSync(0)
	yaroze.VSync(0)
	print("case ", n, " ", label, ": v=", v)
	if v >= 0 && v < 24 {
		for _, off := range []uintptr{0, 2, 4, 6, 8, 0xa} {
			print(" ")
			word(reg(v, off))
		}
		print(" env ", reg(v, 0xc) != 0)
		yaroze.SsUtKeyOff(v, vab, p, t, note)
	}
	println()
	n++
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
	println("vab", vab)
	// The header after the transfer: the first program entry's last eight
	// bytes and the VAG table.
	print("entry0")
	for _, b := range saved[0x20:0x30] {
		print(" ")
		word(uint16(b))
	}
	println()
	yaroze.SsUtAllKeyOff(0)
	quiet()

	keyOn("base", 0, 0, 60)
	// Tone 0 of program 0 is at 820h: priority, mode, volume, pan, center,
	// shift, min, max at 0 to 7, ADSR at 16 to 19, program at 20, VAG at 22.
	set(0x830, 0x0f, 0x0f, 0x1f, 0x1f)
	keyOn("tone adsr 0f0f 1f1f", 0, 0, 60)
	restore()
	set(0x824, 48)
	keyOn("tone center 48", 0, 0, 60)
	restore()
	set(0x825, 64)
	keyOn("tone shift 64", 0, 0, 60)
	restore()
	set(0x822, 0x20, 0x00)
	keyOn("tone vol 20h pan 0", 0, 0, 60)
	restore()
	set(0x836, 2, 0)
	keyOn("tone vag 2", 0, 0, 60)
	restore()
	set(0x826, 0x7f)
	keyOn("tone min 7fh", 0, 0, 60)
	restore()
	set(0x827, 0x10)
	keyOn("tone max 10h", 0, 0, 60)
	restore()
	set(0x821, 0x01, 0x02)
	keyOn("tone mode 1 vol 2", 0, 0, 60)
	restore()
	// Program 0's entry is at 20h: tones, volume, priority, mode, pan, then
	// the bytes SsVabTransfer wrote.
	set(0x21, 0x20)
	keyOn("program vol 20h", 0, 0, 60)
	restore()
	set(0x24, 0x7f)
	keyOn("program pan 7fh", 0, 0, 60)
	restore()
	set(0x20, 0)
	keyOn("program tones 0", 0, 0, 60)
	restore()
	set(0x28, 1, 0, 0, 0)
	keyOn("entry 0 word at 8 = 1", 0, 0, 60)
	restore()
	set(0x2c, 0x00, 0x03, 0x00, 0x04)
	keyOn("entry 0 halves at 12 = 0300h 0400h", 0, 0, 60)
	restore()
	set(0x3c, 0x00, 0x05, 0x00, 0x06)
	keyOn("entry 1 halves at 12 = 0500h 0600h", 0, 0, 60)
	restore()
	set(0x12, 0, 0)
	keyOn("header programs 0", 0, 0, 60)
	restore()
	set(0x16, 0, 0)
	keyOn("header vags 0", 0, 0, 60)
	restore()
	set(0xc22, 0x10, 0x00)
	keyOn("vag table entry 1 = 16", 0, 0, 60)
	restore()
	set(0, 0, 0, 0, 0)
	keyOn("magic zeroed", 0, 0, 60)
	restore()
	h := hdr()
	for i := range h {
		volatile.StoreUint8(&h[i], 0)
	}
	keyOn("header zeroed", 0, 0, 60)
	restore()
	// Which rows the entry's word at 8 picks: program 1's tone 0 is row 16
	// (program 0 has 5 tones but 16 rows); row 16's center changed shows
	// whether the word is read.
	set(0x28, 1, 0, 0, 0)
	set(0x820+16*32+4, 48)
	keyOn("entry 0 word at 8 = 1, row 16 center 48", 0, 0, 60)
	restore()
	set(0x836, 0, 0)
	keyOn("tone vag 0", 0, 0, 60)
	restore()
	set(0x836, 3, 0)
	keyOn("tone vag 3", 0, 0, 60)
	restore()
	set(0x836, 2, 0)
	set(0x2c, 0x00, 0x03, 0x00, 0x04)
	keyOn("tone vag 2, entry 0 halves 0300h 0400h", 0, 0, 60)
	restore()
	set(0x2c, 0, 0, 0, 0)
	keyOn("entry 0 halves 0", 0, 0, 60)
	restore()
	set(0x2c, 0, 0, 0, 0)
	set(0x3c, 0, 0, 0, 0)
	keyOn("entries 0 and 1 halves 0", 0, 0, 60)
	restore()
	for i := range h {
		volatile.StoreUint8(&h[i], 0)
	}
	set(0x2c, saved[0x2c], saved[0x2d], saved[0x2e], saved[0x2f])
	keyOn("header zeroed but entry 0 halves", 0, 0, 60)
	restore()
	for i := range h {
		volatile.StoreUint8(&h[i], 0)
	}
	set(0x2c, saved[0x2c], saved[0x2d], saved[0x2e], saved[0x2f])
	set(0x836, 1, 0)
	keyOn("header zeroed but entry 0 halves and tone vag 1", 0, 0, 60)
	restore()
	keyOn("base again", 0, 0, 60)
	// Program 1, then its tone row changed.
	keyOn("program 1", 1, 0, 60)
	set(0x820+5*32+4, 48)
	keyOn("program 1 center 48 (row 5)", 1, 0, 60)
	restore()
	set(0x820+16*32+4, 48)
	keyOn("program 1 center 48 (row 16)", 1, 0, 60)
	restore()
	println("done")
	for {
		yaroze.VSync(0)
	}
}
