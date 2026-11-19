# Milestone 11: floating point, number parsing and errno

Status: implemented, in exp/cleanlib/softfloat.go, math.go, strtod.go
and mathexport.go. The software floating point the compiler calls
(__adddf3 and the rest, which the probe found the resident library
supplies), _get_errno, frexp, ldexp, modf, floor, ceil, fmod, sqrt, exp,
sinh, cosh, sin, cos, tan, tanh, atan, atan2, asin, acos, hypot, log,
log10, pow, strtod and atof match Sony's library on every probed input
of the seven probes. Sony's library turns out to be a plain one:
Taylor and atanh series summed to convergence, continued fractions,
two-term series for small arguments, and iterations run a fixed number
of times, with the textbook forms only in sqrt and floor (Seventh
Edition Unix) and in the reductions of sin and tan (Cody and Waite).

The C library calls the clean library still lacks:

- Floating point (double): acos, asin, atan, atan2, ceil, cos, cosh, exp,
  floor, fmod, frexp, hypot, ldexp, log, log10, modf, pow, sin, sinh,
  sqrt, tan, tanh.
- Parsing: atof, strtod.
- _get_errno, the error the last file call left (spec 11's calls).

No example makes these calls. The work is a probe that must match Sony's
library bit for bit.

## The console has no floating point unit

The R3000A has no FPU on the PlayStation, so Sony's library works its
doubles in software, and so does the clean library: TinyGo compiles Go's
float64 arithmetic to software routines for this target. The `yaroze`
bindings declare these calls with float64 arguments and results
(`yaroze/zz_generated_yaroze.go`), and TinyGo passes them in the
target's soft-float calling convention on both sides, so an exported Go
function with the same signature fits.

Bit-exact results are the goal: a game that uses these calls for its
state must run the same on both libraries. Go's math package or a
straight port of a textbook algorithm will often differ in the last bit
from Sony's. The probe decides. Where Sony's results show a known
algorithm (fdlibm's, a polynomial of a given degree, a range
reduction), write that down here, as a rule, from the results.

## How it is checked

A probe in `exp/cleanlib/spec/probes/math/`, built for Sony's library
and for the clean one (as spec 09 says), calls each function over a
fixed list of inputs and prints each result as the 16 hex digits of its
bits, so that no formatting hides a difference. The inputs:

- each function's ordinary range, densely: a few hundred values from a
  fixed pseudo-random sequence, and simple ones (0, ±0.5, ±1, ±2, π/4,
  π/2, π, 10, 100, 1e10);
- the edges: ±0, subnormals, the largest and smallest normals, ±Inf,
  NaN (print the bits: which NaN comes back counts), values just inside
  and outside each domain (asin(1+ε), log(0), log(−1), sqrt(−0),
  pow(0, 0), pow(−8, 1/3), fmod(x, 0), atan2 with every sign and zero
  combination), and large arguments for sin, cos and tan (1e6, 1e15,
  1e300);
- after each call that can fail, _get_errno: whether the math calls set
  it at all (EDOM, ERANGE), or only the file calls do;
- frexp's and modf's second results, with the pointer's memory around
  it printed too (is anything past the stored value written?);
- atof and strtod over strings: plain and signed numbers, exponents,
  leading spaces, hexadecimal, "inf" and "nan" in any case, overflow
  and underflow, empty and garbage strings, and strtod's end pointer as
  an offset into the string;
- _get_errno after spec 11's file calls fail (a missing file, a full
  card, a bad fd) and succeed, with each card setup spec 11 lists (its
  harness puts cards in).

Printing is slow in the emulator, so log results in memory and print
them after each group, as `spec/probes/voices` does.

## Done when

- The probe gives the same TTY on the clean library as on Sony's, bit
  for bit, every line.
- Every rule found is written here, under a heading for each function or
  family.
- `exp/cleanlib/regress.sh` gives no FAIL lines, and the same TIME lines
  as before: cdda, cdda:4000, check, dino, dungeon, flame, morph,
  offscr, terra, timers, tunnel, tunnel:0800.

## The probe and its harness

The probe is `spec/probes/math`. Built for Sony's library with `go run
./cmd/yaroze disc --opt=2 --elf .build/mathprobe-sony/prog.elf -o
.build/mathprobe-sony/mathprobe-sony.bin exp/cleanlib/spec/probes/math`
and for the clean one with `exp/cleanlib/build.sh mathprobe-clean
exp/cleanlib/spec/probes/math`, it runs on spec 11's harness,
`.build/cardprobe` (not in git) TestDump, with a blank card in slot 1,
to which this milestone added:

- ERRNO_STUB=1: OpenBIOS has no B0h 54h and halts on it, so a breakpoint
  at 0xB0 answers it with 1000h plus the number of such calls so far,
  returning to the caller. Every probe line's errno column is that.
- STALL=n and DONE=text: stop after n frames with no new TTY (a call
  that never returns) or once the TTY holds the text.
- WHERE=1: at a stall, print the PC and, inside the probe's own text,
  its symbol, and the probe's `main.cur` (the argument in progress).
- FN=symbol, SETREGS, SETREG: log the calls of one of the program's
  link symbols with arguments and returns; set registers at its entry.

`ERRNO_STUB=1 STALL=1200 DONE="math probe done" FRAMES=300000 CARD1=blank
DISC=... OUT=... go test -run TestDump` in `.build/cardprobe`; compare
the TTYs from the line "math probe errno" on, less the library's own
start-up lines ("PS-X Control PAD Driver", "addr=", "ResetGraph:"),
which the two libraries print at different times.

What the probe has to do to give the same TTY twice:

- No floating point of its own: the program's own float64 operations
  call the library's helpers, Sony's in a Sony build, so inputs are made
  from bits (`bits`, `fromInt`, `neg`) and the skip tests are on bits.
- Nothing allocated (the target's allocator never frees): printing goes
  through `ps`/`pi` into a line written with write(1, ...).
- The calls go through function values or memory: a call of a C math
  name with a constant argument is worked out by the compiler (LLVM
  knows sqrt, ldexp and the rest) and never reaches the library.
- Interrupts are off for the math (EnterCriticalSection, let in again
  only to print), and the random groups print every 20 calls. Without
  the second, Sony's sqrt never returned some way into a run of a hundred
  random calls (at a different argument in each build of the probe, and
  never when the same calls were made alone); the cause was not found.
- The calls Sony's library never returns from are skipped, printed as
  deaddeaddeaddead: see "Calls that never return".

A second probe, `spec/probes/mathtiny`, calls each function over a grid
of the smallest arguments (every exponent field from 0 up, several
mantissas each) and over dense scans of a few ordinary ranges, and runs
on the same harness in under a minute (`STALL=600 DONE="mathtiny probe
done" FRAMES=100000`). Below about 2^-511 a square underflows, and the
arithmetic's underflow makes garbage (the exponent runs into the sign):
the garbage is a function of exactly which products and quotients the
library takes, in which order, and it is how the forms below were found.
A result's bits can also show its last operation: a quotient's result
below the normal range comes out as a denormal (nothing else makes
one), and a difference of two numbers near one has trailing zero bits.

A third probe, `spec/probes/mathsingle`, calls the single precision
helpers and the double quotient directly over grids (same harness,
`STALL=600 DONE="mathsingle probe done" FRAMES=100000`, a few seconds):
__truncdfsf2, __divsf3 and __mulsf3 with the result's exponent walked
from the bottom of the single range down past 64 (every shortfall to 70,
then larger ones including those hypot's open ratios take), for ten
mantissas; __divdf3 over the same kind of grid; __mulsf3 over products
whose bits below the last place take chosen shapes (exact ties, near
ties, just above and below the half, at several exponents), over a grid
of the low sixteen bits of the dropped part, and over the squares hypot
takes of its truncated ratios; and strtod at the limits of its exponent.
Walking one operation at a time is what found the rules below, where
the earlier probes, which only saw the operations through hypot and
strtod, had given results no single rule fit. The two libraries give the
same TTY on it, 7429 lines.

A fourth probe, `spec/probes/mathatan` (same harness, `STALL=600
DONE="mathatan probe done" FRAMES=100000`, fifteen seconds on Sony's
library), is how atan's last cases were closed. It works atan's continued
fraction out step by step on the library's own helpers (__muldf3,
__divdf3, __adddf3 and __floatsidf called by name), so that one run on
Sony's library scores four candidate orderings of the term against
atan(x) over the whole tiny grid, prints every intermediate value of the
fraction for the arguments the open cases hand atan, and prints the
chain of asin and atan2 for those cases; and it calls __muldf3 directly
on 468 products made on the host so that the exact product's fraction
below the last place is a chosen distance above or below the half
(mulpairs.go). The clean build of the same probe gives the same 1232
lines. Comparing the two builds line by line is what found the product's
form: the first line to differ named the operation.

A fifth probe, `spec/probes/mathpow` (same harness, `STALL=600
DONE="mathpow probe done" FRAMES=100000`, seven seconds), closed pow's
and log's last cases. It calls exp on 240 arguments within a few ulps of
(k + 1/2) ln 2 (ties.go, made by a host simulation of the clean
library's arithmetic), where the two ways of forming exp's N round to
different integers; prints pow(x, y) beside exp(y * log(x)) worked out
on the library's own log, product and exp for the open cases; calls log
and frexp over a grid of 15 exponent fields, both ends of the range and
a few in the middle, by 28 mantissas placed around sqrt(1/2) and at the
ends (420 lines); and works log's own form n ln 2 + 2 sum(f) on the
library's helpers for chosen n and f, to name the pair that gives
log(NaN) and log(2^-1074). The clean build gives the same 727 lines.

A sixth probe, `spec/probes/mathldexp` (same harness, `STALL=600
DONE="mathldexp probe done" FRAMES=100000`, ten seconds), closed ldexp.
It calls ldexp on 36 arguments (a fixed fraction at eleven exponent
fields from 0 to 2047, both signs; the two zeros; the powers of two
2^-1022, 2^-1021 and 2^-1020, the smallest denormal pattern, one and the
infinity pattern, both signs) with the power at and around every 1024
boundary from 4096 down to -8192 (each boundary and three on each side),
further out at -16384, -32768, -65536, -100000, -1442695, -92332570 (the
two exp hands it for -1e6 and -6.4e7), -2^20, -2^24, -2^30 and -2^31+1,
at 65536, 2^30 and 2^31-1, around the anomaly (-1073 to -1077), and at
the powers that put the field plus the power on 0, 1, 1023, 1024, 2046,
2047, 2048, -1, -1024, -1025, -2048, -2049, -3072, -3073 and -4096; then
scans every power from -1000 to -1100 and from -2030 to -2130 for three
arguments. 5070 lines, the same on both libraries. The rule was first
fitted on the host against the main probe's ldexp, exp, sinh, cosh and
frexp lines, then checked against this probe's lines before the clean
build ran.

A seventh probe, `spec/probes/mathdenorm` (same harness, `STALL=600
DONE="mathdenorm probe done" FRAMES=100000`, a few seconds), closed the
last two lines, hypot of the smallest denormal with itself and sin of
-(2^-1021 - ulp). It calls __muldf3, __adddf3, __subdf3, __divdf3 and
__ltdf2 by name on operands whose exponent field is 0 (eight fractions)
against twelve normal numbers and against each other, both operand
orders, and __adddf3 and __subdf3 over a scan of the exponent difference
from 2046 down against the garbage hypot's first step makes; works
hypot's three steps on the helpers for the open pair and twelve
neighbors, printing every intermediate value and the final value of six
orderings of the step beside hypot itself; calls hypot on thirty tiny
pairs; and calls sin on sixteen pairs of arguments of both signs, chosen
on the host so that a reduction on the signed argument and one on its
magnitude round apart, printing both reductions worked on the helpers.
739 lines, the same on both libraries. Comparing the two builds line by
line is what decided both cases: every helper line was the same, so what
differed was the order of hypot's step and the sign sin reduces (see
those sections).

## What the compiler rewrites

The clean library's math is plain Go arithmetic, which the compiler
turns into calls of the library's own helpers in the order written, with
two exceptions that change results and so go through the helpers by name
(math.go's div and sub): a quotient by a power of two becomes a product
(exact in IEEE arithmetic, but the library's product has no denormal
result where its quotient does, and exp's garbage for tiny arguments
depends on that), and a - b*c becomes a + b*(-c) (for a zero b the
product is -0, and the library's sum of +0 and -0 returns its second
operand where its difference returns the first: sin(0) and tan(0) came
out -0). A product with -1 becomes a sign flip, which agrees with the
library's product even for a zero (both negate it); only the library's
own negation, __negdf2, differs, making +0 of either zero, so the code
negates through neg where Sony's does and multiplies where it
multiplies (sin's odd multiples of pi). A product by 2 becomes a sum of
the operand with itself, which agrees with the library's product on
every pattern (exact, a zero's sign kept, and an infinity of the sign
past the top: hypot's step depends on that overflow).

## Software floating point

Every float64 and float32 operation of a program is a call of a helper
the resident library exports: __adddf3, __subdf3, __muldf3, __divdf3,
__negdf2, __eqdf2, __nedf2, __ltdf2, __ledf2, __gtdf2, __gedf2,
__fixdfsi, __floatsidf, __addsf3, __mulsf3, __divsf3, __extendsfdf2 and
__truncdfsf2 (no __subsf3, __unorddf2, __floatunsidf or 64-bit
conversions: a program that needs them does not link). The math calls
work in the same arithmetic. The clean library exports them all.

### Numbers

- Zero is all bits zero, either sign. Every other pattern is a normal
  number: the exponent field is never special, so a denormal reads as
  1.f times 2^-1023, an infinity as 2^1024 and a NaN as 1.f times 2^1024.
  The mathdenorm probe fed the field-0 patterns to every double helper
  directly (eight fractions from 1 to 2^52 - 1, times, plus, minus and
  over twelve normal numbers from 2^-1022 to 2^1000 and the garbage -1.6
  times 2^1023, both operand orders, and against each other): every
  result is the one this reading gives, and a result's packing is the
  same whether or not an operand's field is 0. The sum's rule for a large
  exponent difference holds at every difference from 2046 down (the
  scan of 1.0 times 2^(e - 1023), e from 0 to 70, against that garbage
  and 2^1023): the larger operand comes back as it is, nothing of the
  smaller shifted in.

### Sums (__adddf3, __subdf3)

- add: a zero first operand returns the second as it is (0 + -0 = -0),
  then a zero second returns the first. sub: a zero second returns the
  first, then a zero first returns the second with its sign flipped
  (-0 - -0 = -0, 0 - -0 = +0).
- The larger magnitude is kept; if the exponents differ by more than 54
  the larger operand is returned as it is (so NaN + 1 is the NaN).
- Both significands get 9 bits below the last place; the smaller is
  shifted right to line up as a two's complement number would be: a
  positive one loses its bits toward zero, a negative one toward minus
  infinity (its magnitude goes up by one in the ninth bit when anything
  is lost). Then the magnitudes are added or subtracted, normalized, and
  rounded to nearest, ties to even, on the 9 bits. An exact zero is +0.
- A result whose exponent field passes 2046 is an infinity of its sign
  (7ff0000000000000). One below 1 is packed anyway: the sign, OR the
  exponent as a 64-bit signed number shifted into place, OR the
  fraction, so a negative exponent sets the sign bit and the top
  exponent bits (2^-1022 - 2^-1023(1+2^-52) = fffffffffffffffe).

### Products and quotients

- __muldf3: zero times anything is a zero, sign the XOR. The product of
  the significands is not the exact 106-bit one: each significand is
  shifted up so that its hidden bit is the top bit of a 64-bit word and
  split into 32-bit halves, and the top 64 bits of the product are taken
  as the high halves' product plus the high words of the two cross
  products, the low halves' product and the cross products' low words
  never added. So the product is short by up to three times 2^-10 of an
  ulp (each of the three dropped parts is below 2^32 times 2^32 in a
  128-bit product whose ulp is 2^74). That is then rounded half up (away
  from zero) on
  the bit below the last place, so where the exact product lies above a
  half ulp by less than what was dropped it rounds down instead of up.
  Overflow and underflow as for sums. The random products of the first
  probe never showed it (the smallest excess over a half among them was
  2^43 units of the 106-bit product, where the loss is at most about
  2^44 and usually far less): it came out in atan's fraction, whose
  numerators k*k*x*x for a denormal x (the quotients asin and atan2 hand
  atan have the form 0003ffffffff....) and for x = 0.75 - 2^-53 lie
  just above a half (by 1 to 2^34 units) and Sony rounds them down, and
  was pinned by the mathatan probe's 468 made products: among the forms
  that fit the random products and the first 360 (the same split with a
  shift of 10 and the low halves' product alone dropped; bytes with
  everything below bit 40 dropped), 108 products on which they disagree
  decided it. 2764 probed products match in all, and the 285 of the
  traced fractions.
- __divdf3: anything over zero is 7fffffffffffffff with the XOR of the
  signs; zero over anything is a zero with that sign. The quotient has 9
  bits below the last place and the remainder as a sticky bit, rounded
  to nearest, ties to even; past the top an infinity. Below the normal
  range, unlike the others, it makes a denormal, and the mathsingle grid
  (2106 quotients, shortfalls 1 to 71) gives its exact form: y is the
  53-bit significand with one guard bit below it, sh the shortfall (one
  less the exponent field), and the result is (y + (1 << sh)) >> (sh+1)
  with the hidden bit's place cleared. The half, 1 << sh, is a 32-bit
  shift: up to a shortfall of 31 this rounds half up and a carry into the
  hidden bit is lost (2^-1022(1-2^-53) / 1 = 0); from 32 on the half lands
  far below the last place and the result is truncated, an ulp below the
  half-up rounding (the "ulp below nearest" atan's and tan's tiny cases
  had shown at shifts of 36 and 39); from a shortfall of 63 the result is
  a signed zero.

### Comparisons, negation, conversions

- The comparisons order the patterns as sign and magnitude, the two
  zeros equal, NaNs as the large numbers they read as. __eqdf2 and
  __nedf2 return 0 if equal, else 1; __ltdf2 and __gedf2 -1 if less,
  else 0; __gtdf2 and __ledf2 1 if greater, else 0.
- __negdf2 flips the sign, but either zero gives +0. (The compiler's own
  negation of a float64 flips the bit, so the clean library negates
  through dNeg where Sony's C did.)
- __fixdfsi truncates toward zero and saturates at 2147483647 and
  -2147483648; -0 gives -2147483648.
- __floatsidf is exact, but never returns for -2147483648.
- Single precision (__addsf3, __mulsf3, __divsf3, __extendsfdf2,
  __truncdfsf2): the same arithmetic on 24-bit significands, found
  through hypot, which works in floats, and then pinned by the
  mathsingle probe. Every pattern but zero is a normal number; the sum
  and quotient round to nearest even, the sum from 6 bits below the last
  place (9 give one of 641 wrong); a sum's or product's result below the
  normal range is packed with its exponent running into the sign (0.1
  times 2^-126 is fecccccd); a zero divisor gives 4f000000 (2^31), or
  4f800000 when the signs differ. __extendsfdf2 is exact, denormals
  included (00000001 is 2^-149). Every probed sum, product, quotient and
  conversion matches, 7429 lines of the mathsingle probe and 641 of the
  math probe.
- __mulsf3 forms the product of the significands from their bytes: the
  nine byte products summed at their weights into the top 32 bits of the
  48-bit product (bits 47 to 16), the lowest, a0*b0, dropped and the two
  of weight 2^8 taken without their low byte; then rounded half up to 24
  bits. On exact products and on anything not within 2^16 of a half this
  is plain half up, which is all the first probes saw; the mathsingle
  grids showed the loss: an exact tie rounds up (even when the mantissa
  is even), a near tie, the half followed by zeros and then something far
  down, rounds down when the something came from the dropped bytes
  (0x80152f squared is 0x802a61, not 0x802a62; 0x8156e2 squared is
  0x82b15a) and up when it did not (a near tie made with 1+2^-23, whose
  dropped bits are the other operand's fraction, rounds up for every
  value of its low sixteen bits), and splitting only one operand into
  bytes misses three of 2701 probed products. The exponent does not
  enter: the same shapes behave the same from 2^-127 up to 2^100.
- __truncdfsf2 rounds to nearest (half up) and gives an infinity from
  the top of the single range up (NaNs included). Below the bottom of
  the range, __truncdfsf2 and __divsf3 pack the result the same way the
  double quotient does, in 32 bits: y is the 24-bit significand with one
  guard bit below it (for the conversion, the double's significand cut
  to 25 bits), sh the shortfall, and the result is (y + (1 << sh)) >>
  (sh+1) with bit 23 cleared, the shift amounts taken modulo 32. So, by
  the shortfall modulo 32: 1 to 23 round half up with a carry into bit
  23 lost (2^-126(2-2^-23) / 2 = 0); 24 always gives 1 (the half of the
  hidden bit); 25 to 30 give 0; 31 shifts by nothing and adds the half
  at bit 31, so the result is 80000000 with y as it is less its bit 23
  (00800000 / 7f000000 = 81000000, 1.25 times 2^-157 narrowed is
  81400000, and the hypot ratios whose shortfall is 31 modulo 32 come
  back as these garbage numbers); from 32 on these repeat, a multiple of
  32 shifting y down by one place only (2^-1023 narrowed is 00400000,
  shortfall 897; 1e-300 is 000156e2, 871; 2^-1022 is 0, 896, nothing
  left once bit 23 is cleared; 2^-191 / 1 is 00400000 by the quotient).
  1100 probed conversions and 2705 probed quotients match.

## frexp, modf, floor, ceil, fmod

- frexp(x, &e): a zero is returned as it is, e = 0. Otherwise e is the
  exponent field less 1022, and the mantissa is ldexp(x, -e) by the
  ldexp rule below: correct up to an exponent field of 2045 (the power
  -1023 is ten products that take the field down to 1022), but for a
  field of 2046 or 2047 (max, infinity, NaN) the power is -1024 or -1025,
  whose bit of 1024 multiplies by -2^1024, and the mantissa comes back an
  infinity of the opposite sign. The spec's earlier reading, x times a power of two built from the
  bits (1023-e) shifted into the exponent field, is the same everywhere
  but at a field of 2045, where that multiplier is a zero and the mantissa
  would be 0: the mathpow probe's grid has 28 such inputs and Sony gives
  the mantissa. Only the int at e is written.
- modf: the integer part is x with the fraction bits below the binary
  point cleared, +0 when |x| < 1 (whatever the sign), x itself from 2^52
  up (NaN included); the fraction is x less that, so an integer's is +0.
  Only the double at the pointer is written.
- floor and ceil are the Seventh Edition Unix ones over that modf:
  floor(d) for d < 0 is -(ip + 1 if the fraction of -d is not 0), else
  modf's integer part; ceil(d) = -floor(-d), with the library's negation,
  so floor(-0) = ceil(-0) = ceil(-0.5) = +0 and a signalling NaN comes
  back unchanged.
- fmod(x, y): 0 if y is 0, else x - ip*y with ip modf's integer part of
  x/y.

## ldexp

- ldexp(x, n) is x times 2^n by binary powering in the library's own
  arithmetic: for each set bit of |n| from the lowest, x is multiplied by
  the running power of two, which starts at 2 for a positive n and at 1/2
  for a negative one and is squared each round (pow's integer method with
  x in place of its 1). There is no check on n or on x's bits; everything
  the results show follows from the products' rules.
- The powers: 2, 4, 16, ..., 2^512, then 2^1024, which overflows to
  +infinity (7ff0000000000000), the pattern every further square keeps.
  Downward: 1/2, 1/4, ..., 2^-512, then 2^-1024, the square of 2^-512,
  whose exponent -1 runs into the sign: fff0000000000000, the pattern of
  -infinity, which the arithmetic reads as -2^1024. Its square is
  +infinity, as are all the squares above it.
- So n = 0 makes no product and returns x. A zero x stays a zero, its
  sign the XOR with each multiplier's. For n from -1023 to 1023 the
  exponent field moves by n, an infinity of x's sign past 2046, and a
  result below 1 is packed with the exponent run into the sign (a
  product's underflow), after which the pattern is a negative number and
  the later products carry that sign on: that is why a negative x with a
  negative result exponent keeps its sign where a positive one gains it.
- The bit of 1024 in a negative n multiplies by -2^1024: the sign flips
  (a zero becomes the other zero) and the exponent goes up by 1024,
  overflowing to an infinity from a field of 1023. The bits of 2048 and
  above multiply by +2^1024, which adds 1024 without a sign change. This
  is what the spec's earlier rules, in bands of 1024, described: for n
  from -2047 to -1024 the low bits take the exponent down by |n| - 1024
  and the bit of 1024 flips the sign and adds 1024 back; where the low
  bits had taken the exponent below 1, their underflow had made the
  pattern negative, and -2^1024 turns it positive and overflows (the
  "+infinity whatever the sign"); for n from -2048 to -3071 the bit of
  2048 adds 1024 to that negative pattern instead, -infinity.
- 2^-1022 with n = -1075 gives -0: |n| = 1075 has the bit of 1, so the
  first product is 2^-1022 times 1/2, whose exponent 0 and fraction 0
  pack to +0, a zero in this arithmetic; the later products are of that
  zero, and the one with -2^1024 flips its sign. With a nonzero fraction
  the same input gives +infinity: the pattern with a field of 0 is a
  number, 1.f times 2^-1023, and the products go on.
- exp(-1e6) is ldexp with n = -1442695 = 2^20 + 2^18 + 2^17 + 2^9 + 2^8 +
  2^7 + 7, which has no bit of 1024, so no sign flip: the low bits take
  the sum's exponent down to about 119, the first +2^1024 brings it to
  1143 and the second overflows to +infinity; -92332570, exp's n for
  -6.4e7, has no bit of 1024 either.
- How it was found: the main probe's grid showed bands of 1024 in n, a
  zero's sign flipping in the second band and staying in the third, and
  -0 for 2^-1022 with -1075 where the same power on a pattern with a
  nonzero fraction gave +infinity. A sign that flips once per band and a
  zero that appears only when the fraction is zero point at products with
  the -infinity pattern and at a product that packs to a zero, and
  squaring 1/2 ten times gives that pattern. Scored on the host against
  the main probe's 2330 returning ldexp lines and its exp, sinh, cosh and
  frexp lines, the rule matches them all; the power accumulated apart
  from x and multiplied in at the end (pow's own p) misses 420 ldexp
  lines and the -0, divisions by the powers of 2 miss 1065, and the bits
  taken from the top miss 622. The mathldexp probe's 5070 lines are the
  same on both libraries, and the main probe's 2424 ldexp lines (94 of
  them the skipped power), where before this 2 differed.
- n = -2147483648 never returns on Sony's library: its negation is
  itself, and a loop over the bits of a negative number shifted right
  never ends. The clean library takes the magnitude unsigned and returns
  (the bit of 2^31 multiplies by +infinity like the bits from 2048 up).

## sqrt

The Seventh Edition Unix sqrt: 0 for x <= 0 (so -0 too); the mantissa
and exponent taken from the bits (mantissa from a half to one, exponent
field less 1022, not frexp's, so the top of the range works:
sqrt(max) = sqrt(inf) = 2^512); an odd exponent doubles the mantissa;
guess 0.5(1 + mantissa), scaled by 2^30 per 60 of exponent and then by
the integer 2^(e/2) (through __floatsidf); five Newton steps
temp = 0.5(temp + x/temp). Matches every probed input.

## The continued fractions: tan and atan

The circular functions rest on two continued fractions, each cut at a
fixed depth and evaluated from the innermost term out, every step a
division of the library's:

- tan f = f / D, D = 1 - z/(3 - z/(5 - ... - z/(17 - z/19))), z = f*f:
  ten terms (the constants 1, 3, ..., 19), d starting at 19 and then
  d = (2k-1) - z/d for k = 9 down to 1, and f/d last. Its garbage for
  the smallest arguments is this exactly (tan(2^-1022) = x/4.0027, D at
  z = -16 as the arithmetic's underflow makes it).
- atan x = x / D, D = 1 + x²/(3 + 4x²/(5 + 9x²/(7 + ... + 361x²/39))):
  nineteen terms, d starting at 39 and then d = (2k-1) + k*k*x*x/d for
  k = 19 down to 1, then x/d. The numerator is taken as C does, left to
  right: k*k (an integer converted) times x, then times x again, and that
  over d; x*x is not formed once. The order matters twice over. In the
  ordinary range the two products of the left to right form round
  differently from k*k times a square (asin(0.6), through atan(0.75 -
  2^-53), is the probed case: an ulp). For a denormal x the squares
  underflow, and what they pack into the sign depends on which product
  underflows: (k*k*x)*x underflows only where k*k*x*x is below 2^-1023,
  while (k*k)*(x*x) carries one underflowed square through every term.
  The mathatan probe scored four orderings against Sony's atan on the
  tiny grid and the open cases on Sony's own helpers: the left to right
  form matches all 208, the others 191 to 198.

## atan, atan2

- atan(x), x ≥ 0: the fraction above for x ≤ 1, and pi/2 - atan(1/x)
  for x > 1 (so atan(1) is the fraction's end, 5 ulps short of pi/4, and
  atan(1+2^-52) is pi/4 + 7: the depth is what makes atan worst there,
  within an ulp elsewhere); atan(+inf) = atan(NaN) = pi/2 exactly. A
  negative x is taken as |x| and the result negated. Matches all 554
  probed inputs and all 480 tiny ones, with the fraction's numerator in
  its left to right order and the product's form above; before those two
  were found it matched 551 and 460, the rest garbage from the squaring.
  What the garbage is: where the result is a denormal it is an ulp below
  the quotient rounded to nearest (0280000000000000 gives ...3e6b, the
  nearest is ...3e6c), which is __divdf3's own denormal rule (the half a
  32-bit shift, so truncation from a shortfall of 32: see "Products and
  quotients"). For a denormal x (read as 1.f times 2^-1023) k*k*x is a
  small normal number and times x underflows and packs into the sign, a
  moderate garbage number about -4k² for each k, and the result x/d is a
  denormal from the quotient; here the products lie within a few units
  of a half ulp for arguments like 0003ffffffffff80, and the product's
  dropped words decide the last bit (the asin and atan2 cases). For x =
  2^-513 and the mantissas 1.0, 1.14 and 1.25, atan(x) = 4x/9 exactly,
  because the fraction collapses to its last two terms: 9x² and above are
  at least 2^-1023 and read as tiny positive numbers, so the terms for k
  from 19 down to 3 leave d at 5 and a little; 4x² is 2^-1024 or a bit
  more, which packs with the exponent -1 into fff... and reads as about
  -2^1024, so d for k = 2 is -2^1024/5 or so; and x² itself, 2^-1026 or
  a bit more, packs with the exponent -3 and reads as about -2^1022, a
  quarter of the k = 2 numerator in the same garbage, so the k = 1 term
  is exactly 5/4 and D = 9/4. That takes x² in [2^-1026, 2^-1025), the
  mantissas below 1.414: for 1.5 and 2-ε the k = 2 numerator no longer
  underflows (9x²/4 is above 2^-1023), d for k = 2 is 3 and a little,
  the k = 1 term is a huge negative number and x/d is -0, as it is for
  every other exponent, where the garbage terms pair off differently.
- atan2(y, x) = asin(y / hypot(x, y)), and for x < 0 that taken from pi
  (y ≥ 0) or from -pi (y < 0); atan2(0, 0) = 0, atan2(±0, x > 0) = ±0,
  atan2(±0, x < 0) = +pi, atan2(y, ±0) = ±pi/2. atan2(1, 1) =
  3fe921fb51989db5, 5e-9 short of pi/4, is exactly what hypot's single
  precision makes of it. Matches all 985 probed inputs and all 35 tiny
  ones; before the fraction's order and the product's form were found it
  matched 947 and 32, the rest atan's garbage for a denormal y/h: for an
  infinite or huge x, and for y = ±1 or ±2 with x = 1e300 or the largest
  double, where y/h is about 1e-300 or less.

## acos and asin

- asin(x) = atan(x / sqrt(1 - x*x)) with this atan and sqrt; 0 outside
  [-1, 1] and for NaN. All 554 probed inputs and all 440 tiny ones. The
  548 and 386 it matched before were atan's: for a tiny x the square is
  garbage of about -4 and up, 1 - x*x is a moderate number, and x over
  its root is a denormal quotient, which atan's fraction turns into the
  products within a few units of a half; and asin(0.6) hands atan 0.75 -
  2^-53, where the k = 16 numerator 256*x*x is 144 less 1.5 ulp plus
  2^-98, which Sony's product rounds down.
- acos(x) = pi/2 - asin(x) exactly, in the library's arithmetic, and 0
  when x > 1 or x < -1 (NaN reads as greater than 1, so 0). All 554 (the
  one it missed before was asin(0.6)).

## sin, cos, tan

- sin reduces the signed x by the nearest multiple of pi in Cody and
  Waite's two parts: f = (x - N*3.1416015625) -
  N*(-8.908910206761537356617e-6), N = (int)(x/pi ± 0.5), the half
  taken toward the sign of x as tan does it, a quotient (a product by
  1/pi would underflow for the smallest x; the quotient's denormal makes
  N = 0). An odd N multiplies the result by -1 (a product, not a sign
  flip: sin(pi) = 3ca1a62000000000 exactly that reduction). No sign is
  taken off or put back: the result's sign is the chain's own, so a zero
  result has the sign of x (sin(-2^-963) = -0: f/2 is negative and the
  quotient f/d underflows to a zero of its sign). For x below -pi*2^31
  N saturates to -2^31 and its conversion back never returns; for a
  positive x of that size N saturates to 2^31 - 1 and sin returns
  garbage, an asymmetry a reduction on |x| could not have.
- The reduction on x and one on |x| with the sign put back give
  different bits now and then: the sum (x - N*C1) - N*C2 lines N*C2 up
  by a shift that drops its low bits, and the library's sum rounds a
  negative addend's dropped bits up and a positive one's down, so for
  some x the rounded f of -x is not -f of x. The mathdenorm probe called
  sin on sixteen such pairs near 303 (chosen on the host, where the two
  models could be run side by side): Sony's sin(-x) is the signed
  reduction's on all sixteen and differs from -sin(x) by one or two
  ulps. The main and tiny probes' 1029 sin lines happen to hold no such
  pair, which is why the spec had sin working on |x| until then.
- Then sin f = 2t / (1 + t*t) with t = tan(f/2) by the continued
  fraction, f/2 a quotient. The fraction gives +0 for a zero argument of
  either sign (its own check, before anything else; the reductions of
  sin and tan return a zero argument unchanged, so sin(±0) = tan(±0) =
  +0). That is the one case where a nonzero argument reaches the zero
  case: sin(±(2^-1021 - ulp)), the field-1 pattern with every fraction
  bit set, has N = 0 and f/2 = 0 by __divdf3's lost carry (the quotient's
  denormal rule: y all ones with a shortfall of 1 rounds into the hidden
  bit, which is cleared), so sin is +0 for both signs where every other
  tiny argument gives a zero of its own sign. Matches all 554 probed
  inputs of the first probe and all 500 of the second's.
- cos(x) = sin(pi/2 - |x|), pi/2 the double 3ff921fb54442d18: cos(pi/2)
  = 0, cos(0) = sin(pi/2) = 1, cos(pi) = -1 exactly, cos(tiny) = 1
  exactly, and near pi/2 the result is off by the constant's own error
  (8 ulps at 1.529). Matches all 502 probed inputs. It never returns for
  |x| past pi*2^31.
- Large arguments give small garbage: sin(x) is about 220/x from about
  1e15 up; sin(max) = sin(inf) = 3fb297f5dba870ea (the reduction's
  failure, which the forms above reproduce).
- tan(x) reduces by pi/2, not pi, in Cody and Waite's two parts: N =
  (int)(x/(pi/2) ± 0.5) (a quotient by the double pi/2, rounded away
  from zero on the signed x), f = (x - N*1.57080078125) -
  N*(-4.454455103380768678308e-6); then t = f/D by the fraction above,
  and for an odd N the result is -1/t (the quotient -1 over f/D, not
  -D/f: the two differ in the last bit). So tan(pi/2) =
  434d02a0aceda9c7, the reciprocal of the double pi/2's distance from
  pi/2, tan(pi/4) = 3feffffffffffffe (N = 1, the reciprocal of
  tan(-pi/4)), tan(1e10) = 1.2e8 (N saturated at 2^31-1, odd, so the
  reciprocal of the fraction's garbage -55/f), and tan(-0) = +0 (the
  reduction returns a zero unchanged and the fraction gives +0 for
  either zero; f/d alone would give -0). Matches all 554 probed inputs
  and 440 of 440 tiny ones
  (2.9e-297, whose denormal result was an ulp off, matches since
  __divdf3's denormal rule was found). It never returns for x below
  -pi*2^31.

## exp

- exp(x) = 2^N * e^g, N = (int)(x/ln2 ± 0.5), the quotient by ln2 and
  not a product by 1/ln2 (rounded away from zero on the signed x, so N
  saturates to -2^31 below about -1.49e9 and the conversion back never
  returns), g = x - N*ln2 with ln2 the one double 3fe62e42fefa39ef (so
  errors grow with |x|, about 250 ulps at -512), and e^g the Taylor
  series 1 + g + g²/2! + ..., summed term by
  term: sum = 1 + g, t = g, then for k = 2, 3, ... t = t*(g/k) and sum +=
  t until |t| < eps, that term dropped. Any eps from 1e-20 to 1.2e-16
  gives Sony's results on every probed input (as does stopping when sum
  + t == sum); the clean library uses 1e-17. The series is summed for
  |g| and inverted (1/sum) for a negative g, so that exp(-x) is exactly
  1/exp(x) (18 of 18 probed pairs). The result is ldexp(sum, N) by the
  ldexp rules (exp(-768) = fab01d1619c0439c, the exponent run into the
  sign; exp(-709) = 00017fcabbc0470b, a field of 0).
- exp of a tiny argument is garbage from the series' first product:
  exp(2^-1022) = 1 + x + (x/2)*x = -11 exactly, the quotient x/2 a
  denormal that reads back as 1.5*2^-1023 and the product underflowing to
  -12; exp(1e-300) = 1 + x*x/2 with x*x the garbage -3.2e16.
- The quotient and the product by 1/ln2 round to the same N on every
  input of the first two probes (none lies near a tie), and the spec had
  the product. They differ where x is within a few ulps of (k + 1/2) ln2,
  which is exactly where pow(2, k + 1/2) lands (y * log 2 is y ln2
  rounded): the mathpow probe calls exp on 240 such arguments, and Sony's
  exp takes the quotient's N on all 240, the product's on 200 (and with
  1/ln2 an ulp either way on 161 and 79). Which side of the tie N falls
  decides whether the series is summed for +g and the result scaled by
  2^k, or for -g, inverted and scaled by 2^(k+1), and the two round a few
  ulps apart: pow(2, 1.5) = 4006a09e667f3bca is 3 ulps below sqrt 8 for
  that reason.
- Matches all 554 probed inputs and the 420 of the tiny probe. The one it
  missed before, exp(-1e6), is ldexp with n = -1442695, which the ldexp
  rule above takes to +infinity (no bit of 1024 in n, so no sign flip);
  the piecewise rules the spec had before gave -infinity there.

## sinh, cosh, tanh

- sinh(x) = x*(1 + z/6), z = x*x, for |x| < 1e-3 (the threshold is
  between 0.00098 and 0.00105; the series stops there, so sinh is up to
  1e-14 off relative just below it), else (e - 1/e)/2 with e = exp(x),
  the quotients and the halving divisions. All 554 probed inputs; the two
  it missed before, -1e6 and -6.4e7, were exp's ldexp case: exp comes
  back +infinity (2^1024 to the arithmetic), 1/e is a denormal the
  difference drops, and the half is 2^1023.
- cosh(x) = (e + 1/e)/2 for every x. All 554 (the same case).
- tanh(x) = x*(1 - z/3) for |x| < 1e-3, else 2/(1 + exp(-2x)) - 1, in
  that order (its cancellation makes tanh about 1e-16 absolute, so
  hundreds of ulps out near 1e-3). All 554 probed inputs. The hangs
  (|x| past about 7e8, NaN) are exp's, reached from either sign through
  -2x.

## pow

- pow(0, y > 0) = 0, pow(0, y <= 0) = 1, pow(x, 0) = 1; pow(x, -0) never
  returns, nor many calls with an infinite, huge or NaN power or base
  (the probe lists those it found).
- A power that converts to an integer n (toInt(y) == y) is done by
  repeated squaring: p = 1, b = x, then for each bit of |n| from the
  lowest, p *= b if the bit is set, b *= b; for a negative n the base is
  1/x first. So pow(3, 2) = 9 and pow(10, -2) = 0.1*0.1 = 3f847ae147ae147c
  exactly, and pow(inf, -2) = (1/inf)², the arithmetic's garbage -6.25.
  All 408 probed integer powers match.
- Otherwise pow(x, y) = exp(y * log(x)) through the library's own log,
  product and exp, and 0 for a negative base. The mathpow probe prints
  pow(x, y) beside exp(y * log(x)) worked out on those three for the 13
  half-integer powers of 2 and 1/2 that were open, three of the NaN and
  2^-1074 cases and seven controls, and the two agree on every line, so
  pow has no path of its own: its misses were exp's N at a tie
  (the 14 half-integer powers of 2 and 1/2, pow(2, 1.5) among them) and
  log's two odd splits (the 13 with a NaN or 2^-1074 base). All 1205
  probed inputs match, and the 35 of the tiny probe.

## hypot

Moler and Morrison's iteration in C floats: p = max(|x|, |y|), q = min,
then three times over t = q/p, r = t*t, s = r/(4+r), p += 2*p*s, q = s*q,
with t, r and s single precision (the quotient q/p in double and stored
through __truncdfsf2, t*t, 4+r and r/(4+r) by the single helpers, s
widened for the two products with p and q, which stay double) and the
result p. The step's correction is 2*p*s in that order: p is doubled
first, then multiplied by s (not 2*s times p, nor 2 times s*p, which
give the same bits as each other but not as this: see the smallest
denormals below). The swap of p and q goes through a float too: when
|x| < |y|, q is |x| rounded to single, so hypot(1/3, 1) =
3ff0dd9028972299 but hypot(1, 1/3) = 3ff0dd9028969bad. Three
iterations converge for every
ratio in double (the third step's correction is below the last place),
and the single precision of s is what makes the result 5e-9 off relative
at a ratio of 1 (hypot(1, 1) = 3ff6a09e68629561), 4e-10 at 1/2, 2e-11
at 1/4, falling as the ratio's fourth power: the first step's s = r/(4+r)
is rounded to 24 bits, and the error can only grow from there.

The count and the precision show in the garbage for small ratios: t is a
single denormal below about 1e-38 and the single product t*t packs its
exponent into the sign, so r is about -4.08, 4+r is a small negative
number and s is 50 to 400; hypot(1, 1e-300) = 7494312.1 is three such
steps (98 times 196 times 391). For ratios between about 1e-19 and 1e-38
the second or third step does this and p triples (s = 1) or more, so
hypot(1024, 1) = 3*1024.0005 and hypot(1e5, 1) = 9*1e5. The garbage is
exactly the single helpers' as "Software floating point" gives them: the
ratios whose shortfall is 31 modulo 32 come back from __truncdfsf2 as
the 8xxxxxxx numbers (hypot(6.2e79, 7.4e-96) = 1.39e87, hypot(max, 8) =
+inf), and the product t*t of a denormal t is a near tie about one time
in sixty, rounded down by the byte-wise product where half up would go
up (hypot(1, 1e-300) was 7494270.7 with half up: s is r/(4+r) with 4+r
a cancellation, so an ulp of r is 2^-18 of the result).

Matches all 985 probed inputs, all 35 tiny ones and the 43 of the
mathdenorm probe. The one it missed before, hypot(2^-1074, 2^-1074),
both arguments the smallest denormal, is where the order of the step
shows: the arithmetic reads the arguments as 2^-1023, so p = q, t = 1
and s = 1/5 exactly, and the first step's correction, 2p (2^-1022)
times 0.2, underflows and packs its exponent into the sign, -1.6 times
2^1023
(ffe99999a0000002), which the sum with p returns as it is; the same by
every order. In the second step p is that garbage and 2p overflows to the
infinity pattern, fff0000000000000, which reads as -2^1024, so the
correction is s times -2^1024 whatever p's fraction: p becomes -(1.6 +
2/17) times 2^1023, and the third step adds 2*s3 the same way, giving
Sony's ffeb7d04f8400002. The forms that multiply s (or 2s) by p first
keep p's fraction in the correction and give ffec9f25cd1aa08c. How it
was found: the mathdenorm probe worked the step on Sony's own helpers
for this pair and its neighbors, and every intermediate value, and the
final value of six orderings of the step (p + (2s)p, p + p(2s), p +
2(sp), (p + sp) + sp, p(1 + 2s), (2s)p + p), was the same on both
libraries while hypot itself differed, so the helpers were not it; and
Sony's results for the pairs (f, f) with f from 1 to 2^40 are
ffeb7d04f8400000 plus 1.6f, the first step's garbage with a correction
that does not depend on p's fraction, which only a product that drops
the fraction, the doubled p overflowed, gives. Scored on the host, 2*p*s
fits all 1063 of Sony's hypot lines; (2s)*p fits 1043, the twenty
misses all pairs of denormals below 2^-1022.

## log, log10

- log(x) is 0 for x < 0 and for x == 0 (the manual says 1 for zero:
  Sony's library gives 0); NaN and the infinities read as numbers.
- The mantissa f and exponent n come from the bits as sqrt's do (so
  log(max) = log(inf) = 1024 ln 2); f below sqrt(1/2) is doubled and n
  lowered, so f is in [0.707, 1.414); then log f is the series 2(z +
  z^3/3 + z^5/5 + ...) in z = (f-1)/(f+1), summed term by term (t = t*w,
  w = z*z, the term t/k added) until a term no longer changes the sum
  (an absolute cut-off of 1e-30 and a relative one of 1e-17 give the
  same results on every probed input; 1e-17 absolute drops a term of
  log(0.9999999) that Sony keeps), and the result is n*ln2 + 2*sum with
  ln2 one constant. Cody and Waite's rational form matched 521 of 554
  and the Seventh Edition's 517, both an ulp or two out wherever |n| is
  small.
- The test is strict: f = sqrt(1/2) exactly (the double
  3fe6a09e667f3bcd) is not doubled. The mathpow grid has that mantissa
  at 15 exponent fields, and where the two splits round apart (fields 2,
  1021 to 1025, 2000 and 2045) Sony's is n ln2 + 2 sum(0.7071) and not
  (n-1) ln2 + 2 sum(1.4142); one bit below it is doubled.
- At the two ends of the exponent field the split goes its own way,
  found by working n ln2 + 2 sum(f) on Sony's helpers for the pairs
  (n, f), (n-1, 2f), (n, 2f) and (n+1, f) against each grid line. At a
  field of 0 (the denormal patterns, read as 1.f times 2^-1023) the
  mantissa is never doubled: n = -1022 and f = 0.5(1.f) whatever f, so
  log(2^-1074) = -1022 ln2 + 2 sum(0.5 + 2^-53), an ulp above -1023 ln2
  + 2^-52, which the ordinary split gives (six grid lines with the
  smallest fractions and the one just below sqrt(1/2) tell the two
  apart; the others round the same). At a field of 2047 (infinity and
  the NaN patterns) the mantissa is always doubled and only n follows
  the test: f below sqrt(1/2) gives (1024, 2f), the ordinary split, and
  f from sqrt(1/2) up gives (1025, 2f), which reads the pattern as twice
  the number the arithmetic does. So log(inf) = 1024 ln2 and log(1.25 *
  2^1024) = 1024 ln2 + log 1.25, but log(7ff8000000000000) =
  4086370cf4379070 = 1025 ln2 + log 1.5, as if of 3 * 2^1024 (and
  pow(NaN, 0.5) is sqrt 3 times 2^512). The fields 1 and 2046 follow the
  ordinary split. What code does this was not worked out: the fields are
  the ones IEEE arithmetic treats specially, so Sony's log tells them
  apart where its arithmetic does not; the grid (fields 0 to 3, 100,
  1021 to 1025, 2000, 2044 to 2047, 28 mantissas each) and the pairs
  pin the behavior.
- With the strict test and the two end rules, matches all 554 probed
  inputs, all 270 tiny ones and all 420 of the mathpow grid.
- log10 = log/ln10 with the same zero and negative rules (log10(0) = 0,
  not 1). All 554.

## strtod, atof

- No white space and no sign are taken: "-1", "+1", "  12.5", "-.5",
  "inf" and "nan" give 0 with nothing consumed.
- Digits, then a point and digits, then e or E, which is taken even
  with no digits after it ("1e" ends after the e, "e5" is 0 ending after
  the 5), then an optional sign and digits. "0x10" stops at the x. The
  end pointer matches Sony's on every probed string.
- The digits, before and after the point alike, make the value ten at a
  time in the library's arithmetic (v = v*10 + d, however many there are:
  a 55-digit string accumulates them all), and the exponent less the
  count of digits after the point, s, scales it by binary exponentiation
  applied to the value itself: p starts at 10 and is squared at each step
  (p = p*p in the library's arithmetic), and for each set bit of |s| from
  the lowest, v is multiplied by p, or divided by p when s is negative.
  So "1e23" is 1e22 times 10 rounded half up (44b52d02c7e14af7, an ulp
  above the exact double), and the powers 1e64 and 1e128 are (1e32)²
  and (1e64)², each an ulp above the exact double (4d384f03e93ff9f6,
  5a827748f9301d33), which is what put every string with a scale of 64
  or more an ulp or two off the table of exact powers: with those two
  constants all 311 probed strings match, and the 51 of the mathsingle
  probe.
- The written exponent accumulates in a 32-bit integer and wraps
  ("1e4294967297" is 10, "1e2147483648" is 0), then is negated for a
  minus sign ("1e-4294967295" is 10). A scale past 255 gives an infinity
  whatever the digits ("0e256" too, and "1e256"), one below -255 zero
  ("1e-256"); "1e255" and "1e-255" are finite. The limit is on the scale,
  not the written exponent: "0.00000001e260" is 1e252 and
  "0.0000000001e-250" is 0.
- The spec's earlier model, the same per-bit scaling by a table of the
  exact powers 10^(2^k), matched 267 of 311: every string whose scale
  had no bit above 32, and none of the rest fit any one rule because the
  rule was in the constants.
- atof(s) = strtod(s, NULL). Neither touches _get_errno.

## _get_errno

A single kernel call, B0h 54h, its return passed through. Under the
harness's stand-in every probe line's errno matches: neither the math
calls nor strtod make that call or change what it returns, and after
each file call (spec 11's, on a blank card formatted by the probe, a
missing file, a full card, a bad descriptor, an unknown device) the
call is the same single kernel call. What the kernel would answer is
the kernel's: OpenBIOS has no B0h 54h, so no real errno value was seen.

## Calls that never return

On Sony's library these never return (the probe skips them): sin and
tan below -pi*2^31; cos past pi*2^31 either sign and NaN; exp, sinh and
cosh below about -1.4e9 and negative NaN; tanh past about 7e8 either
sign and NaN; pow in the cases listed in the probe's table; ldexp with
n = -2147483648 (the loop over the bits of -n, which is n itself, never
ends: see "ldexp"); __floatsidf(-2147483648), which is the cause of most
of the others (an integer that saturated being turned back into a
double). The clean library's __floatsidf never returns there either; its
own conversions inside the math calls do not hang, and its ldexp returns
for every n.

## Open

- What log's code does at the exponent fields 0 and 2047 (see "log,
  log10"): the behavior is pinned by the grid and reproduced, the code
  behind it is not known. The arithmetic forms tried for the split (the
  mantissa by a product or quotient with a power of two built from the
  bits or by ldexp, the test on a scaled x or against a scaled
  sqrt(1/2)) give either the ordinary split or garbage at those fields,
  so the fields are told apart on purpose.
- __muldf3's form is pinned where the probes look: the 2764 probed
  products, 468 of them within a chosen distance of a half ulp, decide
  between the forms tried (exact; 32-bit halves after a shift of 10 or
  11, with the low halves' product dropped and the cross products' low
  words kept or dropped; bytes or 16-bit pieces with everything below a
  cut dropped), and one fits them all. A form that drops the same bits
  some other way would fit as well.
- __mulsf3's byte-wise product is pinned only where the probes look:
  which of the two lowest byte products is truncated and which dropped
  cannot be told apart on any probed product (the variants agree on all
  2701), and the chosen form, a0*b0 dropped and the two of weight 2^8
  without their low byte, is the one that also gives the squares hypot
  takes. __mulsf3 above the single range: products that overflow give an
  infinity on every probed input, and nothing was probed between.
- Why Sony's sqrt never returned in a long run of random calls without
  printing between (see the probe).
