//go:build yaroze

// Command accsf times __addsf3 by its operands, one thing changed at a time and then
// at random. Spec 34. Run with PROBE_WATCH at __addsf3: each line is a case, with the
// result, and each case is three calls, so that an interrupt in one shows.
package main

import "go.chrisrx.dev/psx/yaroze"

//export __addsf3
func addsf3(a, b uint32) uint32

var s uint32 = 0x9E3779B9

func rnd() uint32 {
	s ^= s << 13
	s ^= s >> 17
	s ^= s << 5
	return s
}

func hex(v uint32) {
	const d = "0123456789abcdef"
	for i := 28; i >= 0; i -= 4 {
		print(d[v>>uint(i)&15:][:1])
	}
}

func run(group string, a, b uint32) {
	var r uint32
	for range 3 {
		r = addsf3(a, b)
	}
	print("C ", group, " ")
	hex(a)
	print(" ")
	hex(b)
	print(" ")
	hex(r)
	println()
}

const sign = 1 << 31

func both(group string, a, b uint32) {
	run(group, a, b)
	run(group, b, a)
}

func main() {
	yaroze.ResetGraph(0)
	const one, m = uint32(0x3f800000), uint32(0x00123456)

	// Signs and order, with the mantissas fixed and no change of exponent where one
	// can be avoided.
	for _, sa := range [...]uint32{0, sign} {
		for _, sb := range [...]uint32{0, sign} {
			run("sign", sa|0x40000000|m, sb|0x3f000000|m)               // the first the bigger
			run("sign", sa|0x3f000000|m, sb|0x40000000|m)               // the second the bigger
			run("sign", sa|0x40000000|0x500000, sb|0x40000000|0x100000) // one exponent, the first bigger
			run("sign", sa|0x40000000|0x100000, sb|0x40000000|0x500000) // one exponent, the second bigger
			run("sign", sa|0x40000000|0x100000, sb|0x40000000|0x100000) // one magnitude
			run("sign", sa|0x41000000|0x500000, sb|0x3f000000|0x100000) // a gap of 4
			run("sign", sa|0x3f000000|0x100000, sb|0x41000000|0x500000) // a gap of 4, the second bigger
		}
	}

	// The exponent gap, by sign and order. One operand's mantissa has only its top
	// bits, so that nothing is lost until the gap is large.
	for d := uint32(0); d <= 40; d++ {
		big, small := uint32(0x43000000|0x200000), (0x43000000-d<<23)|0x200000
		run("gap++", big, small)
		run("gap++r", small, big)
		run("gap+-", big, sign|small)
		run("gap-+r", sign|small, big)
		run("gap-+", sign|big, small)
		run("gap+-r", small, sign|big)
		run("gap--", sign|big, sign|small)
		run("gap--r", sign|small, sign|big)
		// With low bits to lose.
		run("gaplow", big, 0x43000000-d<<23|0x7fffff)
		run("gaplowneg", big, sign|(0x43000000-d<<23)|0x7fffff)
	}

	// Exponent itself, away from the gap.
	for e := uint32(1); e < 255; e += 13 {
		run("exp", e<<23|m, e<<23|0x400000)
		run("expgap", e<<23|m, (e-min(e, 3))<<23|0x400000)
	}

	// A carry, with one bit of the sum's top moved at a time.
	for k := uint32(0); k < 23; k++ {
		run("carry", 0x3f800000|1<<k, 0x3f800000|1<<k)
		run("nocarry", 0x3f800000|1<<k, 0x3f000000|1<<k)
		run("carryall", 0x3f800000|0x7fffff, 0x3f800000-k<<23|0x7fffff)
	}
	// A carry out of rounding only.
	run("roundcarry", 0x3fffffff, 0x33800000)
	run("roundcarry", 0x3fffffff, 0x33000000)
	run("roundcarry", 0x3fffffff, 0x34000000)

	// Renormalizing distance: 1 - (1 - 2^-k) and the like.
	for k := uint32(1); k <= 24; k++ {
		// a = 1.xxx, b = -(1.xxx - 2^(k-23)) on the same exponent, moving down k-ish places.
		x := uint32(0x3f800000 | 0x7fffff)
		run("down", x, sign|(x-1<<(k-1)))
		run("downr", sign|(x-1<<(k-1)), x)
		run("down0", one|1<<(k-1), sign|one)
		// One place of gap.
		run("down1", 0x40000000, sign|(0x3f800000|(0x7fffff&^(1<<(k-1)-1))))
	}

	// Mantissa bits: both at set counts, no carry, no change of exponent.
	for k := uint32(0); k <= 22; k++ {
		run("bitsa", one|(0x7fffff>>(22-k))&0x3fffff, 0x3e000000)
		run("bitsb", 0x40400000, 0x3f000000|(0x7fffff>>(22-k))&0x3fffff)
		run("bitsS", 0x40400000, 0x3f000000|1<<k)
	}

	// Rounding that carries out of the top, by sign, order and gap.
	for k := uint32(0); k < 3; k++ {
		run("rcarry", 0x3fffffff, 0x33000000+(k+1)<<23)
		run("rcarry", 0x33000000+(k+1)<<23, 0x3fffffff)
		run("rcarry", 0xbfffffff, 0xb3000000+(k+1)<<23)
		run("rcarry", 0xb3000000+(k+1)<<23, 0xbfffffff)
		run("rcarry", 0x3fffffff, 0x3f800000-(k+1)<<23|0x7fffff)
		run("rcarry", 0x40000000, 0xb3000000+(k)<<23)
		run("rcarry", 0x3fffffff-1, 0x33800000+k)
		run("rcarry", 0x3fffffff, 0x33800000+k)
	}

	// Out of range at the top and the bottom, by sign, order and the path there.
	for _, g := range [...]uint32{0, sign} {
		both("range", g|0x7f7fffff, g|0x7f7fffff)
		both("range", g|0x7f7fffff, g|0x7f000000)
		both("range", g|0x7f7fffff, g|0x73000000)
		both("range", g|0x7f7fffff, g|0x73800000)
		both("range", g|0x7f000000, g|0x7f000000)
		both("range", g|0x7f800000, g|0x7f000000)
		both("range", g|0x00400000, (g^sign)|0x00000001)
		both("range", g|0x00800000, (g^sign)|0x00000001)
		both("range", g|0x00800000, (g^sign)|0x007fffff)
		both("range", g|0x00900000, (g^sign)|0x00800000)
		both("range", g|0x00c00000, (g^sign)|0x00ffffff)
		both("range", g|0x01000000, (g^sign)|0x00ffffff)
		both("range", g|0x01400000, (g^sign)|0x013fffff)
		both("range", g|0x02400000, (g^sign)|0x023fffff)
		both("range", g|0x7fffffff, (g^sign)|0x7fffff00)
	}

	// Zeros and other special values.
	vals := [...]uint32{0, sign, 1, sign | 1, 0x00400000, 0x00800000, 0x7f800000, 0xff800000, 0x7fc00000, 0x7f7fffff, 0xff7fffff, one, sign | one}
	for _, a := range vals {
		for _, b := range vals {
			run("special", a, b)
		}
	}

	// At random: sparse and dense mantissas, small gaps more often than large.
	for range 1500 {
		r := rnd()
		m1, m2 := rnd()&0x7fffff, rnd()&0x7fffff
		if r&3 == 0 {
			m1 &= rnd() & rnd()
		}
		if r&12 == 0 {
			m2 &= rnd() & rnd()
		}
		if r&16 != 0 {
			m1 &^= 1<<(rnd()%23) - 1
		}
		if r&32 != 0 {
			m2 &^= 1<<(rnd()%23) - 1
		}
		e1 := 120 + (r>>8)%16
		d := (r >> 16) % 32
		if r&(1<<30) != 0 {
			d %= 4
		}
		e2 := e1 - d
		a, b := e1<<23|m1, e2<<23|m2
		if r&(1<<24) != 0 {
			b |= sign
		}
		if r&(1<<25) != 0 {
			a |= sign
		}
		if r&(1<<26) != 0 {
			a, b = b, a
		}
		run("random", a, b)
	}
	// Held out: none of these were looked at for the rule.
	s = 0xC2B2AE3D
	for range 800 {
		r := rnd()
		e1, d := 100+(r>>8)%50, (r>>16)%30
		a, b := r>>31<<31|e1<<23|rnd()&0x7fffff, (r>>30&1)<<31|(e1-d)<<23|rnd()&0x7fffff
		if r&(1<<26) != 0 {
			a, b = b, a
		}
		run("fresh", a, b)
	}
	println("END")
	for {
		yaroze.VSync(0)
	}
}
