---
title: Getting started
weight: 1
group: Docs
section: Guide
description: Writing PlayStation programs in Go: getting started, the packages a program uses, how the machine works, making its data, and troubleshooting.
---

A PlayStation program is an ordinary Go module, and building one needs nothing but Go 1.27 or newer. The build command, `yaroze`, carries its own compiler, TinyGo compiled to WebAssembly, and its own emulator.

## A new program

Make a module, add the library, and add `yaroze` as a tool, which `go tool` then runs without installing anything:

```bash
mkdir mygame && cd mygame
go mod init example.com/mygame
go get go.chrisrx.dev/psx
go get -tool go.chrisrx.dev/psx/cmd/yaroze
```

Then a `main.go`:

```go src=../tutorial/hello/main.go
```

Build it onto a disc, or build it and play it in a window, with the keyboard as the controller:

```bash
go tool yaroze disc    # mygame.bin and mygame.cue
go tool yaroze run
```

The disc boots in any emulator that opens a cue sheet, and on a console that plays burned discs. `go tool yaroze build` stops at the executable, `mygame.exe`. [`yaroze`](../tools/yaroze.md) has the rest of the commands.

The program needs no build constraint. The `yaroze` package builds under the ordinary Go toolchain too, with the library's functions as stubs, so an editor shows their documentation and `go vet` and `go build` work on the program as on any other.

## Where next

- [Writing programs in Go](writing-programs.md): what the packages give a program.
- [The tutorial](../tutorial/index.md) explains the program above, then writes a bigger one, with a model, the controller and music.
- [The playground](/play/) does all of this in the browser, with nothing installed.

## The compiler

`yaroze` carries its compiler: TinyGo compiled to WebAssembly, the one the [playground](/play/) runs in the browser, so no tinygo command is involved and nothing but Go is installed. It runs under wazero, which is built in, or under the wasmtime command when there is one, which is quicker (`--runtime`, or `YAROZE_RUNTIME`). It builds at the optimization levels its sysroot's cache holds, 2 by default and z. [`yaroze`](../tools/yaroze.md) has the details.

The Nix flake in this repository provides a TinyGo with the PlayStation targets, which is what that compiler is built from and what working on the targets needs; it is not needed to build a program.

## Working on psxsdk itself

```bash
git clone https://github.com/ChrisRx/psxsdk
cd psxsdk
go test ./...          # or: task test, which vets too
nix develop            # or: direnv allow, for TinyGo, go-task, mednafen and a patched psxe
task build:discs       # every example onto .build/NAME/NAME.cue
task test:emu          # boot each disc in the emulator and compare screenshots
task serve             # build the site and the playground, and serve them on localhost:8000
```

`go run ./cmd/yaroze run ./examples/check` builds and plays one example, as `task play NAME=check` does. The build's single steps are Go tools too, so `go tool mkdisc` lays out a disc by hand.

`task test:emu` boots with the OpenBIOS image in `emu/openbios`, or whatever `PSX_BIOS` names. The memory card test drives Sony's shell, which OpenBIOS does not have, so it only runs when `PSX_SONY_BIOS` points at a Sony BIOS. The serial tests run against the clean-room monitor in `serial/monitor`, which `task build:serial` builds, and against Sony's monitor too when `PSX_YAROZE_BOOT` also names an image of the Net Yaroze boot disc, a `.bin` or the `.zip` redump has. No Sony BIOS or boot disc is distributed here.
