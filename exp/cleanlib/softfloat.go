//go:build yaroze

package cleanlib

import (
	"math"
	"math/bits"
)

// Software floating point. The R3000A has no floating point unit, so the
// compiler turns every float64 and float32 operation into a call to a
// helper (__adddf3 and so on), and the resident library supplies them:
// a program built for the console calls these for its own arithmetic as
// the math functions do for theirs. They are worked out from Sony's
// results (spec 12) and differ from IEEE 754 in ways a program can see:
//
//   - Every bit pattern but zero is a normal number: the exponent field
//     is never special, so a denormal reads as 1.f times 2^-1023, an
//     infinity as 2^1024 and a NaN as 1.f times 2^1024. Zero is all
//     bits zero, either sign.
//   - Sums are rounded to nearest, ties to even, from nine bits below the
//     last place; an addend shifted to line up loses what falls past
//     those as a two's complement number would, toward minus infinity.
//     Products round half up on a product formed from 32-bit halves with
//     its lowest parts left out (dMul), quotients to nearest.
//   - A result too large is an infinity of its sign. One too small for a
//     normal number is packed anyway, its exponent below zero running
//     into the sign bit; division alone makes a denormal of it, by a
//     shift whose rounding half is a 32-bit quantity. A single precision
//     sum that small is an infinity of its sign and a range error instead
//     (spec 26's sweep of __subsf3).
//   - Comparisons put the bit patterns in order as sign and magnitude,
//     with the two zeros equal.
//   - A result that overflows, in the sense of the bullet above or by a
//     conversion to an integer past its range, reports it through _err_math.
//     Underflow and a quotient by zero do not.

const (
	dSign     = 1 << 63
	dFracBits = 52
	dFracMask = 1<<dFracBits - 1
	dHidden   = 1 << dFracBits
	dExpMax   = 0x7ff
	dInf      = dExpMax << dFracBits

	// guard is how many bits below the last place a result is worked
	// out to before it is rounded.
	guard = 9
)

func dIsZero(a uint64) bool { return a<<1 == 0 }

// dUnpack splits a into its sign, exponent field and significand with
// the hidden bit, which every pattern has.
func dUnpack(a uint64) (s uint64, e int, m uint64) {
	return a & dSign, int(a >> dFracBits & dExpMax), a&dFracMask | dHidden
}

// dPack rounds m, a significand whose point is guard bits above its
// bottom, to 53 bits and packs it with sign s and exponent e. A result
// whose exponent passes the top is an infinity; one below the bottom is
// packed as it is, the negative exponent running into the sign.
func dPack(s uint64, e int, m uint64, halfUp bool) uint64 {
	m, e = dNormalize(m, e, false)
	if halfUp {
		m = m>>guard + m>>(guard-1)&1
	} else {
		m = roundEven(m)
	}
	if m>>(dFracBits+1) != 0 {
		m >>= 1
		e++
	}
	if e >= dExpMax {
		errMath(mathERANGE)
		return s | dInf
	}
	return s | uint64(int64(e))<<dFracBits | m&dFracMask
}

// dNormalize shifts m so that its top bit is the hidden bit's place
// above guard bits, adjusting e, and keeps what a right shift drops as a
// sticky bit if sticky is set.
func dNormalize(m uint64, e int, sticky bool) (uint64, int) {
	top := 63 - bits.LeadingZeros64(m)
	want := dFracBits + guard
	switch {
	case top > want:
		sh := top - want
		lost := m << (64 - sh)
		m >>= sh
		if sticky && lost != 0 {
			m |= 1
		}
		e += sh
	case top < want:
		m <<= want - top
		e -= want - top
	}
	return m, e
}

// roundEven drops the guard bits of m, rounding to nearest, ties to
// even.
func roundEven(m uint64) uint64 {
	const half = 1 << (guard - 1)
	low := m & (1<<guard - 1)
	m >>= guard
	if low > half || low == half && m&1 != 0 {
		m++
	}
	return m
}

// dAddMag adds a and b, both nonzero, as signed significands: the
// smaller is shifted to line up as a two's complement number would be, so
// a negative one loses its low bits toward minus infinity and a positive
// one toward zero.
func dAddMag(a, b uint64) uint64 {
	sa, ea, ma := dUnpack(a)
	sb, eb, mb := dUnpack(b)
	if eb > ea || eb == ea && mb > ma {
		sa, ea, ma, sb, eb, mb = sb, eb, mb, sa, ea, ma
	}
	d := ea - eb
	if d > dFracBits+2 {
		// The smaller cannot reach the larger's last place.
		return sa | uint64(ea)<<dFracBits | ma&dFracMask
	}
	ma <<= guard
	mb <<= guard
	if d > 0 {
		lost := mb << (64 - d)
		mb >>= d
		if sb != 0 && lost != 0 {
			mb++
		}
	}
	var m uint64
	if sa == sb {
		m = ma + mb
	} else {
		m = ma - mb
	}
	if m == 0 {
		return 0
	}
	return dPack(sa, ea, m, false)
}

func dAdd(a, b uint64) uint64 {
	if dIsZero(a) {
		return b
	}
	if dIsZero(b) {
		return a
	}
	return dAddMag(a, b)
}

func dSub(a, b uint64) uint64 {
	if dIsZero(b) {
		return a
	}
	if dIsZero(a) {
		return b ^ dSign
	}
	return dAddMag(a, b^dSign)
}

// dMul forms the product as Sony's does: each significand shifted up so
// that its hidden bit is the top bit of a 64-bit word and split into
// 32-bit halves, and the product's top 64 bits taken as the high halves'
// product plus the high words of the two cross products. The low halves'
// product and the cross products' low words are never added, so the
// product comes out short by up to three times 2^-10 of an ulp, which
// shows when the exact product lies within that of a half ulp: it is
// rounded down where the exact product would round up (spec 12 has the
// cases, atan's fraction among them).
func dMul(a, b uint64) uint64 {
	s := (a ^ b) & dSign
	if dIsZero(a) || dIsZero(b) {
		return s
	}
	_, ea, ma := dUnpack(a)
	_, eb, mb := dUnpack(b)
	const up = 63 - dFracBits
	ah, al := ma<<up>>32, ma<<up&0xffffffff
	bh, bl := mb<<up>>32, mb<<up&0xffffffff
	p := ah*bh + ah*bl>>32 + al*bh>>32
	// p's top bit is bit 63 or 62; dPack wants the hidden bit's place
	// at bit 61 with guard bits below, or one above it.
	return dPack(s, ea+eb-1023, p>>1, true)
}

func dDiv(a, b uint64) uint64 {
	r, _ := dDivQuotient(a, b)
	return r
}

// dDivQuotient is dDiv, also returning the quotient's bits to two below the last
// place, for what they say of the time Sony's takes, or 0 when one of them is zero.
func dDivQuotient(a, b uint64) (uint64, uint64) {
	s := (a ^ b) & dSign
	if dIsZero(b) {
		return s | (dSign - 1), 0
	}
	if dIsZero(a) {
		return s, 0
	}
	_, ea, ma := dUnpack(a)
	_, eb, mb := dUnpack(b)
	// The quotient with guard bits below the last place, the remainder
	// kept as a sticky bit.
	q, r := bits.Div64(ma>>(64-dFracBits-guard), ma<<(dFracBits+guard), mb)
	whole := q >> (guard - 2)
	if r != 0 {
		q |= 1
	}
	e := ea - eb + 1023
	q, e = dNormalize(q, e, true)
	if e > 0 {
		m := roundEven(q)
		if m>>(dFracBits+1) != 0 {
			m >>= 1
			e++
		}
		if e >= dExpMax {
			errMath(mathERANGE)
			return s | dInf, whole
		}
		return s | uint64(e)<<dFracBits | m&dFracMask, whole
	}
	// Too small for a normal number: a denormal. The significand with
	// one guard bit below it gets a half added at the shortfall's bit and
	// is shifted down by the shortfall plus one, and the hidden bit's
	// place is cleared, so a carry into it is lost. The half is a 32-bit
	// shift: from a shortfall of 32 it lands far below the last place and
	// the result is truncated, an ulp below the half-up rounding of
	// smaller shortfalls. From a shortfall of 63 the result is a signed
	// zero.
	sh := 1 - e
	if sh >= 63 {
		return s, whole
	}
	y := q >> (guard - 1)
	m := (y + uint64(uint32(1)<<(uint(sh)&31))) >> uint(sh+1)
	return s | m&^dHidden, whole
}

// dCompare orders a and b as sign and magnitude, the zeros equal: -1,
// 0 or 1.
func dCompare(a, b uint64) int {
	if dIsZero(a) {
		a = 0
	}
	if dIsZero(b) {
		b = 0
	}
	if a == b {
		return 0
	}
	sa, sb := a>>63, b>>63
	if sa != sb {
		if sa != 0 {
			return -1
		}
		return 1
	}
	less := a&^dSign < b&^dSign
	if sa != 0 {
		less = !less
	}
	if less {
		return -1
	}
	return 1
}

// dNeg flips the sign; either zero gives positive zero.
func dNeg(a uint64) uint64 {
	if dIsZero(a) {
		return 0
	}
	return a ^ dSign
}

// dFix converts a to an integer, toward zero, a value past the range
// giving its end. Negative zero gives the most negative integer.
func dFix(a uint64) int32 {
	if a == 0 {
		return 0
	}
	if a == dSign {
		return -1 << 31
	}
	s, e, m := dUnpack(a)
	var v int64
	switch {
	case e < 1023:
		v = 0
	case e >= 1023+31:
		errMath(mathERANGE)
		v = 1 << 31
	default:
		v = int64(m >> (1023 + dFracBits - e))
	}
	if s != 0 {
		v = -v
		if v < -1<<31 {
			v = -1 << 31
		}
	} else if v > 1<<31-1 {
		v = 1<<31 - 1
	}
	return int32(v)
}

// dFloat converts i exactly. (Sony's __floatsidf never returns for the
// most negative integer; the export does that, not this.)
func dFloat(i int32) uint64 {
	if i == 0 {
		return 0
	}
	var s uint64
	v := uint32(i)
	if i < 0 {
		s, v = dSign, -v
	}
	n := bits.LeadingZeros32(v)
	return s | uint64(1023+31-n)<<dFracBits | uint64(v)<<(n+21)&dFracMask
}

func f2u(x float64) uint64 { return math.Float64bits(x) }
func u2f(u uint64) float64 { return math.Float64frombits(u) }

// Single precision, from Sony's results as the double above: every
// pattern but zero a normal number, products rounded half up, sums and
// quotients to nearest even, a sum's or product's result below the
// normal range packed with its exponent running into the sign, and a
// quotient's or a narrowing's made a denormal by sDenorm. Sums carry 6
// bits below the last place (the double's 9 give one sum of 641 probed
// wrong); a zero divisor gives 2^31 (4f000000) or, with the signs
// differing, 2^32 (4f800000).
const (
	sSign     = 1 << 31
	sFracBits = 23
	sFracMask = 1<<sFracBits - 1
	sHidden   = 1 << sFracBits
	sExpMax   = 0xff
	sInf      = sExpMax << sFracBits
	sGuard    = 6
)

func sIsZero(a uint32) bool { return a<<1 == 0 }

func sUnpack(a uint32) (s uint32, e int, m uint64) {
	return a & sSign, int(a >> sFracBits & sExpMax), uint64(a&sFracMask | sHidden)
}

func sNormalize(m uint64, e int, sticky bool) (uint64, int) {
	top := 63 - bits.LeadingZeros64(m)
	want := sFracBits + sGuard
	switch {
	case top > want:
		sh := top - want
		lost := m << (64 - sh)
		m >>= sh
		if sticky && lost != 0 {
			m |= 1
		}
		e += sh
	case top < want:
		m <<= want - top
		e -= want - top
	}
	return m, e
}

func sRoundEven(m uint64) uint64 {
	const half = 1 << (sGuard - 1)
	low := m & (1<<sGuard - 1)
	m >>= sGuard
	if low > half || low == half && m&1 != 0 {
		m++
	}
	return m
}

func sPack(s uint32, e int, m uint64, halfUp bool) uint32 {
	m, e = sNormalize(m, e, false)
	if halfUp {
		m = m>>sGuard + m>>(sGuard-1)&1
	} else {
		m = sRoundEven(m)
	}
	if m>>(sFracBits+1) != 0 {
		m >>= 1
		e++
	}
	if e >= sExpMax || e < 0 {
		errMath(mathERANGE)
		return s | sInf
	}
	return s | uint32(int32(e))<<sFracBits | uint32(m)&sFracMask
}

func sAddMag(a, b uint32) uint32 {
	sa, ea, ma := sUnpack(a)
	sb, eb, mb := sUnpack(b)
	if eb > ea || eb == ea && mb > ma {
		sa, ea, ma, sb, eb, mb = sb, eb, mb, sa, ea, ma
	}
	d := ea - eb
	if d > sFracBits+2 {
		return sa | uint32(ea)<<sFracBits | uint32(ma)&sFracMask
	}
	ma <<= sGuard
	mb <<= sGuard
	if d > 0 {
		lost := mb << (64 - d)
		mb >>= d
		if sb != 0 && lost != 0 {
			mb++
		}
	}
	var m uint64
	if sa == sb {
		m = ma + mb
	} else {
		m = ma - mb
	}
	if m == 0 {
		return 0
	}
	return sPack(sa, ea, m, false)
}

func sAdd(a, b uint32) uint32 {
	if sIsZero(a) {
		return b
	}
	if sIsZero(b) {
		return a
	}
	return sAddMag(a, b)
}

// sMul forms the product of the significands from their bytes, as
// Sony's does: the nine byte products are summed at their weights into
// the top 32 bits of the 48-bit product (bits 47 to 16), the lowest
// product, a0*b0, dropped and the two of weight 2^8 taken without their
// low byte, and the sum is rounded half up to 24 bits. What the dropped
// bits would have carried into bit 16 is lost, which shows at near ties:
// a product whose exact bits below the last place are a half, zeros and
// then something rounds down when the something came from the dropped
// bytes (0x80152f squared is 0x802a61, not 0x802a62), while an exact tie
// rounds up.
func sMul(a, b uint32) uint32 {
	s := (a ^ b) & sSign
	if sIsZero(a) || sIsZero(b) {
		return s
	}
	_, ea, ma64 := sUnpack(a)
	_, eb, mb64 := sUnpack(b)
	ma, mb := uint32(ma64), uint32(mb64)
	a2, a1, a0 := ma>>16, ma>>8&0xff, ma&0xff
	b2, b1, b0 := mb>>16, mb>>8&0xff, mb&0xff
	t := a2*b2<<16 + (a2*b1+a1*b2)<<8 + a2*b0 + a1*b1 + a0*b2 + a1*b0>>8 + a0*b1>>8
	e := ea + eb - 127
	var m uint32
	if t>>31 != 0 {
		m = (t + 0x80) >> 8
		e++
	} else {
		m = (t + 0x40) >> 7
	}
	if e >= sExpMax {
		errMath(mathERANGE)
		return s | sInf
	}
	return s | uint32(int32(e))<<sFracBits | m&sFracMask
}

func sDiv(a, b uint32) uint32 {
	s := (a ^ b) & sSign
	if sIsZero(b) {
		if s != 0 {
			return 0x4f800000
		}
		return 0x4f000000
	}
	if sIsZero(a) {
		return s
	}
	_, ea, ma := sUnpack(a)
	_, eb, mb := sUnpack(b)
	// The quotient of ma<<29 by mb, by the CPU's own division a few bits at a time,
	// as the remainder is under 24 bits.
	d := uint32(mb)
	w := uint32(ma) / d
	r := uint32(ma) % d
	for _, k := range [...]uint32{8, 8, 8, sFracBits + sGuard - 24} {
		r <<= k
		w = w<<k | r/d
		r %= d
	}
	q := uint64(w)
	if r != 0 {
		q |= 1
	}
	e := ea - eb + 127
	q, e = sNormalize(q, e, true)
	if e > 0 {
		m := sRoundEven(q)
		if m>>(sFracBits+1) != 0 {
			m >>= 1
			e++
		}
		if e >= sExpMax {
			errMath(mathERANGE)
			return s | sInf
		}
		return s | uint32(e)<<sFracBits | uint32(m)&sFracMask
	}
	return sDenorm(s, uint32(q>>(sGuard-1)), 1-e)
}

// sDenorm packs a result below the single range as Sony's quotient and
// narrowing do. y is the 24-bit significand with its hidden bit at bit
// 24 and one guard bit below it; sh is the shortfall, how far the
// exponent is below the smallest normal's. A half is added at bit sh and
// the sum shifted right by sh+1, both as 32-bit operations whose shift
// amounts are taken modulo 32, and the hidden bit's place is cleared. So
// up to a shortfall of 23 the result is rounded half up, with a carry
// into bit 23 lost (2^-126(2-2^-23) / 2 = 0); a shortfall of 24 always
// gives 1 and 25 to 30 give 0; 31 shifts by nothing and keeps the half,
// bit 31, with y as it is (00800000 / 7f000000 = 81000000); and from 32
// on these repeat.
func sDenorm(s uint32, y uint32, sh int) uint32 {
	m := (y + 1<<(uint(sh)&31)) >> (uint(sh+1) & 31)
	return s | m&^sHidden
}

// sTrunc narrows a double: to nearest (half up), an infinity from the
// top of the single range up (NaNs included), and below the bottom of
// the range sDenorm's packing of the significand cut to 24 bits and a
// guard bit; so 2^-1023 (a shortfall of 897) gives 00400000, 1e-300
// (871) gives 000156e2 and 2^-1022 (896) gives 0.
func sTrunc(a uint64) uint32 {
	if dIsZero(a) {
		return uint32(a >> 32)
	}
	s := uint32(a>>63) << 31
	e := int(a>>dFracBits&dExpMax) - 1023 + 127
	if e >= sExpMax {
		errMath(mathERANGE)
		return s | sInf
	}
	y := uint32((a&dFracMask | dHidden) >> (dFracBits - sFracBits - 1))
	if e <= 0 {
		return sDenorm(s, y, 1-e)
	}
	r := (y + 1) >> 1
	if r>>(sFracBits+1) != 0 {
		r >>= 1
		e++
		if e >= sExpMax {
			errMath(mathERANGE)
			return s | sInf
		}
	}
	return s | uint32(e)<<sFracBits | r&sFracMask
}

// sExtend widens a single: a denormal is normalized (its true value),
// every other pattern but zero a normal number of its field.
func sExtend(a uint32) uint64 {
	if sIsZero(a) {
		return uint64(a) << 32
	}
	s := uint64(a>>31) << 63
	e := int(a>>sFracBits&sExpMax) - 127 + 1023
	m := uint64(a & sFracMask)
	if a>>sFracBits&sExpMax == 0 {
		for m&sHidden == 0 {
			m <<= 1
			e--
		}
		e++
		m &= sFracMask
	}
	return s | uint64(e)<<dFracBits | m<<(dFracBits-sFracBits)
}
