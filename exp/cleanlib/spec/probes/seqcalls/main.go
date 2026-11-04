//go:build yaroze

// Command seqcalls is a probe for spec 10: the sequence calls the
// examples do not make (SsSetTempo, SsSeqSetAccelerando,
// SsSeqSetRitardando, SsIsEos, SsSeqSetNext, SsPlayBack, SsSetMute),
// SsSeqPause on a playing sequence, and the SEQ events tempo and
// controllers 7, 10 and 11. Each case runs its steps on given frames and
// records SsIsEos of the sequences it watches every frame; all is printed
// at the end, so that printing does not move the cases' frames. The SPU
// writes show the rest. Built for the resident library it shows Sony's
// behavior. Not here, as not specified: an accelerando together with
// SsSetTempo or tempo meta events, and a time of 0, on which Sony's
// library stops at a break (a division by zero).
//
// The SEQs and the bank are made by gen.go (go run gen.go):
//
//   - BEND: a held note on program 1 and a pitch bend every tick, high
//     byte 4 to 123 round again, so each blank's pitch write tells the
//     last tick played; 96 ticks a quarter note at 120 beats a minute.
//   - TMETA: BEND with tempo meta events on the way.
//   - CTL: controllers 7, 10 and 11 between notes.
//   - CTL2: the same over three channels and two programs.
//   - SHORT, SHORT2: two notes each.
//   - PROBE2.VH: the voices probe's bank, program 1's tone with pbmin 2,
//     pbmax 7 and shift 37.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

var (
	vhData     [(3616 + 2047) / 2048 * 512]uint32
	vbData     [(1008 + 2047) / 2048 * 512]uint32
	bendData   [(4096 + 2047) / 2048 * 512]uint32
	tmetaData  [(4096 + 2047) / 2048 * 512]uint32
	ctlData    [512]uint32
	ctl2Data   [512]uint32
	shortData  [512]uint32
	short2Data [512]uint32

	vh     = disc.Register("ASSETS/PROBE2.VH", "assets/PROBE2.VH").Into(vhData[:])
	vb     = disc.Register("ASSETS/PROBE.VB", "assets/PROBE.VB").Into(vbData[:])
	bend   = disc.Register("ASSETS/BEND.SEQ", "assets/BEND.SEQ").Into(bendData[:])
	tmeta  = disc.Register("ASSETS/TMETA.SEQ", "assets/TMETA.SEQ").Into(tmetaData[:])
	ctl    = disc.Register("ASSETS/CTL.SEQ", "assets/CTL.SEQ").Into(ctlData[:])
	ctl2   = disc.Register("ASSETS/CTL2.SEQ", "assets/CTL2.SEQ").Into(ctl2Data[:])
	short  = disc.Register("ASSETS/SHORT.SEQ", "assets/SHORT.SEQ").Into(shortData[:])
	short2 = disc.Register("ASSETS/SHORT2.SEQ", "assets/SHORT2.SEQ").Into(short2Data[:])
)

var vab int16

// a and b are the case's sequences, opened by its setup.
var a, b int16 = -1, -1

// unopened is an id no case opens.
const unopened = 31

type step struct {
	at int // the frame of the case to run it in
	do func()
}

type kase struct {
	open   func()  // opens a and b
	steps  []step  // in frame order
	frames int     // how long the case runs
	watch  []int16 // the sequences whose SsIsEos is recorded, by *id
}

// eos holds SsIsEos for each watched sequence each frame, a character a
// frame.
var eos [2][600]byte

func open(d []uint32) int16 { return yaroze.SsSeqOpen(&d[0], vab) }

func play(mode byte, loops int16) func() { return func() { yaroze.SsSeqPlay(a, mode, loops) } }

var cases = []kase{
	// 0: tempo meta events.
	{func() { a = open(tmetaData[:]) }, []step{{0, play(1, 1)}}, 480, []int16{0}},
	// 1: SsSetTempo.
	{func() { a = open(bendData[:]) }, []step{
		{0, play(1, 1)},
		{40, func() { yaroze.SsSetTempo(a, 0, 60) }},
		{80, func() { yaroze.SsSetTempo(a, 0, 133) }},
		{110, func() { yaroze.SsSetTempo(a, 0, 250) }},
		{130, func() { yaroze.SsSetTempo(a, 0, 1) }},
		{160, func() { yaroze.SsSetTempo(a, 0, 120) }},
		{200, func() { yaroze.SsSeqStop(a) }},
	}, 210, nil},
	// 2: accelerando.
	{func() { a = open(bendData[:]) }, []step{
		{0, play(1, 1)},
		{30, func() { yaroze.SsSeqSetAccelerando(a, 240, 120) }},
		{250, func() { yaroze.SsSeqStop(a) }},
	}, 260, nil},
	// 3: ritardando.
	{func() { a = open(bendData[:]) }, []step{
		{0, play(1, 1)},
		{30, func() { yaroze.SsSeqSetRitardando(a, 60, 120) }},
		{300, func() { yaroze.SsSeqStop(a) }},
	}, 310, nil},
	// 4: each the wrong way, and over one tick.
	{func() { a = open(bendData[:]) }, []step{
		{0, play(1, 1)},
		{10, func() { yaroze.SsSeqSetAccelerando(a, 60, 50) }},
		{100, func() { yaroze.SsSeqSetRitardando(a, 200, 50) }},
		{160, func() { yaroze.SsSeqSetAccelerando(a, 300, 1) }},
		{180, func() { yaroze.SsSeqSetRitardando(a, 90, 1) }},
		{300, func() { yaroze.SsSeqStop(a) }},
	}, 310, nil},
	// 5: SsIsEos through a sequence's life.
	{func() { a = open(shortData[:]) }, []step{
		{5, play(1, 1)},
		{15, func() { yaroze.SsSeqPause(a) }},
		{20, func() { yaroze.SsSeqReplay(a) }},
		{70, play(1, 1)},
		{80, func() { yaroze.SsSeqStop(a) }},
		{85, play(0, 1)},
		{95, func() { yaroze.SsSeqReplay(a) }},
		{140, func() { yaroze.SsSeqClose(a) }},
	}, 150, []int16{0, 2}},
	// 6: SsIsEos looping, twice and for ever.
	{func() { a = open(shortData[:]); b = open(short2Data[:]) }, []step{
		{0, play(1, 2)},
		{0, func() { yaroze.SsSeqPlay(b, 1, 0) }},
		{150, func() { yaroze.SsSeqStop(b) }},
	}, 160, []int16{0, 1}},
	// 7: SsSeqSetNext.
	{func() { a = open(shortData[:]); b = open(short2Data[:]) }, []step{
		{0, func() { yaroze.SsSeqSetNext(a, b) }},
		{0, play(1, 1)},
	}, 130, []int16{0, 1}},
	// 8: SsSeqSetNext after two plays.
	{func() { a = open(shortData[:]); b = open(short2Data[:]) }, []step{
		{0, func() { yaroze.SsSeqSetNext(a, b) }},
		{0, play(1, 2)},
	}, 160, []int16{0, 1}},
	// 9: SsSeqSetNext while a plays.
	{func() { a = open(shortData[:]); b = open(short2Data[:]) }, []step{
		{0, play(1, 1)},
		{20, func() { yaroze.SsSeqSetNext(a, b) }},
	}, 130, []int16{0, 1}},
	// 10: SsPlayBack while another plays.
	{func() { a = open(bendData[:]); b = open(shortData[:]) }, []step{
		{0, play(1, 1)},
		{20, func() { yaroze.SsPlayBack(b, 0, 1) }},
		{100, func() { yaroze.SsSeqStop(a) }},
	}, 110, []int16{0, 1}},
	// 11: SsPlayBack with nothing playing.
	{func() { a = open(shortData[:]); b = open(short2Data[:]) }, []step{
		{5, func() { yaroze.SsPlayBack(b, 0, 2) }},
	}, 120, []int16{0, 1}},
	// 12: SsSeqPause and SsSeqReplay on a playing sequence.
	{func() { a = open(bendData[:]) }, []step{
		{0, play(1, 1)},
		{40, func() { yaroze.SsSeqPause(a) }},
		{70, func() { yaroze.SsSeqReplay(a) }},
		{100, func() { yaroze.SsSeqPause(a); yaroze.SsSeqReplay(a) }},
		{130, func() { yaroze.SsSeqPause(a) }},
		{135, func() { yaroze.SsSeqPause(a) }},
		{150, func() { yaroze.SsSeqStop(a) }},
	}, 170, []int16{0}},
	// 13: SsSeqPause between notes' on and off.
	{func() { a = open(shortData[:]) }, []step{
		{0, play(1, 1)},
		{8, func() { yaroze.SsSeqPause(a) }},
		{20, func() { yaroze.SsSeqReplay(a) }},
	}, 80, []int16{0}},
	// 14: controllers 7, 10 and 11.
	{func() { a = open(ctlData[:]) }, []step{{0, play(1, 1)}}, 175, nil},
	// 15: the same with SsSeqSetVol.
	{func() { a = open(ctlData[:]) }, []step{
		{0, play(1, 1)},
		{20, func() { yaroze.SsSeqSetVol(a, 64, 100) }},
		{100, func() { yaroze.SsSeqSetVol(a, 127, 30) }},
	}, 175, nil},
	// 16: SsSetMute while a sequence plays.
	{func() { a = open(ctlData[:]) }, []step{
		{0, play(1, 1)},
		{20, func() { yaroze.SsSetMute(1) }},
		{60, func() { yaroze.SsSetMute(0) }},
		{150, func() { yaroze.SsSetMute(1) }},
		{170, func() { yaroze.SsSetMute(0) }},
	}, 180, nil},
	// 17: SsSeqPause right after SsSeqPlay, and SsSeqPause before
	// SsSeqPlay.
	{func() { a = open(shortData[:]); b = open(short2Data[:]) }, []step{
		{0, play(1, 1)},
		{0, func() { yaroze.SsSeqPause(a); yaroze.SsSeqPause(b) }},
		{1, func() { yaroze.SsSeqPlay(b, 1, 1) }},
		{20, func() { yaroze.SsSeqReplay(a) }},
	}, 80, []int16{0, 1}},
}

// tempoCase plays BEND, calls f on frame 10 and stops it on frame 180.
func tempoCase(f func()) kase {
	return kase{func() { a = open(bendData[:]) }, []step{
		{0, play(1, 1)},
		{10, f},
		{180, func() { yaroze.SsSeqStop(a) }},
	}, 190, nil}
}

func init() {
	cases = append(cases,
		// 18: SsSetTempo at slow tempos.
		kase{func() { a = open(bendData[:]) }, []step{
			{0, play(1, 1)},
			{5, func() { yaroze.SsSetTempo(a, 0, 2) }},
			{15, func() { yaroze.SsSetTempo(a, 0, 3) }},
			{25, func() { yaroze.SsSetTempo(a, 0, 4) }},
			{35, func() { yaroze.SsSetTempo(a, 0, 5) }},
			{45, func() { yaroze.SsSetTempo(a, 0, 10) }},
			{55, func() { yaroze.SsSetTempo(a, 0, 37) }},
			{65, func() { yaroze.SsSetTempo(a, 0, 38) }},
			{75, func() { yaroze.SsSetTempo(a, 0, 121) }},
			{100, func() { yaroze.SsSeqStop(a) }},
		}, 110, []int16{0}},
		// 19 to 28: accelerando and ritardando over other spans.
		tempoCase(func() { yaroze.SsSeqSetAccelerando(a, 240, 60) }),
		tempoCase(func() { yaroze.SsSeqSetAccelerando(a, 180, 120) }),
		tempoCase(func() { yaroze.SsSeqSetAccelerando(a, 150, 30) }),
		tempoCase(func() { yaroze.SsSeqSetAccelerando(a, 500, 100) }),
		tempoCase(func() { yaroze.SsSeqSetRitardando(a, 60, 60) }),
		tempoCase(func() { yaroze.SsSeqSetRitardando(a, 90, 120) }),
		tempoCase(func() { yaroze.SsSeqSetRitardando(a, 100, 30) }),
		tempoCase(func() { yaroze.SsSeqSetRitardando(a, 30, 100) }),
		tempoCase(func() { yaroze.SsSeqSetAccelerando(a, 120, 50) }),
		tempoCase(func() { yaroze.SsSeqSetAccelerando(a, 133, 7) }),
		// 29: CTL2.
		kase{func() { a = open(ctl2Data[:]) }, []step{{0, play(1, 1)}}, 120, nil},
		// 30: SsSeqPause before SsSeqOpen.
		kase{func() { yaroze.SsSeqPause(0); a = open(shortData[:]) }, []step{
			{0, play(1, 1)},
		}, 60, []int16{0}},
		// 31: SsSeqPause on an open sequence, SsSeqPlay five frames on.
		kase{func() { a = open(shortData[:]) }, []step{
			{0, func() { yaroze.SsSeqPause(a) }},
			{5, play(1, 1)},
		}, 60, []int16{0}},
		// 32: SsSeqSetNext to a sequence played paused three times.
		kase{func() { a = open(shortData[:]); b = open(short2Data[:]) }, []step{
			{0, func() { yaroze.SsSeqPlay(b, 0, 3) }},
			{0, func() { yaroze.SsSeqSetNext(a, b) }},
			{0, play(1, 1)},
		}, 160, []int16{0, 1}},
		// 33: SsPlayBack, then SsSeqPause and SsSeqReplay.
		kase{func() { a = open(shortData[:]) }, []step{
			{0, func() { yaroze.SsPlayBack(a, 0, 1) }},
			{10, func() { yaroze.SsSeqPause(a) }},
			{15, func() { yaroze.SsSeqReplay(a) }},
		}, 60, []int16{0}},
		// 34: SsSetTempo before SsSeqPlay.
		kase{func() { a = open(bendData[:]) }, []step{
			{0, func() { yaroze.SsSetTempo(a, 0, 60) }},
			{1, play(1, 1)},
			{60, func() { yaroze.SsSeqStop(a) }},
		}, 70, nil},
		// 35: SsSeqSetNext from a sequence that loops for ever, and
		// SsSeqStop on the first.
		kase{func() { a = open(shortData[:]); b = open(short2Data[:]) }, []step{
			{0, func() { yaroze.SsSeqSetNext(a, b) }},
			{0, play(1, 0)},
			{70, func() { yaroze.SsSeqStop(a) }},
		}, 120, []int16{0, 1}},
		// 36: SsSeqSetNext from b to a, b's slot after a's.
		kase{func() { a = open(shortData[:]); b = open(short2Data[:]) }, []step{
			{0, func() { yaroze.SsSeqSetNext(b, a) }},
			{0, func() { yaroze.SsSeqPlay(b, 1, 1) }},
		}, 80, []int16{0, 1}},
	)
}

// text is what the cases print, kept until the end.
var (
	text    [24576]byte
	textLen int
)

func out(t string) {
	textLen += copy(text[textLen:], t)
}

func outInt(n int) {
	if n < 0 {
		out("-")
		n = -n
	}
	var b [12]byte
	i := len(b)
	for {
		i--
		b[i] = '0' + byte(n%10)
		n /= 10
		if n == 0 {
			break
		}
	}
	textLen += copy(text[textLen:], b[i:])
}

func eosChar(v int16) byte {
	if v >= 0 && v <= 9 {
		return '0' + byte(v)
	}
	if v == -1 {
		return '-'
	}
	return '?'
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
	println("vab", vab, "unopened", yaroze.SsIsEos(unopened, 0))
	for range 30 {
		yaroze.VSync(0)
	}
	println("start")
	yaroze.VSync(0)
	for c := range cases {
		k := &cases[c]
		a, b = -1, -1
		k.open()
		ids := [2]*int16{&a, &b}
		s := 0
		for f := range k.frames {
			for s < len(k.steps) && k.steps[s].at == f {
				k.steps[s].do()
				s++
			}
			for i, w := range k.watch {
				if w == 2 {
					eos[i][f] = eosChar(yaroze.SsIsEos(unopened, 0))
				} else {
					eos[i][f] = eosChar(yaroze.SsIsEos(*ids[w], 0))
				}
			}
			yaroze.VSync(0)
		}
		if a >= 0 {
			yaroze.SsSeqClose(a)
		}
		if b >= 0 {
			yaroze.SsSeqClose(b)
		}
		yaroze.SsUtAllKeyOff(0)
		for range 150 {
			yaroze.VSync(0)
		}
		out("case ")
		outInt(c)
		out(" a ")
		outInt(int(a))
		out(" b ")
		outInt(int(b))
		out("\n")
		for i := range k.watch {
			out("  eos")
			outInt(i)
			out(" ")
			out(unsafe.String(&eos[i][0], k.frames))
			out("\n")
		}
		yaroze.VSync(0)
	}
	// Printed only now: printing takes the libraries different times, which
	// would move the cases' frames.
	println(unsafe.String(&text[0], textLen))
	println("done")
	for {
		yaroze.VSync(0)
	}
}
