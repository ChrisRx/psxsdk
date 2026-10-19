---
title: Setting up
weight: 1
description: A module of your own with the toolchain as a dependency, the smallest program that puts something on the screen, and the one command that turns it into a disc.
---

A program for the PlayStation is an ordinary Go module. It depends on `go.chrisrx.dev/psx` for the library bindings, and on its build command, `nyc`, for turning the program into a disc. This page sets one up and gets a line of text on the screen.

## What you need

- **Go**, 1.27 or newer.
- **TinyGo**, which compiles Go for the console's MIPS processor and links against the Net Yaroze library. Any TinyGo of the version the toolchain is built against will do: `nyc` adds the `playstation-yaroze` target to one that does not have it. Without TinyGo, `nyc` can run the compiler the [playground](/play/) runs, TinyGo compiled to WebAssembly, instead; [its page](../tools/nyc.md) says how.
- **An emulator** to run the result in. The toolchain has one: `nyc run` plays a program in a window, and `psxemu` runs one headless and saves a picture of the screen. [mednafen](https://mednafen.github.io/) or any other that takes a cue sheet works too.
- **A BIOS image**, for `psxemu` and other emulators. The console's own is Sony's and cannot be distributed, but the open source [OpenBIOS](https://github.com/grumpycoders/pcsx-redux/tree/main/src/mips/openbios) boots everything here, and `nyc run` has it built in. The copy the [examples](../examples/index.md) on this site run on is at [openbios.bin](/play/openbios.bin).

## The module

Make a directory and a module in it, add the toolchain, and add `nyc` and the emulator as tool dependencies, which `go tool` then runs without installing anything:

```bash
mkdir hello && cd hello
go mod init example.com/hello
go get go.chrisrx.dev/psx
go get -tool go.chrisrx.dev/psx/cmd/nyc go.chrisrx.dev/psx/tools/psxemu
curl -LO https://chrisrx.github.io/psxsdk/play/openbios.bin
```

## The program

One file, `main.go`:

```go src=hello/main.go
```

It already has the shape every program has: set up the display, then, for ever, draw a frame, wait for the screen, and swap. Three things about it are not how a program for a modern machine would read.

**The screen is a package-level variable.** The machine has two megabytes of RAM, no memory protection and, in this runtime, no garbage collector running, so memory allocated in the frame loop is memory the program never gets back. A program sets up what it needs once, at startup, in variables that last as long as it does, and the frame loop allocates nothing. `Open` allocates the screen's buffers then, and only then.

**The frame is a list before it is a picture.** The GPU does not draw what it is told as it is told. A program sorts drawing commands into an ordering table, an array of linked lists indexed by depth, and then hands the table to the GPU, which walks it while the program goes on to the next frame. There are two tables, and two areas for the commands themselves, because the GPU is still drawing one frame while the program builds the next. The `Screen` holds both pairs. `Open` sets the console's video mode and the display, 320 by 240 unless its fields say otherwise, and `Frames` is the loop: each time round it empties the table for the buffer about to be drawn and hands it to the loop's body, and when the body is done it waits for the GPU and the display, swaps the buffers, sorts a command that clears the screen to its `Background` color where the GPU will meet it before anything else, and starts the GPU on the table. Underneath are the library's own `GsInitGraph`, `GsClearOt`, `GsSortClear`, `GsDrawOt` and the rest, which a program can call itself, and `Screen`'s documentation says which it calls.

**The text goes through the library's font.** `Font` puts the library's 8x8 font into video memory at a corner the display never shows and opens a window of text, here at 16, 16 and 256 by 64 pixels. `Print` writes into it, and the frame draws it when it ends. The library's own `FntPrint` is printf-like and takes a C string; `Print` takes a Go string and copies it, with the terminator C wants, into a buffer of its own.

The program needs no build constraint. The `yaroze` package builds under the ordinary Go toolchain too, with every function of the library a stub that panics if it is called, so an editor shows the library's documentation as you write, and `go vet ./...` and `go build ./...` work on the program. Only TinyGo's PlayStation target has the library itself, and only a program it built runs.

## Building it

One command turns the source into a disc the console boots:

```bash
go tool nyc disc
```

It does four things. TinyGo compiles and links the program against the Net Yaroze library's addresses. The few instructions LLVM emits that the console's R3000A does not have are replaced. The program is combined with the library itself, which a Net Yaroze had resident in RAM and an ordinary console does not, into the PSX-EXE the BIOS knows how to load. And the executable goes onto a disc as `MAIN.EXE`, with the `SYSTEM.CNF` the BIOS reads to find it. The disc is named after the directory, `hello.bin` with `hello.cue` beside it, which any emulator opens. `go tool nyc build` stops at the executable, `hello.exe`, and `-v` says how long each step took.

## Running it

`nyc run` builds the disc as `nyc disc` does and plays it in a window, with the keyboard as the controller, arrows for the directions, Enter for Start and Z, X, A and S for the four buttons:

```bash
go tool nyc run
```

`psxemu` boots the disc headless instead and, when stopped, saves what the screen shows:

```bash
go tool psxemu --bios openbios.bin --disc hello.cue --png hello.png
```

Stop it after a few seconds and `hello.png` holds the frame: a teal screen with the text in the corner. mednafen and other emulators open the cue sheet directly, and a console that plays burned discs boots it from a CD-R. This is the same program, built as this page builds it:

<!-- player hello -->

The next page starts on the square: [the frame and the controller](frame.md).
