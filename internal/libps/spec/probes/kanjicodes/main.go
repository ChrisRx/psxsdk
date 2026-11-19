//go:build yaroze

// Command kanjicodes is a probe for the spec beside spec/probes/kanji:
// Krom2Tim and Krom2Tim2 over every trail byte of the non-kanji rows, a
// few codes of each other lead byte, every printable ASCII character and
// some other single bytes, and strings that mix good and bad characters
// or have newlines, printing each result, how far the TIM was written and
// a hash of it, and the strings' TIMs whole.
package main

import "go.chrisrx.dev/psx/yaroze"

const fill = 0x5a5a5a5a

// tim has room for four rows of 16 characters past the header.
var tim [4096]uint32

// pair holds a code's bytes.
var pair [2]byte

// str holds the string a call is given, NUL-terminated.
var str [64]byte

// label and code name the call in its line.
var (
	label string
	code  uint32
)

var hex = "0123456789abcdef"

func word(w uint32) {
	// Substrings of hex, which allocate nothing: the probe makes
	// thousands of calls.
	for i := range 8 {
		d := w >> (28 - 4*i) & 15
		print(hex[d : d+1])
	}
}

func run(two bool, s []byte) {
	for i := range len(tim) {
		tim[i] = fill
	}
	print(label, " ", two, " ")
	word(code)
	str = [64]byte{}
	copy(str[:], s)
	b := &str
	var r int32
	if two {
		r = yaroze.Krom2Tim2(&b[0], &tim[0], 640, 0, 0, 480, 0x7fff, 0)
	} else {
		r = yaroze.Krom2Tim(&b[0], &tim[0], 640, 0, 0, 480, 0x7fff, 0)
	}
	last := -1
	// Ranging over the array, even for its index, would copy it.
	for i := range len(tim) {
		if tim[i] != fill {
			last = i
		}
	}
	h := uint32(2166136261)
	for i := 0; i <= last; i++ {
		h = (h ^ tim[i]) * 16777619
	}
	print(" ", r, " ", last, " ")
	word(h)
	println()
	// The strings' TIMs are printed whole, and the newline cases'
	// headers.
	if label == "n" {
		for i := 13; i < 16; i++ {
			print(" ")
			word(tim[i])
		}
		println()
	}
	if label == "s" || label == "n" {
		for i := 0; i <= last; i++ {
			word(tim[i])
			if i%8 == 7 || i == last {
				println()
			} else {
				print(" ")
			}
		}
	}
}

var strs = []string{
	"AB", "ABC", "ABCD", " ", "~", "\x7f", "\n", "\t", "\x01",
	"\x82\xa0\x98\x9f", "\x98\x9f\x82\xa0", "\x82\xa0\xa1\x82\xa0", "\xa1\x82\xa0",
	"\x82\xa0\x85\x40\x98\x9f", "\x81\x40\x81\x40", "\x82\x00",
	"A\nB", "\n\n", "\nA", "AAAAAAAAAAAAAAAA\n", "AAAAAAAAAAAAAAAA\nB", "AAAAAAAAAAAAAAA\nB",
	"A\n\x82\xa0\n\nB", "\x89\x7f\x82\xa0", "\x82\xa0\x82", "\x82\xa0\x82\x00\x82\xa2",
}

// lines are strings with newlines, for how they size and place the
// characters.
var lines = []string{
	"A\nB", "AA\nB", "AAAAAAAA\nB", "AAAAAAAAAAAAAA\nB", "AAAAAAAAAAAAAAA\nB",
	"A\nAAAAAAAAAAAAAAA", "A\nAAAAAAAAAAAAAAAA", "A\nAAAAAAAAAAAAAAAAA",
	"\nAAAAAAAAAAAAAAAA", "\nAAAAAAAAAAAAAAAAA", "A\n", "AA\n", "AAAAAAAAAAAAAAA\n",
	"AAAAAAAAAAAAAAAAA\n", "AAAAAAAAAAAAAAAAA\nB", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\nB",
	"\n\n\n", "A\n\n", "A\nB\nC", "AAAAAAAAAAAAAAA\nB\nC", "\nAAAAAAAAAAAAAAA\nB",
	"AAAAAAAA\nAAAAAAAA\nB", "AAAAAAAAAAAAAAAAAAAAAAAA\nB", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\nB",
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\nB",
}

func main() {
	yaroze.ResetGraph(0)
	for k := range 2 {
		two := k == 1
		for i, s := range strs {
			label, code = "s", uint32(i)
			run(two, []byte(s))
		}
		for hi := 0x81; hi <= 0x84; hi++ {
			for lo := 1; lo < 256; lo++ {
				label, code = "c", uint32(hi<<8|lo)
				pair[0], pair[1] = byte(hi), byte(lo)
				run(two, pair[:])
			}
		}
		for _, hi := range []int{0x80, 0x85, 0x87, 0x88, 0x89, 0x97, 0x98, 0x99, 0x9f, 0xa0, 0xe0, 0xef, 0xfd, 0xff} {
			for _, lo := range []int{0x3f, 0x40, 0x7e, 0x7f, 0x80, 0x9e, 0x9f, 0xfc, 0xfd} {
				if (hi < 0x88 || hi > 0x98) && lo != 0x40 && lo != 0x9f {
					continue
				}
				label, code = "c", uint32(hi<<8|lo)
				pair[0], pair[1] = byte(hi), byte(lo)
				run(two, pair[:])
			}
		}
		for _, c := range []int{0x9872, 0x9873, 0x9874, 0x987e, 0x9880} {
			label, code = "c", uint32(c)
			pair[0], pair[1] = byte(c>>8), byte(c)
			run(two, pair[:])
		}
		for _, c := range []int{0x01, 0x09, 0x0a, 0x1f, 0x20, 0x21, 0x30, 0x41, 0x5c, 0x5f, 0x60, 0x61, 0x7b, 0x7e, 0x7f, 0x80, 0x9f, 0xa0, 0xa1, 0xdf, 0xe0, 0xfd, 0xff} {
			label, code = "b", uint32(c)
			pair[0] = byte(c)
			run(two, pair[:1])
		}
	}
	for k := range 2 {
		for c := 0x20; c < 0x7f; c++ {
			label, code = "a", uint32(c)
			pair[0] = byte(c)
			run(k == 1, pair[:1])
		}
	}
	for k := range 2 {
		for i, s := range lines {
			label, code = "n", uint32(i)
			run(k == 1, []byte(s))
		}
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}
