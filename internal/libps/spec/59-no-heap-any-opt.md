# No heap at any optimization level

## Why

Nothing allocated on this target is ever freed, so the library must not allocate after start-up. `TestImageNoHeap` (spec 44) checks the resident image, which is built at one optimization level. A program that links the library statically is built at whatever level it asks for. `yaroze build` defaults to `--opt=2`, but `--opt=s` and `--opt=z` are allowed, and the playground builds at `-opt=z`.

At `-opt=z` the library allocates. Here is a static build of `docs/tutorial/hello` (`go run ./cmd/yaroze build --opt=z -o OUT.elf docs/tutorial/hello`), disassembled, with the callers of `runtime.alloc` listed:

| | calls to `runtime.alloc` |
|---|---|
| `(*sequence).noteVolume` | 5 |
| `libps_interrupt` | 4 |
| `main` (the program's own, at start-up) | 4 |

At `--opt=2` the same program's only callers are `main`'s 4.

The source lines a new compiler check points to are:
- `seq.go`: `noiseVolume`'s `side := func(...)` closure,
- `sndut.go`: `voiceVolume`'s `side := func(...)` closure,
- `gpu.go`: `lateInterrupts`' call of `libps_interrupt`.

From these, the closures look to be what allocates, and the interrupt handler reaches them through the sequence's tick. That makes it a leak on every note played, and an allocation inside the interrupt handler.

The check comes from main's compiler. It refuses a program whose frame loop allocates each time round, and it is why main can't be merged into `libps-go` until this is fixed. You don't have it in this tree, so use the disassembly instead.

## What to do

1. **Find every allocation in the library** at `--opt=0`, `1`, `2`, `s` and `z`, not only the ones above. To cover the whole library, not just what one program reaches:
   - Build the image with `image/make.sh` at each level if it can take a level. If it can't, add a way to `make.sh`'s build that leaves the default as it is.
   - Look for `runtime.alloc` in its disassembly. Use `llvm-objdump -d` from the nix store's LLVM 22.
   - A start-up allocation the library makes once is allowed only where `TestImageNoHeap` already allows it at the default level.
2. **Remove each allocation** without changing what the code does. Closures that capture variables are the likely cause. A plain function, or passing the values as parameters, does the same job without one.
3. **Keep the default build the same where you can.** Check whether the image at the default level changes. If its code changes, run the checks below and say what moved.
4. **Add a test** like `TestImageNoHeap` that fails if the library allocates at any of the levels in step 1. Builds take time, so the test may take a built image's path or skip when one isn't there, as `TestImageNoHeap` does.

## Testing it

- the new test at every level,
- `TestImageNoHeap`,
- the ten timing tests (`.build/s59/tests.sh`),
- `image/regress.sh`: 38 PASS, 3 TIME, 0 FAIL before,
- `docs/tutorial/hello` built statically at `--opt=z` and at `--opt=2` with no `runtime.alloc` call outside `main` and the runtime.

Say what the image's SHA-1 is at the default level against `32e80acc` before the change.

## Results

### What allocated

The whole library, built at each level and disassembled (`jal` to `runtime.alloc`), before the change:

| level | functions that call `runtime.alloc` |
|---|---|
| 0 | see "Level 0" below |
| 1 | none |
| 2 | none |
| s | `CdRead` (1 call) |
| z | `CdRead` (1), `vblankInterrupt` (4, from `noiseVolume` through the tick, inlined), `voiceVolume` (5) |

The causes at s and z:

- `voiceVolume` and `noiseVolume`: the `side` closures. Each captured variable is a heap cell where the compiler does not inline the closure.
- `CdRead`: `m`, the mode byte, passed by address to `cd.control`. This one is not a closure, and it allocates on every `CdRead` whose mode differs from the drive's.

The compiler's IR shows two more at every level, which only LLVM's later passes removed: `for i := range seqs` in `SsSeqOpen` and `stopSequences` loads the whole array (5,248 bytes) into a heap cell, since the range expression is evaluated even when only the index is used.

### What changed

- `sndut.go`: `voiceVolume`'s closure is the function `voiceSide`, with every value a parameter.
- `seq.go`: `noiseVolume`'s closure is the method `noiseSide`. The two `range seqs` loops are `range &seqs`.
- `cd.go`: the driver has a `readMode` byte, in the padding after `mode`, so no field moves. `CdRead` stores the mode there when it has a Setmode to send, and passes that address. It cannot pass `&cd.mode`: the command may start the drive first, which sets `mode` to 0.
- `cdfile.go` (`CdSearchFile`), `light.go` (`matrixWords`), `strtod.go` (`parseFloat`): closures that captured a variable are plain functions (`pathByte`, `matrixHalf`, `strByte`). These allocated at level 0 only, and are the same pattern that allocated at z.
- `image/make.sh`: `OPT=N` builds at another level onto `.build/cleanimage-optN/image.elf`. Such a build stops after the first link, unpacked, with no `libps.exe`: the pins hold level 2's sizes, and at level 0 the packed link script has section names the linker refuses (`{` in a name, from gen.py). At level 0 it also gives the link break instructions under nine names the runtime's unused float32 printing and unwinding want (`tinygo_longjmp`, `__nesf2` and so on). With `OPT` unset the script does what it did: the image built from the old source is still `32e80acc`.
- `image/probetest/noheap_test.go`: `TestNoHeapAnyOpt`.

### The default image

`32e80acc` before, `e39432de` after. The image ends at 0x80066288 as before, and every pinned section fits. What moved:

- `CdRead` is 20 bytes longer (2E4h to 2F8h). On the path with no Setmode the instructions are the same with other registers. The Setmode path has five more: the store of `readMode`, two reloads of it and a constant.
- `vblankInterrupt` is the same size. In the noise note's volume the same instructions come in another order, with other registers. A plain function, a method and a closure that captures nothing all gave these same bytes, and passing the channel's pan as a value added four instructions, so the method reads it as the closure did.
- `CdRead` and some unpinned sections are packed at other addresses, since `CdRead` grew.
- The other source changes (`CdSearchFile`, `matrixWords`, `parseFloat`, `range &seqs`) change no byte. `range len(seqs)` did: `SsSeqOpen` came out 352 bytes shorter, so it is `range &seqs`.

### Level 0

The library cannot be free of allocations at `-opt=0`, whatever its source, and the test lists that level without failing it. At level 0 the compiler does not look at where a pointer goes. So:

- Every local whose address is taken is a heap cell. That includes the standard library's `math.Float64bits` and `math.Float64frombits`, which `f2u` and `u2f` call, and `math/bits.Div64` (`dDivQuotient`).
- The map of operands `device.AsmFull` takes is built at run time: 16 functions call `runtime.hashmapMakeGeneric` (`setBK`, `setColorMatrix`, `setLightMatrix`, `setDQ`, `setFC`, `setH`, `setOFX`, `setOFY`, `setZSF3`, `setZSF4`, `intOff`, `intRestore`, `leadingZeros` among them).

Besides those, these still call `runtime.alloc` at level 0 only:

| function | what is on the heap | why it stays |
|---|---|---|
| `callC`, `cCompare` | the local func value a C address is written into | no way in Go to make a func value from an address without the address of a variable; an assembly trampoline would change the callback's path at the default level |
| `LoadImage`, `StoreImage`, `startImage`, `sendImage`, `readImage` | the `transfer`, whose methods have pointer receivers | value receivers changed `LoadImage`, `StoreImage` and `drawNext` at the default level |
| `loadImage` | its `RECT`, passed to `LoadImage` by address | not tried: it wants a package-level rectangle |
| `soundTick` | `settled` and `live`, passed to `tickVoiceTime` by address | returning them made `vblankInterrupt` 188 bytes shorter at the default level |
| `padPow` | the double it reads the bits of | `math.Float64bits` allocates there too |

All of these are on paths whose times are matched to Sony's, so they were left for the spec side to decide. Before the change level 0 had 34 calls in 18 library functions. Now it has 11 in 10, plus the standard library's and the operand maps.

`yaroze build --opt=0` could not be tried on a program: the compiler's cache here holds the target's C and assembly at z and 2 only.

### The test

`TestNoHeapAnyOpt` in `image/probetest` reads `.build/cleanimage/image.elf` (level 2) and `.build/cleanimage-optN/image.elf` (0, 1, s, z), one subtest a level, and skips a level that is not built. It decodes every `jal` and fails for each library function that calls `runtime.alloc`, or a runtime or standard library function that reaches it. Calls through a register are not followed. Level 0 is logged and skipped.

    for o in 0 1 s z; do OPT=$o exp/cleanlib/image/make.sh; done
    go test ./exp/cleanlib/image/probetest -run TestNoHeapAnyOpt -v

On the old source it fails at s (`CdRead`) and z (`CdRead`, `vblankInterrupt`, `voiceVolume`).

### Checks

| check | result |
|---|---|
| `TestNoHeapAnyOpt` | pass at 1, 2, s and z; level 0 skipped with 45 allocating calls listed |
| `TestImageNoHeap` | pass |
| the ten timing tests, image `e39432de` | all ten pass |
| `image/regress.sh` | 38 PASS, 3 TIME (check, offscr, terra), 0 FAIL, as before |
| `docs/tutorial/hello`, static, `--opt=z` | `runtime.alloc` called from `main` only (4) |
| `docs/tutorial/hello`, static, `--opt=2` | `runtime.alloc` called from `main` only (4) |

Not run: the `GEN_SHIFT` sweep of `TestVSyncPoll`, which this spec does not ask for.

### For the spec side

- `CdRead`'s Setmode path is five instructions longer. If its time against Sony's matters to that precision, a CD read probe on both libraries would show it. No console run is needed for the rest.
- Level 0: decide whether it is a supported level. Making it clean needs compiler changes (inline assembly operands, `math.Float64bits`), not library ones.
- `gen.py` cannot write a link script for level 0's section names. Nothing needs it while non-default levels stop at the first link.
