# The programs still off in time

Spec 31 worked through routines by list, and only one of eight timing programs reached `same`. This spec goes the other way: start from each program's first difference, and fix what caused it.

Baseline: `.build/carchive/summary-verify37.txt`, 98 same.

| Program | Now | First lead |
|---|---|---|
| `test-demo-fm` | `differ`, change 72, frame 362 on Sony's | `init` is 566,082 cycles short, about one frame of a kernel wait in start-up |
| `event-event001-rave096` | `tty`, 1,078,906 and 1,077,866 bytes | 99.9% of Sony's output: printf's flagged paths and other library time |
| `scee-topten-cardem` | `differ`, frame 43 | Not traced |
| `pssoft-koro-koro` | `differ`, frame 362/361 | Not traced |
| `scee-topten-adven` | `differ`, frame 694/690 | Unexplained |
| `pssoft-airbob-2-airbob` | `differ`, frame 1065/1066 | Moved earlier with spec 31's paced math, which may be the cause |
| `test-pd961207` | `differ`, first shift 93/94, picture 845 | Moved earlier with spec 31's paced math, which may be the cause |

`demos-scee-chap5-1` differs by one digit of the root counter value it prints. It's a free check: start-up and printf time that's right should bring it back.

## How

Take the programs in the table's order, with `test-demo-fm` first, and for each:

1. Find where the two sides first part in time, from before the first visible difference: the tools in specs 20 to 25 (`FIRST=1`, `TestArchiveCosts`, `ARCHIVE_ALLCOSTS`, `ARCHIVE_START`, `ARCHIVE_SHOW`, `TestArchiveBus`). A program can have many small errors, so find which one moves it past a frame edge, not only the biggest.
2. Name the cause: a library routine by arguments, the kernel's side, something the program reads, or not the library at all.
3. Probe that routine across its arguments, so the fix follows Sony's behavior in general.
4. Fix it, then run the program again, and go on to the next difference if it's still not `same`.

Move to the next program when a program reaches `same`, or when what stops it is outside the library, or when you can't explain the next difference after a fair try. Say which.

Fix causes, not programs. Never tune a figure to one program's sum: spec 31's `sin` pads were fitted because no cause was found, and removing such a fit by finding the cause is worthwhile. Where a fit is all you can get, say so plainly.

What exists only to match goes through `fidelity.go`'s operations or the `LIBPS_FIDELITY` macros, so static programs don't pay.

## Acceptance

Time by cycles only. Report per program: what first differed, the cause, what you changed, what it shows now, and what stops it if it isn't `same`.

- `FIRST=1 exp/cleanlib/image/archive.sh` on the 7, then the full `exp/cleanlib/image/archive.sh`, against the baseline: name anything that got worse.
- `ARCHIVE_TEXT` on `demos-scee-balls demos-scee-chap5-1 demos-scee-morph event-event001-hhpd0002 scei-sample-graphics-bmirror scei-sample-graphics-cube test-bomb96-bomb test-dungeon-dungeon test-dyddem01 test-dydref01 test-dydtst02 test-gr-01 test-wnfs-t2-wnfs-t2 test-wstg-tv3`: still no difference.
- `exp/cleanlib/regress.sh` without `PSX_BIOS`, `exp/cleanlib/image/regress.sh`: 33 PASS, 8 TIME, 0 FAIL. `exp/cleanlib/image/terra.sh`: all pass.
- `task --force build:discs`, then `go test ./tests`: static programs still work, and goldens differ only in printed readouts, named for the spec side.
- The image ends below 80068000h. Say early if it won't fit, since the spec side lays it out.
- Fidelity builds near spec 25's disc-layout limit still work: `tunnel-clean`'s `MAIN.EXE` is 196,608 bytes and fails at 204,800.
- `costs.txt` refreshed, with before and after.

## What the work on this spec found

`same` went from 98 to 102 of 140. `test-demo-fm`, `pssoft-koro-koro`, `scee-topten-adven` and `test-wme-wme` reached it. `event-event001-rave096` is 52 bytes off, and `scee-topten-cardem`, `pssoft-airbob-2-airbob` and `test-pd961207` still differ. The table's leads were wrong in two places: `init` was 566,082 cycles long, not short, and `airbob` and `pd961207` aren't mainly paced math.

- **Start-up:** Sony's start runs InitCard and StartCard inside a critical section, so a blank during FlushCache waits until StartCard's own exit, and the card driver's first frame is that blank.
- **The tick follows voice state:** 18 cycles more per voice that has read envelope 0 for a long time, from a count starting at 2 and topping out at 16, so a program's first 14 ticks are 432 cycles shorter. A live voice takes the short path, and a written voice costs 42 more. This is the kernel-side interrupt difference left open in spec 31.
- **printf's printability** comes from a class table indexed by the signed byte. Below it lies live memory: A3h to E8h are the two pad buffers, and 93h to 96h a word GsInitGraph sets. So A4h to A6h and C7h aren't special bytes, as earlier specs said, but the pad buffers' current contents.
- **`_dbl_shift`** shifted the wrong way. Spec 17 is wrong: a1 is the direction, 0 left and logical at 28 cycles a turn, nonzero right and arithmetic at 24.
- **MulMatrix0's padding** is the sign, not 0. ScaleMatrix writes its last element as a word, and GsGetLws's ls matrix leaves the same sign word.
- **Krom2Tim** is nearly flat by character on Sony's.

Fits, not causes: `padPow`, `ssVabTransferPad` (from one program; Sony's interrupt for the transfer's end holds the machine about 22,000 cycles, not modeled), `kromCharTime`'s ranges (from koro's strings), `tickWordsTime`'s coefficients, `enqueueNops = 13` (26 cycles lost and not explained), the start-up pads, and spec 31's sin and sqrt.

Not explained: `__divdf3`'s spread beyond quotient bits, which stops `test-pd961207`; GsSwapDispBuff's 8 cycles between buffers; class table bytes outside 93h to E8h. Not verified: ScaleMatrixL's word, pow with negative x or y of 0, Krom2Tim's other ranges and Krom2Tim2, putchar's tab time, and VSync(1) before any library start on a static build.

What stops the others: `scee-topten-cardem` and `pssoft-airbob-2-airbob` are held back by the image stub's `j` and `nop` on entries called very often (`bcopy`, GsGetActiveBuff, GsSetWorkBase), which is the spec side's, and `airbob` also by `__addsf3`, up to +200 a sum.
