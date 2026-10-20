---
title: How it fits together
weight: 2
group: Docs
section: Architecture
description: How psxsdk fits together and works inside: the path from a Go package to a disc, the emulator, the compiler in the browser, and the choice of compression.
---

```
examples/check/main.go
        |
        |  nyc disc ./examples/check
        |    TinyGo, -target=playstation-yaroze      tinygo, or TinyGo as WebAssembly
        |      -> prog.elf, traps replaced           (mips1fix)
        |      -> MAIN.EXE, libps combined in        (eco2exe)
        |      -> the disc, with assets/... that       (mkpsxiso)
        |         the program registers
        v
check.bin + check.cue
        |
        +--------------------------+--------------------------+-----------------------------+
        v                          v                          v                             v
emu (go test, emutest)    web (WebAssembly, Pages)    nyc run, psxplay (a window)    real hardware / psxe
```

[`nyc`](../tools/nyc.md) runs all of it, from the package to the disc, in one step, and `nyc run` plays the disc in a window as well. Each step is also a tool of its own, named on its right, for a build that wants only one of them.

A Net Yaroze program is only half a program: the rest is `libps`, the library the Net Yaroze boot disc leaves resident in RAM. `eco2exe` combines the two into a standalone PSX-EXE, so the result boots from an ordinary disc in an emulator or on a console that plays burned discs, with no Net Yaroze hardware involved.
