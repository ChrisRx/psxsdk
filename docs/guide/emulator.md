---
title: The emulator
weight: 3
---

[`emu`](emu) is a rewrite in Go of [psxe](https://github.com/allkern/psxe) by Allkern (Lisandro Alarcón), written with psxe as a guide and used under its MIT license (see [`emu/PSXE-LICENSE.txt`](emu/PSXE-LICENSE.txt)). It emulates the CPU, GTE, GPU, SPU (with reverb and gaussian interpolation), CD-ROM drive including XA and CD audio, MDEC, DMA, timers, controllers and memory cards. It was written to be driven from tests rather than played: it runs headless and deterministically, counting emulated cycles with no reference to the wall clock, so a run takes the same course however loaded the host is.

[`emu/emutest`](emu/emutest) wraps it for Go tests. A test boots a machine, runs it against budgets, presses buttons, and asserts on what was printed, what is in memory or what is on screen:

```go
m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("check.cue"))
m.RunUntilTTY("Execute !", 10*emutest.Second)
m.RunFrames(60)
m.Hold(1, emu.ButtonSelect, 10)
m.RunUntilTTY("End of Main", 5*emutest.Second)
```

A program that runs an illegal instruction or makes a misaligned access fails the test there, with the registers, the instructions leading up to it and the program's output. The [`tests`](tests) package uses this to boot every example and compare screenshots against golden images.

The emulator follows psxe closely so the two can be compared one instruction at a time: the flake builds psxe with a patch that writes an instruction trace, `psxemu --trace` writes the same format, and `psxtrace diff` reports the first record where they disagree.

[`web`](web) compiles the emulator to WebAssembly (using Go's SIMD support for pixel conversion) and runs it in a worker behind the page in `web/static`. The Pages workflow builds every example's disc and publishes the site on each push to `main`, and publishes a preview for any pull request that changes an example.
