# Fidelity by build

Matching Sony's time and leftover memory only matters on the resident image, where original programs run. A new program linked statically has no Sony's to match, and pads only waste its time. Until now the two have been told apart by a run-time flag, `resident`, in some places, and not at all in most.

This separates them **by build, with one function body**.

## The mechanism

A `fidelity` build tag, and `LIBPS_FIDELITY` in assembly. The resident image's target has them, and so does `playstation-yaroze-clean-fidelity`, which `exp/cleanlib/build.sh` and so `regress.sh` use. A program's static target, `playstation-yaroze-clean`, does not.

Everything that exists only to match Sony's time or leftover memory goes through small named operations, defined in one file per build, real under `fidelity` and empty otherwise, so that the compiler removes them:

```go
func Krom2Tim(...) int32 {
	clearBelowStack(73728)
	// the real work
	pad(krom2TimEnd)
}
```

- **No branches in function bodies**, no `if resident`, and the `resident` variable goes. Today's `delay(...)` calls and pad constants become these operations. Choose the names that read best.
- **They may affect only time and leftover memory, never a result** that a correct program depends on. If you find a pad doing more than that, it isn't a pad: keep the work and say so.
- **In assembly**, the macros (`G`, `GAP`, `STUBTIME` and the like) switch the same way, wherever a spacing exists only to match. Instruction order tuned to Sony's stays shared.
- A loop or order that only exists for accuracy stays shared unless it's a clear waste, and then say which. Don't rewrite anything to make it faster: that is a later stage.

## Krom2Tim's stack

Under `fidelity`, Krom2Tim and Krom2Tim2 clear the stack below them as Sony's does (spec 29): 73,728 bytes of a 73,872-byte frame on every successful call, 8 cycles a byte, in place of the pad that stands in for that time. Probe what's left in that memory and how deep. Static builds don't do it, so they can't overwrite the top of the heap. Sony's Kanji flush frame (spec 29) can be small on static builds if nothing there needs it.

`Krom2Tim2` is in `playstation-yaroze-members.S`, which is yours to change, and a program for the resident image links it whichever library image is under it.

## Acceptance

- `exp/cleanlib/image/make.sh`, then the image's behavior doesn't move at all: `exp/cleanlib/image/archive.sh`'s `.build/carchive/summary.txt` is the same line for line as `.build/carchive/summary-verify33.txt`, apart from what the deeper stack clear changes, each named with why. `exp/cleanlib/image/regress.sh`: 33 PASS, 8 TIME, 0 FAIL. `exp/cleanlib/image/terra.sh`: all pass.
- `exp/cleanlib/regress.sh`, without `PSX_BIOS`: 33 PASS, 8 TIME, 0 FAIL, since its build keeps fidelity.
- Static builds work, faster: `task --force build:discs`, then `go test ./tests`. Goldens may differ only in printed readouts of a program's own time: name each, and leave them for the spec side. Any other difference is a bug: something depended on a pad. Report it, with the program.
- Before and after for a few static programs (`hello`, `square`, `car`, `dino`, `dungeon`, `morph`): the size of `.build/NAME/NAME.bin`'s MAIN.EXE or the ELF's code, and the time from entering GsSortObject4, FntFlush and printf to their return, by `cyc` on a probe.
- Grep shows no `resident` variable, and no pad or `delay` call outside the fidelity file that isn't one of its operations.

## One more tag

Only the image's target has the build tag `cleanimage`. What differs between the image and the static fidelity build, the image's pads and `padReadingStart`, chooses by this tag, in a file of its own beside `fidelity.go`, and the run-time `onImage` and `imageStarted` go.

## What the work on this spec found

Krom2Tim's clear covers 73,784 to 56 below the caller's sp, and failed calls clear too, against the first section: a failed call's time goes from 100,796 to 706,090, with Sony's at 705,012. Above it each cell has 32 bytes holding its 30 ROM glyph bytes, 8,192 bytes in all, and the image sits at 65,592 below sp in rows of 16 cells, written only by successful calls. Krom2Tim2's frame copies Krom2Tim's, since Sony's member can no longer be run: its times match the old member's within 2 cycles, but its frame was not compared.

`padBegin`'s early return was more than a pad, and is kept as `padReadingStart`. Five nops were needed by loads or the GTE, and stay written out.

Static programs lose about 11% of their code. FntFlush takes from 1.5 to 1.9 times less and GsSortObject4 about 10 to 17% less. printf doesn't change: its time is the kernel's character output.

Not verified: Krom2Tim2 on the static fidelity build (`krom2Tim2Frame = 72` is unchecked), the small Kanji frame on static builds beyond the tests, the float members on static builds. On real hardware, `gso_lg4`, `lgt4`, `fg4` and `fgt4` have `lwc2 $1` right before NCCT or NCDT, which the emulator doesn't model.
