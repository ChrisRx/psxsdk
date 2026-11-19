# VSync(0)'s wait pass

## Why

At 240 lines `VSync(0)` returns some time after the vertical blank's interrupt ends, and how long depends on where its wait for the count is when the interrupt comes: just before its read of the count it leaves at once, just after it takes a whole pass more. So the spread of the return over the phases is the pass. Spec 45 matched the clean pass and return to Sony's with the costs off, where every instruction takes 2 cycles. Under a console's costs the clean pass is wider than Sony's.

`TestVSyncPoll` checks the return at 240 lines against 80 to 155 cycles with the costs on, a window drawn round both libraries' returns as the probe sampled them. The clean library's widest return is 154, at its edge. Spec 52 checked that the test passed with the rest of the packing shifted 16 to 128 bytes (`GEN_SHIFT`). Since spec 55 it fails at shifts of 32 and 64: the shift moves nothing on the VSync path (every section there is pinned, and its stores go to the same addresses, spec 56), but it changes the timing on the way into the waits, so the probe's waits land on other phases, and the first wait after the poll lands at the widest one and misses a line or two as well (167 and 174 cycles).

## What was measured (spec side)

**In the emulator, under the costs.** A copy of `vsyncpoll` with 192 delays, the return at 240 lines after the interrupt's end, cycles: number of waits:

- Sony's: 86:11 88:18 98:65 99:28 106:4 112:1 113:7 120:15 127:3 131:6 136:25 137:16. From 86 to 137.
- Clean, the packing as it is: 81:3 86:21 95:36 103:13 110:32 123:18 130:10 132:1 133:6 138:20 147:12 154:27. From 81 to 154.
- Clean, shifted 32 bytes: the same values, from 81 to 156.

**`probes/waitpass`** (new) times it from the program alone. Each field it starts straight after an interrupt, waits one scratchpad store more than the field before (64 steps, then again), calls `VSync(0)`, and polls timer 2 to the next interrupt: A is the cycles from the return to that interrupt. The blanks come a fixed time apart, so A's spread is the return's. 256 fields at 240 lines:

| | A, mean | sd | least to most |
|---|---|---|---|
| Sony's, emulator | 544,283 | 16.3 | 544,262 to 544,311 (49) |
| clean, emulator | 544,416 | 22.6 | 544,368 to 544,447 (79) |
| Sony's, console | 542,142 | 44.5 | 542,043 to 542,253 |
| clean, console | 542,103 | 49.4 | 542,013 to 542,238 |

The console's interrupt varies in length by more than a pass, which scrambles the phase each field there (A by delay is flat over 32 sweeps), so on a console the comparison is the variance: a phase spread evenly over a pass of W cycles adds W²/12 to it. The clean variance is 462 over Sony's on the console (with an error of some 250) and 247 over in the emulator. So the clean pass is wider than Sony's on the console as well, by what the emulator shows or more. (The model's A against the console's is off by some 2,100 cycles for both libraries and by 130 between them, as spec 46 found on this path. Only the spread is to be taken from the model.)

The images: s56b, `.build/console/libps-s56b.exe.gz`, d0751133. The console's output is in `.build/hardware/spec57` (`waitpass-sony.txt`, `waitpass-clean.txt`).

## What to do

1. Pace `libps_wait_blanks`'s pass (`exp/cleanlib/sync.go`) under the console's costs to Sony's: `waitpass` in the emulator with `PROBE_CPUCOST=1` should give the clean A an sd and a spread within a few cycles of Sony's (16.3, 49).
2. Keep its timing with the costs off as it is: `TestVSyncPoll` with the costs off checks the call's accesses and the return (38 to 60 cycles at 240 lines) at Sony's cycles, and `TestVSyncCount`, `TestIRQGaps` and `TestFieldTime` hold the rest.
3. If `waitField`'s wait for the count (`blanksPaced`) paces its passes the same way, say whether it wants the same, and do it if the tests show it.
4. Say what a console run should check.

## Testing it

- `waitpass` in the emulator on both libraries, before and after.
- The ten timing tests, at the packing as it is and with `GEN_SHIFT` 16, 32, 48, 64, 96 and 128: `TestVSyncPoll` should pass at every shift.
- `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), 0 FAIL before.
- `vsyncpoll`'s 240-line returns after the interrupt (`TestVSyncPoll`'s log): the clean range against Sony's 86 to 137.

## Results

Done 2026-11-13 (implementation side) and 2026-11-14 (the console, spec side). The pass is paced: under the costs `VSync(0)` at 240 lines returns 85 to 133 cycles after the interrupt, 108.5 on average with an sd of 17.3, where Sony's returns 86 to 137, 109.8 and 16.8. `waitpass` in the emulator gives the clean A an sd of 16.3, Sony's. `TestVSyncPoll` passes at every shift, and nothing moved with the costs off. `waitField`'s wait for the count is not changed: what the emulator shows there is not a matter of the pass (below), and it is the spec side's to measure first.

### What changed

- **`libps_wait_blanks`** (`playstation-yaroze-clean.S`): the same 12 instructions a pass and 7 on the way out, at the same addresses, with other ones of them loading from RAM. A pass was five loads, a nop, the count, two loads and the test, with one load and four nops on the way out. It is now a nop, three loads, two nops, the count, two nops and the test, with three loads on the way out, a nop after each of the first two. Under the costs a warm pass takes 36 cycles where it took 58.
- The count's load has a written nop after it now, so a build without `LIBPS_FIDELITY`, where `NOPS` is empty, no longer runs `subu` in the load's delay slot. The resident image is always built with it and was not affected.
- `sync.go`: `waitBlanks`'s comment only.

Nothing else changed: no pin, no test, nothing under `emu/`. The section is the size it was. The image is `.build/console/libps-s57c.exe.gz`, SHA-1 5b9ee0f1 (`libps.exe`), and ends at 80066288h as before.

### Why the pass was wide

The return's spread is not the pass alone. The interrupt runs enough code to put the loop's six cache lines out of the cache, so after it the loop is cold, and what the return costs depends on where the interrupt landed in two ways: the instructions left to the count's read, and the lines that have to be filled on the way. A line entered in the middle is filled from that word on (`emu/cpu/cost.go`), so landing after the count's read costs the rest of the pass, a whole pass, and the count's line a second time. A model of that for this loop (`.build/s57/sim.py`: what a nop, a load from RAM, the instruction after a load and a miss's fill with its wait for RAM cost) gives the old pass's eleven returns exactly as `vsyncpoll` shows them: 81 86 95 103 110 123 130 133 138 147 154. The old pass's 58 cycles became a spread of 73. It predicted each candidate's returns to the cycle as well.

So the pass was chosen with the model, over which of the 12 and 7 instructions are loads, against Sony's returns: the least, the most, and the share of time at each return between.

### Sony's returns again, with the phase spread evenly

`vsyncpoll`'s delay grows by one turn of a loop a field, 5 cycles or so under the costs, and how many fields land on each return then depends on how that step beats against the pass: three copies with 192 delays, differing only in code that runs before or after the waits, had 20%, 40% and 20% of Sony's waits at 136 and 137. A copy with 240 delays in steps of a single scratchpad store (`.build/s57/vp192`, and `.build/s57/zz_s57_test.go`, which goes in `image/probetest` and uses `runWaits` as `TestVSyncPoll` does) spreads the phase evenly: the clean shares come out as the model's, each instruction's time over the pass's. The return after the interrupt's end at 240 lines, costs on, cycles: number of waits, of 479:

- Sony's: 86:19 88:31 98:161 99:49 106:12 112:7 113:23 120:60 127:14 131:9 136:64 137:30. Mean 109.8, sd 16.8.
- s56b (192 delays in `vsyncpoll`'s steps): 81 to 154, mean 117.1, sd 22.4.
- s57a: 87:6 92:5 93:12 95:24 96:26 101:66 110:19 114:37 119:93 128:98 136:93. Mean 117.1, sd 14.6.
- s57b: 85:84 98:52 104:97 112:51 113:1 119:6 124:17 125:6 128:31 133:3 134:28 136:103. Mean 112.4, sd 18.5.
- **s57c: 85:20 90:51 91:85 100:1 101:102 108:49 121:11 122:10 125:23 130:10 131:15 133:102. Mean 108.5, sd 17.3.**

Sony's spends 44% of its time where the return is 98 or 99, which no arrangement of 12 nops and loads gives. s57c has 54% of its waits at 101 or under where Sony's has 54% at 99 or under, and 27% at 130 and over where Sony's has 22% at 131 and over. With the costs off the four clean images are the same: 38 to 60, 11 values, wait for wait.

The three candidates:

- **s57a** (6b5a1659): one load before the count and two after, 34 cycles. It fitted the spec's 192-delay figures, and is narrow and late against the even sample.
- **s57b** (7e2932c8): four loads before the count, 40 cycles, 85 to 136. It passed the ten tests and failed `TestVSyncPoll` at shifts 96 and 128, with 156 against the window's 155: the first wait after the probe prints. That wait returns up to 20 cycles later than any other at its phase, and its interrupt takes 500 cycles longer: the printing before it leaves more of the path cold. On the way out that is most likely the two lines of `libps_vsync_wait` after the call (8003C100 and 8003C110 by `image.elf`), each a fill of 9 cycles and a wait for RAM: not traced, but the 20 is the same for the old pass and the new. Which return it adds them to depends on the phase the shift leaves it at. s56b's 167 and 174 were the same thing: 154 and 13 or 20.
- **s57c**: the longest return is 133, so that wait is 153 at most.

### `waitpass` in the emulator

| | A, mean | sd | least to most |
|---|---|---|---|
| Sony's | 544,283 | 16.3 | 544,262 to 544,311 (49) |
| s56b | 544,416 | 22.6 | 544,368 to 544,447 (79) |
| s57a | 544,417 | 18.0 | 544,382 to 544,447 (65) |
| s57b | 544,419 | 17.0 | 544,383 to 544,447 (64) |
| s57c | 544,420 | 16.3 | 544,397 to 544,461 (64) |

s57c's sd is Sony's. Its spread is 50 for 255 of the 256 fields, 544,397 to 544,447, with one field at 544,461. The spread can't be read finer than that: the probe's poll takes 15 cycles a step, so A comes in bins 15 apart (Sony's at 544,262, 277, 292 and 307, s57c's at 544,398, 413, 428 and 443), and a return 50 cycles wide falls into four bins or five by where its edges lie against the steps. A copy of the probe with 0 to 5 scratchpad stores between `VSync`'s return and the timer read (`.build/s57/wpoff`) moves the edges. Sony's own A then has an sd of 19.1, 14.4, 17.9, 18.5, 16.7 and 14.8 and a spread of 64, 49, 64, 64, 65 and 64; s57c's 16.9, 17.7, 17.6, 16.3, 17.2 and 18.6, and 60, 65, 49, 64, 64 and 64. Root mean square over the six: Sony's 17.0, s57c 17.4.

### `waitField`'s wait for the count

`blanksPaced` paces a pass as the old `libps_wait_blanks` did: five loads, a store, the count and two loads, some 61 cycles under the costs. It does not want the same change, and is left as it is:

- With the costs off the return at 480 lines follows bit 31's change, not the count, and `TestVSyncPoll` checks that. None of the ten tests times the count's pass there.
- Under the costs the scratch probe's 480-line half has the interrupt end after the field's change in 240 of its 479 waits, and `VSync(0)` then returns at the count. **Sony's return is 130 or 131 cycles after the interrupt's end in 235 of those 240, whatever the delay** (145 and 152 the rest). The clean one's is 140, 148, 155 or 168. `VSync(2)` the same way: Sony's 130 or 131, clean 183 or 192.
- A wait that polls the count can't return within a cycle of one time at every phase: a pass with one load from RAM spreads it by 6 at least. So at 480 lines Sony's wait is not caught at any point of a pass, as it is at 240, and what it does there is for the spec side to find from outside. The clean return is 10 to 38 cycles late and four values wide. A shorter pass would move it but could not make it Sony's.

### Checks

- The ten timing tests pass on s57c at the packing as it is.
- `TestVSyncPoll` passes with `GEN_SHIFT` 16, 32, 48, 64, 96 and 128. Its 240-line returns, costs on, 95 waits a run:

  | shift | returns | the first wait after the printing |
  |---|---|---|
  | 0 | 85 to 133 | in the range |
  | 16 | 85 to 133 | in the range |
  | 32 | 85 to 133 | 105 |
  | 48 | 85 to 133 | 141 |
  | 64 | 85 to 133 | 111 |
  | 96 | 85 to 133 | 145 |
  | 128 | 85 to 133 | 145 |

  With the costs off every run has the same 95 returns, 38 to 60 (38:1 40:14 42:14 44:13 46:3 50:9 54:21 56:13 58:5 60:2), as s57b has, and the 240-delay copy gives s56b, s57a, s57b and s57c the same 479.
- The image was rebuilt without a shift after the sweep: 5b9ee0f1 again.
- `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), 0 FAIL, as before.

### What a console run should check

`waitpass` on the console with s57c (`.build/console/libps-s57c.exe.gz`) and with Sony's in the same session, 256 fields each, more runs if there is time, since the variance's error is what limits it:

- The clean A's variance should be Sony's within the error, where s56b's was 462 over (sd 49.4 against 44.5): in the emulator the variance of the return went from 220 over Sony's to 17 over (sd 22.4 to 17.3 against 16.8), and `waitpass`'s from 245 over to none. So an sd of about 45, and a least to most as wide as Sony's.
- The clean mean A should rise by 4 or so (544,416 to 544,420 in the emulator) and stay some 35 under Sony's. That difference is not the pass (spec 46).

### The console (spec side)

Run 2026-11-14, `waitpass` at 256 fields a run, two runs a library in the same session (`.build/hardware/spec57/c`), with spec 57's runs of s56b and Sony's beside them:

| | A, mean | sd | variance |
|---|---|---|---|
| Sony's, run 1 | 542,145 | 44.3 | 1,961 |
| Sony's, run 2 | 542,143 | 45.3 | 2,055 |
| Sony's, spec 57's run | 542,142 | 44.5 | 1,978 |
| s57c, run 1 | 542,114 | 45.1 | 2,034 |
| s57c, run 2 | 542,117 | 44.2 | 1,957 |
| s56b, spec 57's run | 542,103 | 49.4 | 2,440 |

s57c's variance is Sony's within the error (Sony's three runs 1,961 to 2,055, s57c's two 1,957 and 2,034), where s56b's was some 440 over. The clean mean A is 29 under Sony's, 12 nearer than s56b's 41.

The spec side's own run of `TestVSyncPoll` with `GEN_SHIFT` 0, 16, 32, 48, 64, 96 and 128 passes at every shift.

### Left open

- **The window's top.** `TestVSyncPoll` allows 155, 18 over Sony's longest return, and the first wait after printing costs the clean library 20 more. So the clean pass's longest return has to be 135 or less, and s57c's is 133 where Sony's is 137. Whether Sony's first wait after printing runs late as well was not looked at: the test's log on Sony's disc would show it.
- **With the costs off, Sony's 240-line return reaches 64**, the clean one's 60. Over the 240 delays Sony's has 38 40 44 46 48 50 52 54 56 60 62 64, and 62 and 64 in 49 of 479 waits. Spec 45's 38 to 60 was 48 delays' worth. The clean pass was left at 12 instructions, as the spec says to, and `TestVSyncPoll`'s window is 38 to 60.
- **`waitField` at 480 lines under the costs**, above: Sony's 130 or 131 cycles after the interrupt against the clean 140 to 168, and 183 or 192 for `VSync(2)`. The console doesn't show it (spec side, 2026-11-14, `.build/hardware/spec58`): `retphase` on Sony's library and on s57c gives `VSync(0)`'s return at 480 lines, A, a mean of 516,501 and 516,498, as spec 46 found on s46b. So the gap is the model's, and the library needs no change for it. The same run has the two differences specs 45 and 46 left: at 240 lines A is 542,181 on Sony's and 542,080 on s57c (spec 46's interrupt length, 170 on s46b), and the clean `VSync(-1)` poll is cheaper, 4,838 polls a field against 4,657 at 240 lines and 4,610 against 4,437 at 480 (spec 45, deferred until after the API merge).
- The scratch tools are in `.build/s57/agent` in the main checkout: `sim.py` and `search.py` (the model and the search over passes), `vp192` and `wpoff` (the probes), `zz_s57_test.go` (copy it into `image/probetest` to run `TestS57Waits`), `hist.sh`, `wpoff.sh`, `vpoll.sh`, `sweep.sh`.
