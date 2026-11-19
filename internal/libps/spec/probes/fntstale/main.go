//go:build yaroze

// Command fntstale is a probe for the spec: a stream opened over a text
// pool that still holds an earlier stream's text (FntLoad does not clear
// the pool), flushed before any print, after an empty print, and twice;
// and the byte a cut or refused FntPrint leaves at the stream's limit,
// read back through a tilde that skips the terminator. Each case is
// preceded by a GP1 marker (10000100h plus the case number) for the GPU
// harness and prints what the calls returned and the packets each flush
// built over the TTY, one primitive a line, as hex words (the tag's
// address field left out).
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const hexDigits = "0123456789abcdef"

func hex(w uint32) string {
	var b [8]byte
	for i := 7; i >= 0; i-- {
		b[i] = hexDigits[w&15]
		w >>= 4
	}
	return string(b[:])
}

// dump prints the packet list from head, "nil" for none.
func dump(head *uint32) {
	if head == nil {
		println("  nil")
		return
	}
	p := head
	for n := 0; n < 200 && p != nil; n++ {
		tag := *p
		size := int(tag >> 24)
		line := "  " + hex(tag)[:2]
		for k := 1; k <= size; k++ {
			line += " " + hex(*(*uint32)(unsafe.Add(unsafe.Pointer(p), 4*k)))
		}
		println(line)
		next := tag & 0xffffff
		if next == 0xffffff {
			break
		}
		p = (*uint32)(unsafe.Pointer(uintptr(0x80000000 | next)))
	}
}

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
	println("case", n)
}

// flush flushes id and prints what came back.
func flush(id int32) {
	head := yaroze.FntFlush(id)
	yaroze.DrawSync(0)
	println("  flush", id)
	dump(head)
}

// open loads the font again (closing every stream, which puts the next
// stream at the start of the pool) and opens one 320 by 240 stream at
// the origin holding n characters.
func open(n int32) {
	yaroze.FntLoad(960, 256)
	println("  open", yaroze.FntOpen(0, 0, 320, 240, 0, n))
	yaroze.DrawSync(0)
}

func print(s string) {
	println("  print", yaroze.Print(s))
}

// known puts "0123456789abcdef" in the pool and flushes it, so that the
// pool holds known bytes after the first (a flush ends the text at its
// start).
func known() {
	open(64)
	print("0123456789abcdef")
	flush(-1)
}

// readback opens a stream of 64 over the pool and prints "ABCDEF~": the
// tilde skips the terminator at 7, so the flush goes on to draw what the
// pool holds from byte 8.
func readback() {
	open(64)
	print("ABCDEF~")
	flush(-1)
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)

	// 0: a stream over a pool whose text was flushed, flushed before
	// any print, then again, then after an empty print.
	mark(0)
	open(64)
	print("ABCD")
	flush(-1)
	open(64)
	flush(-1)
	flush(-1)
	print("")
	flush(-1)

	// 1: a stream over a pool holding text that was never flushed.
	mark(1)
	open(64)
	print("ABCD")
	open(64)
	flush(-1)
	flush(-1)

	// 2: the same under a stream of 8, shorter than the stale text.
	mark(2)
	open(64)
	print("0123456789abcdef")
	open(8)
	flush(-1)
	flush(-1)

	// 3: the same with the empty string printed before the flush.
	mark(3)
	open(64)
	print("0123456789abcdef")
	open(64)
	print("")
	flush(-1)

	// 4: a second stream, opened without FntLoad, over stale text from
	// an earlier stream that reached its part of the pool.
	mark(4)
	open(512)
	print("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdefGHIJKLMN")
	open(64)
	print("AB")
	println("  open", yaroze.FntOpen(0, 0, 320, 240, 0, 64))
	yaroze.DrawSync(0)
	flush(1)
	flush(0)
	flush(1)

	// 5: the byte at the limit after a cut print.
	mark(5)
	known()
	open(8)
	print("ABCDEFGHIJ")
	flush(-1)
	readback()

	// 6: after a whole print then a refused one.
	mark(6)
	known()
	open(8)
	print("ABCDEFGH")
	print("XY")
	flush(-1)
	readback()

	// 7: after a whole print alone.
	mark(7)
	known()
	open(8)
	print("ABCDEFGH")
	flush(-1)
	readback()

	// 8: after a whole print then a refused empty one.
	mark(8)
	known()
	open(8)
	print("ABCDEFGH")
	print("")
	flush(-1)
	readback()

	// 9: a print cut in its second call.
	mark(9)
	known()
	open(8)
	print("ABCDE")
	print("FGHIJ")
	flush(-1)
	readback()

	// 10: whether the limit or the count bounds the flush: a stream of 8
	// holding 7 whose tilde skips the terminator into the stale pool.
	mark(10)
	known()
	open(8)
	print("ABCDEF~")
	flush(-1)

	// 11: the same with 8 held, the tilde last.
	mark(11)
	known()
	open(8)
	print("ABCDEFG~")
	flush(-1)

	// 12: the byte a cut print leaves at the limit is the next stream's
	// first.
	mark(12)
	known()
	open(8)
	println("  open", yaroze.FntOpen(0, 0, 320, 240, 0, 64))
	yaroze.DrawSync(0)
	print("ABCDEFGHIJ")
	flush(1)
	flush(0)

	// 13: how the limit counts a tilde escape: three escapes in a stream
	// of 8, the last skipping the terminator into the stale pool.
	mark(13)
	known()
	open(8)
	print("~a~b~d~")
	flush(-1)

	// 14: an empty print after a cut: what it returns and where its
	// terminator goes.
	mark(14)
	known()
	open(8)
	print("ABCDEFGHIJ")
	print("")
	flush(-1)
	readback()

	// 15: a refused print after a cut: where its byte goes.
	mark(15)
	known()
	open(8)
	print("ABCDEFGHIJ")
	print("XY")
	flush(-1)
	readback()

	mark(16)
	println("done")
	for {
		yaroze.VSync(0)
	}
}
