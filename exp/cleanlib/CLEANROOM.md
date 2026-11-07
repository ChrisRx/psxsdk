# Clean-room rules for exp/cleanlib

`exp/cleanlib` is a reimplementation of the Net Yaroze resident library
(`libps.exe`) written without reference to Sony's code or to anything
derived from reading it. Its purpose is to let the repository drop the
embedded Sony image and the decompiled library entirely. That only works
if the people and agents writing it never see the original's code.

Work is split in two:

- The **spec side** writes `spec/*.md`: what each function does, as the
  manuals describe it and as the resident library was observed to behave
  from outside (black-box probes: calling its functions in the emulator and
  recording arguments, results, memory, GPU commands and kernel calls).
  Specs describe behavior, never how the original is written.
- The **implementation side** writes the Go (and any assembly or C) in
  `exp/cleanlib`, from the specs and the allowed sources below only.

## Allowed sources (implementation side)

- `exp/cleanlib/spec/*.md` and this file.
- Sony's public manuals in `docs/manuals/` (the Library Reference
  `030-1997-libraryref.pdf`, the User Guide `020-1997-userguide.pdf`, the
  file format manual `040-1997-fileformat.pdf`, and the `.md` pages).
- Public hardware documentation of the PlayStation (GPU, SPU, GTE, CD,
  DMA, interrupts, timers, the kernel's A/B/C function tables), such as
  the No$PSX / psx-spx specifications, from your own knowledge.
- Public C library sources and their behavior, such as 4.3BSD's printf, from your
  own knowledge, as a guess to test: Sony's results decide what the spec says.
- This repository's emulator, `emu/`, which models the hardware.
- The `yaroze` package's declarations (`yaroze/*.go` and `yaroze/libps.txt`):
  they are the library's public interface, from Sony's headers.
- Kernel call trampolines and C storage in
  `internal/toolchain/_tinygo/targets/playstation-yaroze-bios.S` and
  `playstation-yaroze-libps-data.c`, and the TinyGo runtime.
- The clean target, `playstation-yaroze-clean.json`, the start-up you
  write for it, `playstation-yaroze-clean.S`, and the link script
  `playstation-yaroze.ld`. (Sony's start-up, `playstation-yaroze.S`,
  includes yours under `LIBPS_CLEAN`; it stays off limits.)
- Black-box tests: running programs or calling resident functions in the
  emulator and comparing observable results (`tests/`, `emu/emutest`).
  Reading the values of the library's public variables (those declared
  in `yaroze/`) in a reference run counts, looked up by exact name; never
  list, profile or disassemble the reference's functions, and never
  look up which of Sony's symbols an address falls in (from
  `playstation-yaroze-symusr.S`, a link map or a disassembler's
  labels): a stalled or traced PC inside Sony's library stays an
  address. A tool must not read the instruction at the PC on Sony's side,
  even to decide what to log: `ARCHIVE_BUS` and `PROBE_BUS` show the I/O
  registers and the GTE from outside. Never save a dump of RAM that holds Sony's library. Build probes
  against Sony's own library (`go run ./cmd/yaroze disc` without
  `--libps`), not a `-ref` build: the reference discs exist for
  TestClean.
- Anything else in the repository not listed below.

## Off limits (implementation side)

Sony's code left this tree at step 3 of replacing the image. The local branch
`sony-reference` holds what it was, and a private copy of the image outside the
repository (spec 15) is what comparisons run against. All of that is off limits, as
are the paths below where they survive, including under other names in git history.

Never open, search, grep, diff or otherwise read these, or ask anyone to
summarize them:

- `exp/libps/` (the decompiled library), including its history in git
  (`git log -p`, `git show`, `git diff` touching it).
- `exp/libps/image/` and its tooling.
- `tools/libpsdis/` and any disassembly or listing it produced.
- `link/libps.exe.gz` (Sony's resident image), and any dump of its bytes.
- `internal/toolchain/_tinygo/targets/playstation-yaroze-libps.S`,
  `playstation-yaroze.S`, `playstation-yaroze-symusr-orig.S` and
  `playstation-yaroze-libps-gte.S`.
- `~/src/ChrisRx/psxsdk-sony/`, which holds Sony's image, its SDK fixtures and the notes on the decompile, and `docs/architecture/libps-go.md` in history.
- Profiles of the reference builds (`TestCleanProfile` on a `-ref` or
  `-sony` build): their symbols name the decompiled library's functions.
  `TestCleanStart` on a Sony build reads instructions at the PC: run it on
  clean builds only.
- `tests/zz_image_test.go`, and `.build/image/`, `.build/archive/` and any
  other build output of the library image prototype. `.build/instr/` and what
  `libpsdis score` writes: a score to climb would steer the work toward Sony's code.
- `exp/cleanlib/image/gen.py`, `exp/cleanlib/image/sonyfont.py`, and the `.ld`,
  `.S`, `.json`, `exports.txt`, `map1.txt` and `sony-cleanfont.exe` files in
  `.build/cleanimage/`: they carry the resident address table or Sony's bytes. This
  covers your own tools too: never write one that reads them, even if it prints only
  names. To name a call, use `entryNames` in `image/probetest`, which reads the public
  entry points from the list the spec side writes, `.build/cleanimage/entries.txt`. Run `exp/cleanlib/image/make.sh` and `regress.sh`, and edit
  `image/main.go` and `image/clib.S` where a spec says to. `.build/cleanimage/missing.txt` and the
  symbols of `.build/cleanimage/image.elf`, which are the clean library's, are
  fine, but a PC in Sony's image on the `-cimg` side stays an address.
- In `.build/carchive/`, the discs, `.bin` and `ZBOOT.EXE` files of the
  `NAME-sony` and `NAME-clean` directories: Sony's image is in them, and on the
  clean side they hold the original programs, whose code is not to be read either.
  The same goes for `prog.elf` in any `.build/*-sony*` or `.build/*-cimg*` directory: don't run `nm` or
  a disassembler over them, even into a count.
  `summary.txt`, `first.txt`, `first/`, `calls/`, `text.txt`, `test.log`, `costs.txt` and `show.png` are fine.
- `~/src/ChrisRx/libps-go-backup/`, `~/.claude/`, and any scratch files of
  other sessions.

If a question can only be answered by looking at one of these, stop and
ask the spec side for a spec of the behavior instead.

Searching counts as reading. Never run a search over the whole
repository: a grep from the root prints lines from `exp/libps/` and the
other files above. Search only the directories you need (`exp/cleanlib`,
`yaroze`, `emu`, `tests` less `zz_image_test.go`, `docs/manuals`), or
exclude every path above explicitly. If a search does print a line from
one of them anyway, base nothing on it and report the slip.

## How work is checked

Each piece has acceptance tests: programs whose pictures, printed output
or recorded behavior must match a reference. The reference is the same
program built against `exp/libps` without the `sonydata` tag (so with the
library's own font, never Sony's), or Sony's resident library observed in
the emulator. Matching a reference's behavior is the goal; reading how it
gets there is not allowed.

## Building

`exp/cleanlib/build.sh NAME DIR` builds the main package in `DIR` against
the clean library onto `.build/NAME/NAME.cue` (with `prog.elf` beside it,
for symbols). Run it from the repository root. Spec 15 covers building the
library as the resident image.
