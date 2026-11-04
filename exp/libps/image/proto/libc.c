/* The C library routines Net Yaroze programs call that the Go library
 * does not have, written from the C standard's descriptions of them.
 * printf writes through the library's putchar, as the kernel's TTY is
 * where a program's output went. */

typedef unsigned long size_t;
typedef __builtin_va_list va_list;
#define va_start(ap, last) __builtin_va_start(ap, last)
#define va_arg(ap, type) __builtin_va_arg(ap, type)
#define va_end(ap) __builtin_va_end(ap)

int putchar(int c);
size_t strlen(const char *s);
void *memcpy(void *, const void *, size_t);
void *memmove(void *, const void *, size_t);

#define USED __attribute__((used))

typedef unsigned long long u64;

/* Formatting, for the printf family, as Sony's library behaved when its
 * functions were probed: the conversions d i u x X o c s p and %, with the
 * flags - 0 + space and #, a width and a precision, either of which may be
 * *, and the lengths l (nothing on this machine) and h or hh (a short).
 * What differs between the functions is in struct style; printf2 and
 * sprintf2 add the floating point conversions f e g E G. */

struct out {
	char *buf; /* sprintf's buffer, or 0 for printf */
	int n;     /* the characters written */
	int cap;   /* the buffer's room, 0 for no limit */
	int conv;  /* the characters the conversions wrote */
};

struct style {
	int floats;   /* f e g E G are conversions */
	int stop;     /* an unknown conversion ends the output, rather than printing its letter */
	int pointer8; /* %p is %X zero-padded to eight digits, rather than %x */
};

static void emit(struct out *o, char c) {
	if (o->buf) {
		if (!o->cap || o->n < o->cap - 1)
			o->buf[o->n] = c;
	} else {
		putchar(c);
	}
	o->n++;
}

static void pad(struct out *o, char c, int n) {
	while (n-- > 0)
		emit(o, c);
}

/* field writes a converted field: the sign or prefix, zeros to fill, the
 * digits, padded to width on the side the flags say. */
static void field(struct out *o, const char *pre, int zeros, const char *s, int len, int width, int left) {
	int total = (int)strlen(pre) + zeros + len;
	if (!left)
		pad(o, ' ', width - total);
	for (; *pre; pre++)
		emit(o, *pre);
	pad(o, '0', zeros);
	for (int i = 0; i < len; i++)
		emit(o, s[i]);
	if (left)
		pad(o, ' ', width - total);
}

static void number(struct out *o, unsigned long v, int neg, int base, int upper,
                   int left, int zero, int plus, int space, int alt, int width, int prec) {
	char digits[32];
	const char *set = upper ? "0123456789ABCDEF" : "0123456789abcdef";
	int n = 0;
	unsigned long x = v;
	while (x) {
		digits[31 - n++] = set[x % base];
		x /= base;
	}
	if (n == 0 && prec != 0)
		digits[31 - n++] = '0';
	char pre[4] = {0};
	int p = 0;
	if (neg)
		pre[p++] = '-';
	else if (plus)
		pre[p++] = '+';
	else if (space)
		pre[p++] = ' ';
	if (alt && base == 16 && v)
		pre[p++] = '0', pre[p++] = upper ? 'X' : 'x';
	int zeros = prec > n ? prec - n : 0;
	if (alt && base == 8 && zeros == 0 && (n == 0 || digits[32 - n] != '0'))
		zeros = 1;
	if (prec < 0 && zero && !left) {
		int total = p + zeros + n;
		if (width > total)
			zeros += width - total;
	}
	field(o, pre, zeros, digits + 32 - n, n, width, left);
}

/* The exact decimal digits of a double, for the floating point
 * conversions. Sony's decoding is kept: every exponent field, 0 and 7FFh
 * included, is taken as a normal number's, so a denormal is read as
 * 2^-1023 times 1.m and an infinity or a NaN as 2^1024 times 1.m; only all
 * bits zero is zero. Its digits agree with the exact value's, rounded half
 * up, to 13 significant digits; past that Sony's carry its scaling error,
 * which these do not.
 *
 * dint holds the integer part's digits and dfrac the fraction's, as many
 * as were asked for. Static, since programs keep their stack in the 1KB
 * scratchpad. */

static char dint[330];
static int nint;
static char dfrac[1200];
static unsigned big[40];
static int nbig;
static int fracbits;

/* decimal sets up the digits of the value m times 2 to the e. */
static void decimal(u64 m, int e) {
	nint = 0;
	nbig = 0;
	fracbits = 0;
	if (e >= 0) {
		/* An integer: m shifted up, divided down by 10^9 for its digits. */
		for (int i = 0; i < 40; i++)
			big[i] = 0;
		int w = e / 32, b = e % 32;
		big[w] = (unsigned)(m << b);
		big[w + 1] = (unsigned)(b ? m >> (32 - b) : m >> 32);
		big[w + 2] = (unsigned)(b ? m >> (64 - b) : 0);
		int n = w + 3;
		while (n > 0 && big[n - 1] == 0)
			n--;
		static char rev[330];
		int r = 0;
		while (n > 0) {
			u64 rem = 0;
			for (int i = n - 1; i >= 0; i--) {
				u64 cur = rem << 32 | big[i];
				big[i] = (unsigned)(cur / 1000000000u);
				rem = cur % 1000000000u;
			}
			while (n > 0 && big[n - 1] == 0)
				n--;
			for (int k = 0; k < 9 && (n > 0 || rem); k++) {
				rev[r++] = '0' + (char)(rem % 10);
				rem /= 10;
			}
		}
		while (r > 0)
			dint[nint++] = rev[--r];
		return;
	}
	int k = -e;
	u64 ip = k < 64 ? m >> k : 0;
	char rev[24];
	int r = 0;
	while (ip) {
		rev[r++] = '0' + (char)(ip % 10);
		ip /= 10;
	}
	while (r > 0)
		dint[nint++] = rev[--r];
	/* The fraction: m's low k bits over 2^k, a digit at a time. */
	for (int i = 0; i < 40; i++)
		big[i] = 0;
	u64 f = k < 64 ? m & ((1ull << k) - 1) : m;
	big[0] = (unsigned)f;
	big[1] = (unsigned)(f >> 32);
	nbig = k / 32 + 2;
	fracbits = k;
}

/* fraction writes the next n digits of the fraction into dfrac from at. */
static void fraction(int at, int n) {
	for (int d = 0; d < n; d++) {
		if (!fracbits) {
			dfrac[at + d] = '0';
			continue;
		}
		u64 carry = 0;
		for (int i = 0; i < nbig; i++) {
			u64 cur = (u64)big[i] * 10 + carry;
			big[i] = (unsigned)cur;
			carry = cur >> 32;
		}
		int w = fracbits / 32, b = fracbits % 32;
		unsigned digit = (unsigned)(big[w] >> b);
		if (b && w + 1 < nbig)
			digit |= big[w + 1] << (32 - b);
		big[w] &= (1u << b) - 1;
		for (int i = w + 1; i < nbig; i++)
			big[i] = 0;
		dfrac[at + d] = '0' + (char)digit;
	}
}

/* roundup adds one to the digits s[0..n), half-up rounding having found
 * the next digit 5 or more, and says whether it carried out of the top. */
static int roundup(char *s, int n) {
	for (int i = n - 1; i >= 0; i--) {
		if (s[i] < '9') {
			s[i]++;
			return 0;
		}
		s[i] = '0';
	}
	return 1;
}

/* The digits of a double, as Sony's library produced them, its printf2
 * and sprintf2 probed over a few hundred values. It decodes every
 * exponent field, 0 and 7FFh included, as a normal number's: a denormal
 * reads as 1.m times 2^-1023, an infinity or a NaN as 1.m times 2^1024;
 * only all bits zero is zero, and a zero has no sign. Then, by the
 * value's size:
 *
 *   below 1, it is multiplied by 10, rounding a product halfway between
 *   two doubles away from zero, until it is 1 or more, and that double's
 *   exact digits are used;
 *   from 1 to 10^16, its own exact digits are used;
 *   from 10^16, it is divided by 10, rounded to nearest, until it is
 *   below 10^16, and that double's first 17 digits are used, then zeros.
 *
 * Probed with a few hundred values at 26 digits, these agree with Sony's
 * to 15 significant digits throughout, and most to the last. A conversion rounds the digits half up at
 * its precision, as Sony's did.
 *
 * A value is kept as m times 2 to the e, m with its top bit at 52. */

/* mul10 multiplies m times 2^e by 10, rounding halfway away from zero. */
static void mul10(u64 *m, int *e) {
	u64 p = *m * 10; /* below 2^57 */
	int s = p >> 56 ? 4 : 3;
	u64 q = (p + (1ull << (s - 1))) >> s;
	*e += s;
	if (q >> 53) {
		q >>= 1;
		*e += 1;
	}
	*m = q;
}

/* div10 divides m times 2^e by 10, rounding to nearest, halfway away
 * from zero. */
static void div10(u64 *m, int *e) {
	u64 n = *m << 11; /* below 2^64 */
	u64 q = n / 10, r = n % 10;
	*e -= 11;
	int t = 0;
	while (q >> (53 + t))
		t++;
	u64 out = q >> t;
	u64 rest = q & ((1ull << t) - 1), half = t ? 1ull << (t - 1) : 0;
	if (t && (rest > half || (rest == half && (r || 1))))
		out++;
	*e += t;
	if (out >> 53) {
		out >>= 1;
		*e += 1;
	}
	*m = out;
}

/* below and atleast compare m times 2^e with 1 and with 10^16. */
static int belowOne(u64 m, int e) { return e < -52; }
static int atLeast1e16(u64 m, int e) {
	if (e <= 0)
		return 0;
	if (e >= 11)
		return 1;
	return (m << e) >= 10000000000000000ull;
}

static char sig[1400]; /* the significant digits */
static int nsig;       /* how many have been made */
static int sigx;       /* the decimal exponent of the first */
static int sigcut;     /* digits from here on are 0, or -1 */

/* digitsOf makes the first n significant digits of m times 2^e into sig,
 * after the integer's: decimal and fraction leave them in dint and dfrac. */
static void significant(u64 bits, int need) {
	nsig = 0;
	sigcut = -1;
	if (bits << 1 == 0) {
		sigx = 0;
		for (int i = 0; i < need; i++)
			sig[i] = '0';
		nsig = need;
		return;
	}
	int ex = (int)(bits >> 52 & 0x7ff);
	u64 m = (bits & ((1ull << 52) - 1)) | 1ull << 52;
	int e = ex - 1075;
	int shift = 0;
	if (belowOne(m, e)) {
		while (belowOne(m, e)) {
			mul10(&m, &e);
			shift--;
		}
	} else if (atLeast1e16(m, e)) {
		while (atLeast1e16(m, e)) {
			div10(&m, &e);
			shift++;
		}
		sigcut = 17;
	}
	decimal(m, e);
	sigx = nint - 1 + shift;
	for (int i = 0; i < nint && nsig < need; i++)
		sig[nsig++] = dint[i];
	if (nsig < need) {
		int more = need - nsig;
		fraction(0, more);
		for (int i = 0; i < more; i++)
			sig[nsig++] = dfrac[i];
	}
	if (sigcut >= 0)
		for (int i = sigcut; i < nsig; i++)
			sig[i] = '0';
}

/* digitAt is the digit of the 10^pos place. */
static char digitAt(int pos) {
	int i = sigx - pos;
	return i >= 0 && i < nsig ? sig[i] : '0';
}

static char fbuf[1300];

/* floating writes a double, of the conversion c, into fbuf: its length. */
static int floating(u64 bits, char c, int prec, int alt, int *neg) {
	*neg = (bits << 1) != 0 && bits >> 63;
	if (prec < 0)
		prec = 6;
	char lc = c | 0x20;
	int upper = c == 'E' || c == 'G';
	int n = 0;
	if (lc == 'g' && prec == 0)
		prec = 1;
	if (lc == 'f') {
		/* Digits down to the 10^-prec place, one more to round with. */
		significant(bits, 1);
		int top = sigx > 0 ? sigx : 0;
		int want = sigx + prec + 2;
		significant(bits, want > 1 ? want : 1);
		int len = top + prec + 1;
		static char s[1300];
		for (int i = 0; i < len; i++)
			s[i] = digitAt(top - i);
		int carry = digitAt(-prec - 1) >= '5' && roundup(s, len);
		if (carry)
			fbuf[n++] = '1';
		for (int i = 0; i <= top; i++)
			fbuf[n++] = s[i];
		if (prec > 0 || alt)
			fbuf[n++] = '.';
		for (int i = 0; i < prec; i++)
			fbuf[n++] = s[top + 1 + i];
		return n;
	}
	/* %e and %g: the first digits, rounded at the precision. */
	int want = lc == 'e' ? prec + 1 : prec;
	significant(bits, want + 1);
	static char s[1300];
	for (int i = 0; i < want; i++)
		s[i] = sig[i];
	int x = sigx;
	if (sig[want] >= '5' && roundup(s, want)) {
		s[0] = '1';
		for (int i = 1; i < want; i++)
			s[i] = '0';
		x++;
	}
	if (bits << 1 == 0)
		x = 0;
	int exp = 1;
	if (lc == 'g') {
		if (x < -4 || x >= prec) {
			prec--;
		} else {
			exp = 0;
			int dec = prec - 1 - x;
			if (x >= 0)
				for (int i = 0; i <= x; i++)
					fbuf[n++] = s[i];
			else
				fbuf[n++] = '0';
			int point = n;
			fbuf[n++] = '.';
			for (int i = 0; i < dec; i++) {
				int at = x + 1 + i;
				fbuf[n++] = at >= 0 ? s[at] : '0';
			}
			if (!alt) {
				while (n > point + 1 && fbuf[n - 1] == '0')
					n--;
				if (fbuf[n - 1] == '.')
					n--;
			}
			return n;
		}
	}
	if (exp) {
		fbuf[n++] = s[0];
		if (prec > 0 || alt)
			fbuf[n++] = '.';
		for (int i = 1; i <= prec; i++)
			fbuf[n++] = s[i];
		if (lc == 'g' && !alt) {
			while (fbuf[n - 1] == '0')
				n--;
			if (fbuf[n - 1] == '.')
				n--;
		}
		fbuf[n++] = upper ? 'E' : 'e';
		fbuf[n++] = x < 0 ? '-' : '+';
		int ax = x < 0 ? -x : x;
		if (ax >= 100)
			fbuf[n++] = '0' + ax / 100;
		fbuf[n++] = '0' + ax / 10 % 10;
		fbuf[n++] = '0' + ax % 10;
	}
	return n;
}

static int format(struct out *o, const struct style *st, const char *f, va_list ap) {
	for (; *f; f++) {
		if (*f != '%') {
			emit(o, *f);
			continue;
		}
		int start = o->n;
		int left = 0, zero = 0, plus = 0, space = 0, alt = 0, width = 0, prec = -1, half = 0;
		for (;; f++) {
			char c = f[1];
			if (c == '-')
				left = 1;
			else if (c == '0')
				zero = 1;
			else if (c == '+')
				plus = 1;
			else if (c == ' ')
				space = 1;
			else if (c == '#')
				alt = 1;
			else
				break;
		}
		f++;
		if (*f == '*') {
			width = va_arg(ap, int);
			if (width < 0) {
				left = 1;
				width = -width;
			}
			f++;
		} else {
			while (*f >= '0' && *f <= '9')
				width = width * 10 + *f++ - '0';
		}
		if (*f == '.') {
			f++;
			prec = 0;
			if (*f == '*') {
				prec = va_arg(ap, int);
				f++;
			} else {
				while (*f >= '0' && *f <= '9')
					prec = prec * 10 + *f++ - '0';
			}
		}
		for (; *f == 'l' || *f == 'h'; f++)
			if (*f == 'h')
				half = 1;
		switch (*f) {
		case 'd':
		case 'i': {
			long v = va_arg(ap, long);
			if (half)
				v = (short)v;
			number(o, v < 0 ? -(unsigned long)v : (unsigned long)v, v < 0, 10, 0, left, zero, plus, space, 0, width, prec);
			break;
		}
		case 'u':
		case 'x':
		case 'X':
		case 'o': {
			unsigned long v = va_arg(ap, unsigned long);
			if (half)
				v = (unsigned short)v;
			int base = *f == 'o' ? 8 : *f == 'u' ? 10 : 16;
			number(o, v, 0, base, *f == 'X', left, zero, 0, 0, alt && *f != 'u', width, prec);
			break;
		}
		case 'p': {
			unsigned long v = (unsigned long)va_arg(ap, void *);
			if (st->pointer8)
				number(o, v, 0, 16, 1, left, 0, 0, 0, 0, width, 8);
			else
				number(o, v, 0, 16, 0, left, zero, 0, 0, 0, width, prec);
			break;
		}
		case 'c': {
			char ch = (char)va_arg(ap, int);
			field(o, "", 0, &ch, 1, width, left);
			break;
		}
		case 's': {
			const char *s = va_arg(ap, const char *);
			if (!s)
				s = "(null)";
			int n = 0;
			while (s[n] && (prec < 0 || n < prec))
				n++;
			field(o, "", 0, s, n, width, left);
			break;
		}
		case '%':
			emit(o, '%');
			break;
		case 'f':
		case 'e':
		case 'g':
		case 'E':
		case 'G':
			if (st->floats) {
				u64 bits = va_arg(ap, u64);
				int neg;
				int n = floating(bits, *f, prec, alt, &neg);
				const char *pre = neg ? "-" : plus ? "+" : space ? " " : "";
				int zeros = 0;
				if (zero && !left && width > n + (int)strlen(pre))
					zeros = width - n - (int)strlen(pre);
				field(o, pre, zeros, fbuf, n, width, left);
				break;
			}
			/* fall through */
		default:
			if (*f == 0)
				return o->n;
			if (st->stop)
				return o->n;
			emit(o, *f);
		}
		o->conv += o->n - start;
	}
	return o->n;
}

static const struct style plain = {0, 0, 0}, sprintfStyle = {0, 1, 1}, withFloats = {1, 0, 0};

/* printf, printf2, sprintf and sprintf2 return what Sony's did: sprintf
 * the length written, the others the characters their conversions wrote,
 * the format's own text not counted. */

USED int printf(const char *f, ...) {
	struct out o = {0, 0, 0, 0};
	va_list ap;
	va_start(ap, f);
	format(&o, &plain, f, ap);
	va_end(ap);
	return o.conv;
}

USED int printf2(const char *f, ...) {
	struct out o = {0, 0, 0, 0};
	va_list ap;
	va_start(ap, f);
	format(&o, &withFloats, f, ap);
	va_end(ap);
	return o.conv;
}

USED int sprintf(char *buf, const char *f, ...) {
	struct out o = {buf, 0, 0, 0};
	va_list ap;
	va_start(ap, f);
	int n = format(&o, &sprintfStyle, f, ap);
	va_end(ap);
	buf[n] = 0;
	return n;
}

USED int sprintf2(char *buf, const char *f, ...) {
	struct out o = {buf, 0, 0, 0};
	va_list ap;
	va_start(ap, f);
	int n = format(&o, &withFloats, f, ap);
	va_end(ap);
	buf[n] = 0;
	return o.conv;
}

/* Strings. */

USED char *strcpy(char *d, const char *s) {
	char *r = d;
	while ((*d++ = *s++))
		;
	return r;
}

USED char *strncpy(char *d, const char *s, size_t n) {
	char *r = d;
	for (; n && *s; n--)
		*d++ = *s++;
	for (; n; n--)
		*d++ = 0;
	return r;
}

USED char *strcat(char *d, const char *s) {
	strcpy(d + strlen(d), s);
	return d;
}

USED char *strncat(char *d, const char *s, size_t n) {
	char *e = d + strlen(d);
	for (; n && *s; n--)
		*e++ = *s++;
	*e = 0;
	return d;
}

USED int strcmp(const char *a, const char *b) {
	for (; *a && *a == *b; a++, b++)
		;
	return (unsigned char)*a - (unsigned char)*b;
}

USED int strncmp(const char *a, const char *b, size_t n) {
	for (; n && *a && *a == *b; n--, a++, b++)
		;
	return n ? (unsigned char)*a - (unsigned char)*b : 0;
}

USED char *strchr(const char *s, int c) {
	for (;; s++) {
		if (*s == (char)c)
			return (char *)s;
		if (!*s)
			return 0;
	}
}

USED char *strrchr(const char *s, int c) {
	const char *r = 0;
	for (;; s++) {
		if (*s == (char)c)
			r = s;
		if (!*s)
			return (char *)r;
	}
}

USED char *strstr(const char *h, const char *n) {
	size_t ln = strlen(n);
	for (; *h; h++)
		if (strncmp(h, n, ln) == 0)
			return (char *)h;
	return ln ? 0 : (char *)h;
}

USED size_t strspn(const char *s, const char *set) {
	size_t n = 0;
	while (s[n] && strchr(set, s[n]))
		n++;
	return n;
}

USED size_t strcspn(const char *s, const char *set) {
	size_t n = 0;
	while (s[n] && !strchr(set, s[n]))
		n++;
	return n;
}

USED char *strpbrk(const char *s, const char *set) {
	s += strcspn(s, set);
	return *s ? (char *)s : 0;
}

static char *tokpos;

USED char *strtok(char *s, const char *sep) {
	if (!s)
		s = tokpos;
	if (!s)
		return 0;
	s += strspn(s, sep);
	if (!*s) {
		tokpos = 0;
		return 0;
	}
	char *e = s + strcspn(s, sep);
	if (*e)
		*e++ = 0;
	else
		e = 0;
	tokpos = e;
	return s;
}

USED void *memchr(const void *p, int c, size_t n) {
	const unsigned char *s = p;
	for (; n; n--, s++)
		if (*s == (unsigned char)c)
			return (void *)s;
	return 0;
}

USED int bcmp(const void *a, const void *b, size_t n) {
	const unsigned char *x = a, *y = b;
	for (; n; n--, x++, y++)
		if (*x != *y)
			return *x - *y;
	return 0;
}

USED int toupper(int c) { return c >= 'a' && c <= 'z' ? c - 32 : c; }
USED int tolower(int c) { return c >= 'A' && c <= 'Z' ? c + 32 : c; }

static int isspace(int c) { return c == ' ' || (c >= '\t' && c <= '\r'); }

USED unsigned long strtoul(const char *s, char **end, int base) {
	const char *p = s;
	while (isspace(*p))
		p++;
	int neg = 0;
	if (*p == '-' || *p == '+')
		neg = *p++ == '-';
	if ((base == 0 || base == 16) && p[0] == '0' && (p[1] == 'x' || p[1] == 'X')) {
		p += 2;
		base = 16;
	} else if (base == 0) {
		base = *p == '0' ? 8 : 10;
	}
	unsigned long v = 0;
	const char *start = p;
	for (;; p++) {
		int d;
		if (*p >= '0' && *p <= '9')
			d = *p - '0';
		else if (*p >= 'a' && *p <= 'z')
			d = *p - 'a' + 10;
		else if (*p >= 'A' && *p <= 'Z')
			d = *p - 'A' + 10;
		else
			break;
		if (d >= base)
			break;
		v = v * base + d;
	}
	if (end)
		*end = (char *)(p == start ? s : p);
	return neg ? -v : v;
}

/* strtod reads a decimal number with an optional fraction and exponent:
 * the digits as an integer, then scaled by powers of ten. Exact for what
 * a program writes in a data file, and within a unit or two of the last
 * place otherwise. */
USED double strtod(const char *s, char **end) {
	const char *p = s;
	while (isspace(*p))
		p++;
	int neg = 0;
	if (*p == '-' || *p == '+')
		neg = *p++ == '-';
	unsigned long long m = 0;
	int scale = 0, digits = 0;
	for (; *p >= '0' && *p <= '9'; p++, digits++) {
		if (m < 100000000000000000ull)
			m = m * 10 + (*p - '0');
		else
			scale++;
	}
	if (*p == '.') {
		for (p++; *p >= '0' && *p <= '9'; p++, digits++) {
			if (m < 100000000000000000ull) {
				m = m * 10 + (*p - '0');
				scale--;
			}
		}
	}
	if (!digits) {
		if (end)
			*end = (char *)s;
		return 0;
	}
	if (*p == 'e' || *p == 'E') {
		const char *q = p + 1;
		int eneg = 0, e = 0;
		if (*q == '-' || *q == '+')
			eneg = *q++ == '-';
		if (*q >= '0' && *q <= '9') {
			for (; *q >= '0' && *q <= '9'; q++)
				if (e < 10000)
					e = e * 10 + (*q - '0');
			scale += eneg ? -e : e;
			p = q;
		}
	}
	if (end)
		*end = (char *)p;
	double d = (double)m;
	double ten = 10;
	int n = scale < 0 ? -scale : scale;
	double f = 1;
	for (; n; n >>= 1, ten *= ten)
		if (n & 1)
			f *= ten;
	d = scale < 0 ? d / f : d * f;
	return neg ? -d : d;
}

USED double atof(const char *s) { return strtod(s, 0); }

USED long strtol(const char *s, char **end, int base) { return (long)strtoul(s, end, base); }
USED int atoi(const char *s) { return (int)strtol(s, 0, 10); }
USED long atol(const char *s) { return strtol(s, 0, 10); }

/* Sorting and searching, by the comparison the program gives. */

static void swap(char *a, char *b, size_t n) {
	while (n--) {
		char t = *a;
		*a++ = *b;
		*b++ = t;
	}
}

USED void qsort(void *base, size_t n, size_t size, int (*cmp)(const void *, const void *)) {
	/* Insertion sort: short, stable, and enough for what these programs
	 * sort. */
	char *b = base;
	for (size_t i = 1; i < n; i++)
		for (size_t j = i; j > 0 && cmp(b + (j - 1) * size, b + j * size) > 0; j--)
			swap(b + (j - 1) * size, b + j * size, size);
}

USED void *bsearch(const void *key, const void *base, size_t n, size_t size,
                   int (*cmp)(const void *, const void *)) {
	const char *b = base;
	while (n) {
		size_t mid = n / 2;
		int c = cmp(key, b + mid * size);
		if (c == 0)
			return (void *)(b + mid * size);
		if (c > 0) {
			b += (mid + 1) * size;
			n -= mid + 1;
		} else {
			n = mid;
		}
	}
	return 0;
}

/* Two of the math functions, on the bits of the double: sqrt correctly
 * rounded, by the digit-by-digit square root of the significand, and
 * floor. */

static u64 bits(double d) {
	union {
		double d;
		u64 u;
	} x = {d};
	return x.u;
}

static double from(u64 u) {
	union {
		u64 u;
		double d;
	} x = {u};
	return x.d;
}

USED double sqrt(double d) {
	u64 u = bits(d);
	if (u << 1 == 0)
		return d;
	if (u >> 63)
		return from(0x7ff8000000000000ull);
	int e = (int)(u >> 52 & 0x7ff);
	if (e == 0x7ff)
		return d;
	/* d is m times 2 to the E, with m's top bit at 52. */
	u64 m = u & ((1ull << 52) - 1);
	if (e == 0) {
		e = 1;
		while (!(m >> 52)) {
			m <<= 1;
			e--;
		}
	} else {
		m |= 1ull << 52;
	}
	int E = e - 1023 - 52;
	if (E & 1) {
		m <<= 1;
		E--;
	}
	/* The root of m times 2 to the 54, a bit at a time from the top: 54
	 * bits, the 53 of the result and one to round with. m fills the top
	 * 64 bits of that 108-bit number, shifted up by 10. */
	u64 rem = 0, root = 0;
	m <<= 10;
	for (int i = 0; i < 54; i++) {
		rem = (rem << 2) | (m >> 62);
		m <<= 2;
		u64 t = (root << 2) | 1;
		root <<= 1;
		if (rem >= t) {
			rem -= t;
			root |= 1;
		}
	}
	/* Rounding off the last bit: an exact half would need the root to be
	 * exact in 54 bits and odd, which a 53-bit square cannot be. */
	u64 r = (root + 1) >> 1;
	int k = E / 2 - 26;
	if (r >> 53) {
		r >>= 1;
		k++;
	}
	return from((u64)(52 + k + 1023) << 52 | (r & ((1ull << 52) - 1)));
}

USED double floor(double d) {
	u64 u = bits(d);
	int e = (int)(u >> 52 & 0x7ff) - 1023;
	if (e >= 52)
		return d;
	if (e < 0) {
		if (u << 1 == 0)
			return d;
		return u >> 63 ? -1.0 : 0.0;
	}
	u64 frac = ((1ull << 52) - 1) >> e;
	if ((u & frac) == 0)
		return d;
	if (u >> 63)
		u += frac + 1;
	return from(u & ~frac);
}

/* FntPrint formats its arguments as printf does and adds the text to a
 * print stream: the one given first, when the first argument is a stream
 * number rather than a pointer to the format, or the default one. */
int libps_fnt_puts(int id, const char *s, int n);
int libps_kanji_puts(int id, const char *s, int n);

/* FntPrint's own formatting, smaller than printf's, as the original's was
 * probed: %d, %x (in capitals, as %X is), %c, which writes its byte as it
 * is, a 0 included, and %s, each with a width of digits, which pads with
 * spaces on the left, or for %x with zeros when the width starts with 0;
 * and %% with no width. Anything else, a flag, a precision, a length or another letter,
 * fills the stream with spaces, which FntPrint answers with -1. fntText
 * gets the text and says whether it came to such a directive. */
static char fntBuf[1100];
static int fntLen;

static void fntEmit(char c) {
	if (fntLen < (int)sizeof fntBuf)
		fntBuf[fntLen++] = c;
}

static void fntField(const char *s, int n, int width, char pad) {
	for (int i = n; i < width; i++)
		fntEmit(pad);
	for (int i = 0; i < n; i++)
		fntEmit(s[i]);
}

static int fntText(const char *f, va_list ap) {
	fntLen = 0;
	for (; *f; f++) {
		if (*f != '%') {
			fntEmit(*f);
			continue;
		}
		f++;
		int width = 0, digits = 0;
		char pad = *f == '0' ? '0' : ' ';
		for (; *f >= '0' && *f <= '9'; f++, digits++)
			width = width * 10 + *f - '0';
		char num[12];
		int n = 0;
		switch (*f) {
		case 'd': {
			int v = va_arg(ap, int);
			unsigned u = v < 0 ? 0u - (unsigned)v : (unsigned)v;
			do {
				num[11 - n++] = '0' + u % 10;
				u /= 10;
			} while (u);
			if (v < 0)
				num[11 - n++] = '-';
			fntField(num + 12 - n, n, width, ' ');
			break;
		}
		case 'x':
		case 'X': {
			unsigned u = va_arg(ap, unsigned);
			do {
				num[11 - n++] = "0123456789ABCDEF"[u & 15];
				u >>= 4;
			} while (u);
			fntField(num + 12 - n, n, width, pad);
			break;
		}
		case 'c': {
			char c = (char)va_arg(ap, int);
			fntField(&c, 1, width, ' ');
			break;
		}
		case 's': {
			const char *s = va_arg(ap, const char *);
			fntField(s, (int)strlen(s), width, ' ');
			break;
		}
		case '%':
			if (!digits) {
				fntEmit('%');
				break;
			}
			/* fall through */
		default:
			return 1;
		}
	}
	return 0;
}

/* KanjiFntPrint's formatting, as the original's was probed: %d and %x
 * (%X too) in full-width characters, two bytes each, the digits from
 * 824Fh and A to F from 8260h, a minus sign 815Bh; %c and %s, their bytes
 * as they are; and %% with no width. A width counts full-width
 * characters, a field's bytes halved, and is made up with single-byte
 * spaces on the left, or for %x with full-width zeros when it starts with
 * 0. Any other conversion is a space, taking no argument. */
static void kanjiWide(int d) {
	fntEmit((char)0x82);
	fntEmit((char)(d < 10 ? 0x4f + d : 0x60 + d - 10));
}

static void kanjiText(const char *f, va_list ap) {
	fntLen = 0;
	for (; *f; f++) {
		if (*f != '%') {
			fntEmit(*f);
			continue;
		}
		f++;
		int width = 0, digits = 0;
		int zeros = *f == '0';
		for (; *f >= '0' && *f <= '9'; f++, digits++)
			width = width * 10 + *f - '0';
		char d[12];
		int n = 0;
		switch (*f) {
		case 'd': {
			int v = va_arg(ap, int);
			unsigned u = v < 0 ? 0u - (unsigned)v : (unsigned)v;
			do {
				d[n++] = (char)(u % 10);
				u /= 10;
			} while (u);
			int chars = n + (v < 0);
			for (int i = chars; i < width; i++)
				fntEmit(' ');
			if (v < 0) {
				fntEmit((char)0x81);
				fntEmit((char)0x5b);
			}
			while (n > 0)
				kanjiWide(d[--n]);
			break;
		}
		case 'x':
		case 'X': {
			unsigned u = va_arg(ap, unsigned);
			do {
				d[n++] = (char)(u & 15);
				u >>= 4;
			} while (u);
			for (int i = n; i < width; i++) {
				if (zeros)
					kanjiWide(0);
				else
					fntEmit(' ');
			}
			while (n > 0)
				kanjiWide(d[--n]);
			break;
		}
		case 'c': {
			char c = (char)va_arg(ap, int);
			for (int i = 0; i < width; i++)
				fntEmit(' ');
			fntEmit(c);
			break;
		}
		case 's': {
			const char *s = va_arg(ap, const char *);
			int len = (int)strlen(s);
			for (int i = len / 2; i < width; i++)
				fntEmit(' ');
			for (int i = 0; i < len; i++)
				fntEmit(s[i]);
			break;
		}
		case '%':
			if (!digits) {
				fntEmit('%');
				break;
			}
			/* fall through */
		default:
			if (*f == 0)
				return;
			fntEmit(' ');
		}
	}
}

/* FntPrint adds its text to a print stream: the one given first, when the
 * first argument is a number rather than a pointer to the format, or the
 * default one. A stream that is not open is left alone, and 0 returned. */
USED int libps_c_FntPrint(unsigned long first, ...) {
	va_list ap;
	va_start(ap, first);
	int id = -1;
	const char *f;
	if (first < 0x80000000ul) {
		id = (int)first;
		f = va_arg(ap, const char *);
	} else {
		f = (const char *)first;
	}
	int fill = fntText(f, ap);
	va_end(ap);
	int n = libps_fnt_puts(id, fntBuf, fntLen);
	if (fill && n > 0) {
		static char spaces[1100];
		if (spaces[0] != ' ')
			memset(spaces, ' ', sizeof spaces);
		n = libps_fnt_puts(id, spaces, sizeof spaces);
	}
	return n;
}

USED int libps_c_KanjiFntPrint(unsigned long first, ...) {
	va_list ap;
	va_start(ap, first);
	int id = -1;
	const char *f;
	if (first < 0x80000000ul) {
		id = (int)first;
		f = va_arg(ap, const char *);
	} else {
		f = (const char *)first;
	}
	kanjiText(f, ap);
	va_end(ap);
	return libps_kanji_puts(id, fntBuf, fntLen);
}

/* Two helpers of the math routines Sony's compiler linked into programs,
 * as they behave when called (each was probed on the original):
 *
 * _dbl_shift shifts the 64 bits of v, a double's two words, left when
 * right is 0 and right otherwise, copying the sign bit, by n, and stores
 * them at p. An n of 0 or less leaves v as it is; 64 or more shifts every
 * bit out. It returns p.
 *
 * _err_math records a math error: the error number in math_errno and the
 * number of the routine that met it in the word after. It returns 0. */

USED u64 *_dbl_shift(u64 *p, int right, u64 v, int n) {
	if (n > 0) {
		if (!right)
			v = n >= 64 ? 0 : v << n;
		else if (n >= 64)
			v = (long long)v < 0 ? ~0ull : 0;
		else
			v = (u64)((long long)v >> n);
	}
	*p = v;
	return p;
}

extern unsigned char math_errno[8];

USED int _err_math(int code, int routine) {
	int *e = (int *)math_errno;
	e[0] = code;
	e[1] = routine;
	return 0;
}

/* The console's input and output, through the kernel's read and write,
 * as the originals behaved when probed: putc writes c to fd as it is and
 * returns what write did; getc reads a byte from fd and returns it, 0xff
 * when none came; getchar reads one from the console and returns it, and
 * what it read before when none came. */

int libps_kread(int fd, void *buf, int n);
int libps_kwrite(int fd, const void *buf, int n);

static char putcByte, getcByte, getcharByte, getsByte;

USED int putc(int c, int fd) {
	putcByte = (char)c;
	return libps_kwrite(fd, &putcByte, 1);
}

USED int getc(int fd) {
	if (libps_kread(fd, &getcByte, 1) != 1)
		return 0xff;
	return (unsigned char)getcByte;
}

USED int getchar(void) {
	libps_kread(0, &getcharByte, 1);
	return (unsigned char)getcharByte;
}

/* gets reads a line from the console into s, echoing it, and returns s.
 * A byte is taken as its low 7 bits. A newline or a carriage return ends
 * the line, echoed as both; a backspace or a delete takes back the last
 * character, echoed as backspace, space, backspace; a tab is a space; any
 * other control character, or a character past the 125 a line holds, is
 * not kept and rings the bell. */
USED char *gets(char *s) {
	static const char crlf[] = "\r\n", rubout[] = "\b \b", bell = 7;
	int n = 0;
	for (;;) {
		libps_kread(0, &getsByte, 1);
		char c = getsByte & 0x7f;
		if (c == '\n' || c == '\r') {
			libps_kwrite(1, &crlf[0], 1);
			libps_kwrite(1, &crlf[1], 1);
			s[n] = 0;
			return s;
		}
		if (c == '\b' || c == 0x7f) {
			if (n > 0) {
				n--;
				for (int i = 0; i < 3; i++)
					libps_kwrite(1, &rubout[i], 1);
			}
			continue;
		}
		if (c == '\t')
			c = ' ';
		if (c < ' ' || n >= 125) {
			libps_kwrite(1, &bell, 1);
			continue;
		}
		s[n++] = c;
		getsByte = c;
		libps_kwrite(1, &getsByte, 1);
	}
}
