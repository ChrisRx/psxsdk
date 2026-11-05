# The web archive's programs: time, and GsLinkObject4

Many of the archive's programs show their first pictures on other frames on the clean
library, one frame early for most and hundreds for some. A program that seeds rand
from the clock, or counts frames, then goes its own way. `spec/probes/arccost` and
`TestArchiveAllCosts` measured what follows on Sony's library. Spec 20 tells how to run
the archive.

    PROBE_WATCH=800411cc,80040cc4,80040d9c,80026f04,8002632c,80023cf4,80027404 \
      exp/cleanlib/image/probe.sh arccost exp/cleanlib/spec/probes/arccost

gives a `w` and an `r` line in `.build/arccost-cimg/out.kernel` and `-clean/out.kernel`
for every call of those entries: Sqrt, Sin, Cos, Krom2Tim, KanjiFntOpen, GsLinkObject4,
GetTPage. Only those from the program count, with ra at 80090000h or above.

Two tools, both given the discs `archive.sh` built, from `exp/cleanlib/image/probetest`
with `PSX_BIOS` at the retail BIOS and `$PWD/../../../../.build/carchive/probetest.test`:

- `ARCHIVE_KERNEL="NAME..." -test.run 'TestArchiveKernel$' -test.v` lists every kernel
  call of both images, the cycles from "Execute !", until cycle 3,800,000.
- `ARCHIVE_COSTS="NAME:FRAMES..." -test.run 'TestArchiveCosts' -test.v` compares the time
  in each library entry over a program's first frames, largest difference first. The
  public entry names are all it prints. Do not look in `map1.txt` yourself.

## GsLinkObject4 changes the model

GsLinkObject4(table, obj, n) points obj.Tmd at the table's nth entry, and then walks that
object's primitives, n_prim of them from prim_top. It is **behavior**, not only time:

- It groups the primitives into runs. A run ends when a primitive's mode byte (the top
  byte of its header, translucent bit included) or flag byte (bits 16-23) is not the
  same as the one before. Differing low 16 bits (olen, ilen) do not end a run.
- **The low 16 bits of the header of each run's first primitive are overwritten with the
  run's length.** The headers of the others stay as they are.
- A primitive's size comes from its mode, with the translucent bit cleared, and its
  flags, not from the header. In bytes:

| mode | size |
|---|---|
| 20h | 16, or 24 with flag bit 2 |
| 21h, 29h | 16 |
| 24h, 31h, 38h | 24 |
| 25h, 34h, 39h | 28 |
| 28h | 20 |
| 2Ch, 2Dh | 32 |
| 30h | 20, or 28 with flag bit 2 |
| 35h, 3Ch | 36 |
| 3Dh | 44 |

- A mode not in the table prints `GPU CODE %02xH not assigned.\n` with the mode, by the
  kernel's printf, and has size 0, so the walk stays on it.

The probe prints what it leaves: with 100 identical primitives, 100 in the first low half
and the original value in the rest.

**So GsSortObject4 can no longer take a primitive's size or a run's end from the headers'
low bits.** It reads each run's length from the low 16 bits of its first header, handles
that many at the size above, and goes on, until n_prim are done. A primitive's own header
inside a run is not looked at: the first one's mode and flags stand for the run.

Time: 62 + 62 per primitive, in all (6262 for 100, 62062 for 1000), a little more for
each new run (7054 at 100 runs of a changed flag). Walk them and spend the rest.

## The image calls clip the caller's rectangle

ClearImage, LoadImage and StoreImage keep the size within video memory **in the rectangle
the caller gave**, when the call runs at once with the queue idle:

| call | leaves W and H | |
|---|---|---|
| ClearImage | W over 1023 becomes 1023, H over 511 becomes 511, negatives 0 | |
| LoadImage, StoreImage | W over 1024 becomes 1024, H over 512 becomes 512, negatives 0 | |
| MoveImage | as they were | |

A call that has to wait in the queue takes a copy and leaves the caller's alone.
`scee-topten-grav` has ClearImage clip a rectangle that is its DRAWENV's clip, so the next
PutDrawEnv has a drawing area to y 510 (E4h with y of 1FEh, at Sony's word 45).

## Start-up

From the program's jump to the library's entry (80010030h) to these kernel calls,
in cycles, on the retail BIOS. Over three programs each was within 2,000 except where
noted (TestArchiveKernel subtracts as well):

| kernel call | Sony | clean now |
|---|---|---|
| B(19h), the first | 8,542 | 1,702 |
| A(49h) | 37,126 | 31,958 |
| A(3Fh) (the ResetGraph line) | 90,320 | 84,032 |
| B(4Ah) | 493,000 to 499,800 | 411,264 |
| B(4Bh) | 522,300 to 528,900 | 440,410 |
| B(50h) | 616,000 to 623,800 | 541,966 |
| B(4Fh) | 616,300 to 624,100 | 542,298 |

These matter because of what follows B(4Fh): a BIOS wait that is decided by the vertical
blanks. On about 8 of the 95 programs the 78,000 cycles clean is ahead land it on the other
side of a blank, and the whole library start, 3.3 million cycles against 2.8, comes one
frame apart (564,480 cycles). Spend the time so the table lines up. Also, the clean
side makes a B(13h) and a second B(5Bh) call after the first A(3Fh) that Sony's does not:
remove any that are not needed for anything. Everything else of the order of the kernel
calls should stay as spec 07 has it.

GsInitGraph takes 114,600 on Sony's, every call, and took 84,500 on the clean one.

## Costs: calls that Sony's takes longer over

Sony's cycles per call from entry to return, when all the archive's programs run their
first 120 frames (`.build/carchive/costs.txt`, which is fine to read), with the probe where
it knows better. Spend the difference, after the call's work, where the clean call is
quicker. A figure need not be exact. It is the sum over a run that matters.

| call | Sony | clean |
|---|---|---|
| Krom2Tim | 610,500 + about 5,200 per character, of one byte or two | 216 + 2,187 per byte |
| KanjiFntOpen | 67,100 to 68,800, a refused open too (the higher means hold a vertical blank) | 770 |
| Sin, Cos | 250,000 to 290,000, 7,500 at 0 | 142,000, 880 at 0 |
| Sqrt | see below | 54,500 |
| `__muldf3` | 3,830 to 4,060 for two nonzero values, 1×1 too, 86 to 92 with a zero | 72 mean, 300 |
| `__divdf3` | about 18,000, some values 10,940 | 10,000 |
| `__fixdfsi` | 410 to 426, 48 at zero | 27, 94 |
| SsVabTransfer | 25,200 mean; the difference is 16,700 for a small one | 11,400 |
| GetTPage | 96 | 28 |
| GetPadBuf | 834 | 98 |
| GsSetLightMatrix | 354 | 264 |
| GsGetLs, GsGetLws | 612, 698 | 562, 655 |
| GsInitFixBg16 | 31,400 at 320 by 240, 108,600 at 640 by 480: by the cell | 18,800 |
| KanjiFntFlush | 98,800 | 85,900 |
| TestCard | 1,429,000 | 1,266,000 |
| SetDispMask | 82 | 18 |
| SsSetMVol | 178 | 58 |
| SsSeqOpen | 722 | 376 |
| SsSeqPlay | 228 | 94 |
| SsSetTickMode | 87 | 38 |
| SsUtAllKeyOff | 5,124 | 2,714 |
| SsUtKeyOn | 2,529 | 1,794 |
| SsUtSetReverbDelay, Feedback | 827, 782 when they write, 188 on a type they write nothing for | 76, 62 |
| SsUtSetReverbDepth, SsUtReverbOn | 248, 137 | 90, 38 |

Sqrt(x) on Sony's depends on x's binary exponent e (x = m·2^e, m in [1, 2)):

| x | cycles |
|---|---|
| 0 | 100 |
| exactly 1 | 23,470 |
| otherwise | about 205,000 + 20,000·\|e\| |

(0.25 gives 226,278, 2 gives 211,154, 1234.5 gives 423,590, 1e6 gives 599,860, 1e12
gives 1,016,836, 1e-9 gives 784,190.) Negative values are not measured here.

TestCard's time depends on where in the frame it starts (757,002 and 1,841,824 on the
same probe), so only the sum holds.

## Calls where the clean library is the slower

These can't be paced, only shortened. They are in `costs.txt` with the rest. Do not take
them on as part of this work unless a change is plain: another round will.
