/*
 * The MIPS R3000A as the PlayStation uses it: the integer helpers the
 * compiler emits calls to and the ISA has no instruction for. Shared by
 * the playstation and playstation-yaroze targets.
 */

/*
 * 64-bit integer division, which MIPS I has no instruction for and which Go
 * reaches for constantly: every duration arithmetic in the time package is a
 * division of an int64.
 *
 * These would normally come from compiler-rt, and the target asked for it at
 * first, but LLVM cannot compile compiler-rt's own soft-float builtins for
 * this ISA:
 *
 *   fatal error: error in backend: Cannot select:
 *   f32 = MipsISD::TruncIntFP, fixunssfdi.c:21:16
 *
 * so the whole library fails to build and takes the integer helpers, which
 * are fine, down with it. The four below are the ones a Go program actually
 * needs. For the same reason this target has no soft-float support at all:
 * float64 arithmetic emits calls to builtins that cannot be built.
 */

/*
 * Shift-and-subtract, one bit at a time, so that nothing in here needs a
 * 64-bit shift by a variable amount: that would itself be a call to a
 * builtin, and these are the builtins.
 *
 * 64 iterations is some 500 cycles a division. compiler-rt's version is far
 * cleverer. If 64-bit division ever turns up in something that has to keep
 * to a frame, the answer is to not divide rather than to tune this.
 */
static unsigned long long
udivmod64(unsigned long long n, unsigned long long d, unsigned long long *rem)
{
	unsigned long long q = 0;
	unsigned long long r = 0;
	int i;

	/* Division by zero is undefined, and there is nothing useful to do
	 * about it here: the trap instruction LLVM would normally use to
	 * report it does not exist on a MIPS I core. */
	if (d == 0) {
		if (rem != 0)
			*rem = 0;
		return 0;
	}

	for (i = 0; i < 64; i++) {
		r = (r << 1) | (n >> 63);
		n <<= 1;
		if (r >= d) {
			r -= d;
			q = (q << 1) | 1;
		} else {
			q <<= 1;
		}
	}

	if (rem != 0)
		*rem = r;
	return q;
}

unsigned long long
__udivmoddi4(unsigned long long a, unsigned long long b, unsigned long long *rem)
{
	return udivmod64(a, b, rem);
}

unsigned long long
__udivdi3(unsigned long long a, unsigned long long b)
{
	return udivmod64(a, b, 0);
}

unsigned long long
__umoddi3(unsigned long long a, unsigned long long b)
{
	unsigned long long r;

	udivmod64(a, b, &r);
	return r;
}

long long
__divdi3(long long a, long long b)
{
	unsigned long long ua, ub, q;
	int neg = 0;

	if (a < 0) {
		ua = -(unsigned long long)a;
		neg = !neg;
	} else {
		ua = (unsigned long long)a;
	}
	if (b < 0) {
		ub = -(unsigned long long)b;
		neg = !neg;
	} else {
		ub = (unsigned long long)b;
	}

	q = udivmod64(ua, ub, 0);
	return neg ? -(long long)q : (long long)q;
}

/* C truncates towards zero, so the remainder takes the sign of the
 * dividend and the sign of the divisor does not matter. */
long long
__moddi3(long long a, long long b)
{
	unsigned long long ua, ub, r;
	int neg = 0;

	if (a < 0) {
		ua = -(unsigned long long)a;
		neg = 1;
	} else {
		ua = (unsigned long long)a;
	}
	ub = b < 0 ? -(unsigned long long)b : (unsigned long long)b;

	udivmod64(ua, ub, &r);
	return neg ? -(long long)r : (long long)r;
}

/* Nonzero if either operand is NaN. Go needs it for float to int conversion
 * and libps.a lacks it. Compares bits so that no float op is needed. */
int
__unordsf2(float a, float b)
{
	union { float f; unsigned int u; } x = { a }, y = { b };

	return (x.u & 0x7fffffff) > 0x7f800000 || (y.u & 0x7fffffff) > 0x7f800000;
}

int
__unorddf2(double a, double b)
{
	union { double f; unsigned long long u; } x = { a }, y = { b };

	return (x.u & 0x7fffffffffffffffULL) > 0x7ff0000000000000ULL ||
	       (y.u & 0x7fffffffffffffffULL) > 0x7ff0000000000000ULL;
}
