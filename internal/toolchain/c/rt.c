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
