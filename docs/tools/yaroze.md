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
yaroze run mygame.cue      # a disc as it is, played in a window, or a .zip of one
yaroze debug ./mygame      # the program stopped at its start, for GDB
```

A build runs the steps that are tools of their own here. TinyGo compiles the package for its `playstation` target, with what makes the program a Net Yaroze's laid over it and the library linked in (see [the target](../architecture/compiler.md#the-target)). The trap instructions LLVM leaves in that an R3000A cannot run are replaced with nops, as [`mips1fix`](mips1fix.md) does. The ELF becomes a PSX-EXE as it is, as [`elf2exe`](elf2exe.md) does. With `--libps resident` the program calls the resident library at its addresses and is combined with that library's image instead, as [`eco2exe`](eco2exe.md) does. For a disc, the executable goes on as `MAIN.EXE` with a `SYSTEM.CNF` naming it, together with the files the program registers with package [`disc`](../guide/writing-programs.md), as [`mkdisc`](mkdisc.md) does. Only the first step is outside `yaroze`.

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
- **`yaroze disc [package]`** writes the disc image, a `.bin` with its `.cue` beside it, or with `-o mygame.zip` the two in [one file](mkdisc.md#one-file). `--elf` keeps the ELF too, `--file DISC=HOST` places more files, `--audio` adds CD audio tracks, and `-v` lists what went where.
- **`yaroze run [package | project | executable | disc.cue | disc.zip]`** builds the disc in a temporary directory and plays it in [a window](#the-window), booting the OpenBIOS built in. Given a cue sheet or a `.zip` holding one, it plays that disc as it is, with nothing built. It also runs a Net Yaroze program built already, as people handed them around: a directory or a `.zip` with no cue sheet, such as one from the showcase, holding the executable, its data and the `auto` file that told siocons what to load where. `yaroze run gasgar.zip` runs the program the `auto` file loads, with the Net Yaroze library laid under it and the files its `dload` lines name put in memory, from a disc holding the directory, for a program that reads its files from the CD. `--auto` names the `auto` file when there are several, as a path inside, or names one to run directly; with no `auto` file a lone executable is run. Given an executable, `yaroze run MAIN` takes its files from an `auto` file beside it that loads it. `--exe prog.exe` runs an executable instead, a PSX-EXE, an ECOFF or an ELF, loaded when the BIOS reaches its shell as `psxemu --exe` does, with `--load file@addr` putting files in memory beside it as siocons' `local dload` did; an ECOFF or an ELF gets the library laid under it, as `eco2exe` does. With a directory or a `.zip`, `--exe` names the program inside to run. With `--hot`, the program is built to take patches, and each save of the package's Go files while it runs builds one and loads it into the running program between frames: the main package's functions are replaced and its variables keep their values, so a change to the frame loop shows in about a second without the boot. A changed constant comes with the functions, and a variable the program has never written takes a changed initial value, while one it has written, a score say, keeps its state. A change that a patch cannot carry, to a variable's type or size, to a type's layout or to another package, is reported on standard error, and the program has to be started again.
- **`yaroze debug [package | program | disc.cue]`**, or `dbg`, runs a program stopped at its first instruction and debugs it, in the emulator or on a console over the serial cable: a Go program with Delve, built in, and any program with GDB. [Debugging](#debugging) has the details.
- **`yaroze patch --base prog.elf [package]`** builds such a patch by hand, for the program whose ELF `--base` names, built with `--hot` and kept with `--elf`, linked at `--at` inside the program's region, which an emulator loads with the `hot` package. It is what `run --hot` does on each save.
- **`yaroze generate`**, or `gen`, is [`discgen`](discgen.md), and **`yaroze iso`** is [`mkdisc`](mkdisc.md) with every flag, for a disc laid out by hand.
- **`yaroze tim`**, **`tmd`**, **`vag`**, **`vab`**, **`seq`**, **`scene`** and **`archive`** (or `arc`) are the converters [`png2tim`](png2tim.md), [`obj2tmd`](obj2tmd.md), [`wav2vag`](wav2vag.md), [`mkvab`](mkvab.md), [`smf2seq`](smf2seq.md), [`mkscene`](mkscene.md) and [`mkarc`](mkarc.md), with the same flags.
- **`yaroze sio`** is [`siocons`](siocons.md), Sony's SIOCONS for a Net Yaroze on the serial cable.
- **`yaroze vet [package]...`** reports the variables a program passes to the library that the library goes on using after they are gone: one of a function other than `main` passed to `LoadImage`, which reads it until `DrawSync`, with no `DrawSync` before the function returns, or one made in a loop or in the body of the frame loop passed to `GsSetWorkBase`, which keeps it for good and would have it made anew every frame. `build`, `disc`, `run` and `patch` check the package the same way before they compile it, and the compiler refuses what is left, through other packages and helpers; `vet` checks without building, and `./...` checks every package under the directory.
- **`yaroze env`** says how a build would run the compiler, and where it is.

`--debug` on `build` and `disc` keeps the DWARF debug information, the line tables and where the variables are, and writes the ELF that holds it beside the output: `mygame.elf` beside `mygame.exe`, or the file `--elf` names for a disc. The PSX-EXE holds no debug information either way. A build with `--debug` has the same instructions as the build without it, but some of its variables can lie at other addresses, so debug a program with the ELF of the same build.

Every command that compiles takes `--opt`, TinyGo's optimization level, 2 by default since a frame has a budget, `--tags`, `--load file@addr` to put a file into the executable as siocons' `local dload` did, `--hot` to build a program that takes patches, which calls the main package's functions through a slot each and reserves 256KB for the patches, `--target FILE.json`, a TinyGo target file of the program's own that inherits `playstation` and changes what the program needs, such as where it is linked, which is built over the resident library's files, `--experiment=noonstore` to turn off a compiler [experiment](../architecture/compiler.md#experiments), and `-v`, which says how the compiler ran and how long each step took.

`--libps` chooses how a program gets the Net Yaroze library. `static`, the default, links the library in like any package: only what the program calls is linked, and the program loads at `0x80010000`. `resident` lays the program over the image a Net Yaroze leaves in memory, at `0x80090000`, as an original program runs, with the library at its fixed addresses. That is the way to run a program on a real Net Yaroze through its boot disc. A C project is always built this way.

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

## Debugging

`yaroze debug` runs a program stopped at its first instruction and debugs it, with a debugger at a prompt in the same terminal. Given a package, it builds it with `--debug` and writes the ELF beside it. A Go program is debugged with [Delve](https://github.com/go-delve/delve), which is built in:

```console
$ yaroze debug ./mygame
yaroze: the program is stopped at 80010000, in the emulator, and Delve debugs it; GDB can connect on 127.0.0.1:1234 too
Type 'help' for list of commands.
(dlv) break main.tick
(dlv) continue
(dlv) args
(dlv) print main.frames
```

`--dap ADDRESS` serves Delve's debug adapter protocol on the address instead, for an editor: its launch configuration needs nothing but to connect, as `yaroze debug` gives Delve the program and the way to it. The session ends when the editor disconnects.

Any other program, in C or C++ or an ECOFF, is debugged with GDB:

```console
$ yaroze debug build/hello.elf
yaroze: the program is stopped at 80140000, in the emulator, and GDB debugs it; another GDB can connect on 127.0.0.1:1234
Reading symbols from /home/me/hello/build/hello.elf...
Remote debugging using 127.0.0.1:1234
0x80140000 in _start ()
(gdb)
```

The GDB is GNU GDB 17 for WebAssembly, from [gdb-wasi](https://github.com/ChrisRx/gdb-wasi), under GDB's own license. `yaroze debug` downloads it the first time, 3 MB, keeps it in its cache directory with the compilers, and runs it itself, so nothing needs installing. `YAROZE_GDB` names a `gdb.wasm` on disk to run instead. It skips `~/.gdbinit`, which is usually for the host's own GDB.

Control-C while the program runs stops it, for either debugger, and the debugger says so. Two within half a second end `yaroze debug`.

Either way, and for any program, `yaroze debug` serves GDB's remote protocol, so that another GDB, or another debugger that speaks it, debugs the program too. `--serve` runs no debugger of its own, and says what to type:

```console
$ yaroze debug --serve ./mygame
yaroze: the program is stopped at 80010000, in the emulator
yaroze: waiting for a debugger on 127.0.0.1:1234; in another terminal:

    gdb mygame.elf -ex 'target remote 127.0.0.1:1234'
```

That GDB has to be one built for MIPS targets. A distribution's `gdb-multiarch` is, and so is the `gdb` in nixpkgs (`nix shell nixpkgs#gdb`). [Debugging](../guide/debugging.md) goes through a session with each debugger.

What it is given says what it debugs:

- **A package**, the current directory's if nothing is given, is built with `--debug`. The ELF goes to the package's name with `.elf`, or to the file `--elf` names.
- **A program built already**: an ELF from `build --debug`, or from GCC with `-g`, which has the symbols too, or a PSX-EXE, with `--elf` naming the ELF it was made from.
- **An ECOFF**, as the Net Yaroze's own GCC 2.95.3 builds. Its debugging information is stabs, which GDB from 16 on crashes on and LLDB never read, so `yaroze debug` writes an ELF with DWARF made from it beside it, the program's name with `.elf` added, and GDB reads that. The program itself runs as it is. The ELF has the functions with their sizes, the resident library's entry points, the source lines, each function's frame, and the types and variables of files compiled with `-g`. [`ecoff2elf`](ecoff2elf.md) makes the same ELF on its own.
- **A disc's cue sheet**, or a `.zip` of one, with `--elf` naming the ELF of the program on it. Without `--elf` the machine is stopped at the BIOS's first instruction, with no symbols.

`--listen` sets the address to wait on, `localhost:1234` if not given. A debugger that detaches leaves the program running, and the next one to connect stops it. GDB's `kill` ends `yaroze debug`. `--packets` prints each packet to and from the debugger.

### In the emulator

Without `--port` the program runs in the emulator, in [the window](#the-window) with the window's flags, on a disc with its files as `yaroze disc` builds it. `--headless` runs it with no window, at a console's pace, or with `--fast` as fast as the host allows.

The debugger works the emulated machine itself, with no monitor in its memory:

- A step is one instruction. A jump or a branch is stepped together with the instruction in its delay slot.
- A breakpoint is the emulator's own and writes nothing into the program, so one can be set before the BIOS has loaded the program. `break` and `hbreak` do the same.
- A fault, such as an address error or a reserved instruction, stops the program at the instruction that took it, in place of going on into the kernel's handler. So does a `break` instruction.
- Control-C stops the program at the end of the frame it is in.
- What the program prints shows in GDB while the program runs.

There are no watchpoints yet. GDB's `watch` works with `set can-use-hw-watchpoints 0`, by stepping, which is slow.

### On a console

With `--port DEVICE[,BAUD]` the program is debugged on a console at the other end of the serial cable, with the monitor waiting at its prompt: the Net Yaroze boot disc's monitor, or the clean one, [`serial/monitor`](https://pkg.go.dev/go.chrisrx.dev/psx/serial/monitor). `-B` gives the speed too, 9600 if neither does.

```bash
yaroze debug --port /dev/ttyUSB0,115200 ./mygame
```

The package is built over the library the console has in memory, as `--libps resident` does, uploaded as [`siocons`](siocons.md) uploads a program, and debugged through the monitor's remote debugger, the one the Net Yaroze's own GDB used. `--start` says how the program is started:

| `--start` | The monitor | The program |
| --- | --- | --- |
| `go`, the default | the clean one | Started with `go`, with a breakpoint at its first instruction. Control-C stops it. |
| `rdb` | either | Started under the monitor's `rdb`. Once it runs it cannot be stopped, short of a breakpoint or a fault. |
| `attach` | the clean one | Nothing is uploaded. The program that `go` started earlier is stopped where it is. |

A breakpoint here is a `break` instruction written over the program while it runs, and taken out again when it stops. A step is the monitor's, which runs a jump or a branch together with its delay slot. Memory outside RAM, the scratchpad, the I/O ports and the BIOS is refused, since reading it would be a bus error in the monitor itself.

The library a console has in memory has no symbols in the program's ELF, so a backtrace from inside the library stops there.

## C projects

`build`, `disc` and `run` also take a directory of C sources with no Go in it, such as a project from the Net Yaroze websites, laid out as the SDK left it:

```bash
yaroze run ./chap1/3       # main.c, Makefile and auto, as they came
```

The `Makefile` is read, not run, for the link address (`-Ttext`), the `-D` and `-I` flags, and the objects of the program `auto` loads, or else of the first thing `all` makes. With no `Makefile`, every `.c` file in the directory is built and the program is linked at 0x80140000. Each `local dload` line in `auto` puts that file in the executable at its address, as `--load` does.

The sources are copied first, changed where a DOS compiler accepted what a current one does not: line ends, the 0x1A that ended a DOS file, includes in any case and with backslashes, a Shift-JIS character whose second byte is a backslash, and a string running over several lines. They compile as gnu89 against headers written for this repository from the [Library Reference](../manuals/index.md), which have every struct, constant and function of the SDK's in the same layout, and link against the resident library at its fixed addresses, as the SDK's did.

The shipped compiler has no clang, so the first C build downloads the one that does, 30.5MB, from `files.yaroze.dev`, checks it against a hash built into `yaroze`, and keeps it in the cache directory. A Go program never needs it, and neither does the playground. `YAROZE_CC` names a `tinygo-full.wasm` on disk to use instead.

Of the 173 projects on the websites with an `auto` file and C source, 154 build. 95 of those came with an executable to compare after 1500 frames with no input: 75 draw the same video memory, byte for byte, 19 differ by under half a percent, and the other's source is not what its executable was built from. The same 154 build with a current GCC and Sony's headers. 134 of them match it with and without input, and the global variables of all of them have the same sizes and contents. Of the 20 that differ, the three that differ by more than half a percent run the same logic at another speed or draw different random numbers, since C does not say which of `f(rand(), rand())` is called first. Most of the 19 that do not build need a file that is not in the archive, a header or another library, or name a precompiled `.o` with no source.

Old code leans on what GCC happened to do, so a C build keeps to that: every variable is word aligned, since the library stores words into a `PACKET` array of bytes, a local starts at zero rather than being undefined, and the optimizer makes no assumptions about aliasing, overflow or null pointers. `--hot`, `--target`, `--tags` and `--experiment` are for Go packages.

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
