# The small Gs calls take Sony's time on a console

## Why

Rerun on Sony's library on 2026-11-10 (spec 35, "four Sony runs were the clean library"), `probes/callcost` showed the clean library much cheaper than Sony's in its small calls. `GsSortObject4` was within a few percent. In eighths of the clock, on the console:

| Call | Sony's | Clean |
|---|---|---|
| `GsSortSprite`, rotated | 157 | 104 |
| `GsSortSprite`, plain | 52 | 32 |
| `GsGetLws` | 85 | 70 |
| `GsSetLightMatrix` | 53 | 43 |
| `GsSortClear` | 31 | 24 |
| `RotMatrix` | 34 | 35 |
| `GsSortObject4`, the seven kinds | 4,159 to 6,955 | 1% less to 6% more |

The runs are in `.build/hardware/callcost-sony2` (Sony's) and `callcost2-clean`. The call's time is part of what a program sees: a frame's budget, and where the next blank lands.

## What to do

Make each of these calls take Sony's time on a console, within an eighth where the call allows, without changing what it computes or writes. Use the emulator's cost model (`PROBE_CPUCOST=1`), which was calibrated against the console in specs 45 and 46. Find where Sony's time goes, from outside: its device and memory accesses in a bus trace, and its time between them. Then pace the clean call the same way, so the console agrees, not only the model.

Leave `GsSortObject4` alone unless a change shared with these calls moves it. Leave `RotMatrix` alone: it is within an eighth. Spec 47 works on the image queue (`LoadImage`, `DrawSync` and the DMA handler) in parallel, so don't touch that code.

## Testing it

`probes/callcost` in the emulator with the cost model on, against Sony's image, for each call. Then on the console, which the spec side runs: `-library sony-ourmon` for Sony's side, and the candidate image under `-library clean`.

## Results

Done 2026-11-11 in the emulator (implementation side). Not tried on a console. The candidate image is `.build/console/libps-s48a.exe.gz`.

### The model against the console, Sony's side

`probes/callcost` under the model (`PROBE_CPUCOST=1`), in eighths, against the console's `callcost-sony2`: `GsSortSprite` rotated 151 / 157, plain 53 / 52, `GsGetLws` 88 / 85, `GsSetLightMatrix` 53 / 53, `GsSortClear` 32 / 31. All within 4%, so the model was taken as it is. (`RotMatrix` is 28 / 34 there, 18% under, but it was not to be touched.)

The clean library's console run (`callcost2-clean`) is an image of 2026-11-10 whose layout is not today's, and the model on today's image gives 95 / 104, 33 / 32, 75 / 70, 43 / 43, 34 / 24. The spread is the instruction cache: in this probe the call runs after `resetPackets` (`GsSetWorkBase`, `GsClearOt` and `spin`), and where those share cache lines with the call, it misses every time. On today's image the clean `GsSortClear` shared lines 61h to 6Fh with `GsClearOt` and paid some 145 cycles of misses a call; the console's image evidently paid less. So the clean figures on the console are a measure of the layout as much as of the call (see below).

### Where Sony's time goes

`image/probetest/calltrace_test.go` (new, `TestCallTrace`, `-calltrace` a callcost disc, `-calltrace-side clean|sony`, `-calltrace-cost`) follows each call the probe makes into the named entries, from the entry to the return, with the cycles of each GTE instruction and device access from the entry. On Sony's side it reads no instruction and logs no PC; on the clean side it logs each instruction with its cost and function, and which other code ran in the call's cache lines since the call before. Warm calls (the probe's least of five), cycles, costs off then on:

| call | Sony's, off / on | clean before, off / on | clean now, off / on |
|---|---|---|---|
| `GsSortSprite` rotated | 870 / 1,189 | 870 / 740 | 870 / 1,190 |
| `GsSortSprite` plain (GsROTOFF) | 298 / 410 | 306 / 249 | 298 / 411 |
| `GsGetLws` | 494 / 680 | 494 / 580 | 494 / 681 |
| `GsSetLightMatrix` | 354 / 418 | 354 / 336 | 354 / 414 |
| `GsSortClear` | 162 / 234 | 162 / 245 (misses) | 162 / 233 |

With the costs off the clean calls were already at Sony's cycle, GTE command for GTE command, but the plain sprite, 8 over. Under the costs they were short because their pads are nops, 2 cycles in the emulator and 1 on a console, where Sony's spends the same emulator time on instructions a console prices higher. The GTE timelines under the costs place it: in the rotated sprite Sony's first `cfc2` comes at 411 (clean 270), its `ctc2`s 79 after it (33), the next three 40 after (19), the `lwc2`s 130 after (91), 12 from the last `lwc2` to RTPT and to RTPS (1: Sony's has more than a nop there), and 375 from the last GTE access to the return (207). In `GsGetLws` Sony's is 49 later to its first GTE access and 133 from its last to the return (clean 42), the clean `libps_compose` between being some 40 slower, after `COPYMATRIX`'s stores. In `GsSetLightMatrix` Sony's is 70 later to its first access and 24 from the last to the return (8).

### What changed

The pads now hold loads from the stack where Sony's takes longer under the costs, each in place of nops of the same emulator time, so that with the costs off every call is at the cycle it was and with them on it takes Sony's time (`playstation-yaroze-clean.S`):

- `LOADS n`: n pairs of a load into `$zero` and a nop, for two nops: 4 cycles in the emulator, 8 on a console (cpucost's `lw ram nop` 8.1). `LOADRUN n`: n loads in a run into `t1` to `t8` in turn, for n nops, where those are free: 2 cycles each in the emulator, 6 on a console (`lw ram` 6.1). `LOADNOP`: a load into `$zero` for the nop between the last `lwc2` and RTPT or RTPS.
- `GsSortSprite`, the quad: 16 pairs in its lead, 8 in the turn's pad, 8 after the `cfc2`, 2 in the next stretch, 8 before the `lwc2`s, the two `LOADNOP`s, and a run of 34 at its end, each stretch placed to bring the GTE accesses to Sony's cycles (rotated, first access 409, the `ctc2`s 499, the `lwc2`s from 665, RTPT 716, the last 802: Sony's 411, 490, 664, 718, 814). The sprite: 24 pairs in `SPRITE_PAD`; the GsROTOFF path's 8 nops are 4 loads in a run, which also takes its emulator time to Sony's 298.
- `GsGetLws`: 3 pairs before `libps_ltw`, 5 before `libps_compose`, 8 after it. `libps_ltw` and `libps_compose` are untouched, so `GsGetLw` and `GsGetLs` are as they were.
- `GsSetLightMatrix`: 11 pairs in its lead, 2 at its end.
- `GsSortClear` is now in the assembly. As Go it could not be paced: its own instructions are 146 of its 162 emulator cycles and cost some 100 on a console, so no swap of its 9 pad instructions reaches 234. The assembly does the same (the screen's size is the word at `libps_cut_limits + 8`, which `updatePlace` keeps with `GsInitGraph`'s; the packets are `libps_clear_packets`) in some 50 instructions and pads with a loop of four loads a turn (12 cycles in the emulator, 27 on a console), four turns, two more loads and a `LOADNOP`. It is in the section `.text.GsClearOt`, so that the image's packing puts it next to `GsClearOt`, which programs call before it every frame, and neither can evict the other from the instruction cache. Placed by size, it landed on `GsClearOt`'s lines in two of three layouts tried (318 cycles under the costs where 233 was meant). Cold, the first call, it takes 336 where Sony's takes 364. `gs.go` declares it, and the Go `clearPackets` is gone. `sortClearPad` in `gpu.go` is left unused, since spec 47 works in that block.

`callcost` under the model now, Sony's / clean, eighths: rotated 151 / 152, plain 53 / 54, `GsGetLws` 88 / 88, `GsSetLightMatrix` 53 / 53, `GsSortClear` 32 / 31. `RotMatrix` 34, `TransMatrix` 5 and `GsSetLsMatrix` 10 are as they were. `GsSortObject4` moves by the layout alone: `lg4` 7,247 to 7,236 and `gt4` 6,196 to 6,208, the rest the same.

What the console should make of it: if the model holds for the clean code, the five calls at Sony's console figures (157, 52, 85, 53, 31) within an eighth or two. The layout is the open risk. The rotated sprite still misses two or three lines a call in this probe (`spin` and the `GsSetWorkBase` stub fall on its lines), some 30 cycles of the 1,190, and Sony's misses none, so another layout could take that much off it. An image packed again for any other change (spec 47's) moves every call but `GsSortClear` against what runs before it.

Found on the way, not changed: in 24-bit color `GsSortClear` takes the width times 3 over 2 in 16 signed bits (the Go's arithmetic, kept), where Sony's does not: a width of 11,000 gives 16,500 on Sony's and C074h on the clean library, 21,845 gives 32,767 and 0. No real screen is that wide. Which width arithmetic Sony's uses (unsigned 16 bits or 32) is for the spec side, with a width over 21,845.

### Checks

`go test ./emu/... ./exp/cleanlib/...` pass. `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`). `TestDrawWait`, `TestFieldTime`, `TestVSyncCount`, `TestVSyncPhase`, `TestIRQGaps` and `TestVSyncPoll` pass on the new image's discs. `probes/sortclear` and `probes/gsclip` print the same on both libraries, and a check of `GsSortClear` in 24-bit color at eight sizes agrees but for the two widths above. The static clean build assembles with and without `LIBPS_FIDELITY` (jumpcube). The embedded sysroot (`cmd/yaroze/wasm`) still has the old assembly, which has no `GsSortClear`: it needs `task yaroze` before the embedded compiler builds the clean library again.

### On the console (to do, spec side)

1. `callcost` (`.build/serial/callcost.elf`) under `-library clean` on `.build/console/libps-s48a.exe.gz`: the five calls against `callcost-sony2`.
2. The same ELF under `-library clean` on `.build/console/libps-s48-base.exe.gz`, this tree's image before the change. Against the model's 95, 33, 75, 43 and 34 it says whether the model prices the clean code as the console does in today's layout. In particular, `GsSortClear` at 34 says the console misses on `GsClearOt`'s lines as the model does. If (2) agrees with the model and (1) does not with Sony's, the loads cost the console other than the model says (a load into `$zero` is the untested case), and a probe of `LOADS` and `LOADRUN` beside cpucost's rows is next.
3. Optionally the same ELF under `-library sony-ourmon`, for Sony's figures in the same session.

### The console on s48a (spec side's runs, 2026-11-11, `.build/hardware/spec48`)

Eighths, Sony's (`sony-ourmon`) / s48a / s48-base: rotated 156 / 148 / 93, plain 52 / 52 / 32, `GsGetLws` 84 / 85 / 71, `GsSetLightMatrix` 53 / 51 / 43, `GsSortClear` 31 / 29 / 32, `RotMatrix` 34 / 35 / 35. `GsSortObject4` did not move (g4 +6% and lg4 +4% over Sony's on both clean images).

### Second round: the console's figures against the model (implementation side, 2026-11-11)

**The model prices the clean code as the console does, and the loads too.** s48-base on the console against the model: 93 / 95, 32 / 33, 71 / 75, 43 / 43, 32 / 34. From s48-base to s48a the loads added, console / model: rotated +55 / +57, plain +20 / +21, `GsGetLws` +14 / +13, `GsSetLightMatrix` +8 / +10, `GsSortClear` -3 / -3. So LOADS, LOADRUN and the load into `$zero` cost the console what the model says, to an eighth or two over some 80 loads, and the clean calls run some 2 eighths under the model on the console, before the change and after alike.

**Sony's rotated sprite is what the model misses.** The model has it at 151, the console at 156: Sony's spends some 40 cycles there that the model does not price, and the clean pads, set to the model's 151, came out at 148 for the console's 156. `RotMatrix` is the same on a larger scale: 28 in the model and 34 on the console for Sony's, where the clean one is 34 and 35. The model gives every multiply the one time cpucost measured with small operands (`costMult`), and the R3000A is documented to take longer for larger ones, by the first operand. Tried in the model (not kept, `emu/cpu/cost.go` is unchanged): 6, 9 or 13 cycles to HI and LO by whether rs fits 11, 20 or 32 bits, signed, takes Sony's rotated sprite to 155 and Sony's `RotMatrix` to 34, both the console's within an eighth, and changes no other row of Sony's; by rt instead, 154 and 33. The clean `RotMatrix` goes to 37 under either (35 on the console), so the rule is near but not exact. `probes/padcost48` below asks the console.

**The library** (`playstation-yaroze-clean.S`), console time added from the s48a figures, the emulator's time with the costs off the same:

- `LOADCYC n, r1, r2, r3, r4` (new): LOADRUN into four named registers in turn, where t1 to t8 are not all free.
- `GsSortSprite`'s quad: its lead's 16 pairs are a run of 32 loads into t4, t7, t8 and t9 (`QUAD_LEAD_R`), 65 more under the model, some 8 eighths.
- `GsSetLightMatrix`: three of its lead's nops are a run of loads into t4 to t6 (`LM_LEAD_R`), 16 more.
- `GsSortClear`: its loop is two turns of eleven loads (`CLEAR_TURNS` 2) where it was four of four, the same instructions run, 18 more.

Under the model now, cycles (eighths): rotated 1,255 (160), plain 411 (54), `GsGetLws` 681 (88), `GsSetLightMatrix` 430 (55), `GsSortClear` 251 (34); with the costs off 870, 298, 494, 354 and 162, Sony's. The model has the clean rotated sprite 9 eighths over Sony's now, on purpose: it is the console's figure that is meant. With the console's 2 eighths under the model for the clean code, the console should show about 156, 52, 85, 53 and 31. The packing moved `GsSortClear` and `GsClearOt` (together, as before) to another gap; the sprite's misses in this probe are the ones s48a had.

**`GsSortClear`'s width in 24-bit color.** `probes/sortclear24` (new, `.build/serial/sortclear24.elf`) prints the fill's size word for widths 320 to 65,535 at 24-bit and 16-bit color. In the emulator, which runs Sony's own code and does integer arithmetic exactly, Sony's takes the width as unsigned 16 bits, times 3 in 32 bits, halved, and keeps the low 16 bits of that: 11,000 gives 16,500 (4074h), 21,845 7FFFh, 30,000 AFC8h, 32,767 BFFEh, 32,768 C000h, 43,691 0, 50,000 24F8h, 65,535 7FFEh. The clean library's signed 16 bits differ from 11,000 to 32,767 (C074h, 0, 2FC8h, 3FFEh) and agree elsewhere. Left as it is until the console has run the probe, as asked; the change would be `GsSortClear`'s seven 24-bit instructions.

**`probes/padcost48`** (new, `.build/serial/padcost48.elf`): rowcost's frame with the pads' patterns and multiplies by operand size. Under the model (warm, `empty` 5): `nop` x32 37; `lw zero nop` x16 133 and `lw v1 nop` x16 133 (8 a pair); `lw zero` x16 116 (7 each); `lw run` x16 101 (6 each), then a nop 103; every multiply row (8 of mult or multu and mflo, rs and rt from 7 to 7FFFFFFFh, by sign, and multu with the top bit) 69, 8 a pair.

### Checks, second round

`go test ./emu/... ./exp/cleanlib/...` pass (`emu/cpu/cost.go` is as it was). `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`). `TestDrawWait`, `TestFieldTime`, `TestVSyncCount`, `TestVSyncPhase`, `TestIRQGaps` and `TestVSyncPoll` pass on s48b's discs. `probes/sortclear` and `probes/gsclip` print the same on both libraries. The candidate is `.build/console/libps-s48b.exe.gz`.

### On the console (to do, spec side)

1. `callcost` (`.build/serial/callcost.elf`) under `-library clean` on `.build/console/libps-s48b.exe.gz`: expected about 156, 52, 85, 53 and 31.
2. `padcost48` (`.build/serial/padcost48.elf`) on either library: the pads' rows against the model's above (they agreed in the round before, in sum), and the multiply rows. If a multiply's time grows with rs as documented, the model takes the rule (`costMult` by rs's width) and Sony's rotated sprite and `RotMatrix` come to the console's figures in the emulator too; then the clean rotated sprite's pads are set under it again, and the clean `RotMatrix` looked at.
3. `sortclear24` (`.build/serial/sortclear24.elf`) under `-library sony-ourmon`, for the width arithmetic: the emulator's answer above is expected, after which `GsSortClear` takes Sony's.

### The console on s48b, `padcost48` and `sortclear24` (spec side's runs, 2026-11-11, `.build/hardware/spec48/r2`)

`callcost` on s48b / Sony's: rotated 156 / 156, plain 52 / 52, `GsGetLws` 85 / 84, `GsSetLightMatrix` 54 / 53, `GsSortClear` 33 / 31, `RotMatrix` 35 / 34. `padcost48`, warm: `empty` 4, `nop` x32 35, `lw zero nop` and `lw v1 nop` x16 132, `lw run` x16 100 and 102 with a nop after, `lw zero` x16 100 (the model had 116). The multiplies go by rs alone: 65 for 8 with rs within 11 bits (and -800h), 89 within 20 (-801h to -100000h), 121 wider (-100001h, 7FFFFFFFh); multu the same with rs unsigned; rt nothing. `sortclear24` on Sony's gives what the emulator gave.

### Third round: the model's multiplies, and the last eighths (implementation side, 2026-11-11)

**The model** (`emu/cpu/cost.go`, with `TestCPUCostMult` and `TestCPUCostLoadsZero` in `emu/cpucost_test.go`):

- A multiply takes `costMultMid` (3) more with rs wider than 11 bits and `costMultWide` (7) more with it wider than 20 (`multWidth`), a negative rs for mult counting as its complement, multu's unsigned. `padcost48` under it, model / console: 69 / 65, 93 / 89, 125 / 121 by the three widths, every row in its place; the model's 0.5 a multiply over the console is the one it had before.
- A load into `$zero` after one into `$zero` no longer costs the next-load cycle: `lw zero` 101 / 100.

Under it, Sony's in the emulator against the console: rotated 155 / 156, plain 53 / 52, `GsGetLws` 88 / 84, `GsSetLightMatrix` 53 / 53, `GsSortClear` 32 / 31, `RotMatrix` 34 / 34 (it was 28), `GsSortObject4` unchanged. The clean `RotMatrix`, whose operands are sines and their products (12 to 13 bits), goes to 37 under the model (console 35); the clean rotated sprite has no multiply on this path and stays at 160 (console 156).

**Not re-paced.** The rotated sprite is at Sony's on the console, 156, and the model has every clean call some 2 to 4 eighths over the console (as for s48-base and s48a), so setting it to the model's 155 would take it under on the console. `RotMatrix` is an eighth over Sony's on the console and has no pad to take time from: its 14 multiplies are all there is, and making the time up elsewhere would move the emulator's time with the costs off. Both left as they are.

**The library** (`playstation-yaroze-clean.S`):

- `GsSortClear` in 24-bit color takes Sony's arithmetic: the width as unsigned 16 bits, times 3 in 32 bits, halved, the low 16 bits kept, with four nops for the four instructions it no longer needs, so that the way takes as long as before. `sortclear24` prints the same on both libraries now.
- `GsSortClear`'s loop has ten loads a turn and a nop in the delay where it had eleven, and the nop before the entry's reload is a nop again: 16 cycles less under the model (251 to 235), for the console's 2 eighths over.
- `GsSetLightMatrix`'s run of loads in its lead is one (`LM_LEAD_R` 1) where it was three: 10 less (430 to 420), for its eighth over.

The emulator with the costs off has every call where it was (870, 298, 494, 354, 162). Under the model now, eighths: rotated 160, plain 54, `GsGetLws` 88, `GsSetLightMatrix` 54, `GsSortClear` 32, `RotMatrix` 37. On the console expected: 156, 52, 85, 53, 31, 35. The image is `.build/console/libps-s48c.exe.gz`.

### Checks, third round

`go test ./emu/... ./exp/cleanlib/...` pass, with the two new model tests. `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`). `TestDrawWait`, `TestFieldTime`, `TestVSyncCount`, `TestVSyncPhase`, `TestIRQGaps` and `TestVSyncPoll` pass on s48c's discs under the corrected model. `probes/sortclear`, `probes/gsclip` and `probes/sortclear24` print the same on both libraries.

### On the console (to do, spec side)

1. `callcost` (`.build/serial/callcost.elf`) under `-library clean` on `.build/console/libps-s48c.exe.gz`, and under `-library sony-ourmon` in the same session: expected 156, 52, 85, 53, 31 and `RotMatrix` 35 on s48c, Sony's as before.
2. `sortclear24` (`.build/serial/sortclear24.elf`) under `-library clean` on s48c: Sony's sizes (4074h, 7FFFh, 8001h, AFC8h, BFFEh, C000h, FFFFh, 0, 24F8h, 7FFEh from 11,000 up).
