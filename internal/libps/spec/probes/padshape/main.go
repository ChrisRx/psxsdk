//go:build yaroze

// padshape times, in system clock cycles, copies of the clean vertical blank
// handler's stretches as the compiler lays them out, for spec 46: on a
// console the stamped image's stretches before the callback (the pad stores
// around the I_STAT and I_MASK reads, the acknowledgment and the count) run
// some 100 cycles over the emulator's model, and the envelope read loop some
// 130, where the probes of single patterns (cpucost4, cpucost5, padcost) have
// the model right to a cycle or two warm. Each shape here is the handler's
// code copied from exp/cleanlib, timed between two reads of timer 2 with
// interrupts off, six times with the instruction cache flushed before each
// (cold, as the handler runs at every blank) and six times warm; and each
// has variants with one thing changed: the pad stores as nops, the stores
// to a word beside the data they interleave with, the envelope reads of one
// voice, the read's store left out. It prints a line a shape and run kind:
// its name, cold or warm, and the six times, then END, and returns. "empty"
// is the two reads alone.
package main

import (
	"device"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const reps = 6

var (
	spacer uint32
	count  uint32
	reads  [24]uint16
	// near is a word right after reads, so that a store to it stays in
	// reads' DRAM row.
	near uint32
)

func timer() uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(0x1f801120))) }

func reg32(a uintptr) *uint32 { return (*uint32)(unsafe.Pointer(a)) }
func reg16(a uintptr) *uint16 { return (*uint16)(unsafe.Pointer(a)) }

var (
	iStat = reg32(0x1f801070)
	iMask = reg32(0x1f801074)
)

// space, nops and spuGet are fidelity.go's and spu.go's.
func space(n int) {
	for range n {
		volatile.StoreUint32(&spacer, 0)
	}
}

func spaceNear(n int) {
	for range n {
		volatile.StoreUint32(&near, 0)
	}
}

//go:inline
func nops(n int) {
	for range n {
		device.Asm("nop")
	}
}

func spuGet(off uint32) uint16 { return volatile.LoadUint16(reg16(uintptr(0x1f801c00 + off))) }

//go:noinline
func empty() uint32 {
	a := timer()
	b := timer()
	return (b - a) & 0xffff
}

// preCount is libps_interrupt's and vblankInterrupt's work up to the count:
// space(5), the I_STAT and I_MASK reads, space(3), the acknowledgment (of
// nothing: all ones), space(19), the count.
//
//go:noinline
func preCount() uint32 {
	a := timer()
	space(5)
	stat := volatile.LoadUint32(iStat) & volatile.LoadUint32(iMask)
	space(3)
	volatile.StoreUint32(iStat, 0xffffffff|stat)
	space(19)
	volatile.StoreUint32(&count, volatile.LoadUint32(&count)+1)
	b := timer()
	return (b - a) & 0xffff
}

//go:noinline
func preCountNops() uint32 {
	a := timer()
	nops(5)
	stat := volatile.LoadUint32(iStat) & volatile.LoadUint32(iMask)
	nops(3)
	volatile.StoreUint32(iStat, 0xffffffff|stat)
	nops(19)
	volatile.StoreUint32(&count, volatile.LoadUint32(&count)+1)
	b := timer()
	return (b - a) & 0xffff
}

// readLoop is soundTick's envelope read loop: 24 reads, 10 stores between.
//
//go:noinline
func readLoop() uint32 {
	a := timer()
	r := &reads
	for v := range 24 {
		r[v] = spuGet(uint32(v*16 + 0x0c))
		space(10)
	}
	b := timer()
	return (b - a) & 0xffff
}

//go:noinline
func readLoopNops() uint32 {
	a := timer()
	r := &reads
	for v := range 24 {
		r[v] = spuGet(uint32(v*16 + 0x0c))
		nops(10)
	}
	b := timer()
	return (b - a) & 0xffff
}

//go:noinline
func readLoopNear() uint32 {
	a := timer()
	r := &reads
	for v := range 24 {
		r[v] = spuGet(uint32(v*16 + 0x0c))
		spaceNear(10)
	}
	b := timer()
	return (b - a) & 0xffff
}

//go:noinline
func readLoopVoice0() uint32 {
	a := timer()
	r := &reads
	for v := range 24 {
		r[v] = spuGet(0x0c)
		space(10)
	}
	b := timer()
	return (b - a) & 0xffff
}

//go:noinline
func readLoopNoStore() uint32 {
	a := timer()
	var sum uint16
	for v := range 24 {
		sum += spuGet(uint32(v*16 + 0x0c))
		space(10)
	}
	b := timer()
	volatile.StoreUint16(&reads[0], sum)
	return (b - a) & 0xffff
}

// readLoopNoSPU is the loop with a RAM load in the SPU read's place.
//
//go:noinline
func readLoopNoSPU() uint32 {
	a := timer()
	r := &reads
	for v := range 24 {
		r[v] = uint16(volatile.LoadUint32(&count) + uint32(v))
		space(10)
	}
	b := timer()
	return (b - a) & 0xffff
}

// tail is libps_interrupt's end: space(6), then what the kernel's way out
// starts with can't be copied; the counter read stands for its first load.
//
//go:noinline
func tail() uint32 {
	a := timer()
	space(6)
	b := timer()
	return (b - a) & 0xffff
}

//go:noinline
func tailNops() uint32 {
	a := timer()
	nops(6)
	b := timer()
	return (b - a) & 0xffff
}

// evictCode is 4 KB of nops and a return: run through, it takes every line of
// the instruction cache, so that what runs next misses, as the handler does at
// a blank. The kernel's FlushCache would do on a console, but the emulator's
// cost model keeps its lines through it.
var evictCode [1026]uint32

func evict() {
	var f func()
	fv := (*[2]uintptr)(unsafe.Pointer(&f))
	fv[0], fv[1] = 0, uintptr(unsafe.Pointer(&evictCode[0]))
	f()
}

func show(name string, f func() uint32) {
	var cold, warm [reps]uint32
	for i := range reps {
		evict()
		cold[i] = f()
	}
	for i := range reps {
		warm[i] = f()
	}
	print(name, " cold:")
	for _, t := range cold {
		print(" ", t)
	}
	println()
	print(name, " warm:")
	for _, t := range warm {
		print(" ", t)
	}
	println()
}

func main() {
	println("padshape")
	yaroze.ResetGraph(0)
	evictCode[1024] = 0x03e00008 // jr ra
	yaroze.FlushCache()
	mode := (*uint32)(yaroze.Pointer(0x1f801124))
	saved := volatile.LoadUint32(mode)
	yaroze.EnterCriticalSection()
	volatile.StoreUint32(mode, 0) // system clock, free running
	show("empty", empty)
	show("preCount", preCount)
	show("preCountNops", preCountNops)
	show("readLoop", readLoop)
	show("readLoopNops", readLoopNops)
	show("readLoopNear", readLoopNear)
	show("readLoopVoice0", readLoopVoice0)
	show("readLoopNoStore", readLoopNoStore)
	show("readLoopNoSPU", readLoopNoSPU)
	show("tail", tail)
	show("tailNops", tailNops)
	volatile.StoreUint32(mode, saved)
	yaroze.ExitCriticalSection()
	println("END")
}
