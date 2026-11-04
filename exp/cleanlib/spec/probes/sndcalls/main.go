//go:build yaroze

// Command sndcalls is a probe for spec 10: the sound calls outside the
// sequencer. The getters (SsGetMute, SsGetMVol, SsGetSerialAttr,
// SsGetSerialVol) after start-up and after their setters, SsSetMute with
// voices playing, SsUtSetReverbDelay and SsUtSetReverbFeedback for every
// reverb type, and SsUtChangePitch and SsUtPitchBend on voices SsUtKeyOn
// started. Each call is a step of its own frame; results are logged in
// memory and printed after each part, so that printing does not move the
// SPU writes. Built for the resident library it shows Sony's behavior.
//
// The bank, assets/PROBE2.VH and PROBE.VB, is made by
// ../seqcalls/gen.go: the voices probe's bank with program 1's tone given
// pbmin 2, pbmax 7 and shift 37.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

var (
	vhData [(3616 + 2047) / 2048 * 512]uint32
	vbData [(1008 + 2047) / 2048 * 512]uint32

	vh = disc.Register("ASSETS/PROBE2.VH", "assets/PROBE2.VH").Into(vhData[:])
	vb = disc.Register("ASSETS/PROBE.VB", "assets/PROBE.VB").Into(vbData[:])
)

var vab int16

// log holds a part's results: a tag and up to two values each.
var log [256]struct {
	tag  byte
	a, b int32
}

var logged int

func rec(tag byte, a, b int32) {
	if logged < len(log) {
		log[logged].tag, log[logged].a, log[logged].b = tag, a, b
		logged++
	}
}

func flush(part string) {
	println("part", part)
	for _, e := range log[:logged] {
		println(" ", string(rune(e.tag)), e.a, e.b)
	}
	logged = 0
	frame()
}

func frame() { yaroze.VSync(0) }

func getters() {
	var v yaroze.SndVolume
	rec('m', int32(yaroze.SsGetMute()), 0)
	yaroze.SsGetMVol(&v)
	rec('v', int32(v.Left), int32(v.Right))
	rec('a', int32(yaroze.SsGetSerialAttr(0, 0)), int32(yaroze.SsGetSerialAttr(0, 1)))
	yaroze.SsGetSerialVol(0, &v)
	rec('s', int32(v.Left), int32(v.Right))
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	// The getters before any other sound call.
	getters()
	if f := disc.LoadAll(300); f != nil {
		println("load failed")
	}
	vab = yaroze.SsVabTransfer(vh.Bytes(), vb.Bytes(), -1, 1)
	getters()
	flush("start")
	for range 30 {
		frame()
	}
	println("start")
	frame()

	// Main volume.
	for _, lr := range [][2]int16{{100, 50}, {200, -5}, {0, 0}, {128, 255}, {-128, 1000}, {127, 127}} {
		yaroze.SsSetMVol(lr[0], lr[1])
		getters()
		frame()
	}
	flush("mvol")

	// CD audio volume and attributes.
	for _, lr := range [][2]int16{{100, 30}, {200, -1}, {0, 0}, {128, 300}, {127, 127}} {
		yaroze.SsSetSerialVol(0, lr[0], lr[1])
		getters()
		frame()
	}
	// (Attributes other than SS_MIX and SS_REV reach other code in
	// Sony's library, writing other registers: not probed.)
	for _, am := range [][2]byte{{0, 1}, {1, 1}, {0, 0}, {1, 0}, {0, 2}, {1, 255}, {0, 0}} {
		yaroze.SsSetSerialAttr(0, am[0], am[1])
		getters()
		frame()
	}
	yaroze.SsSetSerialAttr(0, 1, 0)
	frame()
	flush("serial")

	// Mute: the getter, then voices keyed on before and during it.
	for _, m := range []byte{1, 0, 2, 0} {
		yaroze.SsSetMute(m)
		rec('m', int32(yaroze.SsGetMute()), int32(m))
		frame()
	}
	v0 := yaroze.SsUtKeyOn(vab, 1, 0, 60, 0, 100, 100)
	rec('k', int32(v0), 0)
	for range 5 {
		frame()
	}
	yaroze.SsSetMute(1)
	rec('m', int32(yaroze.SsGetMute()), 1)
	frame()
	frame()
	v1 := yaroze.SsUtKeyOn(vab, 1, 0, 64, 0, 100, 60)
	rec('k', int32(v1), 0)
	frame()
	yaroze.SsSetMVol(64, 64)
	getters()
	frame()
	yaroze.SsUtSetVVol(v0, 30, 90)
	frame()
	yaroze.SsSetMute(1)
	frame()
	yaroze.SsSetMute(0)
	rec('m', int32(yaroze.SsGetMute()), 0)
	frame()
	yaroze.SsSetMVol(127, 127)
	yaroze.SsUtAllKeyOff(0)
	for range 60 {
		frame()
	}
	flush("mute")

	// Reverb delay and feedback, for each type.
	for t := int16(0); t < 10; t++ {
		rec('t', int32(yaroze.SsUtSetReverbType(t)), int32(t))
		frame()
		for _, d := range []int16{0, 1, 64, 127, 128, -1, 40} {
			yaroze.SsUtSetReverbDelay(d)
			frame()
		}
		for _, d := range []int16{0, 1, 64, 127, 128, -1, 40} {
			yaroze.SsUtSetReverbFeedback(d)
			frame()
		}
		yaroze.SsUtSetReverbDepth(64, 32)
		frame()
		yaroze.SsUtSetReverbDelay(100)
		frame()
		yaroze.SsUtSetReverbFeedback(100)
		frame()
		rec('g', int32(yaroze.SsUtGetReverbType()), 0)
	}
	yaroze.SsUtSetReverbType(0)
	frame()
	flush("reverb")

	// SsUtChangePitch.
	pv := yaroze.SsUtKeyOn(vab, 1, 0, 60, 0, 100, 100)
	rec('k', int32(pv), 0)
	frame()
	cp := func(voice, vabid, prog, on, of, nn, nf int16) {
		rec('c', int32(yaroze.SsUtChangePitch(voice, vabid, prog, on, of, nn, nf)), int32(nn)<<16|int32(uint16(nf)))
		frame()
	}
	cp(pv, vab, 1, 60, 0, 62, 0)
	cp(pv, vab, 1, 60, 0, 64, 50)
	cp(pv, vab, 1, 62, 0, 65, 0)
	cp(pv, vab, 1, 64, 50, 66, 0)
	cp(pv, vab, 2, 60, 0, 61, 0)
	cp(pv, vab, 0, 60, 0, 61, 0)
	cp(pv, vab+1, 1, 60, 0, 61, 0)
	cp(pv, vab, 1, 60, 10, 61, 0)
	cp(pv, vab, 1, 59, 0, 61, 0)
	for _, nf := range [][2]int16{{60, 127}, {60, 128}, {60, 200}, {127, 127}, {0, 0}, {130, 0}, {48, 64}} {
		cp(pv, vab, 1, 60, 0, nf[0], nf[1])
	}
	cp(24, vab, 1, 60, 0, 61, 0)
	cp(-1, vab, 1, 60, 0, 61, 0)
	cp(pv+1, vab, 1, 60, 0, 61, 0)
	yaroze.SsUtKeyOff(pv, vab, 1, 0, 60)
	frame()
	cp(pv, vab, 1, 60, 0, 63, 0)
	yaroze.SsUtAllKeyOff(0)
	for range 30 {
		frame()
	}
	flush("change")

	// SsUtPitchBend.
	pv = yaroze.SsUtKeyOn(vab, 1, 0, 60, 0, 100, 100)
	rec('k', int32(pv), 0)
	frame()
	pb := func(voice, vabid, prog, note, b int16) {
		rec('b', int32(yaroze.SsUtPitchBend(voice, vabid, prog, note, b)), int32(b))
		frame()
	}
	for _, b := range []int16{64, 0, 32, 63, 65, 100, 127, 128, 200, -1, 0x2000, 0x3fff, 66, 71, 80, 1, 50} {
		pb(pv, vab, 1, 60, b)
	}
	pb(pv, vab, 1, 61, 100)
	pb(pv, vab, 0, 60, 100)
	pb(pv, vab+1, 1, 60, 100)
	pb(24, vab, 1, 60, 100)
	// Bend, then change the pitch, then bend again.
	pb(pv, vab, 1, 60, 90)
	cp(pv, vab, 1, 60, 0, 67, 0)
	pb(pv, vab, 1, 60, 90)
	pb(pv, vab, 1, 67, 90)
	cp(pv, vab, 1, 67, 0, 60, 20)
	pb(pv, vab, 1, 60, 40)
	pb(pv, vab, 1, 61, 40)
	cp(pv, vab, 1, 60, 0, 62, 30)
	pb(pv, vab, 1, 67, 70)
	pb(pv, vab, 1, 62, 70)
	pb(pv, vab, 1, 60, 70)
	yaroze.SsUtAllKeyOff(0)
	for range 10 {
		frame()
	}
	// A key on with a fine pitch: what the bend starts from, before and
	// after SsUtChangePitch.
	pv = yaroze.SsUtKeyOn(vab, 1, 0, 60, 40, 100, 100)
	rec('k', int32(pv), 0)
	frame()
	pb(pv, vab, 1, 60, 64)
	pb(pv, vab, 1, 60, 90)
	cp(pv, vab, 1, 60, 0, 60, 100)
	pb(pv, vab, 1, 60, 64)
	pb(pv, vab, 1, 60, 30)
	// A tone with no bend range.
	qv := yaroze.SsUtKeyOn(vab, 0, 1, 60, 0, 100, 100)
	rec('k', int32(qv), 0)
	frame()
	pb(qv, vab, 0, 60, 100)
	pb(qv, vab, 0, 60, 20)
	cp(qv, vab, 0, 60, 0, 72, 0)
	yaroze.SsUtAllKeyOff(0)
	for range 30 {
		frame()
	}
	flush("bend")

	println("done")
	for {
		frame()
	}
}
