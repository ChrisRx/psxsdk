//go:build yaroze

package cleanlib

import "unsafe"

// strtod, worked out from Sony's results (spec 12): no white space and
// no sign are taken; digits, a point and more digits, then e or E, which
// is taken even with nothing after it, and the exponent's sign and
// digits. The digits make the value ten at a time in the library's
// arithmetic; the exponent, less the digits after the point, scales it by
// binary exponentiation applied to the value itself: p runs through 10,
// 100, 1e4, ... squared each step in the library's arithmetic (so 1e64
// and 1e128 come out an ulp above the exact doubles), and the value is
// multiplied, or divided for a negative scale, by p for each set bit of
// the scale, the lowest first. A scale past 255 gives an infinity, even
// for a zero value, and one below -255 zero; the written exponent
// accumulates in a 32-bit integer and wraps.
//
// base is the string's address.
func parseFloat(base uintptr) (float64, int) {
	i := 0
	v := 0.0
	isDigit := func(c byte) bool { return c >= '0' && c <= '9' }
	for isDigit(strByte(base, i)) {
		v = v*10 + toFloat(int32(strByte(base, i)-'0'))
		i++
	}
	var scale int32
	if strByte(base, i) == '.' {
		i++
		for isDigit(strByte(base, i)) {
			v = v*10 + toFloat(int32(strByte(base, i)-'0'))
			scale--
			i++
		}
	}
	if c := strByte(base, i); c == 'e' || c == 'E' {
		i++
		negExp := false
		switch strByte(base, i) {
		case '-':
			negExp = true
			i++
		case '+':
			i++
		}
		var e int32
		for isDigit(strByte(base, i)) {
			e = e*10 + int32(strByte(base, i)-'0')
			i++
		}
		if negExp {
			e = -e
		}
		scale += e
	}
	if scale > 255 {
		errMath(mathERANGE)
		return u2f(dInf), i
	}
	if scale < -255 {
		errMath(mathERANGE)
		return 0, i
	}
	n := scale
	if n < 0 {
		n = -n
	}
	// The powers go through the library's own product by name so that the
	// compiler cannot fold the chain into the exact constants.
	p := 10.0
	for ; n != 0; n >>= 1 {
		if n&1 != 0 {
			if scale > 0 {
				v = mul(v, p)
			} else {
				v = div(v, p)
			}
		}
		p = mul(p, p)
	}
	return v, i
}

// strByte is the i-th byte of the string at base.
func strByte(base uintptr, i int) byte { return *(*byte)(unsafe.Pointer(base + uintptr(i))) }

//export strtod
func libStrtod(s *byte, end **byte) float64 {
	base := uintptr(unsafe.Pointer(s))
	v, n := parseFloat(base)
	if end != nil {
		*end = (*byte)(unsafe.Pointer(base + uintptr(n)))
	}
	return v
}

//export atof
func libAtof(s *byte) float64 { return libStrtod(s, nil) }
