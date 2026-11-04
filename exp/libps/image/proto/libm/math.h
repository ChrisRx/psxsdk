/* What musl's math sources take from <math.h>, for the image: on this
 * target double is the soft-float double and long double the same. */
#ifndef _LIBPS_MATH_H
#define _LIBPS_MATH_H

#include "features.h"
#define weak_alias(old, new) extern __typeof(old) new __attribute__((__weak__, __alias__(#old)))

typedef double double_t;
typedef float float_t;

#define NAN __builtin_nanf("")
#define INFINITY __builtin_inff()
#define HUGE_VAL __builtin_huge_val()
#define HUGE_VALF __builtin_huge_valf()

#define FP_NAN 0
#define FP_INFINITE 1
#define FP_ZERO 2
#define FP_SUBNORMAL 3
#define FP_NORMAL 4

int __fpclassify(double);
#define fpclassify(x) __fpclassify(x)
#define isnan(x) __builtin_isnan(x)
#define isinf(x) __builtin_isinf(x)
#define isfinite(x) __builtin_isfinite(x)
#define isnormal(x) __builtin_isnormal(x)
#define signbit(x) __builtin_signbit(x)

#define M_PI 3.14159265358979323846
#define M_PI_2 1.57079632679489661923

double sin(double), cos(double), tan(double), asin(double), acos(double), atan(double), atan2(double, double);
double exp(double), expm1(double), log(double), log10(double), pow(double, double);
double sinh(double), cosh(double), tanh(double), hypot(double, double), sqrt(double), fabs(double);
double ldexp(double, int), scalbn(double, int), frexp(double, int *), modf(double, double *), fmod(double, double);
double ceil(double), floor(double);

#endif
