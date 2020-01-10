# psxsdk

psxsdk is a collection of tools and libraries for Playstation 1 development.

- [What is psxsdk](#what-is-psxsdk)
- [Getting started](#getting-started)
- [What's included](#whats-included)
  - [eco2exe](#eco2exe)
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

The only requirement for building is [Go 1.13](https://golang.org/dl/#stable).

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
- [ ] PSX ISO builder
- [ ] Document code and add godoc badge to README.md
- [ ] Tests for all packages
- [ ] Split library code into separate repo/Go module
- [ ] Setup drone.io and goreleaser (publish binaries to GitHub Releases)
- [ ] Include collected PDFs and other Net Yaroze documents in repo
- [ ] Add initial references/links
