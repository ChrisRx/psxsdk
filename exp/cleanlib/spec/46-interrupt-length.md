# The clean vertical blank interrupt runs longer on a console

## Why

Spec 45 found that on a console the clean library's vertical blank interrupt takes some 110 to 180 cycles longer than Sony's, every blank. Every program pays it. At 240 lines `VSync(0)` returns at the interrupt's end, so there the clean `VSync(0)` returns some 210 cycles after Sony's (`retphase`'s A). The emulator, with the cost model on, doesn't show it: there the clean interrupt is as long as Sony's or shorter.

## What is known

From spec 45's Results ("1, a third time" onward), console runs in `.build/hardware/spec45d` to `spec45k`:

- `irqsplit`, means over 32 blanks at 480 lines: the kernel's part 15,953 on Sony's and 15,975 on the clean library, the library's part up to `VSyncCallback` 1,603 and 1,645, the callback to the program's next poll 6,402 and 6,487.
- Cold minus warm (`icachemap`'s gaps) is some 650 cycles on both, so the extra is not misses that a fill outside the interrupt changes. Misses inside one interrupt, kernel and library code evicting each other, are not ruled out. The blank takes 193 cache lines on Sony's and 194 on the clean library, some 18 different each way.
- The device accesses are the same in number, apart from two extra DMA register reads and four fewer I_STAT reads on the clean library.
- The timing padding is priced right to within a few cycles (`padcost`): at most some 14 cycles of the 85 after the callback.
- The console's kernel is not scph1001's, the emulator's.

## What to do

Find where in the clean handler the extra cycles go, part by part, and make it take Sony's time on a console. A clean image built for the measurement only (not shipped), with timer 2 stamps inside `libps_interrupt`, `vblankInterrupt` and the sound tick, run with `irqsplit` on the console and in the emulator, would show which stretch the console prices differently from the model. If the cost model is what is wrong, say what to measure for it.

Then check that `retphase`'s 240p A comes to Sony's, and that statlines, vsynclate and the six timing tests still pass.

## Testing it

Console runs are the spec side's. Name each run and image needed, and what each result would tell.

## Results

Done 2026-11-11 in the emulator (implementation side). Not tried on a console. The library is unchanged: `.build/cleanimage/libps.exe` from this tree is byte for byte `libps-spec45b`.

### The model against the console, row by row

`cpucost4` and `padcost` rerun in the emulator with the costs on, against the console's `spec45g` and `spec45h` (a pattern's cost: the warm runs' mean less `empty`, over the count):

| pattern | console | model |
|---|---|---|
| `lw ram` | 6.1 | 6.0 |
| `lh spu` | 20.9 | 21.0 |
| `lh spu nop` | 21.0 | 22.0 |
| `lw istat` | 4.0 | 4.0 |
| `sh spu` | 7.4 | 7.5 |
| `sw istat` | 1.9 | 2.1 |
| `sh spu lw ram` | 8.2 | 8.1 |
| `sh spu lh spu` | 28.9 | 30.0 |
| `sh spu nop8 lw ram` | 15.1 | 15.9 |
| `sw istat lw ram` | 8.3 | 8.0 |
| `sw ram lh spu` | 22.2 | 24.0 |
| `sw ram x4 lh spu` | 28.3 | 30.0 |
| `spin`, a turn | 17.1 | 17.0 |
| `space 16` | 44 to 53, mean 50 | 42 to 46, mean 44 |
| `space then load 16` | 48 | 48 |
| `loadSpace 16` | 116 to 121 | 116 |
| `spaceRun 16` | 68 to 70 | 67 |
| `nops 128` | 151 | 156 |

Where the two differ by more than a cycle the console is the cheaper: the instruction after an SPU load costs nothing there (`lh spu nop` 21.0 against `lh spu` 20.9) where the model charges its cycle, and the pads' stores before an SPU load cost 1.5 to 2 less a pattern. Both libraries make the same 24 SPU reads an interrupt, so that moves both alike, by some 24 to 48 cycles, and not the difference. `space 16` is the one row the console has dearer, by some 6 over 16 stores, and `space then load 16` is not: the stores cost the same, and it is the counter read after a run of stores that costs more on the console, which the handler does not do. So of what the handler runs, nothing the console has measured is priced under; what it has not measured is below.

### What the clean handler runs, and what the model charges it

`image/probetest/irqtrace_test.go` (new, `TestIRQTrace`, `-irqtrace` the clean disc of `irqsplit` or `irqstamp`, `-irqtrace-image` the image's ELF) follows one 480-line blank interrupt on the clean image with the costs on and logs the model's charges by the probe's stretches, by region (the kernel, the clean library, the probe) and by kind of instruction, with the library's time by function. It reads the kernel's and the clean library's instructions, never Sony's. On the stamped image below, blank 8 at 480 lines:

- The kernel's part (K0 to K3), 4,829 instructions, 13,909 cycles: 1,037 loads from RAM 7,721 (7.45 each), 1,343 ALU 1,858, 835 nops 1,275, 172 loads of I_STAT or I_MASK 1,020, 856 branches 970, 468 stores 505, the controller port's 27 accesses 179.
- K3 to the callback, 1,480: the kernel's 424 instructions 1,162 (68 loads 442, 122 nops 253, 114 ALU 201, 22 I_STAT loads 155, 81 branches 69), the probe's 7, and the library's 93 instructions 291: 43 stores to RAM 111 (2.6 each), 24 ALU 71, 8 nops 38, the 4 stamps' counter reads 29, 2 loads from RAM 19, 2 loads of I_STAT and I_MASK 17, 2 branches 9, and the rest free. By function: `vblankInterrupt` 60 instructions 178, `libps_interrupt` 30 and 99, `libps_intr_entry` 3 and 17.
- The callback to the poll, 6,313: the library's 2,603 instructions 5,978, the kernel's way out 66 and 353, the probe's 15 and 42. The library's: 1,170 ALU 1,444 (1.2 each); 133 loads of words from RAM 1,402 (10.5 each: `spin`'s load after its own store, waiting for the store, which padcost prices right); 354 nops 676; 24 SPU loads 659 (27.5 each: each waits for the 10 pad stores before it to drain); 74 byte loads from RAM 568 (7.7); 363 stores of words 435; 318 branches 332; 24 halfword loads 154; 48 halfword stores 55; 24 byte stores 47; 29 `j`/`jal` 24; 6 `jr`/`jalr` 12; 2 loads of DICR 15; 6 SPU stores 18. By function: `vblankInterrupt` (the tick inlined) 1,837 instructions 3,767, `spin` 681 and 1,910, `libps_interrupt` 72 and 238.

So, of what the clean handler runs, the console has measured and the model has right: the SPU and I_STAT accesses, `spin`, runs of word stores to one word and the loads after them, the nops. It has not measured: byte and halfword loads and stores to RAM (170 an interrupt, the voices' shadow and `tickReads`), loads from DICR (2), `jal` and `jr`, and any cost of changing DRAM row between accesses, which the handler does at every pad: cpucost's runs of stores go to consecutive words of one row, and `space` stores to one word, `spacer`, between the handler's work on its stack, the shadow and the SPU. Sony's handler, running its own code where the clean one pads, changes row less often if it does at all.

### `probes/cpucost5`

`probes/cpucost5` (new, `.build/serial/cpucost5.elf`) measures those, as cpucost4 does (machine code timed with timer 2, interrupts off, six runs): loads and stores alternating between `data[0]` and `data[1024]`, 4 KB apart ("rows"), against consecutive words and one word; `lhu`, `lh`, `lbu`, `sh` and `sb` in runs; a store then a load, and a load then a store, in the same row and across rows; four stores to one word then a load from another row or the same; ten stores to one word then an SPU load (the tick's pattern), with a halfword store to another row between; loads of I_MASK, DICR, DPCR and I_STAT; and 16 `jal`s of a cached `jr ra`. In the emulator with the costs on (below) a row costs nothing and a byte or halfword what a word does; on a console each row either does the same, and the handler's extra is not there, or says what the model is missing.

| pattern (a pattern's cost, the model) | | pattern | |
|---|---|---|---|
| `lw ram`, `lw ram rows`, `lw ram same word` | 6.0 | `sw then lw same row`, `sw then lw rows` | 12.9 |
| `lhu ram`, `lh ram`, `lbu ram` | 6.0 | `lw then sw same row`, `lw then sw rows` | 12.8 |
| `sw ram`, `sw ram same word`, `sw ram rows` | 2.0 | `sw x4 same word lw rows`, `... lw same row` | 18.9 |
| `sh ram`, `sb ram` | 2.0 | `sw x10 same word lh spu` | 42.0 |
| `lw imask`, `lw istat` | 4.0 | `sw x10 same word sh ram lh spu` | 44.0 |
| `lw dicr`, `lw dpcr` | 5.0 | `jal ret` | 4.3 |

The console's figures for the rows cpucost measured already are 6.1 (`lw ram`), 2.0 to 2.3 a store in a run (`sw ram 32`, 69 to 78 over six runs where the model has 70 every time) and 13.3 (`sw then lw ram`): the runs of stores vary on the console by up to 9 cycles in 32, which the model never does, and a handler making some 360 stores an interrupt could pay 20 to 40 of them there.

### The stamped image: `probes/irqstamp`

`exp/cleanlib/irqstamp.go` (new, build tag `irqstamp`; `irqstamp_off.go` has the stamp empty without it, so the shipped library is unchanged) stores timer 2 in the scratchpad's last 16 words, 1F8003C0h on, which the library never uses otherwise, at twelve points of the handler: 0 `libps_interrupt`'s start, 1 after its reads of I_STAT and I_MASK, 2 after the acknowledgment, 3 after the count (the probe's callback stamp follows), 4 after the callback, 5 after the tick's first pace, 6 after the envelope reads, 7 after the voice writes, 8 after `tickWordsTime`, 9 after the key words and the sequences, 10 after the exit pace, 11 `libps_interrupt`'s end. A stamp is a load from the counter and a store to the scratchpad, some 6 cycles, more where its line misses. The image is built with `TGOFLAGS=-tags=irqstamp exp/cleanlib/image/make.sh` and saved as `.build/console/libps-s46-irqstamp.exe.gz` (its ELF, for symbols, was `.build/cleanimage/image.elf` of that build; the plain build has since replaced it). For the measurement only, never shipped.

`probes/irqstamp` (new, `.build/serial/irqstamp.elf`) is irqsplit with the inner stamps read as well: it clears them before each blank and prints, after irqsplit's `BLANK` line, an `INNER` line when the image left stamps, with each stretch from one stamp to the next (`k3-s0` from the kernel's priority 3 stamp, `s3-cb` and `cb-s4` around the callback, `s11-poll` to the poll), and `SPLIT` lines for each. One ELF for Sony's library (no `INNER`), the shipped image (none) and the stamped one.

In the emulator with the costs on, means over 32 blanks (the stamped image's outer parts carry the stamps' own cost: 1,478 to the callback against the plain image's 1,420, 6,309 after it against 6,179):

| stretch | what it holds | 480i | 240p |
|---|---|---|---|
| kernel (K0 to K3) | the kernel's handlers | 13,900 | 13,900 |
| k3-s0 | the kernel's priority 3 chain and its custom exit to the library | 1,228 | 1,229 |
| s0-s1 | `space(5)`, the I_STAT and I_MASK reads | 38 | 39 |
| s1-s2 | `space(3)`, the acknowledgment | 79 | 79 |
| s2-s3 | `space(19)`, the count | 92 | 92 |
| s3-cb | the callback's load and call | 39 | 39 |
| cb-s4 | the callback's return | 24 | 24 |
| s4-s5 | `pace(194)`, `space(2)`, `nops(10)` | 352 | 352 |
| s5-s6 | the 24 envelope reads, 10 stores between each | 1,112 | 1,110 |
| s6-s7 | the voice write loop, nothing dirty | 531 | 531 |
| s7-s8 | the settled loop over 24 voices and `tickWordsTime` (`spend(827)`) | 2,935 | 2,891 |
| s8-s9 | the six key words | 136 | 136 |
| s9-s10 | `pace(362)` | 515 | 516 |
| s10-s11 | the DMA checks, `drawDone`'s nops, `space(6)` | 271 | 271 |
| s11-poll | the kernel's way out to the program | 432 | 433 |

On the console the same stretches, each over the emulator's, place the extra: Sony's parts there run 10 to 15% over the model (the kernel's 15,953 against 13,881; its to the callback 1,602 against 1,448; after it 6,402 against 6,192), so a clean stretch at that ratio is priced as Sony's is, and one well above it holds what the model misses. The kernel's `k3-s0` and `s11-poll` are the kernel's code under both libraries; a difference there is the cache, the kernel's lines the clean library's code takes within the interrupt, and the console's kernel is not scph1001's. `s2-s3` is 19 stores and a load; `s5-s6` the SPU reads behind pad stores (cpucost4 and cpucost5's `sw x10 same word lh spu` price it); `s7-s8` nearly all byte and halfword accesses to the shadow behind pad stores, and `spend`'s spin (cpucost5's `lbu`, `sb`, `sh` and "rows" rows price it).

### Checks

Before the library change below, the plain build of this tree (`exp/cleanlib/image/make.sh` without the tag) was byte for byte `libps-spec45b`: `irqStamp` is empty without the tag and the handler's code is as it was. `go test ./exp/cleanlib/...` passes. `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), as before. `TestIRQGaps`, `TestVSyncPoll`, `TestVSyncCount`, `TestVSyncPhase`, `TestDrawWait` and `TestFieldTime` pass on the plain image's discs.

### On the console (to do, spec side)

1. `irqstamp` (`.build/serial/irqstamp.elf`) on the stamped image, `.build/console/libps-s46-irqstamp.exe.gz`. Its `SPLIT` lines against the table above, stretch by stretch: the ratio of each to the emulator's, against the 1.10 to 1.15 Sony's parts show, says where the clean handler's 110 to 180 cycles go. The same ELF on Sony's library and on spec45b, for its outer parts against irqsplit's `spec45f` figures, checks that the probe measures as irqsplit did.
2. `cpucost5` (`.build/serial/cpucost5.elf`) on either library, against the emulator's rows above. A row the console has dearer goes into the cost model (emu/cpu/cost.go), `irqstamp` is rerun in the emulator, and if the model then shows the clean stretch long, the clean handler is set to Sony's times under it (the pads around the shadow's accesses first, `tickReadGap`'s stores and `tickWordsTime`'s spend, then `vblankCountSpace` for the 43 before the callback).

What the results together would say: if `s7-s8` is the long stretch and cpucost5's byte, halfword or row changes are dearer than the model, the console charges the shadow's accesses behind pad stores what the model does not, and the tick's pads shrink by it. If `s5-s6`, the pad stores before each SPU read. If `k3-s0` and `s11-poll`, the kernel's own code is slower under the clean library and only its cache lines can be: the clean handler's lines against the console kernel's, which the layout of the image's interrupt code would have to avoid, from a cache map taken on the console with the kernel's lines told from the library's (icachemap's blank map less a map of a `VSync(0)` with the library's handler short-circuited, say). If every stretch is 10 to 15% over, as Sony's are, the stamps have moved the difference and the plain image's irqsplit figures stand as the only measure.

### The console's stamps and rows (spec side, 2026-11-11, `.build/hardware/spec46`)

`irqstamp` on Sony's library (`sony/`), on spec45b (`45b-fable/`) and on the stamped image (`fable-stamp/`), and `cpucost5` on spec45b. The outer parts at 480 lines, means over 32 blanks:

| | Sony's | spec45b | stamped |
|---|---|---|---|
| kernel | 15,976 | 15,967 | 15,751 |
| library to callback | 1,577 | 1,632 | 1,667 |
| callback to poll | 6,406 | 6,490 | 6,555 |

The probe repeats irqsplit's `spec45f` figures (Sony's 15,953, 1,603, 6,402; the clean 15,975, 1,645, 6,487), and the kernel's part under the stamped image is 225 cycles shorter than under either other library: the kernel's own code moves by a couple of hundred cycles with the library's layout on the console, where the emulator has it the same under all three (13,887 to 13,900). The stamped image's inner stretches, the console against the emulator's model of that morning:

| stretch | console | model | over |
|---|---|---|---|
| k3-s0 | 1,319 | 1,228 | +91 |
| s0-s1 | 63 | 38 | +25 |
| s1-s2 | 109 | 79 | +30 |
| s2-s3 | 140 | 92 | +48 |
| s3-cb | 34 | 39 | -5 |
| cb-s4 | 24 | 24 | 0 |
| s4-s5 | 381 | 352 | +29 |
| s5-s6 | 1,246 | 1,112 | +134 |
| s6-s7 | 533 | 531 | +2 |
| s7-s8 | 2,960 | 2,935 | +25 |
| s8-s9 | 142 | 136 | +6 |
| s9-s10 | 524 | 515 | +9 |
| s10-s11 | 241 | 271 | -30 |
| s11-poll | 500 | 432 | +68 |

Every stretch is steady from blank to blank (`s5-s6` 1,237 to 1,259). The stretches over the model by the most, for their size, are the three before the callback that are pad stores and an I/O access or two (`s0-s1`, `s1-s2`, `s2-s3`: +103 together, 27 stores), and `s5-s6`, the envelope reads with ten stores between each (+134, 5.6 a read). `s7-s8`, the shadow's byte and halfword accesses, is within 1%.

`cpucost5` on the console, a pattern's cost (warm, the mean less `empty`, over the count), against the model's: `lw ram` 6.1 (6.0), the same across rows, to one word, and as `lhu`, `lh` and `lbu`; `sw ram` 2.0 to 2.3 (2.0), the same to one word and as `sh` and `sb`; **`sw ram rows` 5.7** (2.0): a run of stores taking turns between two rows costs 3.7 more a store; `sw then lw` 12.25 in the same row and across rows (12.9), `lw then sw` 12.1 both (12.8); `sw x4 same word lw` 18.6 across rows and 18.3 in the row (18.9); `sw x10 same word lh spu` 41.0 (42.0); with the `sh` to another row 44.6 (44.0); `lw imask`, `lw istat` 4.0 (4.0); `lw dicr`, `lw dpcr` 4.0 (5.0); `jal ret` 4.25 (4.3). So of the kinds unmeasured before, only a run of stores changing rows costs more than the model has, and the tick's shape (ten stores to one word, a halfword store elsewhere, an SPU read) costs the model's: `s5-s6`'s +134 is not in the stores' rows or in the SPU read's pattern measured warm.

**Where the extra is: the cache miss with stores draining.** The handler runs cold at every blank (icachemap, spec 45), and the probes' cold runs (the first of six) are what the handler sees. Cold less warm, a cache line (cpucost, console / the model of that morning): a line of nops 5.1 / 5.1; of loads 8.1 / 5.5 (`lw ram`, `lw ram nop`, `sw then lw ram`); of one store and three nops 10.2 / 5.2 (`sw ram nop3`); of two stores 11.8 / 5.3 (`sw ram nop`); of four stores 11.9 / 3.4 (`sw ram 128`); padcost's `space 16` cold 89 / 52, `space then load 16` 101 / 67. The model filled a missed line at once; on a console the fill waits for RAM, busy with the write buffer's stores draining or with the last load. That is what the clean handler's pads do to it: Sony's handler runs its own code where the clean one stores, and the kernel's code after the handler misses behind the clean handler's last stores (`s11-poll` +68).

**The model** (emu/cpu/cost.go): a miss's fill waits until RAM is free, `costMissAfterStore` 4 cycles after the write buffer's last store to RAM leaves it and `costMissAfterLoad` 3 after a load from RAM (`ramFree`); and the DMA controller's registers cost a load 4.0 (`regionDMA`, `costDMALoad`). With the store wait at 6 the store rows came out 13 to 15 a line cold, over the console's 10 to 12, at 4 a cycle over; at 3, cold less warm a line, console / model: `sw ram nop3` 10.2 / 10.1, `sw ram nop` 11.8 / 12.2, `sw ram 128` 11.9 / 12.2, `sw ram 64` 11.5 / 12.4, `sw ram 32` 12.1 / 12.9, `sw then lw ram` 8.1 / 7.3, `lw ram` 8.1 / 8.5, `lw ram nop` 8.1 / 8.5; padcost's `space 16` cold 89 / 88 and `space then load 16` 101 / 108 (at 4). Rows the model still has wrong, left for later: a run of stores changing DRAM rows (`sw ram rows` 5.7 a store, no rows in the model), and a taken branch cold (cpucost's `branch taken` cold less warm 394 against the model's 248 over 64 branches, 2.3 a branch), which both libraries' handlers and the kernel make. The emulator's tests pass (`go test ./emu/...`).

Under it the stamped image's stretches come to the console's before the callback and in the tick's loops, the console / the model: `s0-s1` 63 / 53, `s1-s2` 109 / 115, `s2-s3` 140 / 130, `s3-cb` 34 / 41 (the four together 346 / 339, where the model had 248), `s4-s5` 381 / 368, `s6-s7` 533 / 535, `s7-s8` 2,960 / 2,946, `s8-s9` 142 / 144, `s9-s10` 524 / 517. Not to: `s5-s6` 1,246 / 1,147 (the envelope reads, 4 a read under), `s10-s11` 241 / 295 (the DMA checks and drawDone's nops, over), `s11-poll` 500 / 472 and `k3-s0` 1,319 / 1,260 (the kernel's, under), and the kernel's part 15,751 / 14,082 (the controller driver's waits, not the CPU's). irqsplit under it at 480 lines, Sony's / spec45b: the kernel's 14,051 / 14,077, to the callback 1,501 / 1,546, after it 6,394 / 6,324: the model now has the clean library 45 longer before the callback, as the console does (55), and still 70 shorter after it, where the console has it 84 longer.

**The library.** The pads before the callback were 27 stores (5 at `libps_interrupt`'s start, 3 before the acknowledgment, 19 before the count); now the 5 and the 3 are nops (`interruptNopsIn`, `vblankAckNops`, 4 of them since the stores' address load is gone), and the 19 are 10 stores then 8 nops (`vblankCountSpace`, `vblankCountNops`). The 6 stores at the handler's end are nops too, 8 of them for the address loads gone there (`interruptNopsOut`): the kernel's way out missed its lines behind them (`s11-poll`, +68 over the model of the morning, the most of any stretch after the callback but the envelope loop). With the costs off every instruction is 2 cycles, so the acknowledgment, the count, the callback and the handler's end stay at Sony's cycles (TestVSyncCount: ack 10,244, count 10,298, callback 10,326 from I_STAT's bit; vsyncpoll's 240-line waits end at the cycle they did; TestIRQGaps, which a 2-cycle shift of the end flipped at 240 lines, passes); with them on, and on a console, a nop costs a cold line nothing beyond its own cycle where a store costs the line's miss its drain and 3 more. The tick's pads are untouched: the console's +84 there over Sony's is not placed (the model, corrected, still has the clean 70 shorter than Sony's after the callback and `s5-s6` 99 under the console), and `padshape` below is for placing it.

TestVSyncPoll's windows with the costs on were Sony's under the model before: its 240-line return 75 to 152 after the interrupt's end, its 480-line return 46 to 51 after bit 31's change. Under the corrected model Sony's disc gives 88 to 137 and 52 to 53 (the test failed on Sony's own disc at 480 lines), Sony's disc gives 88 to 137 and 48 to 53 and this image 81 to 154 and 52 to 56, so the windows are now 80 to 155 and 48 to 56, holding both: on a console the clean return at 480 lines is within 4 cycles of Sony's (retphase, spec 45), and the 240-line return is the interrupt's length, which this spec measures on the console, not in the model.

In the emulator under the corrected model, irqsplit at 480 lines (means), Sony's / spec45b / this image:

| | Sony's | spec45b | this image |
|---|---|---|---|
| kernel | 14,051 | 14,077 | 14,076 |
| library to callback | 1,501 | 1,546 | 1,492 |
| callback to poll | 6,394 | 6,324 | 6,307 |

The image is `.build/console/libps-s46a.exe.gz` (the tree's `make.sh` build).

`probes/padshape` (new, `.build/serial/padshape.elf`): copies of the handler's shapes as the compiler lays them out, each timed six times cold (4 KB of nops run first to take every cache line, since the emulator's model keeps its lines through the kernel's FlushCache) and six warm: `preCount` (the 27 stores, the I_STAT and I_MASK reads, the acknowledgment, the count) and `preCountNops`; `readLoop` (the 24 envelope reads with ten stores between) and it with nops, with the stores to a word beside `reads` (`Near`), reading voice 0 only, without the read's store, and with a RAM load for the SPU read; `tail` (`space(6)`) and `tailNops`. In the emulator under the corrected model (cold, warm):

| shape | cold | warm |
|---|---|---|
| `preCount` | 196 | 87 |
| `preCountNops` | 110 | 59 |
| `readLoop` | 1,147 | 1,087 |
| `readLoopNops` | 928 | 896 |
| `readLoopNear` | 1,138 | 1,087 |
| `readLoopVoice0` | 1,147 | 1,088 |
| `readLoopNoStore` | 1,067 | 1,016 |
| `readLoopNoSPU` | 905 | 846 |
| `tail` | 40 | 19 |
| `tailNops` | 22 | 12 |

On a console, `readLoop` against the stamped image's `s5-s6` (1,246 with a stamp each end) says whether the loop costs its 5.6 a read more there in the probe too; the variants say whether it is the stores (nops), their row against `reads`' (near), the voices' registers (voice 0), the read's store, or the SPU read itself (no SPU). `preCount` cold against 196 checks the model's new wait on the shape the library change rests on, and `preCountNops` what the change buys.

### Checks, with the model and the library changed

`go test ./exp/cleanlib/...` and `go test ./emu/...` pass. `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), as before. `TestIRQGaps`, `TestVSyncPoll`, `TestVSyncCount`, `TestVSyncPhase`, `TestDrawWait` and `TestFieldTime` pass on this image's discs, and `TestIRQGaps` and `TestVSyncPoll` on Sony's under the corrected model with the windows above. With the costs off the handler's end is at the cycle it was (vsyncpoll's 240-line waits end at 15,544 from I_STAT's bit on Sony's, on spec45b and here; an image with one nop fewer at the end, at 15,542, flipped irqgaps' 240-line field at line 263).

### On the console (to do, spec side)

1. `irqsplit` (`.build/serial/irqsplit.elf`, spec 45's) on `.build/console/libps-s46a.exe.gz`, against `spec46/sony` and `45b-fable`: the library to the callback should come to Sony's 1,577, from spec45b's 1,632 (the model says 9 under; the console's cold line costs more than the model's by a cycle or so a store, so a little over is as likely); the kernel's part and the part after the callback say what the layout moved. Then `vsynccount` is not needed (costs off, checked here), but `retphase` on the image, for the 240p A against Sony's 542,172: the clean return at 240 lines should move some 50 of its 210 nearer.
2. `padshape` (`.build/serial/padshape.elf`) on either library, against the table above: which variant of the read loop loses the 5.6 a read, and whether `preCount` cold is 196. If `readLoopNops` is what loses it on the console as in the model (219 the loop), the tick's `tickReadGap` stores are the next to turn into nops with the costs-off time kept, which would put the clean tick near Sony's on the console if the other after-callback stretches hold; if `readLoopNear`, the stores' row; if `readLoopVoice0` or `NoSPU`, the SPU's own reads, which both libraries pay.
3. `irqstamp` is not needed again unless the padshape result points back into the handler.

### The console on s46a (spec side, 2026-11-11, `.build/hardware/spec46/fable-s46a`)

irqsplit, means in cycles: the library to the callback 1,597 (Sony's 1,603, spec45b 1,645), so the part before the callback is at Sony's. The callback to the poll 6,470 at 480 lines and 6,433 at 240 (Sony's 6,402 both, spec45b 6,487 and 6,450), the whole 24,044 and 24,002 (Sony's 23,959 and 23,962). retphase's median A at 240 lines 542,007 (Sony's 542,172, spec45b 541,962) and at 480 lines 516,498 (516,501): the 240-line return is still 165 late. statlines and vsynclate as before. `padshape` and a second `cpucost5` (another agent's, with rows for the read loop) ran on it too.

### Third round: the DRAM's rows, the read loop, and where the rest is (implementation side, 2026-11-11)

Done in the emulator against the console's rows above. The new image is not tried on a console.

**`padshape` on the console against the model.** Cold and warm, the console's mean, the model of the round before, and the model now:

| shape | console | model before | model now |
|---|---|---|---|
| `empty` | 11, 4 | 4, 5 | 4, 5 |
| `preCount` | 186, 91 | 196, 87 | 196, 87 |
| `preCountNops` | 106, 56 | 110, 59 | 110, 59 |
| `readLoop` | 1,139, 1,080 | 1,147, 1,087 | 1,123, 1,063 |
| `readLoopNops` | 908, 872 | 928, 896 | 904, 872 |
| `readLoopNear` | 1,137, 1,078 | 1,138, 1,087 | 1,114, 1,063 |
| `readLoopVoice0` | 1,131, 1,081 | 1,147, 1,088 | 1,123, 1,064 |
| `readLoopNoStore` | 1,047, 991 | 1,067, 1,016 | 1,043, 992 |
| `readLoopNoSPU` | 929, 871 | 905, 846 | 905, 846 |
| `tail` | 35, 19 | 40, 19 | 40, 19 |
| `tailNops` | 18, 10 | 22, 12 | 22, 12 |

Two things follow.

- The probe's read loop costs the console what the model said, 45 a read, where the same loop in the handler cost it 4 more (`s5-s6`, 1,246 against 1,147). So the extra is not in the loop's instructions. It is in where its stores go. `readLoopNear` was meant to test that and did not: in the probe `spacer` (800920C4h), `reads` (800920CCh) and `near` (800920FCh) are all in one row of the DRAM, so `readLoop` and `readLoopNear` are the same test. In the image `tickReads` is at 800668C8h and `spacer` at 80020624h: each read's store and the ten pad stores after it are in different rows, 24 times a blank. cpucost5 had measured what that costs (`sw ram rows`, 5.7 a store against 2.0) and the model had no rows.
- With nops for the stores the model was a cycle a read over (896 against 872): the instruction after a load from the SPU. cpucost4 had it too (`lh spu nop` 21.0 against `lh spu` 20.9).

`empty` cold is 11 on the console and 4 in the model. The two reads of the counter are 4 apart warm; cold, the second waits for the miss of the line after it, three instructions on (the CPU fetches ahead of what it runs). The model charges a miss to the instruction missed, so a counter read just before a missed line is up to 7 cycles early in it. The sums are the same; a stamp next to a line's edge moves by that much, and the stamped image's stretches below carry it (`s10-s11` and `s11-poll` are 54 over and 28 under).

**The model** (`emu/cpu/cost.go`, with `TestCPUCostPorts` and the new `TestCPUCostRows` in `emu/cpucost_test.go`):

- A load from the SPU costs 20, and 21 straight after another (`costSPULoad` 19 beyond its cycle, `costSPUAgain`). Console / before / now: `lh spu` 20.9 / 21 / 21; `lh spu nop` 21.0 / 22 / 21; `sw ram lh spu` 22.2 / 24 / 23; `sw ram x4 lh spu` 28.3 / 30 / 29; `sw x10 same word lh spu` 41.0 / 42 / 41; `sh spu lh spu` 28.9 / 30 / 29; `readLoopNops` warm 872 / 896 / 872.
- A store to RAM in another row than the store to RAM before it, while that one is still in the write buffer, takes 1.5 cycles longer to leave it, and the store after it waits 2 more (`wbRowHalves`, `wbRowHoldHalves`; a row is 2 KB, `ramRowShift`). Console / before / now: `sw ram rows` 5.72 / 2.03 / 5.38 a store; `sw x10 same word sh ram lh spu` 44.4 / 44.0 / 45.0, which is 3.5 over the row without the halfword store on the console and 4.0 in the model (half cycles round up). Loads show no rows on the console and have none in the model. The rule is fitted to those two rows and is the simplest that gives both: all 3.5 on the store itself gives the first row and 2 too many on the second. The row's size is taken from the RAM's 9 bits of column, not measured: the probe's words were 4 KB apart. `probes/rowcost` below asks the console for both.

Under it, the other agent's cpucost5 rows for the read loop (its source run here from a copy, not kept), a read, console / model: ten stores to one word 44.8 / 44.0, to ten words 44.8 / 44.0, eight stores and two nops 41.2 / 40.0, six and four 36.5 / 36.0, sixteen stores 2.19 a store to one word and 2.09 to sixteen / 2.06. The word stored to does not matter, as the model has it. Its cold chains come out 1.9 a line over the console on every row, the control included (`cold nop nop j nop` 9.6 a line against 11.6): a jump from a line's last word into a missed line costs the console some 0.6 more than running into it, where the model charges `costMissJump`'s 2. The rows over the control, which are what the wait for RAM rests on, agree: a store in the delay slot 5.5 / 5.4, a store first 5.0 / 4.8, three stores 9.8 / 9.4, a load 7.9 / 7.8.

What the console shows and the model still does not have:

- A branch taken in a line still being filled costs some 3 cycles (cpucost, `branch taken` cold 589 against `branch not taken` 440, 2.3 a branch, the model 445 both). Tried: 3 on a jump from a filled line to a cached one adds 33 cycles to Sony's part after the callback and 30 to the clean one, which puts both at the console's Sony figure, and changes nothing between them. Left out: the probe's branch goes to the next word, which the model's fetch can't tell from running on, so the row can't check it.
- A run of stores hiccups on the console: 9 or 10 cycles more in some runs (`sw ram 32` 69 to 78, `sw ram 128` 274 to 285 where 2 a store is 261), about 0.1 a store, more in a long run. The model is steady at 2.0.
- `costMissJump` as above.

**The stamped image again.** `libps-s46-irqstamp` rebuilt here (byte for byte) and run under the model now, 480 lines, console / model: `k3-s0` 1,319 / 1,260, `s0-s1` 63 / 53, `s1-s2` 109 / 115, `s2-s3` 140 / 130, `s3-cb` 34 / 41, `cb-s4` 24 / 29, `s4-s5` 381 / 368, **`s5-s6` 1,246 / 1,219** (1,147 before), `s6-s7` 533 / 535, `s7-s8` 2,960 / 2,945, `s8-s9` 142 / 144, `s9-s10` 524 / 518, `s10-s11` 241 / 295, `s11-poll` 500 / 472. The callback to the poll 6,555 / 6,526. The rows are in the read loop only: no other stretch moved. What is left of `s5-s6`, 27, is the size of the hiccups on its 264 stores.

**Where the rest is: not in the handler's instructions.** irqsplit under the model now, means at 480 lines:

| | kernel | library to callback | callback to poll |
|---|---|---|---|
| Sony's | 14,052 | 1,501 | 6,371 |
| s46a | 14,076 | 1,492 | 6,375 |
| console, Sony's | 15,953 | 1,603 | 6,402 |
| console, s46a | 15,977 | 1,597 | 6,470 |

So with the rows in, the model has the clean part after the callback at Sony's, within 10, and the console has it 68 over. For Sony's and for the stamped clean image the console is the same 30 over the model after the callback (6,402 against 6,371, 6,555 against 6,526); for the unstamped clean images it is 95 over. The console's parts, image by image (means, 480 lines; `entry` is `gap` less `whole`, the kernel's entry before any handler and half a poll):

| image | kernel | library to callback | callback to poll | entry |
|---|---|---|---|---|
| Sony's (`spec45f`, `spec46/sony`) | 15,953, 15,976 | 1,603, 1,577 | 6,402, 6,406 | 461, 464 |
| spec45b (`spec45f`, `45b-fable`) | 15,974, 15,967 | 1,644, 1,632 | 6,487, 6,490 | 533, 532 |
| s46a | 15,977 | 1,596 | 6,469 | 531 |
| the other agent's s46e | 15,970 | 1,605 | 6,494 | 530 |
| the other agent's stamped image | 15,971 | 1,625 | 6,709 | 529 |
| `libps-s46-irqstamp` | 15,751 | 1,667 | 6,555 | 464 |

Every clean image but one has the kernel's entry 69 cycles longer than under Sony's, and that one, `libps-s46-irqstamp`, has it at Sony's and the kernel's own handlers 220 shorter than under any other library. The entry is the kernel's code alone, the same instructions under each library, so its time can differ only by which of its cache lines the last interrupt left. What differs in that image is where the handler is: `libps_interrupt` grew by its three stamps and the image's packing put it at 80041F8Ch, cache lines F8 to 0F, where every other clean image has it at 80044BE8h, lines BE to E1 (`vblankInterrupt` and `spin` are where they were; `TestIRQTrace` now logs the lines each part of the interrupt runs). So on the console the clean handler's code takes lines of the kernel's entry, which Sony's leaves, and lines of the kernel's handlers, which Sony's takes too. The kernel's way out, the last of the part after the callback, is the kernel's code as well, and the stamped image's 500 for it against the model's 472 is the 30 that Sony's has. That is the lead for the 60 or so after the callback that the unstamped images have and neither Sony's nor the model does: the clean handler evicting the kernel's way out, as it does the entry. The emulator can't show it: scph1001's entry runs lines 08, 41, 42, 59, C8 to E0 and EA and its way out 00, 01, 0B, 5E, 5F and F4 to FF, Sony's handler runs across C8 to E0 there, and both libraries' entries cost the same 470. The console's kernel is another, and its lines are not known. `probes/stagemap` below gets them.

Two things about irqsplit's figures, both the probe's:

- Its first 12 blanks at 240 lines are in another state than the rest. The clean tick counts each voice's silent reads up to 16 from the library's start and then takes another path; on the console the clean image is uploaded just before the probe, and Sony's has been running since the console started. The callback to the poll on s46a is 6,365 for blanks 1 to 11 and 6,478 from 12 on, at both heights, where Sony's is 6,402 throughout (in the emulator Sony's library is started by the disc and shows the step too, 5,915 to 6,371). So the 240-line mean, 6,433, is a mix, and the clean part after the callback is 73 over Sony's at both heights, not 31 and 68. The clean library before the count is some 35 under Sony's settled time; nothing says what Sony's takes before its own count on a console.
- It prints a line after every blank, so every blank but the first of each height comes after the library's print path has run. Blank 0, which comes after a `VSync(0)` instead, has the kernel's part 370 shorter on Sony's and 200 on s46a. The parts are what a blank costs after that program's code, on both libraries alike, but they are not the cost after any other.

Of retphase's 165 at 240 lines irqsplit's parts on s46a hold some 150: the entry 69, the kernel's handlers 15, before the callback 6 less, after it 73.

**The library** (`spu.go`, `fidelity.go`, `nofidelity.go`). The pads are set under the model now, and no longer rest on stores:

- The read loop's ten stores to `spacer` are seven nops and three loads of it (`tickReadNops`, `tickReadLoads`): 16 instructions a read as before, so 32 cycles with the costs off. With them on the reads are 53 apart, Sony's 52 (the first 401 after the callback and the last 1,637, Sony's 417 and 1,633), where the stores had them 48 apart in the model and some 49 on the console. Nops and loads are what the console prices as the model does (`nop` 1.00, `lw ram` 6.1 and 7 into one register, `loadSpace 16` 118 against 116), and they have no rows and no hiccups.
- The tick's last pad was 29 turns of `spin`, 17 cycles a turn on a console for its 12 in the emulator. It is now `tickExitPace`: 13 turns of `spin` and 13 of `nopRun` (new: a loop of three nops, 12 cycles a turn in the emulator and 6 on a console), with five nops for what the call leaves. It is a function of its own because the handler must stay the size it was: when `vblankInterrupt` grew by the loop's 12 instructions the image's packing moved `putchar` and six more of the print path, `SsSeqPlay`, `libps_biosC` and others, the part before the callback changed by 24 cycles in the model, and `TestVSyncPoll`'s first 240-line wait went to 167 (80 to 155). As it is, the new function took `libps_putchar_other`'s place and ten small functions moved after it (`GsSetClip2D`, `__negdf2`, `GsSetAmbient`, `__divsf3`, `putc`, `InitHeap`, `FlushCache`, `_get_errno`, `SetVideoMode`); the handler, `spin`, `VSync` and the rest are where s46a has them.

With the costs off the interrupt is cycle for cycle s46a's: irqsplit's `SPLIT` lines and every device access of a blank, from the kernel's priority 3 stamp to the poll, are at the cycles they were (the callback at 1,016, the reads from 1,268 every 32, the key words from 5,558, the DMA checks at 6,004 and 6,014, the poll at 6,270). With them on, irqsplit at 480 lines, Sony's / s46a / this image (s46b):

| | Sony's | s46a | s46b |
|---|---|---|---|
| kernel | 14,052 | 14,076 | 14,077 |
| library to callback | 1,501 | 1,492 | 1,492 |
| callback to poll | 6,371 | 6,375 | 6,365 |
| whole | 21,924 | 21,944 | 21,936 |

and the tick's accesses from the callback, Sony's / s46a / s46b: the first read 417 / 401 / 401, the last 1,633 / 1,537 / 1,637, the key words from 5,129 / 5,035 / 5,139, Sony's reads of I_STAT and the clean ones of DICR from 5,853 / 5,749 / 5,740, the poll 6,369 / 6,364 / 6,363. (The clean handler's end, after its DMA checks, costs more under a console's costs than Sony's after its last reads, so the checks come some 110 before Sony's reads for the end to be right, as they did in s46a.) retphase under the model, median A, Sony's / s46b: 544,350 / 544,311 at 240 lines, 516,561 / 516,561 at 480.

What the console should make of s46b, if the reading above is right: the read loop some 75 longer than s46a's (53 a read for some 49 to 50) and the exit pad some 110 shorter, so the callback to the poll some 35 under s46a's 6,470, still some 30 over Sony's, and the entry still at 531: the rest is the layout's, and this image does not change where the handler is.

The image is `.build/console/libps-s46b.exe.gz`. A stamped build of the same tree, for the measurement only, is `.build/console/libps-s46b-irqstamp.exe.gz`; its `libps_interrupt` is at 80041F8Ch again, as `libps-s46-irqstamp`'s. Its stretches under the model, 480 lines: `k3-s0` 1,260, `s0-s1` 37, `s1-s2` 97, `s2-s3` 106, `s3-cb` 41, `cb-s4` 29, `s4-s5` 367, `s5-s6` 1,323, `s6-s7` 534, `s7-s8` 2,945, `s8-s9` 144, `s9-s10` 398, `s10-s11` 308, `s11-poll` 470; the kernel's part 14,082, the library to the callback 1,541, the callback to the poll 6,521.

**`probes/rowcost`** (new, `.build/serial/rowcost.elf`): cpucost5 with rows for the rule above. `sw alt +N`, 32 stores taking turns between a word at a 4 KB boundary and one N bytes on, N from 4 to 4,096, and across a 1 KB, 2 KB and 4 KB boundary 4 bytes apart, for the row's size; runs of 16 stores with one change of row at the end, in the middle, at the start, with two and with three, for what a change costs and where; pairs of stores 1, 2, 4 and 8 nops apart, to two rows and to one, for how close they must be; and the tick's read loop, eight reads, as it was (`read far`, the store to another row than the pads), in one row (`read near`), with a nop between (`read far nop1`), and as it is now (`read nops loads same reg`, and `three regs`). In the emulator under the model, warm (the six runs alike; `empty` 5):

| row | cycles | row | cycles |
|---|---|---|---|
| `sw alt +4` to `+1024`, `1020 1024` | 70 | `sw A nop1 sw B nop1`, and `same` | 38, 38 |
| `sw alt +2048`, `+4096`, `2044 2048`, `4092 4096` | 177 | `sw A nop2 sw B nop2`, and `same` | 53, 53 |
| `sw A x16` | 38 | `sw A nop4 sw B nop4`, and `same` | 85, 85 |
| `sw A x15 B` | 40 | `sw A nop8 sw B nop8`, and `same` | 149, 149 |
| `sw A x8 B x8` | 42 | `read far` | 389 |
| `sw A x8 B A x7` | 45 | `read near` | 357 |
| `sw B A x15` | 42 | `read far nop1` | 357 |
| `sw A x4 B x4 A x4 B x4` | 49 | `read nops loads same reg`, `three regs` | 397, 381 |

**`probes/stagemap`** (new, `.build/serial/stagemap.elf`): icachemap with the measurement moved into the interrupt, for the lines the interrupt has taken by each of four points. Machine code fills the cache with a map of 256 routines, one a line, arms a stage and waits for the blank; at the stage a hook calls every routine between two reads of timer 2. The stages: `entry`, first in the kernel's chain of priority 0 (the kernel's entry); `kernel`, first in the chain of priority 3; `callback`, the library's `VSyncCallback`; `poll`, after the interrupt (icachemap's `blank`); and `control`, no interrupt. A stage's lines less the one before's are what ran between. It warms its measuring loop on a routine of its own first, so that line 0 is not charged the loop's misses, and takes a cached line's time as the control runs' commonest. In the emulator with the costs on, both heights alike and steady:

| stage | Sony's | s46b |
|---|---|---|
| `entry` | 30: 08 41-42 59 C8-E0 EA | the same |
| `kernel` | 151: 00-06 08 12-16 1E-1F 41-42 45-56 58-6E 79-87 89-91 95-A9 AE B1 B4-BD C2-E4 EA | the same |
| `callback` | 180: those and 3B-40 6F-72 E5-E9 EC-EF F1-FA | 170: those and 32-33 3F-40 43-44 57 6F-72 BE-C1 E5-E9 |
| `poll` | 196: 00-0D 12-16 1E-1F 3B-42 45-56 58-72 79-87 89-91 95-A9 AE B1 B4-BD BF-FF | 195: 00-06 08 0B 12-1F 32-33 3F-72 79-91 95-AC AE B1 B4-EA F4-FF |

The `entry` and `kernel` rows are `TestIRQTrace`'s lines for the kernel's entry and its handlers, so the probe sees what runs.

### Checks, third round

`go test ./emu/...` and `go test ./exp/cleanlib/...` pass. `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), as before. `TestIRQGaps`, `TestVSyncPoll`, `TestVSyncCount`, `TestVSyncPhase`, `TestDrawWait` and `TestFieldTime` pass on s46b's discs and, under the model now, on Sony's, with `TestVSyncPoll`'s windows as the round before left them (80 to 155 and 48 to 56).

### On the console (to do, spec side)

1. `rowcost` (`.build/serial/rowcost.elf`), either library, against the table. `sw alt`: the N at which a store goes from 2 to 5.7 is the row's size (the model has 2 KB). `sw A x15 B`, `x8 B x8`, `x8 B A x7`, `B A x15` over `sw A x16`: the model says 2, 4, 7 and 4 more; if `sw B A x15` is at `sw A x16`'s time a change of row after RAM has been idle is free, as the model has it for the read's store, and if the pairs with nops between differ from their `same` rows, stores further apart than the model's pay too. `read far` less `read near` is what the old loop paid for its rows (the model: 4 a read; the stamped image says nearer 5), and `read nops loads same reg` what the new one costs (49 a read here, without the loop's own three instructions).
2. `stagemap` (`.build/serial/stagemap.elf`) on Sony's library and on s46b. Its `entry` lines are the console kernel's entry, and `poll` less `callback` on Sony's holds its way out. The clean handler's lines are known (`TestIRQTrace`, and `poll` less `kernel` here): those of them on the kernel's entry and way out that Sony's `callback` and `poll` rows do not take are the 69 and the 60, and the clean handler's code is then laid out off them (`libps_interrupt`'s place first, since one image shows what moving it does). If the `entry` rows of the two libraries are not the same lines, the probe's hook is being reached differently and the reading needs another look.
3. `irqsplit`, `retphase`, `statlines` and `vsynclate` (spec 45's ELFs) on `.build/console/libps-s46b.exe.gz`, against `fable-s46a` and Sony's. Expected: the library to the callback as s46a's (1,597); the callback to the poll some 35 under s46a's (about 6,435 at 480 lines); the entry 531; retphase's 240-line A some 35 nearer Sony's. The callback to the poll at Sony's 6,402 or under would say the stores cost the handler more than the rows and hiccups counted here, and the exit pad then gives back some turns of `spin` (11 cycles a turn moved from `nopRun`).
4. `irqstamp` (`.build/serial/irqstamp.elf`) on `.build/console/libps-s46b-irqstamp.exe.gz`, against the stretches above: `s5-s6` at 1,323 and `s9-s10` at 398 say the new pads cost the console what the model has, in place; and the entry at 464 and the kernel's part near 15,750 again, with `libps_interrupt` at 80041F8Ch again, say that it is the handler's place that the kernel's 69 and 220 follow.

### The console on s46b, and on the probes (spec side, 2026-11-11, `.build/hardware/spec46/r3`)

irqsplit on s46b, means: the kernel's part 15,977, the library to the callback 1,597, the callback to the poll 6,444 at 480 lines and 6,440 at 240, the whole 24,019 and 24,016, `gap` 24,553 and 24,550. retphase's median A 542,002 at 240 lines (Sony's 542,172, s46a 542,007) and 516,497 at 480 (516,501). statlines 251 514 776 and 263 515 778; vsynclate as Sony's but for its one outlier line. `rowcost` and `stagemap` on Sony's library, `stagemap` on s46b, and `irqstamp` on the stamped s46b.

### Fourth round: the rest is the clean monitor's (implementation side, 2026-11-11)

Done in the emulator against the round 3 console runs. The library is not changed: the candidate stays `libps-s46b`, and there is no s46c.

**irqsplit on s46b.** Its 240-line run has no step this time: 6,435 to 6,473 from blank 1 on, where s46a's went from 6,365 to 6,478 at blank 12. s46b's irqsplit was not the first probe after the upload, so the clean tick's count of silent reads had run out before it; the tick's code for the count is as it was, and the emulator still shows the step on s46b (6,265 to 6,371). So the settled figure is the one to compare: 6,478 on s46a, 6,442 on s46b at both heights, 36 less, the 35 expected. The 240-line mean "not moving" (6,433 to 6,440) is s46a's mean being a mix of the two states.

**`rowcost`.** A row is 1 KB, not 2: `sw alt +512` 2.2 a store, `+1024` 5.6, and two words 4 bytes apart across a 1 KB boundary 5.7. A change of row in a run costs 4 cycles wherever it is in the run (`sw A x8 B x8` 4 over `sw A x16`, `x8 B A x7` 8, `x4 B x4 A x4 B x4` 12 to 22, `sw B A x15` 4, the first store after idle RAM costing nothing more). Stores to two rows a nop or two apart still cost 5 each, four nops apart nothing more than to one row. A change of row at the run's end is not waited for by the counter's read (`sw A x15 B` 2 under `sw A x16`). The read loops, a read: the old one 49.0 (`read far`), in one row 44.9, with a nop before the pads 49.4, and the new one 49.0 with its loads into one register and 47.4 into three, where the model had 49 and 47: the new loop costs the console what the model says.

The model's rule is now that: a store to RAM in another 1 KB row than the store before it starts to leave the write buffer no sooner than 3.5 cycles after that one left, and a load from a port waits for the last store as if it had changed no row (`wbRowHalves` 7, `ramRowShift` 10, `wbPort`; `TestCPUCostRows`). Console / model, cycles for the row less `empty`: `sw alt +1024` and on 179 to 182 / 170; `sw A x15 B` 31 / 33; `x8 B x8` 37 / 37; `x8 B A x7` 41 / 40; `sw B A x15` 38 / 37; `x4 B x4 A x4 B x4` 48 / 44; the pairs 1, 2, 4 and 8 nops apart 81, 81, 83, 144 / 82, 82, 82, 144 (to one row 35, 49, 81, 144 / 33, 48, 80, 144); `read far` 392 / 384, `read near` 359 / 352, `read far nop1` 395 / 384, the new loop 392 and 379 / 392 and 376; cpucost5's `sw ram rows` 5.72 / 5.31 a store and `sw x10 same word sh ram lh spu` 44.4 / 43.0.

**`irqstamp` on the stamped s46b.** The new pads landed: `s5-s6` 1,328 on the console against the model's 1,323, `s9-s10` 399 against 398, and the callback to the poll 6,523 against 6,521 (the other stretches as before: `s4-s5` 380 / 367, `s7-s8` 2,971 / 2,945, `s10-s11` 243 / 308, `s11-poll` 499 / 470). The entry is 463 and the kernel's part 15,773 again, with `libps_interrupt` at 80041F8Ch again.

**`stagemap`: the console kernel's lines are the emulator kernel's, and the clean runs have seven more at the entry.** On Sony's library the `entry` stage is 08, 41-42, 59, C8-E0 and EA, the 30 lines scph1001's entry runs; the `kernel` stage adds 00-06, 12-16, 1E-1F, 45-56, 58-62, 79-87, 89-91, 95-A9, AE, B1, B4-BD, C2-C7 and E1-E4 (scph1001's, less 63 to 6E); the `callback` stage 3B-40, 63-64, 66-67, 69-6B, 6D-72, E5-E9, EC-EF and F1-FA; and `poll` 07, 09-0D, BF-C1, EB, F0 and FB-FF. So Sony's handler runs across the kernel's entry lines and handlers' lines on the console as in the emulator, and the round before's reading of the stamped image was the wrong way round: with `libps_interrupt` at lines BE to E1 the clean handler takes those lines as Sony's does, and with it at F8 to 0F (the stamped images) it leaves them, and the kernel's entry and handlers then run 69 and 220 cycles quicker than they do under Sony's. **`libps_interrupt` is to stay where it is.**

On s46b the `entry` stage has those 30 lines and E7-E8, F6-F8 and FB-FC: seven lines of code run between the exception and the kernel's first chain element that Sony's runs do not have. They are not the library's (nothing of its interrupt path is on them, and in the emulator the clean `entry` stage is the 30 lines). They are the clean monitor's. On the console the clean library runs under the clean monitor, `serial/monitor`, which hwrun uploads with it, and Sony's under the boot disc's. The clean monitor points the exception vector at 80h at code of its own (`installVector`, `hook_yaroze.go`): at every interrupt it reads the cause, its `watching` flag and the serial port's status before the kernel's vector runs, 20 instructions in three places of a 96-word array, which is the seven lines, two, three and two, 15 and 5 lines apart. It also points the kernel's B-function stub at B0h at its `dispatch` (`installHook`), 12 instructions comparing the function's number before every B-function, and the kernel's way out of an interrupt, ReturnFromException, is one.

**`probes/monhook`** (new, `.build/serial/monhook.elf`) measures what the two cost: irqsplit with copies of the monitor's two interrupt paths put in by the probe (the same instructions at the same places in arrays laid out as the monitor's, every way out going on to the kernel, entered through the four instructions each replaces, so it works over the kernel's stubs and over the monitor's), 32 blanks plain and hooked at each height and plain again. In the emulator under the model, 480 lines, hooked less plain: the kernel's part 0, the library to the callback 0, the callback to the poll 38 on Sony's library and 40 on the clean one, and the entry (`gap` less `whole`) 78 on both.

The console's clean runs less Sony's, s46b: the entry 534 less 461 to 464, 70 to 73; the callback to the poll 6,442 less 6,402, 40. So the two parts that were left are the monitor's hooks, to within what the model can say, and the library's own time after the callback is Sony's:

| s46b less Sony's | console | model, the library | model, the monitor's hooks |
|---|---|---|---|
| entry | 70 to 73 | 0 | 78 |
| kernel | 24 | 26 | 0 |
| library to callback | -6 | -9 | 0 |
| callback to poll | 40 | -19 | 38 to 40 |
| `gap` | 133 | -1 | 117 |

(Under the model with 1 KB rows irqsplit at 480 lines is, Sony's / s46b: the kernel's part 14,051 / 14,077, the library to the callback 1,501 / 1,492, the callback to the poll 6,379 / 6,360, the whole 21,933 / 21,931.) What is the library's on the console is then the kernel's part, 24 longer under the clean library, which the model has too: the kernel misses two lines more an interrupt under the clean library, `vblankInterrupt`'s code on lines 4C, 50 and 53 taking the kernel's 44C0h, 4500h and 4530h (spec 45), and the console's kernel is on scph1001's lines. retphase's 170 at 240 lines is the `gap`'s 133 and what the return through `VSync(0)` adds, not measured apart.

So nothing in the library is changed for the 40 and the 70. Whether the tick's exit pad should give back two turns (the model has the clean part after the callback 19 under Sony's, the console level with it once the hooks' 38 to 40 are taken off) waits for the hooks' cost on the console, which the runs below give.

**The layout the image needs** (for the packing, `image/gen.py`):

1. `libps_interrupt` stays at 80044BE8h (cache lines BE to E1). There its code takes lines of the kernel's entry (C8 to E0) and of its handlers (C2 to E4), as Sony's handler does, whose code after the callback runs lines BF to EA. An image with it at 80041F8Ch (lines F8 to 0F) has the kernel's entry 69 cycles and its handlers 220 cycles quicker than under Sony's on the console. A build must not move it off lines C4 to E1: the stamped builds do, by growing it, and they are for the stretches only.
2. `spin` stays at 8001AD5Ch (lines D5 to D8), in the kernel's entry lines as well.
3. `vblankInterrupt` stays at 800113F0h and its size 1E7Ch: its growth by 12 instructions moved the print path's functions and others, and changed a wait in `TestVSyncPoll` by 13 cycles. The 24 cycles the kernel's part runs longer under the clean library would go if its code before the callback and its tick left lines 4C, 50 and 53 to the kernel (it runs 3F to 57 there). That is inside one function, not the packing's; it would take the function's start moving by a few lines with everything else held, and is not asked for: 24 of 24,000.

### Checks, fourth round

`go test ./emu/...` and `go test ./exp/cleanlib/...` pass with the model's new rule. The image is s46b's, byte for byte, so `image/regress.sh` stands (38 PASS, 3 TIME). The six timing tests were not run before that round's agent was stopped; the fifth round below ran them.

### Fifth round: the fourth, checked and finished (implementation side, 2026-11-11)

The fourth round's agent was stopped before its checks. What it left: no library change half-applied (`.build/cleanimage/libps.exe` is `libps-s46b` byte for byte, and there is no s46c), the model's 1 KB row rule with its test in `emu/`, `probes/monhook` with its ELF, and the section above, whole but for its test results. Nothing was backed out and nothing in the library or the model is changed in this round. Each reading above was made again from the console's files, and stands:

- **irqsplit on s46b.** The run came after `stagemap` on the same upload, so it has no counting state: the callback to the poll is 6,417 to 6,473 from blank 1 at both heights, where s46a's run, the first after its upload, has 6,329 to 6,401 for blanks 1 to 11 and 6,476 on from 12. Settled against settled, s46b is 36 under s46a.
- **`rowcost`.** Rerun under the model, warm, the console / the model: `sw alt +4` to `+512` 69 to 79 / 70; `+1024` and on, and the three pairs across a boundary, 175 to 192 / 175; `sw A x16` 37 / 38, `x15 B` 35 / 38, `x8 B x8` 41 / 42, `x8 B A x7` 45 / 45, `B A x15` 41 / 42, `x4 B x4 A x4 B x4` 49 to 59 / 49; the pairs with nops 79 to 94, 84 to 89, 84 to 94, 148 / 87, 87, 87, 149 and to one row 37, 53, 85, 148 / 38, 53, 85, 149; `read far` 392 to 401 / 389, `read near` 356 to 368 / 357, `read far nop1` 394 to 403 / 389, the new loop 396 and 384 / 397 and 381 (`empty` 4 / 5). The model needs no more from it.
- **The seven lines are the monitor's vector, by its addresses.** In the clean monitor's ELF (`.build/serial/monitor.elf` of the main checkout) `vector` is at 8007EE7Ch. An interrupt runs its words 0 to 4 (the cause, the branch), on lines E7 and E8; words 58 to 68 (`watching`, the serial port's status), on F6 to F8; and words 80 to 83 (the kernel's four instructions), on FB and FC. Those are stagemap's seven. All seven are taken again by the library's and the kernel's code before the next blank (s46b's `callback` stage has B4 to EA, its `poll` stage F4 to FF), so each blank pays seven misses, 20 instructions, a load from RAM and one from the serial port: 65 to 95 cycles by the model's prices, for the console's 70 to 73. `dispatch` is at 8007ED7Ch, lines D7 to DA, which `spin` and `libps_interrupt` take every blank: four misses and 12 instructions before ReturnFromException, for the 40.
- **`monhook` under the model**, rerun, 480 lines, hooked less plain: the entry 78 on both libraries (471 to 549 and 471 to 548), the callback to the poll 38 on Sony's (6,384 to 6,422) and 41 on the clean (6,363 to 6,404), the other two parts 0.
- **The kernel's entry and way out, and the clean handler's lines.** The console kernel's entry is 08, 41 to 42, 59, C8 to E0 and EA under both libraries, scph1001's 30 lines. Sony's `poll` stage less its `callback` stage is 07, 09 to 0D, BF to C1, EB, F0 and FB to FF; s46b's is 0B, 17 to 1D, 65, 88, 92 to 94, AA to AC, F4, F5, F9, FA and FD to FF. What both have, 0B and lines from F4 up, is in scph1001's way out (00, 01, 0B, 5E, 5F, F4 to FF; the rest of it was taken earlier in the blank). `TestIRQTrace` on s46b has the clean library's code before the callback on lines 32 to 33, 3F to 4B and BE to C4, and after it on 16 to 1E, 33, 4B to 57, 65 to 6B, 7C to 83, 86 to 8C, 8E to 91, 96 to A1, A3 to AC, C4, C6 to C9, CB to D3, D5 to D9 and DD to E1. So the clean handler takes 23 of the entry's 30 lines each blank (41, 42 and all of C8 to E0 but CA, D4, DA to DC), as Sony's takes them (its code after the callback is on BF to EA in the emulator), and none of the way out's.
- **The stamped image.** Two builds with `libps_interrupt` at 80041F8Ch, both with the entry at 463 to 464 and the kernel's part at 15,751 and 15,773, so its place does move the kernel's time on the console, by some 67 and 205. But the unstamped place is the one that gives Sony's: the kernel's part, which the monitor's hooks do not touch, is 15,977 on s46b against Sony's 15,953 to 15,976, and 200 under it on the stamped builds. The stamped entry at Sony's 464 is the hooks' 70 less the 67, not Sony's time. The emulator does not show the 67 and the 205 (the kernel's part is 14,077 and 14,082 there); why not is open, and does not bear on the library, since the shipped place is the one that matches.

So the answer to "is the handler's place the cause" is no: the place matters, and it is right as it is. What the clean runs have over Sony's on the console, 70 at the entry and 40 after the callback, is the clean monitor's two hooks, which Sony's runs do not go through. The library's own difference from Sony's is the kernel's part, 0 to 24 longer, before the callback 6 shorter, and after the callback level to within what the hooks' cost on the console turns out to be (the model has the clean tick 19 to 21 shorter than Sony's; the console has it level if the hooks cost there what the model says).

### Checks, fifth round

`go test ./emu/...` and `go test ./exp/cleanlib/...` pass. `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), as before. `TestIRQGaps`, `TestVSyncPoll`, `TestVSyncCount`, `TestVSyncPhase`, `TestDrawWait` and `TestFieldTime` pass on s46b's discs and on Sony's, under the model with 1 KB rows, with `TestVSyncPoll`'s windows as the second round left them (80 to 155 and 48 to 56).

### On the console (to do, spec side)

The images: `.build/console/libps-s46b.exe.gz` (this worktree) and Sony's own. The ELFs: `irqsplit.elf` and `retphase.elf` are spec 45's, in the main checkout's `.build/serial/`; `monhook.elf` is in this worktree's.

1. `irqsplit` and `retphase` (spec 45's ELFs) on **Sony's library under the clean monitor** (hwrun's take-over with Sony's image for the clean one, as `statlines-sonylib-ourmon` was run), against s46b's runs in `r3/s46b`. This is the comparison the clean library should be held to: both under one monitor. Expected: Sony's entry at 531 or so, its callback to the poll some 40 up at about 6,440, and retphase's 240-line A within some 30 of s46b's 542,002. Run `retphase` first and `irqsplit` after it on the same upload, as s46b's were run after `stagemap`, so that neither library's sound tick is in a state fresh from the upload; if blanks 1 to 11 still differ from 12 on, compare from 12 on. If Sony's callback to the poll under the clean monitor is under 6,420, the clean tick is that much long and the exit pad moves turns from `spin` to `nopRun` (11 a turn); if over 6,465, the other way.
2. `monhook` (`.build/serial/monhook.elf`) on Sony's library under the boot disc's monitor, and on s46b: `480i hooked` less `480i plain` in each part. Expected from the model: the callback to the poll 38 to 40, the entry 78, nothing in the other two. On Sony's library that is the clean monitor's cost added to Sony's own figures (6,402 and 461), which should then come to s46b's 6,442 and 534; on s46b it is a second layer of the hooks and says what one costs there. The probe puts the vector and the stub back before each plain run and before END, and every loop of it is bounded.
3. Nothing else. `stagemap`, `rowcost` and the stamped image are done with.

### The console on the hooks (spec side, 2026-11-11, `.build/hardware/spec46/r5`)

`monhook` on Sony's library under the boot disc's monitor (`sony-bootmon/`) and on s46b under the clean monitor (`s46b/`), and Sony's library image loaded by hwrun under the clean monitor, `retphase` then `irqsplit` on one upload (`sony-cleanmon/`).

### Sixth round: the hooks' cost on the console, and the close (implementation side, 2026-11-11)

Done from the round 5 console files and in the emulator. The library and the model are not changed: the candidate stays `libps-s46b`, and there is no s46c.

**`monhook`: what the hooks cost.** Means over blanks 1 to 31 (blank 0 follows a `VSync(0)`, not a print), plain to hooked:

| part | Sony's, 240 lines | Sony's, 480 | s46b, 240 | s46b, 480 | the model |
|---|---|---|---|---|---|
| entry (`gap` less `whole`) | 461 to 538, +77 | 463 to 539, +76 | 532 to 609, +77 | 532 to 607, +75 | +78 |
| kernel | 15,950 to 15,936, -14 | 15,935 to 15,946, +11 | 15,977 to 15,981, +4 | 15,973 to 15,966, -7 | 0 |
| library to callback | 1,656 to 1,657 | 1,657 to 1,658 | 1,571 to 1,570 | 1,571 to 1,570 | 0 |
| callback to poll | 6,396 to 6,432, +36 | 6,394 to 6,431, +37 | 6,428 to 6,469, +41 (blanks 12 on) | 6,434 to 6,471, +37 | +38 to +41 |

(The 480-line plain figures are the two plain runs' mean. The kernel's part varies by some 40 from blank to blank, so a mean of 31 is good to 7 and a difference to 10: its -14 to +11 is nothing. s46b's 240-line plain run came first after the upload and has the tick's counting state in blanks 1 to 11, 6,330 for 6,428, so its row is from blank 12.)

So on the console the hooks cost **76 at the entry (75 to 77) and 37 after the callback (36 to 41), and nothing in the kernel's part or before the callback**, on either library, as a first layer over the boot disc's monitor or a second over the clean one. The model had 78 and 38 to 41. The clean monitor's own vector costs a little less than the probe's copy of it: s46b's plain entry is 532 against Sony's 462, 70.

**The library's difference, with the hooks out.** Two ways, both at 480 lines. From irqsplit, each library under its own monitor (`r3/s46b`, `spec45f/sony`; blanks 1 to 31), less the 76 and the 37; and from monhook itself, s46b plain (one layer, the monitor's) against Sony's hooked (one layer, the probe's), the same ELF on both:

| s46b less Sony's | irqsplit, raw | irqsplit, hooks out | monhook, one layer each |
|---|---|---|---|
| entry | +74 | -2 | -7 |
| kernel | +22 | +22 | +27 (plain to plain +38) |
| library to callback | -4 | -4 | -87 |
| callback to poll | +43 | +6 | +3 |
| `gap` | +135 | +22 | -64 |

- The entry is level: all of its 70 to 74 is the monitor's vector.
- The callback to the poll is level, +3 to +6: all of its 40 was the monitor's stub. The exit pad stays as it is: a turn moved between `spin` and `nopRun` is 11 cycles, more than what is left.
- The kernel's part is 22 to 38 longer under the clean library. The model has 20 (irqsplit) and 64 (monhook): the kernel's lines 4C, 50 and 53, which `vblankInterrupt`'s code takes (fourth round). Not changed, as there.
- The library to the callback is 4 shorter in irqsplit and 87 shorter in monhook. The two probes are the same measurement in two programs: monhook is irqsplit with 12 KB of arrays and the hooks' code, so its own code is elsewhere in the cache. Under monhook Sony's part before the callback is 52 longer than under irqsplit (1,657 for 1,605) and the clean one 30 shorter (1,571 for 1,601); the model has the same two moves, smaller (1,502 to 1,521, 1,496 to 1,475), and moves the kernel's part too (14,065 to 14,000 on Sony's, 14,085 to 14,064 on the clean). So a part's difference between the libraries moves by tens of cycles with the measuring program's place in the cache, each library its own way, and the sum goes from 22 over to 64 under. Differences of that size are not the library's to chase: no program sees one figure.

**retphase's A at 240 lines.** s46b's is 542,002 under the clean monitor and Sony's 542,172 under the boot disc's: 170 late. The hooks' two parts are 113 of it. The 57 left is not placed by a console run, and the model says why it need not be the library's. `probes/rethook` (new, `.build/serial/rethook.elf`) is retphase with monhook's hooks: after 16 blanks to settle, retphase's 32 pairs of fields at 240 lines plain and hooked, then at 480 lines plain and hooked. In the emulator under the model, mean A from field 8:

| | Sony's | s46b |
|---|---|---|
| retphase, 240 lines | 544,305 | 544,296 |
| rethook, 240 lines plain | 544,254 | 544,328 |
| rethook, 240 lines hooked | 544,124 | 544,132 |
| rethook, 480 lines plain, hooked | 516,569, 516,568 | 516,565, 516,565 |

(retphase's means in the fifth round, 544,350 and 544,311, had Sony's first fields in them: in the emulator the disc starts Sony's library, and its first 12 blanks are in the tick's first state.) Three things: the hooks cost a 240-line return 130 on Sony's and 196 on the clean library in this program, more than their 113 in the interrupt's parts, since their lines also take from what runs after the interrupt; with one layer on both, the two libraries return within 8 cycles; and without hooks the clean return is 9 after Sony's in retphase's layout and 74 before it in rethook's. So the model's s46b is at Sony's return to within what the program's layout moves, and the console's 170 is the hooks' 113 to 196 with that scatter. The one console run that measures it like for like is below.

**Sony's library under the clean monitor (`sony-cleanmon`): what is usable.**

- Its entry, 535 to 536 at both heights: usable. The entry is the kernel's and the monitor's code alone, and Sony's library under the clean monitor has the clean runs' entry (532 to 535), 73 over Sony's own under the boot disc's monitor. That is the 70 again, on Sony's library, without the probe's copy.
- retphase's 480-line A, 516,501 (516,501 to 516,506): usable. It is Sony's own figure under the boot disc's monitor, so the hooks do not move the 480-line return (rethook under the model says the same), and s46b's 516,497 is 4 from it under either monitor.
- Its kernel's part, 15,865, is not. monhook shows the hooks add nothing to that part, and Sony's own is 15,935 to 15,965 under the boot disc's monitor in three programs: the 70 to 100 less is the library's state, not the monitor. With no start-up run the tick does not run (782 after the callback for 6,400), so its code and data take no cache lines from the kernel between blanks, and whatever else the start-up arms is not armed. It cannot be set against s46b's 15,977.
- Its library to the callback, 1,481, is not, for the same reason: 124 under Sony's own 1,605 in the same probe.
- retphase's 240-line A, 548,137, is not: the return is some 5,800 cycles early for the tick that does not run.

**Fair comparisons from here** (the monitor and hwrun are the spec side's; this is what the implementation side needs of them):

1. Best: Sony's library under the clean monitor with its start-up done. hwrun's take-over loads the image and never runs its start-up; for Sony's image the start-up has to run as the boot disc runs it before the monitor's hooks go in. The check that it took: irqsplit's callback to the poll at about 6,440 (Sony's 6,402 and the stub's 37), not 782. Then every probe compares the two libraries under one monitor, with no pricing.
2. Or the monitor unhooks while a probe measures. The stub at B0h is how a program's prints reach the serial port, so it cannot be out for a whole run: what is needed is a pair of calls a probe can make, hooks out and hooks in, that do nothing under the boot disc's monitor (or a hwrun flag that starts the program with both hooks out and puts them back when it returns, for probes that hold their lines until the end, as retphase does within a display). With the vector out the monitor cannot break in over the serial port until the hooks are back; the probes' loops are bounded.
3. Until then, price the hooks out with the probe's own copies, as monhook and rethook do: Sony's library, under the boot disc's monitor, hooked, against the clean library plain, one ELF. Taking 76 and 37 off irqsplit's parts is right for the parts. It is not right for a return time (A), where the hooks' lines cost the code after the interrupt as well.
4. Whichever is used: a difference between the libraries under some 50 cycles in a part, or some 80 in the whole, is within what the measuring program's own layout moves, on the console and in the model. Compare with one ELF on both, and do not read a part's difference from one program as the library's.

### Checks, sixth round

Nothing in the library or the model changed: `.build/cleanimage/libps.exe` is `libps-s46b` byte for byte, and `emu/` is as the fifth round left it. `go test ./emu/...` and `go test ./exp/cleanlib/...` pass. `image/regress.sh` and the six timing tests were not run again; the fifth round's results stand for the same image and model (38 PASS, 3 TIME; `TestIRQGaps`, `TestVSyncPoll`, `TestVSyncCount`, `TestVSyncPhase`, `TestDrawWait` and `TestFieldTime` pass on s46b's discs and on Sony's). `rethook` builds and runs to END on both libraries in the emulator, with the vector and the stub put back.

### Closing

The hooks explain the gap. On the console, clean under the clean monitor against Sony's under the boot disc's, the clean runs had 74 more at the entry and 43 more after the callback; the clean monitor's two hooks measure 76 and 37 there. With them out, s46b against Sony's at 480 lines:

| part | Sony's | s46b, hooks out | difference |
|---|---|---|---|
| entry | 461 | 459 | -2 |
| kernel | 15,962 | 15,984 | +22 |
| library to callback | 1,605 | 1,601 | -4 |
| callback to poll | 6,404 | 6,410 | +6 |
| `gap` | 24,432 | 24,454 | +22 |

Spec 45 began this at 22, 42 and 85 over in the three parts, with 72 at the entry not yet seen.

**What changed in this worktree, overall.**

- The library (`kernel.go`, `spu.go`, `fidelity.go`, `nofidelity.go`): the handler's pads no longer rest on stores to RAM, which a console prices by what is around them (a cold line's fill waits for the write buffer, and a store changing DRAM row waits 3.5 cycles). Before the callback the 27 stores are 10 stores and nops (`interruptNopsIn`, `vblankAckNops`, `vblankCountSpace`, `vblankCountNops`); the 6 at the handler's end are nops (`interruptNopsOut`); the read loop's ten stores are seven nops and three loads (`tickReadNops`, `tickReadLoads`); the exit pad is `tickExitPace`, 13 turns of `spin` and 13 of `nopRun`. With the costs off every device access of a blank is at the cycle it was. `irqstamp.go` (tag `irqstamp`, measurement builds only) and `irqstamp_off.go`.
- The model (`emu/cpu/cost.go`, `emu/cpucost_test.go`): a missed line's fill waits for RAM after a store or a load; a store to another 1 KB row than the store before it leaves the write buffer 3.5 cycles later; a load from the SPU costs 20, and 21 after another; the DMA controller's registers cost a load 4.
- The tests: `TestVSyncPoll`'s windows with the costs on are 80 to 155 and 48 to 56, holding both libraries under the model; `TestIRQTrace` (new) logs one blank's charges on the clean image.
- The probes: `cpucost5`, `irqstamp`, `padshape`, `rowcost`, `stagemap`, `monhook`, `rethook`.
- The image: `.build/console/libps-s46b.exe.gz`. The stamped builds and s46a are for the record only.

**What is accepted.**

- The kernel's part 22 to 38 longer under the clean library (the model has it): three of the kernel's lines under `vblankInterrupt`. Moving the function is not worth 0.1% of a blank against what a move costs the rest of the layout.
- The layout as the fourth round set it: `libps_interrupt` at 80044BE8h, `spin` at 8001AD5Ch, `vblankInterrupt` at 800113F0h with its size.
- The 240-line return under the clean monitor some 170 after Sony's under the boot disc's: the monitor's, with the layout's scatter. `rethook` below confirms or reopens it.
- The model's open rows (a taken branch in a line being filled, `costMissJump`, the stores' hiccups, the 67 and 205 the kernel gains with `libps_interrupt` at 80041F8Ch), which move both libraries alike or not the shipped one.
- The 480-line return 4 cycles before Sony's, spec 45's, which the hooks do not move.

### On the console (to do, spec side)

One run, to confirm the close; nothing else is needed for this spec.

1. `rethook` (`.build/serial/rethook.elf`, this worktree) on Sony's library under the boot disc's monitor, and on `.build/console/libps-s46b.exe.gz` under the clean monitor. Read the mean A from field 8 on. Sony's `240p hooked` against s46b's `240p plain` is the 240-line return like for like, one layer of the hooks on each: the model has them within 8, and anything within some 50 closes it. Sony's hooked less plain is what the hooks cost a return (the model: 130), and s46b's what a second layer costs it (196). If s46b's plain A is more than some 50 under Sony's hooked A, the clean return is late by more than a layout moves, and the exit pad gives turns from `spin` to `nopRun`, 11 cycles a turn, in a round of its own. The 480-line rows should be 516,501 and 516,497 hooked or not. The probe puts the vector and the stub back after each hooked run and before END, and every loop of it is bounded.
