# The web archive's programs: fifth round

After spec 23, `exp/cleanlib/image/archive.sh` has 91 of 140 programs `same`. Spec 20 tells how to
run things, spec 21 the timing tools, spec 23 the call traces.

## Accuracy comes first, and assembly is open

**The aim inside this library is to match Sony's library as closely as black-box observation
allows.** Readability does not limit that here: it is for the API, and for a later stage that
starts from an accurate base. Earlier specs limited assembly to a few short routines and asked you
to weigh each gain against readability. That no longer applies. This changes nothing about the
clean-room rules: everything still comes from what is observed from outside.

Use assembly wherever it gets closer: whole routines in `.S` where Go's output can't take the
cycles out, such as the compiler's load-delay nops and `$gp` reads, and inline for small sequences.
`device.AsmFull` has no memory operands. Prefer code that really costs the right number of cycles
to a pad over quicker code: a pad is a stand-in, and where the real code gets the cost right per
path the pad should go. `exp/cleanlib/image/clib.S` is the image's assembly file and
`internal/toolchain/_tinygo/targets/playstation-yaroze-clean.S` the linked-in target's. Whatever you
write must build for both, since `exp/cleanlib/regress.sh` uses the second.

## Tools added

- `TestArchiveCalls` now presses the pad as the archive test does, notes ` prints N` on a call that
  printed, and saves `calls/NAME-SIDE.tty`. `ARCHIVE_CALLS_A0=GsSortSprite,GsSortFastSprite` adds the
  10 words at a0 for those entries.
- `ARCHIVE_TEXT="NAME ..."`, `TestArchiveText`, compares what programs send the GPU over a whole run
  with the font's 8 by 8 glyphs left out, into `.build/carchive/text.txt`.

The second settles a class: `balls`, `chap5-1`, `morph`, `hhpd0002`, `bmirror`, `sample-cube`, `bomb`,
`dungeon`, `dyddem01`, `dydref01`, `dydtst02`, `wnfs` and `wstg-tv3` send the same words as Sony's, to
the last one of 1,200 frames, without the glyphs. They differ only in the figures they print. Keep
them that way: run `TestArchiveText` on them after your work.

## Behavior

**GsSortSprite, scaled and rotated.** `event-event002-hhmd0001` gets the same GsSPRITE on both sides,
but corners 1 pixel apart at its frame 348. This one, attribute 01000000h:

    x E6h, y 54h, w 10h, h 10h, mx 8, my 8, scalex and scaley 0BFCh, rotate 00C45000h

gives, Sony's then the clean library's, in order of the packet:

    (E9,62) (DF,5F) (EC,58) (E2,55)
    (E8,62) (DF,5E) (EC,59) (E3,55)

Others in the same frame have rotate 007F4000h, 0030F000h, 0102C000h, 00A7E000h, 00313000h, 0072D000h,
000A6000h, 0097C000h, 00695000h, 005A0000h, 00794000h, 0079B000h, 00744000h, 0020C000h and 00DE9000h,
with scale 0E60h down to 08CCh by 0CCh. These go beyond 360 degrees. Probe a sweep of scale and rotate on
Sony's, and find the rule. Only the first difference is known, so others may follow it.

**CdPlay(2, list, 0)**: `pssoft-fatal` calls it at frame 720 with a track its disc does not have, and
Sony's prints `2: track overflow` and a newline, 19 bytes with whatever the TTY adds. The clean library
prints nothing. Probe CdPlay's modes on tracks past the disc's last: the message, its number, the
result, what happens to the drive, and the next CdPlay and the time each takes.

**CdReadFile of a missing file.** In `demos-scee-chap3-1`, after its frame 360, the program looks for a
file the disc does not have. CdSearchFile returns 0 on both. CdReadFile(name, address, 0) returns 0 on
both. Then CdReadSync(1) on Sony's returns 1 for 650,022 cycles from the CdReadFile, 815 polls at about
484 cycles each, and then 0. The clean library returns 0 at the first poll, so the program goes round
far more often and prints more. Probe what Sony's does in that time, and if a different earlier state
changes it.

## Time

**GsSortObject4 first.** Sony's 4,330 a call and the clean library's 4,861 on `costs.txt`. Spec 22's
work found unlit kinds near the same and lit kinds about 30 to 70 cycles a polygon over. This is what
holds `test-cube`, `test-pd961207`, `test-gr-01` and `event-dengeki001-nanatan-nanatan` back. Write its
loops in assembly and bring each kind of polygon, under each GsDIV level a program uses, to Sony's
cycles per polygon, by probe.

**Then in order of total in `costs.txt`, all with assembly open**, Sony/clean:

| call | cycles |
|---|---|
| ApplyMatrix | 47/64 |
| ApplyMatrixSV | 53/81 |
| ApplyMatrixLV | 124/156 |
| RotMatrix | 243/327 |
| RotMatrixX, Y, Z | 178/260 |
| GsLinkObject4 | 470/555 |
| GsSortSprite | 246/264 |
| FntFlush | 2,649/2,831 |
| FntPrint | 941/1,029 |
| printf | 19,560/20,233 (each directive about 480 over) |
| gteMIMefunc | 5,468/6,422 |
| sprintf | 646/820 |
| GsSortFixBg16 | 15,576/15,909 |
| rand | 24/28 |
| TransMatrix | 18/30 |
| PushMatrix, PopMatrix | 62/137 |
| StartRCnt, GetRCnt, ResetRCnt | 26/95 |

**GsSortFastSprite by attribute.** Mean 95/94, but `scee-topten-cardem` has attribute 50000000h and
51000000h, with a gap to the next call of 306/290 and 674/658: the clean library is 16 cycles a call
quick on them, which is 1.3 frames by frame 40 there, and the whole of its difference. Probe the time by
attribute.

**printf of Shift-JIS text.** `event-event001-rave096` calls printf(8015130Ch) with a literal of two-byte
characters over and over, 15,788 cycles between calls on Sony's and 17,358 on the clean library, and
ends a run at 91% of Sony's output. Literal characters matched on a probe of plain ASCII: probe bytes
with the top bit set.

**Sin and cos** still have about 10,000 cycles' spread by argument with no pattern found. Spec 23's
figures get `event-event002-r3` right and leave `test-demo-fm` 2,000 a call short. Take this last. If you
can find what it follows, a table is fine.

## Not to chase

As spec 23, and these:

- `scee-topten-hover`: the first difference is a GsSPRITE on the program's stack whose x and y it hasn't
  set yet, at its frame 602. It's 18 of 15,235 sprites, and the program's own.
- `event-event002-pokris-pokris`: `srand(GetRCnt(...))` when the pad is pressed.

## Acceptance

- `FIRST=1 exp/cleanlib/image/archive.sh event-event002-hhmd0001 pssoft-fatal demos-scee-chap3-1
  scee-topten-cardem event-event001-rave096 test-cube test-pd961207 test-gr-01
  event-dengeki001-nanatan-nanatan test-demo-fm pssoft-koro-koro scee-topten-adven test-ftptest
  pssoft-airbob-2-airbob`. Say what each shows. `hhmd0001`, `fatal`, `chap3-1` and `cardem` should be
  `same`, or say what stops them.
- `TestArchiveText` on the 13 programs above: still no difference.
- `exp/cleanlib/regress.sh` without `PSX_BIOS` in the environment: FAIL stays 0, and a new TIME has to be
  explained. `exp/cleanlib/image/regress.sh`: FAIL stays 0. `exp/cleanlib/image/terra.sh`: all pass.
- The full `exp/cleanlib/image/archive.sh`, with counts before and after, and name anything that got
  worse. Before: 91 same, 31 differ, 3 shift, 10 noboot, 3 tty, 1 stall, 1 wild.
- `costs.txt` refreshed, with before and after for the table above.

## What the work on this spec found

Probes of Sony's library by the agent that did the work. Where these disagree with the sections
above or with earlier specs, these are right.

**GsSortSprite**: the angle is Rotate/360 taken in 16 bits, so a turn past 32767 is a negative angle. A
scale other than 4096 on either axis leaves R33 at 0 in the GTE.

**CdPlay**: a track past the disc's last prints `N: track overflow` in decimal with `\r\n`, by the
library's own printing, and plays the last track. A negative track plays from 00:00:00. An offset at or
past the list's end goes to 0 in mode 2, and returns -1 in mode 1. Any mode but 0 and 3 starts a list:
mode 4 and -1 start, and whether they repeat is not known, so the clean library plays them once. Mode 3's
replay keeps the place the start set and does not print again. A track's end moving on to an overflow
does print.

**CdReadSync** is not about the missing file. Any CdReadSync, mode 0 or 1, more than 60 blanks after the
last read ended makes that read again: it sends Pause, waits for Pause's second answer, then sends Setloc,
Setmode A0h and ReadN. Polling does not put it off, and with no read yet nothing happens. The library's own
directory reads count. `demos-scee-chap3-1`'s 650,022 cycles are that.

**GetRCnt, StartRCnt, ResetRCnt** (spec 02 is wrong): Sony's makes no kernel call. It uses the registers,
taking the low half of the spec as the counter's number. Counter 3 returns 0 and does nothing.

**GsSortObject4's order**: FLAG is tested before NCLIP. A cut polygon drops off-screen before lighting, and
one turned away costs about 192.

**Left over, on probes.** Polygons with a bad FLAG are 4 cycles short on most kinds, 8 over on lit G3. Lit
gouraud quads sorted are +12 to +20, lit 28h +10, and a quad whose fourth point is bad up to +42. Cut
polygons and gradated triangles stay in Go: DIV1 sorted about 34% over (942 against 698 for mode 20), DIV2
about 15%, and a bad point under a cut about 2.2 times Sony's. `test-dungeon-dungeon`, `test-dyddem01` and
`test-dydref01` use DIV1. FntFlush is about 180 over in the queue path, and printf about 90 a digit over.
RotMatrix is 258 against 252. Sin and cos, GsSortFixBg16 and sprintf were not touched. The sprite quad with
a turn is +8 to +10. gteMIMefunc with n of 0 did not return on Sony's in a probe and returns at once on the
clean library.

**Not explained.** On real models Sony's GsSortObject4 takes more than any probe path, and the clean library
does not: a 179-polygon 28h model in `test-gr-01` has a gap to the next call of 33,937 against 30,492, and a
3-polygon one 1,018 against 1,000. That leaves `test-gr-01` and `test-pd961207` ahead now, and the archive
mean at 4,211/3,981. Probes ruled out run lengths, facing mixes, kind changes, changing normals and colors,
each polygon having its own vertices and depths, turning, off-center translation, the scratch address, the
shift, the light mode, saturated lighting, IR0 saturated and off-screen polygons. GsLinkObject4 is exact on
every probe, yet its archive mean is 470/542.

Not rerun on the final image: `cdplay`'s times, `cdmiss`, the sprite quad's pads, `matrix` and `gtelib`.
