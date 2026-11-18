//go:build yaroze

// Command timnull is a probe for spec 37: GsGetTimInfo with a null pointer, either
// one. Address 0 is memory on this machine. With info null the fields go to address
// 0 on; with the TIM null it is read from address 0 on. Each case prints the eight
// words at address 0 or in info.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var (
	withClut = [...]uint32{9, 44, 0x00f00100, 0x00010100, 0xaaaa5555, 0xbbbb6666, 0xcccc7777, 0xdddd8888, 0x11112222, 0x33334444, 0x55556666, 0x77778888, 28, 0x01000040, 0x00020010, 1, 2, 3, 4}
	noClut   = [...]uint32{2, 28, 0x00e00080, 0x00200010, 1, 2, 3, 4}
	info     [8]uint32
)

// low is address 0, through the mirror a nil check doesn't see.
func low(i int) *uint32 {
	return (*uint32)(unsafe.Pointer(uintptr(0x80000000) + uintptr(4*i)))
}

// Saved so that the cases leave the kernel's words as they were.
var saved [32]uint32

func save() {
	for i := range saved {
		saved[i] = volatile.LoadUint32(low(i))
	}
}

func restore() {
	for i := range saved {
		volatile.StoreUint32(low(i), saved[i])
	}
}

func fill(p func(int) *uint32, n int, v uint32) {
	for i := range n {
		volatile.StoreUint32(p(i), v+uint32(i))
	}
}

func show(what string, p func(int) *uint32) {
	print(what)
	for i := range 8 {
		print(" ", volatile.LoadUint32(p(i)))
	}
	println()
}

func inInfo(i int) *uint32 { return &info[i] }

func tim(words []uint32) *uint32 { return &words[0] }

func main() {
	yaroze.ResetGraph(0)
	nilInfo := (*yaroze.GsIMAGE)(nil)
	nilTim := (*uint32)(nil)
	base := uint32(uintptr(unsafe.Pointer(&withClut[0])))
	println("palette tim at", base, "no palette tim at", uint32(uintptr(unsafe.Pointer(&noClut[0]))))

	save()
	fill(low, 8, 0x5a5a0000)
	yaroze.GsGetTimInfo(tim(withClut[:]), nilInfo)
	c := [8]uint32{}
	for i := range c {
		c[i] = volatile.LoadUint32(low(i))
	}
	fill(low, 8, 0x5a5a0000)
	yaroze.GsGetTimInfo(tim(noClut[:]), nilInfo)
	d := [8]uint32{}
	for i := range d {
		d[i] = volatile.LoadUint32(low(i))
	}
	restore()
	println("info null, palette", c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7])
	println("info null, none", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7])

	// The TIM at address 0, with and without a palette, into an ordinary info.
	for i, w := range noClut {
		volatile.StoreUint32(low(i), w)
	}
	fill(inInfo, 8, 0x5a5a0000)
	yaroze.GsGetTimInfo(nilTim, (*yaroze.GsIMAGE)(unsafe.Pointer(&info[0])))
	restore()
	show("tim null, none", inInfo)
	save()
	for i, w := range withClut {
		volatile.StoreUint32(low(i), w)
	}
	fill(inInfo, 8, 0x5a5a0000)
	yaroze.GsGetTimInfo(nilTim, (*yaroze.GsIMAGE)(unsafe.Pointer(&info[0])))
	restore()
	show("tim null, palette", inInfo)

	// Both null: the words at 0 are the TIM and then the fields over it.
	for i, w := range noClut {
		volatile.StoreUint32(low(i), w)
	}
	yaroze.GsGetTimInfo(nilTim, nilInfo)
	e := [8]uint32{}
	for i := range e {
		e[i] = volatile.LoadUint32(low(i))
	}
	restore()
	println("both null", e[0], e[1], e[2], e[3], e[4], e[5], e[6], e[7])
	println("END")
	for {
	}
}
