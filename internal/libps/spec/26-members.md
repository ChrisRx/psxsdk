# The start-up and the linked members

Sony's code still reaches a program in two ways besides the resident image, and this replaces both so that it can leave the
repository. Sony's versions are still here while you do this, off limits to you as ever, and probes compare with them.

A program for the resident library links, into itself:

- **The start-up**: `_start`, `__main` and `exit`, with the data `_sys_ramsize`, `_sys_init_stacksize`, `_sys_init_sp`,
  `_sys_init_fp`, `_sys_init_gp`, `_sys_init_heapbase`, `_sys_init_heapsize` and `_sys_init_bss_flag`.
- **The members**: `puts`, `SetVideoMode`, `Krom2Tim2`, and the float routines `__eqsf2`, `__nesf2`, `__ltsf2`, `__lesf2`, `__gtsf2`,
  `__gesf2`, `__negsf2`, `__subsf3`, `__fixsfsi`, `__fixunssfsi`, `__fixunsdfsi` and `__floatsisf`.

The first three members call into the resident library: `puts` the kernel's or library's character output, `SetVideoMode` the
resident half at `SetVideoMode_orig` and SsSetTickMode, `Krom2Tim2` the resident's Krom2RawAdd2. The float routines call nothing
resident, and **the static target `playstation-yaroze-clean` links them too**, so it needs them as well. It has its own start-up
(spec 07) and its own `puts`, `SetVideoMode` and `Krom2Tim2` in the library.

## What to write

Two new files in `internal/toolchain/_tinygo/targets/`:

- `playstation-yaroze-resident.S`: the start-up for a program on the resident image. Spec 07 says what the start-up does, and
  `playstation-yaroze-clean.S` is your own version of it for the static target. Share what you can. The resident's InitHeap is reached
  by its entry point here.
- `playstation-yaroze-members.S`: the members, in sections of their own so `--gc-sections` can drop them. The first three must be
  left out under `LIBPS_GO`, as the static target has its own. Assembly, or C in the target's C files if a routine suits it.

Then wire them, all of which is yours to change:

- A third new file, `playstation-yaroze-start.S`, which includes `playstation-yaroze-clean.S` under `LIBPS_CLEAN` and otherwise
  `playstation-yaroze-resident.S`.
- In `playstation-yaroze.json`'s `extra-files`, `playstation-yaroze-start.S` in place of `playstation-yaroze.S`, and
  `playstation-yaroze-members.S` in place of `playstation-yaroze-libps.S`.
- In `internal/toolchain/c.go`'s `cTargetFiles`, the same two, which is what a C project links. Keep the crt0 first.
- `exp/cleanlib/image/gen.py` is not yours: say if the image needs anything of it.

A change to a target file reaches `yaroze` only through `task compiler:overlay && task --force playground:sysroot && task yaroze`,
about 3 minutes. `exp/cleanlib/image/make.sh` and `exp/cleanlib/build.sh` copy the target files themselves.

## What to match

Results first, then time in cycles (`cyc` in `PROBE_WATCH`, `TestArchiveCalls`, `TestArchiveCosts`; spec 25 on why cycles). Measure Sony's
versions **before you change the JSON**: `exp/cleanlib/image/probe.sh` builds a probe with them, and it won't afterward. Keep what you
measured in a table in your report.

- The start-up: the time to `main` and to each call it makes is part of every program's boot, so keep it to the cycle, as spec 07 did.
  Its data's values, read by exact name, and what a program sees if it sets them. `exit` and a `main` that returns.
- `puts`: spec notes in `exp/cleanlib/kernel.go` and spec 15 say the resident's own is gated on a word nothing sets. Find what the linked
  one prints and returns. A null string.
- `SetVideoMode`: its result, what it sets, SsSetTickMode's effect in both modes.
- `Krom2Tim2`: as Krom2Tim in specs 13 and 23, with its own arguments. Probe the differences.
- The float routines: every class of input, on a sweep with edge cases: zeros of both signs, denormals, infinities, NaNs, rounding,
  overflow and the conversions' limits. Sony's results decide, and may not be IEEE: match them, including any odd NaN or denormal
  handling. These are short and called often, so match their cycles per path too.

## Acceptance

Nothing Sony's should be linked into a program after this: `nm` on a built program's ELF shows only your start-up and members, and the
JSON names neither old file. Then:

- Your probes of each routine, Sony's before against yours after.
- The full `exp/cleanlib/image/archive.sh`: 98 same, 27 differ, 9 noboot, 3 shift, 1 wild, 1 tty, 1 stall before. It runs the same
  programs on both images with their own start-ups, so it shouldn't move, and anything that does needs explaining.
- `exp/cleanlib/image/regress.sh`, where both sides now carry your start-up and members: FAIL stays 0. Before: 33 PASS, 8 TIME.
  `exp/cleanlib/regress.sh` without `PSX_BIOS` in the environment: FAIL stays 0. Before: 33 PASS, 8 TIME.
- `exp/cleanlib/image/terra.sh`: all pass.
- `go test ./tests`, after `task build:serial`, which covers the serial monitor's programs.
- A C project builds and runs: `internal/cproject`'s tests and whatever `tests` has for it.

## What the work on this spec found

Probes of Sony's versions by the agent that did the work. On Sony's image the clean ones match in result and cycles on every sweep, 644,576 float calls
among them, with the exceptions below.

- **puts** adds no newline, leaves v0 and v1 alone for "", prints `<NULL>` for null, and returns the last putchar's result.
- **`__fixsfsi(-0)`** returns 80000000h. Positive overflow reports an error and negative overflow doesn't. **`__fixunssfsi` and `__fixunsdfsi`** report EDOM for every
  negative input, -0 included, and return the negated magnitude. **`__floatsisf`** rounds half up. **`__negsf2(-0)`** returns +0.
- **The start-up**: its -1 sentinels work per word, `_sys_init_heapsize` is overwritten, and an empty `.bss` still has one word written at `_fbss`. **exit** ignores its
  status and goes back to the BIOS with interrupts on.
- **Krom2Tim and Krom2Tim2** stop at 16 rows, with 4 more cycles if the string goes on. Spec 13 said no limit was met.
- **SetVideoMode and Krom2Tim2 are weak**. A C program defining any other member gets a duplicate-symbol link error, as on Sony's.
- A single-precision sum below the range is an infinity of its sign plus `_err_math(34)`.
- `__subsf3` calls `__addsf3` and the conversions call `_err_math`, against this spec. There is no `puts` in `exp/cleanlib`, also against it. C projects link
  `playstation-yaroze-symusr-orig.S` as well, for `SetVideoMode_orig`.

**Not matched, on purpose.** `__floatsisf(-2^31)` hangs on Sony's, at least 3,000 frames with interrupts off, and the clean one returns CF000000h: Sony-only crashes are not matched.
Krom2Tim2's stack frame is 73,872 bytes on Sony's and 56 on the clean one, so its stack garbage and any overrun past a 64 KB stack differ. A program's `main` returning runs the
member's own `syscall` where Sony's goes through a resident entry: the time is equal, but the BIOS's "End of Main" dump prints a different epc and hi.

**On the clean image**, none of it in these files: `__addsf3` +50 to +190 a sum, InitHeap 14 short, `_err_math` +10 to +18, Krom2RawAdd2 about +218 per ASCII glyph, SetVideoMode +24 to
+32, puts +14 a character. Not explained: the shape of Sony's Krom2Tim2 end time, fitted and paced to a formula that holds on every case tried.
