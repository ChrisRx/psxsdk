//go:build yaroze

// Command voices is a probe for the spec: which voice SsUtKeyOn takes,
// above all when none is free. Each scenario starts from SsUtAllKeyOff and
// a quiet SPU, keys notes on and off on given frames, and prints the voice
// each key on returned and which voices sound (envelope above 0). Built
// for the resident library it shows Sony's behavior.
//
// The bank, assets/PROBE.VH and PROBE.VB, has program 0 (priority 64) with
// tones:
//
//	0: priority 0, looping, fast release
//	1: priority 64, looping, fast release
//	2: priority 127, looping, fast release
//	3: priority 64, looping, slow release (about two seconds)
//	4: priority 64, a short sound that ends, fast release
//
// and program 1 (priority 0) with one tone like tone 1.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

var (
	vhData [(3616 + 2047) / 2048 * 512]uint32
	vbData [(1008 + 2047) / 2048 * 512]uint32

	vh = disc.Register("ASSETS/PROBE.VH", "assets/PROBE.VH").Into(vhData[:])
	vb = disc.Register("ASSETS/PROBE.VB", "assets/PROBE.VB").Into(vbData[:])
)

var vab int16

// voice is what each key on returned, by the order of the key ons in a
// scenario, for key offs to name.
var voice [128]int16

type step struct {
	wait int // frames to wait before this step
	do   func()
}

// log holds what a scenario did, printed after it so that printing does
// not stretch its timing: each entry is a key on or off, its result and
// the vertical blank count.
var log [192]struct {
	off      bool
	i        int
	p, t, n  int16
	r        int16
	vblank   int32
	sounding uint32
}

var logged int

func record(off bool, i int, p, t, n, r int16) {
	if logged < len(log) {
		e := &log[logged]
		e.off, e.i, e.p, e.t, e.n, e.r = off, i, p, t, n, r
		e.vblank = yaroze.VSync(-1)
		e.sounding = sounding()
		logged++
	}
}

// on keys tone t of program p on at note n and records the voice as the
// i-th of the scenario.
func on(i int, p, t, n int16) func() {
	return func() {
		voice[i] = yaroze.SsUtKeyOn(vab, p, t, n, 0, 100, 100)
		record(false, i, p, t, n, voice[i])
	}
}

// off keys the i-th key on's voice off.
func off(i int, p, t, n int16) func() {
	return func() {
		r := yaroze.SsUtKeyOff(voice[i], vab, p, t, n)
		record(true, i, p, t, n, r)
	}
}

// alloff calls SsUtAllKeyOff.
func alloff() func() {
	return func() {
		yaroze.SsUtAllKeyOff(0)
		record(true, -1, 0, 0, 0, 0)
	}
}

// fill keys 24 notes of tone t on, from note n up, k a frame.
func fill(base int, p, t, n int16, k int) []step {
	var s []step
	for i := range 24 {
		w := 0
		if k > 0 && i%k == 0 && i > 0 {
			w = 1
		}
		s = append(s, step{w, on(base+i, p, t, n+int16(i))})
	}
	return s
}

// sounding returns the voices whose envelope is above 0, a bit each.
func sounding() uint32 {
	var m uint32
	for v := range 24 {
		if volatile.LoadUint16((*uint16)(yaroze.Pointer(uintptr(0x1f801c0c+v*16)))) != 0 {
			m |= 1 << v
		}
	}
	return m
}

func cat(parts ...[]step) []step {
	var s []step
	for _, p := range parts {
		s = append(s, p...)
	}
	return s
}

var scenarios = [][]step{
	// 0: fill one a frame, then six more one a frame.
	cat(fill(0, 0, 1, 40, 1), []step{
		{1, on(24, 0, 1, 70)}, {1, on(25, 0, 1, 71)}, {1, on(26, 0, 1, 72)},
		{1, on(27, 0, 1, 73)}, {1, on(28, 0, 1, 74)}, {1, on(29, 0, 1, 75)},
	}),
	// 1: thirty in one frame.
	cat(fill(0, 0, 1, 40, 0), []step{
		{0, on(24, 0, 1, 70)}, {0, on(25, 0, 1, 71)}, {0, on(26, 0, 1, 72)},
		{0, on(27, 0, 1, 73)}, {0, on(28, 0, 1, 74)}, {0, on(29, 0, 1, 75)},
	}),
	// 2: fill four a frame, then six more in one frame.
	cat(fill(0, 0, 1, 40, 4), []step{
		{1, on(24, 0, 1, 70)}, {0, on(25, 0, 1, 71)}, {0, on(26, 0, 1, 72)},
		{0, on(27, 0, 1, 73)}, {0, on(28, 0, 1, 74)}, {0, on(29, 0, 1, 75)},
	}),
	// 3: priorities. Voices of tone 1, but the 8th and 16th key on tone
	// 0 (priority 0) and the 4th tone 2 (127); then key ons of tone 1,
	// tone 2 and tone 0.
	func() []step {
		s := fill(0, 0, 1, 40, 1)
		s[3].do = on(3, 0, 2, 43)
		s[7].do = on(7, 0, 0, 47)
		s[15].do = on(15, 0, 0, 55)
		return append(s,
			step{1, on(24, 0, 1, 70)}, step{1, on(25, 0, 1, 71)},
			step{1, on(26, 0, 2, 72)}, step{1, on(27, 0, 0, 73)},
			step{1, on(28, 0, 1, 74)}, step{1, on(29, 0, 2, 75)})
	}(),
	// 4: all priority 127, then key ons of priority 0, 64, 127.
	cat(fill(0, 0, 2, 40, 1), []step{
		{1, on(24, 0, 0, 70)}, {1, on(25, 0, 1, 71)}, {1, on(26, 0, 2, 72)},
		{1, on(27, 0, 2, 73)},
	}),
	// 5: all priority 0, then key ons of priority 0, 64, 127.
	cat(fill(0, 0, 0, 40, 1), []step{
		{1, on(24, 0, 0, 70)}, {1, on(25, 0, 1, 71)}, {1, on(26, 0, 2, 72)},
		{1, on(27, 0, 0, 73)},
	}),
	// 6: slow releases. Fill with tone 3, key off the 11th then the 5th,
	// then key ons a frame later, then the 18th off and on in one frame.
	cat(fill(0, 0, 3, 40, 1), []step{
		{1, off(10, 0, 3, 50)}, {0, off(4, 0, 3, 44)},
		{1, on(24, 0, 3, 70)}, {1, on(25, 0, 3, 71)}, {1, on(26, 0, 3, 72)},
		{1, off(17, 0, 3, 57)}, {0, on(27, 0, 3, 73)},
		{1, on(28, 0, 3, 74)},
	}),
	// 7: slow releases, key ons only after 30 frames.
	cat(fill(0, 0, 3, 40, 1), []step{
		{1, off(10, 0, 3, 50)}, {0, off(4, 0, 3, 44)},
		{30, on(24, 0, 3, 70)}, {1, on(25, 0, 3, 71)}, {1, on(26, 0, 3, 72)},
	}),
	// 8: sounds that end by themselves without a key off.
	cat(fill(0, 0, 4, 40, 1), []step{
		{60, on(24, 0, 1, 70)}, {1, on(25, 0, 1, 71)}, {1, on(26, 0, 1, 72)},
	}),
	// 9: the same note twice, the same voice? Then key off by the first.
	{
		{0, on(0, 0, 1, 60)}, {1, on(1, 0, 1, 60)}, {1, on(2, 0, 1, 60)},
		{1, off(0, 0, 1, 60)}, {1, on(3, 0, 1, 61)},
	},
	// 10: program priority. Fill with program 1 (priority 0) and
	// program 0 tone 1 alternating, then key ons of program 0 tone 1.
	func() []step {
		s := fill(0, 0, 1, 40, 1)
		for i := 1; i < 24; i += 2 {
			s[i].do = on(i, 1, 0, 40+int16(i))
		}
		return append(s, step{1, on(24, 0, 1, 70)}, step{1, on(25, 0, 1, 71)}, step{1, on(26, 1, 0, 72)})
	}(),
	// 11: free some voices out of order, then key on 6.
	cat(fill(0, 0, 1, 40, 1), []step{
		{1, off(20, 0, 1, 60)}, {0, off(3, 0, 1, 43)}, {0, off(12, 0, 1, 52)},
		{10, on(24, 0, 1, 70)}, {1, on(25, 0, 1, 71)}, {1, on(26, 0, 1, 72)},
		{1, on(27, 0, 1, 73)},
	}),
	// 12: steal repeatedly: what order the stolen voices follow over 30.
	func() []step {
		s := fill(0, 0, 1, 40, 1)
		for i := range 30 {
			s = append(s, step{1, on(24+i, 0, 1, 70+int16(i%40))})
		}
		return s
	}(),
}

// timing returns a scenario that fills the voices g a frame, waits d
// frames, then steals six times in one frame and six times one a frame.
func timing(g, d int) []step {
	var s []step
	for i := range 24 {
		w := 0
		if i > 0 && i%g == 0 {
			w = 1
		}
		s = append(s, step{w, on(i, 0, 1, 40+int16(i))})
	}
	for i := range 6 {
		w := 0
		if i == 0 {
			w = d
		}
		s = append(s, step{w, on(24+i, 0, 1, 70+int16(i))})
	}
	for i := range 6 {
		s = append(s, step{1, on(30+i, 0, 1, 80+int16(i))})
	}
	return s
}

func init() {
	for _, g := range []int{1, 24, 4, 2, 3, 6, 8, 12} {
		for d := range 4 {
			scenarios = append(scenarios, timing(g, d))
		}
	}
	// Voice 0 the newest: free it and key it on again, then steal.
	s := fill(0, 0, 1, 40, 1)
	s = append(s, step{1, off(0, 0, 1, 40)}, step{20, on(24, 0, 1, 64)})
	for i := range 6 {
		s = append(s, step{5, on(25+i, 0, 1, 70+int16(i))})
	}
	scenarios = append(scenarios, s)
	// Two halves ten frames apart, steals ten frames after.
	s = nil
	for i := range 24 {
		w := 0
		if i == 12 {
			w = 10
		}
		s = append(s, step{w, on(i, 0, 1, 40+int16(i))})
	}
	for i := range 8 {
		w := 1
		if i == 0 {
			w = 10
		}
		s = append(s, step{w, on(24+i, 0, 1, 70+int16(i))})
	}
	scenarios = append(scenarios, s)
	// Many steals in one frame, increasing notes, then the same note.
	for _, same := range []bool{false, true} {
		s = fill(0, 0, 1, 40, 1)
		for i := range 30 {
			w := 0
			if i == 0 {
				w = 5
			}
			n := int16(70)
			if !same {
				n += int16(i)
			}
			s = append(s, step{w, on(24+i, 0, 1, n)})
		}
		scenarios = append(scenarios, s)
	}
	// Two steals a frame for twelve frames.
	s = fill(0, 0, 1, 40, 1)
	for i := range 24 {
		w := 0
		if i%2 == 0 {
			w = 1
		}
		s = append(s, step{w, on(24+i, 0, 1, 70+int16(i))})
	}
	scenarios = append(scenarios, s)
	// 50: fast voices all keyed off, then priority 0 key ons, which take
	// only free voices, in the same frame and after.
	scenarios = append(scenarios, cat(fill(0, 0, 1, 40, 1), []step{
		{1, alloff()}, {0, on(24, 0, 0, 70)}, {1, on(25, 0, 0, 71)},
		{1, on(26, 0, 0, 72)}, {3, on(27, 0, 0, 73)},
	}))
	// 51: one fast voice keyed off, priority 0 key ons after 0 to 5 frames.
	scenarios = append(scenarios, cat(fill(0, 0, 1, 40, 1), []step{
		{1, off(5, 0, 1, 45)}, {0, on(24, 0, 0, 70)}, {1, on(25, 0, 0, 71)},
		{1, on(26, 0, 0, 72)}, {1, on(27, 0, 0, 73)}, {2, on(28, 0, 0, 74)},
	}))
	// 52: slow voices, stolen from, then all keyed off: priority 0 key
	// ons, then priority 64 ones.
	scenarios = append(scenarios, cat(fill(0, 0, 3, 40, 1), []step{
		{3, on(24, 0, 3, 70)}, {1, on(25, 0, 3, 71)}, {1, on(26, 0, 3, 72)},
		{1, alloff()}, {2, on(27, 0, 0, 73)}, {1, on(28, 0, 1, 74)},
		{1, on(29, 0, 1, 75)}, {1, on(30, 0, 1, 76)}, {1, on(31, 0, 0, 77)},
	}))
	// 53: slow voices, one keyed off: a priority 0 key on, then steals.
	scenarios = append(scenarios, cat(fill(0, 0, 3, 40, 1), []step{
		{3, off(20, 0, 3, 60)}, {2, on(24, 0, 0, 70)}, {1, on(25, 0, 3, 71)},
		{1, on(26, 0, 3, 72)},
	}))
	// 54: like 53, the keyed off voice the oldest after a steal of 0.
	scenarios = append(scenarios, cat(fill(0, 0, 3, 40, 1), []step{
		{3, on(24, 0, 3, 70)}, {1, off(1, 0, 3, 41)}, {2, on(25, 0, 3, 71)},
		{1, on(26, 0, 3, 72)},
	}))
	// 55 to 59: when a voice is free again. 23 voices hold notes; the
	// 24th plays a short sound (notes 40, 60, 84) or is keyed off (fast
	// and slow release); then a priority 0 key on every frame, which
	// takes only a free voice.
	poll := func(last func() []step) []step {
		s := fill(0, 0, 1, 40, 1)[:23]
		s = append(s, last()...)
		for i := range 60 {
			s = append(s, step{1, on(30+i, 0, 0, 90)})
		}
		return s
	}
	for _, n := range []int16{40, 60, 84} {
		scenarios = append(scenarios, poll(func() []step { return []step{{1, on(23, 0, 4, n)}} }))
	}
	for _, t := range []int16{1, 3} {
		scenarios = append(scenarios, poll(func() []step {
			return []step{{1, on(23, 0, t, 63)}, {5, off(23, 0, t, 63)}}
		}))
	}
	// 60 to 67: more notes for the short sound.
	for _, n := range []int16{20, 30, 50, 70, 96, 108, 120, 127} {
		scenarios = append(scenarios, poll(func() []step { return []step{{1, on(23, 0, 4, n)}} }))
	}
	// 68: two short sounds keyed on in one frame.
	scenarios = append(scenarios, poll(func() []step {
		return []step{{1, on(23, 0, 4, 60)}, {0, on(24, 0, 4, 60)}}
	}))
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	if f := disc.LoadAll(300); f != nil {
		println("load failed")
	}
	vab = yaroze.SsVabTransfer(vh.Bytes(), vb.Bytes(), -1, 1)
	yaroze.SsSetMVol(127, 127)
	println("vab", vab)
	for c, s := range scenarios {
		yaroze.SsUtAllKeyOff(0)
		quiet := 0
		for sounding() != 0 && quiet < 6000 {
			yaroze.VSync(0)
			quiet++
		}
		for range 120 {
			yaroze.VSync(0)
		}
		println("quiet after", quiet)
		logged = 0
		start := yaroze.VSync(-1)
		for _, st := range s {
			for range st.wait {
				yaroze.VSync(0)
			}
			st.do()
		}
		yaroze.VSync(0)
		yaroze.VSync(0)
		println("case", c)
		for _, e := range log[:logged] {
			if e.off && e.i < 0 {
				print("  f", e.vblank-start, " alloff")
			} else if e.off {
				print("  f", e.vblank-start, " off", e.i, "(v", voice[e.i], ")=", e.r)
			} else {
				print("  f", e.vblank-start, " on", e.i, "(p", e.p, " t", e.t, " n", e.n, ")=", e.r)
			}
			println(" sounding", e.sounding)
		}
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}
