---
title: elf2exe
weight: 2
---

`elf2exe` writes a linked program out as a PSX-EXE, for a program that is the
whole of the machine rather than a Net Yaroze one.

It is the plain conversion: the loadable image and the entry point go into the
2048-byte header and nothing is added. `eco2exe` exists because a Net Yaroze
program is only half a program, and has to be combined with the library the
boot disc leaves resident in RAM; a program that brings its own crt0 and
drives the hardware itself needs none of that.

```bash
$ elf2exe prog.elf MAIN.EXE
created "MAIN.EXE": load=0x80010000 entry=0x80010000 8192 bytes, d00f145ee4fd44f1
```

The input may be ELF or ECOFF, since only the loadable segments and the entry
point are read. `--stack-addr` sets the stack pointer the BIOS installs before
jumping to the entry point, which matters only for a program whose crt0 does
not set its own.
