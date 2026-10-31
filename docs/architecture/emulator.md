---
title: The emulator
weight: 1
---

[`emu`](emu) is a rewrite in Go of [psxe](https://github.com/allkern/psxe) by Allkern (Lisandro Alarcón), written with psxe as a guide and used under its MIT license (see [`licenses/PSXE-LICENSE.txt`](licenses/PSXE-LICENSE.txt)). It emulates the CPU, GTE, GPU, SPU (with reverb and gaussian interpolation), CD-ROM drive including XA and CD audio, MDEC, DMA, timers, controllers, memory cards and the serial port. It was written to be driven from tests rather than played: it runs headless and deterministically, counting emulated cycles with no reference to the wall clock, so a run takes the same course however loaded the host is.

[`emu/emutest`](emu/emutest) wraps it for Go tests. A test boots a machine, runs it against budgets, presses buttons, and asserts on what was printed, what is in memory or what is on screen:

```go
m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("check.cue"))
m.RunUntilTTY("Execute !", 10*emutest.Second)
m.RunFrames(60)
m.Hold(1, emu.ButtonSelect, 10)
m.RunUntilTTY("End of Main", 5*emutest.Second)
```

A program that runs an illegal instruction or makes a misaligned access fails the test there, with the registers, the instructions leading up to it and the program's output. The [`tests`](tests) package uses this to boot every example and compare screenshots against golden images, which are OpenBIOS's; a BIOS of Sony's shows a few screens differently, by the count of syncs it reports say, and has a directory of its own under `tests/testdata` holding the images where it differs.

`Machine.ConnectSerial` plugs a cable into the serial port and returns its host end, which package [`serial`](serial) takes as a `Port`, as it takes a serial device on the host. The serial tests in [`tests`](tests) boot a monitor and talk to it over that cable: the clean-room one in [`serial/monitor`](serial/monitor), and when `PSX_YAROZE_BOOT` names an image of the Net Yaroze boot disc and `PSX_SONY_BIOS` is set too, Sony's own, from a copy of the disc with the access card check patched out. The same specs run against both: the monitor's commands, uploads, `siocons`'s batch files and the remote debugger.

`Machine.SetBreak` sets a breakpoint: a function called before the instruction at an address runs, with the machine stopped there, which can read the registers and memory, change them, and stop the run, which then returns `emu.ErrBreak`; the next run executes the instruction without stopping again. The addresses are a bitmap over RAM, so any number of breakpoints costs the CPU's loop one test a step. A breakpoint on a function needs the function to have an address: at the size optimization the programs are built with, the compiler inlines most small functions into their callers, and `go tool nm` on the program's ELF lists the ones it kept; `//go:noinline` keeps one. The web player's worker exposes them as `break`, `step` and `registers` messages, and `yaroze run --break` and `psxemu --break` print the registers at each hit.

The emulator follows psxe closely so the two can be compared one instruction at a time: the flake builds psxe with a patch that writes an instruction trace, `psxemu --trace` writes the same format, and `psxtrace diff` reports the first record where they disagree.

[`web`](web) compiles the emulator to WebAssembly (using Go's SIMD support to draw polygons and rectangles four pixels at a time, to convert the display's pixels, and for the MDEC's inverse DCT) and runs it in a worker behind the page in `web/static`. The player and [`yaroze run`](../tools/yaroze.md#the-window) both start fast. The BIOS and a program's loading are a few seconds of black screen, so until the display first shows something they run the machine as fast as the host allows, drawing and playing none of it, and then go on in real time. `Machine.Shown` is what they ask. The Pages workflow builds every example's disc and publishes the site on each push to `main`, and builds it for every pull request, which catches a change that breaks an example.
