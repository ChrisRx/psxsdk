//go:build yaroze

// Command vagtable is a probe for the spec: where a key on finds a VAG's start
// address when a bank has more VAGs than it has room for in its programs'
// entries (spec 36). Each bank is built here, with p programs of one tone and vs
// VAGs of 16 or 32 bytes, each ending in an end flag so that it stops by itself.
// Parts, each printing lines over the TTY:
//
//   - bank P VS: the halves at bytes 12 to 15 of the program entries after
//     SsVabTransfer, which were all marked with A000h + the half's number before
//     it, and how many marks are left.
//   - ut: SsUtKeyOn's result and start address with row 0's VAG index v, and the
//     start address again with half v-1 of the header marked 4000h + v-1 after the
//     transfer.
//   - seq: the same for a sequence's note on, a voice found by its start address.
//   - counts: the header's counts changed after the transfer.
//   - two: two banks open at once.
//   - voice: seven key ons of a sound that ends by itself, one a frame, as a
//     program makes them, then a key on after a wait. The voice it returns follows
//     from where the seven started.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	maxPrograms = 8
	entries     = 0x20
	rows        = 0x820
	rowSize     = 32
	rowVag      = 22
)

var (
	vhA, vhB [(rows + maxPrograms*16*rowSize + 512) / 4]uint32
	vbData   [(254*32 + 64) / 4]uint32

	// plain is a sequence of one note, 60 at tick 48 on program 0, at 120 beats a
	// minute and 96 ticks a quarter: blank 16 after the play.
	plainBytes = [...]byte{
		0x70, 0x51, 0x45, 0x53, 0, 0, 0, 1, 0, 0x60, 0x07, 0xa1, 0x20, 0, 0, 0,
		0xc0, 0, 0x30, 0x90, 0x3c, 0x64, 0x30, 0x3c, 0, 0, 0xff, 0x2f, 0,
	}
	plain [8]uint32
)

const hex = "0123456789abcdef"

func word(w uint16) string {
	var b [4]byte
	for i := range b {
		b[i] = hex[w>>(12-4*i)&15]
	}
	return string(b[:])
}

func put8(h unsafe.Pointer, off int, v uint8) {
	volatile.StoreUint8((*uint8)(unsafe.Add(h, off)), v)
}

func put16(h unsafe.Pointer, off int, v uint16) {
	put8(h, off, uint8(v))
	put8(h, off+1, uint8(v>>8))
}

func get16(h unsafe.Pointer, off int) uint16 {
	return uint16(volatile.LoadUint8((*uint8)(unsafe.Add(h, off)))) | uint16(volatile.LoadUint8((*uint8)(unsafe.Add(h, off+1))))<<8
}

// build writes a bank of p programs and vs VAGs to h, and the samples to vbData.
// The tones' ADSR is adsr1, adsr2.
func build(h unsafe.Pointer, p, vs int, adsr1, adsr2 uint16) {
	for i := range rows + maxPrograms*16*rowSize + 512 {
		put8(h, i, 0)
	}
	for i, b := range [...]byte{0x70, 0x42, 0x41, 0x56, 7} {
		put8(h, i, b)
	}
	put16(h, 18, uint16(p))
	put16(h, 20, uint16(p))
	put16(h, 22, uint16(vs))
	put8(h, 24, 127)
	put8(h, 25, 64)
	for k := range 128 {
		e := entries + k*16
		if k < p {
			put8(h, e, 1)
			put8(h, e+1, 127)
			put8(h, e+2, 64)
			put8(h, e+4, 64)
		}
		put16(h, e+12, 0xa000+uint16(k*2))
		put16(h, e+14, 0xa000+uint16(k*2+1))
	}
	for k := range p {
		r := rows + k*16*rowSize
		for i, b := range [...]byte{64, 0, 127, 64, 60, 0, 0, 127} {
			put8(h, r+i, b)
		}
		put16(h, r+16, adsr1)
		put16(h, r+18, adsr2)
		put16(h, r+20, uint16(k))
		put16(h, r+rowVag, 1)
	}
	sizes := rows + p*16*rowSize
	at := 0
	vb := unsafe.Pointer(&vbData[0])
	for v := 1; v <= vs; v++ {
		n := 16 + 16*(v%2)
		put16(h, sizes+v*2, uint16(n>>3))
		for i := range n {
			put8(vb, at+i, 0)
		}
		put8(vb, at+n-15, 1) // the last block's flags: the end, no repeat
		at += n
	}
}

func halfOff(h int) int {
	return entries + h/2*16 + 12 + h%2*2
}

func reg(v int16, off uintptr) uint16 {
	return volatile.LoadUint16((*uint16)(yaroze.Pointer(0x1f801c00 + uintptr(v)*16 + off)))
}

func wait(frames int) {
	for range frames {
		yaroze.VSync(0)
	}
}

func quiet() {
	yaroze.SsUtAllKeyOff(0)
	wait(3)
}

// keyOn keys program 0 tone 0 on with row 0's VAG index v and prints the result and
// the voice's start address two frames later.
func keyOn(h unsafe.Pointer, id int16, row int, v int16) {
	put16(h, row+rowVag, uint16(v))
	r := yaroze.SsUtKeyOn(id, 0, 0, 60, 0, 100, 100)
	wait(2)
	print(" r=", r)
	if r >= 0 && r < 24 {
		print(" ", word(reg(r, 6)))
	}
	quiet()
}

func utCase(h unsafe.Pointer, id int16, v int16) {
	print("ut v", v)
	keyOn(h, id, rows, v)
	if v >= 1 && v <= 256 {
		o := halfOff(int(v) - 1)
		old := get16(h, o)
		put16(h, o, 0x4000+uint16(v-1))
		print(" marked")
		keyOn(h, id, rows, v)
		put16(h, o, old)
	}
	println()
}

func seqCase(h unsafe.Pointer, s int16, v int16) {
	put16(h, rows+rowVag, uint16(v))
	var before [24]uint16
	for i := range int16(24) {
		before[i] = reg(i, 6)
	}
	yaroze.SsSeqPlay(s, 1, 1)
	wait(19)
	print("seq v", v, ":")
	found := false
	for i := range int16(24) {
		if a := reg(i, 6); a != before[i] {
			print(" v", i, " ", word(a))
			found = true
		}
	}
	if !found {
		print(" none")
	}
	println()
	yaroze.SsSeqStop(s)
	wait(3)
	quiet()
}

func utList(h unsafe.Pointer, id int16, vs int) {
	// Around the bank's own bounds: the last VAG, the end half and the marks past them.
	for v := max(vs-1, 1); v <= vs+3; v++ {
		if v != 255 {
			utCase(h, id, int16(v))
		}
	}
	for _, v := range [...]int16{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 16, 17, 254, 256, 257} {
		if int(v) < vs-1 || int(v) > vs+3 {
			utCase(h, id, v)
		}
	}
}

// bankCase opens a bank of p programs and vs VAGs, reports what the transfer wrote,
// plays each index and closes it.
func bankCase(p, vs int) {
	h := unsafe.Pointer(&vhA[0])
	build(h, p, vs, 0x80ff, 0x5fdf)
	id := yaroze.SsVabTransfer((*uint8)(h), (*uint8)(unsafe.Pointer(&vbData[0])), -1, 1)
	print("bank ", p, " ", vs, " id ", id, ":")
	marks := 0
	for i := range 256 {
		w := get16(h, halfOff(i))
		if w == 0xa000+uint16(i) {
			marks++
		} else {
			print(" ", i, "=", word(w))
		}
	}
	println(" marks left ", marks)
	if id < 0 {
		return
	}
	quiet()
	utList(h, id, vs)
	yaroze.SsVabClose(id)
}

func seqBank(p, vs int) {
	h := unsafe.Pointer(&vhA[0])
	build(h, p, vs, 0x80ff, 0x5fdf)
	id := yaroze.SsVabTransfer((*uint8)(h), (*uint8)(unsafe.Pointer(&vbData[0])), -1, 1)
	s := yaroze.SsSeqOpen(&plain[0], id)
	println("seqbank", p, vs, "id", id, "seq", s)
	quiet()
	for _, v := range [...]int16{1, 2, 3, 5, 6, 7, 9, 14, 15, 16, 17} {
		seqCase(h, s, v)
	}
	yaroze.SsSeqClose(s)
	yaroze.SsVabClose(id)
}

// countsCase changes the header's counts after the transfer.
func countsCase() {
	h := unsafe.Pointer(&vhA[0])
	build(h, 1, 14, 0x80ff, 0x5fdf)
	id := yaroze.SsVabTransfer((*uint8)(h), (*uint8)(unsafe.Pointer(&vbData[0])), -1, 1)
	quiet()
	println("counts: as transferred")
	for _, v := range [...]int16{2, 3, 10, 15, 16, 17} {
		utCase(h, id, v)
	}
	put16(h, 18, 8)
	put16(h, 22, 1)
	println("counts: 8 programs and 1 VAG after")
	for _, v := range [...]int16{2, 3, 10, 15, 16, 17} {
		utCase(h, id, v)
	}
	yaroze.SsVabClose(id)

	build(h, 8, 3, 0x80ff, 0x5fdf)
	id = yaroze.SsVabTransfer((*uint8)(h), (*uint8)(unsafe.Pointer(&vbData[0])), -1, 1)
	quiet()
	put16(h, 18, 1)
	put16(h, 22, 14)
	println("counts: 8 and 3, then 1 program and 14 VAGs after")
	for _, v := range [...]int16{2, 3, 5, 10, 16, 17} {
		utCase(h, id, v)
	}
	yaroze.SsVabClose(id)
}

// twoBanks has two banks open at once, the first with VAGs past its programs and
// the second with programs past its VAGs.
func twoBanks() {
	a, b := unsafe.Pointer(&vhA[0]), unsafe.Pointer(&vhB[0])
	build(a, 1, 14, 0x80ff, 0x5fdf)
	ia := yaroze.SsVabTransfer((*uint8)(a), (*uint8)(unsafe.Pointer(&vbData[0])), -1, 1)
	build(b, 3, 4, 0x80ff, 0x5fdf)
	ib := yaroze.SsVabTransfer((*uint8)(b), (*uint8)(unsafe.Pointer(&vbData[0])), -1, 1)
	println("two: ids", ia, ib)
	quiet()
	for _, v := range [...]int16{1, 3, 6, 7, 10, 15, 16} {
		print("first")
		utCase(a, ia, v)
		print("second")
		utCase(b, ib, v)
	}
	yaroze.SsVabClose(ib)
	yaroze.SsVabClose(ia)
}

// voiceCase keys VAG v on seven times, one a frame, waits, keys again and prints the
// voices. The ADSR is a slow fall, so a voice whose sample doesn't end takes about
// 270 frames to go quiet.
func voiceCase(v int16, w int) {
	h := unsafe.Pointer(&vhA[0])
	build(h, 1, 14, 0x80ff, 0xcf28)
	id := yaroze.SsVabTransfer((*uint8)(h), (*uint8)(unsafe.Pointer(&vbData[0])), -1, 1)
	quiet()
	put16(h, rows+rowVag, uint16(v))
	print("voice v", v, " wait ", w, ":")
	for range 7 {
		print(" ", yaroze.SsUtKeyOn(id, 0, 0, 60, 0, 100, 100))
		wait(1)
	}
	wait(w)
	print(" after: ", yaroze.SsUtKeyOn(id, 0, 0, 60, 0, 100, 100), " env")
	for i := range int16(8) {
		print(" ", word(reg(i, 0xc)))
	}
	println()
	quiet()
	wait(60)
	yaroze.SsVabClose(id)
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	for i, b := range plainBytes {
		volatile.StoreUint8((*uint8)(unsafe.Add(unsafe.Pointer(&plain[0]), i)), b)
	}
	yaroze.SsSetMVol(127, 127)
	voiceCase(1, 240)
	voiceCase(9, 20)
	voiceCase(9, 240)
	voiceCase(14, 240)
	for _, c := range [...][2]int{
		{1, 0}, {1, 1}, {1, 2}, {1, 3}, {1, 14}, {1, 254},
		{2, 3}, {2, 4}, {2, 5}, {2, 9},
		{3, 4}, {3, 6}, {3, 7},
		{4, 20},
	} {
		bankCase(c[0], c[1])
	}
	for _, c := range [...][2]int{{1, 14}, {3, 4}} {
		seqBank(c[0], c[1])
	}
	countsCase()
	twoBanks()
	println("END")
}
