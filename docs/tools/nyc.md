---
title: nyc
weight: 1
description: Build a Go program for the Net Yaroze into a PSX-EXE or a disc image in one step, with the tinygo command or with TinyGo compiled to WebAssembly.
---

`nyc` builds Go programs for the Net Yaroze. From a main package it writes a PSX-EXE, or a disc image that boots one, in one step:

```bash
nyc build ./mygame      # mygame.exe
nyc disc ./mygame       # mygame.bin and mygame.cue
nyc run ./mygame        # the disc, played in a window
```

A build runs the steps that are tools of their own here. TinyGo compiles the package for the `playstation-yaroze` target. The trap instructions LLVM leaves in that an R3000A cannot run are replaced with nops, as [`mips1fix`](mips1fix.md) does. The ELF becomes a PSX-EXE with the Net Yaroze library combined in, as [`eco2exe`](eco2exe.md) does. For a disc, the executable goes on as `MAIN.EXE` with a `SYSTEM.CNF` naming it, together with the files the program registers with package [`disc`](../guide/writing-programs.md), as [`mkpsxiso`](mkpsxiso.md) does. Only the first step is outside `nyc`.

## Installing

As a tool dependency of a program's module, which `go tool` builds and runs:

```bash
go get -tool go.chrisrx.dev/psx/cmd/nyc
go tool nyc disc
```

or on its own with `go install go.chrisrx.dev/psx/cmd/nyc@latest`. In this repository, `task nyc` builds one with the WebAssembly compiler built in, onto `.build/nyc`.

## The commands

- **`nyc build [package]`** writes the PSX-EXE, named after the package's directory unless `-o` names it. A name ending in `.elf` writes the ELF TinyGo linked instead, with the traps replaced, which has the symbols an emulator's debugger wants.
- **`nyc disc [package]`** writes the disc image, a `.bin` with its `.cue` beside it. `--elf` keeps the ELF too, `--file DISC=HOST` places more files, `--audio` adds CD audio tracks, and `-v` lists what went where.
- **`nyc run [package]`** builds the disc in a temporary directory and plays it in a window, booting the OpenBIOS built in. It is [`psxplay`](psxplay.md), inside `nyc`, with its keys and its flags: `--bios` boots another BIOS, `--card` attaches a memory card image, `--scale` sizes the window, and `--mute` leaves the sound off. The window loads X11, OpenGL and the sound library at run time, so `nyc` still builds with nothing but Go.
- **`nyc generate`** is [`discgen`](discgen.md), and **`nyc iso`** is [`mkpsxiso`](mkpsxiso.md) with every flag, for a disc laid out by hand.
- **`nyc env`** says which compiler a build would use, and where it is.

Every command that compiles takes `--opt`, TinyGo's optimization level, 2 by default since a frame has a budget, `--tags`, `--load file@addr` to put a file into the executable as siocons' `local dload` did, and `-v`, which says which compiler ran and how long each step took.

## The compiler

`nyc` compiles with one of two TinyGos, chosen with `--compiler` or `NYC_COMPILER`:

- **`tinygo`** is the tinygo command, `NYC_TINYGO` or the one on the `PATH`. A TinyGo without the PlayStation targets gets them anyway. The targets and the runtime for them are all the TinyGo fork adds, so `nyc` carries those files and lays them over the installed TinyGo's root, as a tree of links made once in its cache directory, and runs TinyGo with that as its root. The TinyGo has to be the version the fork is of, since the runtime files are written against its internals.
- **`wasm`** is TinyGo compiled to WebAssembly, the compiler the [playground](/play/) runs in the browser, run in-process by [wazero](https://wazero.io/), so it needs no tinygo command, no go command and no C toolchain. It has no clang, so the target's C and assembly come compiled in its sysroot's cache, and it builds only at the optimization levels the cache holds. It reads the program and the modules it uses from the host's file system, read only. It is built into a `nyc` made by `task nyc`, with a sysroot of its own that holds levels z and 2. Otherwise `NYC_WASM` names a directory holding `tinygo.wasm` and `sysroot.tar.gz`, as the site's `play/` directory does, and the two files can be downloaded from there. The playground's sysroot holds only z, which keeps it small for the browser, so a build with it is at `-opt=z` unless `--opt` asks for a level it cannot build at, which is an error.
- **`auto`**, the default, is `tinygo` when there is a tinygo command and `wasm` otherwise.

Both write the same executable, byte for byte. What they cost differs:

| Disc of | `tinygo` | `wasm` |
| --- | --- | --- |
| hello | 0.3s | 2.9s |
| square | 0.4s | 4.1s |
| dino | 0.5s | 4.4s |
| tunnel | 0.9s | 8.1s |
| terra | 1.7s | 15.0s |

Those are on a 24-thread x86-64 desktop. Each `wasm` build includes about a second of loading the compiled module from the cache. The first build anywhere also unpacks the compiler and has wazero compile its 75MB module, which takes about 11 seconds over every core and is kept, about 520MB of it, in the cache directory. A `wasm` build of square peaks at about 1.4GB of memory, where `tinygo` takes about 130MB.

`NYC_CACHE` moves the cache directory, which is `nyc` under the user's cache directory otherwise.
