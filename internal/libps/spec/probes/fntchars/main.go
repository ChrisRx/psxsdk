//go:build yaroze

// Command fntchars is a probe for the spec: FntPrint and FntFlush on
// strings of different lengths and kinds, each between two GP1 marker
// words (10000200h plus the case number), so that a trace shows how each
// call's time grows with the characters.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000200|n)
}

// The cases: FntPrint of the string is case 2i, FntFlush after it 2i+1.
var cases = []string{
	"",
	"A",
	"AB",
	"ABCD",
	"ABCDEFGH",
	"ABCDEFGHIJKLMNOP",
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ012345",
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz!?",
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcd",
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijkl",
	"                ",
	"\n\n\n\n",
	"\t\t\t\t",
	"A A A A A A A A ",
	"AB\nCD\nEF\nGH\n",
	// A space or a tab that fills the line (40 glyphs wide), against the
	// same glyphs without it.
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA ",
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\t",
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\t",
	// The window is 30 rows tall: text on the last row and past it.
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nAB",
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nAB",
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nABCDEFGH",
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nABCDEFGH",
	// A line that fills the last row: the wrap that reaches the bottom.
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABCDEFGH",
	// A space and a tab that fill the last row, against spaces and tabs
	// one short of it.
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA ",
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA  ",
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\t",
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\t\t",
}

// The cases after the first 31 need the stream set up before their
// markers: the window reopened (FntLoad closes every stream, FntOpen
// opens one at the same place) with isbg 1 or holding 8 characters, or
// the stream filled by a print before the measured one. The setup runs
// outside the measured spans. Case i is len(cases)+j for more[j].
var more = []struct {
	open bool // FntLoad(960, 256) and FntOpen(0, 0, 320, 240, isbg, n) first
	isbg int32
	n    int32
	fill string // printed before the markers
	s    string
}{
	// 31-32: the empty string and 4 glyphs on the window as main opened
	// it, with nothing set up: this loop's own overhead against cases 0
	// and 3.
	{s: ""},
	{s: "ABCD"},
	// 33-35: isbg 1, the empty string and 4 and 8 glyphs.
	{open: true, isbg: 1, n: 512, s: ""},
	{s: "ABCD"},
	{s: "ABCDEFGH"},
	// 36-41: isbg 0 again, a stream of 512: 10, 5 and 3 glyphs and the
	// "%%" pair (one character), against "AB" and "ABCD".
	{open: true, isbg: 0, n: 512, s: "ABCDEFGHIJ"},
	{s: "FGHIJ"},
	{s: "ABC"},
	{s: "%%"},
	{s: "A%%B"},
	{s: "%%%%"},
	// 42-47: a stream of 8. 10 characters cut at 8 (returns -1); 8
	// exactly (returns 8); 5 held then 5 more, cut at 3; the stream full
	// (8 held), then "I", the empty string and 8 characters, each
	// returning -1.
	{open: true, isbg: 0, n: 8, s: "ABCDEFGHIJ"},
	{s: "ABCDEFGH"},
	{fill: "ABCDE", s: "FGHIJ"},
	{fill: "ABCDEFGH", s: "I"},
	{fill: "ABCDEFGH", s: ""},
	{fill: "ABCDEFGH", s: "ABCDEFGH"},
	// 48: the window reopened as main opened it, the empty string.
	{open: true, isbg: 0, n: 512, s: ""},
}

// stride separates the rounds' marker numbers; fnt.py uses the same.
const stride = 128

// runMore runs the more cases of a round. It is a function of its own
// so that main's loop over cases compiles as it did before more existed
// and the first 31 cases' numbers stay comparable.
//
//go:noinline
func runMore(round uint32) {
	for j, c := range more {
		n := round*stride + uint32(len(cases)+j)*2
		if c.open {
			yaroze.FntLoad(960, 256)
			yaroze.FntOpen(0, 0, 320, 240, c.isbg, c.n)
		}
		yaroze.DrawSync(0)
		if c.fill != "" {
			yaroze.Print(c.fill)
		}
		mark(n)
		yaroze.Print(c.s)
		mark(n + 1)
		yaroze.FntFlush(-1)
		mark(n + 2)
	}
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(0, 0, 320, 240, 0, 512)
	yaroze.DrawSync(0)
	for round := range uint32(2) {
		for i, s := range cases {
			n := round*stride + uint32(i)*2
			yaroze.DrawSync(0)
			mark(n)
			yaroze.Print(s)
			mark(n + 1)
			yaroze.FntFlush(-1)
			mark(n + 2)
		}
		runMore(round)
		yaroze.VSync(0)
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}
