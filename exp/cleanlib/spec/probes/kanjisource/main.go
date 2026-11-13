//go:build yaroze

// Command kanjisource is a probe for spec 51: where the pixels of a Kanji glyph
// cell come from when no character of the font fills it. It returns, so it runs
// on a console too.
//
// Part 1 calls Krom2Tim and Krom2Tim2 with one code each and names what the
// cell holds: nothing, the 30 ROM bytes just before the non-kanji table
// (BFC65FE2h), those just before the kanji table (BFC69D4Ah), or something
// else, which is a glyph or what Krom2Tim2 writes for a code it has no glyph
// for. The ROM bytes are read by the probe itself and printed first.
//
// Before anything else it counts the words below sp that are not 0, the 96,000
// bytes part 2 fills: what a program finds on its stack.
//
// Part 2 writes a pattern over the stack below sp, each word holding its own
// place, flushes a Kanji stream and reads the glyph image back from video
// memory: a cell that holds pattern words was loaded from stack memory the
// flush never wrote, and the words say from where below the caller's sp.
package main

import (
	"device"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var hex = "0123456789abcdef"

func word(w uint32) {
	for i := range 8 {
		d := w >> (28 - 4*i) & 15
		print(hex[d : d+1])
	}
}

var buf [128]byte

func cstr(s string) *byte {
	buf = [128]byte{}
	copy(buf[:], s)
	return &buf[0]
}

const (
	romNonKanjiBefore = 0xbfc65fe2
	romKanjiBefore    = 0xbfc69d4a
)

// nibbles is a glyph row's byte as a word of 4-bit pixels.
func nibbles(b byte) uint32 {
	var w uint32
	for k := range 8 {
		if b&(0x80>>k) != 0 {
			w |= 1 << (4 * k)
		}
	}
	return w
}

func rom(addr uintptr, i int) byte {
	return *(*byte)(yaroze.Pointer(addr + uintptr(i)))
}

var tim [256]uint32

// cellIs reports whether the one-cell TIM in tim holds the glyph at addr, or a
// blank cell for addr 0.
func cellIs(addr uintptr) bool {
	for r := range 16 {
		var hi, lo byte
		if addr != 0 && r < 15 {
			hi, lo = rom(addr, 2*r), rom(addr, 2*r+1)
		}
		if tim[16+2*r] != nibbles(hi) || tim[17+2*r] != nibbles(lo) {
			return false
		}
	}
	return true
}

var codes = [...]uint16{
	0x8140, 0x81ad, 0x81b0, 0x81c0, 0x81fd, 0x8200, 0x823f, 0x8240, 0x8259, 0x827f,
	0x82f2, 0x82fd, 0x8397, 0x8461, 0x84be, 0x84bf,
	0x889f, 0x8900, 0x893f, 0x897f, 0x89fd, 0x9873, 0x8540,
}

func krom(two bool) {
	for i := 0; i < len(codes); i++ {
		c := codes[i]
		for k := 0; k < len(tim); k++ {
			tim[k] = 0x5a5a5a5a
		}
		buf = [128]byte{}
		buf[0], buf[1] = byte(c>>8), byte(c)
		var r int32
		if two {
			r = yaroze.Krom2Tim2(&buf[0], &tim[0], 640, 0, 0, 480, 0x7fff, 0)
			print("Krom2Tim2 ")
		} else {
			r = yaroze.Krom2Tim(&buf[0], &tim[0], 640, 0, 0, 480, 0x7fff, 0)
			print("Krom2Tim ")
		}
		word(uint32(c))
		print(" -> ", r, " ")
		switch {
		case tim[0] == 0x5a5a5a5a:
			println("nothing written")
		case cellIs(0):
			println("blank")
		case cellIs(romNonKanjiBefore):
			println("the 30 bytes before the non-kanji table")
		case cellIs(romKanjiBefore) && c != 0x84be:
			println("the 30 bytes before the kanji table")
		default:
			// A glyph, or what Krom2Tim2 writes for a code it has no glyph for.
			h := uint32(2166136261)
			for k := 16; k < 48; k++ {
				h = (h ^ tim[k]) * 16777619
			}
			print("other, hash ")
			word(h)
			println()
		}
	}
}

const (
	patTop   = 0xa5000000
	patWords = 24000 // 96,000 bytes below sp
	patSkip  = 64    // words right below sp left alone
)

var (
	paintSP uint32
	img     [16 * 2 * 16 * 4]uint32
)

// paint fills the stack from 256 bytes below its own sp to 96,000 below with
// words that hold their distance below paintSP in words.
//
//go:noinline
func paint() {
	sp := uint32(device.AsmFull(".set push\n.set noat\nmove {}, $$29\n.set pop", nil))
	// The range has to lie well above the program and its heap.
	if sp-4*patWords < uint32(uintptr(unsafe.Pointer(&img[0])))+uint32(unsafe.Sizeof(img))+0x20000 {
		paintSP = 0
		print("(the stack is too near the program: not filled) ")
		return
	}
	paintSP = sp &^ 3
	for i := uint32(patSkip); i < patWords; i++ {
		*(*uint32)(yaroze.Pointer(uintptr(paintSP - 4*i))) = patTop | i
	}
}

// held counts the words of the range paint fills that are not 0, for what the
// stack holds when the program starts.
//
//go:noinline
func held() int {
	sp := uint32(device.AsmFull(".set push\n.set noat\nmove {}, $$29\n.set pop", nil)) &^ 3
	n := 0
	if sp-4*patWords < uint32(uintptr(unsafe.Pointer(&img[0])))+uint32(unsafe.Sizeof(img))+0x20000 {
		return -1
	}
	for i := uint32(patSkip); i < patWords; i++ {
		if *(*uint32)(yaroze.Pointer(uintptr(sp - 4*i))) != 0 {
			n++
		}
	}
	return n
}

// flush prints text to a new stream, flushes it and reads the image's rows
// back. It returns its own sp, the flush's caller's.
//
//go:noinline
func flush(text string, rows int32) uint32 {
	sp := uint32(device.AsmFull(".set push\n.set noat\nmove {}, $$29\n.set pop", nil))
	yaroze.KanjiFntClose()
	id := yaroze.KanjiFntOpen(8, 8, 300, 100, 768, 256, 768, 504, 0, 64)
	n := yaroze.KanjiFntPrint(cstr(text))
	ok := yaroze.KanjiFntFlush(id) != nil
	yaroze.DrawSync(0)
	print(" id ", id, " print ", n, " flush ", ok, "\n")
	for i := 0; i < len(img); i++ {
		img[i] = 0x5a5a5a5a
	}
	r := yaroze.RECT{X: 768, Y: 256, W: 64, H: int16(16 * rows)}
	yaroze.StoreImage(&r, &img[0])
	yaroze.DrawSync(0)
	return sp
}

// cells says what each of the image's cells holds, 16 to a row of 32 words.
func cells(n int, sp uint32) {
	for c := 0; c < n; c++ {
		pat, zero, other := 0, 0, 0
		var lo, hi uint32
		for r := 0; r < 16; r++ {
			for k := 0; k < 2; k++ {
				w := img[(c/16*16+r)*32+c%16*2+k]
				switch {
				case paintSP != 0 && w&0xff000000 == patTop && w&0xffffff < patWords:
					// The word's address, as bytes below the flush's caller's sp.
					below := sp - (paintSP - 4*(w&0xffffff))
					if pat == 0 || below > hi {
						hi = below
					}
					if pat == 0 || below < lo {
						lo = below
					}
					pat++
				case w == 0:
					zero++
				default:
					other++
				}
			}
		}
		print("  cell ", c, ": stack ", pat, " zero ", zero, " other ", other)
		if pat > 0 {
			print(" from sp-", hi, " to sp-", lo)
		}
		if other > 0 {
			// A hash of the cell, to tell one glyph from another.
			h := uint32(2166136261)
			for r := 0; r < 16; r++ {
				for k := 0; k < 2; k++ {
					h = (h ^ img[(c/16*16+r)*32+c%16*2+k]) * 16777619
				}
			}
			print(" hash ")
			word(h)
		}
		if pat > 0 && pat < 32 {
			print(" rows:")
			for r := 0; r < 16; r++ {
				print(" ")
				word(img[(c/16*16+r)*32+c%16*2])
				print(".")
				word(img[(c/16*16+r)*32+c%16*2+1])
			}
		}
		println()
	}
}

const kan = "\x82\xa0"

func rep(s string, n int) string {
	r := ""
	for range n {
		r += s
	}
	return r
}

func stream(label, text string, n int, painted bool) {
	rect := yaroze.RECT{X: 768, Y: 256, W: 64, H: 64}
	yaroze.ClearImage(&rect, 0x50, 0x50, 0x50)
	yaroze.DrawSync(0)
	if painted {
		paint()
	}
	print("flush ", label, ":")
	rows := int32((n-1)/16 + 1)
	sp := flush(text, rows)
	cells(int(rows)*min(n, 16), sp)
}

func main() {
	// -1: the stack is too near the program for the range to be read.
	print("stack below sp at the start: ", held(), " of ", patWords-patSkip, " words not 0\n")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)

	print("rom before non-kanji:")
	for i := range 30 {
		print(" ", hex[rom(romNonKanjiBefore, i)>>4:][:1], hex[rom(romNonKanjiBefore, i)&15:][:1])
	}
	print("\nrom before kanji:")
	for i := range 30 {
		print(" ", hex[rom(romKanjiBefore, i)>>4:][:1], hex[rom(romKanjiBefore, i)&15:][:1])
	}
	println()
	krom(false)
	krom(true)

	// The image has a cell for each character and one more.
	stream("1 char", "A", 2, true)
	stream("no char", "\n", 1, true)
	stream("3 chars", kan+kan+kan, 4, true)
	stream("16 chars", rep(kan, 16), 17, true)
	stream("20 chars", rep(kan, 20), 21, true)
	stream("empty cell 81ad between two", kan+"\x81\xad"+kan, 4, true)
	stream("1 char, stack as the last flush left it", "A", 2, false)
	stream("3 chars, the same", kan+kan+kan, 4, false)
	println("END")
	// A moment before the return, for a harness that looks for the END now
	// and then: the return prints more.
	for i := 0; i < 200000; i++ {
		spin.Set(uint32(i))
	}
}

var spin volatile.Register32
