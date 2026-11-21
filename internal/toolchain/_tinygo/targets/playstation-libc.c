/*
 * Sony PlayStation (R3000A): the target's C. First the handful of C library
 * functions the compiler emits calls to on its own, then the default clock.
 *
 * Everything here is weak, as everything in playstation.S is, so that a
 * program with a C library or a clock of its own replaces any of it by
 * defining the same function: see "The platform hooks" in
 * src/runtime/runtime_playstation.go.
 *
 * This target has no libc. Setting one would mean building picolibc for
 * mipsel to get a few functions, when what is actually needed is only what
 * LLVM reaches for while compiling ordinary Go: zeroing a struct becomes a
 * memset, copying a slice becomes a memcpy, copying one that might overlap
 * becomes a memmove, and a loop that counts bytes up to a NUL, as
 * internal/reflectlite's reading of a type's name does, becomes a strlen.
 *
 * They are compiled as -fno-builtin would compile them, by the pragma
 * below, without which clang recognizes each loop as the very function it
 * is defining and replaces it with a call to itself. It is a pragma on
 * these functions rather than a flag of the target, since a function
 * compiled that way is never inlined into a caller that was not, and the
 * hooks an SDK defines in C should be.
 *
 * Correctness first: these are byte loops. A word-at-a-time version would be
 * perhaps four times faster on aligned buffers, and is worth writing if a
 * profile ever says so, but a subtle bug in one of these corrupts memory
 * somewhere else entirely and is close to undebuggable on this hardware.
 */

#define WEAK __attribute__((weak))

typedef __SIZE_TYPE__ size_t;

#pragma clang attribute push(__attribute__((no_builtin)), apply_to = function)

WEAK void *
memset(void *dst, int c, size_t n)
{
	unsigned char *p = (unsigned char *)dst;

	while (n-- > 0)
		*p++ = (unsigned char)c;
	return dst;
}

WEAK void *
memcpy(void *dst, const void *src, size_t n)
{
	unsigned char *d = (unsigned char *)dst;
	const unsigned char *s = (const unsigned char *)src;

	while (n-- > 0)
		*d++ = *s++;
	return dst;
}

WEAK void *
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

WEAK int
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

WEAK size_t
strlen(const char *s)
{
	const char *p = s;

	while (*p != '\0')
		p++;
	return (size_t)(p - s);
}

#pragma clang attribute pop

/*
 * The default clock, and with it the default start-up hook, which is what
 * starts it. They are in C rather than in the runtime so that they can be
 * weak: a program that runs under something that owns the timers, or that
 * keeps time some other way, defines the same three functions and these
 * are left out of the link.
 *
 * Root counter 2, one of the three hardware timers, is the clock. Counter
 * 0 is tied to the dot clock and counter 1 to the horizontal retrace, both
 * of which stop meaning anything if the program reprograms the display;
 * counter 2 is driven by the CPU clock and keeps running whatever the GPU
 * is doing.
 *
 * I/O is uncached at these addresses in every region, so no cache
 * maintenance is needed around them.
 */

#define TIMER_COUNT  (*(volatile unsigned int *)0x1f801120) /* current value, 16 bits */
#define TIMER_MODE   (*(volatile unsigned int *)0x1f801124) /* clock source and counting mode */
#define TIMER_TARGET (*(volatile unsigned int *)0x1f801128)

/*
 * Clock source 2 is the CPU clock divided by eight. Writing the mode
 * register also resets the counter.
 */
#define TIMER_MODE_DIV8 (2 << 8)

/*
 * 33.8688MHz divided by eight. Expressed as a ratio against a nanosecond
 * it is 312500/1323 exactly, which is what the conversions below use
 * instead of rounding to 236ns a tick. Both overflow after about 80 days
 * of uptime, which is longer than the hardware has ever been asked to stay
 * on.
 */
#define TIMER_NS_NUM 312500
#define TIMER_NS_DEN 1323

/*
 * The counter is 16 bits and wraps every 15.5ms, so the high bits are kept
 * in software and advanced whenever a read comes back lower than the one
 * before it.
 *
 * That makes the clock correct only if it is read more often than it
 * wraps. psx_sleep polls, so a sleep of any length is accounted for
 * exactly, but a program that computes a duration across more than 15.5ms
 * of doing nothing else will read it short. Keeping the whole thing honest
 * would mean taking the timer interrupt, which would mean owning the
 * exception vector, which is not the runtime's to own on a machine where
 * the program may want it.
 */
static unsigned short timer_last;
static unsigned long long timer_high;

static unsigned long long
timer_ticks(void)
{
	unsigned short now = (unsigned short)TIMER_COUNT;

	if (now < timer_last)
		timer_high += 1 << 16;
	timer_last = now;
	return timer_high + now;
}

/* psx_init runs once, before anything else in the runtime. */
WEAK void
psx_init(void)
{
	TIMER_TARGET = 0;
	TIMER_MODE = TIMER_MODE_DIV8;
	timer_last = 0;
	timer_high = 0;
}

/* psx_nanotime returns the nanoseconds since psx_init. */
WEAK long long
psx_nanotime(void)
{
	return (long long)timer_ticks() * TIMER_NS_NUM / TIMER_NS_DEN;
}

/*
 * psx_sleep waits for the given number of nanoseconds. It busy-waits:
 * there is nothing else to run, and halting the CPU would need an
 * interrupt to wake it.
 */
WEAK void
psx_sleep(long long ns)
{
	unsigned long long target = timer_ticks() + (unsigned long long)(ns * TIMER_NS_DEN / TIMER_NS_NUM);

	while (timer_ticks() < target)
		;
}
