//go:build yaroze

// Command cdinit is a probe for spec 18: CdInit twice, and then during a read,
// between GP1 marker words (10000100h plus the case), for an I/O trace
// (.build/iotrace, TestIO) of what it sends the drive and writes elsewhere.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var (
	cdInit     func() int32
	cdControlF func(com int32, param *uint8) int32
)

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

var pos = [4]uint8{0, 0x02, 0x16, 0}

func wait(frames int) {
	for range frames {
		yaroze.VSync(0)
	}
}

func main() {
	(*[2]uintptr)(unsafe.Pointer(&cdInit))[1] = 0x8003921c
	(*[2]uintptr)(unsafe.Pointer(&cdControlF))[1] = 0x80039618
	yaroze.ResetGraph(0)
	wait(4)
	mark(1)
	v := cdInit()
	mark(2)
	println("CdInit returns", v)
	wait(4)
	mark(3)
	v = cdInit()
	mark(4)
	println("again returns", v)
	cdControlF(2, &pos[0])
	wait(4)
	cdControlF(6, nil)
	wait(4)
	mark(5)
	v = cdInit()
	mark(6)
	println("in a read returns", v)
	println("END")
	for {
	}
}
