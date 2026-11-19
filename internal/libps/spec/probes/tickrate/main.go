//go:build yaroze

// Command tickrate is a probe for the spec: what a rate above the tempo does,
// where a tempo does not reach a tenth of a tick a blank, and the modes the
// tickmode probe leaves out. On the voices probe's bank, a copy of ONE with its
// note where the case says plays. Each case prints the frame, from SsSeqPlay, in
// which a voice's registers first changed (-1 if never in 300), and 60 frames on
// whether SsIsEos says the sequence still plays: the note off comes 48 ticks
// after the note and the end of the track at once. SetVideoMode then SsSeqOpen,
// with no SsSetTickMode between, comes last.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

//export SsSetTickMode
func ssSetTickMode(mode int32)

var (
	vhData  [(3616 + 2047) / 2048 * 512]uint32
	vbData  [(1008 + 2047) / 2048 * 512]uint32
	oneData [512]uint32

	// Copies of ONE: long is rewritten for each case, far has its note at
	// tick 4800.
	longData [8]uint32
	farData  [8]uint32

	vh  = disc.Register("ASSETS/PROBE.VH", "assets/PROBE.VH").Into(vhData[:])
	vb  = disc.Register("ASSETS/PROBE.VB", "assets/PROBE.VB").Into(vbData[:])
	one = disc.Register("ASSETS/ONE.SEQ", "assets/ONE.SEQ").Into(oneData[:])

	vab int16
)

func reg(v int16, off uintptr) uint16 {
	return volatile.LoadUint16((*uint16)(yaroze.Pointer(0x1f801c00 + uintptr(v)*16 + off)))
}

type snapshot [24][6]uint16

func snap() (s snapshot) {
	for v := range int16(24) {
		for i := range 6 {
			s[v][i] = reg(v, uintptr(i)*2)
		}
	}
	return s
}

func quiet() {
	yaroze.SsUtAllKeyOff(0)
	for n := 0; n < 40; n++ {
		yaroze.VSync(0)
	}
}

// play starts s and returns the frame of the first register change.
func play(s int16) int {
	before := snap()
	yaroze.SsSeqPlay(s, 1, 1)
	for f := range 300 {
		if snap() != before {
			return f
		}
		yaroze.VSync(0)
	}
	return -1
}

// long builds a copy of ONE in buf with its note at tick (hi&7f)<<7 | lo, which
// times a fast rate closely: the frame is that tick, in tenths, over the rate,
// plus one.
func long(buf *[8]uint32, hi, lo byte) {
	in := unsafe.Slice((*byte)(unsafe.Pointer(&oneData[0])), 29)
	out := unsafe.Slice((*byte)(unsafe.Pointer(buf)), 30)
	copy(out, in[:18])
	out[18], out[19] = hi, lo
	copy(out[20:], in[19:])
}

func run(what string, m int32, seq *uint32, tempo int16) {
	ssSetTickMode(m)
	o := yaroze.SsSeqOpen(seq, vab)
	if tempo != 0 {
		yaroze.SsSetTempo(o, 0, tempo)
	}
	f := play(o)
	for range 60 {
		yaroze.VSync(0)
	}
	println(what, "mode", m, "tempo", tempo, ": first change at frame", f, "playing after 60 more:", yaroze.SsIsEos(o, 0))
	yaroze.SsSeqStop(o)
	yaroze.SsSeqClose(o)
	quiet()
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	if f := disc.LoadAll(300); f != nil {
		println("load failed")
	}
	vab = yaroze.SsVabTransfer(vh.Bytes(), vb.Bytes(), -1, 1)
	yaroze.SsSetMVol(127, 127)
	long(&longData, 0x8c, 0x40)
	long(&farData, 0xa5, 0x40)

	// The two copies at a rate where they must play: 51 and 151.
	run("long", 6, &longData[0], 0)
	run("far", 6, &farData[0], 0)
	// Where the tempo is under a tenth of a tick a blank, by the inverse
	// 1500, 375, 213 and 3413 tenths, the note at ticks around one, two and
	// three blanks of it.
	at := func(m int32, tempo int16, ticks ...int) {
		for _, n := range ticks {
			long(&longData, 0x80|byte(n>>7), byte(n&0x7f))
			print("note at ", n, ": ")
			run("", m, &longData[0], tempo)
		}
	}
	at(2, 1, 96, 102, 103, 150, 151, 251, 252, 300, 301)
	at(5, 1, 20, 26, 27, 37, 38, 75, 76)
	at(4095, 0, 5, 21, 22, 42, 43)
	at(65536, 0, 341, 342, 682, 683)
	at(6, 0, 48)
	// The mode past 4095 with bit 12 clear, then set: rate 8192, 16 and 16.
	at(8192, 0, 48)
	at(0x3005, 0, 48)
	at(-4096, 0, 48)
	// A negative mode without bit 12, after a mode it could be ignored
	// against.
	ssSetTickMode(2)
	ssSetTickMode(-8192)
	o := yaroze.SsSeqOpen(&oneData[0], vab)
	println("mode 2 then -8192: first change at frame", play(o))
	yaroze.SsSeqStop(o)
	yaroze.SsSeqClose(o)
	quiet()
	yaroze.SetVideoMode(1)
	ssSetTickMode(2)
	ssSetTickMode(-8192)
	o = yaroze.SsSeqOpen(&oneData[0], vab)
	println("PAL, mode 2 then -8192: first change at frame", play(o))
	yaroze.SsSeqStop(o)
	yaroze.SsSeqClose(o)
	quiet()
	yaroze.SetVideoMode(0)
	// SetVideoMode after a mode of 240, with no SsSetTickMode after it.
	for _, vm := range [...]int32{1, 0} {
		ssSetTickMode(2)
		yaroze.SetVideoMode(vm)
		o = yaroze.SsSeqOpen(&oneData[0], vab)
		println("mode 2 then video", vm, ": first change at frame", play(o))
		yaroze.SsSeqStop(o)
		yaroze.SsSeqClose(o)
		quiet()
	}
	println("done")
}
