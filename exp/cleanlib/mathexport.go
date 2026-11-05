//go:build yaroze

package cleanlib

import "math/bits"

// The C names of the math calls and of the software floating point
// helpers. The helpers take and return the bits, which the o32 soft-float
// convention passes as it does a double: in a register pair.

//export __adddf3
func softAdddf3(a, b uint64) uint64 { return dAdd(a, b) }

//export __subdf3
func softSubdf3(a, b uint64) uint64 { return dSub(a, b) }

//export __muldf3
func softMuldf3(a, b uint64) uint64 { return dMul(a, b) }

//export __divdf3
func softDivdf3(a, b uint64) uint64 { return dDiv(a, b) }

//export __negdf2
func softNegdf2(a uint64) uint64 { return dNeg(a) }

// The comparisons return what Sony's do: equal or not as 0 or 1; less
// as -1 or 0 for lt and ge; greater as 1 or 0 for gt and le.

//export __eqdf2
func softEqdf2(a, b uint64) int32 { return cmpNe(a, b) }

//export __nedf2
func softNedf2(a, b uint64) int32 { return cmpNe(a, b) }

//export __ltdf2
func softLtdf2(a, b uint64) int32 { return cmpLess(a, b) }

//export __gedf2
func softGedf2(a, b uint64) int32 { return cmpLess(a, b) }

//export __gtdf2
func softGtdf2(a, b uint64) int32 { return cmpGreater(a, b) }

//export __ledf2
func softLedf2(a, b uint64) int32 { return cmpGreater(a, b) }

func cmpNe(a, b uint64) int32 {
	if dCompare(a, b) != 0 {
		return 1
	}
	return 0
}

func cmpLess(a, b uint64) int32 {
	if dCompare(a, b) < 0 {
		return -1
	}
	return 0
}

func cmpGreater(a, b uint64) int32 {
	if dCompare(a, b) > 0 {
		return 1
	}
	return 0
}

//export __fixdfsi
func softFixdfsi(a uint64) int32 { return dFix(a) }

// Programs reach the arithmetic through the entries below, which take as long
// as Sony's on what they are given (spec 21). The library's own arithmetic,
// the compiler's calls of __muldf3 and the rest above, stays as quick as it is:
// the math calls are paced as a whole. Sony's figures are much the same for any
// two nonzero values, 3,900 for a product, and small for a zero: 90 and 48. A
// quotient goes by its bits, as a probe of 378 divisions found: 10,680 and 260 for
// each one bit of the quotient to two places below its last, all within 340.
const (
	programMulPad     = 3602
	programMulZeroPad = 38
	programDivPad     = 400
	programDivBitPad  = 260
	programFixPad     = 290
	programFixZeroPad = 14
	programFloatPad   = 302
)

//export libps_c___muldf3
func programMuldf3(a, b uint64) uint64 {
	r := dMul(a, b)
	if dIsZero(a) || dIsZero(b) {
		pace(programMulZeroPad)
	} else {
		pace(programMulPad)
	}
	return r
}

//export libps_c___divdf3
func programDivdf3(a, b uint64) uint64 {
	r, q := dDivQuotient(a, b)
	if q != 0 {
		spinCycles(programDivPad + programDivBitPad*uint32(bits.OnesCount64(q)))
	}
	return r
}

//export libps_c___fixdfsi
func programFixdfsi(a uint64) int32 {
	r := dFix(a)
	if dIsZero(a) {
		pace(programFixZeroPad)
	} else {
		pace(programFixPad)
	}
	return r
}

// __floatsidf never returns for the most negative integer, as Sony's.
//
//export __floatsidf
func softFloatsidf(i int32) uint64 {
	if i == -1<<31 {
		for {
		}
	}
	return dFloat(i)
}

// Sony's takes 450 cycles for any integer but 0.
//
//export libps_c___floatsidf
func programFloatsidf(i int32) uint64 {
	r := softFloatsidf(i)
	if i != 0 {
		pace(programFloatPad)
	}
	return r
}

// Single precision: the same arithmetic on 24-bit significands, worked
// out from Sony's results as the double was (softfloat.go).

//export __extendsfdf2
func softExtendsfdf2(a uint32) uint64 { return sExtend(a) }

//export __truncdfsf2
func softTruncdfsf2(a uint64) uint32 { return sTrunc(a) }

//export __addsf3
func softAddsf3(a, b uint32) uint32 { return sAdd(a, b) }

//export __mulsf3
func softMulsf3(a, b uint32) uint32 { return sMul(a, b) }

//export __divsf3
func softDivsf3(a, b uint32) uint32 { return sDiv(a, b) }

//export _get_errno
func getErrno() int32 { return libps_biosB(biosGetErrno, 0, 0, 0, 0) }

// sqrt, sin and cos take as long as Sony's, which get there by the long way
// around (spec 21, "Costs"). The pads are in the entries alone, so that asin
// and the others that use them inside cost what they did. Sony's sin and cos go
// up and down by some 10,000 cycles with the argument, in no plain way; these are the
// means over a probe's 2,000 arguments, a sign making no difference.
const (
	sinZeroPad  = 6600   // Sony's 7,500 at 0
	sinNearPad  = 132900 // 266,700 below pi/2
	sinFarPad   = 143100 // and 279,000 from there
	cosTinyPad  = 118100 // Sony's 254,300 at 0 and below about 1e-18
	cosNearPad  = 134600 // 268,800 below pi
	cosFarPad   = 145400 // and 281,000 from there
	sqrtBasePad = 140500 // Sony's about 205,000, less the 54,500 of the work and a blank's
	sqrtExpPad  = 20000  // and 20,000 for each power of two from 1

	// The bits of pi/2 and pi, and of the 2^-60 where cos stops being tiny.
	bitsHalfPi = 0x3ff921fb54442d18
	bitsPi     = 0x400921fb54442d18
	bitsTiny   = (1023 - 60) << dFracBits
)

//export sqrt
func libSqrt(x float64) float64 {
	r := mathSqrt(x)
	// Nothing to spend on 0, on 1, whose Sony's is quicker, or below 0, not
	// measured.
	if b := f2u(x); b != 0 && b != 1023<<dFracBits && b&dSign == 0 {
		e := int(b>>dFracBits&dExpMax) - 1023
		spinCycles(sqrtBasePad + sqrtExpPad*uint32(max(e, -e)))
	}
	return r
}

//export sin
func libSin(x float64) float64 {
	r := mathSin(x)
	switch size := f2u(x) &^ dSign; {
	case dIsZero(size):
		spinCycles(sinZeroPad)
	case size < bitsHalfPi:
		spinCycles(sinNearPad)
	default:
		spinCycles(sinFarPad)
	}
	return r
}

//export cos
func libCos(x float64) float64 {
	r := mathCos(x)
	switch size := f2u(x) &^ dSign; {
	case size < bitsTiny:
		spinCycles(cosTinyPad)
	case size < bitsPi:
		spinCycles(cosNearPad)
	default:
		spinCycles(cosFarPad)
	}
	return r
}

//export tan
func libTan(x float64) float64 { return mathTan(x) }

//export asin
func libAsin(x float64) float64 { return mathAsin(x) }

//export acos
func libAcos(x float64) float64 { return mathAcos(x) }

//export atan
func libAtan(x float64) float64 { return mathAtan(x) }

//export atan2
func libAtan2(x, y float64) float64 { return mathAtan2(x, y) }

//export sinh
func libSinh(x float64) float64 { return mathSinh(x) }

//export cosh
func libCosh(x float64) float64 { return mathCosh(x) }

//export tanh
func libTanh(x float64) float64 { return mathTanh(x) }

//export exp
func libExp(x float64) float64 { return mathExp(x) }

//export log
func libLog(x float64) float64 { return mathLog(x) }

//export log10
func libLog10(x float64) float64 { return mathLog10(x) }

//export pow
func libPow(x, y float64) float64 { return mathPow(x, y) }

//export hypot
func libHypot(x, y float64) float64 { return mathHypot(x, y) }

//export floor
func libFloor(x float64) float64 { return mathFloor(x) }

//export ceil
func libCeil(x float64) float64 { return mathCeil(x) }

//export fmod
func libFmod(x, y float64) float64 { return mathFmod(x, y) }

//export modf
func libModf(x float64, ip *float64) float64 {
	frac, i := mathModf(x)
	*ip = i
	return frac
}

//export frexp
func libFrexp(x float64, e *int32) float64 {
	m, n := mathFrexp(x)
	*e = int32(n)
	return m
}

//export ldexp
func libLdexp(x float64, n int32) float64 { return mathLdexp(x, n) }
