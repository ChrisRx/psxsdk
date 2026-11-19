//go:build yaroze

// Command tempo0b is the tempo0 probe's cases after the tempo meta event
// of 0, which stops Sony's library before they run there: ONE (a note at
// tick 48) with its header's tempo bytes set to 0 before SsSeqOpen, then
// SsSetTempo(id, 0, 0) before SsSeqPlay, then a control. Each case prints
// the frame, from SsSeqPlay, in which a voice's registers first changed
// (-1 if never in 150), and the voices that changed.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

var (
	vhData  [(3616 + 2047) / 2048 * 512]uint32
	vbData  [(1008 + 2047) / 2048 * 512]uint32
	oneData [512]uint32

	vh  = disc.Register("ASSETS/PROBE.VH", "assets/PROBE.VH").Into(vhData[:])
	vb  = disc.Register("ASSETS/PROBE.VB", "assets/PROBE.VB").Into(vbData[:])
	one = disc.Register("ASSETS/ONE.SEQ", "assets/ONE.SEQ").Into(oneData[:])

	vab int16
	n   int
)

func reg(v int16, off uintptr) uint16 {
	return volatile.LoadUint16((*uint16)(yaroze.Pointer(0x1f801c00 + uintptr(v)*16 + off)))
}

var hex = "0123456789abcdef"

func word(w uint16) {
	var b [4]byte
	for i := range b {
		b[i] = hex[w>>(12-4*i)&15]
	}
	print(string(b[:]))
}

type regs [6]uint16

var offs = [6]uintptr{0, 2, 4, 6, 8, 0xa}

func read(v int16) (r regs) {
	for i, off := range offs {
		r[i] = reg(v, off)
	}
	return r
}

type snapshot [24]regs

func snap() (s snapshot) {
	for v := range int16(24) {
		s[v] = read(v)
	}
	return s
}

func changed(s *snapshot) bool {
	for v := range int16(24) {
		if read(v) != s[v] {
			return true
		}
	}
	return false
}

func printChanged(s *snapshot) {
	any := false
	for v := range int16(24) {
		r := read(v)
		if r == s[v] {
			continue
		}
		any = true
		print(" v", v, ":")
		for _, w := range r {
			print(" ")
			word(w)
		}
		print(" env ", reg(v, 0xc) != 0)
	}
	if !any {
		print(" none")
	}
}

func sounding() bool {
	for v := int16(0); v < 24; v++ {
		if reg(v, 0xc) != 0 {
			return true
		}
	}
	return false
}

func quiet() {
	yaroze.SsUtAllKeyOff(0)
	for n := 0; sounding() && n < 60; n++ {
		yaroze.VSync(0)
	}
	for range 4 {
		yaroze.VSync(0)
	}
}

// tempoCase plays s after setup and prints the frame in which a voice's
// registers first changed, then stops it.
func tempoCase(label string, s int16, setup func()) {
	println("t", n)
	before := snap()
	if setup != nil {
		setup()
	}
	yaroze.SsSeqPlay(s, 1, 1)
	first := -1
	for f := range 150 {
		if first < 0 && changed(&before) {
			first = f
		}
		yaroze.VSync(0)
	}
	print("tempo ", n, " ", label, ": first change at frame ", first, ":")
	printChanged(&before)
	println()
	n++
	yaroze.SsSeqStop(s)
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
	println("vab", vab)
	yaroze.SsUtAllKeyOff(0)
	quiet()

	h := unsafe.Slice((*byte)(unsafe.Pointer(&oneData[0])), 32)
	for i := 0x0a; i < 0x0d; i++ {
		volatile.StoreUint8(&h[i], 0)
	}
	s := yaroze.SsSeqOpen(&oneData[0], vab)
	println("t", n, "opened", s)
	tempoCase("ONE with header tempo 0", s, nil)
	for i, b := range []byte{0x07, 0xa1, 0x20} {
		volatile.StoreUint8(&h[0x0a+i], b)
	}
	s2 := yaroze.SsSeqOpen(&oneData[0], vab)
	println("t", n, "opened", s2)
	tempoCase("ONE, SsSetTempo 0 before the play", s2, func() { yaroze.SsSetTempo(s2, 0, 0) })
	tempoCase("ONE, control", s2, nil)
	println("done")
	for {
		yaroze.VSync(0)
	}
}
