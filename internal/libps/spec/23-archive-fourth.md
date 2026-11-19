# The web archive's programs: fourth round

After specs 20 to 22, `exp/cleanlib/image/archive.sh` has 88 of 140 programs `same`. This spec
holds what tracing the other programs' calls found. Spec 20 tells how to run things, spec 21
the timing tools.

There is a new tool. With `ARCHIVE_CALLS="NAME:FRAMES ..."`, `TestArchiveCalls` writes
`.build/carchive/calls/NAME-sony.txt` and `NAME-clean.txt`, a line for each call a program makes
into the library: the frame, the cycle count at entry, the entry's name, a0 to a3, v0 on return,
and a1 to a3 on return. Comparing two lists shows which call a difference starts at, and the
cycles between two calls show which call takes longer. Run it from
`exp/cleanlib/image/probetest` with `PSX_BIOS` set to the retail BIOS, after an `archive.sh` has
built the discs:

    ARCHIVE_CALLS="pssoft-bm:150" ../../../../.build/carchive/probetest.test \
      -test.run TestArchiveCalls -test.timeout 10m -archive

Register values other than a call's real arguments and results are whatever was there, and
differ freely. A swing of about 15,500 cycles between two calls, with the opposite swing nearby,
is the vertical blank's interrupt landing in another call, and means nothing.

## Behavior

**VSync(1) moves nothing.** Spec 01 was wrong here, and is now corrected. VSync(1) returns the
lines since the last waiting VSync returned, and changes neither that count's start nor the
count of blanks a later VSync(n) goes from. Only a mode that waits moves them, as it returns. So
two VSync(1) in a row give the same figure or nearly, and the VSync(0) after them returns the
lines from the same start. A negative mode moves nothing either. Sony's, on
`spec/probes/vsyncone`, with a wait of 300 lines after a VSync(0):

    VSync 1 = 300 blanks 1
    VSync 1 = 309 blanks 1
    VSync 1 = 348 blanks 1
    VSync 0 = 357 blanks 2
    VSync 1 = 9 blanks 2

and VSync(1) = 300 followed by VSync(2) returns 309 at blanks 2, where the clean library gives
9 at blanks 3. Programs print these as a CPU and a GPU figure, VSync(1), DrawSync(0), VSync(1):
`test-dyddem01`, `test-dydref01`, `test-dydtst02`, `test-gr-01`, `event-event001-hhpd0002`,
`scei-sample-graphics-bmirror`, `scei-sample-graphics-cube`. Sony's picture shows `GPU : 15422`
where the clean library's shows `GPU : 0`.

**SsVabTransfer** returns -1, FFFFFFFFh, where the clean library returns 0000FFFFh. It is the
same in every program I looked at that loads a bank, so check other results of this kind in the
sound calls for the same slip: a -1 in 16 bits that should be 32.

**SsSeqOpen at an odd address.** `pssoft-bm`'s second call is SsSeqOpen(8009219Dh, -1). The call
never returns on the clean library, and on Sony's it returns 1. The data there starts:

    70 51 45 53  00 00 00 01  01 e0 09 27 c0 04 02 ...

A SEQ can be at any byte address, and all its fields have to be read by bytes. The program's
first call, at 80090014h, returned 0 on both. Sony's goes on to 891 picture changes where the
clean library has 125. See that it plays correctly from an odd address too, not only that it
returns, and look at the VAB and the other sound data calls for the same assumption.

**GsSetLightMode** writes GsLIGHT_MODE, not GsLMODE (spec 22). Fix it.

## Time: three pads went the wrong way

Spec 22's work set three pads under Sony's figure per unit so that the means in `costs.txt`
would line up: FntFlush's glyph by 14 cycles, FntPrint's C character at 32 against 36, and
GsLinkObject4's primitive at 56 against 62. That was the wrong trade: long inputs then run ahead
of Sony's. `test-hasire86-mh` calls GsLinkObject4 2,941 times, and is 39 frames ahead at the end
of the trace for that alone. Its picture changes begin at frame 529 against Sony's 584.

From that program, GsLinkObject4(8009000Ch, ...) to the next call is 92,054 cycles on Sony's and
69,166 on the clean library, every time. Put each pad back to the figure per unit that a probe
gives, and take any excess from the fixed part of the call.

## Time: too quick

| call | program | gap to the next call, Sony/clean |
|---|---|---|
| sin | `event-event002-r3`, 450 calls, 12 frames ahead | 300,000/260,000 |
| sin | `test-demo-fm`, 512 calls, 15 frames ahead | 302,000/260,000 |
| cos | `test-cube` | 226,098 and 268,092 for one argument, 268,252 and 285,632 on the clean library |
| Krom2Tim | `pssoft-koro-koro`, 300 calls | 631,720/614,618 |
| KanjiFntOpen | `test-hasire86-mh` | 91,880/67,602 |

Sony's sin and cos vary with the argument, and the clean library's is about the same for all of
them. r3's arguments run 3F91DF46A2506B91h, 3FA1DF46A2506B91h and up (a degree's worth of radians
and its multiples), demo-fm's 3F8921FAFC8B007Ah, 3F9921FAFC8B007Ah and up. Probe Sony's sin and cos
over such arguments and over the usual range, and pace by what you find. Plain figures that get
the sum right over those programs are enough. These run long enough that a few percent is a
frame. Check these with the gap between calls as above, not only with `costs.txt`.

## Time: still slower

Each of these tips some program past a frame: `scee-topten-adven` is one frame late at its
frame 326 for a printf, `test-pd961207` is 3.9 frames late by its end.

| call | Sony | clean |
|---|---|---|
| GsSortObject4 | 4,185 | 4,745 |
| printf | 19,560 | 21,302 |
| `__divdf3` in `test-pd961207` | gap 11,536 | gap 34,226 |
| TestCard, first call of `scee-topten-hover` | gap 1,327,126 | gap 1,468,252 |
| GsSortFixBg16 | 15,576 | 17,697 |

The figure for `__divdf3` is for one form of its arguments, among those spec 21 notes at 10,940:
probe which values take the short way. For GsSortObject4, spec 22's work stopped because what is
left needs assembly in about 16 loops. Do not do that now. Look only for what plain Go can do.

## Not to chase

These are understood and not the library's fault.

- `test-padlib-padlib-sample`: the logo's position is two stack bytes of the program's own, passed
  to its own routine, which probably sets them only for an analog pad. They are whatever earlier
  calls left. Sony's gives (176,99) and the clean library (160,120), the screen's center.
- `scee-topten-grav` and `tool-cardedit-2`: stack garbage as well (spec 20).
- `test-zero-zero`, `event-event003-24h-wires` and `-s`: `srand(GetRCnt(...))`, seeded from a
  running timer. They match only at exact cycles.
- `showcase-sf` and `event-event003-theme-krwire`: Sony's side stops. `sf` never comes back from
  a cos on Sony's. We do not match Sony's crashes.
- `test-wme-wme`: the start-up's card wait, spec 21.

Figures printed on the screen, as CPU and GPU above, stay different until the time they measure
is the same to the line. A program counts as fixed when its text differs only in those digits.

## Acceptance

- `exp/cleanlib/image/probe.sh vsyncone exp/cleanlib/spec/probes/vsyncone`: the figures match but
  for a line or so from printing's own time, and every `blanks` matches.
- `FIRST=1 exp/cleanlib/image/archive.sh pssoft-bm test-hasire86-mh event-event002-r3 test-demo-fm
  test-cube test-dydtst02 test-dydref01 test-gr-01 pssoft-koro-koro scee-topten-adven
  test-pd961207`. `pssoft-bm` should reach Sony's 891 changes, and `r3`, `demo-fm` and `hasire86`
  should not be frames ahead. Say what each shows. In `first.txt` the second figure of the CPU and
  GPU programs should now be digits.
- `exp/cleanlib/regress.sh`, without `PSX_BIOS` in the environment: FAIL stays 0, and a new TIME
  has to be explained. `exp/cleanlib/image/regress.sh`: FAIL stays 0.
- The full `exp/cleanlib/image/archive.sh`, with its counts before and after, and name anything
  that got worse.
- `exp/cleanlib/image/terra.sh`: all pass.

## What the work on this spec found

Probes of Sony's library by the agent that did the work. Where these disagree with the sections
above or with earlier specs, these are right.

**GsLinkObject4**: 62 cycles a primitive holds only for modes 20h and 30h. Every other mode is 56.

**SsVabTransfer**: Sony's never returns with the header at +2, so the VAB needs no byte reads.
+1 and +3 were not tried.

**SsSeqOpen**: a first byte other than `p` prints `This is an old SEQ Data Format.` and opens. A
byte 7 other than 1 prints `This is not SEQ Data.` and returns -1, and that slot stays taken.
Bytes 1 to 3 and 4 to 6 are ignored. The old format's real layout is not known: it played no
notes in 300 frames on either side over 4 test layouts.

**SsUtGetVVol** (spec 05) returns the live register over 129, not the values SsUtKeyOn was given.
**SsUtSetReverbType** with -1 to -3 returns 1 to 3 and stores 101h to 103h. **SsGetSerialAttr**
returns 1 for s or attr above 1. **printf(NULL)** prints nothing.

**Times, in cycles:**

- KanjiFntOpen (spec 21): 67,300 + 48·n. There is no blank in the call: 91,880 is n = 512.
- Krom2Tim (spec 21): newlines and later rows cost about 2,050 per cell of the image, 2,225 per
  cell reached and 875 per glyph.
- `__divdf3`: 10,680 + 260 per one bit of the quotient to 2 places below its last, all within 340
  over 378 divisions.
- `__floatsidf`: about 450 flat.
- sin: under π/2 about 266,700, from there about 279,000.
- cos: under π about 268,800, from there about 281,000, about 254,300 at 0 or tiny values. The
  2^-60 threshold is a guess: 1e-30 is tiny and 1e-15 is not.
- memmove's backward copy: 14.4 a byte.
- The first TestCard's gap depends on the frame's phase. Second calls match within 304.

Sony's sin and cos also vary by about 10,000 with the argument, in no pattern found. Sqrt spreads
widely between the two libraries by argument, 215,050 and 267,508 at 3.5, with its mean close.

Left over, clean/Sony: GsSortObject4 4,861/4,330, printf 20,233/19,560 (each directive about 480
over), GsSortFixBg16 15,909/15,576, and fixed excess of 38 in C FntPrint, 24 in FntFlush and 14 in
GsLinkObject4. The figures per unit match. GsSortObject4 still needs assembly in about 16 loops to
go further, and is what holds back `test-cube`, `test-pd961207`, `test-gr-01` and
`event-dengeki001-nanatan-nanatan`, the last of which lost a frame when Krom2Tim's correct time
stopped hiding it.

Not measured: tan, asin and the other math calls after `dFloat` got quicker.
