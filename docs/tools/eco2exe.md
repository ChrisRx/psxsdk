---
title: eco2exe
weight: 1
---

The `eco2exe` tool takes a Net Yaroze program and creates a working PSX-EXE executable ready to be used in an emulator (if it supports running bare PSX-EXEs), or built into a disc image to be loaded by a real PlayStation (if it can play burned discs). The input may be the ECOFF produced by the SDK's own toolchain or the ELF produced by TinyGo or a current mipsel-none-elf toolchain.

It does this by parsing the compiled program, combining it with the Net Yaroze development library (aka `libps.exe`) and then converting the combined program to the PSX-EXE executable format. The executable is patched with a startup stub that initializes the library before jumping to the program's entry point.

```bash
$ eco2exe prog.elf MAIN.EXE
created "MAIN.EXE": 47fb1ad6d1aee3c8bdf71bdc2ecd144b
```

*Note: The Net Yaroze development library itself is relatively small, so it has been embedded in the `eco2exe` binary, meaning it doesn't need to be provided by the user!*

[`yaroze`](yaroze.md) does the same for every program it builds, so a build with it needs no separate step.
