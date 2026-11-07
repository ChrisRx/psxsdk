//go:build yaroze

// Command accdiv times __divdf3 by its operands, one thing changed at a time and then at
// random. Spec 34. Run with PROBE_WATCH at __divdf3: each line is a case, with the
// result, and each case is three calls, so that an interrupt in one shows.
package main

import "go.chrisrx.dev/psx/yaroze"

//export __divdf3
func divdf3(a, b uint64) uint64

var s uint64 = 0x9E3779B97F4A7C15

func rnd() uint64 {
	s ^= s << 13
	s ^= s >> 7
	s ^= s << 17
	return s
}

func hex(v uint64) {
	const d = "0123456789abcdef"
	for i := 60; i >= 0; i -= 4 {
		print(string(rune(d[v>>uint(i)&15])))
	}
}

func run(group string, a, b uint64) {
	var r uint64
	for range 3 {
		r = divdf3(a, b)
	}
	print("C ", group, " ")
	hex(a)
	print(" ")
	hex(b)
	print(" ")
	hex(r)
	println()
}

const (
	one  = uint64(0x3ff0000000000000)
	sign = uint64(1) << 63
	frac = uint64(1)<<52 - 1
)

func main() {
	yaroze.ResetGraph(0)

	// Signs and exponents, with one quotient.
	for _, sa := range [...]uint64{0, sign} {
		for _, sb := range [...]uint64{0, sign} {
			run("sign", sa|0x4008000000000000, sb|one)
			run("sign", sa|0x400921fb54442d18, sb|0x4005bf0a8b145769)
		}
	}
	for e := uint64(1); e < 2047; e += 97 {
		run("expa", e<<52|0x921fb54442d18, one)
		run("expb", 0x400921fb54442d18, e<<52|0x5bf0a8b145769)
		run("expab", e<<52|0x921fb54442d18, e<<52|0x5bf0a8b145769)
	}

	// Zeros and other special values.
	vals := [...]uint64{0, sign, 1, 0x0008000000000000, 0x0010000000000000, 0x7ff0000000000000, 0xfff0000000000000, 0x7ff8000000000000, 0x7fefffffffffffff, 0x0000000000000001 | sign, one, one | sign, 0x3ff8000000000000}
	for _, a := range vals {
		for _, b := range vals {
			run("special", a, b)
		}
	}

	// One bit in the dividend, then in the divisor, the other 1.
	run("one", one, one)
	for k := uint(0); k < 52; k++ {
		run("abit", one|1<<k, one)
		run("bbit", one, one|1<<k)
		run("bits", one|1<<k, one|1<<k)
	}
	// Dividend's and divisor's runs of bits from the top and from the bottom.
	for k := uint(1); k <= 52; k++ {
		run("atop", one|frac&^(1<<(52-k)-1), one)
		run("alow", one|(1<<k-1), one)
		run("btop", one, one|frac&^(1<<(52-k)-1))
		run("blow", one, one|(1<<k-1))
		run("same", one|(1<<k-1), one|(1<<k-1))
	}
	// A quotient with a chosen number of set bits and an exact remainder of zero: q*b/b.
	// The dividend is q times a short divisor.
	for k := uint(0); k <= 40; k++ {
		q := uint64(1)<<k - 1
		run("exact3", uint64(0x3ff)<<52|(q*3)&frac, 0x4008000000000000)
		run("exact5", uint64(0x3ff)<<52|(q*5)&frac, 0x4014000000000000)
		run("exact2", one|q, 0x4000000000000000)
	}
	// Short divisors, 1/3 and 1/7 and the like, with long quotients.
	for d := uint64(3); d < 64; d += 2 {
		e := uint64(1023)
		v := d
		for v > 1 {
			v >>= 1
			e++
		}
		m := (d << (52 - (e - 1023))) & frac
		run("short", one, e<<52|m)
		run("short3", 0x4008000000000000, e<<52|m)
		run("shortr", e<<52|m, 0x4008000000000000)
	}

	// Below the normal range: the shortfall, one place at a time, then far. A power of two
	// over a power of two, and a longer quotient.
	for e := uint64(1); e < 90; e++ {
		run("denorm1", e<<52, uint64(1023+90-0)<<52)
		run("denorm3", e<<52|1<<51, uint64(1023+90)<<52)
		run("denormL", e<<52|0x5555555555555&frac, uint64(1023+90)<<52|0x2222222222222)
		run("denormN", sign|e<<52|0x5555555555555&frac, uint64(1023+90)<<52|0x2222222222222)
	}
	for e := uint64(1); e < 1023; e += 53 {
		run("denormF", e<<52, 0x7fe0000000000000)
		run("denormF3", e<<52|1<<51, 0x7fe0000000000000)
	}
	// Above the top, by sign and quotient.
	for _, g := range [...]uint64{0, sign} {
		run("over", g|0x7fe0000000000000, 0x3fe0000000000000)
		run("over", g|0x7fe8000000000000, 0x3fe0000000000000)
		run("over", g|0x7fe5555555555555, 0x3fe2222222222222)
		run("over", g|0x7fefffffffffffff, 0x3fe0000000000000)
		run("over", g|0x7fefffffffffffff, 0x3fe0000000000001)
		run("over", g|0x4000000000000000, 0x0010000000000000)
	}
	// Rounding that carries out of the top, and a quotient that rounds to a power of two.
	for k := uint(0); k < 8; k++ {
		run("rcarry", one|(1<<53-1)&frac, one+uint64(k))
		run("rcarry", 0x3fffffffffffffff, one+uint64(k)*2)
		run("rcarry", 0x7fefffffffffffff, 0x7fe0000000000000+uint64(k))
	}
	// A divisor near the dividend, or twice it: the two words about equal for a long way.
	for k := uint(0); k < 52; k += 3 {
		run("near", one, one|1<<k)
		run("near", one|1<<k, one)
		run("near", one|1<<k, one|1<<k|1)
		run("near", one|(1<<k-1), one|1<<k)
		run("near", one|0x80000000, one|1<<k)
		run("near", one|1<<k, one|0x80000000)
		run("near", one|0xffffffff, one|1<<k)
		run("near", one|1<<k, one|0xffffffff)
	}

	// At random: sparse and dense mantissas, the two exponents near.
	for range 1500 {
		r := rnd()
		m1, m2 := rnd(), rnd()
		if r&3 == 0 {
			m1 &= rnd() & rnd()
		}
		if r&12 == 0 {
			m2 &= rnd() & rnd()
		}
		if r&16 != 0 {
			m1 &^= (1 << (rnd() % 52)) - 1
		}
		if r&32 != 0 {
			m2 &^= (1 << (rnd() % 52)) - 1
		}
		e1 := uint64(1023 + (r>>8)%8)
		e2 := uint64(1023 + (r>>16)%8)
		run("random", e1<<52|m1&frac, e2<<52|m2&frac)
	}
	// Held out: none of these were looked at for the rule. Any exponent, any signs.
	s = 0xC2B2AE3D27D4EB4F
	for range 800 {
		r := rnd()
		m1, m2 := rnd(), rnd()
		if r&7 == 0 {
			m1 &^= (1 << (rnd() % 52)) - 1
		}
		if r&56 == 0 {
			m2 &^= (1 << (rnd() % 52)) - 1
		}
		e1, e2 := 400+(r>>8)%1200, 400+(r>>20)%1200
		run("fresh", r>>40<<63|e1<<52|m1&frac, r>>41<<63|e2<<52|m2&frac)
	}
	println("END")
	for {
		yaroze.VSync(0)
	}
}
