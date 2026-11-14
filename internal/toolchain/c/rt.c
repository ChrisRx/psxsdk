/*
 * rt.c: what a C program's compiler calls and the resident library has
 * none of, which the SDK's GCC got from libgcc. Only what a program uses is
 * linked, and they are weak, since a program may well have its own abs.
 *
 * The conversions call the library's routines by name. Written with casts,
 * they are a pattern the optimizer knows, and it turns them into a call to
 * the very function being written.
 */

double __floatsidf(int i);
double __adddf3(double a, double b);
float __truncdfsf2(double d);

__attribute__((weak)) int abs(int i) { return i < 0 ? -i : i; }

__attribute__((weak)) long labs(long i) { return i < 0 ? -i : i; }

__attribute__((weak)) double __floatunsidf(unsigned int u)
{
	if ((int)u >= 0)
		return __floatsidf(u);
	return __adddf3(__floatsidf(u - 0x80000000u), 2147483648.0);
}

__attribute__((weak)) float __floatunsisf(unsigned int u) { return __truncdfsf2(__floatunsidf(u)); }

/*
 * Conversions between 64-bit integers and floating point. These work on the
 * bits with integer arithmetic alone: a cast would be a call to the function
 * being written, and the resident __floatsidf does not return for INT_MIN.
 * Integers round to nearest, ties to even, as GCC's libgcc did.
 */

typedef unsigned long long u64_t;

union rt_double {
	double d;
	u64_t u;
};

union rt_float {
	float f;
	unsigned int u;
};

/* The bits of u as a floating-point value with mbits bits of mantissa and
 * an exponent bias of bias. */
static u64_t rt_from_u64(u64_t u, int mbits, int bias)
{
	int p = 63, shift;
	u64_t m, rem, half;

	if (u == 0)
		return 0;
	while (!((u >> p) & 1))
		p--;
	shift = p - mbits;
	if (shift <= 0) {
		m = u << -shift;
	} else {
		m = u >> shift;
		rem = u & ((1ULL << shift) - 1);
		half = 1ULL << (shift - 1);
		if (rem > half || (rem == half && (m & 1))) {
			m++;
			if (m >> (mbits + 1)) {
				m >>= 1;
				p++;
			}
		}
	}
	return (u64_t)(p + bias) << mbits | (m & ((1ULL << mbits) - 1));
}

/* The magnitude of the floating-point value in bits, truncated toward zero,
 * or max if it is limit bits or more, or not a number. */
static u64_t rt_to_u64(u64_t bits, int mbits, int bias, int ebits, int limit, u64_t max)
{
	int e = (int)((bits >> mbits) & ((1u << ebits) - 1)) - bias;
	u64_t m;

	if (e < 0)
		return 0;
	if (e >= limit)
		return max;
	m = (bits & ((1ULL << mbits) - 1)) | 1ULL << mbits;
	return e >= mbits ? m << (e - mbits) : m >> (mbits - e);
}

__attribute__((weak)) double __floatundidf(u64_t u)
{
	union rt_double r;

	r.u = rt_from_u64(u, 52, 1023);
	return r.d;
}

__attribute__((weak)) double __floatdidf(long long i)
{
	union rt_double r;

	if (i >= 0)
		return __floatundidf(i);
	r.u = rt_from_u64(-(u64_t)i, 52, 1023) | 1ULL << 63;
	return r.d;
}

__attribute__((weak)) float __floatundisf(u64_t u)
{
	union rt_float r;

	r.u = rt_from_u64(u, 23, 127);
	return r.f;
}

__attribute__((weak)) float __floatdisf(long long i)
{
	union rt_float r;

	if (i >= 0)
		return __floatundisf(i);
	r.u = rt_from_u64(-(u64_t)i, 23, 127) | 0x80000000u;
	return r.f;
}

/* A negative value converted to unsigned is undefined, and gives 0. */
__attribute__((weak)) u64_t __fixunsdfdi(double d)
{
	union rt_double r = {d};

	if (r.u >> 63)
		return 0;
	return rt_to_u64(r.u, 52, 1023, 11, 64, ~0ULL);
}

__attribute__((weak)) long long __fixdfdi(double d)
{
	union rt_double r = {d};
	u64_t m = rt_to_u64(r.u & ~(1ULL << 63), 52, 1023, 11, 63, 1ULL << 63);

	return r.u >> 63 ? -m : (m >> 63 ? m - 1 : m);
}

__attribute__((weak)) u64_t __fixunssfdi(float f)
{
	union rt_float r = {f};

	if (r.u >> 31)
		return 0;
	return rt_to_u64(r.u, 23, 127, 8, 64, ~0ULL);
}

__attribute__((weak)) long long __fixsfdi(float f)
{
	union rt_float r = {f};
	u64_t m = rt_to_u64(r.u & 0x7fffffffu, 23, 127, 8, 63, 1ULL << 63);

	return r.u >> 31 ? -m : (m >> 63 ? m - 1 : m);
}

/*
 * Complex multiplication and division, as the compiler calls them. They
 * are the textbook formulas, without C99's recovery of an infinity from a
 * result that comes out NaN, which a GCC of 1996 did not have either. The
 * float ones work in double, where nothing they compute can overflow, and
 * double division scales by the larger part of the divisor, as GCC did, so
 * that squaring it does not.
 */

static double rt_fabs(double x)
{
	union rt_double r = {x};

	r.u &= ~(1ULL << 63);
	return r.d;
}

__attribute__((weak)) _Complex float __mulsc3(float a, float b, float c, float d)
{
	_Complex float z;

	__real__ z = (double)a * c - (double)b * d;
	__imag__ z = (double)a * d + (double)b * c;
	return z;
}

__attribute__((weak)) _Complex double __muldc3(double a, double b, double c, double d)
{
	_Complex double z;

	__real__ z = a * c - b * d;
	__imag__ z = a * d + b * c;
	return z;
}

__attribute__((weak)) _Complex float __divsc3(float a, float b, float c, float d)
{
	double den = (double)c * c + (double)d * d;
	_Complex float z;

	__real__ z = ((double)a * c + (double)b * d) / den;
	__imag__ z = ((double)b * c - (double)a * d) / den;
	return z;
}

__attribute__((weak)) _Complex double __divdc3(double a, double b, double c, double d)
{
	double r, den;
	_Complex double z;

	if (rt_fabs(c) < rt_fabs(d)) {
		r = c / d;
		den = c * r + d;
		__real__ z = (a * r + b) / den;
		__imag__ z = (b * r - a) / den;
	} else {
		r = d / c;
		den = c + d * r;
		__real__ z = (a + b * r) / den;
		__imag__ z = (b - a * r) / den;
	}
	return z;
}
