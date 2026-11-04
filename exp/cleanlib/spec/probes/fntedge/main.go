//go:build yaroze

// Command fntedge is a probe for the spec: the edges of FntPrint and
// FntFlush. A stream given more characters than it holds, a percent sign
// (two and one), printing with no stream open, a tilde not followed by c,
// characters the font lacks, flushing an id that is not open, and a
// second stream flushed by its id. Each case prints what the calls
// returned and the packets the flush built over the TTY, one primitive a
// line, as hex words (the tag's address field left out).
package main

import (
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

// flush flushes id and prints what came back.
func flush(id int32) {
	head := yaroze.FntFlush(id)
	yaroze.DrawSync(0)
	println("  flush", id)
	dump(head)
}

// open loads the font again (closing every stream) and opens one
// 320 by 240 stream at the origin holding n characters.
func open(n int32) {
	yaroze.FntLoad(960, 256)
	println("  open", yaroze.FntOpen(0, 0, 320, 240, 0, n))
	yaroze.DrawSync(0)
}

func print(s string) {
	println("  print", yaroze.Print(s))
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)

	// 0: a stream of 8 given 10, then more, then nothing, then flushed.
	println("case 0")
	open(8)
	print("ABCDEFGHIJ")
	print("KL")
	print("")
	flush(-1)
	print("MN")
	flush(-1)

	// 1: a stream of 8 filled exactly, then one more.
	println("case 1")
	open(8)
	print("ABCDEFGH")
	print("I")
	flush(-1)

	// 2: a stream of 8 filled in two prints, the second overflowing.
	println("case 2")
	open(8)
	print("ABCDE")
	print("FGHIJ")
	flush(-1)

	// 3: no stream open.
	println("case 3")
	yaroze.FntLoad(960, 256)
	print("AB")
	flush(-1)
	flush(0)

	// 4: tildes. The text pool is not cleared by FntLoad, and a tilde
	// can read past the terminator, so the pool is first given known
	// contents.
	println("case 4")
	open(64)
	print("0123456789abcdef")
	flush(-1)
	print("A~xB")
	flush(-1)
	print("A~~B")
	flush(-1)
	print("A~")
	flush(-1)
	print("A~c7")
	flush(-1)
	print("A~cB")
	flush(-1)

	// 5: characters the font lacks: controls, DEL, high bytes.
	println("case 5")
	open(64)
	print("A\x01B\x1bC\rD\bE\x0bF\x7fG\x80H\xa0I\xffJ")
	flush(-1)
	print("A\x60B\x7bC\x7eD")
	flush(-1)

	// 6: ids that are not open, with one stream holding text.
	println("case 6")
	open(64)
	print("AB")
	flush(1)
	flush(7)
	flush(-2)
	flush(0)

	// 7: two streams, the second with a background, flushed by id.
	println("case 7")
	yaroze.FntLoad(960, 256)
	println("  open", yaroze.FntOpen(0, 0, 160, 240, 0, 64))
	println("  open", yaroze.FntOpen(160, 0, 160, 240, 1, 64))
	yaroze.DrawSync(0)
	print("AB")
	flush(1)
	flush(0)
	flush(1)

	// 8: percent signs, last: a lone one is formatted with whatever is in
	// the argument registers and leaves garbage in the pool.
	println("case 8")
	open(64)
	print("A%%B")
	flush(-1)
	print("A%B")
	flush(-1)
	print("A%")
	flush(-1)

	println("done")
	for {
		yaroze.VSync(0)
	}
}
