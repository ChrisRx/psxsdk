/*
 * Sony PlayStation (R3000A), standalone: the handful of C library functions
 * the compiler emits calls to on its own.
 *
 * This target has no libc. Setting one would mean building picolibc for
 * mipsel to get three functions, when what is actually needed is only what
 * LLVM reaches for while compiling ordinary Go: zeroing a struct becomes a
 * memset, copying a slice becomes a memcpy, and copying one that might
 * overlap becomes a memmove.
 *
 * The target compiles this with -fno-builtin, without which clang
 * recognises each loop below as the very function it is defining and
 * replaces it with a call to itself.
 *
 * Correctness first: these are byte loops. A word-at-a-time version would be
 * perhaps four times faster on aligned buffers, and is worth writing if a
 * profile ever says so, but a subtle bug in one of these corrupts memory
 * somewhere else entirely and is close to undebuggable on this hardware.
 */

typedef unsigned long size_t;

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

void *
memmove(void *dst, const void *src, size_t n)
{
	unsigned char *d = (unsigned char *)dst;
	const unsigned char *s = (const unsigned char *)src;

	if (d == s || n == 0)
		return dst;
	if (d < s) {
		while (n-- > 0)
			*d++ = *s++;
	} else {
		/* Backwards, so a forward overlap is read before it is
		 * overwritten. */
		d += n;
		s += n;
		while (n-- > 0)
			*--d = *--s;
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

