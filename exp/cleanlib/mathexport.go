//go:build yaroze

package cleanlib

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

//export sqrt
func libSqrt(x float64) float64 { return mathSqrt(x) }

//export sin
func libSin(x float64) float64 { return mathSin(x) }

//export cos
func libCos(x float64) float64 { return mathCos(x) }

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
