---
title: yaroze
weight: 1
description: Build a Go program for the Net Yaroze into a PSX-EXE or a disc image in one step, with nothing but Go installed.
---

`yaroze` builds Go programs for the Net Yaroze. From a main package it writes a PSX-EXE, or a disc image that boots one, in one step:

```bash
yaroze build ./mygame      # mygame.exe
yaroze disc ./mygame       # mygame.bin and mygame.cue
yaroze run ./mygame        # the disc, played in a window
yaroze run mygame.cue      # a disc as it is, played in a window
```

A build runs the steps that are tools of their own here. TinyGo compiles the package for the `playstation-yaroze` target. The trap instructions LLVM leaves in that an R3000A cannot run are replaced with nops, as [`mips1fix`](mips1fix.md) does. The ELF becomes a PSX-EXE with the Net Yaroze library combined in, as [`eco2exe`](eco2exe.md) does. For a disc, the executable goes on as `MAIN.EXE` with a `SYSTEM.CNF` naming it, together with the files the program registers with package [`disc`](../guide/writing-programs.md), as [`mkdisc`](mkdisc.md) does. Only the first step is outside `yaroze`.

## Installing

`yaroze` has its compiler built in, TinyGo compiled to WebAssembly, so it builds programs with nothing but Go installed, and `go run` alone is enough:

```bash
go run go.chrisrx.dev/psx/cmd/yaroze@latest disc ./mygame
```

As a tool dependency of a program's module, which `go tool` builds and runs:

```bash
go get -tool go.chrisrx.dev/psx/cmd/yaroze
go tool yaroze disc
```

or on its own with `go install go.chrisrx.dev/psx/cmd/yaroze@latest`.

## The commands

- **`yaroze build [package]`** writes the PSX-EXE, named after the package's directory unless `-o` names it. A name ending in `.elf` writes the ELF TinyGo linked instead, with the traps replaced, which has the symbols an emulator's debugger wants.
- **`yaroze disc [package]`** writes the disc image, a `.bin` with its `.cue` beside it. `--elf` keeps the ELF too, `--file DISC=HOST` places more files, `--audio` adds CD audio tracks, and `-v` lists what went where.
- **`yaroze run [package | disc.cue]`** builds the disc in a temporary directory and plays it in [a window](#the-window), booting the OpenBIOS built in. Given a cue sheet, it plays that disc as it is, with nothing built. `--exe prog.exe` runs a PSX-EXE instead, loaded when the BIOS reaches its shell as `psxemu --exe` does, with `--load file@addr` putting files in memory beside it as siocons' `local dload` did. With `--hot`, the program is built to take patches, and each save of the package's Go files while it runs builds one and loads it into the running program between frames: the main package's functions are replaced and its variables keep their values, so a change to the frame loop shows in about a second without the boot. A changed constant comes with the functions, and a variable the program has never written takes a changed initial value, while one it has written, a score say, keeps its state. A change that a patch cannot carry, to a variable's type or size, to a type's layout or to another package, is reported on standard error, and the program has to be started again.
- **`yaroze patch --base prog.elf [package]`** builds such a patch by hand, for the program whose ELF `--base` names, built with `--hot` and kept with `--elf`, linked at `--at` inside the program's region, which an emulator loads with the `hot` package. It is what `run --hot` does on each save.
- **`yaroze generate`** is [`discgen`](discgen.md), and **`yaroze iso`** is [`mkdisc`](mkdisc.md) with every flag, for a disc laid out by hand.
- **`yaroze env`** says how a build would run the compiler, and where it is.

Every command that compiles takes `--opt`, TinyGo's optimization level, 2 by default since a frame has a budget, `--tags`, `--load file@addr` to put a file into the executable as siocons' `local dload` did, `--hot` to build a program that takes patches, which calls the main package's functions through a slot each and reserves 256KB for the patches, and `-v`, which says how the compiler ran and how long each step took.

## The window

`yaroze run` plays in a window, with the keyboard or a gamepad as the pad in port 1 and the SPU's sound on the speakers. It is the desktop counterpart of the player on the [examples](../examples/index.md) pages of this site, which runs the same emulator in a browser.

OpenBIOS is built in, so no BIOS image is needed, and `--bios` boots another, such as Sony's. `--card` attaches a memory card image to slot 1 and writes it back on exit, `--scale` sets the window's initial size as a multiple of 320x240, and `--mute` leaves the sound off. The BIOS's start and a program's loading, a few seconds of black screen, run as fast as the host allows, with no picture or sound, until the display first shows something, and in real time from then on. `--fast-start=false` runs them in real time too. What the program prints goes to standard output. `--break 0x80012345` prints the frame and the argument, return address and stack registers each time the program reaches an address, which `go tool nm` on the program's ELF gives for a function. It can be given more than once.

The keyboard is laid out as the player has it:

| Keys | Pad |
| --- | --- |
| Arrows | D-pad |
| Z, X, A, S | Cross, circle, square, triangle |
| Q, W | L1, R1 |
| 1, 2 | L2, R2 |
| Enter, Backspace | Start, Select |

A gamepad with a standard layout works as itself, with the left stick doubling as the d-pad. F11 toggles full screen and Escape leaves it. F12 saves the picture as a PNG at the display's own size, `shot-0001.png` and so on under the first free number, into the directory `--shots` names, the current one by default. Sending the process SIGUSR1 saves one the same way, so that a script can take a picture of a running window: `kill -USR1 $(pgrep -f 'yaroze run')`. The picture flashes brighter for a moment, and the path written is printed on standard output.

In the repository, `task play NAME=check` builds an example and plays it, with flags after `--` passed on: `task play NAME=check -- --mute`.

The window is drawn with [Ebitengine](https://ebitengine.org), which loads X11, OpenGL and ALSA at run time rather than linking against them, so `yaroze` still builds with nothing but Go. On Linux the libraries must be on the loader's path to run it, which the Nix development shell does.

## The compiler

The compiler is the one the [playground](/play/) runs in the browser: TinyGo compiled to WebAssembly ([how it is built](../architecture/compiler.md)), so it needs no tinygo command, no go command and no C toolchain. It has no clang, so the target's C and assembly come compiled in its sysroot's cache, and it builds only at the optimization levels the cache holds: `yaroze`'s own sysroot holds z and 2. It reads the program and the modules it uses from the host's file system. The compiler and its sysroot are committed under `cmd/yaroze/wasm`, which `task yaroze` refreshes, building `.build/yaroze` as well, and are unpacked once into the cache directory, keyed by the build. `YAROZE_WASM` names a directory holding another `tinygo.wasm` and `sysroot.tar.gz`, as the site's `play/` directory does, to build with that compiler instead, for working on it; the playground's sysroot holds only z, which keeps it small for the browser, so a build with it is at `-opt=z` unless `--opt` asks for a level it cannot build at, which is an error.

What runs the module is chosen with `--runtime` or `YAROZE_RUNTIME`:

- **`wazero`** is [wazero](https://wazero.io/), built in, so it is always there. It keeps the module loaded for the builds of a command, so the patches `run --hot` makes get the compiler with the standard library already parsed and type-checked, which is most of a build's time outside LLVM. Between commands a snapshot of the compiler's memory does the same: the first build of a process writes the module's memory, about 140MB, to a file of the user's alone in the temporary directory, and the next process maps it under a fresh module in place of starting the compiler. The file lives in the temporary directory because the system clears that as it sees fit and a snapshot is a build away from being made again; a snapshot that fails to restore is removed and the build starts afresh. The first build anywhere also has wazero compile the 44MB module, which takes a few seconds over every core and is kept, about 270MB of it, beside the unpacked compiler.
- **`wasmtime`** is the [wasmtime](https://wasmtime.dev/) command, `YAROZE_WASMTIME` or the one on the `PATH`, which runs the module as a command for each build. Its code runs the compiler about twice as fast as wazero's, and the module it compiles once, beside the unpacked compiler and tied to the command's version, loads in a few milliseconds, so a build is as quick as one from a snapshot with nothing kept in memory or in the temporary directory. The command has no read-only mount, so the compiler sees the whole file system writable, where wazero gives it the file system read only.
- **`auto`**, the default, is `wasmtime` when there is a wasmtime command and `wazero` otherwise.

Both write the same executable, byte for byte. What a build costs, on a 24-thread x86-64 desktop:

| Disc of | `wazero`, no snapshot | `wazero`, from the snapshot | `wasmtime` |
| --- | --- | --- | --- |
| square | 1.2s | 0.7 to 1.2s | 0.5 to 0.8s |
| dino | 1.5s | 0.8 to 1.4s | 0.7s |
| terra | 3.3s | 3 to 4.5s | 1.7s |

Those include about 0.2 seconds of wazero loading its compiled module. The wazero is a fork ([ChrisRx/wazero](https://github.com/ChrisRx/wazero), branch `psxsdk`) with one change: on a cache hit it does not validate the module's function bodies again, which was another 0.3 seconds of every start. A large program gains little from the snapshot under wazero, since LLVM is most of its build and the kept standard library is more for the collector to walk. The first build on a machine also unpacks the compiler and has the runtime compile the 44MB module, about 4 seconds under wazero over every core and 5 under wasmtime, once per runtime. A build of square peaks at about 660MB of memory under wazero.

The cache directory, `YAROZE_CACHE` or `yaroze` under the user's cache directory, holds a compiler per build of `yaroze` that was run, each about 300MB with what the runtimes make of it. They are trimmed the way the go command trims its build cache: a compiler is marked used at most once an hour, and once a day those not used for five days are removed.
