//go:build yaroze

package cleanlib

import "go.chrisrx.dev/psx/internal/volatile"

// The double precision math calls, worked out from Sony's results (spec
// 12 has the rules, one heading per family) in the library's own
// arithmetic (softfloat.go). sqrt and floor are the Seventh Edition Unix
// ones and log is Cody and Waite's; the rest are plainer than textbook
// forms: exp is its Taylor series summed until a term is negligible, the
// circular functions are continued fractions, the hyperbolic ones go
// through exp, and pow and hypot iterate. The arithmetic below is plain
// float64, which the compiler turns into calls of that arithmetic, in
// the order written; negation, which the compiler would do on the sign
// bit alone, goes through neg, and a quotient the compiler would turn
// into a product through div, as the library's own do.

// math_errno is the one word the library reports a math error in: a variable a
// program reads by name, and only the program clears. The library only writes
// it, so a plain store would go, and the variable with it, at link time.

//go:extern math_errno
var mathErrno uint32

// The two codes that also deliver a kernel event.
const (
	mathEDOM   = 33
	mathERANGE = 34

	mathEventClass = 0xf4000002
	mathEventEDOM  = 0x0301
	mathEventRange = 0x0302
)

// errMath is the one way the library reports a math error (spec 17): it sets
// math_errno to code, and for 33 and 34 alone delivers an event.
//
//export _err_math
func errMath(code int32) int32 {
	volatile.StoreUint32(&mathErrno, uint32(code))
	switch code {
	case mathEDOM:
		libps_biosB(biosDeliverEvent, mathEventClass, mathEventEDOM, 0, 0)
	case mathERANGE:
		libps_biosB(biosDeliverEvent, mathEventClass, mathEventRange, 0, 0)
	}
	return 0
}

func neg(x float64) float64   { return u2f(dNeg(f2u(x))) }
func toInt(x float64) int32   { return dFix(f2u(x)) }
func toFloat(i int32) float64 { return u2f(dFloat(i)) }

const pio2 = 1.570796326794896619231

// mathFrexp splits x into a mantissa from a half to one and a power of
// two: the power from the exponent field, the mantissa x scaled by minus
// it through the ldexp rules, so that at the top of the range it goes
// wrong as Sony's does (spec 12).
func mathFrexp(x float64) (float64, int) {
	b := f2u(x)
	if dIsZero(b) {
		return x, 0
	}
	e := int(b>>dFracBits&dExpMax) - 1022
	return mathLdexp(x, int32(-e)), e
}

// mathLdexp is x times 2^n by binary powering in the library's own
// arithmetic, which is what Sony's does (spec 12): for each set bit of |n|
// from the lowest, x is multiplied by the running power of two, which
// starts at 2, or at 1/2 for a negative n, and is squared each round. The
// products' overflow and underflow make everything the results show: a
// power past 1023 packs the exponent into the sign (2^-1024, the square
// of 2^-512, is the pattern of -infinity), so the bit of 1024 in a
// negative n flips the sign, and the squares above it overflow to
// +infinity. Sony's never returns for n = -2147483648, whose negation is
// itself; here the magnitude is taken unsigned and the loop ends.
func mathLdexp(x float64, n int32) float64 {
	b := 2.0
	m := uint32(n)
	if n < 0 {
		b = 0.5
		m = uint32(-n)
	}
	for ; m != 0; m >>= 1 {
		if m&1 != 0 {
			x = mul(x, b)
		}
		b = mul(b, b)
	}
	return x
}

// mathModf splits x into its integer part, the fraction bits cleared,
// zero below one whatever the sign, and x less that.
func mathModf(x float64) (frac, ip float64) {
	b := f2u(x)
	e := int(b>>dFracBits&dExpMax) - 1023
	switch {
	case e < 0:
		ip = 0
	case e >= dFracBits:
		ip = x
	default:
		ip = u2f(b &^ (dFracMask >> e))
	}
	return x - ip, ip
}

// splitBits is frexp done on the bits, as sqrt and log do it: the
// mantissa from a half to one and the exponent field less 1022, where
// frexp itself goes wrong at the top of the range.
func splitBits(x float64) (float64, int) {
	b := f2u(x)
	return u2f(b&^(dExpMax<<dFracBits) | 1022<<dFracBits), int(b>>dFracBits&dExpMax) - 1022
}

func mathFloor(d float64) float64 {
	if d < 0 {
		frac, ip := mathModf(neg(d))
		if frac != 0 {
			ip += 1
		}
		return neg(ip)
	}
	_, ip := mathModf(d)
	return ip
}

func mathCeil(d float64) float64 { return neg(mathFloor(neg(d))) }

// sqrt: Newton's iteration from a first guess half way between 1 and
// the argument's mantissa, with the exponent halved.
func mathSqrt(arg float64) float64 {
	if arg < 0 {
		errMath(mathEDOM)
	}
	if arg <= 0 {
		return 0
	}
	x, exp := splitBits(arg)
	for x < 0.5 {
		x *= 2
		exp--
	}
	if exp&1 != 0 {
		x *= 2
		exp--
	}
	temp := 0.5 * (1.0 + x)
	for exp > 60 {
		temp *= 1 << 30
		exp -= 60
	}
	for exp < -60 {
		temp /= 1 << 30
		exp += 60
	}
	if exp >= 0 {
		temp *= toFloat(1 << (exp / 2))
	} else {
		temp /= toFloat(1 << (-exp / 2))
	}
	for range 5 {
		temp = 0.5 * (temp + arg/temp)
	}
	return temp
}

// The circular functions are continued fractions (spec 12): tan f is
// f/(1 - z/(3 - z/(5 - ... - z/19))) with z = f*f, ten terms deep, and
// atan x is x/(1 + x²/(3 + 4x²/(5 + ... + 361x²/39))), nineteen deep. sin
// comes from the tangent of the half angle, cos from sin, asin and acos
// from atan, and atan2 from asin of the ratio to hypot.
const pi = 3.14159265358979323846

// cfTan is the tangent of f by its continued fraction, ten terms deep. A
// zero f gives +0 whatever its sign: that is where tan(-0) = +0 comes
// from, and sin of the one argument whose half is a zero quotient (spec
// 12, sin).
func cfTan(f float64) float64 {
	if f == 0 {
		return 0
	}
	z := f * f
	d := 19.0
	for k := int32(9); k >= 1; k-- {
		d = toFloat(2*k-1) - div(z, d)
	}
	return div(f, d)
}

// cfAtan is the arc tangent of x by its continued fraction, nineteen
// terms deep. The term's numerator is k*k*x*x taken left to right, the
// square of k times x and then times x again, not k*k times a square of
// x worked out once: the two round differently, and for a denormal x the
// first makes the garbage Sony's does (spec 12).
func cfAtan(x float64) float64 {
	d := 39.0
	for k := int32(19); k >= 1; k-- {
		d = toFloat(2*k-1) + div(toFloat(k*k)*x*x, d)
	}
	return div(x, d)
}

// piReduce takes x, of either sign, to f = x - N pi for the nearest N,
// pi in Cody and Waite's two parts; N comes from a quotient by pi (a
// product by 1/pi underflows for the smallest x), rounded away from zero
// on the signed x as tan's is. The reduction is of the signed argument,
// not of its magnitude: the sum rounds the lined-up addend N*C2 one way
// for a positive addend and the other for a negative one, so sin(-x) is
// not always -sin(x) (spec 12, sin, has the probed cases), and N
// saturates to -2^31 below -pi*2^31, where Sony's never returns.
const (
	sinC1 = 3.1416015625
	sinC2 = -8.908910206761537356617e-6
)

func piReduce(x float64) (f float64, n int32) {
	y := div(x, pi)
	if x < 0 {
		n = toInt(y - 0.5)
	} else {
		n = toInt(y + 0.5)
	}
	xn := toFloat(n)
	return sub(sub(x, xn*sinC1), xn*sinC2), n
}

// mathSin: sin f = 2t/(1 + t*t) with t the tangent of f/2 by the
// fraction, the odd multiples of pi put back by a product with -1. A zero
// result's sign is the chain's own (a zero quotient takes the XOR of its
// operands' signs), and either zero argument gives +0 through the
// fraction's zero case.
func mathSin(x float64) float64 {
	f, n := piReduce(x)
	t := cfTan(div(f, 2))
	s := div(2*t, 1+t*t)
	if n&1 != 0 {
		s *= -1
	}
	return s
}

func mathCos(x float64) float64 { return mathSin(pio2 - abs(x)) }

// mathTan: Cody and Waite's reduction by pi/2 in two parts, N rounded
// away from zero on the signed x; an odd N gives -1/tan(f) by the
// fraction, so that tan(pi/2) is the reciprocal of the constant's own
// error. tan(-0) = +0 through the fraction's zero case (a zero x reduces
// to itself).
const (
	tanC1 = 1.57080078125
	tanC2 = -4.454455103380768678308e-6
)

func mathTan(x float64) float64 {
	y := div(x, pio2)
	var n int32
	if x < 0 {
		n = toInt(y - 0.5)
	} else {
		n = toInt(y + 0.5)
	}
	xn := toFloat(n)
	t := cfTan(sub(sub(x, xn*tanC1), xn*tanC2))
	if n&1 != 0 {
		return div(-1, t)
	}
	return t
}

func mathAtan(x float64) float64 {
	a := abs(x)
	var r float64
	if a > 1 {
		r = pio2 - cfAtan(div(1, a))
	} else {
		r = cfAtan(a)
	}
	if x < 0 {
		return neg(r)
	}
	return r
}

func mathAsin(x float64) float64 {
	if x > 1 || x < -1 {
		errMath(mathEDOM)
		return 0
	}
	return mathAtan(div(x, mathSqrt(1-x*x)))
}

func mathAcos(x float64) float64 {
	if x > 1 || x < -1 {
		errMath(mathEDOM)
		return 0
	}
	return pio2 - mathAsin(x)
}

// mathAtan2: asin of y over the hypotenuse, moved to the left half plane
// for a negative x. Matched only as far as hypot is (spec 12).
func mathAtan2(y, x float64) float64 {
	if x == 0 && y == 0 {
		errMath(mathEDOM)
		return 0
	}
	h := mathHypot(x, y)
	if h == 0 {
		return 0
	}
	a := mathAsin(div(y, h))
	if x < 0 {
		if y < 0 {
			return neg(pi) - a
		}
		return pi - a
	}
	return a
}

// div, sub and mul go through the library's own quotient, difference and
// product where the compiler would rewrite the operation: a quotient by
// a power of two into a product, which has no denormal result (the
// series below depend on one, spec 12, exp), a - b*c into a + b*(-c),
// which for a zero b adds a negative zero to a and so returns -0 where
// the difference returns a, and a chain of products of constants into
// the exact constants (strtod's powers of ten).
func div(a, b float64) float64 { return u2f(dDiv(f2u(a), f2u(b))) }
func sub(a, b float64) float64 { return u2f(dSub(f2u(a), f2u(b))) }
func mul(a, b float64) float64 { return u2f(dMul(f2u(a), f2u(b))) }

// exp: the Taylor series of e^g summed term by term, g the argument less
// the nearest multiple of ln 2 taken as one constant, and the sum scaled
// by that power of two through the ldexp rules. The series is summed for
// |g| and inverted for a negative g, so that e^-x is exactly 1/e^x.
const (
	ln2 = 0.69314718055994530942

	// seriesEps ends a series once a term falls below it: any value
	// between 1e-20 and 1.2e-16 gives Sony's results on every probed
	// input.
	seriesEps = 1e-17
)

// expSeries sums 1 + g + g^2/2! + ..., each term the last times g/k, until
// a term is below seriesEps (that term is dropped).
func expSeries(g float64) float64 {
	sum := 1 + g
	t := g
	for k := int32(2); k < 100; k++ {
		t = t * div(g, toFloat(k))
		if abs(t) < seriesEps {
			break
		}
		sum += t
	}
	return sum
}

func abs(x float64) float64 {
	if x < 0 {
		return neg(x)
	}
	return x
}

// mathExp's N is the nearest integer to x/ln2, the quotient and not a
// product by 1/ln2: the two round to different integers where x is within
// a few ulps of (k + 1/2) ln2, which is where pow(2, k + 1/2) lands
// (spec 12, exp).
func mathExp(x float64) float64 {
	if x == 0 {
		return 1
	}
	var n int32
	if x < 0 {
		n = toInt(div(x, ln2) - 0.5)
	} else {
		n = toInt(div(x, ln2) + 0.5)
	}
	g := sub(x, toFloat(n)*ln2)
	inv := g < 0
	if inv {
		g = neg(g)
	}
	s := expSeries(g)
	if inv {
		s = div(1, s)
	}
	return mathLdexp(s, n)
}

// log: the mantissa's log by the series 2(z + z^3/3 + z^5/5 + ...) in
// z = (f-1)/(f+1), summed until a term no longer changes the sum, plus
// the exponent's times ln 2 as one constant; f is doubled below sqrt(1/2)
// so that |z| stays under 0.172 (about twenty terms). Zero and negative
// arguments give 0 (the manual says 1 for zero: Sony's library gives 0).
const (
	sqrto2 = 0.70710678118654752440
	ln10   = 2.30258509299404568402
)

func mathLog(x float64) float64 {
	if x < 0 {
		errMath(mathEDOM)
		return 0
	}
	if x == 0 {
		errMath(mathERANGE)
		return 0
	}
	// A number near the ends of the range reports a range error, though the
	// result that follows is the usual one.
	if field := f2u(x) >> dFracBits & dExpMax; field < 512 || field >= 1535 {
		errMath(mathERANGE)
	}
	f, n := splitBits(x)
	// At the two ends of the exponent field Sony's split goes its own way
	// (spec 12, log): at a field of 0 the mantissa is never doubled, and
	// at a field of 2047 it always is, with n lowered only where the test
	// says so, so that log(NaN) reads the pattern as twice the number the
	// arithmetic does.
	field := f2u(x) >> dFracBits & dExpMax
	small := f < sqrto2
	switch {
	case field == 0:
	case field == dExpMax:
		f *= 2
		if small {
			n--
		}
	case small:
		f *= 2
		n--
	}
	z := (f - 1) / (f + 1)
	w := z * z
	sum := z
	t := z
	for k := int32(3); k < 200; k += 2 {
		t *= w
		next := sum + div(t, toFloat(k))
		if next == sum {
			break
		}
		sum = next
	}
	return toFloat(int32(n))*ln2 + 2*sum
}

func mathLog10(arg float64) float64 {
	return mathLog(arg) / ln10
}

// pow: an integral power by repeated squaring, of 1/x for a negative
// one; otherwise exp(y log x), and 0 for a negative base.
func mathPow(x, y float64) float64 {
	if x == 0 {
		if y <= 0 {
			errMath(mathEDOM)
			return 1
		}
		return 0
	}
	if y == 0 {
		return 1
	}
	n := toInt(y)
	if toFloat(n) == y {
		return powInt(x, n)
	}
	if x < 0 {
		errMath(mathEDOM)
		return 0
	}
	return mathExp(y * mathLog(x))
}

// powInt is x^n, the bits of |n| from the lowest, the base squared for
// each; 1/x is the base for a negative n.
func powInt(x float64, n int32) float64 {
	if n < 0 {
		x = div(1, x)
		n = -n
	}
	p := 1.0
	b := x
	for n > 0 {
		if n&1 != 0 {
			p *= b
		}
		n >>= 1
		// Not squared past the top bit, as math_errno shows: pow(2, 1000)
		// reports no overflow. By name, since the compiler would otherwise
		// work the plain product out whether it is used or not.
		if n > 0 {
			b = mul(b, b)
		}
	}
	return p
}

// sinh, cosh and tanh: through exp, but for the smallest arguments the
// first two terms of their series.
const smallHyp = 1e-3

func mathSinh(x float64) float64 {
	if abs(x) < smallHyp {
		z := x * x
		return x * (1 + div(z, 6))
	}
	e := mathExp(x)
	return div(e-div(1, e), 2)
}

func mathCosh(x float64) float64 {
	e := mathExp(x)
	return div(e+div(1, e), 2)
}

func mathTanh(x float64) float64 {
	if abs(x) < smallHyp {
		z := x * x
		return x * (1 - div(z, 3))
	}
	return div(2, 1+mathExp(x*-2)) - 1
}

// hypot: Moler and Morrison's iteration, three times over, from the
// larger magnitude, with the ratio, its square and the step's factor in
// single precision (C floats, so the garbage for the smallest ratios is
// the single arithmetic's) and the swap of the two through a float too
// (hypot(1/3, 1) differs from hypot(1, 1/3)). The step's correction is
// 2*p*s in that order, p doubled first: once p is above 2^1023 (the
// garbage the smallest denormals make of it) the doubling overflows to
// the infinity pattern, 2^1024 to this arithmetic, and the correction is
// s times that whatever p's fraction. Spec 12 has the evidence.
func mathHypot(x, y float64) float64 {
	p, q := abs(x), abs(y)
	if p < q {
		p, q = q, u2f(sExtend(sTrunc(f2u(p))))
	}
	// A zero q runs no step, as math_errno shows: its quotient by a zero p
	// would report an overflow in the narrowing.
	if dIsZero(f2u(q)) {
		return p
	}
	const four = 0x40800000
	for range 3 {
		t := sTrunc(dDiv(f2u(q), f2u(p)))
		r := sMul(t, t)
		s := u2f(sExtend(sDiv(r, sAdd(four, r))))
		p += 2 * p * s
		q = s * q
	}
	return p
}

// fmod: x less the whole multiples of y, with x's sign; 0 for y 0.
func mathFmod(x, y float64) float64 {
	if y == 0 {
		return 0
	}
	_, ip := mathModf(x / y)
	return x - ip*y
}
