# Milestone 10: the memory card

Status: implemented in exp/cleanlib/card.go. The probe matches Sony's
library in ten card setups, TTY and card bytes; examples/card passes
TestClean in the five runs in regress.sh.

The memory card and file calls: open, read, write, lseek, close,
delete, rename, firstfile, nextfile, format and TestCard. examples/card
makes most of them.

## How it is checked

- **TestClean.** examples/card, with a blank card in slot 1 (TestClean's
  `withCard`), and the runs' card contents compared at the end. Its menu
  is worked with a list of buttons (PSX_CLEAN_PAD, a comma list: each
  mask held 10 swaps, then let go 10, from the 30th swap). Up is 0010h,
  down 0040h, cross 4000h. Menu order: SAVE, LOAD, LIST, DELETE, DUMP,
  FORMAT, TEST, starting on SAVE. Runs to add to `regress.sh`:
  - `card`: it starts, tests the card and lists nothing.
  - `card:0010,0010,4000,0040,0040,4000,0040,0040,4000`: format, save,
    list.
  - `card:0010,0010,4000,0040,0040,4000,0040,4000`: format, save,
    load.
  - `card:0010,0010,4000,0040,0040,4000,0010,0010,0010,4000,4000`:
    format, save, dump the save.
  - `card:0010,0010,4000,0040,0040,4000,0040,0040,0040,4000,4000`:
    format, save, delete the save.

  The reference build is `.build/card-ref`.
- **Probes on Sony's library.** A probe in
  `exp/cleanlib/spec/probes/card/` makes each call over the cases that
  matter, and prints the results. The cases:
  - TestCard with no card, a blank card and a formatted one, in both
    slots;
  - format;
  - open with each flag (creating with a size in blocks in the high
    half, as `yaroze/card.go` does), read and write of whole sectors
    and of sizes that are not, lseek;
  - firstfile and nextfile with patterns;
  - rename, delete, and the same on files that are not there.

  The emulator's card is `emu.MemoryCard` (`emu/memcard.go`,
  `emu.NewMemoryCard`, `Machine.InsertCard`). The dump harnesses in
  `.build` (`voiceprobe` TestDump) insert no card. Copy one and add a
  card in slot 1 (and slot 2 where needed). Dump the card's bytes at the
  end next to the TTY.
- **Tracing.** `.build/iotrace` (TestIO) logs every kernel call with its
  arguments and return, which shows how each call reaches the kernel's
  file system, and the card driver's start. Use it on Sony's library
  only. Use `.build/bootprobe` TestTrace too.

## Sources

- The kernel's own memory card file system (the BIOS's B-functions for
  files, "bu00:" and "bu10:" devices, its card driver and events), from
  public hardware documentation of the PlayStation (No$PSX / psx-spx).
- The Library Reference and `docs/manuals/savefile.md`, and the `yaroze`
  package (`yaroze/card.go`, the bindings' comments).
- Sony's library starts the card driver in its initialization routine,
  before the program (as the "PS-X Control PAD Driver" line shows, spec
  05). The clean library starts it in the library's start-up, the
  program's first ResetGraph (spec 07, "The library's start-up"), or at
  the first card call in a program that makes one before any
  ResetGraph. The kernel calls are below, traced from the routine's
  first instruction.

## Done when

- The probe gives the same TTY and card contents on the clean library
  as on Sony's, case for case.
- examples/card passes TestClean in the runs above.
- Every rule found is written here, under a heading for its call.
- `exp/cleanlib/regress.sh` gives no FAIL lines, and the same TIME lines
  as before: cdda, cdda:4000, check, dino, dungeon, flame, morph,
  offscr, square, terra, timers.

## The probe

`spec/probes/card` tests both slots six times, formats "bu00:", "bu10:"
and "bu20:", tests again, and on slot 1 creates (1 block, the same
again, 2 blocks, 0 blocks), opens a file that is not there, writes,
reads and seeks over whole sectors and other sizes, past the end and
after close, opens two files at once, lists with "*", a full name,
"B*", "?ETA", "Z*", the bare device and slot 2, renames (to a new
name, from a missing file, onto an existing name, across devices),
deletes (twice, a missing file, on slot 2), and tests three times more.
Each case starts with GP1 10000100h plus its number. Before its first
line it calls TestCard(0) once, unprinted, so that the card driver has
started in both libraries before the first marker.

Built for Sony's library with `go run ./cmd/yaroze disc --opt=2 --elf
.build/cardprobe-sony/prog.elf -o .build/cardprobe-sony/cardprobe-sony.bin
exp/cleanlib/spec/probes/card` and for the clean one with
`exp/cleanlib/build.sh cardprobe-clean exp/cleanlib/spec/probes/card`.
The harness is `.build/cardprobe` (not in git): TestDump runs DISC for
FRAMES frames with the cards CARD1 and CARD2 ("none", "blank" or an
image file) and writes OUT.tty, OUT.card1 and OUT.card2; CARDn_AT and
CARDn_OUT put a card in or take it out that many frames after the
first marker. TestTrace logs every kernel call made from RAM with its
arguments and result, the TTY as text and the markers, from main or,
with FROM_BOOT=address, from that address (Sony's executable starts its
library at its PC0, 80095068h for the probe).

The setups, all matching TTY (less the carriage returns, which the
TinyGo runtime's putchar does not write in the clean library, spec 01)
and card bytes: both slots blank; none; slot 1 blank only; slot 2
blank only; both formatted (900 to 1500 frames); a blank card put in
slot 1 during the first tests (TestCard 4); a formatted one (2); a
blank one in slot 2 (4 on slot 2); a card taken out while TestCard
writes its sector; and one taken out while it reads.

Results on the kernel's file system (OpenBIOS here) that the library
passes through, for reference: TestCard on a card already there gives
1 whether it is formatted or not; format gives 1 on "bu00:" and
"bu10:" even with no card, 0 on "bu20:"; open with O_CREAT on a file
that exists gives -1, and with 0 blocks makes a file of size 0; reads
and writes of a size that is not a multiple of 128 give 0; a write past
the file's end gives -1; lseek takes any offset, aligned or not, and
whence 2 returns the position unchanged; firstfile fills in the name,
attr (50h), size and head only, leaving next and the rest as they were;
rename onto an existing name or across devices gives 0.

## Starting the card driver

Sony's library starts the card driver in its initialization routine,
before the program (traced from the executable's PC0): after the
controller driver (B0h 12h), the drive's reset and the sound system
(spec 07, "The library's start-up"), it makes these kernel calls in this
order:

1. B0h 08h OpenEvent(F4000001h, spec, 2000h, 0) for the specs 0004h
   (done), 8000h (error), 0100h (timeout) and 2000h (new card): the
   file system's four events.
2. The same for class F0000011h, the driver's own four events.
3. B0h 4Ah InitCard(1), B0h 4Bh StartCard(), A0h 70h _bu_init() (about
   1.7 million cycles here, with interrupts).
4. B0h 5Bh ChangeClearPad(0).
5. B0h 0Ch EnableEvent for each of the eight, in the order opened.

The clean library makes the same calls in the library's start-up (the
program's first ResetGraph, spec 07), once, and before that at the first
TestCard, open, firstfile, delete, rename or format of a name that
starts with "bu" (either case) in a program that makes one before any
ResetGraph; read, write, lseek, close and nextfile work on what those
opened, and a program's other files (the TTY) never start it. Before
the driver it starts the controller driver if that has not been
started: the card driver also reads the controllers, into the buffers
InitPad gave it, and with none it writes them to address 0, which
TestCard's sector write below then copies to the card. Sony's routine
always gives the buffers first.

## TestCard

TestCard(slot): port is slot<<4 (00h, 10h).

1. A0h ABh _card_info(port).
2. TestEvent (B0h 0Bh) on the file system's events in the order done,
   error, timeout, new, over and over, until one returns 1. Nothing
   clears them first.
3. done: return 1. error: return 3 (not observed). timeout: return 0
   (no card).
4. new: TestEvent once on each of the driver's four events (clearing
   any pending), B0h 50h _new_card(port), B0h 4Eh _card_write(port,
   3Fh, 0), which writes the 128 bytes at address 0 to sector 3Fh,
   and TestEvent on the driver's events (done, error, timeout, new in
   turn) until one comes. Whichever comes, the library goes on: the
   four cleared again, B0h 4Fh _card_read(port, 0, buf) into a buffer
   of 128 bytes, and the driver's events waited for again.
5. done: return 2 if the sector starts with "MC" (a formatted card),
   else 4. timeout: return 0 (a card taken out during the read). error
   and new: not observed; the clean library returns 3.

A card the driver has seen since _bu_init is not new: the probe's cards
in at power on give 1, even blank. In the emulator a test with no card
takes about 2.9 frames on slot 1 and 2 on slot 2 (the timeout), with a
card about 2 and 1; each TestEvent call is about 340 cycles.

## open, read, write, lseek, close

Each is the kernel's call with the same arguments, its result
returned: open is B0h 32h (name, flags), lseek B0h 33h (fd, offset,
whence), read B0h 34h (fd, buf, n), write B0h 35h (fd, buf, n), close
B0h 36h (fd). Sony's library calls them straight from the program's
call (the return address is the program's). write on file 1 is the
TTY: the library's own printing keeps calling B0h 35h directly.

## firstfile, nextfile

firstfile is B0h 42h (name, dir), nextfile B0h 43h (dir). Each returns
the kernel's result, dir or 0.

## rename, delete, format

rename is B0h 44h (from, to), delete B0h 45h (name), format B0h 41h
(name), each returning the kernel's 1 or 0.

## Where the spec was silent

- The spec did not say that the card driver needs the controller
  driver's buffers; without them the kernel writes the controllers'
  bytes to address 0, and TestCard's new-card write copies address 0
  to the card.
- TestCard's write of sector 3Fh from address 0 on a new card, and
  that a new card's result comes from reading sector 0.
- Unformatted (4) is only reported for a card new since the last test.
