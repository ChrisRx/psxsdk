/*
 * The C library functions the compiler emits calls to on its own, for a
 * program with the Net Yaroze library linked in (internal/libps), where
 * they stand in for the resident library's and a program's timing against
 * that library matters (internal/libps/spec/08-timing.md). They are the
 * TinyGo playstation target's byte loops, which they replace: its copies
 * are weak. Kept here so that the timing is this library's to keep,
 * whatever becomes of the target's: memset takes 8 cycles a byte and
 * memcpy 12, as the resident library's do, and memmove is paced to its 14.
 *
 * Compiled as -fno-builtin would compile them, as the target's are,
 * without which clang recognizes each loop below as the very function it
 * is defining and replaces it with a call to itself.
 */

#pragma clang attribute push(__attribute__((no_builtin)), apply_to = function)

typedef __SIZE_TYPE__ size_t;

void *
memset(void *dst, int c, size_t n)
{
	unsigned char *p = (unsigned char *)dst;

	while (n-- > 0)
		*p++ = (unsigned char)c;
	return dst;
}

void *
memcpy(void *dst, const void *src, size_t n)
{
	unsigned char *d = (unsigned char *)dst;
	const unsigned char *s = (const unsigned char *)src;

	while (n-- > 0)
		*d++ = *s++;
	return dst;
}

/*
 * The resident library's memmove takes 14 cycles a byte where this loop
 * takes 12, so one nop an iteration keeps the two in step.
 */
#define MEMMOVE_PACE() __asm__ volatile("nop")

void *
memmove(void *dst, const void *src, size_t n)
{
	unsigned char *d = (unsigned char *)dst;
	const unsigned char *s = (const unsigned char *)src;

	if (d == s || n == 0)
		return dst;
	if (d < s) {
		while (n-- > 0) {
			*d++ = *s++;
			MEMMOVE_PACE();
		}
	} else {
		/* Backwards, so a forward overlap is read before it is
		 * overwritten. */
		d += n;
		s += n;
		while (n-- > 0) {
			*--d = *--s;
			MEMMOVE_PACE();
		}
	}
	return dst;
}

int
memcmp(const void *a, const void *b, size_t n)
{
	const unsigned char *p = (const unsigned char *)a;
	const unsigned char *q = (const unsigned char *)b;

	while (n-- > 0) {
		if (*p != *q)
			return (int)*p - (int)*q;
		p++;
		q++;
	}
	return 0;
}

size_t
strlen(const char *s)
{
	const char *p = s;

	while (*p != '\0')
		p++;
	return (size_t)(p - s);
}

#pragma clang attribute pop
