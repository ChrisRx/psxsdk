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

or on its own with `go install go.chrisrx.dev/psx/cmd/nyc@latest`.

`nycw` is `nyc` with the WebAssembly compiler built in, so it builds programs with nothing but Go installed, and `go run` alone is enough:

```bash
go run go.chrisrx.dev/psx/cmd/nycw@latest disc ./mygame
```

Its commands and flags are `nyc`'s, less `--compiler`: it builds only with the WebAssembly compiler, even when there is a tinygo command, and always its own copy of it, ignoring `NYC_WASM`. The compiler and its sysroot are committed under `cmd/nycw/wasm`, which `task nyc` refreshes, building `.build/nycw` as well.

## The commands

- **`nyc build [package]`** writes the PSX-EXE, named after the package's directory unless `-o` names it. A name ending in `.elf` writes the ELF TinyGo linked instead, with the traps replaced, which has the symbols an emulator's debugger wants.
- **`nyc disc [package]`** writes the disc image, a `.bin` with its `.cue` beside it. `--elf` keeps the ELF too, `--file DISC=HOST` places more files, `--audio` adds CD audio tracks, and `-v` lists what went where.
- **`nyc run [package]`** builds the disc in a temporary directory and plays it in a window, booting the OpenBIOS built in. It is [`psxplay`](psxplay.md), inside `nyc`, with its keys and its flags: `--bios` boots another BIOS, `--card` attaches a memory card image, `--scale` sizes the window, `--mute` leaves the sound off, and `--fast-start=false` plays the start in real time rather than running it as fast as it goes until the first picture. The window loads X11, OpenGL and the sound library at run time, so `nyc` still builds with nothing but Go. With `--hot`, the program is built to take patches, and each save of the package's Go files while it runs builds one and loads it into the running program between frames: the main package's functions are replaced and its variables keep their values, so a change to the frame loop shows in about a second without the boot. A changed constant comes with the functions, and a variable the program has never written takes a changed initial value, while one it has written, a score say, keeps its state. A change that a patch cannot carry, to a variable's type or size, to a type's layout or to another package, is reported on standard error, and the program has to be started again.
- **`nyc patch --base prog.elf [package]`** builds such a patch by hand, for the program whose ELF `--base` names, built with `--hot` and kept with `--elf`, linked at `--at` inside the program's region, which an emulator loads with the `hot` package. It is what `run --hot` does on each save.
- **`nyc generate`** is [`discgen`](discgen.md), and **`nyc iso`** is [`mkpsxiso`](mkpsxiso.md) with every flag, for a disc laid out by hand.
- **`nyc env`** says which compiler a build would use, and where it is.

Every command that compiles takes `--opt`, TinyGo's optimization level, 2 by default since a frame has a budget, `--tags`, `--load file@addr` to put a file into the executable as siocons' `local dload` did, `--hot` to build a program that takes patches, which calls the main package's functions through a slot each and reserves 256KB for the patches, and `-v`, which says which compiler ran and how long each step took.

## The compiler

`nyc` compiles with one of two TinyGos, chosen with `--compiler` or `NYC_COMPILER`:

- **`tinygo`** is the tinygo command, `NYC_TINYGO` or the one on the `PATH`. A TinyGo without the PlayStation targets gets them anyway. The targets and the runtime for them only add files to TinyGo, so `nyc` carries those files (`internal/toolchain/_tinygo`) and lays them over the installed TinyGo's root, as a tree of links made once in its cache directory, and runs TinyGo with that as its root. The TinyGo has to be the version the files are written for, the revision the flake builds, since the runtime files are written against its internals.
- **`wasm`** is TinyGo compiled to WebAssembly, the compiler the [playground](/play/) runs in the browser ([how it is built](../architecture/compiler.md)), run in-process by [wazero](https://wazero.io/), so it needs no tinygo command, no go command and no C toolchain. It has no clang, so the target's C and assembly come compiled in its sysroot's cache, and it builds only at the optimization levels the cache holds. It reads the program and the modules it uses from the host's file system, read only. It is built into `nycw`, with a sysroot of its own that holds levels z and 2. Otherwise `NYC_WASM` names a directory holding `tinygo.wasm` and `sysroot.tar.gz`, as the site's `play/` directory does, and the two files can be downloaded from there. The playground's sysroot holds only z, which keeps it small for the browser, so a build with it is at `-opt=z` unless `--opt` asks for a level it cannot build at, which is an error.
- **`auto`**, the default, is `tinygo` when there is a tinygo command and `wasm` otherwise.

Both write the same executable, byte for byte. What they cost differs:

| Disc of | `tinygo` | `wasm` |
| --- | --- | --- |
| hello | 0.3s | 1.4s |
| square | 0.4s | 1.6s |
| dino | 0.5s | 1.8s |
| tunnel | 0.9s | 2.3s |
| terra | 1.7s | 3.4s |

Those are on a 24-thread x86-64 desktop. Each `wasm` build includes about half a second of loading the compiled module from the cache. The first build anywhere also unpacks the compiler and has wazero compile its 44MB module, which takes about 3 seconds over every core and is kept, about 270MB of it, in the cache directory. A `wasm` build of square peaks at about 660MB of memory, where `tinygo` takes about 130MB.

`NYC_CACHE` moves the cache directory, which is `nyc` under the user's cache directory otherwise.
