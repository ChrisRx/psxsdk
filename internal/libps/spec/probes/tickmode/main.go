//go:build yaroze

// Command tickmode is a probe for the spec: what SsSetTickMode does to a
// sequence's timing, for each mode under each video mode. On the voices
// probe's bank, ONE (a note at tick 48) plays, and each case prints the frame,
// from SsSeqPlay, in which a voice's registers first changed (-1 if never in
// 300). The first part sets the mode on a sequence already open, the second
// before SsSeqOpen, and the third while a sequence plays. The mode's own
// effect on a tempo set later comes last.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

//export SsSetTickMode
func ssSetTickMode(mode int32)

var (
	vhData  [(3616 + 2047) / 2048 * 512]uint32
	vbData  [(1008 + 2047) / 2048 * 512]uint32
	oneData [512]uint32

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
	s := yaroze.SsSeqOpen(&oneData[0], vab)
	println("vab", vab, "seq", s, "video", yaroze.GetVideoMode())
	quiet()

	modes := [...]int32{0, 1, 2, 3, 4, 5, 6, 30, 50, 60, 120, 240, 0x1000, 0x1001, 0x1005, 0x101e, 0x103c, -1}
	for _, vm := range [...]int32{0, 1} {
		prev := yaroze.SetVideoMode(vm)
		println("SetVideoMode", vm, "returned", prev)
		yaroze.SsSeqClose(s)
		for _, m := range modes {
			ssSetTickMode(m)
			o := yaroze.SsSeqOpen(&oneData[0], vab)
			f := play(o)
			println("video", vm, "open after mode", m, ": seq", o, "first change at frame", f)
			yaroze.SsSeqStop(o)
			yaroze.SsSeqClose(o)
			quiet()
		}
		for _, m := range modes {
			ssSetTickMode(5)
			o := yaroze.SsSeqOpen(&oneData[0], vab)
			ssSetTickMode(m)
			f := play(o)
			println("video", vm, "open, then mode", m, ": first change at frame", f)
			yaroze.SsSeqStop(o)
			yaroze.SsSeqClose(o)
			quiet()
		}
		for _, m := range [...]int32{1, 2, 3, 4, 60} {
			ssSetTickMode(5)
			o := yaroze.SsSeqOpen(&oneData[0], vab)
			ssSetTickMode(m)
			yaroze.SsSetTempo(o, 0, 120)
			f := play(o)
			println("video", vm, "open at 5, mode", m, ", SsSetTempo 120: first change at frame", f)
			yaroze.SsSeqStop(o)
			yaroze.SsSeqClose(o)
			quiet()
		}
		s = yaroze.SsSeqOpen(&oneData[0], vab)
	}
	println("done")
}
