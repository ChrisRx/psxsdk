# Kanji glyphs on a console

## Why

Spec 35's console session (its Results, "The Kanji flush's DMA wait") found that the Kanji glyphs differ between the two libraries on the console, where the emulator shows them the same. Sony's library gives set pixels in an empty cell (81ADh) and in the edge columns of glyphs, and in the glyph images that `kanjistream` reads back from video memory. The clean library gives zeros in all of them.

The console's BIOS font is the same as `scph1001`'s (26 samples, all equal), so the font isn't the cause. The likely cause is what RAM and video memory hold before the program starts. The emulator clears them, and a console doesn't. Sony's library would then leave some memory as it found it where the clean library writes zeros. A program that draws Kanji text on a console shows the difference on screen.

The emulator can now start with memory filled: `PROBE_FILL=N` in the probe tests fills RAM, the scratchpad and video memory from seed N before the boot (`emu.Machine.FillMemory`). The console outputs are in `.build/hardware/group2-kanji` (`kanji`, `kanjistream`, `kanjipast` and `kanjilimits`, each with a `-sony` and a `-clean` file).

## What to do

1. Show the difference in the emulator. Run the Kanji probes on both libraries with memory filled, and with a few seeds. Show which outputs differ and that a different seed changes Sony's output.
2. Find, from outside, where each of Sony's pixels comes from: which memory Sony's library reads but never writes, or doesn't clear, on the way to each glyph and cell. Use bus traces (`PROBE_BUS`), different seeds, and the probes' read-back output.
3. Stop and report before changing the library. The fix may mean the clean library reads memory it never wrote, as Sony's does, and the user decides whether to copy that. Report:
   - what Sony's library does;
   - what copying it would take;
   - what each choice would change for a program.

   If the cause is something else, such as a step the clean library leaves out or a different order of writes, say so. In that case you may fix it, with a test.
4. Spec 35 left one difference on purpose, by the user's decision on 2026-11-10: Sony's library stalls on a flush that fills its pool exactly, and the clean library doesn't. Keep it as it is.

## Testing it

- The Kanji probes (`kanji`, `kanjicodes`, `kanjistream`, `kanjipast`, `kanjilimits`) must still match Sony's in the emulator with memory cleared, as specs 13 and 29 left them.
- Any fix must make them match with memory filled too, over several seeds.
- `go test ./emu/... ./exp/cleanlib/...` and `image/regress.sh` must pass.
- On the console, the spec side reruns `kanji` and `kanjistream` on both libraries. Their outputs should then agree.

## Results


Done 2026-11-12 (implementation side), in two rounds. The first stopped at step 3 with the library unchanged, on the tree's image, `.build/console/libps-s51-base.exe.gz` (SHA-1 starts ac82982d, the same image as `libps-s49h`): the sections from "Summary" to "The choices" are that round's. The user then decided both choices, and "The decisions" and what follows are the second round, with the candidate image `.build/console/libps-s51a.exe.gz` (SHA-1 starts 64b31acc). Emulator runs are with the scph1001 BIOS unless they say OpenBIOS.

### Summary

- There are two causes, and neither is what RAM or video memory held at power-on.
- **A code with no glyph.** Under Krom2Tim's reading (Krom2Tim and KanjiFntFlush), Sony's draws a code from 8140h to 84BEh that has no glyph as the 30 bytes just before the non-kanji table, BFC65FE2h, where the clean library draws a blank cell. On the console's BIOS those bytes are the end of a copyright line and four addresses, so the cell shows a pattern. On OpenBIOS they are zeros, which is why the emulator showed the two libraries the same: spec 13's comparisons ran on OpenBIOS. With the scph1001 BIOS the emulator gives the console's `kanji` output on both libraries, line for line, with memory cleared.
- **The cell after the last character.** Sony's flush loads an image with one cell more than it has characters and never writes that cell: it loads what the stack held there. The clean library writes zeros into it, as spec 13 decided on 2026-11-03. The other cells of the last row that no character reaches are stack memory on both libraries (spec 29).
- Every difference in the console's `kanji`, `kanjistream`, `kanjipast` and `kanjilimits` outputs is one of these two (or the stall spec 35 left on purpose). The "edge columns" spec 35 saw are these cells.
- `PROBE_FILL` changes nothing either library does here, over seeds 1, 2, 3 and 7. The BIOS clears RAM on its way to the program, so a program booted from a reset finds its stack zeroed whatever the fill. The console's session ran one program after another with no reset between, and the stack kept what the programs before had left.

### What was run

`PROBE_FILL` reaches the machine through `probe.sh` (the test binary inherits it), but the Kanji probes end with `done` and a loop, not `END`, so `probe.sh` runs each to its frame cap. The runs here built the discs with `PROBE_FRAMES=1 exp/cleanlib/image/probe.sh s51NAME DIR` and then ran the same test binary with `PROBE_END` set to `done`. The scripts are in `.build/s51/` (`run.sh`, `batch.sh`, `batch2.sh`, `body.sh`, `cells.py`), the outputs in `.build/s51NAME-cimg/` and `.build/s51NAME-cimg-clean/` as `out.tty` (cleared) and `out.fN.tty` (seed N).

Lines that differ between Sony's and the clean library in the emulator, the probe's own output only:

| probe | lines | cleared | seed 1 | seed 2 | seed 3 |
|---|---|---|---|---|---|
| `kanji` | 371 | 15 | 15 | 15 | 15 |
| `kanjistream` | 6,407 | 818 | 818 | 818 | 818 |
| `kanjipast` | 3,210 | 322 | 322 | 322 | 322 |
| `kanjicodes` | 13,264 | 393 | not run | not run | not run |

A seed changes no line of `kanji` or `kanjistream` on either library. It changes 15 lines of `kanjipast` on both, the same 15 with the same words: case 70 (dx 992, dy 496) reads back video memory that nothing wrote. `kanjilimits` was run cleared on the clean library only (Sony's doesn't get past case 44), and its console files differ in the stall and in the cell after the last character (case 22's cell 12, cases 40 and 42's cell 15), as spec 13 lists.

Against the console's files (`.build/hardware/group2-kanji`):

- `kanji`: the emulator's output is the console's on Sony's (371 lines) and on the clean library, with no line different. The 15 lines are cases 14, 17, 18, 19 and 23 of Krom2Tim: the strings 81ADh, 823Fh, 827Fh, 82FDh and a lead byte 82h alone. Each has the same two rows of words and the hash c89bb439 on Sony's.
- `kanjistream`: Sony's differs between the emulator and the console in 89 lines, of three kinds. `print before open` gives 2 and `flush before open` true on the console, because `kanji` had run before it and left a stream open: the library's state outlives a program there. The cell after the last character differs in cases 1 to 6 and in no case after them: by then the program's own flushes have written over the stack where the image goes. And the first word of each image read back is 0, 2 or 0003BD3Fh in the emulator where the console has the glyph's row, on both libraries alike (25 cases on Sony's and 23 on the clean library, whose files differ in 25 lines, all of the first and third kind). That last one is an emulator difference in the read-back or in when the transfer reads the word, not looked into here.
- `kanjipast`: 80 lines on Sony's and 78 on the clean library. They are the first word again (16 and 14 cases) and cases 69 to 71, whose images lie off the page and whose read-back takes in video memory that nothing wrote: the console's held something, the emulator's zeros. Those are the same cells on both libraries.

By cell (`cells.py`, the emulator's two sides): `kanjistream` differs in 57 of 79 cases and `kanjipast` in 32 of 37 stream cases, each in the cell after the last character and nowhere else, but for `kanjistream` case 33 (cells 0 and 4, the codes 823Fh and 81ADh) and case 62 (cell 1, a lead byte before the NUL). `kanjipast` case 50 flushes twice, and its cell 0 is the second flush's one cell. In `kanjicodes` all 393 lines are Krom2Tim's (368 single codes and 3 strings), none Krom2Tim2's.

### A code with no glyph

`probes/kanjisource` (new, returns) prints the ROM's 30 bytes before each table, read by the probe itself, then calls Krom2Tim and Krom2Tim2 on one code each and names what the cell holds. On Sony's with scph1001:

	rom before non-kanji: 6d 65 6e 74 20 49 6e 63 2e 00 00 00 00 00 70 10 80 1f 74 10 80 1f f0 10 80 1f f4 10 80 1f
	rom before kanji: 03 00 03 00 03 00 03 00 03 00 03 00 03 00 ff fe 03 00 03 00 03 00 03 00 03 00 03 00 03 00

- Krom2Tim with 81ADh, 81B0h, 81C0h, 81FDh, 8200h, 823Fh, 8240h, 8259h, 827Fh, 82F2h, 82FDh, 8397h and 8461h returns 0 and the cell is the first line's bytes, all 15 rows. They are the unassigned cells of rows 1 to 8 and the trail bytes Shift JIS doesn't use. The first nine bytes are `ment Inc.`, the end of the ROM's "Sony Computer Entertainment Inc." just before the font.
- Krom2Tim with 8900h, 893Fh, 897Fh and 89FDh gives the second line's bytes, as spec 13 has it and as the clean library does.
- So Sony's rule is one rule: a code in a table's range with no place in it draws the glyph one before the table's first. For the kanji table that is the last box drawing character, a real glyph. For the non-kanji table it is whatever the ROM holds before the font.
- Krom2Tim2 gives a blank cell for all of them on both libraries.
- A flush draws the same bytes: `kanjisource`'s stream of a hiragana, 81ADh and a hiragana has them in cell 1, and a quadrangle draws that cell on the screen. The clean library's cell is blank.
- The 8140h cell, the full-width space, is a real glyph, blank in the ROM, on both libraries.

The bytes differ by BIOS. Of the ROM files at hand, those named scph1000, 1001, 1002, 3000, 5501, 5502, 5552, 7001, 7003, 7502, 101 and 4.1A hold the same 30 bytes as above, and those named scph3500, 5000, 5500 and 7000, and OpenBIOS, hold zeros there. With OpenBIOS the `kanji` probe has no line different between the two libraries, and `kanjisource` gives `blank` for every one of the codes on Sony's.

This is a read of memory the library never wrote in the plain sense that it is outside the font, but it is ROM: the same on every run of a console, and nothing a program or a reset changes.

### The cell after the last character

`kanjisource` fills the 96,000 bytes of stack below its sp with words that each hold their own place, flushes a stream, and reads the image back with StoreImage. On Sony's:

	flush 1 char:   cell 1: stack 32 zero 0 other 0 from sp-65616 to sp-65372
	flush no char:  cell 0: stack 32 zero 0 other 0 from sp-65624 to sp-65500
	flush 3 chars:  cell 3: stack 32 zero 0 other 0 from sp-65600 to sp-65116
	flush 16 chars: cell 16: stack 32 zero 0 other 0 from sp-63576 to sp-61652
	                cells 17 to 31 the same, 8 bytes on for each
	flush 20 chars: cells 20 to 31 the same way

All 32 words of the cell come from the stack, from the place the image has in the flush's frame (it starts 65,624 bytes below the sp of the probe's function that calls the flush, read at that function's entry). The clean library gives zeros for the cell after the last character (cells 1, 0, 3, 16 and 20 above) and the same stack words as Sony's, from the same addresses, for the rest of the row (17 to 31, and 21 to 31).

What the stack holds there is mostly the image of an earlier flush. A flush of one character right after one of three, with no fill between, has in its cell 1 the rows of the earlier image's cell 1 in every other row and the fill between them: the image is 2 cells wide now and was 4 wide, so the same bytes are other pixels. That is the "stale glyph" of spec 13. Before a program's first flush the cell holds whatever calls made before it left that deep, or what the stack held when the program started.

What the stack holds when a program starts: `kanjisource` counts the words that are not 0 in those 96,000 bytes before it calls anything. In the emulator it is 30 of 23,936 on Sony's and 34 on the clean library, the start-up's own, with memory cleared and with it filled (seed 7): the BIOS clears RAM before it loads the program. So a disc booted from a reset starts with a clean stack on a console too (spec 35: a reset clears RAM from 800A0000h up). A program started by the monitor after other programs, as in the console session, gets their stack.

No quadrangle draws this cell. It is in video memory, at the image's place, and shows only to a program that reads video memory back or draws from that part of the page with its own primitives.

### The choices

**1. A code with no glyph: copy Sony's read of BFC65FE2h, or keep the blank.**

- Copying takes one case in `glyphFor`: for Krom2Tim's reading, a code from 8140h to 84BEh with no cell gives `romNonKanji - romGlyph` as a ROM glyph, as the kanji range's bad trail byte already gives `romKanji - romGlyph`. Krom2Tim2's reading and `Krom2RawAdd2` stay. Then Krom2Tim's and the flush's times for such a code have to be measured again against Sony's, because the cell becomes a glyph read from the ROM where it was a cell cleared (the pads in `kanji.go` count the two apart), and what Krom2Tim leaves on the stack for it. The tests would be the five probes with scph1001 and with OpenBIOS, and `kanjisource`.
- Copied: a program that prints such a code gets Sony's pattern in the cell on a console and in the emulator with a Sony BIOS, and a blank cell with OpenBIOS. The console's `kanji` outputs then agree, and `kanjistream`'s apart from choice 2. The picture depends on the BIOS, as Sony's does.
- Kept: such a code is a blank cell everywhere. The console's `kanji` keeps its 15 lines.
- Text a program means to print doesn't hold these codes: they are unassigned. They come from a bad string, such as one cut in the middle of a character, where the lead byte takes the NUL as its trail byte (8200h and the like).

**2. The cell after the last character: leave the stack's words in it, or keep the zeros.**

- Copying takes the flush's loop over the cells stopping at the last character (`range k`, not `range cells`), and the flush's pad adjusted for the one cell it no longer writes. The image's place in the frame is already Sony's (spec 29), so the cell then loads the same addresses. The words match Sony's when the stack holds the same there: after an earlier flush it does, the image being the same on both, and before the first it depends on what each library's earlier calls left that deep, which specs 26 and 29 brought close but not equal.
- Copied: the image in video memory is Sony's in that cell too, where the stack agrees. Nothing on the screen changes, since no quadrangle draws the cell. `kanjistream`, `kanjipast` and `kanjilimits` would then match Sony's in the emulator but for choice 1, and on the console as far as the stack agrees.
- Kept: the cell is always blank, the same on every run. The rest of the last row is already stack memory on both libraries, so the clean library is not free of it either way. The alternative on that side, clearing the whole last row, would move the clean library further from Sony's.

Neither choice touches the stall on a flush that fills the pool, which stays as spec 35 left it.

### The decisions

The console ran `kanjisource` on both libraries after the first round (`.build/hardware/spec51/r1`): from `rom before non-kanji` to `END` both files are the emulator's, line for line. Its first line read 6,406 words not 0 on Sony's and 6,551 on the clean library, of 23,936, other programs having run before it.

The user's decisions, 2026-11-12:

1. **A code with no glyph: copied.** Krom2Tim and the flush now draw the 30 bytes before the non-kanji table, as Sony's does.
2. **The cell after the last character: the zeros stay,** as spec 13 decided. The reason: nothing on the screen changes either way, since no quadrangle draws the cell, and copying would match Sony's only where the two stacks held the same, which they do after an earlier flush and need not before the first.

### What changed

- `glyphFor` (`kanji.go`): under Krom2Tim's reading, a code from 8140h to 84BEh with no cell gives the ROM address `romNonKanji - romGlyph`, BFC65FE2h, as a glyph. Krom2Tim2's reading still gives a blank cell, and `Krom2RawAdd2` is as it was. The flush and what Krom2Tim leaves on the stack follow from it with no change of their own: both already took a glyph's address.
- `kromCharTime` (`fidelity.go`) has a table, `kromGaps`, for the codes from 817Fh to 837Fh that its ranges left to the default: the cells with no glyph and the symbols past the first range. Its range for the Greek characters now runs to 84BEh.
- Krom2Tim's frame grew from 80 to 88 bytes with that table, so `krom2TimFrame` is 88, the clear is 8 bytes shorter and `krom2TimPad` allows for it. With the old figure every call left its glyph bytes and its image 8 bytes off Sony's place.
- Spec 13's rule under "The ROM font and the codes" ("a blank cell if not") now holds for Krom2Tim2 only. The spec side's to change.

### Times and the stack

`probes/kanjinoglyph` (new, returns) calls Krom2Tim and Krom2Tim2 on 23 codes with no glyph, on 20 with one, and on strings of both kinds of the same shape, then flushes streams of them with and without `isbg`, 140 cases, each call between two GP1 markers. Before a Krom2Tim call it fills the 80,000 bytes of stack below sp with words that hold their place, and after it hashes the words that changed, with their places, in four ranges: under Sony's frame, the glyphs' bytes, the image, and the rest. Run with a bus trace for the times:

	PROBE_BUS=start exp/cleanlib/image/probe.sh s51noglyph exp/cleanlib/spec/probes/kanjinoglyph

`.build/s51/times.py` and `compare.py` read it, and the table of all 140 cases is `.build/s51/noglyph-times.txt`.

**Krom2Tim, one character.** Cycles over Sony's, which is 616,170 give or take 30 for every code here (615,765 for the probe's first four calls):

| code | before | now |
|---|---|---|
| no glyph, 817Fh to 81F8h | -90 at 81ADh, the one measured | +52 to +62 |
| no glyph, 81FDh | not measured | +34 |
| no glyph, 8200h, 823Fh, 8240h | +296 | +60 |
| no glyph, 8259h, 827Ah, 827Fh, 829Bh | not measured | +56 |
| no glyph, 82F2h, 82FDh | +584 at 82FDh | +116, +118 |
| no glyph, 837Fh | not measured | +184 |
| no glyph, 8397h, 83B7h, 83D7h | not measured | +90, +160, +232 |
| no glyph, 8461h, 847Fh, 8492h | +942 at 8461h | +308, +380, +452 |
| a hiragana, 82A0h | +58 | +56 |
| the space, 8140h | +44 | +50 |
| symbols 8180h to 81FCh | +170 at 81B8h, +418 at 81FCh | +48 to +60 |
| digits, capitals, small letters | +70, +108, +144 | +64, +102, +134 |
| katakana 8340h, 8380h | +68, +78 | +68, +72 |
| Greek 839Fh, 83BFh | +140, +202 | +136, +198 |
| Cyrillic and box drawing, 8440h to 84BEh | +1,124 to +1,314 | +300 to +488 |
| a kanji's bad trail byte, 897Fh | +2 | +4 |

The "before" column is the tree's image, where a code with no glyph was a cleared cell. The glyph walk takes about 72 cycles more for each range it passes and Sony's time is flat, which the table evens out up to 829Eh. Past that the figure is already 0, so the codes from 82F2h on run over by what the walk takes, as the Greek characters did and do. The symbols and the Cyrillic and box drawing glyphs came nearer Sony's on the way: they shared the default with the gaps.

**Krom2Tim, strings.** A string of codes with no glyph now takes what the same string of hiragana takes, within 100 cycles: 3 characters +846 against +828, 16 characters +6,254 against +6,160, 17 characters +7,800 against +7,698, and with a newline +2,310 against +2,288. Those excesses over Sony's are as before this spec (+830, +6,134, +7,670 for the hiragana).

**What Krom2Tim leaves.** For all 55 calls that succeed, the TIM, the result and the hashes of the glyphs' bytes, of the image and of the rest of the frame are Sony's, codes with no glyph included (the glyph range holds the 30 ROM bytes for such a cell). The range under Sony's frame differs in every case, with or without a glyph, as before: Sony's leaves 7 words there from the calls it makes and the clean library none.

**The flush.** A flush takes a code with no glyph down the same path as a glyph now, so it needed no pacing of its own. Its time for such a code is that of a symbol, whose walk is as short: 16 of them take 57,730 cycles against Sony's 59,624, where 16 hiragana take 62,786 against 59,528. The flush has no evening out by character, and Sony's time is flat. The images read back are Sony's in every cell a character has. The short flushes here differ from Sony's by a steady 11,000 to 12,700 cycles, glyph or not, which is the flush's pad being a mean over the archive (spec 21) and not this spec's.

### Tests

- The five Kanji probes on both libraries, Sony's against the candidate's, lines that differ:

| probe | scph1001 before | scph1001 now | OpenBIOS now |
|---|---|---|---|
| `kanji` | 15 | 0 | 0 |
| `kanjicodes` | 393 | 0 | 0 |
| `kanjistream` | 818 | 803 | 732 |
| `kanjipast` | 322 | 322 | 316 |
| `kanjilimits` | not run | 13 | 20 |

  What is left in `kanjistream` and `kanjipast` is the cell after the last character in each case that has it and nothing else, under both BIOSes (`cells.py`: 57 of 79 and 32 of 37 cases, each one cell). `kanjilimits` differs in the same cell (cases 22, 40 and 42), in Sony's stall after each 301-character flush, which the emulator now prints as `GPU timeout`, and in case 44, where Sony's doesn't come back: all as specs 13 and 35 have them. Its Sony side was run to 2,500 frames.
- With memory filled (seeds 1, 2 and 3) the first round showed that the fill reaches nothing these calls read, on either library, so the filled runs were not repeated on the candidate: its change reads ROM only.
- `kanjisource` on the candidate: Sony's lines for every code, and for the cell of 81ADh in a stream. The lines left are the cell after the last character, zeros by decision 2. With OpenBIOS every code with no glyph is `blank` on both.
- `TestKanjiNoGlyph` (new, `image/probetest/kanjinoglyph_test.go`) runs `kanjisource`'s clean disc on OpenBIOS and on `PSX_BIOS`. It checks that Krom2Tim draws the 13 codes as the bytes before the table where the ROM has any and blank where it has zeros, that Krom2Tim2 draws them blank, that the stream's cell follows Krom2Tim, and that the cell after the last character is zeros. It passes on the candidate:

      PSX_BIOS=$HOME/.mednafen/firmware/scph1001.bin go test ./exp/cleanlib/image/probetest -run TestKanjiNoGlyph \
          -kanjisource $PWD/.build/s51kanjisource-cimg-clean/s51kanjisource-cimg-clean.cue

- `go test ./emu/... ./exp/cleanlib/...` passes.
- `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`, each with its standing reason), no FAIL. The log is `.build/s51/regress.txt`.
- The image ends at 80067E18h (80067BA8h before), below 80068000h with 488 bytes to spare.

### Left as it is

- Decision 2's cell, and Sony's stall on a full pool (spec 35).
- A Krom2Tim that fails leaves other words than Sony's, with or without a code with no glyph before the bad one, and did before this spec: Sony's has 4 more words changed among the glyphs' bytes and its message's calls under its frame, where the clean library's glyph bytes land elsewhere and its message's calls high in the cleared range. Its time is 1,950 cycles over Sony's of 619,555. Not looked into further.
- The codes with no glyph from 82F2h on, and the Greek, Cyrillic and box drawing glyphs, run 90 to 490 cycles over Sony's in Krom2Tim, as above.
- The first word of an image read back differs between the emulator and the console on both libraries (first round).

### On the console, the candidate

The spec side ran five programs on `libps-s51a` (64b31acc), back to back with no reset (`.build/hardware/spec51/r2`). All reached `END`. No difference found is a new one in the library, and nothing was changed after these runs.

- **`kanjisource`, clean:** the emulator's expected file, line for line. Against r1's Sony file 8 lines differ, where 22 did: the cell after the last character in each of the 8 flushes (decision 2).
- **`kanji`, clean, against spec 35's Sony file:** 16 lines, all in stream cases 54 to 64, and none in a glyph. It is state carried over, not the library: `kanjisource` ran before it and ends with one stream open (n 64), `kanji` opens without closing first, and `hwrun` runs no init between programs. So `kanji`'s open gives id 1, its prints go to stream 0, the earlier program's, and its flushes of id 1 never empty that one. The counts follow to the digit: 10, then 20 where Sony's file has 10, 25 for 5, 54 for 29, 64 and 74 and then -1 once the count has passed 64, and -1 from there on; the second stream is id 2. Sony's file came from a run with no stream open before it. Sony's library would print the same after `kanjisource`, by the rules of spec 13.
- **`kanjistream`, clean, against spec 35's Sony file:** 1,738 lines, where 1,768 differed. By cell, 58 of the 79 cases differ, each in the one cell after the last character and in no other (the emulator has 57: on the console Sony's case 1 held stack from earlier programs). Cases 33 and 62, the codes with no glyph, now differ in that cell only. The probe closes the streams before each case, so nothing carries over but its first two lines, which read 2 and true on both libraries' files. Against the emulator's clean output the console's differs in those two lines and in the first word of 23 images (first round).
- **`kanjinoglyph`, both libraries:** every flush line is the emulator's on each library, and every Krom2Tim and Krom2Tim2 line is but for the fields below.
  - The result, the TIM and the hashes of the glyphs' bytes, the image and the rest of the frame agree between the libraries in all 112 calls but cases 55 and 56.
  - The first stack field, `7:` and a hash on Sony's and `0:811c9dc5` on the clean library, is the range under Sony's frame: Sony's leaves 7 words there from its calls and the clean library none. It was so before this spec and in the emulator, on every case. Not a code with no glyph's.
  - Cases 55 and 56, a Krom2Tim that fails, differ as "Left as it is" says, with or without a code with no glyph.
  - The flush images differ in cases 114 to 120 and 127 to 133, the flushes of one to three characters, and agree in the flushes of 16 and 20. The hash is over the image's first row of 16 cells, which takes in the cell after the last character when the flush has fewer than 16. That cell is zeros on the clean library, so its hash is the same with `isbg` 0 and 1, and on Sony's it is what the stack held, which is another flush's image by the second round, so Sony's two hashes differ. All decision 2. `kanjisource` and `kanjistream` show the other cells equal, cell by cell.

### Console runs wanted

None needed. If the stream cases of `kanji` are to be seen equal on a console: the returning copy of `kanji` with `-library clean` and `libps-s51a`, as the first program after a reset. Expected: no line different from `.build/hardware/group2-kanji/kanji-sony.txt`.
