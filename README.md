# psxsdk

[![PkgGoDev](https://pkg.go.dev/badge/github.com/ChrisRx/psxsdk)](https://pkg.go.dev/github.com/ChrisRx/psxsdk)
[![Build Status](https://cloud.drone.io/api/badges/ChrisRx/psxsdk/status.svg)](https://cloud.drone.io/ChrisRx/psxsdk)

psxsdk is a collection of tools and libraries for Playstation 1 development.

- [What is psxsdk](#what-is-psxsdk)
- [Getting started](#getting-started)
- [What's included](#whats-included)
  - [eco2exe](#eco2exe)
  - [elf2exe](#elf2exe)
  - [mkpsxiso](#mkpsxiso)
  - [dumpsxiso](#dumpsxiso)
  - [hdr2go](#hdr2go)
  - [mips1fix](#mips1fix)
  - [objdump](#objdump)
  - [sioload](#sioload)
- [Reference](#reference)

## What is psxsdk

psxsdk was initially creating software to use the Net Yaroze on linux, but the scope has grown to include any PS1 development software that comes out of getting the Net Yaroze development environment working on modern computers. While not a complete SDK, and no specific goal to create one, psxsdk will include any software I create while working on Net Yaroze and standard Playstation development.

When getting started with a Net Yaroze DTL-H3001, I found that the development environment on modern computers and popular development platforms (linux), were incredibly lacking. After some initial successes with [mipself-ecoff-toolchain](https://github.com/ChrisRx/mipsel-ecoff-toolchain) with creating a working compiler toolchain, I started looking for other opportunities to make Net Yaroze development better for myself and others. While the Net Yaroze might not be the ideal PS1 development environment, I'm compelled to work on this because I do not want the ability to use the Net Yaroze to be lost to time.

With this said, some of the major goals of this project:

 * Create software that is cross-platform and easy to build
 * Use modern languages (such as Go and Rust) with features that will help ensure longevity of the software being developed
 * Ensure that libraries and tools are well-defined and well-documented
 * Source code for everything will always be available

## Getting started

There are pre-built binaries available for linux, macOS, and Windows that can be found [here](https://github.com/ChrisRx/psxsdk/releases/latest). Building from source is incredibly easy with Go and the only requirement for building is [Go 1.27](https://golang.org/dl/#stable) (or higher).

First, download and build all of the psxsdk example binaries:

```bash
git clone https://github.com/ChrisRx/psxsdk
cd psxsdk
make
```

If successful there should now be several binaries in your `bin/` directory. Running any of these will print the command help:

```bash
$ bin/sioload
Error: accepts 1 arg(s), received 0
Usage:
  sioload [flags] <file>

Flags:
  -b, --baud int             baud rate (default 115200)
  -d, --device-name string   serial device name (e.g. /dev/ttyUSB0)
      --exec                 execute uploaded file
  -h, --help                 help for sioload
      --stdout               output response to stdout

2020/01/10 12:55:29 accepts 1 arg(s), received 0
```

## What's included

The included tools are more so examples at this stage, but are still good at showing what has been accomplished so far, and what ultimately can be created to aid in PSX development.

#### eco2exe

The `eco2exe` tool takes a Net Yaroze compiled program (an ECOFF executable) and creates a working PSX-EXE executable ready to be used in an emulator (if it supports running bare PSX-EXEs), or compiled into a burnable ISO to be loaded by a real Playstation (if it can play burned games).

It does this by parsing the compiled program, combining it with the Net Yaroze development library (aka `libps.exe`) and then converting the combined program to the PSX-EXE executable format. The executable is patched when combining with the Net Yaroze library to ensure it boots properly.

To see it in action, use `eco2exe` on the provided test fixture:

```bash
$ bin/eco2exe pkg/format/ecoff/testdata/main-ecoff psx.exe
created "psx.exe": 36174751559112fbf3e6255be181c9fd
```

and this will create a working PSX-EXE executable.


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
$ bin/elf2exe prog.elf MAIN.EXE
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
$ bin/mkpsxiso -o game.bin --boot MAIN.EXE --volume-id MYGAME ./disc
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
$ bin/mkpsxiso --padding 0 -o game.bin --boot MAIN.EXE ./disc
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
$ bin/mkpsxiso -o game.bin --boot MAIN.EXE --audio track1.wav --audio track2.wav ./disc
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
$ bin/mkpsxiso --iso -o test.iso ./disc
$ xorriso -indev test.iso -find /
```

Every image carries the 64-byte string that identifies it as a PlayStation disc, written into the system area at sector 4. This is format identification: without it mednafen cannot tell the image from any other CD-XA disc and rejects it outright with *"Could not find a system that supports this CD"*. It is not a protection bypass, and could not be — an unmodified console decides whether to boot from the wobble encoded in a pressed disc's lead-in groove, a physical property no file can reproduce.

Booting on real hardware additionally wants Sony's licence data, which holds the logo shown at startup. That cannot be redistributed, so supply your own copy with `--license`; it replaces the whole system area.

#### dumpsxiso

`dumpsxiso` reads a disc image back. With no `--output` it lists what the image holds, which is the quickest way to see whether a file landed under the name a program will ask for:

```bash
$ bin/dumpsxiso game.bin
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

$ bin/hdr2go --layouts layouts.txt --header libps.h --package libps \
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
$ bin/mips1fix prog.o
  .text+0x1c50: teq (0x00a001f4) -> nop
  patched 1 trap instruction(s)
```

A trap and a nop are the same width, so nothing moves and no relocation or symbol needs adjusting. Clang suppresses these with `-mno-check-zero-division`, but that is an LLVM backend option rather than a target feature and so cannot be reached through TinyGo. Either way, integer division by zero becomes undefined rather than trapping.

`-n` reports without writing.

#### objdump

`objdump` displays information from ECOFF object files. It is similar in functionality to the objdump included in [GNU Binutils](https://www.gnu.org/software/binutils/) (although not intended to be exactly the same).

This was built while reverse engineering the Net Yaroze development static library, in an attempt to convert it from the aging ECOFF format to a more modern (and supported) format like ELF (stay tuned!). The test fixtures can be used to show how it works:

```bash
$ bin/objdump pkg/format/ecoff/testdata/puts.o
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

The linux version of siocons, found on the [psxdev.net forums](http://www.psxdev.net/forum/viewtopic.php?f=67&t=1078) (and floating around other places), did not initially work when loading a Net Yaroze executable. Even after figuring out a way to make it work, it didn't work for all baud rates, didn't work consistently, and had several huge bugs in how it loaded binaries (it loaded the sections incorrectly in a way that still worked), so I started working on a replacement.

`sioload` isn't a full-feature siocons replacement, but was the initial result of implementing a serial executable loader in Go. Unless specified it finds the first available serial port and defaults to a baud rate of 115200. Usage should be simply:

```bash
$ bin/sioload pkg/format/ecoff/testdata/main-ecoff
```

I am pleased to report that it has been working very consistently (so far) and for all tested baud rates! I am using a Net Yaroze DTL-H3050 serial communications cable connected via usb using a [TRENDnet USB to Serial converter](https://www.amazon.com/dp/B0007T27H8/ref=cm_sw_em_r_mt_dp_U_FHmgEbZAAPNX5).

## Reference

- [mipsel-ecoff-toolchain](https://github.com/ChrisRx/mipsel-ecoff-toolchain) - a compiler toolchain for Net Yaroze development on linux
- [psxdev.net](http://www.psxdev.net) - a helpful/active Playstation 1 development community, with relatively active forums
- [Identical Software](http://www.identicalsoftware.com/yaroze/) - a small collection of useful Net Yaroze related software
- [Nocash PSX Specification](https://problemkaputt.de/psx-spx.htm) - comprehensive PSX hardware information
- [Hitmen PSX FAQ](http://hitmen.c02.at/html/psx_faq.html)
- [Net Yaroze for Linux](https://www.cebix.net/downloads/yarlinux.pdf) - older guide for setting up Net Yaroze on linux
- [Andrew Kieschnick/Napalm](http://napalm-x.thegypsy.com/andrewk/psx/) - creator of eco2exe/exefixup and used as a guide for building the eco2exe in this repo

## TODO

- [x] ECOFF to PSX-EXE converter (eco2exe)
- [x] Net Yaroze executable serial loader (sioload)
- [x] PSX ISO builder (mkpsxiso), with CD-XA attributes, streamed XA/STR files and CD audio tracks
- [x] PSX ISO reader and extractor (dumpsxiso)
- [ ] Document code and add godoc badge to README.md
- [ ] Tests for all packages
- [ ] Include collected PDFs and other Net Yaroze documents in repo
- [ ] Add initial references/links
