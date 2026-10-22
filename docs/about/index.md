---
title: History
weight: 3
description: The Net Yaroze, why psxsdk exists, what it changes about developing for it, and how it was made.
group: Docs
section: About
---

## The Net Yaroze

The Net Yaroze was Sony's kit for making PlayStation games at home, sold in Japan from 1996 and in Europe and North America from 1997, for $750 in the United States. The [Start Up Guide](../manuals/010-1998-startupguide.pdf) lists the package: a black PlayStation that only members could have, two controllers, an access card, a boot disc, a development disc, a serial cable and three manuals. Members also got a login to one of Sony's three members' websites, for Japan, Europe and North America, where they traded help and shared what they made.

A program never had a disc of its own. The boot disc left the library, `libps`, resident in RAM, and the program and its data came over the serial cable from the PC. The library could read files from a CD, but only from the boot disc. Sony's [FAQ](../manuals/faq.md) answers "No" both to reading any other CD and to making your own: the console's copy protection would not allow it, no doubt for fear of piracy. So everything had to fit in memory, and showing a game to anyone meant bringing the console, the boot disc, the cable and the PC along with it. The members' websites are gone, and the SDK is a set of binaries built for MS-DOS and Windows 95, around GCC 2.6.

## Why psxsdk exists

I wanted a Net Yaroze when it was announced, but $750 in 1990s money was more than a kid could manage. When I finally got one, in the mid-2010s, the first thing I found was how hard the SDK was even to run. It was built on GCC, but a GCC decades old, and since newer versions had dropped ECOFF there was no moving it forward. I wanted to use modern tools, and I wanted anyone to be able to build Net Yaroze programs. That started [mipsel-ecoff-toolchain](https://github.com/ChrisRx/mipsel-ecoff-toolchain), and then this repository.

The point is preservation: that anyone can still write, build and play Net Yaroze games, all these years later.

## What it changes

- **Programs boot from a disc.** The library is combined into the program, which boots from an ordinary disc in an emulator or on a console that plays burned discs, with no Net Yaroze hardware.
- **Data comes off the disc.** A program declares its files with package [`disc`](https://pkg.go.dev/go.chrisrx.dev/psx/disc), the build puts them on its disc, and the program reads them as it needs them, rather than everything having to fit in memory up front.
- **Programs are written in Go**, compiled by TinyGo against bindings that keep Sony's names, so the manuals still apply. One command, [`yaroze`](../tools/yaroze.md), builds a program onto a disc.
- **The tools run anywhere Go does**, and so does the emulator, which also runs in a browser and drives the tests. The converters for pictures, sounds, music and models are checked against what Sony's tools made from members' files.
- **Nothing is lost along the way.** Sony's [manuals](../manuals/index.md), supplements and FAQs are here as published, the [examples](../examples/index.md) port Sony's demos and members' games, and [`sioload`](../tools/sioload.md) still uploads a program to a real Net Yaroze over its cable.

## How it was made

Some of what it took, each written up in full elsewhere:

- **Sony's library, relinked.** `libps` came as ECOFF objects, a format current toolchains no longer link. [`ecoff2asm`](../tools/ecoff2asm.md) turns them into GNU as source, and `ecoffcmp` checks the result against the original word by word.
- **A disc's files, declared in the program.** `disc.Register` compiles to a record of constant data in the executable. The disc builder finds each record, places the file, and writes its sector back into the record, so the program reads the file without searching the disc. One declaration serves the build and the program both.
- **A compiler in the browser.** The playground runs TinyGo itself, compiled to WebAssembly with LLVM and lld linked in. Clang, which would be more than half of the module, is left out: the target's C comes precompiled, in a cache that a second build with clang fills ahead of time. [`yarozew`](../tools/yaroze.md) carries the same compiler, 22MB gzipped, so `go run` alone builds a program. [The compiler in the browser](../architecture/compiler.md) tells how it is built.
- **An emulator for tests.** [The emulator](../architecture/emulator.md) is deterministic, counting emulated cycles rather than wall time, so a test takes the same course every run. It was traced instruction by instruction against psxe, its reverb matches Mednafen's sample for sample, and it skips the library's busy wait for the vertical blank, 28% of the instructions one example executes, with a test that checks nothing visible changes.
- **A game with no source.** [Terra Incognita](../examples/terra.md), Team Fatal's Net Yaroze game, was reconstructed from its executable: its island, creatures and scenes are the original's data files, and everything that moves them is Go, matching the original's story through long runs of scripted play.
- **Compression chosen by measurement.** The archives pack with deflate, not the LZSS games of the time used, because it packs the examples' textures and models a quarter to a third smaller than LZ4, and unpacks on the console at or above the speed a double speed drive reads. [Compression](../architecture/compression.md) has the measurements.

## Goals

- Software that is cross-platform and easy to build
- Modern languages with features that help the software last
- Libraries and tools that are well-defined and well-documented
- Source code for everything, always available
