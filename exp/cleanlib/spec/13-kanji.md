# Milestone 12: Kanji text

Status: implemented, in exp/cleanlib/kanji.go. The probe
`spec/probes/kanji` gives the same TTY and the same GPU words between
markers on the clean library as on Sony's, all 68 cases. Two more probes
beside it, `spec/probes/kanjicodes` (Krom2Tim and Krom2Tim2 over 2,500
codes and strings, every TIM word hashed and the strings' TIMs printed
whole) and `spec/probes/kanjistream` (84 stream cases, with the glyph
image read back from video memory), match too, but for the one cell of a
stream's glyph image that no quadrangle draws (see KanjiFntFlush). A
fourth, `spec/probes/kanjipast` (73 cases: bad codes after long runs and
destinations off the page for Krom2Tim and Krom2Tim2, and streams whose
text runs past the window's end, windows of no width or height, and glyph
images off a page's corner), was run on Sony's library and on the
reference library (`--libps go`) on 2026-11-03: the GPU words between
markers are the same on both, case for case, and the TTY differs only in
the KanjiFntPrint return value noted under KanjiFntPrint (the reference
returns the count since the same day). The clean library matches too
since 2026-11-03: the GPU words case for case, and the TTY as the
reference's, the stale undrawn cell in the images read back being the
only difference from Sony's. Its GPU stream matched only once its
StoreImage read whole blocks of 16 words by DMA, with GP1 04000003h
after the size word, as Sony's does for the 256- and 640-word images
here and in kanjistream (spec 08, StoreImage). A fifth,
`spec/probes/kanjilimits` (45 cases: which n KanjiFntOpen refuses,
flushes of 64 to 302 characters, and text printed past the pool's end),
was run on Sony's library on 2026-11-03 and gives the limits under
KanjiFntOpen, KanjiFntPrint and KanjiFntFlush; the clean library matches
it in the GPU words of all 45 cases but where "Limits of the clean library"
says.

The calls:

- Krom2Tim and Krom2Tim2: render a Shift JIS string from the console's
  kanji font into a 4-bit TIM in memory, for LoadImage.
- KanjiFntOpen, KanjiFntPrint, KanjiFntFlush and KanjiFntClose: print
  streams of Shift JIS text, like FntOpen and the rest (spec 01), drawn
  with glyphs from the same font.

No example in `examples/` makes these calls. The work is a probe that
must match Sony's library, TTY and GPU stream.

## The kanji font

The console's BIOS ROM holds a Shift JIS font: 16 by 15 pixel glyphs of
the non-kanji characters and the first level kanji, at 1 bit per pixel,
at BFC66000h on Sony's BIOS (public hardware documentation, psx-spx,
"BIOS Character Sets"). The emulator runs OpenBIOS
(`emu/openbios/openbios.bin`), which has font data at the same offset,
66000h. Both libraries read the ROM the emulator gives them, so the
probe compares them on that font. Where a result depends on the glyph
data (which codes have a glyph, which are blank), work it out from the
ROM's bytes and from Sony's results, never from a guess about the real
BIOS. The manual's member FAQ (`docs/manuals/member-faq.md`) says only
non-kanji and first level kanji are there.

## How it is checked

`spec/probes/kanji` (spec side, do not edit it; add a probe of your own
beside it if you need more cases) makes:

- Krom2Tim, then Krom2Tim2, over 23 strings: single characters from
  each part of the code space (hiragana, full-width letters, the first
  and last first level kanji, second level kanji, Cyrillic, an empty
  row, an empty cell, past the last row, the user-defined area, trail
  bytes out of range), ASCII alone and between two characters,
  half-width katakana, a lead byte alone, the empty string, and 16 and
  17 characters. String 1 is also made at four other places and colors
  (dx, dy, cdx, cdy, fg, bg), with odd positions and bits set past 15.
  Before each call the TIM buffer is filled with 5A5A5A5Ah. Each prints
  the result, the last word written, the first 32 words, and a hash of
  all words written.
- The Kanji print streams: open, print, flush, flush empty, print and
  flush again, other text, a newline and a line past the width, more
  than n characters, percent signs, a second stream with isbg, which
  stream KanjiFntPrint writes to, close, reopen, and a stream next to
  the ASCII font's. Each prints the results.

Each case sits between two GP1 marker words, 10000100h plus its number.

Build it for Sony's library:

    go run ./cmd/yaroze disc --opt=2 --elf .build/kanjiprobe-sony/prog.elf \
        -o .build/kanjiprobe-sony/kanjiprobe-sony.bin exp/cleanlib/spec/probes/kanji

and for the clean one with `exp/cleanlib/build.sh kanjiprobe-clean
exp/cleanlib/spec/probes/kanji`. The TTY comes from `.build/voiceprobe`
(TestDump, DISC=... FRAMES=1500 OUT=..., the TTY in OUT.tty), the GPU
stream from `.build/gpuprobe` (TestGPU). Compare the GPU words between
markers, including the image data each LoadImage sends.

The words a LoadImage sends by DMA do not show in TestGPU's stream, only
those the CPU writes, so a stream's glyph image is not compared there:
`spec/probes/kanjistream` reads it back with StoreImage after each case
and prints it. It and `spec/probes/kanjicodes` build and run the same
way (kanjicodes needs about 16,000 frames, kanjistream 8,000 on the
clean library). In a probe that makes thousands of calls, never range
over a large array, even for its index alone: TinyGo copies it to the
heap each time, and with this target's leaking collector the program
runs out of memory.

`spec/probes/kanjilimits` builds and runs the same way (3,000 frames;
Sony's library does not get past case 44). Its cases 0 to 17 are
KanjiFntOpen with n values alone and summed (each line the ids
returned), 18 to 20 streams of n 1023, -1 and -100, 21 to 23 and 40 to
43 the pool (a hash of each of 17 cells of the glyph image read back,
by which the glyphs a second stream draws are matched to the first's
characters; the first cell's hash is not to be trusted, the first word
of Sony's image being overwritten before the DMA, as noted under
KanjiFntFlush), 24 to 39 flushes of 64 to 301 characters and 44 the
flush of 302. Its window is 1,024 by 1,024 at 0, 0, the glyph image at
768, 0 and the palette at 896, 504; its text is hiragana 82A0h to 82D1h
in turn, so that a cell tells which character it holds.

`spec/probes/kanjipast` builds and runs the same way (4,500 frames on
either library). Its cases 0 to 17 are Krom2Tim, 18 to 35 the same
strings on Krom2Tim2, 36 to 72 the streams; each prints a line of
results, a Krom2Tim case the TIM's words 3 and 13 to 15 and a hash of all
words written, a stream case the glyph image read back and, for the dx,
dy cases, the pixels drawn in the window's first two cells, read back
after the flush (the window and the page's corner having been cleared to
known colors first). Its GPU stream came from `.build/gpuprobe.test`
(`go test -c -o .build/gpuprobe.test ./.build/gpuprobe`), compared line
for line from the first `case 0:` on.

Seen on Sony's library so far: one character writes 48 words (192
bytes), three 112, 16 characters 528 and 17 characters 1040, so the
manual's size (128 bytes a character plus 84) is not the layout.
Krom2Tim returns -1 for second level kanji, an empty row, past the last
row and the user-defined area; Krom2Tim2 returns 0 for some of those and
-1 for others, after writing part of the TIM. KanjiFntPrint returns the
characters in the stream, and -1 past n. The probe is the authority:
find the rules from its TTY and GPU stream.

## Rules found

The rules below come from the three probes' results on Sony's library,
the ROM's bytes and public documentation; none from Sony's code.

### The ROM font and the codes

The font is two tables of 30-byte glyphs, 16 pixels wide and 15 high, a
row in two bytes with the leftmost pixel in the top bit:

- At BFC66000h, the non-kanji characters, JIS rows 1 to 8 (Shift JIS
  8140h to 84BEh), in JIS order with the unassigned cells left out: 524
  glyphs. The assigned cells, as Shift JIS ranges: 8140h-817Eh,
  8180h-81ACh, 81B8h-81BFh, 81C8h-81CEh, 81DAh-81E8h, 81F0h-81F7h, 81FCh
  (symbols); 824Fh-8258h, 8260h-8279h, 8281h-829Ah (digits and Latin
  letters); 829Fh-82F1h (hiragana); 8340h-837Eh, 8380h-8396h (katakana);
  839Fh-83B6h, 83BFh-83D6h (Greek); 8440h-8460h, 8470h-847Eh,
  8480h-8491h (Cyrillic); 849Fh-84BEh (box drawing).
- At BFC69D68h, the first level kanji, JIS rows 16 to 47 (889Fh to
  9872h), 94 to a row with none left out: glyph (row-16)*94 + cell-1.

The kernel's Krom2RawAdd (B(51h)) on OpenBIOS gives the same addresses
for every code tried, and Sony's results match these tables glyph for
glyph. A Shift JIS code converts to its JIS row and cell the usual way:
for lead byte L and trail byte T, row (L-81h)*2+1 and cell T-3Fh for T
from 40h to 7Eh, cell T-40h for T from 80h to 9Eh, and row (L-81h)*2+2,
cell T-9Eh for T from 9Fh to FCh.

A string is read a character at a time. A byte from 81h to 98h is a lead
byte: it and the byte after it, whatever that is, the NUL included, make
a two-byte code, and reading goes on after them. (So a lead byte just
before the string's NUL makes one blank cell, and reading goes on past
the NUL to the next byte.) Lead bytes 99h to 9Fh and E0h to EFh are not
taken as lead bytes: they are single bytes. Any other byte is a single
byte, and a NUL ends the string.

What a character draws as (Krom2Tim's reading; Krom2Tim2's differs where
it says):

- A newline (0Ah) draws nothing (see the layout).
- A byte from 20h to 7Eh is drawn as its full-width character: space
  8140h, ! 8149h, " 8168h, # 8194h, $ 8190h, % 8193h, & 8195h, ' 8166h,
  ( 8169h, ) 816Ah, * 8196h, + 817Bh, comma 8143h, - 817Ch, . 8144h,
  / 815Eh, the digits 824Fh on, : 8146h, ; 8147h, < 8171h, = 8181h,
  > 8172h, ? 8148h, @ 8197h, the capitals 8260h on, [ 816Dh, \ 818Fh
  (the yen sign), ] 816Eh, ^ 814Fh, _ 8151h, ` 8165h, the small letters
  8281h on, { 816Fh, | 8162h, } 8170h, ~ 8150h.
- Any other single byte (control characters but the newline, 7Fh, 80h,
  99h to FFh) is not drawable: Krom2Tim prints `bad hankaku code 0x%x`
  and a newline (the byte in lower-case hexadecimal, no leading zeros)
  and fails; Krom2Tim2 draws a blank cell and returns -1 at the end.
- A two-byte code from 8140h to 84BEh draws its glyph if its cell is
  assigned, and a blank cell if not (an unassigned cell, or a trail byte
  below 40h, 7Fh or above FCh). This is so for both calls.
- A two-byte code from 889Fh to 9872h draws its kanji. With a trail byte
  below 40h, 7Fh or above FCh, Krom2Tim draws the glyph just before the
  first kanji, at BFC69D4Ah (the last box drawing character's), and
  Krom2Tim2 a blank cell.
- Any other two-byte code (lead 81h to 98h outside those ranges: 85h to
  87h, 8800h to 889Eh, 9873h on, the second level kanji) is not in the
  font: Krom2Tim prints `bad sjis code 0x%x` and a newline and fails;
  Krom2Tim2 draws a blank cell and still returns 0.

### Krom2Tim

Krom2Tim reads the whole string first. At the first character that is
not drawable it prints the line above and returns -1, having written
nothing to taddr; only that one line is printed. The whole string is
checked, however long: a bad code after 17, 33, 64, 128 or 200 characters
is reported the same way, with nothing written (kanjipast cases 0, 1, 3,
4 and 5), as is one after a newline that follows a full row (case 6), and
with two bad characters it is the first in the string that is reported,
whichever kind it is (cases 7 to 10: a bad single byte then a bad code,
and the other way around). The 200 character string makes a TIM 13 rows
high on Krom2Tim2 (case 23), so no limit on the rows was met.

Otherwise it writes a 4-bit TIM with a palette and returns 0. The
characters are laid out in rows of 16 cells, each cell 16 by 16 pixels:
each character takes the next cell, the next row's first after the
sixteenth, and a newline moves to the start of the next row (so a
newline right after a full row leaves an empty row). The image is as
wide as the characters, up to 16 cells, or 16 cells whenever the string
has a newline; it is as high as the rows used, a row counted once
anything is in it or a newline has moved past it, and at least one row
high. The empty string makes a TIM one row high and 0 wide.

The TIM, in words:

- 0: 10h, the ID. 1: 8, 4-bit with a palette.
- 2: 2Ch, the palette block's bytes. 3: cy<<16 | cx, a 32-bit OR, so
  bits of cx past 15 land in cy's half: cx and cy both 70000 (11170h)
  give 11711170h, not 11701170h (kanjipast case 17). 4: 00010010h, 16
  colors by one row.
- 5: fg<<16 | bg, a 32-bit OR: color 0 is bg's low 16 bits and color 1
  fg's low 16 bits ORed with bg's high 16 (fg 12345h with bg FEDCBA98h
  gives FFDDBA98h).
- 6 to 12: 0, the other 14 colors.
- 13: the pixel block's bytes, 12 + 8*cells wide*16*rows.
- 14: dy<<16 | dx, the same 32-bit OR (dx and dy both 70000 give
  11711170h, kanjipast case 16; -4 and -16 give FFFFFFFCh, case 15).
  15: (16*rows)<<16 | 4*cells wide, the size in 16-bit units.
- Then the pixels, row by row, two words for each cell of a row: pixel x
  of a cell in nibble x (the low nibble first), 1 where the glyph's bit
  is set and 0 where not. A cell's rows 0 to 14 are the glyph's, row 15
  is always 0, and cells with no character are 0.

So a character takes 32 words and the header 16: 1 character 48 words,
3 characters 112, 16 characters 528, 17 characters 1040 (two rows of 16
cells). The manual's 128 bytes a character plus 84 is not the layout.

Where the TIM is for does not change it: dx, dy only go into word 14.
With 16 characters at 1020, 510 and 17 at 1020, 510, at 1008, 496 and at
960, 240 (an image 256 pixels wide and 32 high, which would run off the
right edge, off the bottom, or over a page's bottom edge once loaded),
the extent written and the size word are those of the same string at
640, 0 and only word 14 differs; nothing is clipped or moved (kanjipast
cases 11 to 14, both calls). That the pixel words are the same as at
640, 0 is inferred from the extent and the size word; the probe prints a
hash of the whole TIM, which includes word 14. Krom2Tim draws nothing
itself, so what a sprite made from such a TIM looks like is up to the
LoadImage and the primitives the program makes.

### Krom2Tim2

Krom2Tim2 is Krom2Tim with the differences given under the codes: it
prints nothing, never fails early, draws a blank cell for every two-byte
code with no glyph (and for kanji-range codes with a bad trail byte),
and draws a blank cell for a byte that is not drawable, returning -1
once the whole TIM is written. The layout and the TIM are Krom2Tim's.

### KanjiFntOpen

KanjiFntOpen(x, y, w, h, dx, dy, cx, cy, isbg, n) first loads the
streams' palette, 16 colors by one row at cx, cy: the words FFFF0000h
and seven zeros (color 0 transparent black, color 1 white), the 8 words
written by the CPU as LoadImage does for an image under 16 words. It
does so on every call, even one that then fails.

At most 4 streams can be open; a fifth returns -1. The id returned
counts up from 0. The streams share a pool of 1,024 bytes of text: each
takes n bytes of it, after the streams opened before it, and its text
starts empty (a NUL at its start). Nothing stops a stream's text from
running on into the next stream's bytes: with stream 0 of n 2 holding 4
bytes, stream 1 sees the last two as its own text, and flushing stream
1 (which ends its text with a NUL) cuts stream 0's.

The pool bounds n: the open is refused, returning -1, when the n values
of the open streams and the new one add up to more than 1,023 (so the
last byte of the pool is never given out). From kanjilimits cases 0 to
17: n 1023 alone is accepted and 1024, 1025, 2048, 65536 and 1<<30
refused; after 1023, 0 is accepted and 1 refused; after 1022, 1 is
accepted; 512 then 512 is refused; after 1016, 8 is refused, 7 accepted
and then 1 refused; 256 is accepted three times and refused the fourth.
A refused open takes no id and adds nothing to the sum: 512, 512 refused
and then 1 gives ids 0, -1, 1, and 1<<30 refused and then four of n 1
give -1, 0, 1, 2, 3. The sum is signed, and a negative n lowers it:
after -100, 1123 is accepted and 1124 refused (cases 14 to 16). A
stream of negative n is otherwise as one of n 0: KanjiFntPrint to it
returns -1 at once and its flush sends the one-cell image and no
quadrangle (cases 19 and 20, n -1 and -100). Where the text of a stream
opened after one of negative n sits was not probed. The palette is
loaded by a refused open too, as above.

The window is x, y, w by h. The glyph image goes at dx, dy, drawn
through texture page GetTPage(0, 0, dx, dy) with u and v from 0 (dx and
dy that are not on a page's edge are not allowed for), and the palette
cy<<6 | cx>>4.

What that means for a dx, dy off a page's corner, from kanjipast cases
68 to 72 (the window's first cells and the page's corner cleared to
known colors before each flush, the window read back after it):

- dx 770, dy 300 (case 68): the image is loaded at 770, 300 (the
  LoadImage words 012C0302h, 0010000Ch), the quadrangles carry texture
  page 1Ch (768, 256) and u, v from 0, and the pixels drawn in the
  window are the page corner's texels at 768, 256 (the clear color's
  nibbles through the palette), not the glyphs.
- dx 1000, dy 300 with 17 characters (case 69): the image rectangle is
  sent as given, 1000, 300, 64 by 32 units (012C03E8h, 00200040h), which
  runs off video memory's right edge; the page is 1Fh (960, 256), u, v
  from 0, cell 16 at v 16; the window shows the corner's texels.
- dx 992, dy 496 with 17 (case 70): the rectangle 01F003E0h, 00200040h
  runs off the bottom edge, page 1Fh; the window shows the corner's
  texels, mixed in the probe with the image's own second row, which the
  GPU wrapped to the top of video memory where the window happened to
  be.
- dx 960, dy 240 with 17 (case 71): the rectangle 00F003C0h, 00200040h
  straddles the page edge at 256; the page is 0Fh (960, 0), so the
  quadrangles read the top of that page, not the image, and the window
  shows the corner's texels at 960, 0.
- dx 768, dy 256 with 17 (case 72, the control): page 1Ch, the glyphs
  drawn.

So neither call allows for the offset: the image goes where dx, dy say,
whole and unclipped, and the drawing reads the page that contains dx,
dy from its corner.

### KanjiFntPrint

KanjiFntPrint(format, ...) always writes to stream 0, the first opened,
whatever the manual's id says (the binding passes the string only).

- With no stream open it returns -1.
- If the stream's count is already past its n, it returns -1 and adds
  nothing. Otherwise the whole formatted text is added, however long:
  the count is checked before the text is added, not after.
- The text is formatted as printf does (%% gives %). The count goes up
  by the formatted text's bytes, with the text each conversion makes
  counted twice (Sony's library returns 25 for "%d" printing an 11-digit
  number followed by three bytes); the text is added at the stream's
  NUL, so the count can run ahead of it. It returns the new count.
- The count is of bytes, not characters: a kanji counts 2, an ASCII
  character 1.
- The new count is returned even when it has passed n: 10 bytes into an
  empty stream of n 4 or 5 return 10, 34 bytes into one of n 32 return
  34, and the text is all there for the flush (kanjipast cases 48, 49
  and 69 to 72). The reference library (`--libps go`) returns -1 in
  those cases while still adding the text (its flush draws the same
  quadrangles); the clean library follows Sony.
- Nothing checks the pool's end either: the text is written on past
  byte 1,024 into the library's memory after the pool. In kanjilimits
  cases 40 and 42, stream A of n 1008 and stream B of n 15 (at offset
  1008) are opened, 520 or 1,000 characters (1,040 or 2,000 bytes) are
  printed into A, returning 1040 and 2000, and B is flushed: it draws 15
  glyphs, the characters at bytes 1008 to 1037, those at 1024 to 1037
  included (their cells match the reference flush of case 23), where
  text cut at the pool's end would give 8. What follows the pool is the
  flush's packets: see KanjiFntFlush.

The clean library's binding passes no arguments for the directives: it
formats each numeric conversion (d, i, u, o, x, X, p) as 0 and the
others as nothing. The probe's "%d" case only needs the conversion to
make one character, which Sony's does with what the probe leaves in the
argument register.

### KanjiFntFlush

KanjiFntFlush(id) returns NULL, drawing nothing, if no stream is open.
An id that is not an open stream's (negative, or past the last opened)
flushes stream 0.

The flush reads the stream's text from its start to the NUL, at most n
characters, newlines, tabs, spaces and tildes counted together, and lays
it out in the window, x and y from 0 at the window's corner:

- A newline: x to 0, y down 16.
- A tab: x right 64. A space: x right 16, drawn by no quadrangle and
  taking no cell. A tilde: nothing at all.
- A drawable character (the codes' rules, Krom2Tim's reading) is drawn
  at x, y in the next cell, and x moves right 16.
- Whenever x moves (after a character, a space or a tab) and comes to or
  past w, x goes to 0 and y down 16. Whenever y moves down (so, or by a
  newline) and comes to or past h, the text ends there: nothing after it
  is placed or looked at.
- A two-byte code outside the font prints `bad sjis code 0x%x` and a
  newline, and a single byte that is not drawable prints nothing; either
  way the flush returns NULL having sent nothing, and the stream keeps
  its text and count. The codes are checked as the text is laid out, so
  one past the point where the text ends is not checked (below).

The wrap is made as x moves, not as the next character is placed, and y
is checked only as it moves, which the kanjipast cases show:

- A character at x 0 is drawn whatever w is: with w 0 three characters
  go to (0, 0), (0, 16) and (0, 32), and with w 10 two go to (0, 0) and
  (0, 16) (cases 58, 59).
- A character at y 0 is drawn whatever h is: one character in a window
  of h 0, 10 or 16 is drawn at (0, 0) (cases 51 to 53). After a newline
  y 16 in a window of h 16 ends the text, and so does y 32 after two
  newlines in one of h 20 (cases 54, 55): the character after is not
  drawn and the flush sends the one-cell image and no quadrangle.
- Two characters that fill w 32 exactly wrap x at once, so a newline
  after them moves to y 32 and the next character is drawn at (0, 32),
  not (0, 16) (case 62; the same with w 16 and one character, case 64).
  A tab after them, moving x to 64, wraps again: the next character is at
  (0, 32) (case 65). A space after them moves x to 16 on the new row and
  the next character is at (16, 16) (case 66).

What comes after the text has ended has no effect at all, TTY or GPU: in
a window of w 32 and h 20, four characters fill two rows and the fifth
wraps to y 32, ending the text. A two-byte code outside the font there,
a second level kanji, a bad single byte (01h or 80h), a newline, tab,
space or tilde followed by a character, or a bad trail byte followed by
a character (cases 38 to 47) all give the same four quadrangles and the
same five-cell image as five good characters (case 36), with nothing
printed and the flush returning the ordering table; the stream is
emptied as usual (case 50: a flush right after sends the one-cell image
and the next print returns 2). The same bad code as the fourth
character, inside the window, prints `bad sjis code 0x8540` and fails
(case 37). The text also ends when a newline or a wrap moves y past h
before anything more is drawn (two newlines then a bad code in h 20,
case 57; a character whose wrap takes y to h, then a bad code, with w 0
and h 16 or with two characters in w 32 and h 16, cases 61 and 67; two
characters filling w 32, a newline and a bad code in h 20, case 63): no
message, the flush succeeds. And the n limit
comes first: with n 4 the fifth character is not read at all (case 48),
with n 5 it is read but is past the end (case 49); both succeed. Whether
the text ends or is only skipped from there on cannot be told apart from
outside; the clean library ends it.

Then it loads the glyph image and queues its drawing:

- The image has a cell for each character drawn and one more, laid out
  as Krom2Tim lays out a string without newlines: rows of 16 cells,
  16*cells wide up to 16 cells (4 per cell in 16-bit units) and 16 high
  for each row. It goes to dx, dy by LoadImage, which, being whole
  blocks of 16 words, all goes by DMA. The last cell, which nothing
  draws, holds whatever Sony's buffer held (a stale glyph, in the
  probes); the clean library leaves it blank. Sony's image also often
  has its first row's first word overwritten before the DMA reads it
  (the probes read back words from the GPU's drawing area registers
  there); the clean library's does not.
- Then a packet list through a one-entry ordering table: with isbg not
  0, a black monochrome rectangle (60000000h, x, y, w, h) over the
  window first, and then a quadrangle for each character drawn, the last
  character's first. Each quadrangle is 9 words, 2D000000h (textured,
  opaque, raw texture) then the corners x, y; x+15, y; x, y+15; x+15,
  y+15 with u, v of the cell (u 16*(k%16), v 16*(k/16) for cell k) to
  u+15, v+15, the palette with the first and the texture page with the
  second. No texture page packet is sent. An empty stream still loads
  its one-cell image and sends the list (empty but for the rectangle).
- The bytes of a quadrangle the flush does not set (the command word's
  three color bytes and the high halves of the third and fourth texture
  words, 0 in every probe so far) are whatever its memory holds: the
  packets lie right after the text pool, a quadrangle 40 bytes from the
  one before (its link word first), the first character's first, and
  text printed past the pool's end lands in them. In kanjilimits case
  41, after 1,041 bytes went into the pool (case 40), a flush of 64
  characters has 2D82AE82h for its last quadrangle's command word (the
  first cell's) and the rest as usual; in case 43, after 2,001 bytes
  (case 42), the last 25 quadrangles carry text bytes in those places
  (25 times 40 is the least multiple past 977) and the other 39 are
  clean. Cases 40 and 42 themselves, stream B at offset 1008, send clean
  quadrangles right after that text was printed: a stream's packets start
  at the place its offset in the pool gives (offset times 40 bytes after
  the pool's end; inferred from these words, and it makes a stream's n
  bound its packets as it bounds its text), so that B's lie far past
  A's text and a stream at offset 0 has the first.
- The texture words keep v to a byte (case 31, cell 288: 7E3820B0h): the
  rows past the page, from cell 256, carry v wrapped.

It then empties the stream (count 0, a NUL at its start) and returns the
ordering table.

How many characters one flush draws (kanjilimits cases 24 to 39 and
44): every one, up to 301. Flushes of 64, 100, 127, 128, 129, 200, 256,
300 and 301 characters in a stream of n 600 (and 301 in one of n 1023)
send as many quadrangles and an image of as many cells plus one, 16
cells wide and 5, 7, 8, 9, 9, 13, 17, 19 and 19 rows high (the size
words 00500040h, 00700040h, 00800040h, 00900040h, 00900040h,
00D00040h, 01100040h, 01300040h, 01300040h). KanjiFntPrint returns
twice the count. A flush that would draw 302 does not come back: it
loads the image, 19 rows as for 301, and then sends some 20,000 words
of garbage and never returns (case 44, n 1023, after which the TTY
ends; in other runs 302 characters with n 600, and 300 kanji followed
by two ASCII letters with n 600, did the same, and 500 and 320 did too).
The limit is on the characters drawn, not read: 301 characters followed
by a space, a newline, a tilde, a tab, seven spaces or four newlines
flush as 301 alone does (cases 34 to 39). Whether the limit depends on
the window (every flush here had 64 cells to a row) was not probed.

The count and the image hold up to 301, but the longest flushes are
already damaged. Packets go out last character first. At 300 characters
(case 31) one quadrangle differs from what the rule gives: the 282nd
character's has y 0 in its four place words (00000190h for 00400190h,
the rest of the packet right). At 301 (cases 32 to 39) the same one
differs, the last character's has u 2Ch for C0h (7E38202Ch for
7E3820C0h), and after the 301 quadrangles the list runs on for 36 more
words, beginning 01000008h and then zeros and small numbers, none of
them a quadrangle. A flush of 256 is clean and 257 to 299 were not
walked. Places past some 280 evidently overrun the library's own data,
the same overrun that breaks at 302. Neither the reference nor the
clean library reproduces it: both draw all 301 clean. A window has to
be far larger than the screen to reach this.

The reference (2026-11-03): `kanjiCells` lets a flush fetch 301
characters, and the 302nd is treated as one the ROM has not, so the
flush returns nil and draws nothing where Sony's breaks. Its text and
packets are one block, so text past the pool lands in the packets as on
Sony's (cases 40 to 43), and a negative n lowers the sum as on Sony's
(cases 13 to 20), with the packets before the array left unset, Sony's
writing over whatever its own layout holds there. Its GPU words match
Sony's through case 43 apart from the damage above and the stale cell.

### KanjiFntClose

KanjiFntClose closes every stream: the next KanjiFntOpen returns 0 and
starts at the pool's start. It sends nothing.

### Limits of the clean library

Against `spec/probes/kanjilimits` (2026-11-03, after the clean library
took on the rules above: sums of n in KanjiFntOpen, 301 characters to a
flush, text and packets in one block):

The GPU words of all 45 cases are the reference's, word for word, and
Sony's but for what follows.

- The one cell of the image that nothing draws is blank, where Sony's
  holds a stale glyph (case 22's cell 12, case 40's and 42's cell 15).
  Deliberate.
- Sony's damaged packets at 300 and 301 characters (above) are not
  reproduced: both draw clean. Deliberate.
- A flush of 302 characters or more returns nil and draws nothing, the
  302nd counting as a character the ROM has not, where Sony's crashes:
  case 44 prints false and the probe reaches `done`, as with the
  reference. Deliberate.
- A print stops at the end of the block of the pool and the packets, 1,024
  plus 1,024 times 40 bytes, keeping its last byte for the NUL, where
  Sony's writes on over its other memory. A stream opened while the sum of n is
  negative sits at the pool's start, the place Sony's gives it not having
  been probed. Both are the clean library's own choices.

### Not probed

Whether a flush that has ended the text goes on reading it without
effect or stops (not observable from outside); that Krom2Tim's pixel
words at a dx, dy off the page are those at 640, 0 (inferred from the
extent and size word, see Krom2Tim); whether the flush's limit of 301
characters depends on the window's width; and where the text of a
stream opened after one of negative n sits. The items once listed here
(Krom2Tim checking the whole string, a flush checking text past the
window's end, dx or dy off a page's edge, Sony's limits on a flush's
characters, on the pool and on KanjiFntOpen's n) are answered by
`spec/probes/kanjipast` and `spec/probes/kanjilimits`, under Krom2Tim,
KanjiFntOpen, KanjiFntPrint and KanjiFntFlush.

## Done when

- The probe gives the same TTY and the same GPU words between markers
  on the clean library as on Sony's, case for case.
- Every rule found is written here, under a heading for each call: the
  TIM's layout (header, palette, pixel words, how fg and bg become the
  palette, where dx, dy, cdx and cdy go), which codes render and which
  fail, how 16 and more characters are laid out, how the streams draw
  (primitives, where the glyphs are loaded, when, and the palette),
  their limits, and what KanjiFntPrint and KanjiFntFlush return.
- `exp/cleanlib/regress.sh` gives no FAIL lines, and the same TIME lines
  as before: cdda, cdda:4000, check, dino, dungeon, flame, morph,
  offscr, square, terra, timers.
