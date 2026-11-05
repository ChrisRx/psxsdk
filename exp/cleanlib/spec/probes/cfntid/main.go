//go:build yaroze

// Command cfntid is a probe for spec 17: how FntPrint and KanjiFntPrint tell an id
// from a format pointer. The first 16 bytes of memory hold letters, so a small first
// argument read as a pointer draws its tail of them, and a1 is a pointer to "OK", so
// read as an id it draws OK. Printed: the return, and what each stream draws. The
// counts of open streams change between groups. The bytes are put back.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export FntPrint
func fntPrint(a0, a1, a2, a3 uint32) int32

const hexDigits = "0123456789abcdef"

var (
	line  [512]byte
	nline int
)

func putc(c byte) {
	if nline < len(line) {
		line[nline] = c
		nline++
	}
	if c == '\n' {
		yaroze.Write(1, &line[0], int32(nline))
		nline = 0
	}
}

func ps(s string) {
	for i := 0; i < len(s); i++ {
		putc(s[i])
	}
}

func pi(v int32) {
	if v < 0 {
		putc('-')
		v = -v
	}
	var d [12]byte
	k := 0
	for {
		d[k] = byte('0' + v%10)
		k++
		v /= 10
		if v == 0 {
			break
		}
	}
	for k > 0 {
		k--
		putc(d[k])
	}
}

func hex32(v uint32) {
	for i := range 8 {
		putc(hexDigits[v>>(28-4*i)&15])
	}
}

var ok = [...]byte{'O', 'K', 0}

func text(id int32) {
	head := yaroze.FntFlush(id)
	yaroze.DrawSync(0)
	var cs [32]byte
	n := 0
	p := head
	for k := 0; k < 40 && p != nil; k++ {
		tag := *p
		if tag>>24 == 3 && n < len(cs) {
			uv := *(*uint32)(unsafe.Add(unsafe.Pointer(p), 12))
			cs[n] = byte(int32(uv>>8&255)/8*16 + int32(uv&255)/8 + 32)
			n++
		}
		next := tag & 0xffffff
		if next == 0xffffff {
			break
		}
		p = (*uint32)(unsafe.Pointer(uintptr(0x80000000 | next)))
	}
	putc('[')
	for i := n - 1; i >= 0; i-- {
		putc(cs[i])
	}
	putc(']')
}

func one(streams int, a0 uint32) {
	yaroze.FntLoad(960, 256)
	for range streams {
		yaroze.FntOpen(0, 0, 320, 240, 0, 64)
	}
	yaroze.DrawSync(0)
	r := fntPrint(a0, uint32(uintptr(unsafe.Pointer(&ok[0]))), 0, 0)
	ps("streams ")
	pi(int32(streams))
	ps(" a0 ")
	hex32(a0)
	ps(" -> ")
	pi(r)
	ps(" ")
	for i := range int32(streams) {
		text(i)
	}
	putc('\n')
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)

	low := (*[16]byte)(yaroze.Pointer(0x80000000))
	var save [16]byte
	copy(save[:], low[:])
	copy(low[:], "ABCDEFGHIJKLMNO\x00")

	for _, streams := range [...]int{1, 2, 4, 8} {
		for a0 := uint32(0); a0 < 16; a0++ {
			one(streams, a0)
		}
	}
	copy(low[:], save[:])
	ps("END\n")
}
