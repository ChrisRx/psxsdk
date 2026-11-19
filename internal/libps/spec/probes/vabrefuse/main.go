//go:build yaroze

// Command vabrefuse is a probe for the spec: which headers SsVabTransfer refuses
// by their counts and sizes (spec 36). Each case builds a header with ps programs, a
// VAG count of vs and the first n VAG sizes set, transfers it and prints the id, the
// halves it wrote and the next bank's first address, which shows whether sound memory
// was taken. Every id it gets is closed.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const rows = 0x820

var (
	vh   [(rows + 8*16*32 + 512) / 4]uint32
	body [1024]uint32
)

const hex = "0123456789abcdef"

func word(w uint16) string {
	var b [4]byte
	for i := range b {
		b[i] = hex[w>>(12-4*i)&15]
	}
	return string(b[:])
}

func put8(off int, v uint8) {
	volatile.StoreUint8((*uint8)(unsafe.Add(unsafe.Pointer(&vh[0]), off)), v)
}

func put16(off int, v uint16) {
	put8(off, uint8(v))
	put8(off+1, uint8(v>>8))
}

func get16(off int) uint16 {
	p := unsafe.Pointer(&vh[0])
	return uint16(volatile.LoadUint8((*uint8)(unsafe.Add(p, off)))) | uint16(volatile.LoadUint8((*uint8)(unsafe.Add(p, off+1))))<<8
}

func build(ps, vs, n int) {
	for i := range rows + 8*16*32 + 512 {
		put8(i, 0)
	}
	for i, b := range [...]byte{0x70, 0x42, 0x41, 0x56, 7} {
		put8(i, b)
	}
	put16(18, uint16(ps))
	put16(20, uint16(ps))
	put16(22, uint16(vs))
	put8(24, 127)
	put8(25, 64)
	for k := range ps {
		put8(0x20+k*16, 1)
	}
	for v := 1; v <= n; v++ {
		put16(rows+ps*16*32+v*2, 2)
	}
}

func transfer() int16 {
	return yaroze.SsVabTransfer((*uint8)(unsafe.Pointer(&vh[0])), (*uint8)(unsafe.Pointer(&body[0])), -1, 1)
}

func refuse(ps, vs, n int) {
	build(ps, vs, n)
	id := transfer()
	print("case ps ", ps, " vs ", vs, " sizes ", n, ": id ", id, " halves")
	for i := range 6 {
		print(" ", word(get16(0x20+i/2*16+12+i%2*2)))
	}
	build(1, 1, 1)
	next := transfer()
	println(" next id ", next, " at ", word(get16(0x20+12)))
	if next >= 0 {
		yaroze.SsVabClose(next)
	}
	if id >= 0 {
		yaroze.SsVabClose(id)
	}
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	for _, c := range [...][3]int{
		{1, 1, 1},
		{1, 0, 0}, {1, 0, 2}, {1, 2, 0}, {1, 2, 2}, {1, 2, 1},
		{0, 2, 2}, {0, 0, 0},
		{1, 254, 2}, {1, 255, 2}, {1, 256, 2},
		{1, 1, 1},
	} {
		refuse(c[0], c[1], c[2])
	}
	println("END")
}
