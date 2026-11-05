# Milestone 1: hello

`docs/tutorial/hello` clears the screen to teal and prints a line with the
debug font, every frame. Through the `yaroze` package's `Screen` it calls
19 library functions, which this milestone implements:

    InitHeap EnterCriticalSection ExitCriticalSection putchar
    SetVideoMode VSync DrawSync
    GsInitGraph GsDefDispBuff GsGetActiveBuff GsSwapDispBuff
    GsSetWorkBase GsClearOt GsSortClear GsDrawOt
    FntLoad FntOpen FntPrint FntFlush

The bindings in `yaroze/` describe each one (read their comments: they
are the public interface), as does the Library Reference
(`docs/manuals/030-1997-libraryref.pdf`). `yaroze/screen.go` and
`yaroze/text.go` show how hello calls them.

## Acceptance

    exp/cleanlib/build.sh hello-clean docs/tutorial/hello
    PSX_CLEAN=hello go test ./tests -run 'TestClean$' -v

The reference disc is `.build/hello-ref/hello-ref.cue`, already built.
The test runs both for 120 frames and must pass: the GPU command stream
(every GP0 and GP1 word, frame by frame) identical, and the last frame's
picture identical. The TTY need not match. The full streams are written
to `.build/hello-clean/ref-gpu.txt` and `clean-gpu.txt`, one line per
frame, to read the expected commands from.

The reference stream is observed behavior and the target. Where it shows
something this spec does not explain, implement what it shows. One known
oddity: during the reset in `GsInitGraph` the reference writes one stray
word, `0004829c`, to GP0 before resetting the GPU. Match it so the
streams line up. The reset then writes GP1 10000007 (the GPU version
query) and sends a zeroed drawing environment with only dithering set.

## Behavior

**InitHeap(head, size)** gives the library's allocator the area. Only
remember it for now (malloc comes in a later milestone).

**EnterCriticalSection / ExitCriticalSection** are the kernel's: the
`syscall` instruction with a0 = 1 and a0 = 2. Enter returns 1 if
interrupts were enabled, else 0.

**putchar(c)** writes c to the TTY through the kernel and returns c.
Traced on Sony's library 2026-11-03 (an iotrace of the seqhdr probe):
each character goes through write(1, &c, 1), B0 table function 35h,
not the kernel's putchar (3Dh), and a newline is two calls, "\r" then
"\n" (the kernel adds another "\r", so a program's lines end "\r\r\n").
Spec 08's printing section has the costs: write takes 4,182 cycles
where putchar 3Dh takes 4,722, so a library using 3Dh prints about 10
percent slower and a long line can push the frame after it late.

**SetVideoMode(mode)**: 0 is NTSC (the default), 1 PAL. Returns the mode
before. It changes the display mode GsInitGraph sets.

**VSync(mode)** needs a vertical blank count, which means handling the
VBLANK interrupt. Mode 0 waits for the next vertical blank; mode n > 1
waits until n blanks have passed since the previous VSync call returned,
and for the next blank if they already have, so it always waits for one;
mode 1 does not wait. With L the count when the last waiting VSync returned and
C the count at this call, mode n > 1 returns at count max(L + n, C + 1)
(probe spec/probes/vsynclate). Modes 0, 1 and n > 1 return the horizontal lines
counted (root counter 1 in hblank mode) since the last waiting VSync returned.
Mode 1 changes neither count, so two in a row read about the same (probe
spec/probes/vsyncone). A negative mode returns the number of vertical blanks
since the count started, without waiting, and changes nothing either. hello
depends on mode 0 waiting exactly one blank.

**DrawSync(0)** waits until the GPU is idle and its DMA channel (2) is
done, and returns 0. DrawSync(1) returns at once: 0 when idle, else
nonzero.

**GsInitGraph(w, h, intmode, dith, vrammode)** resets the GPU and sets
up a w by h display (w one of 256, 320, 384, 512, 640; h 240 or 480 for
NTSC). intmode bit 0 is interlace, bit 2 picks the offset mode
(GsOFSGPU, 4: the GPU's drawing offset positions primitives; GsOFSGTE,
0: the GTE's). dith turns dithering on in the drawing mode. It fills in
`GsDISPENV` and `GsDRAWENV` and the identity matrices `GsIDMATRIX` and
`GsIDMATRIX2` (struct layouts in `yaroze/`). The display stays blanked
until the first swap.

**GsDefDispBuff(x0, y0, x1, y1)** sets the top left of the two display
buffers in VRAM. Buffer 0 is drawn into first.

**GsGetActiveBuff()** returns the buffer being drawn into, 0 or 1.

**GsSwapDispBuff()** shows the buffer just drawn (display start address),
makes the other one the drawing buffer (drawing area and offset), turns
the display on, and flips the index GsGetActiveBuff returns. It writes
GP1 05h, then the display mode (08h) on every swap but the first, then
03h.

**GsSetWorkBase(p)** sets where the Gs sort functions build packets.
Each sort writes its packet there and advances it.

**GsClearOt(offset, point, ot)** clears the ordering table `ot`, whose
`Length` and `Org` the program set: `1 << Length` entries of
`GsOT_TAG`, linked so that drawing from `ot.Tag` visits every entry from
the far end to entry 0, the last entry ending the list (address field
0xFFFFFF). It stores offset and point and sets `ot.Tag`. An ordering
table entry is a word: the top 8 bits a packet's length in words, the low
24 the address of the next packet.

**GsSortClear(r, g, b, ot)** builds, at the work base, a packet that sets
the drawing environment for the current buffer and fills the drawing
area with the color, and links it at the start of the table so it draws
before everything else (it is the last thing linked in front of ot.Tag).
The packet is 8 words: its tag, the fill rectangle (02h) and four zero
words. The drawing environment words (E1h to E6h) are not part of it:
GsSwapDispBuff, GsInitGraph and GsDefDispBuff send them, each as two
sends after GP1 04000002 (the new clip with the old offset, then the new
offset).

(Spec 09, "GsSortClear", corrects this from Sony's library: the packet
is the library's own, one for each buffer, 4 words, put first in the
entry Tag points to, and Tag is left alone.)

**GsDrawOt(ot)** starts DMA channel 2 in linked-list mode from `ot.Tag`
and returns without waiting.

**FntLoad(tx, ty)** uploads the font (`fontPixels` in `fontdata.go`,
128 by 32 pixels of 4-bit color, generated by `gen/font`) to VRAM at
(tx, ty), and a 16-entry CLUT at (tx, ty + 128), and closes every
stream. In the CLUT index 0 is transparent black, index 1 white
(0xFFFF), the rest 0. The CLUT is written by the CPU and the image by
DMA, so only the CLUT's words appear in the stream.

**FntOpen(x, y, w, h, isbg, n)** opens a print stream drawing in the
window at (x, y), w by h, holding at most n characters, and returns its
id (0 for the first). With isbg 1, flushing the stream first clears the
window to black; with 0 it draws over what is there. FntPrint writes to
stream 0.

FntOpen gives its streams characters from one pool of 1024 in all
(probe `spec/probes/fntpool` on Sony's library, with its stores traced):
a stream gets n characters or what is left of the pool, whichever is
fewer, and FntOpen still returns its id when that is 0 (1025 or 2000
give one stream of 1024; 1024 then 1 gives the second none; eight of
100 take 800). At most 8 streams open after a FntLoad; the ninth
returns -1. The pool is all the room the font needs: 1024 characters
of text and 1024 sprite packets of 16 bytes. FntOpen sets up each of
its stream's packets as it opens it: byte 3 (the tag's length) 03h,
byte 7 (the code) 74h, and the halfword at 14 the palette FntLoad
loaded (603Ch for FntLoad(960, 256)), the rest left as it was. It also
sets the stream's count to 0 and writes a terminator over the first
byte of the stream's text (one byte store to the pool in the trace of
`spec/probes/fntstale`, 2026-11-03), so a stream opened over a pool
that still holds an earlier stream's text, flushed before any print,
draws nothing and returns the empty list (the texture-page packet
alone), and so does a second flush and a flush after an empty print.
That holds whether the earlier text was flushed or not, and for a
second stream opened without FntLoad over bytes an earlier stream
left: fntstale cases 0 to 4 give the empty list on every such flush
and "AB" from the first stream alone. The rest of the pool is left as
it was. See spec 08 for its timing.

**FntPrint(s)** appends the NUL-terminated string to the current stream
and returns the number of characters in it. hello passes plain text.

Its edges, probed on Sony's library with `spec/probes/fntedge`
(2026-11-03; it prints the return values and each flush's packets over
the TTY):

- The text is cut at the stream's limit and the print that reaches it
  returns -1: a stream of 8 given "ABCDEFGHIJ" holds ABCDEFGH and
  returns -1; given "ABCDE" then "FGHIJ" it holds ABCDEFGH, the second
  print returning -1; given "ABCDEFGH" exactly it returns 8, and the
  next print, "I", -1. A flush empties it and prints count again.
- How the cut works (probe `spec/probes/fntstale`, 2026-11-03, which
  reads the pool back through a tilde that skips the terminator, and
  Sony's stores traced): FntPrint appends byte by byte, writing each
  character at text[count] and counting it before it checks the room,
  so the character that does not fit is written at text[n], the byte
  after the stream's n (the first byte of the next stream's text, when
  there is one), the count becomes n + 1, no terminator is written, and
  the print returns -1. The stream is then overflowed: every later
  print, the empty string included, returns -1 at once and writes
  nothing, until a flush. A stream filled exactly (count n, by prints
  that returned their count) is not overflowed: the empty string on it
  writes its terminator at text[n] again and returns n, and a nonempty
  print writes its first character at text[n] and overflows it,
  returning -1. So "ABCDEFGHIJ" on a stream of 8 over a pool holding
  "0123456789abcdef" leaves "ABCDEFGHI9abcdef" and a later "" or "XY"
  changes nothing; "ABCDEFGH" then "XY" leaves "ABCDEFGHX9abcdef";
  "ABCDEFGH" then "" leaves the terminator at 8 and returns 8. The
  flush draws 8 glyphs in every case (it is bounded by n, below). A
  stream opened right after the stream of 8 holds the cut character as
  its first byte: flushed without a print it draws it and the stale
  bytes after it ("I9abcdef" in fntstale case 12).
- "%%" is one "%" ("A%%B" is three characters, A % B). A lone "%" is a
  printf directive, which Sony's library formats with whatever the
  argument registers hold (the Go binding passes none): the output is
  garbage and the print returns -1. Undefined; not to be matched.
- With no stream open, FntPrint returns -1 and FntFlush returns nil
  and draws nothing.

**FntFlush(id)** builds the stream's text into one packet list of
sprites (one 8 by 8 textured sprite per printable character, from the
font in VRAM), draws it (drawing synchronously), empties the stream, and
returns the list: a pointer to its first packet, the texture page's. id
-1 flushes stream 0.
A newline moves to the start of the next 8-pixel row; text past the
window's right edge wraps. Each stream is one DMA list: an E1h texture
page word and an E2h word, then one 74h sprite per character, last
character first, color 808080. Spaces advance without drawing;
lowercase prints as uppercase.

The layout, probed on Sony's library with `spec/probes/fntlayout` (a 320
by 240 window at the origin; each flush's packets printed over the TTY;
the reference gives the same packets):

- A glyph and a space advance x by 8, a tab by 32 (always 32, not to a
  tab stop: "A\tB" puts B at 40, "AB\tC" puts C at 48, "\t\tB" puts B
  at 64). Tabs and spaces draw nothing.
- After any of the three advances, when x has reached the window's
  right edge (x0 + w), the line wraps there: x back to x0, y down 8. So
  the 40th glyph of a line in a 320-wide window is drawn at 312 and the
  wrap follows it at once, before the next character is looked at; a
  space or a tab that reaches the edge wraps the same way (37 glyphs
  and four spaces: the third space reaches 320 and wraps, the fourth
  advances to 8 on the next row; 38 glyphs and a tab: 304 + 32 reaches
  the edge, the next glyph is at 0 on the next row).
- A newline moves to x0 on the next row whether or not a wrap has just
  happened, so 40 glyphs, a newline and a glyph put the glyph on the
  third row (y 16), two newlines on the fourth; 39 glyphs, a newline and
  a glyph put it on the second.
- The returned pointer is the E1h/E2h packet itself, the first of the
  list; the sprites follow it, last character first, and the list ends
  there. No empty node precedes the packet.
- A color directive ~cRGB changes the color word of the sprites after it
  (74000070 for ~c700) and takes no room.
- A tilde followed by anything but c skips the tilde and the character
  after it: "A~xB" and "A~~B" draw A at 0 and B at 8 (probe
  `spec/probes/fntedge`). Neither escape looks for the terminator: the
  tilde skips a NUL like any character and the flush goes on into
  whatever the text pool holds after it (FntLoad does not clear the
  pool, FntPrint writes only the characters given and one NUL): "A~"
  printed over "A~~B" draws A and the stale B. ~c takes its three bytes
  the same way: "A~c7" printed over "0123456789abcdef" takes '7', the
  NUL and '5' (color 500070) and then draws "6789abcdef".
- The cell a byte c (0 to 255) is drawn from: k = c - 32, or c - 64
  for a to z only ('`' and '{' are not folded); u = (k % 16) * 8 and
  v = (k / 16) * 8 with C's signed division (toward zero), each kept to
  a byte. So every byte outside the font draws some cell and advances
  8, like a glyph: \x01 (k -31) gives u 88h v F8h; \x1b (k -5) u D8h v
  0; \r, \b and \v are cells too (E8h/F8h, C0h/F8h, D8h/F8h); DEL u 78h
  v 28h; \x80 (k 96) u 0 v 30h; \xa0 (k 128) u 0 v 40h; \xff (k 223)
  u 78h v 68h. Only \n, \t, ~ and % mean anything.
- An id that is not open flushes stream 0: with one stream open holding
  "AB", FntFlush(1) draws it, and FntFlush(7), FntFlush(-2) and then
  FntFlush(0) each return the empty list (the texture-page packet
  alone): a flush ends the stream's text, so a later flush finds none
  even though the pool still holds the bytes. With two streams open,
  FntFlush(1) flushes stream 1 alone: its isbg tile and nothing else,
  since FntPrint writes stream 0. A stream opened over a stale pool and
  flushed before any print draws nothing: FntOpen terminates its text
  (above, under FntOpen).
- The flush handles at most n characters, n the stream's size from
  FntOpen, and the count plays no part in it (probe
  `spec/probes/fntstale`, 2026-11-03). Every byte handled counts one,
  and a tilde escape counts one though it takes two bytes (four for
  ~c), so a flush can run past the count into the pool, and it stops
  after n characters whatever follows. After the n-th it reads one more
  byte, and ends whether or not that byte is the terminator (the two
  exits differ by 4 cycles, spec 08). With a stream of 8 over a pool
  holding "0123456789abcdef": "ABCDEF~" draws ABCDEF and then the
  stale 8 (6 glyphs, the escape, a glyph: 8), "ABCDEFG~" draws ABCDEFG
  alone (7 and the escape), "~a~b~d~" draws 89ab (four escapes and four
  glyphs), and "ABCDEFGHIJ", cut at 8, draws ABCDEFGH and not the I at
  text[8]. A stream of 64 given "ABCDEF~" over "ABCDEFGHI9abcdef" draws
  ABCDEF and then I9abcdef, to the terminator. A stream whose first
  byte was written by the previous stream's cut (above) draws it and
  what follows, with its count 0.
- When a newline (or a wrap) takes y to the window's bottom (y0 + h),
  the flush ends there: nothing after it is drawn, and the characters
  left are not charged (30 newlines and text in a 240-tall window give
  the texture-page packet alone; 29 and text draw it on the last row).
  A wrap that reaches the bottom ends the flush the same way: 29
  newlines and 40 glyphs draw the 40 on the last row and nothing after
  them. The wrap after a space or a tab that reaches the bottom ends it
  too (measured by cost in `spec/probes/fntchars`: a second space or
  tab after it costs nothing).
- The edges are x0 + w and y0 + h as given, not rounded to the 8 grid
  (probe `spec/probes/fntgeom`, 2026-11-03; the reference agrees on
  every case). A glyph is drawn whenever its x is below x0 + w, even
  when it overhangs the edge, and the wrap follows when the advance
  reaches x0 + w: a 100-wide window at the origin draws 13 glyphs on a
  row (the 13th at 96) and wraps after it; 316 wide draws 40 and wraps;
  324 wide draws 41 (the 41st at 320) and wraps; a space from 96 or a
  tab from 88 reaches 100 and wraps. The origin shifts the edge with
  it: x0 4 and w 100 draws 13 glyphs (the last at 100), x0 4 and w 96
  draws 12. Likewise a row is drawn whenever its y is below y0 + h: 12
  newlines in a 100-tall window put text at 96 and draw it (with y0 4,
  at 100), 13 end the flush, and a wrap from the row at 96 reaches 104
  and ends it. A window narrower than a glyph (w 4 or 8) draws one
  glyph a row. The bottom is only tested at a newline or a wrap, so the
  first row is always drawn: a window 4 tall, or 0 tall, draws "AB" of
  "AB\nC" at y0 and the newline ends the flush.
- A stream opened with w 0 never wraps ("AB" in a 0-wide window puts B
  at 8). With isbg 1 its background is sized from the text instead of
  the window: as wide as the longest line that a newline ended (0 when
  no newline was printed: "ABC" gives a 0 by 8 tile; "ABCDE\nAB\n" a 40
  by 24 one; "AB\nCDE" 16 by 16, the last line not counted) and 8 tall
  for every row started, counting the row a newline moved to even when
  it ended the flush ("AB\nCDE" in a 0 by 0 window gives 16 by 16 and
  draws only "AB") and counting no further: the newlines after the
  ending one add nothing ("AB\nCD\nEF" in a 0 by 0 window is 16 by 16
  too, and "ABCDE\nAB\nEFGHIJ\n" 40 by 16). The height comes from the
  text whether or not h was given (0 by 100 with "AB\nCDE" gives 16 by
  16 as well). The tile sits at (x0, y0). A window with a width but h 0
  keeps its given size (100 by 0) and draws its first row.

The timing of each kind of character is in spec 08, "FntPrint and
FntFlush by the character".

Probed on Sony's library (`spec/probes/isbg`, `isbg1`, `flush`):
isbg 1 draws a black rectangle over the whole window each flush, even
when the stream is empty; isbg 0 draws none. FntPrint writes to stream
0. FntFlush(-1) flushes stream 0 only, not every stream.
