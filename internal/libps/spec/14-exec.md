# Milestone 13: loading and running programs

Status: implemented (`exp/cleanlib/exec.go`, with the table lookup it
and CdReadFile use in `cdfile.go`). The probe gives the same TTY as
Sony's library in 28 of its 32 cases byte for byte; the other four differ
only in ways that are not the library's (see "Result"), and iotrace shows
the same kernel calls in the same order in every case.

The calls: CdReadExec, Exec, Load and LoadTest. No example makes them.
The work is a probe that must match Sony's library, TTY and traced
kernel calls.

## How it is checked

`spec/probes/exec` (spec side, do not edit it; add a probe of your own
beside it if you need more cases) carries eight small PSX-EXE files,
which `spec/probes/exec/mkchild` writes into `spec/probes/exec/files/`
(run `go run ./exp/cleanlib/spec/probes/exec/mkchild
exp/cleanlib/spec/probes/exec/files` first; the .exe files are not in
git). Each child stores a GP1 marker (10000180h plus its number),
records sp, gp, ra, a0, a1, fp and a2 at offset 100h of its text, bumps
a run count there, samples its first bss word, writes BB55h to its
second, and returns 1234h. Their headers differ: `child.exe` is plain,
`stack.exe` and `junk.exe` ask for a stack (`junk.exe` also trashes sp,
gp and fp before returning), `long.exe` has five sectors of text,
`entry.exe` starts 200h past its load address, `short.exe` has a text
size that is not whole sectors, `nobss.exe` has no bss, and `bad.bin`
has a bad magic. They go on both discs through package disc.

The probe, each case between GP1 marker words 10000100h plus its number:

- CdReadExec on each child, on a file that is not there, on a name with
  a `cdrom:` prefix, without the version, in lower case, and empty. It
  prints the EXEC the call returns right away and again after
  CdReadSync, whether CdReadSync(1) ever returned a positive count, what
  memory the read wrote (first and last words changed from the 5A5A5A5Ah
  fill of 80100000h..801B0000h, and a hash), then runs the child with
  Exec and prints what it recorded.
- Exec on the same EXEC twice, on a copy, outside a critical section,
  with S_addr and S_size changed by hand, with B_size 0, with a header
  made by hand over a copy of the child's code, and on `junk.exe`.
- LoadTest and Load on an empty name, a bare name, a `\` path and a
  `bu00:` name. (On the emulator's BIOS a `cdrom:` name never comes
  back from the kernel's device, not even for a missing file, so the
  probe tries none; see "Load and LoadTest".)

The values that come from the caller's registers are printed as
differences or booleans, since the two builds' stacks and code differ.

Build it for Sony's library:

    go run ./cmd/yaroze disc --opt=2 --elf .build/execprobe-sony/prog.elf \
        -o .build/execprobe-sony/execprobe-sony.bin exp/cleanlib/spec/probes/exec

and for the clean one with `exp/cleanlib/build.sh execprobe-clean
exp/cleanlib/spec/probes/exec`. The TTY comes from `.build/voiceprobe`
(TestDump, FRAMES=3000, OUT.tty); Sony's is at
`.build/execprobe-sony/out.tty`. Compare from the line "case 0" on. The
lines before it print where each child landed on the disc; those sectors
may differ between the two discs, since the executables differ in size,
and then the words a read brings in past a child's text (see below) are
another file's and the hash of a "wrote" range may differ for that
reason alone. Check such a case by hand before calling it a difference.
`.build/iotrace` (TestIO, CALLS=CdReadExec,Exec,Load,LoadTest,
CdReadSync,CdReadFile) shows the kernel calls each call makes and the
CD traffic; the clean library's should be the same calls in the same
order.

## What Sony's library does

Found with the probe and iotrace on Sony's library.

### Load and LoadTest

Each is the kernel's call, made straight from the program's call (the
return address the kernel sees is the program's): LoadTest is A0h 41h
(name, exec), Load is A0h 42h (name, exec). The result is the kernel's.
The kernel prints its own messages (`NAME is not known device` and the
device list for a name without a known device prefix) and returns 0,
leaving the EXEC untouched. A `bu00:` name with no card returns 0 with
nothing printed.

Rules found implementing them:

- The clean library passes each straight to the kernel too, through its
  trampoline (`libps_biosA`), with the program's arguments. The
  wrappers are kept out of line (`go:noinline`) so that a trace of the
  program sees the call; Sony's build is seen through the yaroze
  package's wrappers the same way.
- For a `bu00:` name the kernel opens the file (B0h 32h), looks the
  device up (A0h 17h three times), opens its four events (B0h 20h,
  class F4000001h), reads the card's first sector (B0h 4Fh), gets the
  driver's answer (A0h A9h, B0h 58h) and opens the events again before
  returning 0. In Sony's run the card driver was already running, started
  by the library's initialization routine (spec 11). The clean library
  starts it in the library's start-up, the program's first ResetGraph
  (spec 07, "The library's start-up"), and at the first use of a card
  name in a program that makes one before any ResetGraph, as its `open`
  does (`cardFile`): Load and LoadTest with a "bu" name make the
  driver's start-up calls first then (InitPad, StartPad, the eight
  OpenEvents, InitCard, StartCard, the kernel's `_bu_init`,
  EnableEvents), and the kernel's calls after A0h 41h/42h are the same as
  Sony's either way. Without the start the kernel's card read gives up
  at once (nothing after B0h 4Fh), with the same TTY, but a card that is
  there could not be read. (When the clean library started the driver
  only at the first card name, the start printed the kernel's "PS-X
  Control PAD Driver" line there, which Sony's routine had printed before
  the program: the one TTY difference in those cases.)
- A name without a device prefix (empty, bare, or a `\` path) makes
  the kernel print its message and device list and return 0 after the
  same calls on both libraries: open, three device lookups (A0h 17h),
  five prints (A0h 3Fh).

On the emulator's BIOS (OpenBIOS) the `cdrom:` device never completes a
read once the library's own CD handler is in place, so a `cdrom:` name
hangs the probe in the kernel, missing file or not. That is the
kernel's, and the same on both libraries; the clean library makes the
same call and does not work around it.

### Exec

The kernel's A0h 43h (exec, argc, argv), made straight from the
program's call, result returned (1). What the kernel does with it, for
reference (seen through the children): it saves the caller's sp, fp,
gp, ra and s0 in the EXEC's Sp, Fp, Gp, Ret and Base; clears B_size
bytes at B_addr, by words (a B_size that is not a multiple of 4 halts
the emulator's kernel on an unaligned store, so the probe never tries
one; B_size 0 clears nothing); if S_addr is not zero it sets sp and fp
to S_addr + S_size, else leaves them; sets gp to Gp0 (0 if the header
says 0); calls Pc0 with a0 = argc, a1 = argv and a2 = argv; and
restores the registers afterwards, so a program that trashes sp, gp and
fp does no harm. The child's return value is dropped. Whether the call
sits in a critical section changes nothing observable. The EXEC may be
anywhere, on the stack or made by hand: Exec runs what it says.

The clean library's Exec is that one kernel call, A0h 43h with the
program's EXEC, argc and argv, result returned. Everything the probe
prints of a run (the EXEC's fields and saved registers, what the child
recorded, the bss words) came out the same in every Exec case, the
child's return address included (the kernel's, BFC07620h on OpenBIOS).

### CdReadExec

The library's own, built on its CdReadFile and CdReadSync (spec 05 and
09). For the name given:

1. CdReadFile(name, buf, ...) into a 2048-byte buffer of its own (on
   its stack), and CdReadSync(0) until done. A name the directory does
   not have (a missing file, a `cdrom:` prefix, the empty string)
   returns 0 (NULL) here, with nothing written to memory the program
   can see. Names are looked up as CdReadFile looks them up: the
   version is optional and case does not matter.
2. The header's 15 words from offset 10h (Pc0 through Base) are copied
   into the library's one static EXEC (the same pointer is returned on
   every call, in RAM). The copy takes Pc0, Gp0, T_addr, T_size,
   D_addr, D_size, B_addr and B_size from the header; S_addr and S_size
   come out 0 whatever the header says (a stack the header asks for is
   not honored), and Sp through Base 0. No magic check: a file whose
   first 8 bytes are not "PS-X EXE" is loaded all the same.
3. CdReadFile(0, T_addr, ...) carries on from the sector after the
   header, then CdReadSync(0) until done. The memory written from
   T_addr is the file's size in whole sectors, header included: one
   sector more than the text itself (T_size 800h writes 1000h bytes,
   2800h writes 3000h, 900h writes 1800h), so the words after the text
   are whatever follows the file on the disc.
4. It returns the EXEC's address. The read is complete when it returns:
   a CdReadSync(1) right after gives no positive count and CdReadSync(0)
   gives 0. The manual's "the read goes on in the background" is not
   what happens.

A CdReadFile after it continues independently (it reads its own file);
the EXEC is unchanged.

Rules found implementing it, from the probe and iotrace (each read is
seen as its Setloc's event, 20h, and its Pause's two, 40h and 20h,
delivered with B0h 07h):

- Names are looked up through the tables CdSearchFile keeps (spec 09),
  not by walking the directories each time. The first CdReadExec of the
  run made five reads: the primary volume descriptor, the path table,
  the root directory's listing, the header sector and the text. Every
  later one made two, header and text, whatever the name: the listing
  is kept. A name the disc does not have (`\NONE.EXE;1`, a `cdrom:`
  prefix, the empty string) made no read at all, and the CdReadFile of
  `\LONG.EXE;1` after a CdReadExec made one, the file's. The clean
  library's CdReadExec and CdReadFile go through the same tables
  (`cdFilesT.search` and `lookup`): the path table read once, the
  listing of the file's directory read when it is not the one last
  searched. The first sector of a listing is what the tables hold, as
  for CdSearchFile; every example's directories fit in one sector. This
  answers the question spec 09 left open: Sony's CdReadFile uses the
  tables (case 13 here, and the readsync trace,
  `.build/iotrace/rs-sony.txt`, a file in a subdirectory: four reads
  before its data, the descriptor, the path table, the subdirectory's
  listing and the file), so the clean CdReadFile does too, and no longer
  walks the directories from the volume descriptor on every call. Note:
  with the change square's loading bar moved from TIME to PASS in
  regress.sh, and tunnel's SPU writes came to differ from the
  reference's by a sound tick at a frame's edge (tunnel and tunnel:0800
  are timing-sensitive runs in TestClean). The final review (spec 08)
  found that the reference's CdReadFile goes through its tables as
  well: the tick's frame follows the start-up's offset, not the lookup.
- The lookup for CdReadFile and CdReadExec ignores case and takes the
  version as optional: `\child.exe;1` and `\CHILD.EXE` both find
  `CHILD.EXE;1`. (CdSearchFile keeps its exact match, spec 09.) The
  clean library compares the directory parts ignoring case too; the
  probe has no subdirectory to say what Sony's does with them.
- The empty string finds nothing: a path with no part at all names no
  file. The clean library's search used to return the root directory's
  own record for it, and CdReadExec("") then loaded the root listing as
  an executable (Pc0 8) and crashed running it.
- The second read's length is the file's size in bytes, rounded up to
  sectors, from the sector after the header: 1000h bytes for a
  1000h-byte file with 800h of text, 3000h for 3000h/2800h, 1800h for
  1100h/900h. The clean library reads the file's size, from its
  directory record, in sectors (`cd.readAt`), and leaves the sector
  after them where a CdReadFile without a name carries on from, as a
  CdReadFile of the file would; the header's T_size plus one sector
  would give the same for these files.
- The header sector goes to a 2048-byte buffer of the library's own
  (static in the clean library, `loader.head`, so that the read does not
  escape to the heap), and the EXEC it fills is one static variable in
  RAM, returned every time.

## Result

The probe's TTY from "case 0", compared with Sony's with the carriage
returns normalized (the TinyGo runtime's println writes "\r\r\n" through
Sony's write and "\r\n" through the clean library's, spec 01 and 11),
and the clean library's "addr=8004d724" taken out of case 0 (the CD
system's start-up notice, printed at the first CD use by a program that
brings its own library, spec 05): 28 of 32 cases the same byte for byte.
The four others:

- Cases 1 and 12 (`\LONG.EXE;1`): the hash of the 3000h bytes written
  differs (564D5A2Bh against 7D6E3DABh). Checked from the two disc
  images: the file's five text sectors hash the same on both
  (625EEB1Bh), and the sixth sector read, past the file's end, is
  MAIN.EXE's header on both discs, which differs between the builds.
  The layout caveat above.
- Case 21 (hand-made EXEC): the hash of 80100000h..801A90FCh differs.
  The region holds the child's copy at 801A8000h with the record its
  run wrote at offset 100h: the caller's sp and fp, and argv's address
  (a1 and a2), raw. Those differ between the two builds, which is why
  the probe prints them as differences everywhere else; the words it
  prints for the case are the same.
- Case 27 (LoadTest `bu00:CHILD`): the clean library starts the card
  driver first, and the kernel prints "PS-X Control PAD Driver" (see
  "Load and LoadTest"). The result and the EXEC are the same.

iotrace, kernel calls per case (the TTY writes and the interrupt
returns, B0h 17h, left out; arguments compared except the addresses of
the probe's own variables and the kernel's stack buffers): the same
calls in the same order in every case; case 27 has the card driver's
start before its LoadTest, then the same calls. In the CdReadExec cases
the DeliverEvents and Execs match one for one: five reads for the first
load, two for each later one, none for a name that is not there, and
one for the CdReadFile after a CdReadExec (case 13). Before CdReadFile
went through the tables, the clean library read the descriptor and the
listing on every call, and the descriptor once for the empty name.

`exp/cleanlib/regress.sh`: no FAIL; TIME for cdda, cdda:4000, check,
dino, dungeon, flame, morph, offscr, terra, timers, and now tunnel and
tunnel:0800 (the note above); square, TIME before, passes.

## Done when

- The probe gives the same TTY from "case 0" on the clean library as on
  Sony's, every line (with the layout caveat above), and iotrace shows
  the same kernel calls in the same order for each case.
- Every rule found is written here, under its call's heading.
- `exp/cleanlib/regress.sh` gives no FAIL lines, and the same TIME lines
  as before: cdda, cdda:4000, check, dino, dungeon, flame, morph,
  offscr, square, terra, timers.
