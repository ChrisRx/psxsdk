---
title: Getting started
weight: 1
group: Docs
section: Guide
description: Writing PlayStation programs in Go: getting started, the packages a program uses, how the machine works, making its data, and troubleshooting.
---

A PlayStation program is an ordinary Go module, and building one needs nothing but Go 1.27 or newer. The build command, `nycw`, carries its own compiler, TinyGo compiled to WebAssembly, and its own emulator.

## A new program

Make a module, add the library, and add `nycw` as a tool, which `go tool` then runs without installing anything:

```bash
mkdir mygame && cd mygame
go mod init example.com/mygame
go get go.chrisrx.dev/psx
go get -tool go.chrisrx.dev/psx/cmd/nycw
```

Then a `main.go`:

```go src=../tutorial/hello/main.go
```

Build it onto a disc, or build it and play it in a window, with the keyboard as the controller:

```bash
go tool nycw disc    # mygame.bin and mygame.cue
go tool nycw run
```

The disc boots in any emulator that opens a cue sheet, and on a console that plays burned discs. `go tool nycw build` stops at the executable, `mygame.exe`. [`nyc`](../tools/nyc.md) has the rest of the commands.

The program needs no build constraint. The `yaroze` package builds under the ordinary Go toolchain too, with the library's functions as stubs, so an editor shows their documentation and `go vet` and `go build` work on the program as on any other.

## Where next

- [Writing programs in Go](writing-programs.md): what the packages give a program.
- [The tutorial](../tutorial/index.md) explains the program above, then writes a bigger one, with a model, the controller and music.
- [The playground](/play/) does all of this in the browser, with nothing installed.

## With TinyGo installed

`nyc` is the same command without the compiler built in. When there is a tinygo command it uses that, adding the PlayStation targets if it lacks them, and can then build at any optimization level. Otherwise it falls back to the WebAssembly compiler:

```bash
go get -tool go.chrisrx.dev/psx/cmd/nyc
go tool nyc run
```

The Nix flake in this repository provides a TinyGo with the PlayStation targets, which the examples are built with.

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

`go run ./cmd/nyc run ./examples/check` builds and plays one example, as `task play NAME=check` does. The build's single steps are Go tools too, so `go tool mkpsxiso` lays out a disc by hand.

`task test:emu` boots with the OpenBIOS image in `emu/openbios`, or whatever `PSX_BIOS` names. The memory card test drives Sony's shell, which OpenBIOS does not have, so it only runs when `PSX_SONY_BIOS` points at a Sony BIOS. No Sony BIOS is distributed here.
