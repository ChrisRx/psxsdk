# psxsdk

[![PkgGoDev](https://pkg.go.dev/badge/go.chrisrx.dev/psx)](https://pkg.go.dev/go.chrisrx.dev/psx)
[![Build/Test](https://github.com/ChrisRx/psxsdk/actions/workflows/go.yml/badge.svg)](https://github.com/ChrisRx/psxsdk/actions/workflows/go.yml)
[![Pages](https://github.com/ChrisRx/psxsdk/actions/workflows/pages.yml/badge.svg)](https://github.com/ChrisRx/psxsdk/actions/workflows/pages.yml)

psxsdk is a Go toolchain for the original PlayStation, built around the Net Yaroze. It covers the whole path from source to screen:

- **Write** PlayStation programs in Go, compiled with [TinyGo](https://github.com/ChrisRx/tinygo) against generated bindings for the Net Yaroze library.
- **Build** them into bootable discs with tools that convert executables and write CD-XA images, including streamed XA/STR files and CD audio tracks.
- **Run** them in a PlayStation emulator written in Go, which drives the test suite headless and runs in a browser as WebAssembly.

**[Try the examples in your browser](https://chrisrx.github.io/psxsdk/)**: eight Net Yaroze demos ported to Go, booting from the discs this repo builds, in the emulator this repo contains, with the open source [OpenBIOS](https://github.com/grumpycoders/pcsx-redux/tree/main/src/mips/openbios). Touch controls are included for phones.

- [How it fits together](#how-it-fits-together)
- [Getting started](#getting-started)
- [Writing programs in Go](#writing-programs-in-go)
- [The emulator](#the-emulator)
- [Tools](#tools)
- [Packages](#packages)
- [Background](#background)
- [Reference](#reference)

## How it fits together

```
examples/check/main.go
        |  tinygo build -target=playstation-yaroze
        v
    prog.elf  --mips1fix-->  prog.elf  --eco2exe-->  MAIN.EXE
                                                        |
                                    disc/DATA/...  ---> mkpsxiso
                                                        v
                                               check.bin + check.cue
                                                        |
                       +--------------------------------+---------------------------+
                       v                                v                           v
              emu (go test, emutest)          web (WebAssembly, Pages)      real hardware / psxe
```

A Net Yaroze program is only half a program: the rest is `libps`, the library the Net Yaroze boot disc leaves resident in RAM. `eco2exe` combines the two into a standalone PSX-EXE, so the result boots from an ordinary disc in an emulator or on a console that plays burned discs, with no Net Yaroze hardware involved.

## Getting started

Everything is built with Go 1.27 or newer:

```bash
git clone https://github.com/ChrisRx/psxsdk
cd psxsdk
go build -o bin/ ./tools/...
go test $(go list ./... | grep -v examples)   # or: task test
```

That is enough for the disc and executable tools, the emulator and the host-side tests. Building the examples additionally needs the TinyGo fork that carries the `playstation-yaroze` target. The Nix flake provides it, along with the cross binutils, [go-task](https://taskfile.dev), mednafen and a patched [psxe](https://github.com/allkern/psxe) for comparison:

```bash
nix develop            # or: direnv allow
task discs             # every example onto .build/NAME/NAME.cue
task test:emu          # boot each disc in the emulator and compare screenshots
task site site:serve   # build the browser demo and serve it on localhost:8000
```

`task test:emu` boots with the OpenBIOS image in `web/static`, or whatever `PSX_BIOS` names. The memory card test drives Sony's shell, which OpenBIOS does not have, and only runs when `PSX_SONY_BIOS` points at a Sony BIOS. No Sony BIOS is distributed here.

The tools a disc build uses are also declared as Go tools, so inside the module they run without installing anything:

```bash
go tool mkpsxiso -o game.bin --boot MAIN.EXE ./disc
```

## Writing programs in Go

The [`yaroze`](yaroze) package binds the Net Yaroze library for TinyGo. Nothing in it is implemented in Go: each function is a bodyless declaration resolved by the linker against the fixed addresses of the resident library, exactly as a C program's calls are. The bindings are generated from the SDK's `libps.h` by [`hdr2go`](#hdr2go), and every struct's size is asserted against clang's layout at compile time, so a struct that disagrees with the C one by a byte fails to build rather than producing a disc that draws nothing. Names are kept as the header spells them, so Sony's documentation reads across directly.

```go
import "go.chrisrx.dev/psx/yaroze"

yaroze.ResetGraph(0)
yaroze.GsInitGraph(320, 240, 4, 0, 0)
yaroze.FntLoad(960, 256)
```

The examples are ports of Sony's Net Yaroze samples and demos, chosen to put a different part of the machine to work each:

| Example | Original | What it exercises |
| --- | --- | --- |
| [check](examples/check) | Sony SDK sample | sprites, controller, CD reads, sequenced music |
| [chrome](examples/chrome) | S. Ashley, 1997 | GTE, 3D library, lighting, environment mapping |
| [clone](examples/clone) | S. Ashley, 1997 | first-person maze shooter: fog, subdivision, SPU voices |
| [dungeon](examples/dungeon) | Mizuki, 1996 | first-person maze built per frame, fog, two lights |
| [flying](examples/flying) | L. Evans, 1996 | ship over a tiled plane, enemies, scanner, nine cameras |
| [offscr](examples/offscr) | L. Evans, 1997 | drawing into a texture at run time |
| [tunnel](examples/tunnel) | L. Evans, 1997 | endless tunnel shooter, three lights, VAB sound effects |
| [mdec](examples/mdec) | new | full-colour video through the motion decoder, driven by its registers |

Each example's `disc/` directory holds the files it reads off the CD, and `task disc NAME=check` builds one. The header comment of each `main.go` notes where the port departs from the original and why.

## The emulator

[`emu`](emu) is a rewrite in Go of [psxe](https://github.com/allkern/psxe) by Allkern (Lisandro Alarcón), written with psxe as a guide and used under its MIT license (see [`emu/PSXE-LICENSE.txt`](emu/PSXE-LICENSE.txt)). It emulates the CPU, GTE, GPU, SPU (with reverb and gaussian interpolation), CD-ROM drive including XA and CD audio, MDEC, DMA, timers, controllers and memory cards. It was written to be driven from tests rather than played: it runs headless and deterministically, counting emulated cycles with no reference to the wall clock, so a run takes the same course however loaded the host is.

[`emu/emutest`](emu/emutest) wraps it for Go tests. A test boots a machine, runs it against budgets, presses buttons, and asserts on what was printed, what is in memory or what is on screen:

```go
m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("check.cue"))
m.RunUntilTTY("Execute !", 10*emutest.Second)
m.RunFrames(60)
m.Hold(1, emu.ButtonSelect, 10)
m.RunUntilTTY("End of Main", 5*emutest.Second)
```

A program that runs an illegal instruction or makes a misaligned access fails the test there, with the registers, the instructions leading up to it and the program's output. The [`tests`](tests) package uses this to boot every example and compare screenshots against golden images.

The emulator follows psxe closely so the two can be compared one instruction at a time: the flake builds psxe with a patch that writes an instruction trace, `psxemu --trace` writes the same format, and `psxtrace diff` reports the first record where they disagree.

[`web`](web) compiles the emulator to WebAssembly (using Go's SIMD support for pixel conversion) and runs it in a worker behind the page in `web/static`. The Pages workflow builds every example's disc and publishes the site on each push to `main`, and publishes a preview for any pull request that changes an example.

## Tools

| Tool | Purpose |
| --- | --- |
| [eco2exe](#eco2exe) | Combine a Net Yaroze program with `libps` into a bootable PSX-EXE |
| [elf2exe](#elf2exe) | Write a standalone ELF or ECOFF program as a PSX-EXE |
| [mips1fix](#mips1fix) | Replace the MIPS II traps LLVM emits with nops so the code runs on an R3000A |
| [mkpsxiso](#mkpsxiso) | Build a CD-XA disc image from a directory |
| [dumpsxiso](#dumpsxiso) | List or extract a disc image |
| [mdecenc](#mdecenc) | Encode pictures into an MDEC movie |
| [hdr2go](#hdr2go) | Generate layout-checked Go bindings from a C header |
| [psxemu](#psxemu) | Run the emulator headless, with traces, VRAM dumps and screenshots |
| [psxtrace](#psxtrace) | Print and diff instruction traces, compare VRAM dumps |
| [sioload](#sioload) | Upload an executable to a Net Yaroze over the serial cable |
| [objdump](#objdump) | Display ECOFF object files, with disassembly |
| [ecoff2asm](#ecoff2asm-and-ecoffcmp) | Convert ECOFF objects to GNU as source, for relinking the Net Yaroze library as ELF |
| [ecoffcmp](#ecoff2asm-and-ecoffcmp) | Check ecoff2asm output against the original objects |
| gteprog | Write a GTE exercise program as a PSX-EXE, for comparing emulators |

#### eco2exe

The `eco2exe` tool takes a Net Yaroze program and creates a working PSX-EXE executable ready to be used in an emulator (if it supports running bare PSX-EXEs), or built into a disc image to be loaded by a real PlayStation (if it can play burned discs). The input may be the ECOFF produced by the SDK's own toolchain or the ELF produced by TinyGo or a current mipsel-none-elf toolchain.

It does this by parsing the compiled program, combining it with the Net Yaroze development library (aka `libps.exe`) and then converting the combined program to the PSX-EXE executable format. The executable is patched with a startup stub that initializes the library before jumping to the program's entry point.

```bash
$ eco2exe prog.elf MAIN.EXE
created "MAIN.EXE": 47fb1ad6d1aee3c8bdf71bdc2ecd144b
```

*Note: The Net Yaroze development library itself is relatively small, so it has been embedded in the `eco2exe` binary, meaning it doesn't need to be provided by the user!*

#### elf2exe

`elf2exe` writes a linked program out as a PSX-EXE, for a program that is the
whole of the machine rather than a Net Yaroze one.

It is the plain conversion: the loadable image and the entry point go into the
2048-byte header and nothing is added. `eco2exe` exists because a Net Yaroze
program is only half a program, and has to be combined with the library the
boot disc leaves resident in RAM; a program that brings its own crt0 and
drives the hardware itself needs none of that.

```bash
$ elf2exe prog.elf MAIN.EXE
created "MAIN.EXE": load=0x80010000 entry=0x80010000 8192 bytes, d00f145ee4fd44f1
```

The input may be ELF or ECOFF, since only the loadable segments and the entry
point are read. `--stack-addr` sets the stack pointer the BIOS installs before
jumping to the entry point, which matters only for a program whose crt0 does
not set its own.

#### mkpsxiso

`mkpsxiso` builds a PlayStation disc image from a directory. A disc is a `.bin` of raw 2352-byte CD-ROM XA Mode 2 sectors plus a `.cue` naming its tracks, not a plain `.iso`, because the format also has to carry CD audio and streamed XA that will not fit in 2048-byte blocks.

The common case is an executable plus the data it loads, so the directory *is* the interface and no layout file is involved:

```bash
$ mkpsxiso -o game.bin --boot MAIN.EXE --volume-id MYGAME ./disc
created "game.bin": 789 blocks (150 of run-out), "game.cue"
```

The tree is placed on the disc as it appears on disk. Names are folded to the restricted uppercase set ISO 9660 allows and given the `;1` version suffix that PlayStation programs use when opening files, so `data/sound/sample1.seq` becomes `\DATA\SOUND\SAMPLE1.SEQ;1`. `--boot` writes a `SYSTEM.CNF` naming the executable, unless the tree already has one. `-v` lists what was placed where:

```
  \MAIN.EXE;1                                1251328 bytes  LBA 23
  \MUSIC.XA;1                                   9344 bytes  LBA 72  raw sectors
  \SYSTEM.CNF;1                                   53 bytes  LBA 634
  \DATA                                        <dir>        LBA 21
  \DATA\SOUND\SAMPLE1.SEQ;1                     5000 bytes  LBA 636
```

The image ends with two seconds of empty sectors. A drive reads ahead of the sector it was asked for, and the console reads a file by asking for whole sectors, so a file that ends at the end of the track is one the console cannot finish reading: the request runs into the lead-out and fails, which looks like a file that is present and unreadable. Pressed discs carry a run-out for the same reason. `--padding` sets how many sectors, and `0` leaves none, which is fine for an image only ever read by an emulator:

```bash
$ mkpsxiso --padding 0 -o game.bin --boot MAIN.EXE ./disc
created "game.bin": 639 blocks, "game.cue"
$ mednafen game.cue
[CDC] In leadout area: 639
```

The volume descriptor fields are flags: `--volume-id`, `--system-id`, `--publisher`, `--preparer` and `--application`. `--date` sets the volume date, and leaving it unset records no date at all, so the same tree always produces the same bytes. `--file-times` records each file's own modification time instead, which is honest about the tree but makes the image depend on the mtimes in it.

##### Streamed media

Files named `.xa` or `.str` are streamed media: XA audio and video are produced as whole 2336-byte Mode 2 sectors with their subheaders already in place, saying which form each sector is and which interleaved channel it belongs to. Those are written through untouched rather than packed into logical blocks, and their directory records carry the CD-XA attributes that mark them Form 2. `--raw` adds patterns to the set and `--no-raw` empties it. A file taken for a stream that is not a whole number of sectors is an error rather than a file padded into nonsense.

Every image is a CD-XA disc: the `CD-XA001` signature in the volume descriptor and the 14-byte attribute block on every directory record, which is what a PlayStation disc carries and what says a file holds a stream. `--no-xa` leaves both out, for comparing against a plain ISO 9660 writer.

##### Audio tracks

`--audio` appends a CD audio track, which a program plays by its number rather than opening as a file. Input is a WAV file or headerless samples, in both cases 44.1kHz 16-bit stereo, which is the only thing a CD holds. The two second pause the standard requires between the data track and the audio that follows it is inserted automatically, and the `.cue` describes the result:

```bash
$ mkpsxiso -o game.bin --boot MAIN.EXE --audio track1.wav --audio track2.wav ./disc
created "game.bin": 789 blocks (150 of run-out) and 2 audio track(s), 15489 sectors in all, "game.cue"
$ cat game.cue
FILE "game.bin" BINARY
  TRACK 01 MODE2/2352
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    INDEX 00 00:10:39
    INDEX 01 00:12:39
  TRACK 03 AUDIO
    INDEX 01 01:44:00
```

##### Plain ISO 9660

`--iso` writes plain 2048-byte blocks instead. That cannot represent audio tracks or Form 2 sectors, and says so rather than writing an image missing either, but ordinary ISO 9660 tools can read it, which is how the filesystem is checked against an independent implementation:

```bash
$ mkpsxiso --iso -o test.iso ./disc
$ xorriso -indev test.iso -find /
```

Every image carries the 64-byte string that identifies it as a PlayStation disc, written into the system area at sector 4. This is format identification: without it mednafen cannot tell the image from any other CD-XA disc and rejects it outright with *"Could not find a system that supports this CD"*. It is not a protection bypass, and could not be: an unmodified console decides whether to boot from the wobble encoded in a pressed disc's lead-in groove, a physical property no file can reproduce.

Booting on real hardware additionally wants Sony's licence data, which holds the logo shown at startup. That cannot be redistributed, so supply your own copy with `--license`; it replaces the whole system area.

#### dumpsxiso

`dumpsxiso` reads a disc image back. With no `--output` it lists what the image holds, which is the quickest way to see whether a file landed under the name a program will ask for:

```bash
$ dumpsxiso game.bin
volume MYGAME, 789 blocks
  system      PLAYSTATION
  created     2026-10-14T00:00:00Z
  extension   CD-XA

  \MAIN.EXE;1                              1251328 bytes  LBA 23
  \MUSIC.XA;1                                 9344 bytes  LBA 72  raw sectors
  \SYSTEM.CNF;1                                 53 bytes  LBA 634
  \DATA                                      <dir>        LBA 21
  \DATA\SOUND\SAMPLE1.SEQ;1                   5000 bytes  LBA 636
```

With `--output` the tree is extracted to a directory, and what comes out can be handed straight back to `mkpsxiso`. A `.bin` of raw sectors and a `.iso` of 2048-byte blocks are both accepted; which one it is follows from the image itself. Streamed files come out as the 2336-byte sectors they went in as, recognised by their CD-XA attributes rather than their names.

Reading a raw image checks every sector's error detection, so a damaged image is reported rather than quietly extracted. A sector of a filesystem is the kind that is meant to be read correctly or not at all, and a file recovered from a failing one is a file the console itself would refuse.

#### mdecenc

`mdecenc` encodes PNG, JPEG or GIF pictures, in the order given, into a movie of run-length codes the MDEC decodes as they are, laid out as [`format/mdec`](format/mdec) describes. Every picture must be the same size, and each is extended to a multiple of 16 pixels each way. `-q` sets the quantization scale, from 1 to 63, where higher is smaller and blurrier.

```bash
$ mdecenc -o MOVIE.MDC frame*.png
```

The [mdec example](examples/mdec) plays one, rendered by its own ray tracer in `examples/mdec/gen`.

#### psxemu

`psxemu` boots a BIOS in the emulator headless, printing what it writes to the TTY, with a disc in the drive or a PSX-EXE loaded at the shell:

```bash
$ psxemu --bios openbios.bin --disc .build/check/check.cue --png screen.png
```

`--trace`, `--skip` and `--count` write an instruction trace in the format the patched psxe writes with `PSXE_TRACE`. `--vram` dumps VRAM at the end, `--png` saves what the display shows, `--card` attaches a memory card image to slot 1, and `--cpuprofile` profiles the run for `go tool pprof`.

#### psxtrace

`psxtrace` reads the traces `psxemu` and psxe write. `dump` prints one as text, `diff` reports the first record two traces disagree on, and `vram` compares two VRAM dumps or converts one to an image:

```bash
PSXE_TRACE=psxe.trace PSXE_TRACE_COUNT=1000000 psxe -a -b bios.bin
psxemu --bios bios.bin --trace emu.trace --count 1000000
psxtrace diff psxe.trace emu.trace
```

#### ecoff2asm and ecoffcmp

The Net Yaroze library and SDK objects are ECOFF, a format current binutils no longer links. `ecoff2asm` converts ECOFF objects and archives to GNU as source, along with an ld script of `PROVIDE` definitions for every absolute symbol, which is how the `libps` entry points are described. `ecoffcmp` checks the conversion: it assembles the output and compares its `.text` against the original word by word, masking only the immediate fields where ECOFF and ELF keep relocation addends in different places.

#### hdr2go

`hdr2go` generates Go bindings from a C header. It was written to bind the Net
Yaroze library for TinyGo, where the alternative was 166 functions and 30
structs transcribed by hand.

Transcription is not safe here. Every struct has to match the C layout byte
for byte, because the library reads and writes them directly, and a field at
the wrong offset does not fail to compile. It draws nothing. So rather than
read the header, `hdr2go` asks the compiler that will build the code, twice:

```bash
clang -target mipsel-none-elf -march=mips1 -msoft-float -std=gnu89 -fsyntax-only \
    -Xclang -ast-dump=json                  hdr.c   # structure and types
clang ... -Xclang -fdump-record-layouts-complete hdr.c  # offsets and sizes

$ hdr2go --layouts layouts.txt --header libps.h --package libps \
    --out libps/zz_generated.go < ast.json
30 structs, 2 aliases, 166 functions (named=31 known=33 sizes=35)
```

Padding the C compiler inserted implicitly is written out explicitly, and
each struct's size is asserted against clang's number:

```go
var _ [unsafe.Sizeof(GsSPRITE{}) - 36]byte
var _ [36 - unsafe.Sizeof(GsSPRITE{})]byte
```

An array cannot have negative length, so the pair fails to compile unless the
size is exactly right. This is deliberately not a test: a host toolchain will
not compile the bodyless function declarations, and a bare-metal target has
nowhere to run one, so an assertion inside the generated file is the only
check that runs during the build that matters.

Names stay as the header spells them, so the library's documentation reads
across directly. The one departure is that a C name beginning with a
lowercase letter is capitalised, since Go would otherwise put it out of reach
of every other package; `//export` carries the real symbol either way.

Anything the type mapping does not recognise is reported rather than guessed
at, and the report names what has to be bound by hand:

```
1 declarations with unmapped types, skipped:
  GsOT_TAG: uses bitfields, bind by hand
8 unprototyped or variadic, bind by hand:
  FntPrint int ()
  ...
```

An empty parameter list is the common case there. In this vintage of C it
means "unspecified", not "none", so generating a zero-argument binding would
produce something that compiles and then passes nothing.

#### mips1fix

`mips1fix` rewrites MIPS II trap instructions as nops so that an object built by LLVM for `mips1` will run on a PlayStation.

LLVM emits a `teq` after every integer division to catch division by zero. That instruction arrived with MIPS II; a MIPS I core takes a reserved instruction exception instead, and the program stops with no indication of why. GCC does not have the problem, branching around a `break` instead.

```bash
$ mips1fix prog.o
  .text+0x1c50: teq (0x00a001f4) -> nop
  patched 1 trap instruction(s)
```

A trap and a nop are the same width, so nothing moves and no relocation or symbol needs adjusting. Clang suppresses these with `-mno-check-zero-division`, but that is an LLVM backend option rather than a target feature and so cannot be reached through TinyGo. Either way, integer division by zero becomes undefined rather than trapping.

`-n` reports without writing.

#### objdump

`objdump` displays information from ECOFF object files. It is similar in functionality to the objdump included in [GNU Binutils](https://www.gnu.org/software/binutils/) (although not intended to be exactly the same).

This was built while reverse engineering the Net Yaroze development static library, as the first step towards converting it from the aging ECOFF format to ELF (see [ecoff2asm](#ecoff2asm-and-ecoffcmp)):

```bash
$ objdump puts.o
MIPSEL-BE ECOFF executable - start=0x00000000 size=96 sections=2

Sections:
 0 .text     len=80   offset=156  0x00000000 0x00000000
 1 .rdata    len=16   offset=236  0x00000050 0x00000050

Symbols:
[  0] e 0000000000000000 st 0 sc 0 index=FFFFF  gcc2_compiled.
[  1] e 0000000000000000 st 0 sc 0 index=FFFFF  __gnu_compiled_c
[  2] e 0000000000000000 st 0 sc 0 index=FFFFF  $LC0
[  3] e 0000000000000000 st 6 sc 1 index=0000   puts
[  4] e 0000000000000000 st 0 sc 0 index=FFFFF  $L13
[  5] e 0000000000000000 st 0 sc 0 index=FFFFF  $L11
[  6] e 0000000000000000 st 1 sc 6 index=FFFFF  putchar
[  7] l 0000000000000000 st b sc 1 index=0004   puts.c
[  8] l 0000000000000000 st 6 sc 1 index=0002   puts
[  9] l 000000000000004C st 8 sc 1 index=0001   puts
[ 10] l 0000000000000000 st 8 sc 1 index=0000   puts.c
```

It uses mewmew's [mips](https://github.com/mewmew/mips) library to decode and disassemble the provided file. Adding the `-d/-disassemble` flag will add the disassembly to the output:

```assembly
...

puts:
        addiu   $sp, $sp, 0xFFE8
        sw      $s0, 0x10($sp)
        addu    $s0, $a0, $zero
        bne     $s0, $zero, 24
        sw      $ra, 0x14($sp)
        lui     $s0, 0x0
        j       0x28
        addiu   $s0, $s0, 0x50
        jal     0x0
        sra     $a0, $a0, 24
        lbu     $a0, 0($s0)
        nop
        sll     $a0, $a0, 24
        bne     $a0, $zero, -24
        addiu   $s0, $s0, 0x1
        lw      $ra, 0x14($sp)
        lw      $s0, 0x10($sp)
        jr      $ra
        addiu   $sp, $sp, 0x18
        nop
        cfc3    $s5, $cp3_9
        syscall 0xF9
        nop
        nop
```

#### sioload

The linux version of siocons, found on the [psxdev.net forums](http://www.psxdev.net/forum/viewtopic.php?f=67&t=1078) (and floating around other places), did not initially work when loading a Net Yaroze executable. Even after figuring out a way to make it work, it didn't work for all baud rates, didn't work consistently, and had several huge bugs in how it loaded binaries (it loaded the sections incorrectly in a way that still worked), so `sioload` was written as a replacement.

`sioload` isn't a full-feature siocons replacement, but a serial executable loader in Go. Unless specified it finds the first available serial port and defaults to a baud rate of 115200:

```bash
$ sioload --exec main-ecoff
```

It has been reliable at every baud rate tested, using a Net Yaroze DTL-H3050 serial cable through a [TRENDnet USB to Serial converter](https://www.amazon.com/dp/B0007T27H8/ref=cm_sw_em_r_mt_dp_U_FHmgEbZAAPNX5).

## Packages

| Package | Purpose |
| --- | --- |
| [`yaroze`](yaroze) | TinyGo bindings for the resident Net Yaroze library, and the host-side `libps` image `eco2exe` combines with |
| [`emu`](emu), [`emu/cpu`](emu/cpu) | The emulator, and its R3000A and GTE |
| [`emu/emutest`](emu/emutest) | Running programs in the emulator from Go tests |
| [`binutils`](binutils) | Combining programs with `libps`, image layout, linker scripts |
| [`format/psx`](format/psx) | PSX-EXE executables |
| [`format/ecoff`](format/ecoff) | ECOFF objects, archives, symbols and relocations |
| [`format/cdrom`](format/cdrom) | CD-ROM sectors, with EDC and ECC |
| [`format/iso9660`](format/iso9660) | ISO 9660 with CD-XA extensions, reading and writing |
| [`format/psxdisc`](format/psxdisc) | Whole PlayStation discs: tracks, cue sheets, audio |
| [`format/mdec`](format/mdec) | MDEC bitstreams and movies |
| [`format/psxtrace`](format/psxtrace) | Instruction traces shared with psxe |

## Background

psxsdk started as software for using the Net Yaroze on Linux. When getting started with a Net Yaroze DTL-H3001, the development environment on modern computers, Linux in particular, was badly lacking. After getting a working compiler with [mipsel-ecoff-toolchain](https://github.com/ChrisRx/mipsel-ecoff-toolchain), the project grew to cover whatever else came out of making Net Yaroze development work on modern machines, and then to writing the programs themselves in Go and running them in an emulator that can check them. The Net Yaroze might not be the ideal PS1 development environment, but the ability to use it should not be lost to time.

The goals have not changed:

* Software that is cross-platform and easy to build
* Modern languages with features that help the software last
* Libraries and tools that are well-defined and well-documented
* Source code for everything, always available

## Reference

- [Nocash PSX Specification](https://problemkaputt.de/psx-spx.htm) - comprehensive PSX hardware information
- [psxe](https://github.com/allkern/psxe) - the emulator `emu` was rewritten from, and is traced against
- [OpenBIOS](https://github.com/grumpycoders/pcsx-redux/tree/main/src/mips/openbios) - the open source BIOS the browser demo and tests boot
- [mipsel-ecoff-toolchain](https://github.com/ChrisRx/mipsel-ecoff-toolchain) - a compiler toolchain for Net Yaroze development on Linux
- [gwald/netyaroze_demo](https://github.com/gwald/netyaroze_demo) - Net Yaroze demo sources the examples are ported from
- [psxdev.net](http://www.psxdev.net) - a helpful and active PlayStation 1 development community
- [Identical Software](http://www.identicalsoftware.com/yaroze/) - a small collection of useful Net Yaroze software
- [Hitmen PSX FAQ](http://hitmen.c02.at/html/psx_faq.html)
- [Net Yaroze for Linux](https://www.cebix.net/downloads/yarlinux.pdf) - an older guide to setting up the Net Yaroze on Linux
- [Andrew Kieschnick/Napalm](http://napalm-x.thegypsy.com/andrewk/psx/) - creator of eco2exe/exefixup, and the guide for the eco2exe in this repo

## TODO

- [x] ECOFF to PSX-EXE converter (eco2exe)
- [x] Net Yaroze executable serial loader (sioload)
- [x] PSX ISO builder (mkpsxiso), with CD-XA attributes, streamed XA/STR files and CD audio tracks
- [x] PSX ISO reader and extractor (dumpsxiso)
- [x] Go bindings for the Net Yaroze library, and examples in Go
- [x] Emulator, headless test harness and browser demo
- [ ] Document code
- [ ] Tests for all packages
- [ ] Include collected PDFs and other Net Yaroze documents in repo

## License

[MIT](LICENSE). Two parts carry their own notices:

- The emulator in `emu` is derived from [psxe](https://github.com/allkern/psxe), Copyright (c) 2024 Allkern (Lisandro Alarcón), under the MIT license in [`emu/PSXE-LICENSE.txt`](emu/PSXE-LICENSE.txt).
- The OpenBIOS image in `web/static` is under the license in [`web/static/OPENBIOS-LICENSE.txt`](web/static/OPENBIOS-LICENSE.txt).
