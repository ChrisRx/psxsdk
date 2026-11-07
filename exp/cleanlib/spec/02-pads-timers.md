# Milestone 2: the font fixes, pads and timers

## First: the font fixes

Probing Sony's library showed three behaviors milestone 1 got wrong (see
the end of `01-hello.md`): isbg 1 draws the window's black background on
every flush, even with no text, and 0 draws none; FntPrint writes to
stream 0; FntFlush(-1) flushes stream 0 only. The probe programs are in
`spec/probes/` and their reference discs are built:

    for p in isbg isbg1 flush; do
      exp/cleanlib/build.sh $p-clean exp/cleanlib/spec/probes/$p
      PSX_CLEAN=$p go test ./tests -run 'TestClean$' -v
    done

All three must pass, and hello must still pass.

## Then: examples/pads and examples/timers

Their reference discs are built (`.build/pads-ref`, `.build/timers-ref`).
Acceptance:

    exp/cleanlib/build.sh pads-clean examples/pads
    PSX_CLEAN=pads go test ./tests -run 'TestClean$' -v
    PSX_CLEAN=pads PSX_CLEAN_PAD=4010 go test ./tests -run 'TestClean$' -v
    exp/cleanlib/build.sh timers-clean examples/timers
    PSX_CLEAN=timers PSX_CLEAN_FRAMES=300 go test ./tests -run 'TestClean$' -v

PSX_CLEAN_PAD holds buttons on the controller in port 1 during the
compared run (4010 is cross and up, in the controller's own bit order:
see `emu/pad.go`). A link error from `build.sh` names each missing
export; the functions these two need beyond milestone 1 are about:

    GetPadBuf GsSortBoxFill GsSortSprite GsGetTimInfo GetTPage LoadImage
    CdControl CdIntToPos CdRead CdReadSync CdReadFile
    GetRCnt ResetRCnt StartRCnt GetVideoMode rand srand

Again the bindings in `yaroze/`, the Library Reference and the reference
streams are the sources. Notes on what is not obvious from them:

**GetPadBuf(buf1, buf2)** stores the addresses of the two controller
receive buffers (ports 1 and 2). The kernel's controller driver fills
them every vertical blank (the B0 table's InitPAD, 12h, with two 34-byte
buffers, then StartPAD, 13h, then ChangeClearPad(0)); the library's
start-up starts it in the program's first ResetGraph (spec 07, "The
library's start-up"), and GetPadBuf starts it only in a program that
asks for the pads before any ResetGraph. Buffer layout as the
kernel writes it: byte 0 is 0 when a controller answered and FFh when
none did; byte 1 is the controller's type in the high nibble and its
data length in halfwords in the low; the data follows, buttons active
low. `yaroze/pad.go` and `yaroze/controller.go` read them. The buffers
must be word aligned. VSync's wait must keep working with the driver
installed (the kernel's driver also hooks the vertical blank).

**GsSortSprite, GsSortBoxFill** build their GPU packets at the work base
and link them into the ordering table at the entry for their priority,
pri (as the manual describes it; check the order against the stream). The
packet words, including any texture page and mode words a sprite sends
before its 64h or 74h-style command, are in the reference stream.
GsSprite's rotation and scaling: timers and pads draw unrotated, unscaled
sprites; implement that case and leave rotation for later, saying so in
a comment.

**GsGetTimInfo(tim, info)** takes the TIM's address after its ID word
(as the manual says) and fills a GsIMAGE: pixel mode, the image's
position, size and pixel address, and the CLUT's when there is one. Either pointer may be null, which
is memory here: info null writes the fields from address 0 on, and a null TIM reads from address 0 on,
with the addresses it stores counted from 0 (`timnull`, spec 37).

**LoadImage(rect, p)** uploads to VRAM and returns at once; DrawSync
waits for it. Uploads may go by DMA or by the CPU; the stream shows what
order and grouping to match (milestone 1 found whole 16-word blocks by
DMA and the remainder by the CPU reproduce the reference).

**CD**: the `disc` package reads files by sector through these. CdRead
reads n sectors from the position the last CdlSetloc set, and returns 1
on starting; CdReadSync(0) waits and returns 0 when done (negative on an
error), CdReadSync(1) returns the sectors still to come. CdControl
issues a CD-ROM controller command with its parameters and waits for the
first response, returning 1 on success. CdIntToPos converts a sector
number to minutes, seconds, sectors in BCD, counting the 2-second
pregap. The emulator's `emu/cdrom.go` is the reference for the drive.

**Root counters**: GetRCnt, StartRCnt and ResetRCnt are the kernel's (the
B0 table, 03h, 04h and 06h). The counter specs are RCntCNT0 to RCntCNT3
(F2000000h to F2000003h).

**rand, srand**: timers' picture depends on the sequence, so it must
equal the reference's. The kernel has a rand (A0 table, 2Fh) and srand
(30h).

**GetVideoMode** returns what SetVideoMode set.

If an example needs another function than those above, implement it the
same way and list it in the report.

## Findings (milestone 2 done)

Everything passes but timers, whose only differences are values it
measures in CPU time: VSync(1)'s line count (14 against 15) and the load
bar drawn from it. Frame-exact timing is not a goal, so timers counts
as passing on everything else. What the implementation found that this
spec lacked:

- GsSortClear's packet is the 02h fill alone. GsClearOt links entry 0 to
  a shared packet of 4 zero words that ends the list.
- Sorted primitives start with an E1h word: the texture page, with the
  semi-transparency rate, and dithering from GsDRAWENV. A sprite is 64h,
  plus 2 when semi-transparent and 1 when brightness is off, at its
  position less (Mx, My). Within one depth the last packet sorted is
  drawn first.
- GsGetTimInfo's Pmode is the TIM's whole flags word, bit 3 included.
- GetPadBuf's buffers read FFh until the kernel's first read, because
  GetPadBuf itself writes FFh into both buffers at every call, whether
  or not the driver is running (Sony's library, probed 2026-11-02: the
  first byte reads FFh right after each call and 0 or 41h two frames
  later). This matters now that the driver starts frames before the
  program's GetPadBuf (spec 07). ChangeClearPAD(0) keeps the library's
  vertical blank count running.
- The CD needs the kernel's CD handlers removed (A0 72h) and DMA channel
  3's interrupt disabled in DICR; the library takes the CD interrupt
  itself. CdRead sends Setmode before ReadN.
- VSync must leave root counter 1 running: programs read it as RCntCNT1. The library's start writes
  its mode 0107h and nothing else, so it holds at 0 until the next vertical blank and then runs free,
  its mode reading 0106h after, and it isn't restarted by a later ResetGraph or GsInitGraph (`linestart`,
  spec 37). The lines the first VSync returns are counted from that blank.
- FntPrint's `~cRGB` sets the text color (each digit times 16), reset to
  808080 at the start of each flush.
- The target's link script PROVIDEs CdControl and CdIntToPos at the
  resident library's addresses, so a missing definition links silently.

**When the pad driver starts (final review, 2026-11-02).** Sony's
resident library gives the kernel the pad buffers in its initialization
routine, before the program (the "PS-X Control PAD Driver" line follows
the first "ResetGraph:" line in every build for Sony's library), and the
kernel reads the ports every vertical blank from the moment the card
driver starts in the same routine, whether or not GetPadBuf is ever
called. The reference and the clean library used to start the driver at
GetPadBuf (the Go API's first pad read), so a program that loads before
its first pad read ran ahead of Sony's library until then (the kernel's
polling is 40 serial port accesses, 5,782 cycles, a frame). Decided: the
driver starts in the library's start-up, the program's first ResetGraph,
in both (spec 07, "The library's start-up"); the reference does so now.
