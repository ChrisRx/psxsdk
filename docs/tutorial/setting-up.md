---
title: Setting up
weight: 1
description: A module of your own with the toolchain as a dependency, the smallest program that puts something on the screen, and the steps that turn it into a disc.
---

A program for the PlayStation is an ordinary Go module. It depends on `go.chrisrx.dev/psx` for the library bindings, and on the module's tools for turning what the compiler produces into a disc. This page sets one up and gets a line of text on the screen.

## What you need

- **Go**, 1.27 or newer.
- **TinyGo** with the `playstation-yaroze` target, which compiles Go for the console's MIPS processor and links against the Net Yaroze library. `tinygo targets | grep playstation` shows whether yours has it.
- **An emulator** to run the result in. The toolchain has one, `psxemu`, which runs headless and saves a picture of the screen; [mednafen](https://mednafen.github.io/) or any other that takes a cue sheet works for playing.
- **A BIOS image.** The console's own is Sony's and cannot be distributed, but the open source [OpenBIOS](https://github.com/grumpycoders/pcsx-redux/tree/main/src/mips/openbios) boots everything here. The copy the [player](/play/player.html) on this site uses is at [openbios.bin](/play/openbios.bin).

## The module

Make a directory and a module in it, add the toolchain, and add the tools a build uses as tool dependencies, which `go tool` then runs without installing anything:

```bash
mkdir hello && cd hello
go mod init example.com/hello
go get go.chrisrx.dev/psx
go get -tool go.chrisrx.dev/psx/tools/mips1fix go.chrisrx.dev/psx/tools/eco2exe \
             go.chrisrx.dev/psx/tools/mkpsxiso go.chrisrx.dev/psx/tools/psxemu
curl -LO https://chrisrx.github.io/psxsdk/play/openbios.bin
```

## The program

One file, `main.go`:

```go
// Hello: a line of text on the screen.
package main

import "go.chrisrx.dev/psx/yaroze"

const (
	screenW, screenH = 320, 240

	// The ordering table's size, as a power of two: one entry is enough
	// for a frame that only clears the screen.
	otLength  = 1
	packetMax = 64
)

var (
	screen     = yaroze.Screen{Background: yaroze.CVECTOR{R: 60, G: 120, B: 120}}
	otTags     [2 << otLength]yaroze.GsOT_TAG
	packetArea [2 * packetMax / 4]uint32
	textBuf    [32]byte
)

func main() {
	yaroze.SetVideoMode(yaroze.MODE_NTSC)
	yaroze.GsInitGraph(screenW, screenH, 4, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, screenH)
	screen.Init(otTags[:], packetArea[:])
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(16, 16, 256, 200, 0, 512)

	hello := yaroze.CString(textBuf[:], "hello from go")
	for {
		screen.Begin()
		screen.End()
		yaroze.FntPrint(hello)
		yaroze.FntFlush(-1)
	}
}
```

It already has the shape every program has: set up the display, then draw a frame, wait for the screen, and swap, for ever. Three things about it are not how a program for a modern machine would read.

**Everything is a package-level variable.** The machine has two megabytes of RAM, no memory protection and, in this runtime, no garbage collector running, so memory a frame allocates is memory the program never gets back. Declaring the buffers once, as globals, is how the Net Yaroze programs this toolchain descends from did it too, and nothing in the tutorial allocates after startup.

**The frame is a list before it is a picture.** The GPU does not draw what it is told as it is told. A program sorts drawing commands into an ordering table, an array of linked lists indexed by depth, and then hands the table to the GPU, which walks it while the program goes on to the next frame. There are two tables, and two packet areas for the commands themselves, because the GPU is still drawing one frame while the program builds the next. The `Screen` holds both pairs, taking each array in halves: `Begin` empties the table for the buffer about to be drawn and returns it, for the frame's drawing to be sorted into, and `End` waits for the GPU and the display, swaps the buffers, sorts a command that clears the screen to its `Background` color in front of everything, and starts the GPU on the table. Underneath are the library's own `GsClearOt`, `GsSortClear` and `GsDrawOt`, which a program can call itself. The packet area is declared as words rather than bytes because the library stores words into it and the compiler aligns an array of bytes only to a byte.

**The text goes through the library's font.** `FntLoad` puts the library's 8x8 font into video memory at a corner the display never shows, `FntOpen` opens a window of text at a position on the screen, and `FntPrint` writes into it. In C `FntPrint` is printf-like; here it takes a finished string, which `CString` copies into a buffer the program owns, with the NUL terminator the library expects.

The program needs no build constraint. The `yaroze` package builds under the ordinary Go toolchain too, with every function of the library a stub that panics if it is called, so an editor shows the library's documentation as you write, and `go vet ./...` and `go build ./...` work on the program. Only TinyGo's PlayStation target has the library itself, and only a program it built runs.

## Building it

Three steps turn the source into an executable the console boots:

```bash
tinygo build -target=playstation-yaroze -o prog.elf .
go tool mips1fix prog.elf
go tool eco2exe prog.elf MAIN.EXE
```

TinyGo compiles and links the program against the Net Yaroze library's addresses. `mips1fix` replaces the few instructions LLVM emits that the console's R3000A does not have. `eco2exe` combines the program with the library itself, which a Net Yaroze had resident in RAM and an ordinary console does not, and writes the result as the PSX-EXE the BIOS knows how to load.

One more step puts it on a disc:

```bash
go tool mkpsxiso -o hello.bin --boot MAIN.EXE --file MAIN.EXE=MAIN.EXE
```

`--boot` writes the `SYSTEM.CNF` the BIOS reads to find the executable, and `--file` places the executable on the disc; there is nothing else to put there yet. The result is `hello.bin` and `hello.cue`, which any emulator opens.

## Running it

`psxemu` boots the disc headless and, when stopped, saves what the screen shows:

```bash
go tool psxemu --bios openbios.bin --disc hello.cue --png hello.png
```

Stop it after a few seconds and `hello.png` holds the frame: a teal screen with the text in the corner. mednafen and other emulators open the cue sheet directly, and a console that plays burned discs boots it from a CD-R.

The next page draws something: [a cube from the disc](cube.md).
