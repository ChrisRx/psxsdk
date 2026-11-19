# Milestone 17: formatted output and the math error entries

Status: spec only. Original C programs built for Sony's library call these entries
at fixed addresses and the clean image lacks them: `printf`, `sprintf`, `printf2`,
`sprintf2`, the C forms of `FntPrint` and `KanjiFntPrint`, `_err_math`, `_dbl_shift`
and the variable `math_errno`. Everything here was observed by calling Sony's entries
in the emulator. Where a rule below is a procedure, it is a rule that reproduces the
results, not a statement of how the original is written.

There are three formatters with different behavior, not one, and code cannot be
shared between them without keeping the differences:

| entry | formatter | output | floating point |
|---|---|---|---|
| `sprintf` | A | memory | no |
| `printf` | B | TTY | no |
| `printf2` | B | TTY | yes |
| `sprintf2` | B | memory | yes |
| `FntPrint`, `KanjiFntPrint` | C | a print stream | no |

Every line of the grids in "Acceptance" was scored on the host against these rules
and all of them match, apart from what "Open" lists.

## Reading the arguments

All the entries take o32 variable arguments: words starting in a0, a1, a2, a3 and
going on up the caller's stack. Spec 15's `libps_va_NAME` export gets them as an array
of words, `args[0]` being a0. Each integer, character, pointer or `*` takes one word.
`l`, `L`, `ll` and `h` never take a second word: `%lld` reads one word, and the next
conversion reads the word after it.

A double (formatter B's e, E, f, g, G) takes two words at the next even index of
`args`, low word first, skipping one if the index is odd. So `sprintf2(s, "%d|%f", 7,
1.5)` reads 7 at index 2, skips 3 and takes the double at 4 and 5, and `sprintf2(s,
"%d|%d|%f", 7, 8, 1.5)` takes the double at 4 and 5 with nothing skipped. `printf2(f,
1.5)` takes it at 2 and 3 (probes `cprintf`, `cfmtdbl` group args).

## Formatter A: sprintf

`long sprintf(char *s, const char *fmt, ...)`. Probe `cfmt`, family 0: 14035 lines.

The result is the bytes made, then a NUL. It returns how many bytes were made, the
NUL not counted, and literal text counts. Nothing past the NUL is written. There is no
internal limit: `%10000d`, `%.8000d` and a 10000 character `%s` come out whole
(`cfmtlong`).

Any byte other than `%` is copied. A directive is read in this strict order:

1. Flags, any number of `-`, `+`, space, `#`, `0`, repeats allowed.
2. Width: `*` (one word, and if negative its magnitude with `-` set) or decimal
   digits, accumulated in 32 bits with wrap (`%4294967301d` has a width of 5).
3. Precision: `.` then `*` (one word, negative meaning no precision given) or decimal
   digits. `.` with neither is 0.
4. Length: any run of `h`, `l`, `L`. An `h` anywhere in the run means 16 bits (`%lhd`,
   `%hld` and `%hLd` all do). `l` and `L` alone change nothing. `q`, `j`, `z`, `t` are
   not length letters.
5. The conversion letter.

Anything out of that order stops the format: a letter not in the table below, or the
format's end anywhere inside a directive, ends the result there. The NUL is written
and the bytes so far are returned, and the arguments and the rest of the format are
not looked at. `a%yb` gives "a", 1. `%` gives "", 0. `%5-d`, `%.-3d`, `%**d`, `%*5d`,
`%.5.3d`, `%l5d` and `%qd` give "", 0. `%0-5d`, `%-05d`, `%--5d`, `%005d` are fine,
being flags.

| letter | takes | text |
|---|---|---|
| `d`, `i` | a word | signed decimal; with `h`, the low 16 bits as signed |
| `u` | a word | unsigned decimal; with `h`, the low 16 bits |
| `o`, `x`, `X` | a word | octal, lower case hex, upper case hex; with `h`, the low 16 bits |
| `p` | a word | see below |
| `c` | a word | the low byte, a NUL included (`%c` of 141h is "A") |
| `s` | a word | see below |
| `n` | a word | see below |
| `%` | nothing | one `%`; flags, width and precision ignored (`%-5%|` is "%\|") |

For `d`, `i`, `u`, `o`, `x`, `X`:

- The digits are the value's, at least the precision's count, zero filled. With a
  precision of 0 a value of 0 has no digits. `+` or space have effect only on `d` and
  `i`: a `-` for a negative value, else `+` if flagged, else a space if flagged.
- `#` on `x` puts "0x" and on `X` "0X" before the digits, whatever the value: `%#x` of
  0 is "0x0", and `%#.0x` of 0 is "0x". `#` on `o` puts a `0` first unless there are no
  digits or the first already is one: `%#o` of 8 is "010", `%#.0o` of 0 is "".
- If the total is under the width: `-` fills spaces on the right; else, with `0` set
  and no precision given, zeros between the sign or prefix and the digits (`%+05d` of 7
  is "+0007", `%#06x` of 2Ah is "0x002a"); else spaces on the left. A precision of any
  kind, even `.0`, turns `0` off: `%06.4d` of 42 is "  0042".

`%p`: eight upper case hex digits, always. The precision and `0` have no effect
(`%012p` of BEEFh is "    0000BEEF", `%.0p` of 0 is "00000000"). `h` keeps the low 16
bits. `#` puts "0p" first (`%#p` of BEEFh is "0p0000BEEF"). The width and `-` work as
above, with spaces.

`%s`, with a pointer p:

- Without `#`: the bytes up to the NUL, at most the precision if given. Spaces fill to
  the width, left unless `-`. `0` has no effect here.
- With p null and no `#`: no precision gives no bytes. A precision gives exactly that
  many bytes copied from address 0, NUL or not. Probe `cfmtlong`, group low memory,
  with "\x03BCD..." at address 0: `%s` gives "", `%.4s` gives "\x03BCD", `%8.4s` gives
  "    \x03BCD". The usual low memory holds zeros, so this reads as NUL bytes
  (`%.4s` of null is 4 NULs on `cfmt`).
- With `#`: a counted string. The byte at p is the length L and the text is the L bytes
  after it, NUL or not, at most the precision if given. Null is not special: it reads
  address 0 the same way (with the above, `%#s` of null gives "BCD"). `%#s` of "hello"
  has L 68h and so copies 104 bytes past the 'h'.

`%n` stores the number of bytes made so far, literal text included, and makes no text.
It stores a word, or the low halfword only with `h` (`abc%hn` over 22222222h leaves
22220003h). Flags, width and precision are ignored. `abc%nxyz` returns 6 and stores 3.

e, E, f, g, G are letters outside the table, so they end the format: `%f` gives "", 0.
sprintf's own documentation says the same of them, though not that the format stops.

A null `fmt` reads address 0 as the format, and a null `s` writes at address 0 (a null
format on the usual low memory gives "", 0, and neither faults: `cfmtlong`).

## Formatter B: printf, printf2, sprintf2

`long printf(const char *fmt, ...)`, `long printf2(const char *fmt, ...)`, `long
sprintf2(char *s, const char *fmt, ...)`. Probes `cfmt` families 1, 2, 3 (13973, 8793
and 8773 lines), `cfmtdbl`, `cprintf`, `cfmtlong`.

### Where the bytes go

- `sprintf2` writes each byte at s, in order. The terminator is written only if at
  least one byte was made: any literal or conversion byte, then a NUL. With none (`""`,
  `%`, `%s` of "", `%.0d` of 0, `%n`), **nothing at all is written**, not even the NUL.
  There is no other limit: `%10000d` and 5000-character strings come out whole.
- `printf` and `printf2` send each byte through the library's putchar (spec 01): one
  kernel call B0h 35h, write(1, &c, 1), per byte, and a newline as "\r" then "\n" (two
  calls). A NUL from `%c` is written like any byte.

### The return value

**The return value counts only the bytes conversions make, not literal text.** `%%`
and an unknown letter count one each and a conversion its whole field with padding, so
the return is the text's length less the format's literal characters:
`printf("n=%d x=%x s=%s\n", 42, 0xbeef, "str")` prints 18 characters and returns 2 +
4 + 3 = 9. `printf("plain")` returns 0, `printf("a%d", 42)` 2,
`printf("%-5d|", 42)` 5, `sprintf2(s, "%d|%f", 7, 1.5)` 9 with 10 bytes and a NUL
made, `printf("%c", 10)` 1 with two kernel calls. A newline adds no extra. No call in
the probes returned a negative number.

### Directives

A directive is read as a loop, in any order, until a conversion letter:

| seen | effect |
|---|---|
| space | sets space (`+` wins over it) |
| `#` | sets alternate |
| `-` | sets left |
| `+` | sets plus |
| `0` | sets zero |
| `1` to `9` | starts a width: this and the digits after it, in 32 bits with wrap; replaces any width |
| `*` | width from one word, replacing; negative sets left and takes the magnitude |
| `.` | then `*` (one word, negative meaning none) or decimal digits, none being 0; replaces any precision |
| `h` | sets short |
| `l` | sets long |
| `L` | nothing |

So `%5-d`, `%5+d`, `%l5d`, `%h5d`, `%.-3d` (a precision of 0 then left, width 3) all
work, `%.5.3d` has a precision of 3, `%5*d` takes its width from the word, and `%**d`
takes two words and uses the second. `%*5d` has a width of 5 after taking a word. The
format's end inside a directive ends the output: `%`, `%5`, `%l` and `a%` make only
what came before.

Everything that is not a loop character is the conversion letter:

| letter | takes | text |
|---|---|---|
| `d`, `i` | a word | signed decimal |
| `D` | a word | as `d` with long set |
| `u`, `U` | a word | unsigned decimal (`U` as `u` with long set) |
| `o`, `O` | a word | octal (`O` as `o` with long set) |
| `x`, `X` | a word | hex, lower or upper case |
| `p` | a word | as `x` but `#` and short have no effect: `%p` of BEEFh is "beef" |
| `c` | a word | the low byte |
| `s` | a word | the string; a null pointer is "(null)" |
| `n` | a word | see below |
| `e`, `E`, `f`, `g`, `G` | a double | `printf2`, `sprintf2`: below. `printf`: as any other letter |
| `%` | nothing | one `%` with no padding |
| anything else, any byte | nothing | that byte alone, with no padding, flags ignored |

So `%y|%d` with 41h and 2Bh is "y|65", `%-6y` is "y", `%5%` is "%", `printf("%f")` is
"f" and takes no argument (the argument then goes to the next conversion).

Short and long, for the conversions that use them (long wins over short):

| | `d`, `i` | `u`, `o`, `x`, `X` |
|---|---|---|
| neither, or long | the word, signed | the word |
| short, `printf` | the low 16 bits, signed | the low 16 bits sign extended to 32: `%hx` of BEEFh is "ffffbeef", `%hu` of 18000h is "4294934528" |
| short, `printf2`, `sprintf2` | the low 16 bits, **unsigned**: `%hd` of FFFFFFD6h is "65494", of 18000h is "32768" | the low 16 bits: `%hx` of BEEFh is "beef" |

The field, for the numeric conversions:

- The sign is `-` for a negative value, else `+` if plus, else a space if space. Only
  `d` and `i` have one.
- The prefix is "0x" or "0X" for `x`, `X` with alternate and a value that is not 0.
  `%#x` of 0 is "0" (formatter A's is "0x0").
- The digits: a precision of 0 and a value of 0 give none, and alternate on `o` puts a
  `0` first if there are digits and the first is not one. `%#.0o` of 0 is "".
- **The sign and prefix count inside the precision.** With fieldsz the digits plus sign
  plus prefix, the zeros added for a precision p are p - fieldsz if positive: `%+.4d`
  of 42 is "+042" (formatter A gives "+0042"), `%#.6x` of BEEFh is "0xbeef", and
  `%+.1000d` makes 1000 bytes where formatter A makes 1001.
- With realsz the larger of fieldsz and the precision, and width over realsz: left
  fills spaces at the end. Otherwise zero, with no precision given and not left, fills
  zeros between the sign or prefix and the digits. Otherwise spaces go first. A
  precision turns zero off here as in formatter A.

`%c` and `%s`: the bytes (`%s` up to the NUL, at most the precision), filled to the
width. Here too zero, if set and not left, fills with **zeros**: `%06s` of "hello" is
"0hello" and `%06c` of 41h is "00000A". `#` has no effect on them: it is not a counted
string. `%.0s` makes nothing. A null `%s` is "(null)" under every form, cut by the
precision like any string (`%.2s` is "(n", `%8s` is "  (null)").

`%n` stores the return value's count so far, so it leaves out literal text: `abc%n`
stores 0, `abc%d%n` of 42 stores 2. It stores a word, or the low halfword with short
(`%hn` over 22222222h with 0 leaves 22220000h). It makes no text. Width and flags have
no effect.

### Doubles: e, E, f, g, G

Long, `L` and short change nothing. All arithmetic in these rules is the library's own
from spec 12: quotients by `__divdf3`, products by `__muldf3`, sums by `__adddf3`,
`modf` as spec 12 has it, truncation to int by `__fixdfsi`, tests by the comparisons.
That is what the output's look comes from. There is no case for an infinity or a NaN:
they are the numbers spec 12 reads them as, 2^1024 and 1.f times 2^1024. There are no
denormals either: 2^-1074's pattern reads as 2^-1023, so `%e` of it is "1.112537e-308".

The precision: none given is 6. For `g` and `G`, 0 becomes 1. Over 39 it becomes 39
and, except for `g` and `G` without alternate, the excess is kept as z zeros to add at
the end, after everything, **after the exponent too**: `%.100e` of pi is the 39-place
digits, "e+00", then 61 zeros (`cfmtlong`). `%.100g` of pi is 39 significant digits and
nothing else. `%#.100g` of pi has 61 zeros after its digits.

The sign: `-` if `__ltdf2(x, 0)` says x is below 0 (so -0 has none, and fff8000000000000
has one), and then the digits come from -x. Else plus or space as for integers. `%f` of
-0 is "0.000000".

The digits, for magnitude x:

1. `fract, integer = modf(x)`. 0.01 below is the double 3f847ae147ae147b.
2. The integer's digits come out last first, into a list, counted by expcnt: while
   integer is not zero, `tmp, integer = modf(integer / 10)` and the digit is
   `(int)((tmp + 0.01) * 10)`. A large number is a power of two times an odd number, so
   tmp is 0 for all but its top few quotients: only the top 16 or 17 digits vary and
   those below are zeros. That is why max prints "17976931348623167" then zeros, 2^1024
   "179769313486232000...", and 1e23 (44b52d02c7e14af7) "1000...0".
3. Fraction digits come one at a time, `fract, d = modf(fract * 10)`, and stop as soon
   as fract is zero. 0.1 times 10 rounds to exactly 1, so 0.1 is "0.1000...", where
   pi's continue to the bits' end: `%.30e` of pi is
   "3.141592653589793115997963468544e+00" and `%.40f` of pi is
   "3.1415926535897931159979634685441851615910", the 39th place rounded and a zero
   added.

Let `round(fract, ch)` be: `d` is the integer part of `fract * 10` if fract is not
zero, else the digit ch. If d is above 4, add one to the last digit made, going left
with carries over the point. If the carry falls off the left end: for e and E the
leading digit becomes 1 and the exponent goes up one, otherwise a new leading 1 is
added. If d is 4 or less and the sign is `-` and every digit made is 0, the sign goes
away (`%.3f` of -0.0004 is "0.000", `%.0f` of -0.4 is "0", and with plus the sign falls
back to it: `%+.3f` of -0.0004 is "+0.000"). A tie is not special: 0.5 rounds up at any
place, `%.0f` of 0.5 is "1" and of 1.5 "2" and of 2.5 "3", and of -0.5 "-1", so this is
half up on the digits, not half even.

**f**: the integer digits, or "0" if none. A point if the precision is not 0 or
alternate is set. Then if fract is not zero: up to precision fraction digits, stopping
when fract goes to zero, and if fract is still not zero, `round(fract)`. Then zeros up
to the precision. `%.0f` of 1e16 is "10000000000000000", `%#.0f` of 3 is "3.".

**e, E**:

- If there are integer digits: the first, a point if the precision is not 0 or
  alternate is set, then up to precision more of the integer digits. If the precision
  ran out with integer digits left, `round` on the next integer digit, fract now being
  taken as zero. The exponent is expcnt - 1.
- Else if fract is not zero: exponent -1, going down one for each digit of step 3 that
  is 0 until one is not, which is the leading digit; then the point if the same test
  says so.
- Else "0", the point if so, exponent 0.
- Then if fract is not zero: up to the rest of the precision in fraction digits,
  stopping at fract zero, and `round(fract)` if fract is left. Then zeros up to the
  precision.
- Then for g's use of this form only, without alternate: the trailing zeros go, and the
  point if it is then last.
- Then `e` (or `E` for E and G), `+` or `-`, and at least two digits of exponent:
  "e+00", "e-05", "e+308".

**g, G**: the e form with a precision one less, taken if expcnt is above the precision,
or if there are no integer digits, fract is not zero and fract is below 0.0001 (the
double 3f1a36e2eb1c432d, itself not below): so `%g` of 0.0001 is "0.0001" and of 1e-5
"1e-05", `%g` of 999999 is "999999" and of 1e6 "1e+06", `%g` of 999999.5 is "1000000",
rounded after the choice. Otherwise:

- The integer digits, the precision going down one for each, or "0" if none.
- If the precision is now not 0, or alternate is set, a point (and note it: "trimming").
- If fract is not zero and the precision is not 0: make digits until one is not 0, none
  of these counting, then that one and up to precision - 1 more, stopping at fract zero.
  `round(fract)` if fract is left.
- With alternate: zeros up to the precision. Without, if there was a point: the
  trailing zeros go, and the point if it is last.
- `%g` of 3 is "3", `%#g` of 3 is "3.00000", `%#g` of 0.95 is "0.950000",
  `%#g` of 0 is "0.000000", `%.0g` of 0.1 is "0.1", and `%.17g` of pi is
  "3.1415926535897931".

Then the field: the sign, then the digits, then the z zeros, with width and left and
zero as for integers but with no precision to turn zero off: `%012.4f|` of 1.5 is
"0000001.5000|", the sign going before the zeros.

Examples, all from `cfmtdbl`:

| bits | `%e` | `%f` | `%g` | `%.3g` |
|---|---|---|---|---|
| 0 | 0.000000e+00 | 0.000000 | 0 | 0 |
| 8000000000000000 | 0.000000e+00 | 0.000000 | 0 | 0 |
| 3fe0000000000000 | 5.000000e-01 | 0.500000 | 0.5 | 0.5 |
| 3fb999999999999a | 1.000000e-01 | 0.100000 | 0.1 | 0.1 |
| 400921fb54442d18 | 3.141593e+00 | 3.141593 | 3.14159 | 3.14 |
| 7fefffffffffffff | 1.797693e+308 | 17976931348623167 + 292 zeros + .000000 | 1.79769e+308 | 1.8e+308 |
| 7ff0000000000000 | 1.797693e+308 | 179769313486232 + 294 zeros + .000000 | 1.79769e+308 | 1.8e+308 |
| fff0000000000000 | -1.797693e+308 | -179769313486232 + 294 zeros + .000000 | -1.79769e+308 | -1.8e+308 |
| 7ff8000000000000 | 2.696540e+308 | 26965397022934765 + 292 zeros + .000000 | 2.69654e+308 | 2.7e+308 |
| 0000000000000001 | 1.112537e-308 | 0.000000 | 1.11254e-308 | 1.11e-308 |
| 0010000000000000 | 2.225074e-308 | 0.000000 | 2.22507e-308 | 2.23e-308 |

Scored on the host in Sony's arithmetic (the clean `softfloat.go` compiled for the
host, spec 12's modf), the rules above match to the byte on 10,972 lines of `cfmtdbl`
(55 formats by 70 values, 50 for negative zero, 4096 lines of a sweep of every exponent
field at two mantissas, 3000 random doubles at `%.25e`) and the 38 long lines of
`cfmtlong`. So the digits do follow from spec 12's arithmetic. They are not exact
decimal, and any other form of the same steps (the product before the quotient, say)
will miss some.

## Formatter C: FntPrint and KanjiFntPrint

`int FntPrint([int id,] char *format, ...)`, and the same for `KanjiFntPrint`. Spec 01
and spec 13 cover the flush, the stream's limits, and what the text does afterwards.
Probes `cfnt`, `cfntid`, `cfntk`.

### The id

The first word a0 is an id if, as an unsigned number, it is below the number of
streams now open (those FntOpen or KanjiFntOpen made since the last FntLoad or close):
the text goes to stream a0, the format is a1, and its arguments start at a2. Otherwise
a0 is the format, the text goes to stream 0, and its arguments start at a1. That test is
the whole rule. `cfntid` shows it, with low memory holding known text and the streams
opened 1, 2, 4 and 8 at a time:

| open | a0 0 | a0 1 | a0 3 | a0 7 | a0 8 |
|---|---|---|---|---|---|
| 1 | id 0 | format at address 1 | format at 3 | format at 7 | format at 8 |
| 2 | id 0 | id 1 | format at 3 | format at 7 | format at 8 |
| 4 | id 0 | id 1 | id 3 | format at 7 | format at 8 |
| 8 | id 0 | id 1 | id 3 | id 7 | format at 8 |

An id that is not open (a0 of 2 with 2 open) is a format pointer into low memory like
any other, usually an empty string, so the call adds nothing and returns the stream's
count. So -1, FFFFFFF8h and the like are pointers too and are not stream -1. The same
holds for `KanjiFntPrint` (`cfntk`, 2 streams open: a0 0 and 1 are ids, 2 and up go to
stream 0 as pointers, and each print returns that stream's new count).

With no stream open `FntPrint` returns -1 in either form.

### The conversions

Formatter C has very few. After the `%`:

- A `%` straight after it is one `%` (`%5%` is not this: see below).
- Then decimal digits are the width. A leading `0` means zero fill, for `x` and `X`
  only: `%05x` of BEEFh is "0BEEF" and `%08x` "0000BEEF", while `%08d` of 42 is
  "      42", `%08d` of -42 "     -42", `%07s` of "hello" "  HELLO" and `%03c` of 41h
  "  A". There is no `*`.
- Then the letter:

| letter | takes | text |
|---|---|---|
| `d` | a word | signed decimal; -2147483648 is whole |
| `x`, `X` | a word | hex, unsigned, 32 bits |
| `c` | a word | the low byte; a NUL ends the text at the flush (`%c|` of 0 is 2 bytes, draws nothing) |
| `s` | a word | the string; a null pointer is as an unknown letter below |

  each right justified to the width with spaces (or zeros as said).
- There are no flags, no precision and no length letters: `-`, `+`, space, `#`, `.`,
  `*`, `h`, `l`, `L` and every other letter (`i u o p e E f g G n` among them) are
  "unknown letters". An unknown letter takes no word, makes a blank field of the width
  or one space if none, and the format goes on after it. So `%-5d|` of 42 is " 5D|"
  (four bytes), `%+d` is " D", `%.3d` is " 3D", `%5.3d` is "     3D" (seven), `%ld` is
  " D", `%5y|` is "     |", `%i` is " ", `%%` is "%". This is on the `plain` lines of
  `cfnt` where a1 was not 0.
- **Sony's unknown letter does not always make a blank.** It can add stale bytes from
  the stack, a pointer's bytes among them, where the blank would be: `cfnt`'s garbage
  lines (`%i` in the id form, `%f` with a zero first argument). This depends on what
  ran before and is not matched. One space, or the width's spaces, is what to do. Do
  not depend on the argument: nothing takes it.
- The format's own NUL right after a `%` (or after the width) is read as the letter, so
  a blank is made and the format **goes on past the NUL** into whatever follows it:
  `a%` makes A, a space, and then the bytes after the terminator up to the next NUL
  (`A B` in `cfnt`, the stale "b" of an earlier format in its buffer). A clean library
  should do the same, bounded by the next NUL in memory.

The text made goes into the stream as literal bytes do. `~` and `%` in a `%s` are
stream text for the flush (spec 01): `%s` of a string with `~c700` changes the color.
Note that a `%` made by a conversion is just a byte to the flush.

### The return value and the limits

`FntPrint` returns the stream's new count, the formatted bytes being counted. It is
exactly spec 01's byte-by-byte rule with the formatted bytes as the input: on a stream
of 8, `%d` of 123456 returns 6, `%d` again returns -1 leaving "12345612" and the "3" at
text[8] (the first byte of the next stream); on a stream of 6, `%d` returns 6 and then
`""` returns 6; on a stream of 5, `%d` returns -1 with "12345" and "6" at text[5], and
later prints return -1. `%10d` on a stream of 8 returns -1 with "    1234". An unknown
letter counts its blank.

A single print is not otherwise limited: in a stream of 1024, `%s` of 1024 characters
returns 1024 and draws 1024, 1025 returns -1 and draws 1024, and `%1024d` returns 1024
and `%1025d` -1 (`cfnt`, group long).

### KanjiFntPrint's text

Spec 13's rules apply (the text is added whole, however long; the count check comes
first; it returns the new count). The formatter differs from `FntPrint` in what it
makes, and that explains spec 13's "counted twice":

- **Numbers are made in full-width Shift JIS**: each digit of `d` is two bytes, 824Fh
  plus the digit, each hex digit of `x` and `X` is that or 8260h plus its value less
  ten (A to F, upper case for both letters), and a minus sign is two bytes, **815Bh**.
  So "%d" of 42 is 4 bytes and draws what literal "42" draws; of -42 it is 6 bytes, and
  its first cell is 815Bh, where a literal "-" draws the cell of 817Ch (`cfntk`,
  groups pairs and negative).
- `%s` and `%c` copy their bytes as they are. `%c` of 61h draws what literal "a" draws.
- The width is in cells: the fill is one space byte per missing cell, cells being the
  field's bytes over two, rounded down. Bytes returned = fill + field's bytes, with fill
  = width less (bytes / 2) if positive:

| format and value | bytes of field | fill | returns |
|---|---|---|---|
| `%d` 42 | 4 | 0 | 4 |
| `%1d` 7 | 2 | 0 | 2 |
| `%2d` 7 | 2 | 1 | 3 |
| `%5d` 42 | 4 | 3 | 7 |
| `%6d` -42 | 6 | 3 | 9 |
| `%4x` 2Ah | 4 | 2 | 6 |
| `%6s` "a" | 1 | 6 | 7 |
| `%6s` "ab" | 2 | 5 | 7 |
| `%6s` "abc" | 3 | 5 | 8 |
| `%8s` "hello" | 5 | 6 | 11 |
| `%6s` "abcdefg" | 7 | 3 | 10 |
| `%6c` 41h | 1 | 6 | 7 |
| `%c` 41h | 1 | 0 | 1 |

- `0` fill for `x`, `X` makes full-width zeros: `%05x` of BEEFh draws "0BEEF".
- An unknown letter is one space byte, or the width's count of space bytes with a
  width (`%y` 1, `%2y` 2, `%5%` 5). A null `%s` is the same. A plain `%%` is one `%`.
  `%f` with a first word of 0 was seen not to come back in 2500 frames, printing stale
  bytes; it is not to be matched.
- Fills are ASCII spaces, which move 16 pixels and draw nothing (spec 13).

## _err_math

`int _err_math(int code)`. No document describes it. It is the one way the library
reports a math error (`cmatherr`, with PROBE_WATCH on it, and `cerrmath`):

- math_errno (a word) becomes code, whatever code is: 0, 1, FFFFFFFFh and the rest.
- Code 33 (21h, EDOM) calls the kernel's B0h 07h DeliverEvent with a0 F4000002h, a1
  0301h. Code 34 (22h, ERANGE) the same with a1 0302h. Any other code calls nothing:
  35, 121h and 10021h do not. a2 and a3 are left as the caller had them; the kernel
  takes two arguments.
- It returns 0 in v0. v1 is whatever it was, or what the kernel leaves in it.
- Only caller-saved registers change (at, v0, v1, a0, a1, t0 to t2 through the kernel).
  It has no other arguments and writes no other memory seen. It costs 34 cycles without
  an event, 478 with one on this kernel, 8 and 56 bytes of stack.

## math_errno

`extern int math_errno;`, from Sony's header `libps.h` and the User Guide: a variable
at its fixed address, not a routine. All 32 bits are the value (`cerrmath`: 12345678h
written reads back, and FFFFFFFFh through `_err_math`). It is 0 at the program's start
and nothing clears it but the program. The image must lay it out as a word at the entry
table's address, not a jump. A program reads it and writes it directly.

Spec 12 does not set it, and the clean library must. In `cmatherr`, with a watch on
`_err_math`, every arrival carries 33 or 34 and no other value of math_errno is seen,
so the rule is two places call it:

**The helpers call `_err_math(34)` when a result overflows.** This is whenever spec
12's arithmetic packs the infinity pattern because the exponent passed the top, in
`__adddf3`, `__subdf3`, `__muldf3`, `__divdf3`, `__addsf3`, `__mulsf3`, `__divsf3` and
`__truncdfsf2`:

- `__muldf3(1, inf)` and `__muldf3(2, max)` set it, since inf is the number 2^1024.
  `__adddf3(max, max)` sets it, `__adddf3(1, inf)` does not (the larger operand comes
  back as it is, with nothing packed). `__divdf3(inf, 1)` sets it.
- Underflow does not set it: `__muldf3(min, min)`, whose exponent runs into the sign,
  leaves it. A quotient by zero does not: `__divdf3(1, 0)` and `__divdf3(0, 0)` leave it.
- `__truncdfsf2` sets it from exactly 2^128 up (47f0000000000000 up). 47efffffefffffffh
  narrows to 7f7fffff and leaves it. Its underflow does not set it.
- `__fixdfsi` sets it for 2^31 and up, and for -2^31 and below (c1e0000000000000 up in
  magnitude), the two saturated results. 41dfffffffe00000 and c1dfffffffe00000 leave it,
  and so does -0.
- `__floatsidf`, `__extendsfdf2`, `__negdf2` and the comparisons never set it.

**Some math calls test for a bad argument and call it first.** These set math_errno
with their usual result per spec 12:

| call | input | code |
|---|---|---|
| sqrt | x below 0 by the comparison (-0 is not; fff8...h and every other pattern with the sign is) | 33 |
| log, log10 | x below 0 likewise | 33 |
| log, log10 | ±0, or x's exponent field below 512 or from 1535 up, the mantissa making no difference | 34 |
| asin, acos | \|x\| above 1 (NaN patterns, read as large numbers, too) | 33 |
| atan2 | both ±0 | 33 |
| pow | x ±0 and y at or below 0 | 33 |
| pow | x negative and y not an integer | 33 |
| strtod, atof | the scale (spec 12) past 255 or below -255 | 34 |

The rest follows from what each call does in spec 12, and needs no test of its own, if
it is done through the helpers that set it:

- ldexp sets 34 from the running power: its square past 2^512 overflows **even if the
  result would be in range and even if that square is not then used**. `ldexp(1, n)`
  leaves it for 0 to 511 and sets it from 512 up, leaves it from -1 to -1023 and sets it
  from -1024 down (`cmatherr2`). A zero x sets it the same. So squaring the power comes
  once more after the top bit of |n| has been used, as the rule in spec 12 implies.
- frexp is that ldexp: exponent field 0 with any fraction bit set and fields 1 to 510
  set it, fields 511 to 2045 leave it, 2046 and 2047 set it, and a zero leaves it
  (`cmatherr2`).
- exp, sinh, cosh, tanh through ldexp: exp(-710) sets it, for example. sin and tan
  through `__fixdfsi` of their N (1e16 sets it, 1e6 does not). hypot, atan2 and fmod
  through their products and quotients. pow through log and exp. floor, ceil, modf,
  neg, atan and cos on `cmatherr`'s inputs leave it.
- `_get_errno` (B0h 54h, spec 12) is unrelated.

The events DeliverEvent gets are seen by any program that opened them. A helper in a
hot loop that overflows makes a kernel call each time, and the clean one must too.

The table above is on `cmatherr`'s inputs. The acceptance test for all of it is that
probe's lines being equal.

## _dbl_shift

`dbl _dbl_shift(dbl x, int n)`, where dbl is a struct of two words, low first, returned
through a hidden pointer. It has no public document. In registers (`cerrmath`):

- a0: where to store the result. a1: unused (the padding before an 8-byte aligned
  struct). a2: x's low word. a3: x's high word. 16(sp), the fifth word: n.
- The 64 bits of x are shifted **right, arithmetically** by n, and stored at a0: the
  low word then the high word. a0 is returned in v0 and the result's high word in v1.
- n zero or negative leaves x as it is (FEDCBA98h, 80000000h, -1 and -64 all do).
- n from 64 up leaves all sign bits: `_dbl_shift(01234567h, F6543210h, 64)` is
  FFFFFFFFh in both words, `(01234567h, 76543210h, 64)` is 0. n of 1000 is the same.
- Only the 8 bytes at a0 are written, with no stack and no kernel call. Only
  caller-saved registers change (v0, v1, a0 to a3, t0).
- Time grows with n: 82 cycles for n of 0, about 28 more for each shift, 2960 at 100,
  28,160 at 1000, 280,156 at 10000. A large positive n such as garbage from an unset
  word takes minutes of machine time: it is not a hang, and a clean one need not copy
  it, as the result is settled from n of 64. The library's math calls it too, thousands
  of times in a run of `cerrmath`, in 32 to 318 cycles, so with a small n.

x 01234567h, 76543210h (low, high):

| n | low | high |
|---|---|---|
| 0 | 01234567 | 76543210 |
| 1 | 0091a2b3 | 3b2a1908 |
| 4 | 00123456 | 07654321 |
| 8 | 10012345 | 00765432 |
| 16 | 32100123 | 00007654 |
| 31 | eca86420 | 00000000 |
| 32 | 76543210 | 00000000 |
| 33 | 3b2a1908 | 00000000 |
| 52 | 00000765 | 00000000 |
| 63 | 00000000 | 00000000 |

With high F6543210h: n 1 is 0091a2b3, fb2a1908; n 32 is f6543210, ffffffff; n 52 is
ffffff65, ffffffff; n 63 and up is ffffffff, ffffffff.

## Kernel calls

On `cstack` and `cprintf` with the kernel log:

| entry | kernel calls |
|---|---|
| `sprintf`, `sprintf2` | none |
| `printf`, `printf2` | B0h 35h per byte, fd 1, length 1, and two for a newline |
| `FntPrint`, `KanjiFntPrint` | none (a Kanji flush printing "bad sjis code" is spec 13's) |
| `_dbl_shift` | none |
| `_err_math` | B0h 07h for codes 33 and 34, else none |

## Stack

Bytes written below sp as it stands at the entry, the deepest of the calls `cstack`
tried, with interrupts off. A program can keep its stack in the 1 KB scratchpad, so a
clean one should stay at or under these, and under 1 KB in any case. A frame an entry
reserves and does not write is not seen. A line of `cstack` shows the figure plus the
frame of the probe's own `call`: the sp at an entry under PROBE_WATCH against the line's
callSP gives that frame, 40 in this build.

| entry | typical | worst seen |
|---|---|---|
| `sprintf`, literal text | 40 | |
| `sprintf`, `%s` | 56 | |
| `sprintf`, `%d`, `%x`, `%c` | 60 | |
| `sprintf`, `%o`, INT_MIN | 68 | |
| `sprintf`, `%020d`, `%.20d` | 76 | **76** |
| `sprintf2`, literal, `%s` | 128 | |
| `sprintf2`, `%d`, `%x`, width | 144 to 152 | |
| `sprintf2`, `%c` | 488 | |
| `sprintf2`, `%f`, `%e`, `%g` of 0 | 648 | |
| `sprintf2`, `%f`, `%e`, `%g` of 1 or large | 856 | |
| `sprintf2`, of pi, or `%.40f`, `%.300e` | 912 | **912** |
| `printf`, no text | 104 | |
| `printf`, text | 264, 296 with a newline | **296** |
| `printf2`, no text | 128 | |
| `printf2`, text | 624, 656 with a newline | |
| `printf2`, `%f`, `%e`, `%g` | 912 | **912** |
| `FntPrint` | 40, 44 with a number | **44** |
| `KanjiFntPrint` | 40, 44 to 48 with a number | **48** |
| `_err_math` | 8, 56 with an event | **56** |
| `_dbl_shift` | 0 | **0** |

printf's figures include the kernel's write on this kernel (OpenBIOS). write by itself
was 72. Width and precision do not add to any of these: `%1000d` is the same as `%20d`.

## Cost

CPU cycles from root counter 2 in system clock mode (the clock over 8 past 60000, so
those are good to 8 cycles), around the call, the minimum of 5, with the 28 cycles of
reading the counter taken off, interrupts off, on `cstack`:

| call | `sprintf` | `sprintf2` |
|---|---|---|
| `""` | 130 | 170 |
| `"x"` | 164 | 202 |
| 16 literal characters | 554 | 682 |
| `%d` 42 | 452 | 464 |
| `%d` INT_MIN | 798 | 888 |
| `%x` BEEFh | 480 | 566 |
| `%o` FFFFFFFFh | 664 | 914 |
| `%s` "hello" | 470 | 544 |
| `%c` | 324 | 362 |
| `%20d` | 862 | 1042 |
| `%020d` | 1048 | 1066 |
| `%1000d` | 18574 | 26594 |
| `%d %d %d %d` | 1312 | 1242 |

So `sprintf` is about 130 + 27 a literal byte + about 440 a short conversion + 18 a fill
byte. `sprintf2` is about 170 + 32 a literal byte + about 450 a short conversion + 26 a
fill byte.

Floating point is much slower and varies:

| `sprintf2` | 0 | 1.0 | pi | max |
|---|---|---|---|---|
| `%f` | 934 | 26136 | 70012 | 230108 |
| `%e` | 1124 | 26312 | 70188 | 219428 |
| `%g` | 772 | 25948 | 63772 | 219436 |

`%.15f` of pi 126804, `%.40f` 274516, `%.300e` 281484. A digit of integer or fraction
costs thousands of cycles.

`printf` is a kernel call a byte, with interrupts off here: `""` 164, `"x"` 686,
`"plain\n"` 3772, `%d` 42 1436, `%d` INT_MIN 6288, `%20d` 10830, `%s` 2992. `printf2`
adds a few tens: `""` 172, `"x"` 702, `%f` of pi 73988. On a kernel with a slower write
these grow (spec 01 and spec 08's figures for the write).

`FntPrint`: `""` 176, `"x"` 218, `%d` 412, `%20d` 1000, `%s` 518, `%x` 508.
`KanjiFntPrint`: `""` 184, `"x"` 218, `%d` 454, `%20d` 826, `%s` 458, `%x` 598.

`_err_math` 72 by the counter (34 between arrival and return), 516 with an event.
`_dbl_shift` as above.

## The probes

Each is `exp/cleanlib/spec/probes/NAME`. They stop at a line "END".

- `cfmt`: the integer, character and string grids on all four entries, each byte as a
  conversion, malformed directives, `%n`.
- `cfmtdbl`: `sprintf2`'s doubles.
- `cfmtlong`: long output, null strings over known low memory, null format and
  destination.
- `cprintf`: `printf` and `printf2` text and returns.
- `cstack`: stack and cycles for every entry.
- `cfnt`: `FntPrint`, both forms, against `sprintf`.
- `cfntid`: the id test.
- `cfntk`: `KanjiFntPrint`'s returns and glyph cells. Its last call, past END, is the one
  that does not come back.
- `cerrmath`: `_err_math`, `_dbl_shift`, and math_errno around the failing calls.
- `cmatherr`: math_errno and the result over every math call and helper's edges.
- `cmatherr2`: where math_errno changes along ldexp's powers and frexp's and log's
  exponent fields. Give it some minutes: it is the slow one.
- `regwatch`: a host harness, not a program. Probes that print long lines are too slow
  on probetest's TestDump, whose TTY check copies the whole output every instruction:
  `cfmt` did not end in ten minutes. TestRegs looks every 64K instructions and also logs
  all 32 registers at each arrival and return at PROBE_WATCH's addresses, and with
  PROBE_KERNEL=1 the kernel calls:

      go test -c -o .build/regwatch.test ./exp/cleanlib/spec/probes/regwatch
      go build -o .build/yaroze-cimg ./cmd/yaroze
      .build/yaroze-cimg disc --elf .build/N-cimg/prog.elf -o .build/N-cimg/N-cimg.bin \
          exp/cleanlib/spec/probes/N
      cd exp/cleanlib/image/probetest
      PROBE_DISC=$R/.build/N-cimg/N-cimg.cue PROBE_OUT=/tmp/N PROBE_FRAMES=800000 \
          $R/.build/regwatch.test -test.run 'TestRegs$'

  which writes /tmp/N.tty, with its carriage returns taken out, and /tmp/N.regs. Run
  it on the clean image with PSX_LIBPS_IMAGE at build time, as probe.sh does.

## Acceptance

1. `cfmt`, `cfmtdbl`, `cfmtlong`, `cprintf`, `cfntid`, `cmatherr`, `cmatherr2` and
   `cerrmath` give the same TTY from their first line of output to END on the clean
   image as on Sony's, the library's start-up lines apart.
2. `cfnt` and `cfntk` give the same, apart from the unknown-letter cases in the id form
   and with a zero first argument, which show stale bytes on Sony's.
3. `cstack`: stack figures at or below the table above, each entry, and cycles
   reported, not matched.
4. A C program's `printf` with a literal and conversions prints the same text and
   returns the same (conversions' bytes only). `image/regress.sh` stays where it was.

## Open

- **Real programs' use of the two math entries.** The jal words of `_err_math` are in
  many web archive programs (tuto, tunelfin, breakout, grav, race2, techdemo, soccer,
  pokris, dungeon, airbob and more, one a program) and of `_dbl_shift` in adven, hover,
  opera, mimet, beta1 and mce. Each is in code linked into the program. Four discs for
  1200 frames and three (adven, hover, opera) for 3000 made no call, with no pad input,
  so no real argument was logged. The registers above come from calling them directly
  and from the library's own calls.
- Whether math_errno is written before DeliverEvent, which a handler reading it would
  show. No event handler was run. And what a handler sees of v0 on a kernel other than
  this one.
- The stack figures on the real kernel, where B0h 35h and B0h 07h may go deeper. Only
  OpenBIOS was run.
- `%f` to `KanjiFntPrint` with a first word of 0: why it did not come back in 2500
  frames. The stale case also comes up in FntPrint's id form as garbage.
- `KanjiFntPrint` after `KanjiFntClose`: `cfntk` got 2 for "A" and 2 again for an id
  form, where spec 13 says -1 with no stream open. Its stale count may be why; a stream
  closed differs from never opened, perhaps.
- Lower case hex from `FntPrint`'s `%x`: the flush folds it, so it can't be seen and
  doesn't matter to the picture, though a program could read it back through the tilde
  trick of spec 01. `KanjiFntPrint`'s is upper case.
- A format pointer as a0 equal to a small id where a stream is open: there is no way
  for a C program to mean that, so it is not a question for programs.
- Whether doubles skip by the argument's index or by its address. With sp 8-aligned, as
  o32 has it, the two are the same; `sprintf2` called with a misaligned sp was not tried.

## What the first runs showed

On the clean image, from outside, with `probe.sh` (2026-11-04).

- `cfmt`, `cfmtdbl`, `cfmtlong`, `cprintf`, `cfntid`, `cerrmath`, `cmatherr` and
  `cmatherr2` are identical to Sony's from their first line of output to END, the
  library's start-up lines apart. The double rules above needed no change.
- probe.sh's harness only stops on END if it is still the TTY's last line when it
  looks. A probe whose `main` returns after END runs on to PROBE_FRAMES, so give such a
  probe no more frames than it needs (`cmatherr2` takes some 7,000).
- hypot with q zero, after its swap to single, makes no step: hypot(0, 0) and
  hypot(x, 0) leave math_errno, and hypot(2^-1022, max), whose q narrows to 0, too, where
  hypot(max, 2^-1022) sets it. atan2 follows.
- pow's integral power does not square its base past the top bit: pow(2, 1000) and
  pow(max, 1) leave math_errno. Spec 12's results do not show this, math_errno does.
- A write of math_errno alone goes at link time, and the variable with it, since
  nothing in the image reads it: it is a volatile store. `_err_math` first showed
  errno 0 everywhere for this.
- `cfnt`: every `sprintf` figure and every `plain` line with a known letter match. Sony's
  stale bytes are wider than "unknown letters in the id form and with a zero first
  argument": on `plain` lines with a first word that is not zero, `%*d`, `%.*d`, `%lld`
  and `%Ld` also print them (43 to 45 bytes), and `%y` and `%f` with their first word
  not zero do in `id0`. `%ld` does not. It depends on what ran before, as this spec says.
  The clean image gives the blank in every case. So 31 lines differ, all of this kind.
- `cfntk`: every return is the same. Its glyph image hashes cannot be compared across
  the images, since Sony's image holds stale bytes the clean one leaves blank (spec 13),
  and 12 of Sony's format and literal pairs differ for that reason where the clean
  image's are equal.
- KanjiFntClose does not close a stream for print or flush, whatever spec 13 says of "no
  stream open". It only has the next open start over. Stream 0 goes on taking prints,
  counting from where it was, and flushing, while the id test sees none open: after a
  close `kprint(0, "A")` has its format at address 0. -1 is only for a program that has
  never opened one (`cfntclose`). This settles the Open item. FntLoad does close font
  streams: FntPrint after one returns -1.
- Stack, by `cstack` less its 40: every entry at or under the table but `_err_math` with
  an event, 64 against 56 (the call of the trampoline takes a frame of 32 where Sony's
  takes 8). sprintf 60, printf 288 with a newline, FntPrint 40, KanjiFntPrint 44,
  sprintf2 and printf2 352.
- A double's digits are worked out in memory of the library's own, a 370 and a 320
  byte array, and not on the stack: the compiler puts anything that size on the heap. A
  double printed from an interrupt while the program is printing one would mix them.
- Doubles cost half of Sony's or less: `%f` of pi 18,000 against 70,000 cycles, since
  the clean arithmetic is faster than Sony's (spec 12 does not pace it). Decimal digits
  come by subtraction, with no buffer, so `%d` of INT_MIN costs 1,330 against 800, and
  width fill is cheaper. KanjiFntPrint walks to the stream's NUL on every print, so its
  cost grows with what the stream holds: 8,500 cycles where `cstack` has printed for a
  while, against Sony's 458. FntPrint stays some 50 over Sony's, for the registers a
  whole formatter saves: 220 for `""` against 176.
