//go:build yaroze

// Command ssstack is a probe for spec 19: the stack the sound and text calls use on a
// program whose stack is in the scratchpad, by the method of gschain, and what the
// sequencer's interrupt takes with a sequence playing. The bank and the sequence come
// from ../seqcalls/assets.
package main

import (
	"device"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

const span = 4096

//export FntPrint
func fntPrint3(s *byte, a, b uint32) int32

var (
	vhData   [(3616 + 2047) / 2048 * 512]uint32
	vbData   [(1008 + 2047) / 2048 * 512]uint32
	bendData [2048]uint32

	vh   = disc.Register("ASSETS/PROBE2.VH", "../seqcalls/assets/PROBE2.VH").Into(vhData[:])
	vb   = disc.Register("ASSETS/PROBE.VB", "../seqcalls/assets/PROBE.VB").Into(vbData[:])
	bend = disc.Register("ASSETS/BEND.SEQ", "../seqcalls/assets/BEND.SEQ").Into(bendData[:])

	prep func()
	call func()
	vab  int16
	seq  int16
	stm  int32
)

//go:noinline
func reach() int32 {
	sp := uint32(device.AsmFull(".set push\n.set noat\nmove {}, $$29\n.set pop", nil))
	low := sp - span
	most := int32(0)
	for _, pat := range [...]uint32{0x5aa5c33c, 0xa55a3cc3} {
		if prep != nil {
			prep()
		}
		for a := low; a < sp-8; a += 4 {
			*(*uint32)(unsafe.Pointer(uintptr(a))) = pat
		}
		call()
		a := low
		for a < sp-8 && *(*uint32)(unsafe.Pointer(uintptr(a))) == pat {
			a += 4
		}
		if d := int32(sp - a); d > most {
			most = d
		}
	}
	return most
}

func run(name string, f func()) {
	call = f
	println("stack", name, reach())
	prep = nil
}

func spin() {
	for i := uint32(0); i < 200000; i++ {
		volatile.LoadUint32((*uint32)(yaroze.Pointer(0x1f801814)))
	}
}

var (
	plain = [...]byte{'h', 'e', 'l', 'l', 'o', '\n', 0}
	dfmt  = [...]byte{'%', 'd', ' ', '%', 'x', '\n', 0}
	wfmt  = [...]byte{'%', '2', '0', 'd', '\n', 0}
)

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	if f := disc.LoadAll(300); f != nil {
		println("load failed")
	}
	vab = yaroze.SsVabTransfer(vh.Bytes(), vb.Bytes(), -1, 1)
	yaroze.SsSetMVol(127, 127)
	for range 10 {
		yaroze.VSync(0)
	}
	run("empty", func() {})
	run("spin, nothing playing", spin)
	seq = yaroze.SsSeqOpen(&bendData[0], vab)
	println("vab", vab, "seq", seq)
	run("SsSeqPlay", func() { yaroze.SsSeqPlay(seq, 1, 0) })
	run("spin, playing", spin)
	run("SsSetMVol", func() { yaroze.SsSetMVol(120, 120) })
	run("SsSeqStop", func() { yaroze.SsSeqStop(seq) })
	run("SsSeqClose", func() { yaroze.SsSeqClose(seq) })
	seq = yaroze.SsSeqOpen(&bendData[0], vab)
	run("SsUtKeyOn", func() { yaroze.SsUtKeyOn(vab, 1, 0, 60, 0, 100, 100) })
	run("spin, keys on", spin)
	run("SsUtAllKeyOff", func() { yaroze.SsUtAllKeyOff(0) })
	prep = func() { yaroze.SsUtKeyOn(vab, 1, 0, 60, 0, 100, 100) }
	run("SsUtAllKeyOff, a key on", func() { yaroze.SsUtAllKeyOff(0) })

	yaroze.FntLoad(960, 256)
	stm = yaroze.FntOpen(0, 0, 320, 240, 0, 512)
	run("FntPrint text", func() { fntPrint3(&plain[0], 0, 0) })
	run("FntPrint %d %x", func() { fntPrint3(&dfmt[0], 1234, 0xbeef) })
	run("FntPrint %20d", func() { fntPrint3(&wfmt[0], 1234, 0) })
	prep = func() {
		yaroze.DrawSync(0)
		fntPrint3(&plain[0], 0, 0)
	}
	run("FntFlush", func() { yaroze.FntFlush(stm) })
	yaroze.DrawSync(0)
	println("END")
	for {
	}
}
