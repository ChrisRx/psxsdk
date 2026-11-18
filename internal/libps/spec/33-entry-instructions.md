# Two entries that begin with a branch

A call through the resident image costs 4 cycles more than a routine's own time: a jump and the instruction in its delay slot. The spec side's layout now puts a routine's first instruction in that slot for routines 2 cycles too slow, which takes off 2. That can't be done with a first instruction that's a branch or a jump, since one can't sit in a delay slot, and two routines begin with one:

| Routine | Calls in the archive's survey | Sony's cycles a call | Clean |
|---|---|---|---|
| `bcopy` | 6,208 (36,306 in `pssoft-airbob-2-airbob` alone) | 2,970 | 2,972 |
| `GsInitCoordinate2` | 33,050 | 54 | 56 |

Make each one's first instruction something other than a branch or jump, without changing its result or its time on any path, on the image or on static builds. Say if that can't be done for one of them. The spec side then adds them to the layout, and they should come out at Sony's time.

Don't try to shorten them by 2 cycles yourself: the layout does that. Their times will read the same as now until it does.

## Acceptance

Time by cycles only.

- Each routine's first instruction, by `llvm-objdump` on `.build/cleanimage/image.elf`, the clean symbols, isn't a branch or jump.
- A probe of each across its paths, by arguments, shows the same cycles before and after: `bcopy` for length 0, short and long, in either direction if it has them, and `GsInitCoordinate2` with `super` nil and not nil.
- `exp/cleanlib/regress.sh` without `PSX_BIOS`, `exp/cleanlib/image/regress.sh`: 33 PASS and 8 TIME, and 34 PASS and 7 TIME. `go test ./tests` passes.

## What the work on this spec found

`GsInitCoordinate2` is assembly on the image and stays Go on static builds, which inline it. Both begin with a non-branch now. `bcopy` with a null source can't keep its time and got 4 to 6 cycles slower: 12 and 14, against Sony's 4. On Sony's call a nil `base` writes to memory at 0, and now the image does too.

Moving a first instruction into the jump's delay slot takes 2 cycles off either kind: a load gains as `lui` does, since the emulator charges only the instruction fetch. `bcopy` now reads 10, 202 and 3,082 at lengths 0, 16 and 256, Sony's times. An earlier version of this section said a load gains nothing, from a misread of a stale probe output.

`GsInitCoordinate2`'s mean hid errors in opposite directions. It now takes Sony's 46 with `super` nil and 58 with one.

`bcopy` with a null source stays at 10 and 12 against Sony's 4. It needs at least 5 instructions and only 4 fit Sony's time through the entry, and testing null first would slow the n<=0 path, which programs do use.
