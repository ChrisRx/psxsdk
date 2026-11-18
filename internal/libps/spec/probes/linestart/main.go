//go:build yaroze

// Command linestart is a probe for spec 37: when root counter 1 starts counting
// lines. The library has started before main. It prints the counter's mode, whether
// VSync(1) and GetRCnt read the counter, the counter at the next vertical blank (lines
// since the blank after the library started), what the first waiting VSync returns,
// and whether a later ResetGraph(0), ResetGraph(1) or GsInitGraph restarts it.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var (
	count = (*uint32)(unsafe.Pointer(uintptr(0x1f801110)))
	mode  = (*uint32)(unsafe.Pointer(uintptr(0x1f801114)))
)

func lines() uint32 { return volatile.LoadUint32(count) & 0xffff }

func waitBlank() {
	from := yaroze.VSync(-1)
	for yaroze.VSync(-1) == from {
	}
}

func main() {
	yaroze.ResetGraph(0)
	c0 := lines()
	m0 := volatile.LoadUint32(mode) & 0x3ff
	v1 := yaroze.VSync(1)
	g1 := yaroze.GetRCnt(yaroze.RCntCNT1)
	c1 := lines()
	b0 := yaroze.VSync(-1)
	waitBlank()
	c2 := lines()
	m2 := volatile.LoadUint32(mode) & 0x3ff
	// The first three reads come a line or so apart by chance of phase, so only what
	// they agree on is printed, and the counter is read exactly at the next blank.
	println("in main: mode", m0, "VSync(1) reads the counter", uint32(v1)-c0 < 2, "GetRCnt reads the counter", uint32(g1)-c1 < 2, "blanks", b0)
	println("after the next blank: counter", c2, "mode", m2)
	first := yaroze.VSync(0)
	println("first VSync(0) under 300:", first < 300, "over 0:", first > 0)
	yaroze.VSync(0)
	a := lines()
	yaroze.ResetGraph(0)
	b := lines()
	println("ResetGraph(0) restarts it:", b < a)
	yaroze.VSync(0)
	a = lines()
	yaroze.ResetGraph(1)
	b = lines()
	println("ResetGraph(1) restarts it:", b < a)
	yaroze.VSync(0)
	a = lines()
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	b = lines()
	println("GsInitGraph restarts it:", b < a)
	println("END")
	for {
	}
}
