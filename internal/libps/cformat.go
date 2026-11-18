//go:build yaroze

package libps

import (
	"device"
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// Formatted output for C programs (spec 17). The library has three formatters
// with different behavior, not one with options, and programs see every
// difference: sprintf is formatter A; printf, printf2 and sprintf2 are B, the
// last two with floating point; FntPrint and KanjiFntPrint are C, with their own
// short list of conversions. Each entry gets its arguments as the words the C
// caller passed (spec 15).
//
// Stack is the constraint here: a C program may keep its stack in the 1 KB
// scratchpad, and these are the deepest calls it makes. So A and C are one
// function each, calling nothing, with no digit buffer: a number's digits come
// out first to last, by digitAt. They walk the format, the arguments and the
// output by address, which leaves the compiler fewer registers to save.

// The flags of a directive.
const (
	fmtLeft = 1 << iota
	fmtPlus
	fmtBlank
	fmtAlt
	fmtZero
	fmtShort
	fmtLong
	fmtUpper
	fmtOctalZero
)

const (
	hexLower = "0123456789abcdef"
	hexUpper = "0123456789ABCDEF"
)

var pow10 = [10]uint32{1, 10, 100, 1000, 10000, 100000, 1000000, 10000000, 100000000, 1000000000}

//go:inline
//go:nobounds
func byteAt(p uintptr) byte { return cBytes(p)[0] }

//go:inline
//go:nobounds
func setByte(p uintptr, c byte) { cBytes(p)[0] = c }

//go:inline
//go:nobounds
func wordAt(p uintptr) uint32 { return cWords(p)[0] }

// digitCount is how many digits v has, at least one, in decimal for a shift of 0
// or else in the base of that many bits a digit.
//
//go:inline
//go:nobounds
func digitCount(v, shift uint32) int32 {
	n := int32(1)
	if shift == 0 {
		for n < 10 && v >= pow10[n] {
			n++
		}
		return n
	}
	for v >>= shift; v != 0; v >>= shift {
		n++
	}
	return n
}

// digitAt is digit k of v, counting from the last, and what to take the next
// digit from: decimal digits come out of v from the top. The division is the
// CPU's, since one by a variable in Go brings a call for its zero check, and a
// power of ten is never 0.
//
//go:inline
//go:nobounds
func digitAt(v uint32, k int32, shift uint32) (d, rest uint32) {
	if shift == 0 {
		p := pow10[k]
		d = uint32(device.AsmFull(`.set push
	.set noat
	divu $$0, {v}, {p}
	mflo {}
	nop
	nop
	.set pop`, map[string]interface{}{"v": v, "p": p}))
		return d, v - d*p
	}
	return v >> (uint32(k) * shift) & (1<<shift - 1), v
}

// digitChar is the digit d as a character.
//
//go:inline
//go:nobounds
func digitChar(d uint32, flags uint32) byte {
	if flags&fmtUpper != 0 {
		return hexUpper[d]
	}
	return hexLower[d]
}

//go:inline
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// libSprintf is formatter A: a directive's parts in strict order, and anything
// out of that order ends the text there, with no more arguments looked at. In the
// target's assembly, for its time.
//
//export libps_va_sprintf
func libSprintf(args uintptr) int32

const (
	// fmtTTY as formatter B's destination sends each byte through putchar.
	fmtTTY = ^uintptr(0)

	// fmtMaxPrec is the most digits a double is worked out to. A precision
	// over it comes out as zeros, after everything, the exponent included.
	fmtMaxPrec = 39
)

// fmtPut writes c where dst says and returns where the next goes.
//
//go:nobounds
func fmtPut(dst uintptr, c byte) uintptr {
	if dst == fmtTTY {
		putchar(int32(c))
		return dst
	}
	setByte(dst, c)
	return dst + 1
}

// fmtFill writes n copies of c. A number mostly has no padding, for which this
// makes no call.
//
//go:inline
func fmtFill(dst uintptr, c byte, n int32) uintptr {
	if n <= 0 {
		return dst
	}
	return fmtFillMore(dst, c, n)
}

//go:noinline
//go:nobounds
func fmtFillMore(dst uintptr, c byte, n int32) uintptr {
	if dst == fmtTTY {
		for ; n > 0; n-- {
			putchar(int32(c))
		}
		return dst
	}
	for ; n > 0; n-- {
		setByte(dst, c)
		dst++
	}
	return dst
}

// fmtLiteral writes the format's text at f up to a percent sign or its end, and
// returns where the next byte goes and where it stopped. It is a call of its own
// because the characters are what a printf mostly spends on, and in formatB's loop
// each would save all that that function keeps in registers around putchar.
//
//go:noinline
//go:nobounds
func fmtLiteral(dst, f uintptr) (uintptr, uintptr) {
	for {
		c := byteAt(f)
		if c == 0 || c == '%' {
			return dst, f
		}
		if dst == fmtTTY {
			// A byte with the top bit set moves no column unless its class says it prints
			// (highClass), and is Sony's 570 by going straight to the write.
			if c >= 0x80 && highClass(c)&0x97 == 0 {
				volatile.StoreUint8(&ttyChar, c)
				libps_biosB(biosWrite, 1, uintptr(unsafe.Pointer(&ttyChar)), 1, 0)
			} else {
				putchar(int32(c))
				space(fmtLiteralSpace)
			}
		} else {
			setByte(dst, c)
			dst++
		}
		f++
	}
}

// fmtLiteralSpace is what a literal character to the TTY spends beyond its work, in
// 2-cycle stores, for Sony's 582 cycles a character, the kernel's write included.
const fmtLiteralSpace = 0

// fmtNull is what %s of a null pointer prints under formatter B.
var fmtNull = [...]byte{'(', 'n', 'u', 'l', 'l', ')', 0}

// formatB is formatter B, with the format at f and its arguments from word next
// of args. A directive is a loop until a conversion letter, in any order, and
// any letter it does not know is itself: nothing stops it but the format's end.
// The result counts only what conversions made, not literal text, and %n stores
// that same count. With doubles (printf2, sprintf2) e, E, f, g and G take a double,
// and short is the low 16 bits; without (printf) they are letters like any other,
// and short is the low 16 bits sign extended. Into memory, the terminator is written
// only if at least one byte was made: with none, nothing is written at all.
//
//go:noinline
//go:nobounds
func formatB(dst, f, args uintptr, next int32, doubles bool) int32 {
	// A null format makes nothing, though the memory at address 0 holds bytes
	// (probe spec/probes/cfmtlong, "null format").
	if f == 0 {
		return 0
	}
	a := cWords(args)
	start := dst
	var count int32
	for {
		dst, f = fmtLiteral(dst, f)
		c := byteAt(f)
		f++
		if c == 0 {
			break
		}
		var flags uint32
		var width int32
		prec := int32(-1)
	directive:
		for {
			c = byteAt(f)
			f++
			switch c {
			case 0:
				goto end
			case ' ':
				flags |= fmtBlank
			case '#':
				flags |= fmtAlt
			case '-':
				flags |= fmtLeft
			case '+':
				flags |= fmtPlus
			case '0':
				flags |= fmtZero
			case '*':
				width = int32(a[next])
				next++
				if width < 0 {
					flags |= fmtLeft
					width = -width
				}
			case '.':
				if byteAt(f) == '*' {
					f++
					prec = int32(a[next])
					next++
					if prec < 0 {
						prec = -1
					}
				} else {
					prec = 0
					for isDigit(byteAt(f)) {
						prec = prec*10 + int32(byteAt(f)-'0')
						f++
					}
				}
			case 'h':
				flags |= fmtShort
			case 'l':
				flags |= fmtLong
			case 'L':
			default:
				if c < '1' || c > '9' {
					break directive
				}
				width = int32(c - '0')
				for isDigit(byteAt(f)) {
					width = width*10 + int32(byteAt(f)-'0')
					f++
				}
			}
		}
		var v, shift uint32
		var sign, prefix byte
		var body uintptr // the bytes of a string or a double
		size, trail := int32(0), int32(0)
		kind := c
		switch c {
		case 'n':
			p := uintptr(a[next])
			next++
			if flags&fmtShort != 0 && flags&fmtLong == 0 {
				cHalves(p)[0] = uint16(count)
			} else {
				cWords(p)[0] = uint32(count)
			}
			continue
		case 'c':
			v = a[next]
			next++
			size = 1
		case 's':
			body = uintptr(a[next])
			next++
			if body == 0 {
				body = uintptr(unsafe.Pointer(&fmtNull))
			}
			for (prec < 0 || size < prec) && byteAt(body+uintptr(size)) != 0 {
				size++
			}
		case 'D', 'U', 'O', 'd', 'i', 'u', 'o', 'x', 'X', 'p':
			v = a[next]
			next++
			if c == 'D' || c == 'U' || c == 'O' {
				flags |= fmtLong
				c += 'a' - 'A'
			}
			if flags&fmtShort != 0 && flags&fmtLong == 0 && c != 'p' {
				if doubles {
					v &= 0xffff
				} else {
					v = uint32(int32(int16(v)))
				}
			}
			switch c {
			case 'd', 'i':
				switch {
				case int32(v) < 0:
					sign = '-'
					v = -v
				case flags&fmtPlus != 0:
					sign = '+'
				case flags&fmtBlank != 0:
					sign = ' '
				}
			case 'o':
				shift = 3
			case 'u':
			default:
				shift = 4
				if c == 'X' {
					flags |= fmtUpper
				}
				if flags&fmtAlt != 0 && v != 0 && c != 'p' {
					prefix = c
				}
			}
			kind = 'd'
			if v != 0 || prec != 0 {
				size = digitCount(v, shift)
			}
			if c == 'o' && flags&fmtAlt != 0 && v != 0 {
				flags |= fmtOctalZero
				size++
			}
			// A precision turns zero off for a number.
			if prec >= 0 {
				flags &^= fmtZero
			}
		case '%':
			dst = fmtPut(dst, '%')
			count++
			continue
		case 'e', 'E', 'f', 'g', 'G':
			if !doubles {
				dst = fmtPut(dst, c)
				count++
				continue
			}
			next += next & 1
			x := uint64(a[next]) | uint64(a[next+1])<<32
			next += 2
			alt := flags&fmtAlt != 0
			if prec > fmtMaxPrec {
				if alt || c != 'g' && c != 'G' {
					trail = prec - fmtMaxPrec
				}
				prec = fmtMaxPrec
			} else if prec < 0 {
				prec = 6
			}
			dbl.neg = dCompare(x, 0) < 0
			if dbl.neg {
				x = dNeg(x)
			}
			fmtDouble(x, prec, c, alt)
			switch {
			case dbl.neg:
				sign = '-'
			case flags&fmtPlus != 0:
				sign = '+'
			case flags&fmtBlank != 0:
				sign = ' '
			}
			body = uintptr(unsafe.Pointer(&dbl.text)) + uintptr(dbl.start)
			size = dbl.size
			kind = 's'
			prec = -1
		default:
			dst = fmtPut(dst, c)
			count++
			continue
		}
		// fieldsz is the sign and prefix and bytes together, which a precision
		// for a number counts inside, and realsz that or the precision.
		fieldsz := size + trail
		if sign != 0 {
			fieldsz++
		}
		if prefix != 0 {
			fieldsz += 2
		}
		realsz := fieldsz
		if kind == 'd' && prec > realsz {
			realsz = prec
		}
		left, zero := flags&fmtLeft != 0, flags&fmtZero != 0
		if !left && !zero {
			dst = fmtFill(dst, ' ', width-realsz)
		}
		if sign != 0 {
			dst = fmtPut(dst, sign)
		}
		if prefix != 0 {
			dst = fmtPut(dst, '0')
			dst = fmtPut(dst, prefix)
		}
		if !left && zero {
			dst = fmtFill(dst, '0', width-realsz)
		}
		switch kind {
		case 'c':
			dst = fmtPut(dst, byte(v))
		case 's':
			for k := int32(0); k < size; k++ {
				dst = fmtPut(dst, byteAt(body+uintptr(k)))
			}
		default:
			dst = fmtFill(dst, '0', prec-fieldsz)
			if flags&fmtOctalZero != 0 {
				dst = fmtPut(dst, '0')
				size--
			}
			for k := size - 1; k >= 0; k-- {
				var d uint32
				d, v = digitAt(v, k, shift)
				dst = fmtPut(dst, digitChar(d, flags))
			}
		}
		dst = fmtFill(dst, '0', trail)
		if left {
			dst = fmtFill(dst, ' ', width-realsz)
		}
		count += max(width, realsz)
	}
end:
	if dst != fmtTTY && dst != start {
		setByte(dst, 0)
	}
	return count
}

// libPrintf is formatter B without doubles, to the TTY. In the target's assembly, for
// its time.
//
//export libps_va_printf
func libPrintf(args uintptr) int32

//export libps_va_printf2
//go:nobounds
func libPrintf2(args uintptr) int32 {
	return formatB(fmtTTY, uintptr(wordAt(args)), args, 1, true)
}

//export libps_va_sprintf2
//go:nobounds
func libSprintf2(args uintptr) int32 {
	return formatB(uintptr(wordAt(args)), uintptr(wordAt(args+4)), args, 2, true)
}

// dbl is a double's conversion in progress: text, the digits made, from start
// for size bytes, with a slot before the first for a carry, and ints, the
// integer part's digits, last first. It is memory of the library's own and not
// the stack: %f of the largest number makes 309 digits, and the compiler would
// put an array that size on the heap, which is never given back. So a double's
// conversion must not be interrupted by another. neg is whether the sign is
// still to show: rounding takes it away from a text of all zeros.
var dbl struct {
	text  [1 + 320 + 2 + fmtMaxPrec + 8]byte
	ints  [320]byte
	start int32
	size  int32
	exp   int32
	neg   bool
}

// The constants of a double's digits, as bits: they go through the library's
// own arithmetic by name, so that the look of the output is its rounding's.
const (
	dblTen    = 0x4024000000000000
	dblHundth = 0x3f847ae147ae147b // 0.01, which keeps a digit from coming out one low
	dblGLimit = 0x3f1a36e2eb1c432d // 0.0001, below which %g takes the e form
)

// dModf is modf on the bits.
//
//go:inline
func dModf(x uint64) (frac, ip uint64) {
	f, i := mathModf(u2f(x))
	return f2u(f), f2u(i)
}

// dblNext makes the next fraction digit into text at t: fract times ten,
// split. It returns the fraction left.
func dblNext(fract uint64) (uint64, int32) {
	frac, ip := dModf(dMul(fract, dblTen))
	return frac, dFix(ip)
}

// dblRound rounds the text made, from t back to start, by the next digit:
// fract's if it is not zero, otherwise ch. withExp is the e form's: a carry off
// the left end makes the first digit 1 and raises the exponent, where otherwise
// it adds a new first digit. A digit of 4 or less over a text of all zeros takes
// a minus sign away.
func dblRound(fract uint64, ch int32, t int32, withExp bool) {
	d := ch
	if !dIsZero(fract) {
		_, ip := dModf(dMul(fract, dblTen))
		d = dFix(ip)
	}
	end := t - 1
	if d > 4 {
		for {
			if dbl.text[end] == '.' {
				end--
			}
			dbl.text[end]++
			if dbl.text[end] <= '9' {
				return
			}
			dbl.text[end] = '0'
			if end == dbl.start {
				if withExp {
					dbl.text[end] = '1'
					dbl.exp++
				} else {
					dbl.start--
					dbl.text[dbl.start] = '1'
				}
				return
			}
			end--
		}
	}
	if dbl.neg {
		for ; end >= dbl.start; end-- {
			if dbl.text[end] != '.' && dbl.text[end] != '0' {
				return
			}
		}
		dbl.neg = false
	}
}

// fmtDouble works out the digits of the magnitude x as spec 17 says, leaving
// them in dbl. Every step is the library's own arithmetic: that, and not exact
// decimal, is what the digits are.
func fmtDouble(x uint64, prec int32, conv byte, alt bool) {
	fract, integer := dModf(x)
	dbl.start = 1
	t := dbl.start

	// The integer's digits come out last first.
	ints := int32(len(dbl.ints))
	expcnt := int32(0)
	for !dIsZero(integer) {
		tmp, ip := dModf(dDiv(integer, dblTen))
		integer = ip
		ints--
		dbl.ints[ints] = '0' + byte(dFix(dMul(dAdd(tmp, dblHundth), dblTen)))
		expcnt++
	}

	eForm, trim := conv == 'e' || conv == 'E', false
	expChar := byte('e')
	if conv == 'E' || conv == 'G' {
		expChar = 'E'
	}
	if conv == 'g' || conv == 'G' {
		if prec == 0 {
			prec = 1
		}
		if expcnt > prec || expcnt == 0 && !dIsZero(fract) && dCompare(fract, dblGLimit) < 0 {
			prec--
			eForm, trim = true, !alt
		}
	}

	if eForm {
		dbl.exp = expcnt
		switch {
		case expcnt != 0:
			dbl.text[t] = dbl.ints[ints]
			t++
			ints++
			if prec != 0 || alt {
				dbl.text[t] = '.'
				t++
			}
			for ; prec != 0 && ints < int32(len(dbl.ints)); prec-- {
				dbl.text[t] = dbl.ints[ints]
				t++
				ints++
			}
			if prec == 0 && ints < int32(len(dbl.ints)) {
				fract = 0
				dblRound(0, int32(dbl.ints[ints]-'0'), t, true)
			}
			dbl.exp--
		case !dIsZero(fract):
			dbl.exp = -1
			var d int32
			for {
				fract, d = dblNext(fract)
				if d != 0 {
					break
				}
				dbl.exp--
			}
			dbl.text[t] = '0' + byte(d)
			t++
			if prec != 0 || alt {
				dbl.text[t] = '.'
				t++
			}
		default:
			dbl.exp = 0
			dbl.text[t] = '0'
			t++
			if prec != 0 || alt {
				dbl.text[t] = '.'
				t++
			}
		}
		if !dIsZero(fract) {
			for prec != 0 && !dIsZero(fract) {
				var d int32
				fract, d = dblNext(fract)
				dbl.text[t] = '0' + byte(d)
				t++
				prec--
			}
			if !dIsZero(fract) {
				dblRound(fract, 0, t, true)
			}
		}
		for ; prec > 0; prec-- {
			dbl.text[t] = '0'
			t++
		}
		if trim {
			t = dblTrim(t)
		}
		dbl.text[t] = expChar
		t++
		e := dbl.exp
		dbl.text[t] = '+'
		if e < 0 {
			dbl.text[t] = '-'
			e = -e
		}
		t++
		if e >= 100 {
			dbl.text[t] = '0' + byte(e/100)
			t++
			e %= 100
		}
		dbl.text[t] = '0' + byte(e/10)
		dbl.text[t+1] = '0' + byte(e%10)
		t += 2
		dbl.size = t - dbl.start
		return
	}

	isG := conv == 'g' || conv == 'G'
	if expcnt != 0 {
		for ; ints < int32(len(dbl.ints)); ints++ {
			dbl.text[t] = dbl.ints[ints]
			t++
			if isG {
				prec--
			}
		}
	} else {
		dbl.text[t] = '0'
		t++
	}
	point := prec != 0 || alt
	if point {
		dbl.text[t] = '.'
		t++
	}
	if !dIsZero(fract) {
		if prec > 0 {
			if isG {
				// The zeros before the first digit that is not one do not count.
				var d int32
				for {
					fract, d = dblNext(fract)
					dbl.text[t] = '0' + byte(d)
					t++
					if d != 0 {
						break
					}
				}
				prec--
			}
			for prec > 0 && !dIsZero(fract) {
				var d int32
				fract, d = dblNext(fract)
				dbl.text[t] = '0' + byte(d)
				t++
				prec--
			}
		}
		if !dIsZero(fract) {
			dblRound(fract, 0, t, false)
		}
	}
	switch {
	case !isG || alt:
		for ; prec > 0; prec-- {
			dbl.text[t] = '0'
			t++
		}
	case point:
		t = dblTrim(t)
	}
	dbl.size = t - dbl.start
}

// dblTrim takes trailing zeros off the text ending before t, and the point if
// it is then last, and returns the new end.
func dblTrim(t int32) int32 {
	for t > dbl.start+1 && dbl.text[t-1] == '0' {
		t--
	}
	if dbl.text[t-1] == '.' {
		t--
	}
	return t
}

// libFntPrint is FntPrint for C programs, formatter C, in the target's assembly: what it
// does at its edges is said there.
//
//export libps_va_FntPrint
func libFntPrint(args uintptr) int32

// libKanjiFntPrint is KanjiFntPrint for C programs, formatter C again, but for
// what it makes: numbers in full-width Shift JIS, two bytes a digit, 815Bh for a
// minus sign, and a width in cells, the fill being a space byte for each cell
// short, cells being the field's bytes over two. The count is checked before the
// text is added, and the whole text goes in however long, at the stream's NUL.
//
//export libps_va_KanjiFntPrint
//go:nobounds
func libKanjiFntPrint(args uintptr) int32 {
	if !kanji.opened {
		return -1
	}
	st := &kanji.streams[0]
	f := uintptr(wordAt(args))
	ap := args + 4
	if id := uint32(f); id < uint32(kanji.open) {
		st = &kanji.streams[id]
		f = uintptr(wordAt(args + 4))
		ap = args + 8
	}
	if st.count > st.n {
		return -1
	}
	end := st.end
	start := end
	for {
		c := byteAt(f)
		f++
		if c != '%' || byteAt(f) == '%' {
			if c == 0 {
				break
			}
			if c == '%' {
				f++
			}
			end = kanji.put(end, c)
			continue
		}
		zero := byteAt(f) == '0'
		var width int32
		for isDigit(byteAt(f)) {
			width = width*10 + int32(byteAt(f)-'0')
			f++
		}
		conv := byteAt(f)
		f++
		var v, shift uint32
		var src uintptr
		size := int32(0)
		switch conv {
		case 'd':
			v = wordAt(ap)
			ap += 4
			zero = false
			if int32(v) < 0 {
				v = -v
				conv = '-'
				size = 2
			}
			size += 2 * digitCount(v, 0)
		case 'x', 'X':
			v = wordAt(ap)
			ap += 4
			shift = 4
			size = 2 * digitCount(v, 4)
		case 'c':
			v = wordAt(ap)
			ap += 4
			zero = false
			size = 1
		case 's':
			src = uintptr(wordAt(ap))
			ap += 4
			zero = false
			if src == 0 {
				conv = 0
				break
			}
			for byteAt(src+uintptr(size)) != 0 {
				space(1)
				size++
			}
		default:
			conv = 0
		}
		if conv == 0 {
			for k := max(width, 1); k > 0; k-- {
				end = kanji.put(end, ' ')
			}
			continue
		}
		for k := width - size/2; k > 0; k-- {
			if zero {
				end = kanji.put(end, 0x82)
				end = kanji.put(end, 0x4f)
			} else {
				end = kanji.put(end, ' ')
			}
		}
		switch conv {
		case 'c':
			end = kanji.put(end, byte(v))
		case 's':
			for k := int32(0); k < size; k++ {
				end = kanji.put(end, byteAt(src+uintptr(k)))
			}
		default:
			if conv == '-' {
				end = kanji.put(end, 0x81)
				end = kanji.put(end, 0x5b)
				size -= 2
			}
			for k := size/2 - 1; k >= 0; k-- {
				var d uint32
				d, v = digitAt(v, k, shift)
				end = kanji.put(end, 0x82)
				if d >= 10 {
					end = kanji.put(end, 0x60+byte(d)-10)
				} else {
					end = kanji.put(end, 0x4f+byte(d))
				}
			}
		}
	}
	*kanji.textByte(end) = 0
	st.end = end
	st.count += int32(end - start)
	return st.count
}
