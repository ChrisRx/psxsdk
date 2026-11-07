# Soft float time: `__addsf3` and `__divdf3`

Two programs are held back by soft float time on the image. Find what each routine's time follows on Sony's, by arguments, and make the clean one take that time because of the same operands, not because of a pad fitted to one program's sum.

| Routine | What's known | Program |
|---|---|---|
| `__addsf3` | Clean is +50 to +190 a sum, up to +200 in the program | `pssoft-airbob-2-airbob`, first differs at frame 1068 |
| `__divdf3` | About 10,680 + 260 per set bit of the quotient to 2 places below its last, within 340 over 378 divisions (spec 23). Some values take about 10,940 (spec 21). What decides the rest isn't known | `test-pd961207`, picture differs at frame 845 |

Specs 12, 17, 21, 23 and 31 hold what's known of their results and time. Spec 31's sums are double precision: a base by signs and order, 24 cycles per exponent place, 60 for a carry and about 81 per place of renormalizing. `__addsf3` may or may not follow the same shape, so measure it.

## Work

For each routine, in the table's order:

1. Probe Sony's by arguments, calling it by name in the emulator and reading the cycles, as `probes/accfp` and `probes/accfp2` do. Change one thing about the operands at a time: signs, order, exponent gap, carry, renormalizing distance, mantissa bits, zeros and other special values. For `__divdf3`, include the divisor's and dividend's own bits and the size of the remainder as it goes, as well as the quotient's bits.
2. Write down the time as a rule and test it on arguments it wasn't fitted on. Say how much of the spread it explains.
3. Make the clean routine match that rule, in assembly if that's what it takes. Results stay bit for bit what they are now.
4. Run the program, and say what it shows now and what stops it if it isn't `same`. If it's another routine, name it and its error, and stop there rather than taking on a third routine.

If `__divdf3`'s time can't be explained, report how far you got and leave its current pacing in place. Don't fit a program's sum. A fit that makes `test-pd961207` pass without a rule isn't wanted.

## Acceptance

Time by cycles only.

- Probes in `exp/cleanlib/spec/probes/`, kept, with a table in this spec of Sony's and clean's cycles for each path.
- `FIRST=1 exp/cleanlib/image/archive.sh` on the two programs, then the full `exp/cleanlib/image/archive.sh`, against `.build/carchive/summary-verify43.txt`: name anything that got worse.
- `ARCHIVE_TEXT` on `demos-scee-balls demos-scee-chap5-1 demos-scee-morph event-event001-hhpd0002 scei-sample-graphics-bmirror scei-sample-graphics-cube test-bomb96-bomb test-dungeon-dungeon test-dyddem01 test-dydref01 test-dydtst02 test-gr-01 test-wnfs-t2-wnfs-t2 test-wstg-tv3`: no differences.
- `exp/cleanlib/regress.sh` without `PSX_BIOS`: 33 PASS and 8 TIME. `exp/cleanlib/image/regress.sh`: 34 PASS and 7 TIME. `exp/cleanlib/image/terra.sh`: all pass. `go test ./tests` passes.
- Any routine calling these, such as sin, cos, pow and printf with floats, keeps its time. Their pads were fitted over the current soft float, so if a change moves them, say by how much and correct the pad only where the probe shows the same rule still holds.
- The image ends below 80068000h, now 80066288h. Say early if it won't fit, since the spec side lays it out.

## What the work on this spec found
Both routines now take Sony's time by rule. On the image, `__addsf3` and a program's `__divdf3` are assembly in `image/clib.S`, doing the work in the steps the time follows, and their results are unchanged. Static builds keep the Go, in `mathexport_static.go`. `make.sh` still reports the image ending at 80066288h.

### `__addsf3`

Over 3,268 probed sums (`probes/accsf`) the rule below gives Sony's cycles exactly, every one, across a spread of 34 to 836. It was worked out from the sums changed one thing at a time. Then it was tested on 1,500 random sums and 800 more held out, with no misses. The clean routine takes Sony's cycles on all 3,268, and the TTY, results included, is identical.

- First operand zero: 34. Second operand zero: 40.
- Exponents more than 25 apart: 54 with the first the larger, 60 with the second. This holds for any signs.
- An exact cancellation: 108.
- Otherwise 150, plus:
  - 2 for each negative operand
  - 2 when the second operand's exponent is the larger (a larger significand alone costs nothing)
  - 6 for a carry
  - 4 and then 8 a place for moving down
  - 2 on a carry or a move down when the exponents are the same
  - 4 when rounding carries out of the top
  - less 2 when the result's last bit before rounding is odd
- Out of range at either end: 484 more, or 486 with a negative result. The clean routine writes `_err_math`'s work out in line, because the Go call took too long. The kernel's event calls have the same count and arguments on both sides.

The exponent's own value, the mantissa's other bits, what falls off in the shift and the guard bits take nothing. It doesn't have spec 31's double precision shape: the base and the place costs differ, and exponent places cost nothing.

| Path | Operands | Sony's | Clean before | Clean now |
|---|---|---|---|---|
| first zero | 00000000 + 3f800000 | 34 | not recorded | 34 |
| second zero | 3f800000 + 00000000 | 40 | not recorded | 40 |
| out of reach, first larger | 43200000 + 36200000 | 54 | 126 | 54 |
| out of reach, second larger | 36200000 + 43200000 | 60 | 114 | 60 |
| cancellation | 40100000 + c0100000 | 108 | 158 | 108 |
| no carry, first larger | 40123456 + 3f123456 | 148 | 334 | 148 |
| no carry, second larger | 3f123456 + 40123456 | 150 | 322 | 150 |
| signs differ | 41500000 + bf100000 | 152 | 342 | 152 |
| both negative | c0123456 + bf123456 | 152 | 334 | 152 |
| carry, one exponent | 3f800000 + 3f800000 | 158 | not recorded | 158 |
| carry, apart | 3fc00000 + 3f400000 | 156 | not recorded | 156 |
| down 1 | 40000000 + bf800000 | 164 | not recorded | 164 |
| down 23 | 3f800001 + bf800000 | 342 | not recorded | 342 |
| rounding carries | 3fffffff + 34000000 | 156 | not recorded | 156 |
| over the top | 7f7fffff + 7f7fffff | 640 | not recorded | 640 |
| below the bottom | 00800000 + 80000001 | 656 | 818 | 656 |

### `__divdf3`

The time follows a division of one bit a turn, 54 turns, on the 53-bit significands. If the dividend's significand is the smaller, it is doubled first. Each turn compares, subtracts on a bit of the quotient, and doubles what's left.

Over a turn with no bit:

- A bit takes 262.
- A bit where the high words match takes 274, or 286 on the first turn, and 282 where the whole significands match.
- High words matching with no bit takes 8.
- A borrow at bit 16 takes 4 off, and a borrow at bit 32 takes 4 off.
- A carry from the low word to the high one when doubling takes 4.

Around the turns, the base is 10,658 with every turn counted in it, which makes 1/1 come to 10,940. Then:

- 76 for the first doubling, 4 more if it carries between the words, 8 more if the high words match
- 4 for bit 32 of the quotient
- 10 less when nothing is left at the end
- 10 for a fraction of zero
- Above the top: 486, or 488 negative.
- Below the bottom: 24 a place for shortfall + 1, with no cap (a far underflow reaches 35,500), 4 for each set low bit of the high word shifted out, and 20 less. A remainder there saves 4 against the usual 10.
- Second operand zero: 94 with the signs alike, 90 with them differing.
- First operand zero: 98.
- A nonzero operand whose high word is zero but for its sign: 2 more for the first, 4 more for the second. This is approximate, from a few special values.

Sign and exponent take nothing otherwise.

How much of the spread it explains, on the 800 random quotients held out of the fit (`fresh`, 14,680 to 21,644, standard deviation 948): the rule gives 793 exactly, and misses the other 7 by 4. Spec 23's rule had an RMS error of 248 and a worst of 2,904 on the same 800. Over all 3,752 quotients the clean routine matches Sony's on 3,718, is 4 under on 31 and 6 over on 3. All 1,500 of the fitted random ones are exact, and 796 of the 800 held out. The misses are in special values, some denormals and divisors whose high words nearly match the dividend's. What decides them wasn't found. The TTY, results included, is identical over all of them.

| Path | Operands | Sony's | Clean before | Clean now |
|---|---|---|---|---|
| zero over x | 0 / 3ff0000000000000 | 98 | 114 | 98 |
| x over +0 | 3ff0000000000000 / 0 | 94 | 106 | 94 |
| x over -0 | 3ff0000000000000 / 8000000000000000 | 90 | 106 | 90 |
| 1/1 | 3ff0000000000000 / 3ff0000000000000 | 10,940 | 10,952 | 10,940 |
| 3/1 | 4008000000000000 / 3ff0000000000000 | 11,192 | 11,218 | 11,192 |
| 1/1.5 | 3ff0000000000000 / 3ff8000000000000 | 17,808 | 17,800 | 17,808 |
| pi/e | 400921fb54442d18 / 4005bf0a8b145769 | 19,570 | 19,658 | 19,570 |
| 1/(1 + 2^-52) | 3ff0000000000000 / 3ff0000000000001 | 24,498 | 24,798 | 24,498 |
| all fraction bits | 3fffffffffffffff / 3ff0000000000000 | 24,686 | not recorded | 24,686 |
| above the top | 7fe0000000000000 / 3fe0000000000000 | 11,426 | not recorded | 11,426 |
| shortfall 1 | 0590000000000000 / 4590000000000000 | 10,992 | not recorded | 10,992 |
| far underflow | 0010000000000000 / 7fe0000000000000 | 35,500 | not recorded | 35,500 |

### The library's own callers

The library's own `/` and `div` still go through `softDivdf3` with its old pacing, and its own float sums through `sAdd`, so nothing they spend has moved. Only a program's calls get the new routines. Measured before and after on the same probes: `pow` 234 of 234 calls unchanged, `sin` and `sqrt` within 2 on all 88 calls, 12 of which moved at all. No pad was corrected. Cos, tan, exp, log, atan, hypot and printf with floats weren't measured, since those probes made no calls at the watched entries. They share the unchanged code. This also means sin and the like don't yet spend the rule's time on their quotients.

### The two programs

`pssoft-airbob-2-airbob` still differs at change 201, frame 1068 on both sides, 37,919 pixels. `__addsf3` was not what held it. It no longer appears in the time differences at all, and all 36,841 float and comparison calls checked to frame 1075 have the same arguments and results. What stops it is phase. From frame 1064 the program runs free, with no wait, at about 441,000 cycles a loop, and clean arrives about 13,900 cycles late. So one `GsSwapDispBuff` lands in frame 1067 on Sony's and frame 1068 on clean. The lateness builds over the loading before that, with no one routine behind it. Over frames 900 to 1063 the gaps add up to +14,174. The largest are `memset` to `bcopy` (+89 a call, +5,808), `bcopy` to `bcopy` (+0.2, +1,860), `__mulsf3` to `__addsf3` (+28.7, +1,376), and malloc, free and `GsGetTimInfo` at about +10 a call.

`test-pd961207` moved from `differ` to `shift`, with 658 changes on both sides and no different picture in any of them. Its first shift is change 249, at frame 490 on Sony's and 489 on clean. Clean is about 9,500 cycles ahead by then, again in a loop with no wait, losing about 19 cycles a loop. `__divdf3` is gone from the time differences. What's left, as gaps to frame 490, is `RotMatrix` to `RotMatrix` (-2 a call, -10,950), `__fixdfsi` to `GsSetFlatLight` (-12.9, -6,352), `GsSetRefView2` to `GsGetLws` (-12, -5,964), offset in part by `__floatsidf` to `__divdf3` (+10, +9,956) and `FntFlush` to `RotMatrix` (+30, +7,410). `__floatsidf` itself is about +25 a call.

Seen along the way and not taken on: `SsUtKeyOn` returns 7 on clean and 0 on Sony's at airbob's frame 600, the same arguments.

### Corrections to earlier specs

- This spec's table: `pssoft-airbob-2-airbob` was not held back by `__addsf3`.
- Spec 23: `__divdf3` isn't 10,680 + 260 per set bit to 2 places below the last. It counts 54 bits, 1 place below the last, at 262 each, with the terms above. "Within 340" held for those 378 divisions, not in general: the worst on the held-out set is 2,904.
- Specs 21 and 23: 10,940 isn't a short way for some values. It is a quotient with one set bit, a significand over itself.
- Spec 31: a quotient isn't 264 per set bit, for the same reason.

### Why `__divdf3` reads short in `event-event002-r3` and `test-demo-fm`

`ARCHIVE_COSTS` on 120 frames shows clean short: r3 by 61,856 over 209 calls and demo-fm by 64,100 with 210 calls against 209. The routine isn't short. The gap is the vertical blank's interrupt, which the entry-to-return count includes, landing inside a different number of these calls on each side. Nothing in the library changed for this.

Per call, from `TestDump` with `PROBE_WATCH` on the entry, on each program's discs, counting calls from the program only:

| | r3, Sony's | r3, clean | demo-fm, Sony's | demo-fm, clean |
|---|---|---|---|---|
| calls | 450 | 450 | 383 | 382 |
| at the rule's cycles exactly | 433 | 438 | 372 | 370 |
| with an interrupt inside | 17 | 12 | 11 | 12 |
| what an interrupt adds | 15,112 to 15,546 | 15,090 to 15,524 | 15,112 to 15,546 | 15,522 to 15,524 |

- Every call without an interrupt in it takes the rule's cycles on both sides. There is no even shortfall, and the arguments are the same call for call.
- No call in r3 has an interrupt on both sides, so the two sets are disjoint. The calls hit have nothing in common but where the blank falls.
- The first 209 calls of r3 reproduce the figure exactly: 3,867,656 and 3,805,800, 9 interrupted on Sony's and 5 on clean.
- demo-fm's window has 7 interrupted on Sony's and 4 on clean, plus Sony's extra call: 3,775,474 and 3,711,374, again exact.
- Over its 382 calls, demo-fm's gap is down to 1,564. The first 450 of r3 still read 77,554, with 5 more interrupted on Sony's.
- The counts fit chance. Time in the routine over a frame of 564,480 cycles predicts about 14 interruptions in r3's 450 calls, against 17 and 12, and about 12 in demo-fm's 382, against 11 and 12. The survey's 460 calls in 4 programs at 16,051 and 15,916 are the same 62,000 or so. That isn't re-derived per program.

`probes/accdivreal` calls the routine directly on all 652 argument pairs from these calls, three times each with interrupts masked. All 652 take the same cycles on both sides, none vary between the three calls, all equal the rule, and the results are identical (98 to 20,778, mean 17,770).

The two sides see different blanks because the programs' phase differs widely: r3's entries run up to 323,128 cycles apart, more than half a frame, and come back to 7,362 by the last 45 calls. The routine doesn't cause that. `sin` takes about 86% of these programs' time and is off per call, but it wasn't separated from the interrupts, so it is the likely source and not a measured one.

An interrupt inside a call adds 22 cycles more on Sony's than on clean, so the handler itself is slightly short. Spec 31 has that open as the interrupt's kernel-side entry.

`__floatsidf`'s 78 a call in r3 isn't a rate. 16,340 of the 209 calls' difference is one interrupt of 15,996 on Sony's, in no call on clean, which leaves 344, or 1.6 a call. It is a separate small error, not part of the `__divdf3` gap. Over r3's 450 calls without interrupts Sony's means 451.6 and clean's flat 445 to 446 (446 where sampled). Sony's varies with the integer: 44 at 0 against clean's 34, and for the smallest values 474 at 1, 466 at 2 and 3, 458 from 4 to 7, 450 at bit length 4, against clean's 446. Not taken on, as asked.

A small-window `ARCHIVE_COSTS` figure on a routine called a few hundred times can swing by multiples of 15,500 without the routine being wrong. Compare calls with `PROBE_WATCH` before reading it as an error. The statements above in this spec about `__divdf3` and its callers stand: nothing in them changes.
